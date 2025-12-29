package worker

import "context"

type RetryDecision struct {
	Retry bool
	Nack  bool
}

type RetryPolicy interface {
	OnError(ctx context.Context, evt *Event, err error) RetryDecision
}

type NoRetry struct{}

func (NoRetry) OnError(ctx context.Context, evt *Event, err error) RetryDecision {
	return RetryDecision{Retry: false, Nack: true}
}
