package worker

import (
	"context"
	"testing"

	relaymessage "github.com/relaymesh/relaybus/sdk/core/go/message"
)

type testCodec struct{}

func (testCodec) Decode(topic string, msg *relaymessage.Message) (*Event, error) { return nil, nil }

type testRetry struct{}

func (testRetry) OnError(ctx context.Context, evt *Event, err error) RetryDecision {
	return RetryDecision{}
}

type testLogger struct{}

func (testLogger) Printf(format string, args ...interface{}) {}

func TestOptionsApply(t *testing.T) {
	sub := &testSubscriber{}
	codec := testCodec{}
	retry := testRetry{}
	logger := testLogger{}
	listener := Listener{}
	worker := New(
		WithSubscriber(sub),
		WithConcurrency(3),
		WithCodec(codec),
		WithRetry(retry),
		WithLogger(logger),
		WithListener(listener),
	)
	if worker.subscriber != sub {
		t.Fatalf("expected subscriber set")
	}
	if worker.concurrency != 3 {
		t.Fatalf("expected concurrency set")
	}
	if worker.codec != codec {
		t.Fatalf("expected codec set")
	}
	if worker.retry != retry {
		t.Fatalf("expected retry set")
	}
	if worker.logger != logger {
		t.Fatalf("expected logger set")
	}
	if len(worker.listeners) != 1 {
		t.Fatalf("expected listener set")
	}
}

type testSubscriber struct{}

func (testSubscriber) Start(ctx context.Context, topic string, handler MessageHandler) error {
	return nil
}
func (testSubscriber) Close() error { return nil }
