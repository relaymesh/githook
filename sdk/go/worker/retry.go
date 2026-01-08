package worker

import "context"

// RetryDecision defines whether a message should be retried or Nacked.
type RetryDecision struct {
	Retry bool
	Nack  bool
}

// RetryPolicy defines a policy for retrying failed messages.
type RetryPolicy interface {
	OnError(ctx context.Context, evt *Event, err error) RetryDecision
}

// NoRetry is a retry policy that never retries.
type NoRetry struct{}

// OnError always returns a decision to not retry and to Nack the message.
func (NoRetry) OnError(ctx context.Context, evt *Event, err error) RetryDecision {
	return RetryDecision{Retry: false, Nack: true}
}
