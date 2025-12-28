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

func TestHTTPURLTarget(t *testing.T) {
	url, err := httpTargetURL(HTTPConfig{Mode: "base_url", BaseURL: "http://localhost:8080/hooks"}, "topic")
	if err != nil {
		t.Fatalf("httpTargetURL: %v", err)
	}
	if url != "http://localhost:8080/hooks/topic" {
		t.Fatalf("unexpected url: %q", url)
	}
}

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
