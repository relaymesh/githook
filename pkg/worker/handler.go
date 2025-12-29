package worker

import "context"

type Handler func(ctx context.Context, evt *Event) error

type Middleware func(Handler) Handler
