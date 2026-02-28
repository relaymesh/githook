package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAppConfigAndRulesConfig(t *testing.T) {
	dir := t.TempDir()
	appPath := filepath.Join(dir, "app.yaml")
	appYAML := "server:\n  public_base_url: https://example.test\noauth:\n  redirect_base_url: https://example.test/done\nauth:\n  oauth2:\n    enabled: true\n"
	if err := os.WriteFile(appPath, []byte(appYAML), 0o600); err != nil {
		t.Fatalf("write app yaml: %v", err)
	}

	appCfg, err := LoadAppConfig(appPath)
	if err != nil {
		t.Fatalf("load app config: %v", err)
	}
	if appCfg.Endpoint != "https://example.test" {
		t.Fatalf("expected endpoint from public_base_url, got %q", appCfg.Endpoint)
	}
	if appCfg.RedirectBaseURL != "https://example.test/done" {
		t.Fatalf("expected redirect base url fallback, got %q", appCfg.RedirectBaseURL)
	}
	if appCfg.Auth.OAuth2.RedirectURL == "" || appCfg.Auth.OAuth2.Mode == "" || len(appCfg.Auth.OAuth2.Scopes) == 0 {
		t.Fatalf("expected oauth2 defaults applied, got %+v", appCfg.Auth.OAuth2)
	}

	rulesPath := filepath.Join(dir, "rules.yaml")
	rulesYAML := "rules:\n  - when: action == \"opened\"\n    emit: [\"topic.a\", \"topic.b\"]\n    driver_id: amqp\nrules_strict: true\n"
	if err := os.WriteFile(rulesPath, []byte(rulesYAML), 0o600); err != nil {
		t.Fatalf("write rules yaml: %v", err)
	}

	rulesCfg, err := LoadRulesConfig(rulesPath)
	if err != nil {
		t.Fatalf("load rules config: %v", err)
	}
	if !rulesCfg.Strict || len(rulesCfg.Rules) != 1 || len(rulesCfg.Rules[0].Emit) != 2 {
		t.Fatalf("unexpected rules config: %+v", rulesCfg)
	}
}

func TestNormalizeRulesErrors(t *testing.T) {
	if _, err := NormalizeRules([]Rule{{When: "a==1", Emit: EmitList{"topic"}}}); err == nil {
		t.Fatalf("expected missing driver_id error")
	}
	if _, err := NormalizeRules([]Rule{{DriverID: "d1", Emit: EmitList{"topic"}}}); err == nil {
		t.Fatalf("expected missing when error")
	}
	if _, err := NormalizeRules([]Rule{{DriverID: "d1", When: "a==1", Emit: EmitList{}}}); err == nil {
		t.Fatalf("expected missing emit error")
	}
}
