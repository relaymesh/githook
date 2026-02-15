package core

import "testing"

// TestRuleEngineEvaluate tests that the rule engine correctly evaluates a simple rule.
func TestRuleEngineEvaluate(t *testing.T) {
	cfg := RulesConfig{
		Rules: []Rule{
			{When: "action == \"opened\"", Emit: EmitList{"pr.opened"}},
			{When: "action == \"closed\" && merged == true", Emit: EmitList{"pr.merged"}},
		},
	}

	engine, err := NewRuleEngine(cfg)
	if err != nil {
		t.Fatalf("new rule engine: %v", err)
	}

	event := Event{
		Provider:   "github",
		Name:       "pull_request",
		RawPayload: []byte(`{"action":"opened","merged":false}`),
	}

	matches := engine.Evaluate(event)
	if len(matches) != 1 {
		t.Fatalf("expected 1 topic, got %d", len(matches))
	}
	if matches[0].Topic != "pr.opened" {
		t.Fatalf("expected topic pr.opened, got %q", matches[0].Topic)
	}
}

// TestRuleEngineEvaluateMissingField tests that the rule engine does not match a rule with a missing field.
func TestRuleEngineEvaluateMissingField(t *testing.T) {
	cfg := RulesConfig{
		Rules: []Rule{
			{When: "missing == true", Emit: EmitList{"never"}},
		},
	}

	engine, err := NewRuleEngine(cfg)
	if err != nil {
		t.Fatalf("new rule engine: %v", err)
	}

	event := Event{
		Provider:   "github",
		Name:       "push",
		RawPayload: []byte(`{}`),
	}

	matches := engine.Evaluate(event)
	if len(matches) != 0 {
		t.Fatalf("expected no topics, got %d", len(matches))
	}
}

// TestRuleEngineWithDrivers tests that the rule engine correctly handles a rule with drivers specified.
func TestRuleEngineWithDrivers(t *testing.T) {
	cfg := RulesConfig{
		Rules: []Rule{
			{When: "action == \"opened\"", Emit: EmitList{"pr.opened"}, DriverID: "amqp"},
		},
	}

	engine, err := NewRuleEngine(cfg)
	if err != nil {
		t.Fatalf("new rule engine: %v", err)
	}

	event := Event{
		Provider:   "github",
		Name:       "pull_request",
		RawPayload: []byte(`{"action":"opened"}`),
	}

	matches := engine.Evaluate(event)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].DriverID != "amqp" {
		t.Fatalf("expected driver id amqp, got %q", matches[0].DriverID)
	}
}

// TestRuleEngineJSONPathDot tests that the rule engine correctly handles a JSONPath expression with dot notation.
func TestRuleEngineJSONPathDot(t *testing.T) {
	cfg := RulesConfig{
		Rules: []Rule{
			{When: "$.pull_request.draft == false", Emit: EmitList{"pr.opened"}},
		},
	}

	engine, err := NewRuleEngine(cfg)
	if err != nil {
		t.Fatalf("new rule engine: %v", err)
	}

	event := Event{
		Provider:   "github",
		Name:       "pull_request",
		RawPayload: []byte(`{"pull_request":{"draft":false}}`),
	}

	matches := engine.Evaluate(event)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
}

// TestRuleEngineJSONPathIndex tests that the rule engine correctly handles a JSONPath expression with an index.
func TestRuleEngineJSONPathIndex(t *testing.T) {
	cfg := RulesConfig{
		Rules: []Rule{
			{When: "$.pull_request[0].draft == false", Emit: EmitList{"pr.opened"}},
		},
	}

	engine, err := NewRuleEngine(cfg)
	if err != nil {
		t.Fatalf("new rule engine: %v", err)
	}

	event := Event{
		Provider:   "github",
		Name:       "pull_request",
		RawPayload: []byte(`{"pull_request":[{"draft":false}]}`),
	}

	matches := engine.Evaluate(event)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
}

// TestRuleEngineJSONPathFilter tests that the rule engine correctly handles a JSONPath expression with a filter.
func TestRuleEngineJSONPathFilter(t *testing.T) {
	cfg := RulesConfig{
		Rules: []Rule{
			{When: "$.pull_request[0].draft == false", Emit: EmitList{"pr.opened"}},
		},
	}

	engine, err := NewRuleEngine(cfg)
	if err != nil {
		t.Fatalf("new rule engine: %v", err)
	}

	event := Event{
		Provider:   "github",
		Name:       "pull_request",
		RawPayload: []byte(`{"pull_request":[{"draft":false},{"draft":true}]}`),
	}

	matches := engine.Evaluate(event)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
}

// TestRuleEngineBareJSONPath tests that the rule engine correctly handles a bare JSONPath expression.
func TestRuleEngineBareJSONPath(t *testing.T) {
	cfg := RulesConfig{
		Rules: []Rule{
			{When: "action == \"opened\" && pull_request.draft == false", Emit: EmitList{"pr.opened"}},
			{When: "pull_requests[0].draft == false", Emit: EmitList{"pr.any"}},
		},
	}

	engine, err := NewRuleEngine(cfg)
	if err != nil {
		t.Fatalf("new rule engine: %v", err)
	}

	event := Event{
		Provider:   "github",
		Name:       "pull_request",
		RawPayload: []byte(`{"action":"opened","pull_request":{"draft":false},"pull_requests":[{"draft":false}]}`),
	}

	matches := engine.Evaluate(event)
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
}

// TestRuleEngineStrictMissing tests that the rule engine in strict mode does not match a rule with a missing field.
func TestRuleEngineStrictMissing(t *testing.T) {
	cfg := RulesConfig{
		Rules: []Rule{
			{When: "missing_field == true", Emit: EmitList{"never"}},
		},
		Strict: true,
	}

	engine, err := NewRuleEngine(cfg)
	if err != nil {
		t.Fatalf("new rule engine: %v", err)
	}

	event := Event{
		Provider:   "github",
		Name:       "pull_request",
		RawPayload: []byte(`{"action":"opened"}`),
	}

	matches := engine.Evaluate(event)
	if len(matches) != 0 {
		t.Fatalf("expected no matches in strict mode, got %d", len(matches))
	}
}

func TestRuleEngineFunctions(t *testing.T) {
	cfg := RulesConfig{
		Rules: []Rule{
			{When: `contains(labels, "bug")`, Emit: EmitList{"label.bug"}},
			{When: `like(ref, "refs/heads/%")`, Emit: EmitList{"branch.push"}},
		},
	}

	engine, err := NewRuleEngine(cfg)
	if err != nil {
		t.Fatalf("new rule engine: %v", err)
	}

	event := Event{
		Provider:   "github",
		Name:       "push",
		RawPayload: []byte(`{"labels":["bug","ui"],"ref":"refs/heads/main"}`),
	}

	matches := engine.Evaluate(event)
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
}

func TestRuleEngineEmitList(t *testing.T) {
	cfg := RulesConfig{
		Rules: []Rule{
			{When: `action == "closed"`, Emit: EmitList{"pr.merged", "audit.pr.merged"}},
		},
	}

	engine, err := NewRuleEngine(cfg)
	if err != nil {
		t.Fatalf("new rule engine: %v", err)
	}

	event := Event{
		Provider:   "github",
		Name:       "pull_request",
		RawPayload: []byte(`{"action":"closed"}`),
	}

	matches := engine.Evaluate(event)
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
	if matches[0].Topic != "pr.merged" || matches[1].Topic != "audit.pr.merged" {
		t.Fatalf("unexpected topics: %v, %v", matches[0].Topic, matches[1].Topic)
	}
}
