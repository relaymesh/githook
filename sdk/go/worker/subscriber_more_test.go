package worker

import (
	"context"
	"testing"

	relaymessage "github.com/relaymesh/relaybus/sdk/core/go/message"
)

func TestBuildSubscriberRequiresDriver(t *testing.T) {
	if _, err := BuildSubscriber(SubscriberConfig{}); err == nil {
		t.Fatalf("expected missing driver error")
	}
}

func TestNewSingleSubscriberUnsupported(t *testing.T) {
	if _, err := newSingleSubscriber(SubscriberConfig{}, "unknown"); err == nil {
		t.Fatalf("expected unsupported driver error")
	}
}

func TestMultiSubscriberEmpty(t *testing.T) {
	m := &multiSubscriber{}
	if err := m.Start(context.Background(), "topic", func(ctx context.Context, msg relaymessage.Message) error { return nil }); err == nil {
		t.Fatalf("expected no subscribers error")
	}
}
