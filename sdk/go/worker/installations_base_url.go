package worker

import (
	"os"
	"strings"
)

func installationsBaseURL() string {
	baseURL := strings.TrimSpace(os.Getenv("GITHOOKS_API_BASE_URL"))
	if baseURL != "" {
		return baseURL
	}
	configPath := configPathFromEnv()
	if configPath != "" {
		if cfg, err := LoadServerConfig(configPath); err == nil {
			if base := serverBaseURL(cfg); base != "" {
				return base
			}
		}
	}
	return defaultInstallationsBaseURL
}

func configPathFromEnv() string {
	configPath := strings.TrimSpace(os.Getenv("GITHOOKS_CONFIG_PATH"))
	if configPath == "" {
		configPath = strings.TrimSpace(os.Getenv("GITHOOKS_CONFIG"))
	}
	return configPath
}
