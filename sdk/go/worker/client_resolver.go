package worker

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"githook/pkg/auth"
	"githook/pkg/providers/bitbucket"
	"githook/pkg/providers/gitlab"
)

const defaultInstallationsBaseURL = "http://localhost:8080"

// ResolveProviderClient returns an authenticated provider client for the event.
// It uses the API base URL from GITHOOK_API_BASE_URL or defaults to localhost.
func ResolveProviderClient(ctx context.Context, evt *Event) (interface{}, error) {
	return ResolveProviderClientWithClient(ctx, evt, &InstallationsClient{BaseURL: installationsBaseURL()})
}

// ResolveProviderClientWithClient returns an authenticated provider client for the event.
// GitHub uses the injected client on the event. GitLab/Bitbucket fetch tokens via the InstallationsClient.
func ResolveProviderClientWithClient(ctx context.Context, evt *Event, client *InstallationsClient) (interface{}, error) {
	if evt == nil {
		return nil, errors.New("event is required")
	}

	switch evt.Provider {
	case "github":
		gh, ok := GitHubClient(evt)
		if !ok {
			return nil, errors.New("github client not available on event")
		}
		return gh, nil
	case "gitlab":
		record, err := ResolveInstallation(ctx, evt, client)
		if err != nil {
			return nil, err
		}
		if record == nil || record.AccessToken == "" {
			return nil, errors.New("gitlab access token missing")
		}
		cfg, _ := providerConfigFromEnv("gitlab")
		return gitlab.NewTokenClient(cfg, record.AccessToken)
	case "bitbucket":
		record, err := ResolveInstallation(ctx, evt, client)
		if err != nil {
			return nil, err
		}
		if record == nil || record.AccessToken == "" {
			return nil, errors.New("bitbucket access token missing")
		}
		cfg, _ := providerConfigFromEnv("bitbucket")
		return bitbucket.NewTokenClient(cfg, record.AccessToken)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", evt.Provider)
	}
}

func serverBaseURL(cfg ServerConfig) string {
	if strings.TrimSpace(cfg.Endpoint) != "" {
		return strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/")
	}
	if strings.TrimSpace(cfg.PublicBaseURL) != "" {
		return strings.TrimRight(strings.TrimSpace(cfg.PublicBaseURL), "/")
	}
	port := cfg.Port
	if port == 0 {
		return ""
	}
	return "http://localhost:" + strconv.Itoa(port)
}

func providerConfigFromEnv(provider string) (auth.ProviderConfig, bool) {
	configPath := configPathFromEnv()
	if configPath == "" {
		return auth.ProviderConfig{}, false
	}
	cfg, err := LoadProvidersConfig(configPath)
	if err != nil {
		return auth.ProviderConfig{}, false
	}
	switch provider {
	case "github":
		return cfg.GitHub, true
	case "gitlab":
		return cfg.GitLab, true
	case "bitbucket":
		return cfg.Bitbucket, true
	default:
		return auth.ProviderConfig{}, false
	}
}
