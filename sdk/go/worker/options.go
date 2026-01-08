package worker

import "github.com/ThreeDotsLabs/watermill/message"

// Option is a function that configures a Worker.
type Option func(*Worker)

// WithSubscriber sets the Watermill subscriber for the worker.
func WithSubscriber(sub message.Subscriber) Option {
	return func(w *Worker) {
		w.subscriber = sub
	}
}

// WithTopics adds a list of topics for the worker to subscribe to.
func WithTopics(topics ...string) Option {
	return func(w *Worker) {
		for _, topic := range topics {
			if topic == "" {
				continue
			}
			w.topics = append(w.topics, topic)
			w.allowedTopics[topic] = struct{}{}
		}
	}
}

// WithConcurrency sets the number of concurrent message processors.
func WithConcurrency(n int) Option {
	return func(w *Worker) {
		if n > 0 {
			w.concurrency = n
		}
	}
}

// WithCodec sets the codec for decoding messages.
func WithCodec(c Codec) Option {
	return func(w *Worker) {
		if c != nil {
			w.codec = c
		}
	}
}

// WithMiddleware adds middleware to the worker's handler chain.
func WithMiddleware(mw ...Middleware) Option {
	return func(w *Worker) {
		w.middleware = append(w.middleware, mw...)
	}
}

// WithRetry sets the retry policy for the worker.
func WithRetry(policy RetryPolicy) Option {
	return func(w *Worker) {
		if policy != nil {
			w.retry = policy
		}
	}
}

// WithLogger sets the logger for the worker.
func WithLogger(l Logger) Option {
	return func(w *Worker) {
		if l != nil {
			w.logger = l
		}
	}
}

// WithClientProvider sets the client provider for the worker.
func WithClientProvider(provider ClientProvider) Option {
	return func(w *Worker) {
		w.clientProvider = provider
	}
}

// WithListener adds a listener to the worker.
func WithListener(listener Listener) Option {
	return func(w *Worker) {
		w.listeners = append(w.listeners, listener)
	}
}
