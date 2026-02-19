package core

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"log"
	"sort"
	"strings"

	"github.com/PaesslerAG/jsonpath"
)

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

func ruleIDFromParts(when string, emit []string, driverID string) string {
	driverKey := strings.TrimSpace(driverID)
	key := strings.TrimSpace(when) + "|" + strings.Join(normalizeRuleParts(emit), ",") + "|" + driverKey
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
