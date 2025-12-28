package internal

import (
	"log"

	"github.com/Knetic/govaluate"
)

type Rule struct {
	When    string   `yaml:"when"`
	Emit    string   `yaml:"emit"`
	Drivers []string `yaml:"drivers"`
}

type compiledRule struct {
	emit    string
	drivers []string
	expr    *govaluate.EvaluableExpression
}

type RuleEngine struct {
	rules []compiledRule
}

type RuleMatch struct {
	Topic   string
	Drivers []string
}

func NewRuleEngine(cfg RulesConfig) (*RuleEngine, error) {
	rules := make([]compiledRule, 0, len(cfg.Rules))
	for _, rule := range cfg.Rules {
		expr, err := govaluate.NewEvaluableExpression(rule.When)
		if err != nil {
			return nil, err
		}
		rules = append(rules, compiledRule{emit: rule.Emit, drivers: rule.Drivers, expr: expr})
	}

	return &RuleEngine{rules: rules}, nil
}

func (r *RuleEngine) Evaluate(event Event) []RuleMatch {
	if len(r.rules) == 0 {
		return nil
	}

	matches := make([]RuleMatch, 0, 1)
	for _, rule := range r.rules {
		result, err := rule.expr.Evaluate(event.Data)
		if err != nil {
			log.Printf("rule eval failed: %v", err)
			continue
		}
		ok, _ := result.(bool)
		if ok {
			matches = append(matches, RuleMatch{Topic: rule.emit, Drivers: rule.drivers})
		}
	}
	return matches
}
