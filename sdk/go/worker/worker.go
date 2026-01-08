package worker

import (
	"context"
	"errors"
	"sync"

	"github.com/ThreeDotsLabs/watermill/message"
)

// Worker is a message-processing worker that subscribes to topics, decodes
// messages, and dispatches them to handlers.
type Worker struct {
	subscriber  message.Subscriber
	codec       Codec
	retry       RetryPolicy
	logger      Logger
	concurrency int
	topics      []string

	topicHandlers  map[string]Handler
	typeHandlers   map[string]Handler
	middleware     []Middleware
	clientProvider ClientProvider
	listeners      []Listener
	allowedTopics  map[string]struct{}
}

// New creates a new Worker with the given options.
func New(opts ...Option) *Worker {
	w := &Worker{
		codec:         DefaultCodec{},
		retry:         NoRetry{},
		logger:        stdLogger{},
		concurrency:   1,
		topicHandlers: make(map[string]Handler),
		typeHandlers:  make(map[string]Handler),
		allowedTopics: make(map[string]struct{}),
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// HandleTopic registers a handler for a specific topic.
func (w *Worker) HandleTopic(topic string, h Handler) {
	if h == nil || topic == "" {
		return
	}
	if len(w.allowedTopics) > 0 {
		if _, ok := w.allowedTopics[topic]; !ok {
			w.logger.Printf("handler topic not subscribed: %s", topic)
			return
		}
	}
	w.topicHandlers[topic] = h
	w.topics = append(w.topics, topic)
}

// HandleType registers a handler for a specific event type.
func (w *Worker) HandleType(eventType string, h Handler) {
	if h == nil || eventType == "" {
		return
	}
	w.typeHandlers[eventType] = h
}

// Run starts the worker, subscribing to topics and processing messages.
// It blocks until the context is canceled.
func (w *Worker) Run(ctx context.Context) error {
	if w.subscriber == nil {
		return errors.New("subscriber is required")
	}
	if len(w.topics) == 0 {
		return errors.New("at least one topic is required")
	}

	topics := unique(w.topics)
	w.notifyStart(ctx)
	defer w.notifyExit(ctx)
	sem := make(chan struct{}, w.concurrency)

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for _, topic := range topics {
		msgs, err := w.subscriber.Subscribe(ctx, topic)
		if err != nil {
			w.notifyError(ctx, nil, err)
			return err
		}
		wg.Add(1)
		go func(topic string, ch <-chan *message.Message) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case msg, ok := <-ch:
					if !ok {
						return
					}
					sem <- struct{}{}
					wg.Add(1)
					go func(msg *message.Message) {
						defer wg.Done()
						defer func() { <-sem }()
						w.handleMessage(ctx, topic, msg)
					}(msg)
				}
			}
		}(topic, msgs)
	}

	<-ctx.Done()
	wg.Wait()
	return nil
}

// Close gracefully shuts down the worker and its subscriber.
func (w *Worker) Close() error {
	if w.subscriber == nil {
		return nil
	}
	return w.subscriber.Close()
}

func (w *Worker) handleMessage(ctx context.Context, topic string, msg *message.Message) {
	evt, err := w.codec.Decode(topic, msg)
	if err != nil {
		w.logger.Printf("decode failed: %v", err)
		w.notifyError(ctx, nil, err)
		decision := w.retry.OnError(ctx, nil, err)
		if decision.Retry || decision.Nack {
			msg.Nack()
			return
		}
		msg.Ack()
		return
	}

	if w.clientProvider != nil {
		client, err := w.clientProvider.Client(ctx, evt)
		if err != nil {
			w.logger.Printf("client init failed: %v", err)
			w.notifyError(ctx, evt, err)
			decision := w.retry.OnError(ctx, evt, err)
			if decision.Retry || decision.Nack {
				msg.Nack()
				return
			}
			msg.Ack()
			return
		}
		evt.Client = client
	}

	if reqID := evt.Metadata["request_id"]; reqID != "" {
		w.logger.Printf("request_id=%s topic=%s provider=%s type=%s", reqID, evt.Topic, evt.Provider, evt.Type)
	}

	w.notifyMessageStart(ctx, evt)

	handler := w.topicHandlers[topic]
	if handler == nil {
		handler = w.typeHandlers[evt.Type]
	}
	if handler == nil {
		w.logger.Printf("no handler for topic=%s type=%s", topic, evt.Type)
		w.notifyMessageFinish(ctx, evt, nil)
		msg.Ack()
		return
	}

	wrapped := w.wrap(handler)
	if err := wrapped(ctx, evt); err != nil {
		w.notifyMessageFinish(ctx, evt, err)
		w.notifyError(ctx, evt, err)
		decision := w.retry.OnError(ctx, evt, err)
		if decision.Retry || decision.Nack {
			msg.Nack()
			return
		}
		msg.Ack()
		return
	}
	w.notifyMessageFinish(ctx, evt, nil)
	msg.Ack()
}

func (w *Worker) wrap(h Handler) Handler {
	wrapped := h
	for i := len(w.middleware) - 1; i >= 0; i-- {
		wrapped = w.middleware[i](wrapped)
	}
	return wrapped
}

func unique(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func (w *Worker) notifyStart(ctx context.Context) {
	for _, listener := range w.listeners {
		if listener.OnStart != nil {
			listener.OnStart(ctx)
		}
	}
}

func (w *Worker) notifyExit(ctx context.Context) {
	for _, listener := range w.listeners {
		if listener.OnExit != nil {
			listener.OnExit(ctx)
		}
	}
}

func (w *Worker) notifyMessageStart(ctx context.Context, evt *Event) {
	for _, listener := range w.listeners {
		if listener.OnMessageStart != nil {
			listener.OnMessageStart(ctx, evt)
		}
	}
}

func (w *Worker) notifyMessageFinish(ctx context.Context, evt *Event, err error) {
	for _, listener := range w.listeners {
		if listener.OnMessageFinish != nil {
			listener.OnMessageFinish(ctx, evt, err)
		}
	}
}

func (w *Worker) notifyError(ctx context.Context, evt *Event, err error) {
	for _, listener := range w.listeners {
		if listener.OnError != nil {
			listener.OnError(ctx, evt, err)
		}
	}
}
