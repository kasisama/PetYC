package admin

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	appconfig "qq-pet-saas/config"
	"qq-pet-saas/models"
)

const (
	maxProfileUploadBytes = 10 << 20
	maxProfileArchiveSize = 100 << 20
	maxProfileAssetFiles  = 500
)

type profileManifest struct {
	SchemaVersion int               `json:"schema_version"`
	AppVersion    string            `json:"app_version"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	CreatedAt     time.Time         `json:"created_at"`
	Files         map[string]string `json:"files"`
}

type profileView struct {
	ID            string                    `json:"id"`
	Name          string                    `json:"name"`
	Description   string                    `json:"description"`
	Source        string                    `json:"source"`
	SchemaVersion int                       `json:"schema_version"`
	AppVersion    string                    `json:"app_version"`
	Builtin       bool                      `json:"builtin"`
	Active        bool                      `json:"active"`
	Dirty         bool                      `json:"dirty"`
	Summary       appconfig.SnapshotSummary `json:"summary"`
	CreatedAt     time.Time                 `json:"created_at"`
	UpdatedAt     time.Time                 `json:"updated_at"`
}

type ProfileAPI struct{ DB *gorm.DB }

func RegisterProfileRoutes(group *gin.RouterGroup, db *gorm.DB) {
	api := &ProfileAPI{DB: db}
	group.GET("/config/profiles", api.List)
	group.POST("/config/profiles", api.Create)
	group.POST("/config/profiles/import", api.Import)
	group.POST("/config/profiles/:id/capture", api.Capture)
	group.POST("/config/profiles/:id/validate", api.Validate)
	group.POST("/config/profiles/:id/activate", api.Activate)
	group.GET("/config/profiles/:id/export", api.Export)
	group.DELETE("/config/profiles/:id", api.Delete)
}

func (api *ProfileAPI) List(c *gin.Context) {
	var profiles []models.ConfigProfile
	if err := api.DB.Order("builtin desc, updated_at desc, name asc").Find(&profiles).Error; err != nil {
		Error(c, codeInternalError, "配置方案读取失败")
		return
	}
	status, err := appconfig.GetConfigStatus(api.DB)
	if err != nil {
		Error(c, codeInternalError, "配置状态读取失败")
		return
	}
	views := make([]profileView, 0, len(profiles))
	for _, profile := range profiles {
		snapshot, decodeErr := appconfig.DecodeSnapshot(profile.Payload)
		if decodeErr != nil {
			continue
		}
		views = append(views, makeProfileView(profile, snapshot, status))
	}
	Success(c, gin.H{"items": views, "active_profile_id": status.ActiveProfileID, "dirty": status.ProfileDirty})
}

func (api *ProfileAPI) Create(c *gin.Context) {
	var request struct{ Name, Description string }
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.Name) == "" {
		Error(c, codeInvalidPayload, "请输入方案名称")
		return
	}
	snapshot, err := appconfig.CaptureSnapshot(api.DB)
	if err != nil {
		Error(c, codeInternalError, "读取当前配置失败")
		return
	}
	var profile models.ConfigProfile
	err = api.DB.Transaction(func(tx *gorm.DB) error {
		var createErr error
		profile, createErr = appconfig.CreateProfileFromSnapshot(tx, uniqueProfileName(tx, request.Name), request.Description, "user", false, snapshot)
		if createErr != nil {
			return createErr
		}
		return appconfig.SetActiveProfile(tx, profile.ID, false)
	})
	if err != nil {
		Error(c, codeInvalidPayload, err.Error())
		return
	}
	Success(c, makeProfileView(profile, snapshot, appconfig.ConfigStatus{ActiveProfileID: profile.ID}))
}

func (api *ProfileAPI) Capture(c *gin.Context) {
	profile, ok := api.profile(c)
	if !ok {
		return
	}
	if profile.Builtin {
		Error(c, codeInvalidPayload, "官方默认方案为只读，请另存为新方案")
		return
	}
	snapshot, err := appconfig.CaptureSnapshot(api.DB)
	if err != nil {
		Error(c, codeInternalError, "读取当前配置失败")
		return
	}
	payload, err := appconfig.EncodeSnapshot(snapshot)
	if err != nil {
		Error(c, codeInvalidPayload, err.Error())
		return
	}
	err = api.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&profile).Updates(map[string]any{"payload": payload, "app_version": appconfig.ApplicationVersion, "schema_version": appconfig.ProfileSchemaVersion}).Error; err != nil {
			return err
		}
		state, err := currentConfigState(tx)
		if err != nil {
			return err
		}
		if state.ActiveProfileID == profile.ID {
			return tx.Model(state).Update("profile_dirty", false).Error
		}
		return nil
	})
	if err != nil {
		Error(c, codeInternalError, "保存方案失败")
		return
	}
	Success(c, gin.H{"message": "当前配置已保存到方案", "summary": snapshot.Summary()})
}

func (api *ProfileAPI) Validate(c *gin.Context) {
	profile, ok := api.profile(c)
	if !ok {
		return
	}
	snapshot, err := appconfig.DecodeSnapshot(profile.Payload)
	if err != nil {
		Error(c, codeInvalidPayload, err.Error())
		return
	}
	conflicts, err := appconfig.CheckSnapshotCompatibility(api.DB, snapshot)
	if err != nil {
		Error(c, codeInternalError, "兼容性检查失败")
		return
	}
	Success(c, gin.H{"valid": len(conflicts) == 0, "conflicts": conflicts, "summary": snapshot.Summary()})
}

func (api *ProfileAPI) Activate(c *gin.Context) {
	profile, ok := api.profile(c)
	if !ok {
		return
	}
	var request struct {
		DiscardChanges bool `json:"discard_changes"`
	}
	_ = c.ShouldBindJSON(&request)
	status, err := appconfig.GetConfigStatus(api.DB)
	if err != nil {
		Error(c, codeInternalError, "配置状态读取失败")
		return
	}
	if status.ProfileDirty && status.ActiveProfileID != profile.ID && !request.DiscardChanges {
		c.JSON(http.StatusConflict, gin.H{"code": 4091, "msg": "当前方案有未保存修改", "data": gin.H{"dirty": true}})
		return
	}
	snapshot, err := appconfig.DecodeSnapshot(profile.Payload)
	if err != nil {
		Error(c, codeInvalidPayload, err.Error())
		return
	}
	conflicts, err := appconfig.CheckSnapshotCompatibility(api.DB, snapshot)
	if err != nil {
		Error(c, codeInternalError, "兼容性检查失败")
		return
	}
	if len(conflicts) > 0 {
		c.JSON(http.StatusConflict, gin.H{"code": 4092, "msg": "目标方案与现有玩家数据不兼容", "data": gin.H{"conflicts": conflicts}})
		return
	}
	if err = appconfig.ValidateLaunchReadiness(snapshot); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 4093, "msg": err.Error()})
		return
	}
	previous, err := appconfig.CaptureSnapshot(api.DB)
	if err != nil {
		Error(c, codeInternalError, "创建回滚快照失败")
		return
	}
	if err = api.DB.Transaction(func(tx *gorm.DB) error {
		if err := appconfig.ApplySnapshot(tx, snapshot); err != nil {
			return err
		}
		return appconfig.SetActiveProfile(tx, profile.ID, false)
	}); err != nil {
		Error(c, codeInvalidPayload, err.Error())
		return
	}
	if err = reloadRuntimeConfig(api.DB); err != nil {
		_ = api.DB.Transaction(func(tx *gorm.DB) error {
			if restoreErr := appconfig.ApplySnapshot(tx, previous); restoreErr != nil {
				return restoreErr
			}
			return appconfig.SetActiveProfile(tx, status.ActiveProfileID, status.ProfileDirty)
		})
		_ = reloadRuntimeConfig(api.DB)
		Error(c, codeInternalError, "方案热重载失败，已恢复切换前配置: "+err.Error())
		return
	}
	Success(c, gin.H{"message": "配置方案已切换并生效", "active_profile_id": profile.ID})
}

func (api *ProfileAPI) Delete(c *gin.Context) {
	profile, ok := api.profile(c)
	if !ok {
		return
	}
	if profile.Builtin {
		Error(c, codeInvalidPayload, "官方默认方案不能删除")
		return
	}
	status, err := appconfig.GetConfigStatus(api.DB)
	if err != nil {
		Error(c, codeInternalError, "配置状态读取失败")
		return
	}
	if status.ActiveProfileID == profile.ID {
		Error(c, codeInvalidPayload, "当前正在使用的方案不能删除")
		return
	}
	if err := api.DB.Delete(&profile).Error; err != nil {
		Error(c, codeInternalError, "删除方案失败")
		return
	}
	Success(c, gin.H{"message": "配置方案已删除"})
}

func (api *ProfileAPI) Export(c *gin.Context) {
	profile, ok := api.profile(c)
	if !ok {
		return
	}
	snapshot, err := appconfig.DecodeSnapshot(profile.Payload)
	if err != nil {
		Error(c, codeInvalidPayload, err.Error())
		return
	}
	archive, err := buildProfileArchive(profile, snapshot)
	if err != nil {
		Error(c, codeInvalidPayload, err.Error())
		return
	}
	filename := fmt.Sprintf("%s_%s.qqpet-config", safeProfileFilename(profile.Name), time.Now().UTC().Format("20060102T150405Z"))
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	c.Data(http.StatusOK, "application/vnd.qqpet.config+zip", archive)
}

func (api *ProfileAPI) Import(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		Error(c, codeInvalidPayload, "请选择配置方案文件")
		return
	}
	if file.Size <= 0 || file.Size > maxProfileUploadBytes {
		Error(c, codeInvalidPayload, "配置包不能超过 10MB")
		return
	}
	reader, err := file.Open()
	if err != nil {
		Error(c, codeInvalidPayload, "配置包读取失败")
		return
	}
	defer reader.Close()
	raw, err := io.ReadAll(io.LimitReader(reader, maxProfileUploadBytes+1))
	if err != nil || len(raw) > maxProfileUploadBytes {
		Error(c, codeInvalidPayload, "配置包读取失败或超过限制")
		return
	}
	manifest, snapshot, assets, err := parseProfileArchive(raw)
	if err != nil {
		Error(c, codeInvalidPayload, err.Error())
		return
	}
	conflicts, err := appconfig.CheckSnapshotCompatibility(api.DB, snapshot)
	if err != nil {
		Error(c, codeInternalError, "配置方案兼容性预检失败")
		return
	}
	profileID := uuid.NewString()
	finalRoot := filepath.Join("图片", "导入", profileID)
	tempRoot := filepath.Join("图片", ".import-"+profileID)
	if err = writeImportedAssets(tempRoot, assets); err != nil {
		_ = os.RemoveAll(tempRoot)
		Error(c, codeInternalError, "导入图片写入失败")
		return
	}
	rewriteSnapshotAssets(&snapshot, assets, filepath.ToSlash(filepath.Join("导入", profileID)))
	if err = appconfig.ValidateSnapshot(snapshot); err != nil {
		_ = os.RemoveAll(tempRoot)
		Error(c, codeInvalidPayload, err.Error())
		return
	}
	if err = os.MkdirAll(filepath.Dir(finalRoot), 0o755); err == nil {
		err = os.Rename(tempRoot, finalRoot)
	}
	if err != nil {
		_ = os.RemoveAll(tempRoot)
		Error(c, codeInternalError, "导入资源提交失败")
		return
	}
	payload, _ := appconfig.EncodeSnapshot(snapshot)
	profile := models.ConfigProfile{ID: profileID, Name: uniqueProfileName(api.DB, manifest.Name), Description: manifest.Description, Source: "import", SchemaVersion: manifest.SchemaVersion, AppVersion: manifest.AppVersion, Payload: payload}
	if err = api.DB.Create(&profile).Error; err != nil {
		_ = os.RemoveAll(finalRoot)
		Error(c, codeInternalError, "创建导入方案失败")
		return
	}
	Success(c, gin.H{"message": "方案已导入，尚未应用", "profile": makeProfileView(profile, snapshot, appconfig.ConfigStatus{}), "conflicts": conflicts})
}

func (api *ProfileAPI) profile(c *gin.Context) (models.ConfigProfile, bool) {
	var profile models.ConfigProfile
	if err := api.DB.First(&profile, "id = ?", c.Param("id")).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			Error(c, 4040, "配置方案不存在")
		} else {
			Error(c, codeInternalError, "配置方案读取失败")
		}
		return profile, false
	}
	return profile, true
}

func makeProfileView(profile models.ConfigProfile, snapshot appconfig.ConfigSnapshot, status appconfig.ConfigStatus) profileView {
	return profileView{ID: profile.ID, Name: profile.Name, Description: profile.Description, Source: profile.Source, SchemaVersion: profile.SchemaVersion, AppVersion: profile.AppVersion, Builtin: profile.Builtin, Active: status.ActiveProfileID == profile.ID, Dirty: status.ActiveProfileID == profile.ID && status.ProfileDirty, Summary: snapshot.Summary(), CreatedAt: profile.CreatedAt, UpdatedAt: profile.UpdatedAt}
}
func uniqueProfileName(db *gorm.DB, requested string) string {
	base := strings.TrimSpace(requested)
	if base == "" {
		base = "导入方案"
	}
	name := base
	for index := 2; ; index++ {
		var count int64
		db.Model(&models.ConfigProfile{}).Where("name = ?", name).Count(&count)
		if count == 0 {
			return name
		}
		name = fmt.Sprintf("%s (%d)", base, index)
	}
}
func currentConfigState(db *gorm.DB) (*models.AdminConfigState, error) {
	var state models.AdminConfigState
	err := db.First(&state, uint(1)).Error
	return &state, err
}
func reloadRuntimeConfig(db *gorm.DB) error {
	if err := appconfig.LoadAllConfigsFromDB(db); err != nil {
		return err
	}
	if RebuildCommandRoutesFunc != nil {
		return RebuildCommandRoutesFunc()
	}
	return nil
}

func buildProfileArchive(profile models.ConfigProfile, snapshot appconfig.ConfigSnapshot) ([]byte, error) {
	configRaw, _ := json.MarshalIndent(snapshot, "", "  ")
	files := map[string][]byte{"config.json": configRaw}
	total := len(configRaw)
	for _, relative := range snapshotAssetPaths(snapshot) {
		data, err := os.ReadFile(filepath.Join("图片", filepath.FromSlash(relative)))
		if err != nil {
			return nil, fmt.Errorf("方案引用的图片不存在: %s", relative)
		}
		total += len(data)
		if total > maxProfileArchiveSize {
			return nil, errors.New("配置包资源总大小超过 100MB")
		}
		files["assets/"+filepath.ToSlash(relative)] = data
	}
	hashes := map[string]string{}
	for name, data := range files {
		sum := sha256.Sum256(data)
		hashes[name] = hex.EncodeToString(sum[:])
	}
	manifest := profileManifest{SchemaVersion: appconfig.ProfileSchemaVersion, AppVersion: appconfig.ApplicationVersion, Name: profile.Name, Description: profile.Description, CreatedAt: time.Now().UTC(), Files: hashes}
	manifestRaw, _ := json.MarshalIndent(manifest, "", "  ")
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	names = append([]string{"manifest.json"}, names...)
	for _, name := range names {
		entry, err := writer.Create(name)
		if err != nil {
			return nil, err
		}
		data := files[name]
		if name == "manifest.json" {
			data = manifestRaw
		}
		if _, err = entry.Write(data); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func parseProfileArchive(raw []byte) (profileManifest, appconfig.ConfigSnapshot, map[string][]byte, error) {
	var manifest profileManifest
	var snapshot appconfig.ConfigSnapshot
	assets := map[string][]byte{}
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return manifest, snapshot, nil, errors.New("配置包不是有效 ZIP 文件")
	}
	if len(reader.File) > maxProfileAssetFiles+2 {
		return manifest, snapshot, nil, errors.New("配置包文件数量超过限制")
	}
	files := map[string][]byte{}
	total := int64(0)
	for _, entry := range reader.File {
		name := filepath.ToSlash(entry.Name)
		clean := filepath.ToSlash(filepath.Clean(name))
		if clean != name || strings.HasPrefix(clean, "../") || filepath.IsAbs(entry.Name) {
			return manifest, snapshot, nil, errors.New("配置包包含不安全路径")
		}
		if entry.FileInfo().IsDir() {
			continue
		}
		if name != "manifest.json" && name != "config.json" && !strings.HasPrefix(name, "assets/") {
			return manifest, snapshot, nil, fmt.Errorf("配置包包含未知文件: %s", name)
		}
		if _, duplicated := files[name]; duplicated {
			return manifest, snapshot, nil, fmt.Errorf("配置包包含重复文件: %s", name)
		}
		if entry.UncompressedSize64 > maxProfileArchiveSize {
			return manifest, snapshot, nil, errors.New("配置包文件解压后过大")
		}
		stream, openErr := entry.Open()
		if openErr != nil {
			return manifest, snapshot, nil, openErr
		}
		data, readErr := io.ReadAll(io.LimitReader(stream, maxProfileArchiveSize+1))
		stream.Close()
		if readErr != nil {
			return manifest, snapshot, nil, readErr
		}
		total += int64(len(data))
		if total > maxProfileArchiveSize {
			return manifest, snapshot, nil, errors.New("配置包解压总大小超过 100MB")
		}
		files[name] = data
	}
	if err = json.Unmarshal(files["manifest.json"], &manifest); err != nil {
		return manifest, snapshot, nil, errors.New("manifest.json 无效")
	}
	if _, exists := manifest.Files["config.json"]; !exists {
		return manifest, snapshot, nil, errors.New("manifest.json 未声明 config.json")
	}
	for name := range files {
		if name == "manifest.json" {
			continue
		}
		if _, declared := manifest.Files[name]; !declared {
			return manifest, snapshot, nil, fmt.Errorf("文件未在清单中声明: %s", name)
		}
	}
	if manifest.SchemaVersion != appconfig.ProfileSchemaVersion {
		return manifest, snapshot, nil, fmt.Errorf("不支持的配置包版本 %d", manifest.SchemaVersion)
	}
	for name, want := range manifest.Files {
		if name == "manifest.json" || (name != "config.json" && !strings.HasPrefix(name, "assets/")) {
			return manifest, snapshot, nil, fmt.Errorf("清单包含不允许的文件: %s", name)
		}
		data, exists := files[name]
		if !exists {
			return manifest, snapshot, nil, fmt.Errorf("配置包缺少文件 %s", name)
		}
		sum := sha256.Sum256(data)
		if !strings.EqualFold(want, hex.EncodeToString(sum[:])) {
			return manifest, snapshot, nil, fmt.Errorf("文件校验失败: %s", name)
		}
	}
	if err = json.Unmarshal(files["config.json"], &snapshot); err != nil {
		return manifest, snapshot, nil, errors.New("config.json 无效")
	}
	if err = appconfig.ValidateSnapshot(snapshot); err != nil {
		return manifest, snapshot, nil, err
	}
	for name, data := range files {
		if !strings.HasPrefix(name, "assets/") {
			continue
		}
		if len(assets) >= maxProfileAssetFiles {
			return manifest, snapshot, nil, errors.New("图片文件数量超过限制")
		}
		detectedExtension, _, _, inspectErr := inspectImageUpload(bytes.NewReader(data))
		if inspectErr != nil {
			err = inspectErr
			return manifest, snapshot, nil, fmt.Errorf("图片 %s 无效: %w", name, err)
		}
		declaredExtension := strings.ToLower(filepath.Ext(name))
		if declaredExtension == ".jpeg" {
			declaredExtension = ".jpg"
		}
		if declaredExtension != detectedExtension {
			return manifest, snapshot, nil, fmt.Errorf("图片扩展名与实际格式不符: %s", name)
		}
		assets[strings.TrimPrefix(name, "assets/")] = data
	}
	referenced := snapshotAssetPaths(snapshot)
	for _, relative := range referenced {
		if _, exists := assets[relative]; !exists {
			return manifest, snapshot, nil, fmt.Errorf("配置包缺少引用图片: %s", relative)
		}
	}
	if len(assets) != len(referenced) {
		return manifest, snapshot, nil, errors.New("配置包包含未被配置引用的图片")
	}
	return manifest, snapshot, assets, nil
}

func writeImportedAssets(root string, assets map[string][]byte) error {
	for relative, data := range assets {
		target := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}
func safeProfileFilename(name string) string {
	value := strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\\|?*`, r) || r < 32 {
			return '-'
		}
		return r
	}, strings.TrimSpace(name))
	if value == "" {
		return "qqpet-config"
	}
	return value
}

func snapshotAssetPaths(snapshot appconfig.ConfigSnapshot) []string {
	set := map[string]bool{}
	add := func(values ...string) {
		for _, value := range values {
			value = filepath.ToSlash(strings.TrimSpace(value))
			value = strings.TrimPrefix(value, "图片/")
			if value != "" && !strings.Contains(value, "://") && !filepath.IsAbs(value) && !strings.Contains(value, "..") {
				set[value] = true
			}
		}
	}
	for _, r := range snapshot.PetSpecies {
		add(r.Image, r.AdoptImage, r.TrainStartImg, r.TrainEndImg, r.StudyStartImg, r.StudyEndImg, r.FitnessStartImg, r.FitnessEndImg, r.EvolutionImage, r.AwakenImage)
	}
	for _, r := range snapshot.Items {
		add(r.Image)
	}
	for _, r := range snapshot.ShopItems {
		add(r.Image)
	}
	for _, r := range snapshot.CheckinRewards {
		add(r.Image)
	}
	for _, r := range snapshot.WorkSettings {
		add(r.StartImage, r.EndImage)
	}
	for _, r := range snapshot.Menus {
		add(r.Image)
	}
	for _, r := range snapshot.Images {
		add(r.Path)
	}
	for _, r := range snapshot.ExpeditionTemplates {
		add(r.StartImage, r.EndImage)
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func rewriteSnapshotAssets(snapshot *appconfig.ConfigSnapshot, assets map[string][]byte, prefix string) {
	rewrite := func(value *string) {
		normalized := filepath.ToSlash(strings.TrimPrefix(strings.TrimSpace(*value), "图片/"))
		if _, ok := assets[normalized]; ok {
			*value = prefix + "/" + normalized
		}
	}
	for i := range snapshot.PetSpecies {
		r := &snapshot.PetSpecies[i]
		rewrite(&r.Image)
		rewrite(&r.AdoptImage)
		rewrite(&r.TrainStartImg)
		rewrite(&r.TrainEndImg)
		rewrite(&r.StudyStartImg)
		rewrite(&r.StudyEndImg)
		rewrite(&r.FitnessStartImg)
		rewrite(&r.FitnessEndImg)
		rewrite(&r.EvolutionImage)
		rewrite(&r.AwakenImage)
	}
	for i := range snapshot.Menus {
		rewrite(&snapshot.Menus[i].Image)
	}
	for i := range snapshot.Items {
		rewrite(&snapshot.Items[i].Image)
	}
	for i := range snapshot.ShopItems {
		rewrite(&snapshot.ShopItems[i].Image)
	}
	for i := range snapshot.CheckinRewards {
		rewrite(&snapshot.CheckinRewards[i].Image)
	}
	for i := range snapshot.WorkSettings {
		rewrite(&snapshot.WorkSettings[i].StartImage)
		rewrite(&snapshot.WorkSettings[i].EndImage)
	}
	for i := range snapshot.Images {
		rewrite(&snapshot.Images[i].Path)
	}
	for i := range snapshot.ExpeditionTemplates {
		rewrite(&snapshot.ExpeditionTemplates[i].StartImage)
		rewrite(&snapshot.ExpeditionTemplates[i].EndImage)
	}
}
