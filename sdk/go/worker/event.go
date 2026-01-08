package worker

import "encoding/json"

// Event represents a message received by the worker.
type Event struct {
	// Provider is the name of the Git provider (e.g., "github", "gitlab").
	Provider string `json:"provider"`
	// Type is the name of the event (e.g., "pull_request", "push").
	Type string `json:"type"`
	// Topic is the name of the topic the message was received on.
	Topic string `json:"topic"`
	// Metadata contains message-broker-specific metadata.
	Metadata map[string]string `json:"metadata"`
	// Payload is the raw JSON payload of the message.
	Payload json.RawMessage `json:"payload"`
	// Normalized is the decoded JSON payload of the event.
	Normalized map[string]interface{} `json:"normalized"`
	// Client is an API client for the provider, if available.
	Client interface{} `json:"-"`
}
