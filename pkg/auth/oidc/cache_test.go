package oidc

import (
	"path/filepath"
	"testing"
	"time"

	"githook/pkg/auth"
)

func TestCacheKey(t *testing.T) {
	cfg := auth.OAuth2Config{
		Issuer:   " https://issuer ",
		Audience: " api ",
		ClientID: " client ",
	}
	if key := CacheKey(cfg); key != "https://issuer|api|client" {
		t.Fatalf("unexpected cache key: %q", key)
	}
}

func TestStoreLoadCachedToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token.json")
	key := "cache-key"
	expiresAt := time.Now().Add(30 * time.Minute)

	if err := StoreCachedToken(path, key, "token-value", expiresAt); err != nil {
		t.Fatalf("store cached token: %v", err)
	}

	token, expiry, ok, err := LoadCachedToken(path, key)
	if err != nil {
		t.Fatalf("load cached token: %v", err)
	}
	if !ok {
		t.Fatalf("expected cached token")
	}
	if token != "token-value" {
		t.Fatalf("expected token-value, got %q", token)
	}
	if expiry.IsZero() {
		t.Fatalf("expected expiry to be set")
	}
}

func TestLoadCachedTokenExpired(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token.json")
	key := "cache-key"
	expiresAt := time.Now().Add(-1 * time.Minute)

	if err := StoreCachedToken(path, key, "token-value", expiresAt); err != nil {
		t.Fatalf("store cached token: %v", err)
	}

	token, _, ok, err := LoadCachedToken(path, key)
	if err != nil {
		t.Fatalf("load cached token: %v", err)
	}
	if ok || token != "" {
		t.Fatalf("expected expired token to be cleared")
	}
}

func TestLoadCachedTokenPathIsDirectory(t *testing.T) {
	dir := t.TempDir()
	if _, _, _, err := LoadCachedToken(dir, "key"); err == nil {
		t.Fatalf("expected error for directory cache path")
	}
}
