package core

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRewriteExpressionHelpers(t *testing.T) {
	rewritten, varMap := rewriteExpression(`contains(action, "open") && $.repository.full_name == "org/repo" && true`)
	if rewritten == "" {
		t.Fatalf("expected rewritten expression")
	}
	if _, ok := varMap["v___action"]; !ok {
		t.Fatalf("expected action mapping")
	}
	if _, ok := varMap["v___repository_full_name"]; !ok {
		t.Fatalf("expected jsonpath mapping")
	}

	if !isFunctionName("contains") || isFunctionName("missing") {
		t.Fatalf("unexpected function name detection")
	}
	if !nextNonSpaceIs("a   (", 1, '(') {
		t.Fatalf("expected next non-space check true")
	}
	if nextNonSpaceIs("abc", 1, '(') {
		t.Fatalf("expected next non-space check false")
	}

	token, next := parseJSONPathToken("$.a[?(@.x==1)] && y", 0)
	if token == "" || next == 0 {
		t.Fatalf("expected parsed jsonpath token")
	}

	if got := safeVarName("$.repository[0].name"); got == "" || got[:2] != "v_" {
		t.Fatalf("expected safe var name, got %q", got)
	}
	if !isKeyword("true") || isKeyword("contains") {
		t.Fatalf("unexpected keyword classification")
	}
	if !isIdentStart('a') || isIdentStart('-') {
		t.Fatalf("unexpected ident start classification")
	}
	if !isTerminator(')') || isTerminator('a') {
		t.Fatalf("unexpected terminator classification")
	}

	ident, idx := parseIdentifier("abc123+z", 0)
	if ident != "abc123" || idx <= 0 {
		t.Fatalf("unexpected identifier parse: %q idx=%d", ident, idx)
	}
}

func TestResolveRuleParamsAndJSONPath(t *testing.T) {
	event := Event{
		Data: map[string]interface{}{"action": "opened", "count": 2},
		RawObject: map[string]interface{}{
			"action":     "opened",
			"repository": map[string]interface{}{"full_name": "org/repo"},
		},
	}

	params, missing := resolveRuleParams(nil, event, []string{"v___action", "v___repo", "direct", "missing"}, map[string]string{
		"v___action": "$.action",
		"v___repo":   "$.repository.full_name",
	})
	if len(missing) == 0 {
		t.Fatalf("expected missing params")
	}
	if params["v___action"] != "opened" {
		t.Fatalf("expected action param")
	}
	if params["v___repo"] != "org/repo" {
		t.Fatalf("expected repo param")
	}

	if v, err := resolveJSONPath(Event{Data: map[string]interface{}{"a": 1}}, "$.a"); err != nil || v != 1 {
		t.Fatalf("expected data jsonpath value, got v=%v err=%v", v, err)
	}
	if v, err := resolveJSONPath(Event{}, "$.a"); err != nil || v != nil {
		t.Fatalf("expected nil value on empty event, got v=%v err=%v", v, err)
	}
	if _, err := resolveJSONPath(Event{RawPayload: []byte("{")}, "$.a"); err == nil {
		t.Fatalf("expected invalid payload error")
	}

	rawPayload, _ := json.Marshal(map[string]interface{}{"items": []interface{}{1, 2}})
	if v, err := resolveJSONPath(Event{RawPayload: rawPayload}, "$.items"); err != nil || v == nil {
		t.Fatalf("expected payload jsonpath result, got v=%v err=%v", v, err)
	}

	if got := normalizeJSONPathResult([]interface{}{}); got != nil {
		t.Fatalf("expected empty slice normalized to nil")
	}
	if got := normalizeJSONPathResult([]interface{}{42}); got != 42 {
		t.Fatalf("expected single item normalization")
	}

	id1 := ruleIDFromParts("a==1", []string{"x", " y "}, "driver")
	id2 := ruleIDFromParts("a==1", []string{"y", "x"}, "driver")
	if id1 != id2 {
		t.Fatalf("expected deterministic rule id regardless of emit order")
	}
}

func TestEmitListUnmarshalAndValues(t *testing.T) {
	var cfg struct {
		Emit EmitList `yaml:"emit"`
	}

	if err := yaml.Unmarshal([]byte("emit: topic.a\n"), &cfg); err != nil {
		t.Fatalf("unmarshal scalar emit: %v", err)
	}
	if len(cfg.Emit.Values()) != 1 || cfg.Emit.Values()[0] != "topic.a" {
		t.Fatalf("unexpected emit scalar values")
	}

	if err := yaml.Unmarshal([]byte("emit:\n  - topic.a\n  - '  '\n  - topic.b\n"), &cfg); err != nil {
		t.Fatalf("unmarshal sequence emit: %v", err)
	}
	values := cfg.Emit.Values()
	if len(values) != 2 || values[0] != "topic.a" || values[1] != "topic.b" {
		t.Fatalf("unexpected emit sequence values: %#v", values)
	}

	if err := yaml.Unmarshal([]byte("emit: {a: b}\n"), &cfg); err == nil {
		t.Fatalf("expected emit map to fail")
	}
}
