package worker

import "context"

// Handler is a function that processes an event.
type Handler func(ctx context.Context, evt *Event) error

// Middleware is a function that wraps a handler to add functionality.
type Middleware func(Handler) Handler
