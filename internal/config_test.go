package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.yaml")
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write app config: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.AppConfig.Server.Port != 8080 {
		t.Fatalf("expected default port 8080, got %d", cfg.AppConfig.Server.Port)
	}
	if cfg.AppConfig.Providers.GitHub.Path != "/webhooks/github" {
		t.Fatalf("expected default github path, got %q", cfg.AppConfig.Providers.GitHub.Path)
	}
	if cfg.AppConfig.Watermill.Driver != "gochannel" {
		t.Fatalf("expected default watermill driver, got %q", cfg.AppConfig.Watermill.Driver)
	}
	if len(cfg.AppConfig.Watermill.Drivers) != 0 {
		t.Fatalf("expected no default drivers, got %v", cfg.AppConfig.Watermill.Drivers)
	}
	if cfg.AppConfig.Watermill.GoChannel.OutputChannelBuffer != 64 {
		t.Fatalf("expected default gochannel output buffer, got %d", cfg.AppConfig.Watermill.GoChannel.OutputChannelBuffer)
	}
	if cfg.AppConfig.Watermill.HTTP.Mode != "topic_url" {
		t.Fatalf("expected default http mode topic_url, got %q", cfg.AppConfig.Watermill.HTTP.Mode)
	}
}

func TestLoadConfigInvalidRule(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "rules:\n  - when: action == \"opened\"\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write rules config: %v", err)
	}

	if _, err := LoadConfig(path); err == nil {
		t.Fatalf("expected error for missing emit")
	}
}

func TestLoadConfigTrimsFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "rules:\n  - when: \"  action == \\\"opened\\\"  \"\n    emit: \"  pr.opened.ready  \"\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write rules config: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load rules config: %v", err)
	}
	if cfg.Rules[0].When != "action == \"opened\"" {
		t.Fatalf("expected trimmed when, got %q", cfg.Rules[0].When)
	}
	if cfg.Rules[0].Emit != "pr.opened.ready" {
		t.Fatalf("expected trimmed emit, got %q", cfg.Rules[0].Emit)
	}
}
