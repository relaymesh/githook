package gitlab

import (
	"testing"

	"github.com/relaymesh/githook/pkg/auth"
)

func TestNormalizeBaseURL(t *testing.T) {
	if got := normalizeBaseURL(""); got != "https://gitlab.com/api/v4" {
		t.Fatalf("expected default base url, got %q", got)
	}
	if got := normalizeBaseURL("https://gitlab.example.com/api/v4/"); got != "https://gitlab.example.com/api/v4" {
		t.Fatalf("unexpected base url: %q", got)
	}
}

func TestNewTokenClientRequiresToken(t *testing.T) {
	if _, err := NewTokenClient(auth.ProviderConfig{}, ""); err == nil {
		t.Fatalf("expected token required error")
	}
}
