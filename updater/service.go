package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	defaultManifestURL = "https://github.com/kasisama/PetYC/releases/latest/download/update-manifest.json"
	defaultReleaseURL  = "https://github.com/kasisama/PetYC/releases/latest"
	maxManifestBytes   = 2 << 20
	maxArtifactBytes   = 512 << 20
)

type CheckInfo struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	Available      bool   `json:"available"`
	Notes          string `json:"notes"`
	PublishedAt    string `json:"publishedAt"`
	CanAutoUpdate  bool   `json:"canAutoUpdate"`
	InstallMode    string `json:"installMode"`
	Reason         string `json:"reason"`
	ReleaseURL     string `json:"releaseUrl"`
}

type Status struct {
	State          string `json:"state"`
	Progress       int    `json:"progress"`
	Downloaded     int64  `json:"downloaded"`
	Total          int64  `json:"total"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	Error          string `json:"error"`
}

type Config struct {
	CurrentVersion string
	ManifestURL    string
	SignatureURL   string
	PublicKey      string
	ReleaseURL     string
	HTTPClient     *http.Client
	ExecutablePath func() (string, error)
	RuntimeOS      string
	RuntimeArch    string
	Environment    func(string) string
	FileExists     func(string) bool
}

type Service struct {
	config Config

	mu            sync.Mutex
	status        Status
	lastCheck     CheckInfo
	manifest      Manifest
	artifact      Artifact
	lastCheckedAt time.Time
	installing    bool
	healthURL     string
	shutdown      func()
}

func NewService(config Config) *Service {
	if config.ManifestURL == "" {
		config.ManifestURL = defaultManifestURL
	}
	if config.SignatureURL == "" {
		config.SignatureURL = config.ManifestURL + ".sig"
	}
	if config.PublicKey == "" {
		config.PublicKey = DefaultPublicKey
	}
	if config.ReleaseURL == "" {
		config.ReleaseURL = defaultReleaseURL
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	if config.ExecutablePath == nil {
		config.ExecutablePath = os.Executable
	}
	if config.RuntimeOS == "" {
		config.RuntimeOS = runtime.GOOS
	}
	if config.RuntimeArch == "" {
		config.RuntimeArch = runtime.GOARCH
	}
	if config.Environment == nil {
		config.Environment = os.Getenv
	}
	if config.FileExists == nil {
		config.FileExists = func(path string) bool {
			_, err := os.Stat(path)
			return err == nil
		}
	}
	return &Service{config: config, status: Status{State: "idle", CurrentVersion: config.CurrentVersion}}
}

func (service *Service) SetRuntime(healthURL string, shutdown func()) {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.healthURL = strings.TrimRight(healthURL, "/")
	service.shutdown = shutdown
}

func (service *Service) Status() Status {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.status
}

func (service *Service) Check(ctx context.Context, force bool) (CheckInfo, error) {
	service.mu.Lock()
	if service.installing {
		service.mu.Unlock()
		return CheckInfo{}, errors.New("更新任务正在执行")
	}
	if !force && !service.lastCheckedAt.IsZero() && time.Since(service.lastCheckedAt) < 24*time.Hour {
		result := service.lastCheck
		service.mu.Unlock()
		return result, nil
	}
	service.status.State = "checking"
	service.status.Error = ""
	service.mu.Unlock()

	manifest, artifact, result, err := service.fetchUpdate(ctx)
	service.mu.Lock()
	defer service.mu.Unlock()
	if err != nil {
		service.status.State = "failed"
		service.status.Error = err.Error()
		return CheckInfo{}, err
	}
	service.manifest = manifest
	service.artifact = artifact
	service.lastCheck = result
	service.lastCheckedAt = time.Now()
	service.status.State = "idle"
	if result.Available {
		service.status.State = "available"
	}
	service.status.LatestVersion = result.LatestVersion
	return result, nil
}

func (service *Service) StartInstall() error {
	service.mu.Lock()
	if service.installing {
		service.mu.Unlock()
		return errors.New("更新任务正在执行")
	}
	service.installing = true
	service.status = Status{State: "checking", CurrentVersion: service.config.CurrentVersion}
	service.mu.Unlock()

	go service.runInstall()
	return nil
}

func (service *Service) runInstall() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	manifest, artifact, info, err := service.fetchUpdate(ctx)
	if err != nil {
		service.fail(err)
		return
	}
	if !info.Available {
		service.fail(errors.New("当前已是最新版本"))
		return
	}
	if !info.CanAutoUpdate {
		service.fail(errors.New(info.Reason))
		return
	}

	service.mu.Lock()
	service.manifest = manifest
	service.artifact = artifact
	service.lastCheck = info
	service.status.State = "downloading"
	service.status.LatestVersion = manifest.Version
	service.status.Total = artifact.Size
	service.mu.Unlock()

	executable, err := service.config.ExecutablePath()
	if err != nil {
		service.fail(fmt.Errorf("定位当前程序: %w", err))
		return
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		service.fail(fmt.Errorf("解析程序路径: %w", err))
		return
	}
	newPath := executable + ".new"
	if err := service.download(ctx, artifact, newPath); err != nil {
		service.fail(err)
		return
	}

	service.mu.Lock()
	service.status.State = "verifying"
	service.status.Progress = 100
	service.mu.Unlock()

	workDir, err := os.Getwd()
	if err != nil {
		service.fail(fmt.Errorf("读取工作目录: %w", err))
		return
	}
	service.mu.Lock()
	healthURL := service.healthURL
	shutdown := service.shutdown
	service.mu.Unlock()
	if healthURL == "" || shutdown == nil {
		service.fail(errors.New("服务尚未完成更新运行环境初始化"))
		return
	}

	if err := launchHelper(helperLaunchConfig{
		Target:          executable,
		Source:          newPath,
		Database:        filepath.Join(workDir, "pet_game.db"),
		WorkingDir:      workDir,
		HealthURL:       healthURL,
		ExpectedVersion: manifest.Version,
		OriginalArgs:    append([]string(nil), os.Args[1:]...),
	}); err != nil {
		service.fail(err)
		return
	}

	service.mu.Lock()
	service.status.State = "restarting"
	service.mu.Unlock()
	time.Sleep(750 * time.Millisecond)
	shutdown()
}

func (service *Service) fail(err error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.installing = false
	service.status.State = "failed"
	service.status.Error = err.Error()
}

func (service *Service) fetchUpdate(ctx context.Context) (Manifest, Artifact, CheckInfo, error) {
	result := CheckInfo{
		CurrentVersion: service.config.CurrentVersion,
		InstallMode:    "manual",
		ReleaseURL:     service.config.ReleaseURL,
	}
	if strings.TrimSpace(service.config.CurrentVersion) == "" || service.config.CurrentVersion == "dev" {
		result.Reason = "开发构建不支持在线更新"
		return Manifest{}, Artifact{}, result, nil
	}
	if _, err := parseVersion(service.config.CurrentVersion); err != nil {
		return Manifest{}, Artifact{}, CheckInfo{}, fmt.Errorf("当前版本无效: %w", err)
	}
	raw, err := service.fetchSmall(ctx, service.config.ManifestURL)
	if err != nil {
		return Manifest{}, Artifact{}, CheckInfo{}, fmt.Errorf("获取更新清单: %w", err)
	}
	signature, err := service.fetchSmall(ctx, service.config.SignatureURL)
	if err != nil {
		return Manifest{}, Artifact{}, CheckInfo{}, fmt.Errorf("获取更新签名: %w", err)
	}
	manifest, err := ParseAndVerifyManifest(raw, signature, service.config.PublicKey)
	if err != nil {
		return Manifest{}, Artifact{}, CheckInfo{}, err
	}
	result.LatestVersion = manifest.Version
	result.Notes = manifest.Notes
	result.PublishedAt = manifest.PublishedAt
	result.Available, err = newerVersion(manifest.Version, service.config.CurrentVersion)
	if err != nil {
		return Manifest{}, Artifact{}, CheckInfo{}, err
	}
	artifact, ok := manifest.Platforms[service.config.RuntimeOS+"-"+service.config.RuntimeArch]
	if !ok {
		result.Reason = "当前操作系统或架构没有可用更新包"
		return manifest, Artifact{}, result, nil
	}
	result.CanAutoUpdate, result.Reason = service.autoUpdateCapability()
	if result.CanAutoUpdate {
		result.InstallMode = "portable"
	}
	return manifest, artifact, result, nil
}

func (service *Service) fetchSmall(ctx context.Context, address string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "PetYC-Updater/"+service.config.CurrentVersion)
	response, err := service.config.HTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxManifestBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxManifestBytes {
		return nil, errors.New("更新清单超过允许的最大尺寸")
	}
	return raw, nil
}

func (service *Service) autoUpdateCapability() (bool, string) {
	if service.config.RuntimeOS != "windows" && service.config.RuntimeOS != "linux" {
		return false, "当前操作系统仅支持手动更新"
	}
	if service.config.RuntimeArch != "amd64" {
		return false, "当前 CPU 架构仅支持手动更新"
	}
	if service.config.FileExists("/.dockerenv") || service.config.Environment("container") != "" {
		return false, "Docker 环境请通过镜像更新"
	}
	if service.config.Environment("INVOCATION_ID") != "" {
		return false, "systemd 服务请通过部署命令更新"
	}
	executable, err := service.config.ExecutablePath()
	if err != nil {
		return false, "无法定位当前程序"
	}
	directory := filepath.Dir(executable)
	testFile, err := os.CreateTemp(directory, ".petyc-update-write-test-*")
	if err != nil {
		return false, "程序目录不可写，请手动更新"
	}
	testName := testFile.Name()
	_ = testFile.Close()
	_ = os.Remove(testName)
	return true, ""
}

func (service *Service) download(ctx context.Context, artifact Artifact, destination string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, artifact.URL, nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", "PetYC-Updater/"+service.config.CurrentVersion)
	response, err := service.config.HTTPClient.Do(request)
	if err != nil {
		return fmt.Errorf("下载更新包: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("下载更新包: HTTP %d", response.StatusCode)
	}
	if artifact.Size > maxArtifactBytes {
		return errors.New("更新包超过允许的最大尺寸")
	}
	temporary := destination + ".download"
	_ = os.Remove(temporary)
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("创建更新临时文件: %w", err)
	}
	defer func() {
		_ = file.Close()
		_ = os.Remove(temporary)
	}()

	hash := sha256.New()
	buffer := make([]byte, 128*1024)
	var downloaded int64
	reader := io.LimitReader(response.Body, maxArtifactBytes+1)
	for {
		count, readErr := reader.Read(buffer)
		if count > 0 {
			downloaded += int64(count)
			if downloaded > maxArtifactBytes {
				return errors.New("更新包超过允许的最大尺寸")
			}
			if _, err := file.Write(buffer[:count]); err != nil {
				return fmt.Errorf("写入更新包: %w", err)
			}
			_, _ = hash.Write(buffer[:count])
			service.mu.Lock()
			service.status.Downloaded = downloaded
			service.status.Progress = int(downloaded * 100 / artifact.Size)
			if service.status.Progress > 100 {
				service.status.Progress = 100
			}
			service.mu.Unlock()
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return fmt.Errorf("读取更新包: %w", readErr)
		}
	}
	if downloaded != artifact.Size {
		return fmt.Errorf("更新包大小不匹配: 得到 %d，期望 %d", downloaded, artifact.Size)
	}
	if actual := hex.EncodeToString(hash.Sum(nil)); !strings.EqualFold(actual, artifact.SHA256) {
		return errors.New("更新包 SHA-256 校验失败")
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("同步更新包: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("关闭更新包: %w", err)
	}
	_ = os.Remove(destination)
	if err := os.Rename(temporary, destination); err != nil {
		return fmt.Errorf("准备更新包: %w", err)
	}
	if service.config.RuntimeOS != "windows" {
		if err := os.Chmod(destination, 0o755); err != nil {
			return fmt.Errorf("设置更新包权限: %w", err)
		}
	}
	return nil
}
