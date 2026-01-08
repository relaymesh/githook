package worker

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestEmitListUnmarshalScalar(t *testing.T) {
	var cfg struct {
		Emit emitList `yaml:"emit"`
	}
	if err := yaml.Unmarshal([]byte("emit: pr.opened\n"), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(cfg.Emit) != 1 || cfg.Emit[0] != "pr.opened" {
		t.Fatalf("unexpected emit list: %v", cfg.Emit)
	}
}

func TestEmitListUnmarshalSequence(t *testing.T) {
	var cfg struct {
		Emit emitList `yaml:"emit"`
	}
	if err := yaml.Unmarshal([]byte("emit: [pr.opened, pr.merged]\n"), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(cfg.Emit) != 2 {
		t.Fatalf("unexpected emit list: %v", cfg.Emit)
	}
}

func TestEmitListUnmarshalInvalid(t *testing.T) {
	var cfg struct {
		Emit emitList `yaml:"emit"`
	}
	if err := yaml.Unmarshal([]byte("emit:\n  key: value\n"), &cfg); err == nil {
		t.Fatalf("expected error for invalid emit value")
	}
}
