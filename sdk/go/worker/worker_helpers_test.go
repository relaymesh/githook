package worker

import (
	"context"
	"testing"
)

func TestWorkerWrap(t *testing.T) {
	order := make([]string, 0)
	base := func(ctx context.Context, evt *Event) error {
		order = append(order, "handler")
		return nil
	}
	w := New(WithMiddleware(
		func(next Handler) Handler {
			return func(ctx context.Context, evt *Event) error {
				order = append(order, "mw-1")
				return next(ctx, evt)
			}
		},
		func(next Handler) Handler {
			return func(ctx context.Context, evt *Event) error {
				order = append(order, "mw-2")
				return next(ctx, evt)
			}
		},
	))
	wrapped := w.wrap(base)
	if err := wrapped(context.Background(), &Event{}); err != nil {
		t.Fatalf("wrapped handler error: %v", err)
	}
	if len(order) != 3 || order[0] != "mw-1" || order[1] != "mw-2" || order[2] != "handler" {
		t.Fatalf("unexpected order: %v", order)
	}
}

func TestWorkerNotify(t *testing.T) {
	var started, exited, msgStart, msgFinish, errCalled bool
	listener := Listener{
		OnStart: func(ctx context.Context) { started = true },
		OnExit:  func(ctx context.Context) { exited = true },
		OnMessageStart: func(ctx context.Context, evt *Event) {
			msgStart = true
		},
		OnMessageFinish: func(ctx context.Context, evt *Event, err error) {
			msgFinish = true
		},
		OnError: func(ctx context.Context, evt *Event, err error) {
			errCalled = true
		},
	}
	w := New(WithListener(listener))
	w.notifyStart(context.Background())
	w.notifyExit(context.Background())
	w.notifyMessageStart(context.Background(), &Event{})
	w.notifyMessageFinish(context.Background(), &Event{}, nil)
	w.notifyError(context.Background(), &Event{}, errTest{})

	if !started || !exited || !msgStart || !msgFinish || !errCalled {
		t.Fatalf("expected all listeners to fire")
	}
}

type errTest struct{}

func (errTest) Error() string { return "err" }
