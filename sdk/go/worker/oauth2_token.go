package worker

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"

	"githook/pkg/auth"
	"githook/pkg/auth/oidc"
	"githook/pkg/core"
)

type tokenCache struct {
	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

var oauth2Cache tokenCache

func oauth2Token(ctx context.Context) (string, error) {
	cfg, ok, err := loadOAuth2Config()
	if err != nil || !ok {
		return "", err
	}
	if !cfg.Enabled {
		return "", nil
	}
	oauth2Cache.mu.Lock()
	if oauth2Cache.token != "" && time.Now().Before(oauth2Cache.expiresAt) {
		token := oauth2Cache.token
		oauth2Cache.mu.Unlock()
		return token, nil
	}
	oauth2Cache.mu.Unlock()

	cachePath := tokenCachePath()
	cacheKey := oidc.CacheKey(cfg)
	if cachePath != "" {
		token, expiresAt, ok, err := oidc.LoadCachedToken(cachePath, cacheKey)
		if err == nil && ok && token != "" && time.Now().Before(expiresAt.Add(-30*time.Second)) {
			return token, nil
		}
	}

	token, err := oidc.ClientCredentialsToken(ctx, cfg)
	if err != nil {
		return "", err
	}
	expiry := time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	if token.ExpiresIn == 0 {
		expiry = time.Now().Add(30 * time.Minute)
	}
	oauth2Cache.mu.Lock()
	oauth2Cache.token = token.AccessToken
	oauth2Cache.expiresAt = expiry
	oauth2Cache.mu.Unlock()
	if cachePath != "" {
		_ = oidc.StoreCachedToken(cachePath, cacheKey, token.AccessToken, expiry)
	}
	return token.AccessToken, nil
}

func loadOAuth2Config() (auth.OAuth2Config, bool, error) {
	configPath := configPathFromEnv()
	if strings.TrimSpace(configPath) == "" {
		return auth.OAuth2Config{}, false, nil
	}
	cfg, err := core.LoadAppConfig(configPath)
	if err != nil {
		return auth.OAuth2Config{}, false, err
	}
	return cfg.Auth.OAuth2, true, nil
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
