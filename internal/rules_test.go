package internal

import "testing"

func TestRuleEngineEvaluate(t *testing.T) {
	cfg := RulesConfig{
		Rules: []Rule{
			{When: "action == \"opened\"", Emit: "pr.opened"},
			{When: "action == \"closed\" && merged == true", Emit: "pr.merged"},
		},
	}

	engine, err := NewRuleEngine(cfg)
	if err != nil {
		t.Fatalf("new rule engine: %v", err)
	}

	event := Event{
		Provider: "github",
		Name:     "pull_request",
		Data: map[string]interface{}{
			"action": "opened",
			"merged": false,
		},
	}

	matches := engine.Evaluate(event)
	if len(matches) != 1 {
		t.Fatalf("expected 1 topic, got %d", len(matches))
	}
	if matches[0].Topic != "pr.opened" {
		t.Fatalf("expected topic pr.opened, got %q", matches[0].Topic)
	}
}

func TestRuleEngineEvaluateMissingField(t *testing.T) {
	cfg := RulesConfig{
		Rules: []Rule{
			{When: "missing == true", Emit: "never"},
		},
	}

	engine, err := NewRuleEngine(cfg)
	if err != nil {
		t.Fatalf("new rule engine: %v", err)
	}

	event := Event{
		Provider: "github",
		Name:     "push",
		Data:     map[string]interface{}{},
	}

	matches := engine.Evaluate(event)
	if len(matches) != 0 {
		t.Fatalf("expected no topics, got %d", len(matches))
	}
}

func TestRuleEngineWithDrivers(t *testing.T) {
	cfg := RulesConfig{
		Rules: []Rule{
			{When: "action == \"opened\"", Emit: "pr.opened", Drivers: []string{"amqp", "http"}},
		},
	}

	engine, err := NewRuleEngine(cfg)
	if err != nil {
		t.Fatalf("new rule engine: %v", err)
	}

	event := Event{
		Provider: "github",
		Name:     "pull_request",
		Data: map[string]interface{}{
			"action": "opened",
		},
	}

	matches := engine.Evaluate(event)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if len(matches[0].Drivers) != 2 {
		t.Fatalf("expected 2 drivers, got %d", len(matches[0].Drivers))
	}
}
