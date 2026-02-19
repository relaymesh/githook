package api

import (
	"strings"

	"githook/pkg/auth"
)

func enabledProvidersList(cfg auth.Config) []string {
	out := make([]string, 0, 3)
	if cfg.GitHub.Enabled {
		out = append(out, "github")
	}
	if cfg.GitLab.Enabled {
		out = append(out, "gitlab")
	}
	if cfg.Bitbucket.Enabled {
		out = append(out, "bitbucket")
	}
	return out
}

func providerConfigFromAuthConfig(cfg auth.Config, provider string) auth.ProviderConfig {
	switch strings.TrimSpace(provider) {
	case "gitlab":
		return cfg.GitLab
	case "bitbucket":
		return cfg.Bitbucket
	default:
		return cfg.GitHub
	}
}

func providerEnabled(provider string, enabled []string) bool {
	for _, item := range enabled {
		if item == provider {
			return true
		}
	}
	return false
}

func providerNotEnabledMessage(provider string, enabled []string) string {
	if len(enabled) == 0 {
		return "provider not enabled (no providers enabled)"
	}
	return "provider not enabled; enabled=" + strings.Join(enabled, ",")
}
