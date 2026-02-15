package webhook

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"testing"

	"githook/pkg/storage"
)

func TestVerifyGitHubSHA1(t *testing.T) {
	secret := "secret"
	body := []byte("payload")
	expected := "sha1=" + hex.EncodeToString(hmacSHA1(secret, body))
	if !verifyGitHubSHA1(secret, body, expected) {
		t.Fatalf("expected signature to verify")
	}
	if verifyGitHubSHA1(secret, body, "sha1=bad") {
		t.Fatalf("expected signature to fail")
	}
}

func TestGithubNamespaceInfo(t *testing.T) {
	raw := []byte(`{"repository":{"id":123,"full_name":"org/repo","name":"repo","owner":{"login":"org"}}}`)
	id, name := githubNamespaceInfo(raw)
	if id != "123" || name != "org/repo" {
		t.Fatalf("unexpected namespace info: %q %q", id, name)
	}

	raw = []byte(`{"repository":{"id":0,"name":"repo","owner":{"login":"org"}}}`)
	id, name = githubNamespaceInfo(raw)
	if id != "" || name != "org/repo" {
		t.Fatalf("unexpected fallback namespace info: %q %q", id, name)
	}
}

func TestRecordHelpers(t *testing.T) {
	record := &storage.InstallRecord{TenantID: "tenant", AccountName: "acct", ProviderInstanceKey: "key"}
	if got := recordAccountName(record, "github"); got != "acct" {
		t.Fatalf("expected account name, got %q", got)
	}
	if got := recordTenantID(record); got != "tenant" {
		t.Fatalf("expected tenant id, got %q", got)
	}
	if got := recordInstanceKey(record); got != "key" {
		t.Fatalf("expected instance key, got %q", got)
	}
	if got := recordAccountName(nil, "github"); got != "github" {
		t.Fatalf("expected provider fallback, got %q", got)
	}
	if recordTenantID(nil) != "" || recordInstanceKey(nil) != "" {
		t.Fatalf("expected empty values for nil record")
	}
}

func hmacSHA1(secret string, body []byte) []byte {
	mac := hmac.New(sha1.New, []byte(secret))
	_, _ = mac.Write(body)
	return mac.Sum(nil)
}
