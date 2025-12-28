package internal

import "fmt"

func Flatten(data map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{})
	for key, value := range data {
		flattenInto(out, key, value)
	}
	return out
}

func flattenInto(out map[string]interface{}, path string, value interface{}) {
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, child := range typed {
			next := fmt.Sprintf("%s.%s", path, key)
			flattenInto(out, next, child)
		}
	case []interface{}:
		out[path] = typed
		out[path+"[]"] = typed
		for i, child := range typed {
			next := fmt.Sprintf("%s[%d]", path, i)
			flattenInto(out, next, child)
		}
	default:
		out[path] = value
	}
}
