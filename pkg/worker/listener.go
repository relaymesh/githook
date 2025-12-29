package worker

import "context"

type Listener struct {
	OnStart         func(ctx context.Context)
	OnExit          func(ctx context.Context)
	OnMessageStart  func(ctx context.Context, evt *Event)
	OnMessageFinish func(ctx context.Context, evt *Event, err error)
	OnError         func(ctx context.Context, evt *Event, err error)
}
