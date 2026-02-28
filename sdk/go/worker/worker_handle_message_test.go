package worker

import (
	"context"
	"errors"
	"testing"

	relaymessage "github.com/relaymesh/relaybus/sdk/core/go/message"
)

type handleMessageCodec struct {
	evt *Event
	err error
}

func (c handleMessageCodec) Decode(topic string, msg *relaymessage.Message) (*Event, error) {
	return c.evt, c.err
}

type handleMessageRetry struct {
	decision RetryDecision
	calls    int
	lastErr  error
}

func (r *handleMessageRetry) OnError(ctx context.Context, evt *Event, err error) RetryDecision {
	r.calls++
	r.lastErr = err
	return r.decision
}

func TestHandleMessageSuccess(t *testing.T) {
	finishCalled := false
	finishErrNil := false
	retry := &handleMessageRetry{decision: RetryDecision{Retry: false, Nack: true}}

	w := New(
		WithCodec(handleMessageCodec{evt: &Event{Topic: "topic", Type: "push", Metadata: map[string]string{}}}),
		WithRetry(retry),
		WithListener(Listener{
			OnMessageFinish: func(ctx context.Context, evt *Event, err error) {
				finishCalled = true
				finishErrNil = err == nil
			},
		}),
	)
	w.topicHandlers["topic"] = func(ctx context.Context, evt *Event) error {
		return nil
	}

	shouldNack := w.handleMessage(context.Background(), "topic", &relaymessage.Message{Metadata: map[string]string{}})
	if shouldNack {
		t.Fatalf("expected successful handler to not request nack")
	}
	if !finishCalled {
		t.Fatalf("expected OnMessageFinish to be called")
	}
	if !finishErrNil {
		t.Fatalf("expected OnMessageFinish err to be nil")
	}
	if retry.calls != 0 {
		t.Fatalf("expected retry policy not to be called on success")
	}
}

func TestHandleMessageHandlerErrorTriggersRetryAndErrorListener(t *testing.T) {
	sentinelErr := errors.New("handler failed")
	finishErr := error(nil)
	errorEvt := (*Event)(nil)
	retry := &handleMessageRetry{decision: RetryDecision{Retry: false, Nack: true}}

	w := New(
		WithCodec(handleMessageCodec{evt: &Event{Topic: "topic", Type: "push", Metadata: map[string]string{}}}),
		WithRetry(retry),
		WithListener(Listener{
			OnMessageFinish: func(ctx context.Context, evt *Event, err error) {
				finishErr = err
			},
			OnError: func(ctx context.Context, evt *Event, err error) {
				errorEvt = evt
			},
		}),
	)
	w.topicHandlers["topic"] = func(ctx context.Context, evt *Event) error {
		return sentinelErr
	}

	shouldNack := w.handleMessage(context.Background(), "topic", &relaymessage.Message{Metadata: map[string]string{}})
	if !shouldNack {
		t.Fatalf("expected failed handler to request nack")
	}
	if !errors.Is(finishErr, sentinelErr) {
		t.Fatalf("expected OnMessageFinish to receive handler error")
	}
	if errorEvt == nil {
		t.Fatalf("expected OnError to receive decoded event")
	}
	if retry.calls != 1 {
		t.Fatalf("expected retry policy called once, got %d", retry.calls)
	}
	if !errors.Is(retry.lastErr, sentinelErr) {
		t.Fatalf("expected retry policy to receive handler error")
	}
}

func TestHandleMessageDecodeErrorTriggersRetryAndErrorListener(t *testing.T) {
	decodeErr := errors.New("decode failed")
	errorEvt := &Event{}
	errorCalled := false
	retry := &handleMessageRetry{decision: RetryDecision{Retry: true, Nack: false}}

	w := New(
		WithCodec(handleMessageCodec{err: decodeErr}),
		WithRetry(retry),
		WithListener(Listener{
			OnError: func(ctx context.Context, evt *Event, err error) {
				errorCalled = true
				errorEvt = evt
			},
		}),
	)

	shouldNack := w.handleMessage(context.Background(), "topic", &relaymessage.Message{Metadata: map[string]string{}})
	if !shouldNack {
		t.Fatalf("expected decode error to request retry/nack decision")
	}
	if !errorCalled {
		t.Fatalf("expected OnError to be called on decode failure")
	}
	if errorEvt != nil {
		t.Fatalf("expected OnError evt to be nil on decode failure")
	}
	if retry.calls != 1 {
		t.Fatalf("expected retry policy called once, got %d", retry.calls)
	}
	if !errors.Is(retry.lastErr, decodeErr) {
		t.Fatalf("expected retry policy to receive decode error")
	}
}
