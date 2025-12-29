package worker

import "encoding/json"

type Event struct {
	Provider   string                 `json:"provider"`
	Type       string                 `json:"type"`
	Topic      string                 `json:"topic"`
	Metadata   map[string]string      `json:"metadata"`
	Payload    json.RawMessage        `json:"payload"`
	Normalized map[string]interface{} `json:"normalized"`
	Client     interface{}            `json:"-"`
}
