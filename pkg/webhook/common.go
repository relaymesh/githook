package webhook

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/ThreeDotsLabs/watermill"

	"githooks/pkg/core"
)

// rawObjectAndFlatten unmarshals a raw JSON byte slice into both an interface{}
// and a flattened map[string]interface{}. This is useful for both preserving the
// original structure and for easy access to nested fields.
func rawObjectAndFlatten(raw []byte) (interface{}, map[string]interface{}) {
	var out interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, map[string]interface{}{}
	}
	objectMap, ok := out.(map[string]interface{})
	if !ok {
		return out, map[string]interface{}{}
	}
	return out, core.Flatten(objectMap)
}

func requestID(r *http.Request) string {
	if r == nil {
		return watermill.NewUUID()
	}
	if id := r.Header.Get("X-Request-Id"); id != "" {
		return id
	}
	if id := r.Header.Get("X-Request-ID"); id != "" {
		return id
	}
	if id := r.Header.Get("X-Correlation-Id"); id != "" {
		return id
	}
	return watermill.NewUUID()
}

func logDebugEvent(logger *log.Logger, provider string, event string, body []byte) {
	if logger == nil {
		logger = log.Default()
	}
	logger.Printf("debug event provider=%s name=%s payload=%s", provider, event, string(body))
}
