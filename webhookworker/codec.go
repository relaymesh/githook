package webhookworker

import (
	"encoding/json"

	"github.com/ThreeDotsLabs/watermill/message"
)

type Codec interface {
	Decode(topic string, msg *message.Message) (*Event, error)
}

type DefaultCodec struct{}

type envelope struct {
	Provider string                 `json:"provider"`
	Name     string                 `json:"name"`
	Data     map[string]interface{} `json:"data"`
}

func (DefaultCodec) Decode(topic string, msg *message.Message) (*Event, error) {
	var env envelope
	if err := json.Unmarshal(msg.Payload, &env); err != nil {
		return nil, err
	}

	metadata := make(map[string]string, len(msg.Metadata))
	for key, value := range msg.Metadata {
		metadata[key] = value
	}

	payload := json.RawMessage(msg.Payload)
	return &Event{
		Provider:   env.Provider,
		Type:       env.Name,
		Topic:      topic,
		Metadata:   metadata,
		Payload:    payload,
		Normalized: env.Data,
	}, nil
}
