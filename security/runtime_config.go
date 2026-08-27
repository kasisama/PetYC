package security

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const (
	runtimeConfigFileName = "runtime.json"
	defaultRuntimePort    = 8080
)

type QQOfficialRuntimeConfig struct {
	AppID              string `json:"app_id"`
	AppSecret          string `json:"app_secret"`
	APIBase            string `json:"api_base,omitempty"`
	TokenURL           string `json:"token_url,omitempty"`
	ShardCount         int    `json:"shard_count,omitempty"`
	MarkdownEnabled    bool   `json:"markdown_enabled"`
	KeyboardEnabled    bool   `json:"keyboard_enabled"`
	InteractionEnabled bool   `json:"interaction_enabled"`
	AuditEnabled       bool   `json:"audit_enabled"`
	GroupEventsEnabled bool   `json:"group_events_enabled"`
	GuildEventsEnabled bool   `json:"guild_events_enabled"`
}

type RuntimeConfig struct {
	ListenAddress string                  `json:"listen_address"`
	Port          int                     `json:"port"`
	OneBotToken   string                  `json:"onebot_token"`
	QQOfficial    QQOfficialRuntimeConfig `json:"qq_official"`
}

var runtimeConfigMu sync.Mutex

func RuntimeConfigPath() string {
	dir, err := dataDir()
	if err != nil {
		return runtimeConfigFileName
	}
	return filepath.Join(dir, runtimeConfigFileName)
}

// WebSetupEnabled reports whether the explicitly opted-in browser-based
// first-run flow may be used from a non-loopback client, such as a browser
// reaching the service through a Docker port mapping.
func WebSetupEnabled() bool {
	return envRuntimeBool("QQPET_WEB_SETUP")
}

func LoadRuntimeConfig() (RuntimeConfig, error) {
	runtimeConfigMu.Lock()
	defer runtimeConfigMu.Unlock()

	dir, err := dataDir()
	if err != nil {
		return RuntimeConfig{}, fmt.Errorf("resolve application data directory: %w", err)
	}
	path := filepath.Join(dir, runtimeConfigFileName)
	raw, err := os.ReadFile(path)
	if err == nil {
		var config RuntimeConfig
		if err := json.Unmarshal(raw, &config); err != nil {
			return RuntimeConfig{}, fmt.Errorf("read runtime config: %w", err)
		}
		applyLegacyRuntimeDefaults(raw, &config)
		if err := validateRuntimeConfig(config); err != nil {
			return RuntimeConfig{}, err
		}
		return config, nil
	}
	if !os.IsNotExist(err) {
		return RuntimeConfig{}, fmt.Errorf("read runtime config: %w", err)
	}

	config, err := runtimeConfigFromEnv()
	if err != nil {
		return RuntimeConfig{}, err
	}
	if err := saveRuntimeConfigLocked(dir, path, config); err != nil {
		return RuntimeConfig{}, err
	}
	return config, nil
}

func SaveRuntimeConfig(config RuntimeConfig) error {
	if err := validateRuntimeConfig(config); err != nil {
		return err
	}
	runtimeConfigMu.Lock()
	defer runtimeConfigMu.Unlock()
	dir, err := dataDir()
	if err != nil {
		return fmt.Errorf("resolve application data directory: %w", err)
	}
	return saveRuntimeConfigLocked(dir, filepath.Join(dir, runtimeConfigFileName), config)
}

func runtimeConfigFromEnv() (RuntimeConfig, error) {
	port := defaultRuntimePort
	if raw := strings.TrimSpace(os.Getenv("PORT")); raw != "" {
		raw = strings.TrimPrefix(raw, ":")
		value, err := strconv.Atoi(raw)
		if err != nil {
			return RuntimeConfig{}, fmt.Errorf("PORT must be a valid port")
		}
		port = value
	}
	config := RuntimeConfig{
		ListenAddress: strings.TrimSpace(os.Getenv("LISTEN_ADDRESS")),
		Port:          port,
		OneBotToken:   strings.TrimSpace(os.Getenv("QQPET_WS_TOKEN")),
		QQOfficial: QQOfficialRuntimeConfig{
			AppID: strings.TrimSpace(os.Getenv("QQBOT_APP_ID")), AppSecret: strings.TrimSpace(os.Getenv("QQBOT_APP_SECRET")),
			APIBase: strings.TrimSpace(os.Getenv("QQBOT_API_BASE")), TokenURL: strings.TrimSpace(os.Getenv("QQBOT_TOKEN_URL")),
			MarkdownEnabled: envRuntimeBool("QQBOT_MARKDOWN_ENABLED"), KeyboardEnabled: envRuntimeBool("QQBOT_KEYBOARD_ENABLED"),
			InteractionEnabled: envRuntimeBool("QQBOT_INTERACTION_ENABLED"), AuditEnabled: envRuntimeBool("QQBOT_AUDIT_ENABLED"),
			GroupEventsEnabled: envRuntimeBoolDefault("QQBOT_GROUP_EVENTS_ENABLED", true), GuildEventsEnabled: envRuntimeBoolDefault("QQBOT_GUILD_EVENTS_ENABLED", true),
		},
	}
	if config.ListenAddress == "" {
		config.ListenAddress = "127.0.0.1"
	}
	if raw := strings.TrimSpace(os.Getenv("QQBOT_SHARD_COUNT")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 {
			return RuntimeConfig{}, fmt.Errorf("QQBOT_SHARD_COUNT must be a positive integer")
		}
		config.QQOfficial.ShardCount = value
	}
	if config.OneBotToken == "" {
		token, err := generateToken()
		if err != nil {
			return RuntimeConfig{}, err
		}
		config.OneBotToken = token
	}
	if err := validateRuntimeConfig(config); err != nil {
		return RuntimeConfig{}, err
	}
	return config, nil
}

func validateRuntimeConfig(config RuntimeConfig) error {
	if config.ListenAddress != "localhost" && net.ParseIP(config.ListenAddress) == nil {
		return fmt.Errorf("listen address must be localhost or a valid IP address")
	}
	if config.Port < 1 || config.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	if config.OneBotToken == "" {
		return fmt.Errorf("onebot token cannot be empty")
	}
	if (config.QQOfficial.AppID == "") != (config.QQOfficial.AppSecret == "") {
		return fmt.Errorf("QQ app ID and app secret must be configured together")
	}
	if config.QQOfficial.ShardCount < 0 {
		return fmt.Errorf("QQ shard count cannot be negative")
	}
	return nil
}

func applyLegacyRuntimeDefaults(raw []byte, config *RuntimeConfig) {
	if strings.TrimSpace(config.ListenAddress) == "" {
		config.ListenAddress = "127.0.0.1"
	}
	var document struct {
		QQOfficial map[string]json.RawMessage `json:"qq_official"`
	}
	if json.Unmarshal(raw, &document) != nil {
		return
	}
	if _, exists := document.QQOfficial["group_events_enabled"]; !exists {
		config.QQOfficial.GroupEventsEnabled = true
	}
	if _, exists := document.QQOfficial["guild_events_enabled"]; !exists {
		config.QQOfficial.GuildEventsEnabled = true
	}
}

func saveRuntimeConfigLocked(dir, path string, config RuntimeConfig) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create application data directory: %w", err)
	}
	payload, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("encode runtime config: %w", err)
	}
	temp, err := os.CreateTemp(dir, ".runtime-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary runtime config: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0600); err != nil {
		temp.Close()
		return fmt.Errorf("secure temporary runtime config: %w", err)
	}
	if _, err := temp.Write(payload); err != nil {
		temp.Close()
		return fmt.Errorf("write temporary runtime config: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync temporary runtime config: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary runtime config: %w", err)
	}
	if err := replaceFile(tempPath, path); err != nil {
		return fmt.Errorf("replace runtime config: %w", err)
	}
	return syncOneBotCredential(config.OneBotToken)
}

func replaceFile(source, target string) error {
	return os.Rename(source, target)
}

func syncOneBotCredential(token string) error {
	credentialsMu.Lock()
	defer credentialsMu.Unlock()
	credentials, err := loadCredentialsLocked()
	if err != nil {
		return err
	}
	if credentials.WebSocketToken == token {
		return nil
	}
	credentials.WebSocketToken = token
	dir, err := dataDir()
	if err != nil {
		return err
	}
	return writeCredentialsFile(dir, filepath.Join(dir, credentialsFileName), credentials)
}

func envRuntimeBool(name string) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func envRuntimeBoolDefault(name string, fallback bool) bool {
	if strings.TrimSpace(os.Getenv(name)) == "" {
		return fallback
	}
	return envRuntimeBool(name)
}
