package githook

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/validate"

	"githook/pkg/auth/oidc"
	"githook/pkg/core"
)

func connectClientOptions() ([]connect.ClientOption, error) {
	interceptor := validate.NewInterceptor()
	opts := []connect.ClientOption{
		connect.WithInterceptors(interceptor),
	}
	cfg, err := loadCLIConfig()
	if err != nil {
		return opts, err
	}
	apiBaseURL = resolveEndpoint(cfg)
	if !cfg.Auth.OAuth2.Enabled {
		return opts, nil
	}
	token, err := cliToken(context.Background(), cfg)
	if err != nil {
		return opts, err
	}
	if token != "" {
		opts = append(opts, connect.WithInterceptors(authHeaderInterceptor(token)))
	}
	return opts, nil
}

func authHeaderInterceptor(token string) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if token != "" {
				req.Header().Set("Authorization", "Bearer "+token)
			}
			return next(ctx, req)
		}
	}
}

func loadCLIConfig() (core.AppConfig, error) {
	if strings.TrimSpace(configPath) == "" {
		return core.AppConfig{}, errors.New("config path not set")
	}
	return core.LoadAppConfig(configPath)
}

func resolveEndpoint(cfg core.AppConfig) string {
	if strings.TrimSpace(apiBaseURL) != "" {
		return strings.TrimSpace(apiBaseURL)
	}
	if strings.TrimSpace(cfg.Endpoint) != "" {
		return strings.TrimSpace(cfg.Endpoint)
	}
	if strings.TrimSpace(cfg.Server.PublicBaseURL) != "" {
		return strings.TrimSpace(cfg.Server.PublicBaseURL)
	}
	return "http://localhost:8080"
}

func cliToken(ctx context.Context, cfg core.AppConfig) (string, error) {
	if token := strings.TrimSpace(os.Getenv("GITHOOK_AUTH_TOKEN")); token != "" {
		return token, nil
	}
	cachePath := tokenCachePath()
	cacheKey := oidc.CacheKey(cfg.Auth.OAuth2)
	if cachePath != "" {
		token, expiresAt, ok, err := oidc.LoadCachedToken(cachePath, cacheKey)
		if err == nil && ok && token != "" && time.Now().Before(expiresAt.Add(-30*time.Second)) {
			return token, nil
		}
	}
	if strings.ToLower(strings.TrimSpace(cfg.Auth.OAuth2.Mode)) == "auth_code" && cfg.Auth.OAuth2.ClientSecret == "" {
		return "", errors.New("auth_code mode requires login; set GITHOOK_AUTH_TOKEN or store a token with githook auth store")
	}
	resp, err := oidc.ClientCredentialsToken(ctx, cfg.Auth.OAuth2)
	if err != nil {
		return "", err
	}
	if cachePath != "" {
		expiresAt := time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second)
		if resp.ExpiresIn == 0 {
			expiresAt = time.Now().Add(30 * time.Minute)
		}
		_ = oidc.StoreCachedToken(cachePath, cacheKey, resp.AccessToken, expiresAt)
	}
	return resp.AccessToken, nil
}

func tokenCachePath() string {
	if path := strings.TrimSpace(os.Getenv("GITHOOK_TOKEN_CACHE")); path != "" {
		return path
	}
	path, err := oidc.DefaultCachePath()
	if err != nil {
		return ""
	}
	return path
}
