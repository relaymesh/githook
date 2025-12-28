package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAppConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.yaml")
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write app config: %v", err)
	}

	cfg, err := LoadAppConfig(path)
	if err != nil {
		t.Fatalf("load app config: %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Fatalf("expected default port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Providers.GitHub.Path != "/webhooks/github" {
		t.Fatalf("expected default github path, got %q", cfg.Providers.GitHub.Path)
	}
	if cfg.Watermill.Driver != "gochannel" {
		t.Fatalf("expected default watermill driver, got %q", cfg.Watermill.Driver)
	}
	if len(cfg.Watermill.Drivers) != 0 {
		t.Fatalf("expected no default drivers, got %v", cfg.Watermill.Drivers)
	}
	if cfg.Watermill.GoChannel.OutputChannelBuffer != 64 {
		t.Fatalf("expected default gochannel output buffer, got %d", cfg.Watermill.GoChannel.OutputChannelBuffer)
	}
	if cfg.Watermill.HTTP.Mode != "topic_url" {
		t.Fatalf("expected default http mode topic_url, got %q", cfg.Watermill.HTTP.Mode)
	}
}

func TestLoadRulesConfigInvalidRule(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "rules:\n  - when: action == \"opened\"\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write rules config: %v", err)
	}

	if _, err := LoadRulesConfig(path); err == nil {
		t.Fatalf("expected error for missing emit")
	}
}

func TestLoadRulesConfigTrimsFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "rules:\n  - when: \"  action == \\\"opened\\\"  \"\n    emit: \"  pr.opened.ready  \"\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write rules config: %v", err)
	}

	cfg, err := LoadRulesConfig(path)
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
