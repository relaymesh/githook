package internal

import (
	"context"
	"testing"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
)

type stubPublisher struct {
	published int
	lastTopic string
}

func (s *stubPublisher) Publish(topic string, msgs ...*message.Message) error {
	s.published += len(msgs)
	s.lastTopic = topic
	return nil
}

func (s *stubPublisher) Close() error {
	return nil
}

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

	if err := pub.Publish(context.Background(), "custom.topic", Event{Provider: "github"}); err != nil {
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
