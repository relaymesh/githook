package worker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallationsBaseURLFromEnv(t *testing.T) {
	t.Setenv("GITHOOK_API_BASE_URL", "http://example.com")
	t.Setenv("GITHOOK_CONFIG_PATH", "")
	t.Setenv("GITHOOK_CONFIG", "")

	if got := installationsBaseURL(); got != "http://example.com" {
		t.Fatalf("expected env base url, got %q", got)
	}
}

func TestInstallationsBaseURLFromConfigPublicBaseURL(t *testing.T) {
	t.Setenv("GITHOOK_API_BASE_URL", "")
	t.Setenv("GITHOOK_CONFIG", "")

	dir := t.TempDir()
	path := filepath.Join(dir, "app.yaml")
	content := `
server:
  public_base_url: https://example.com/base/
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("GITHOOK_CONFIG_PATH", path)

	if got := installationsBaseURL(); got != "https://example.com/base" {
		t.Fatalf("expected public base url, got %q", got)
	}
}

func TestInstallationsBaseURLFromConfigEndpoint(t *testing.T) {
	t.Setenv("GITHOOK_API_BASE_URL", "")
	t.Setenv("GITHOOK_CONFIG", "")

	dir := t.TempDir()
	path := filepath.Join(dir, "app.yaml")
	content := `
endpoint: https://api.example.com/base/
server:
  public_base_url: https://example.com/ignored/
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("GITHOOK_CONFIG_PATH", path)

	if got := installationsBaseURL(); got != "https://api.example.com/base" {
		t.Fatalf("expected endpoint base url, got %q", got)
	}
}

func TestInstallationsBaseURLFromConfigPort(t *testing.T) {
	t.Setenv("GITHOOK_API_BASE_URL", "")
	t.Setenv("GITHOOK_CONFIG", "")

	dir := t.TempDir()
	path := filepath.Join(dir, "app.yaml")
	content := `
server:
  port: 9090
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("GITHOOK_CONFIG_PATH", path)

	if got := installationsBaseURL(); got != "http://localhost:9090" {
		t.Fatalf("expected localhost base url, got %q", got)
	}
}

func TestInstallationsBaseURLDefault(t *testing.T) {
	t.Setenv("GITHOOK_API_BASE_URL", "")
	t.Setenv("GITHOOK_CONFIG_PATH", "")
	t.Setenv("GITHOOK_CONFIG", "")

	if got := installationsBaseURL(); got != defaultInstallationsBaseURL {
		t.Fatalf("expected default base url, got %q", got)
	}
}
