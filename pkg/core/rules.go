package core

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/Knetic/govaluate"
	"github.com/PaesslerAG/jsonpath"
	"gopkg.in/yaml.v3"

	"githook/pkg/cache"
)

// Rule defines a condition and an action to take when the condition is met.
type Rule struct {
	// ID is an optional identifier for the rule.
	ID string `yaml:"id"`
	// When is a govaluate expression that is evaluated against the event data.
	When string `yaml:"when"`
	// Emit is the topic to publish the event to if the 'When' expression is true.
	Emit EmitList `yaml:"emit"`
	// Drivers is a list of publisher drivers to use for this rule.
	// If empty, the default drivers are used.
	Drivers []string `yaml:"drivers"`
}

// compiledRule is a pre-processed version of a Rule.
type compiledRule struct {
	id      string
	when    string
	emit    []string
	drivers []string
	vars    []string
	varMap  map[string]string
	expr    *govaluate.EvaluableExpression
}

// RuleEngine evaluates events against a set of rules.
type RuleEngine struct {
	mu     sync.RWMutex
	rules  []compiledRule
	strict bool
	logger *log.Logger
	tenant *cache.TenantCache[ruleSet]
}

type ruleSet struct {
	rules  []compiledRule
	strict bool
}

// RuleMatch represents a successful rule evaluation.
type RuleMatch struct {
	Topic   string
	Drivers []string
}

// MatchedRule represents a successful rule evaluation with the original rule data.
type MatchedRule struct {
	ID      string
	When    string
	Emit    []string
	Drivers []string
}

// NewRuleEngine creates a new RuleEngine from a set of rules.
// It pre-compiles the expressions in the rules for faster evaluation.
func NewRuleEngine(cfg RulesConfig) (*RuleEngine, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = log.Default()
	}
	engine := &RuleEngine{logger: logger, strict: cfg.Strict}
	if err := engine.Update(cfg); err != nil {
		return nil, err
	}
	return engine, nil
}

// Update replaces the rule set and strict mode in the engine.
func (r *RuleEngine) Update(cfg RulesConfig) error {
	logger := cfg.Logger
	if logger == nil {
		logger = r.logger
	}
	rules := make([]compiledRule, 0, len(cfg.Rules))
	for _, rule := range cfg.Rules {
		rewritten, varMap := rewriteExpression(rule.When)
		expr, err := govaluate.NewEvaluableExpressionWithFunctions(rewritten, ruleFunctions())
		if err != nil {
			return err
		}
		emit := rule.Emit.Values()
		ruleID := strings.TrimSpace(rule.ID)
		if ruleID == "" {
			ruleID = ruleIDFromParts(rule.When, emit, rule.Drivers)
		}
		rules = append(rules, compiledRule{
			id:      ruleID,
			when:    rule.When,
			emit:    emit,
			drivers: rule.Drivers,
			vars:    expr.Vars(),
			varMap:  varMap,
			expr:    expr,
		})
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	tenantID := strings.TrimSpace(cfg.TenantID)
	if tenantID == "" {
		r.rules = rules
		r.strict = cfg.Strict
	} else {
		if r.tenant == nil {
			r.tenant = cache.NewTenantCache[ruleSet]()
		}
		if len(rules) == 0 {
			r.tenant.Delete(tenantID)
		} else {
			r.tenant.Set(tenantID, ruleSet{rules: rules, strict: cfg.Strict})
		}
	}
	if logger != nil {
		r.logger = logger
	}
	return nil
}

// EmitList supports either a string or list of strings in YAML.
type EmitList []string

func (e *EmitList) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		if value.Value == "" {
			*e = nil
			return nil
		}
		*e = EmitList{value.Value}
		return nil
	case yaml.SequenceNode:
		out := make([]string, 0, len(value.Content))
		for _, item := range value.Content {
			if item.Kind != yaml.ScalarNode {
				return fmt.Errorf("emit items must be strings")
			}
			out = append(out, item.Value)
		}
		*e = EmitList(out)
		return nil
	default:
		return fmt.Errorf("emit must be a string or list of strings")
	}
}

func (e EmitList) Values() []string {
	out := make([]string, 0, len(e))
	for _, val := range e {
		trimmed := strings.TrimSpace(val)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func ruleFunctions() map[string]govaluate.ExpressionFunction {
	return map[string]govaluate.ExpressionFunction{
		"contains": containsFunc,
		"like":     likeFunc,
	}
}

func containsFunc(args ...interface{}) (interface{}, error) {
	if len(args) == 1 {
		value := reflect.ValueOf(args[0])
		if value.IsValid() && (value.Kind() == reflect.Slice || value.Kind() == reflect.Array) && value.Len() >= 2 {
			unpacked := make([]interface{}, value.Len())
			for i := 0; i < value.Len(); i++ {
				unpacked[i] = value.Index(i).Interface()
			}
			args = unpacked
		}
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("contains expects 2 args, got 0")
	}
	if len(args) > 2 {
		haystack := args[:len(args)-1]
		needle := args[len(args)-1]
		return sliceContains(haystack, needle), nil
	}
	if len(args) != 2 {
		return nil, fmt.Errorf("contains expects 2 args, got %d", len(args))
	}
	if args[0] == nil || args[1] == nil {
		return false, nil
	}
	switch hay := args[0].(type) {
	case string:
		needle, ok := args[1].(string)
		if !ok {
			return false, nil
		}
		return strings.Contains(hay, needle), nil
	case []interface{}:
		return sliceContains(hay, args[1]), nil
	case []string:
		needle, ok := args[1].(string)
		if !ok {
			return false, nil
		}
		for _, item := range hay {
			if item == needle {
				return true, nil
			}
		}
		return false, nil
	}
	return reflectContains(args[0], args[1]), nil
}

func sliceContains(values []interface{}, needle interface{}) bool {
	for _, item := range values {
		if reflect.DeepEqual(item, needle) {
			return true
		}
	}
	return false
}

func reflectContains(hay interface{}, needle interface{}) bool {
	value := reflect.ValueOf(hay)
	switch value.Kind() {
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			if reflect.DeepEqual(value.Index(i).Interface(), needle) {
				return true
			}
		}
	case reflect.Map:
		key := reflect.ValueOf(needle)
		if key.IsValid() {
			return value.MapIndex(key).IsValid()
		}
	}
	return false
}

func likeFunc(args ...interface{}) (interface{}, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("like expects 2 args")
	}
	left, ok := args[0].(string)
	if !ok {
		return false, nil
	}
	pattern, ok := args[1].(string)
	if !ok {
		return false, nil
	}
	regex := likePatternToRegex(pattern)
	matched, err := regexp.MatchString(regex, left)
	if err != nil {
		return false, err
	}
	return matched, nil
}

func likePatternToRegex(pattern string) string {
	escaped := regexp.QuoteMeta(pattern)
	escaped = strings.ReplaceAll(escaped, "%", ".*")
	escaped = strings.ReplaceAll(escaped, "_", ".")
	return "^" + escaped + "$"
}

// Evaluate runs an event through the rule engine and returns a list of topics to publish to.
func (r *RuleEngine) Evaluate(event Event) []RuleMatch {
	return r.evaluateWithLogger(event, r.logger)
}

func (r *RuleEngine) EvaluateWithLogger(event Event, logger *log.Logger) []RuleMatch {
	return r.evaluateWithLogger(event, logger)
}

func (r *RuleEngine) EvaluateForTenant(event Event, tenantID string) []RuleMatch {
	return r.evaluateWithLoggerForTenant(event, tenantID, r.logger)
}

func (r *RuleEngine) EvaluateForTenantWithLogger(event Event, tenantID string, logger *log.Logger) []RuleMatch {
	return r.evaluateWithLoggerForTenant(event, tenantID, logger)
}

func (r *RuleEngine) evaluateWithLogger(event Event, logger *log.Logger) []RuleMatch {
	return r.evaluateWithLoggerForTenant(event, "", logger)
}

func (r *RuleEngine) evaluateWithLoggerForTenant(event Event, tenantID string, logger *log.Logger) []RuleMatch {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tenantID = strings.TrimSpace(tenantID)
	rules := r.rules
	strict := r.strict
	if tenantID != "" {
		if r.tenant == nil {
			return nil
		}
		set, ok := r.tenant.Get(tenantID)
		if !ok {
			return nil
		}
		rules = set.rules
		strict = set.strict
	}
	if len(rules) == 0 {
		return nil
	}
	if logger == nil {
		logger = log.Default()
	}

	matches := make([]RuleMatch, 0, 1)
	for _, rule := range rules {
		params, missing := resolveRuleParams(logger, event, rule.vars, rule.varMap)
		logger.Printf("rule debug: when=%q params=%v", rule.expr.String(), params)
		if strict && len(missing) > 0 {
			logger.Printf("rule strict missing params: %v", missing)
			continue
		}
		result, err := rule.expr.Evaluate(params)
		if err != nil {
			logger.Printf("rule eval failed: %v", err)
			continue
		}
		ok, _ := result.(bool)
		if ok {
			for _, topic := range rule.emit {
				matches = append(matches, RuleMatch{Topic: topic, Drivers: rule.drivers})
			}
		}
	}
	return matches
}

// EvaluateRules returns rule-level matches with the original rule metadata.
func (r *RuleEngine) EvaluateRules(event Event) []MatchedRule {
	return r.evaluateRulesWithLogger(event, r.logger)
}

// EvaluateRulesWithLogger returns rule-level matches using the provided logger.
func (r *RuleEngine) EvaluateRulesWithLogger(event Event, logger *log.Logger) []MatchedRule {
	return r.evaluateRulesWithLogger(event, logger)
}

func (r *RuleEngine) EvaluateRulesForTenant(event Event, tenantID string) []MatchedRule {
	return r.evaluateRulesWithLoggerForTenant(event, tenantID, r.logger)
}

// EvaluateRulesForTenantWithLogger returns rule-level matches scoped to a tenant using the provided logger.
func (r *RuleEngine) EvaluateRulesForTenantWithLogger(event Event, tenantID string, logger *log.Logger) []MatchedRule {
	return r.evaluateRulesWithLoggerForTenant(event, tenantID, logger)
}

func (r *RuleEngine) evaluateRulesWithLogger(event Event, logger *log.Logger) []MatchedRule {
	return r.evaluateRulesWithLoggerForTenant(event, "", logger)
}

func (r *RuleEngine) evaluateRulesWithLoggerForTenant(event Event, tenantID string, logger *log.Logger) []MatchedRule {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tenantID = strings.TrimSpace(tenantID)
	rules := r.rules
	strict := r.strict
	if tenantID != "" {
		if r.tenant == nil {
			return nil
		}
		set, ok := r.tenant.Get(tenantID)
		if !ok {
			return nil
		}
		rules = set.rules
		strict = set.strict
	}
	if len(rules) == 0 {
		return nil
	}
	if logger == nil {
		logger = log.Default()
	}

	matches := make([]MatchedRule, 0, 1)
	for _, rule := range rules {
		params, missing := resolveRuleParams(logger, event, rule.vars, rule.varMap)
		logger.Printf("rule debug: when=%q params=%v", rule.expr.String(), params)
		if strict && len(missing) > 0 {
			logger.Printf("rule strict missing params: %v", missing)
			continue
		}
		result, err := rule.expr.Evaluate(params)
		if err != nil {
			logger.Printf("rule eval failed: %v", err)
			continue
		}
		ok, _ := result.(bool)
		if ok {
			matches = append(matches, MatchedRule{
				ID:      rule.id,
				When:    rule.when,
				Emit:    append([]string(nil), rule.emit...),
				Drivers: append([]string(nil), rule.drivers...),
			})
		}
	}
	return matches
}

func resolveRuleParams(logger *log.Logger, event Event, vars []string, varMap map[string]string) (map[string]interface{}, []string) {
	if logger == nil {
		logger = log.Default()
	}
	if len(vars) == 0 {
		if len(event.RawPayload) == 0 {
			return event.Data, nil
		}
		return nil, nil
	}

	params := make(map[string]interface{}, len(vars))
	missing := make([]string, 0)
	for _, name := range vars {
		if path, ok := varMap[name]; ok {
			value, err := resolveJSONPath(event, path)
			if err != nil {
				missing = append(missing, path)
				logger.Printf("rule warn: jsonpath error path=%s err=%v", path, err)
				params[name] = nil
			} else {
				if value == nil {
					missing = append(missing, path)
					logger.Printf("rule warn: jsonpath no match path=%s", path)
				}
				params[name] = value
			}
			continue
		}
		if value, ok := event.Data[name]; ok {
			params[name] = value
		} else {
			missing = append(missing, name)
			params[name] = nil
		}
	}
	return params, missing
}

func resolveJSONPath(event Event, path string) (interface{}, error) {
	if event.RawObject != nil {
		value, err := jsonpath.Get(path, event.RawObject)
		if err != nil {
			return nil, err
		}
		return normalizeJSONPathResult(value), nil
	}
	if len(event.RawPayload) == 0 {
		if event.Data != nil {
			value, err := jsonpath.Get(path, event.Data)
			if err != nil {
				return nil, err
			}
			return normalizeJSONPathResult(value), nil
		}
		return nil, nil
	}
	var raw interface{}
	if err := json.Unmarshal(event.RawPayload, &raw); err != nil {
		return nil, err
	}
	value, err := jsonpath.Get(path, raw)
	if err != nil {
		return nil, err
	}
	return normalizeJSONPathResult(value), nil
}

func ruleIDFromParts(when string, emit []string, drivers []string) string {
	key := strings.TrimSpace(when) + "|" + strings.Join(normalizeRuleParts(emit), ",") + "|" + strings.Join(normalizeRuleParts(drivers), ",")
	sum := sha1.Sum([]byte(key))
	return "rule_" + hex.EncodeToString(sum[:])
}

func normalizeRuleParts(values []string) []string {
	clean := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		clean = append(clean, trimmed)
	}
	sort.Strings(clean)
	return clean
}

func normalizeJSONPathResult(value interface{}) interface{} {
	items, ok := value.([]interface{})
	if !ok {
		return value
	}
	if len(items) == 0 {
		return nil
	}
	if len(items) == 1 {
		return items[0]
	}
	return items
}

func rewriteExpression(expr string) (string, map[string]string) {
	var out strings.Builder
	out.Grow(len(expr))

	varMap := make(map[string]string)
	inString := false
	var stringQuote byte

	for i := 0; i < len(expr); {
		ch := expr[i]

		if inString {
			out.WriteByte(ch)
			if ch == '\\' && i+1 < len(expr) {
				out.WriteByte(expr[i+1])
				i += 2
				continue
			}
			if ch == stringQuote {
				inString = false
			}
			i++
			continue
		}

		if ch == '"' || ch == '\'' {
			inString = true
			stringQuote = ch
			out.WriteByte(ch)
			i++
			continue
		}

		if ch == '$' || isIdentStart(ch) {
			if isIdentStart(ch) {
				if ident, next := parseIdentifier(expr, i); isFunctionName(ident) && nextNonSpaceIs(expr, next, '(') {
					out.WriteString(ident)
					i = next
					continue
				}
			}
			token, next := parseJSONPathToken(expr, i)
			if isKeyword(token) {
				out.WriteString(token)
				i = next
				continue
			}
			path := token
			if token[0] != '$' {
				path = "$." + token
			}
			safe := safeVarName(path)
			varMap[safe] = path
			out.WriteString(safe)
			i = next
			continue
		}

		out.WriteByte(ch)
		i++
	}

	return out.String(), varMap
}

func isFunctionName(token string) bool {
	switch token {
	case "contains", "like":
		return true
	default:
		return false
	}
}

func parseIdentifier(expr string, start int) (string, int) {
	i := start
	for i < len(expr) {
		ch := expr[i]
		if isIdentStart(ch) || (ch >= '0' && ch <= '9') {
			i++
			continue
		}
		break
	}
	return expr[start:i], i
}

func nextNonSpaceIs(expr string, start int, want byte) bool {
	for i := start; i < len(expr); i++ {
		switch expr[i] {
		case ' ', '\t', '\n', '\r':
			continue
		default:
			return expr[i] == want
		}
	}
	return false
}

func parseJSONPathToken(expr string, start int) (string, int) {
	i := start
	bracketDepth := 0
	parenDepth := 0
	var quote byte

	for i < len(expr) {
		ch := expr[i]

		if quote != 0 {
			if ch == '\\' && i+1 < len(expr) {
				i += 2
				continue
			}
			if ch == quote {
				quote = 0
			}
			i++
			continue
		}

		switch ch {
		case '\'', '"':
			quote = ch
			i++
			continue
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
		case '(':
			if bracketDepth > 0 {
				parenDepth++
			}
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
		}

		if bracketDepth == 0 && parenDepth == 0 && isTerminator(ch) {
			break
		}

		i++
	}
	return expr[start:i], i
}

func isTerminator(ch byte) bool {
	switch ch {
	case ' ', '\t', '\n', '\r', ',', ';':
		return true
	case '+', '-', '*', '/', '%':
		return true
	case '=', '!', '<', '>', '&', '|':
		return true
	case ')':
		return true
	default:
		return false
	}
}

func safeVarName(token string) string {
	var b strings.Builder
	b.Grow(len(token) + 2)
	b.WriteString("v_")
	for i := 0; i < len(token); i++ {
		ch := token[i]
		if isIdentStart(ch) || (ch >= '0' && ch <= '9') {
			b.WriteByte(ch)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

func isIdentStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isKeyword(token string) bool {
	switch token {
	case "true", "false", "null":
		return true
	default:
		return false
	}
}
