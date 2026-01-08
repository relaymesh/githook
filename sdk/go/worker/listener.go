package worker

import "context"

// Listener provides hooks into the worker's lifecycle for logging, metrics, etc.
type Listener struct {
	// OnStart is called when the worker starts.
	OnStart func(ctx context.Context)
	// OnExit is called when the worker exits.
	OnExit func(ctx context.Context)
	// OnMessageStart is called when a message is received.
	OnMessageStart func(ctx context.Context, evt *Event)
	// OnMessageFinish is called when a message has been processed.
	OnMessageFinish func(ctx context.Context, evt *Event, err error)
	// OnError is called when an error occurs.
	OnError func(ctx context.Context, evt *Event, err error)
}
