package worker

import "github.com/ThreeDotsLabs/watermill/message"

type Option func(*Worker)

func WithSubscriber(sub message.Subscriber) Option {
	return func(w *Worker) {
		w.subscriber = sub
	}
}

func WithTopics(topics ...string) Option {
	return func(w *Worker) {
		w.topics = append(w.topics, topics...)
	}
}

func WithConcurrency(n int) Option {
	return func(w *Worker) {
		if n > 0 {
			w.concurrency = n
		}
	}
}

func WithCodec(c Codec) Option {
	return func(w *Worker) {
		if c != nil {
			w.codec = c
		}
	}
}

func WithMiddleware(mw ...Middleware) Option {
	return func(w *Worker) {
		w.middleware = append(w.middleware, mw...)
	}
}

func WithRetry(policy RetryPolicy) Option {
	return func(w *Worker) {
		if policy != nil {
			w.retry = policy
		}
	}
}

func WithLogger(l Logger) Option {
	return func(w *Worker) {
		if l != nil {
			w.logger = l
		}
	}
}

func WithClientProvider(provider ClientProvider) Option {
	return func(w *Worker) {
		w.clientProvider = provider
	}
}

func WithListener(listener Listener) Option {
	return func(w *Worker) {
		w.listeners = append(w.listeners, listener)
	}
}
