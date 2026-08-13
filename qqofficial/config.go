package qqofficial

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"qq-pet-saas/security"
)

type Config struct {
	AppID           string
	Secret          string
	APIBase         string
	TokenURL        string
	Intents         int
	ShardCount      int
	MarkdownEnabled bool
	KeyboardEnabled bool
}

func LoadConfig() (Config, bool, error) {
	runtimeConfig, err := security.LoadRuntimeConfig()
	if err != nil {
		return Config{}, false, err
	}
	stored := runtimeConfig.QQOfficial
	config := Config{
		AppID: stored.AppID, Secret: stored.AppSecret, APIBase: stored.APIBase, TokenURL: stored.TokenURL,
		ShardCount:      stored.ShardCount,
		MarkdownEnabled: stored.MarkdownEnabled, KeyboardEnabled: stored.KeyboardEnabled,
	}
	if stored.GroupEventsEnabled {
		config.Intents |= IntentsGroupAndC2C
	}
	if stored.GuildEventsEnabled {
		config.Intents |= IntentsPublicGuildMessages
	}
	if config.AppID == "" && config.Secret == "" {
		return withConfigDefaults(config), false, nil
	}
	if config.AppID == "" || config.Secret == "" {
		return config, false, fmt.Errorf("QQBOT_APP_ID and QQBOT_APP_SECRET must be configured together")
	}
	if stored.InteractionEnabled {
		config.Intents |= IntentsInteraction
	}
	if stored.AuditEnabled {
		config.Intents |= IntentsMessageAudit
	}
	return withConfigDefaults(config), true, nil
}

func LoadConfigFromEnv() (Config, bool, error) {
	config := Config{
		AppID: strings.TrimSpace(os.Getenv("QQBOT_APP_ID")), Secret: strings.TrimSpace(os.Getenv("QQBOT_APP_SECRET")),
		APIBase: strings.TrimSpace(os.Getenv("QQBOT_API_BASE")), TokenURL: strings.TrimSpace(os.Getenv("QQBOT_TOKEN_URL")),
		Intents:         IntentsGroupAndC2C | IntentsPublicGuildMessages,
		MarkdownEnabled: envBool("QQBOT_MARKDOWN_ENABLED"), KeyboardEnabled: envBool("QQBOT_KEYBOARD_ENABLED"),
	}
	if config.AppID == "" && config.Secret == "" {
		return config, false, nil
	}
	if config.AppID == "" || config.Secret == "" {
		return config, false, fmt.Errorf("QQBOT_APP_ID 和 QQBOT_APP_SECRET 必须同时配置")
	}
	if envBool("QQBOT_INTERACTION_ENABLED") {
		config.Intents |= IntentsInteraction
	}
	if envBool("QQBOT_AUDIT_ENABLED") {
		config.Intents |= IntentsMessageAudit
	}
	if raw := strings.TrimSpace(os.Getenv("QQBOT_SHARD_COUNT")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 {
			return config, false, fmt.Errorf("QQBOT_SHARD_COUNT 必须是正整数")
		}
		config.ShardCount = value
	}
	if config.APIBase == "" {
		config.APIBase = DefaultAPIBase
	}
	if config.TokenURL == "" {
		config.TokenURL = DefaultTokenURL
	}
	return config, true, nil
}

func withConfigDefaults(config Config) Config {
	if config.APIBase == "" {
		config.APIBase = DefaultAPIBase
	}
	if config.TokenURL == "" {
		config.TokenURL = DefaultTokenURL
	}
	return config
}

func envBool(name string) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}
