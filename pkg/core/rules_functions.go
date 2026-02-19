package core

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/Knetic/govaluate"
)

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
