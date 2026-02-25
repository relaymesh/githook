package bitbucket

import (
	"testing"

	"github.com/relaymesh/githook/pkg/auth"
)

func TestNormalizeBaseURL(t *testing.T) {
	if got := normalizeBaseURL(" https://api.bitbucket.org/2.0/ "); got != "https://api.bitbucket.org/2.0" {
		t.Fatalf("unexpected base url: %q", got)
	}
	if got := normalizeBaseURL(" "); got != "" {
		t.Fatalf("expected empty base url, got %q", got)
	}
}

func TestNewTokenClientRequiresToken(t *testing.T) {
	if _, err := NewTokenClient(auth.ProviderConfig{}, ""); err == nil {
		t.Fatalf("expected token required error")
	}
}
