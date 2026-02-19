package core

// Event represents a webhook event from a Git provider.
type Event struct {
	// Provider is the name of the Git provider (e.g., "github", "gitlab").
	Provider string `json:"provider"`
	// Name is the name of the event (e.g., "pull_request", "push").
	Name string `json:"name"`
	// RequestID links the event back to the inbound webhook request.
	RequestID string `json:"request_id,omitempty"`
	// LogID links the event to a stored event log entry.
	LogID string `json:"-"`
	// Headers contains the inbound webhook request headers.
	Headers map[string][]string `json:"-"`
	// Data is the flattened JSON payload of the event.
	Data map[string]interface{} `json:"data"`
	// RawPayload is the raw JSON payload from the webhook.
	RawPayload []byte `json:"-"`
	// RawObject is the unmarshalled JSON payload.
	RawObject interface{} `json:"-"`
	// StateID maps the event to an installation/account id for token lookup.
	StateID string `json:"-"`
	// InstallationID maps the event to a provider installation for token lookup.
	InstallationID string `json:"-"`
	// NamespaceID identifies the repository/namespace for the event.
	NamespaceID string `json:"-"`
	// NamespaceName is the human-readable namespace (owner/repo).
	NamespaceName string `json:"-"`
}
