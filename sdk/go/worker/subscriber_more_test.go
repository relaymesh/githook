package worker

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ThreeDotsLabs/watermill/message"
)

func TestBuildSubscriberDefault(t *testing.T) {
	sub, err := BuildSubscriber(SubscriberConfig{})
	if err != nil {
		t.Fatalf("build subscriber: %v", err)
	}
	if sub == nil {
		t.Fatalf("expected subscriber")
	}
	_ = sub.Close()
}

func TestBuildSingleSubscriberUnsupported(t *testing.T) {
	if _, err := buildSingleSubscriber(SubscriberConfig{}, nil, "unknown"); err == nil {
		t.Fatalf("expected unsupported driver error")
	}
}

func TestMultiSubscriberEmpty(t *testing.T) {
	m := &multiSubscriber{}
	if _, err := m.Subscribe(context.Background(), "topic"); err == nil {
		t.Fatalf("expected no subscribers error")
	}
}

func TestClosingSubscriberCloseError(t *testing.T) {
	baseErr := errors.New("sub")
	closeErr := errors.New("close")
	sub := &errSubscriber{err: baseErr}
	c := &closingSubscriber{
		Subscriber: sub,
		closeFn: func() error {
			return closeErr
		},
	}
	err := c.Close()
	if err == nil || !strings.Contains(err.Error(), "sub") || !strings.Contains(err.Error(), "close") {
		t.Fatalf("expected combined error, got %v", err)
	}
}

type errSubscriber struct {
	err error
}

func (e *errSubscriber) Subscribe(ctx context.Context, topic string) (<-chan *message.Message, error) {
	return nil, e.err
}

func (e *errSubscriber) Close() error {
	return e.err
}
