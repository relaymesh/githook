package core

import (
	"context"
	"testing"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"google.golang.org/protobuf/proto"

	cloudv1 "githook/pkg/gen/cloud/v1"
)

// stubPublisher is a mock publisher for testing.
type stubPublisher struct {
	published    int
	lastTopic    string
	lastPayload  []byte
	lastMetadata message.Metadata
}

// Publish increments the published count and records the topic.
func (s *stubPublisher) Publish(topic string, msgs ...*message.Message) error {
	s.published += len(msgs)
	s.lastTopic = topic
	if len(msgs) > 0 {
		s.lastPayload = append([]byte(nil), msgs[0].Payload...)
		s.lastMetadata = msgs[0].Metadata
	}
	return nil
}

// Close is a no-op.
func (s *stubPublisher) Close() error {
	return nil
}

// TestRegisterPublisherDriver tests that a custom publisher driver can be registered and used.
func TestRegisterPublisherDriver(t *testing.T) {
	const driverName = "custom"

	orig, had := publisherFactories[driverName]
	defer func() {
		if had {
			publisherFactories[driverName] = orig
		} else {
			delete(publisherFactories, driverName)
		}
	}()

	stub := &stubPublisher{}
	closed := false
	RegisterPublisherDriver(driverName, func(cfg WatermillConfig, logger watermill.LoggerAdapter) (message.Publisher, func() error, error) {
		return stub, func() error { closed = true; return nil }, nil
	})

	pub, err := NewPublisher(WatermillConfig{Driver: driverName})
	if err != nil {
		t.Fatalf("new publisher: %v", err)
	}

	if err := pub.PublishForDrivers(context.Background(), "custom.topic", Event{Provider: "github"}, nil); err != nil {
		t.Fatalf("publish: %v", err)
	}

	if stub.published != 1 || stub.lastTopic != "custom.topic" {
		t.Fatalf("expected publish to custom.topic once, got %d to %q", stub.published, stub.lastTopic)
	}

	if err := pub.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if !closed {
		t.Fatalf("expected custom close to be called")
	}
}

// TestHTTPURLTarget tests that the HTTP target URL is constructed correctly.
func TestHTTPURLTarget(t *testing.T) {
	url, err := httpTargetURL(HTTPConfig{Mode: "base_url", BaseURL: "http://localhost:8080/hooks"}, "topic")
	if err != nil {
		t.Fatalf("httpTargetURL: %v", err)
	}
	if url != "http://localhost:8080/hooks/topic" {
		t.Fatalf("unexpected url: %q", url)
	}
}

// TestMultipleDrivers tests that the publisher can be configured to publish to multiple drivers.
func TestMultipleDrivers(t *testing.T) {
	orig := publisherFactories["multi-a"]
	origB := publisherFactories["multi-b"]
	defer func() {
		if orig != nil {
			publisherFactories["multi-a"] = orig
		} else {
			delete(publisherFactories, "multi-a")
		}
		if origB != nil {
			publisherFactories["multi-b"] = origB
		} else {
			delete(publisherFactories, "multi-b")
		}
	}()

	a := &stubPublisher{}
	b := &stubPublisher{}

	RegisterPublisherDriver("multi-a", func(cfg WatermillConfig, logger watermill.LoggerAdapter) (message.Publisher, func() error, error) {
		return a, nil, nil
	})
	RegisterPublisherDriver("multi-b", func(cfg WatermillConfig, logger watermill.LoggerAdapter) (message.Publisher, func() error, error) {
		return b, nil, nil
	})

	pub, err := NewPublisher(WatermillConfig{Drivers: []string{"multi-a", "multi-b"}})
	if err != nil {
		t.Fatalf("new publisher: %v", err)
	}

	if err := pub.PublishForDrivers(context.Background(), "multi.topic", Event{Provider: "github"}, nil); err != nil {
		t.Fatalf("publish: %v", err)
	}

	if a.published != 1 || b.published != 1 {
		t.Fatalf("expected publish to both drivers, got a=%d b=%d", a.published, b.published)
	}
}

// TestPublishUsesRawPayloadAndMetadata ensures raw payload is forwarded and metadata is set.
func TestPublishUsesRawPayloadAndMetadata(t *testing.T) {
	const driverName = "payload"

	orig, had := publisherFactories[driverName]
	defer func() {
		if had {
			publisherFactories[driverName] = orig
		} else {
			delete(publisherFactories, driverName)
		}
	}()

	stub := &stubPublisher{}
	RegisterPublisherDriver(driverName, func(cfg WatermillConfig, logger watermill.LoggerAdapter) (message.Publisher, func() error, error) {
		return stub, nil, nil
	})

	pub, err := NewPublisher(WatermillConfig{Driver: driverName})
	if err != nil {
		t.Fatalf("new publisher: %v", err)
	}

	raw := []byte(`{"hello":"world"}`)
	event := Event{
		Provider:   "github",
		Name:       "push",
		RequestID:  "req-123",
		RawPayload: raw,
	}
	if err := pub.PublishForDrivers(context.Background(), "payload.topic", event, nil); err != nil {
		t.Fatalf("publish: %v", err)
	}

	var env cloudv1.EventPayload
	if err := proto.Unmarshal(stub.lastPayload, &env); err != nil {
		t.Fatalf("unmarshal proto payload: %v", err)
	}
	if string(env.GetPayload()) != string(raw) {
		t.Fatalf("expected raw payload to be forwarded")
	}
	if stub.lastMetadata.Get("provider") != "github" {
		t.Fatalf("expected provider metadata")
	}
	if stub.lastMetadata.Get("event") != "push" {
		t.Fatalf("expected event metadata")
	}
	if stub.lastMetadata.Get("request_id") != "req-123" {
		t.Fatalf("expected request_id metadata")
	}
}
