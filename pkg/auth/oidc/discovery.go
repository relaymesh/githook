package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Discovery struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
}

type cacheEntry struct {
	value      Discovery
	expiresAt  time.Time
	retryAfter time.Time
}

var (
	discoveryCache   = make(map[string]cacheEntry)
	discoveryCacheMu sync.Mutex
)

func Discover(ctx context.Context, issuer string) (Discovery, error) {
	issuer = strings.TrimRight(strings.TrimSpace(issuer), "/")
	if issuer == "" {
		return Discovery{}, errors.New("issuer is required")
	}
	var stale *cacheEntry
	discoveryCacheMu.Lock()
	if entry, ok := discoveryCache[issuer]; ok {
		if time.Now().Before(entry.expiresAt) {
			discoveryCacheMu.Unlock()
			return entry.value, nil
		}
		if time.Now().Before(entry.retryAfter) {
			stale = &entry
			discoveryCacheMu.Unlock()
			return entry.value, nil
		}
		stale = &entry
	}
	discoveryCacheMu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, issuer+"/.well-known/openid-configuration", nil)
	if err != nil {
		return Discovery{}, err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		if stale != nil {
			discoveryCacheMu.Lock()
			stale.retryAfter = time.Now().Add(2 * time.Minute)
			discoveryCache[issuer] = *stale
			discoveryCacheMu.Unlock()
			return stale.value, nil
		}
		return Discovery{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if stale != nil {
			discoveryCacheMu.Lock()
			stale.retryAfter = time.Now().Add(2 * time.Minute)
			discoveryCache[issuer] = *stale
			discoveryCacheMu.Unlock()
			return stale.value, nil
		}
		return Discovery{}, errors.New("oidc discovery failed: " + resp.Status)
	}

	var payload Discovery
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		if stale != nil {
			return stale.value, nil
		}
		return Discovery{}, err
	}
	discoveryCacheMu.Lock()
	jitter := time.Duration(rand.Intn(60)) * time.Second
	discoveryCache[issuer] = cacheEntry{
		value:      payload,
		expiresAt:  time.Now().Add(5*time.Minute + jitter),
		retryAfter: time.Now().Add(1 * time.Minute),
	}
	discoveryCacheMu.Unlock()
	return payload, nil
}
