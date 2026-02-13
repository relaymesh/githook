package worker

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

// NewFromConfigPath creates a worker from a config file and applies options.
func NewFromConfigPath(path string, opts ...Option) (*Worker, error) {
	return newFromConfigPath(path, "", opts...)
}

// NewFromConfigPathWithDriver creates a worker from a config file and overrides the subscriber driver.
func NewFromConfigPathWithDriver(path, driver string, opts ...Option) (*Worker, error) {
	return newFromConfigPath(path, driver, opts...)
}

// NewFromConfigPathWithDriverFromAPI creates a worker using driver config from the server API.
func NewFromConfigPathWithDriverFromAPI(path, driver string, opts ...Option) (*Worker, error) {
	driver = strings.TrimSpace(driver)
	if driver == "" {
		return nil, errors.New("driver is required")
	}
	providers, err := LoadProvidersConfig(path)
	if err != nil {
		return nil, err
	}
	cfg, err := subscriberConfigFromAPI(context.Background(), driver)
	if err != nil {
		return nil, err
	}
	sub, err := BuildSubscriber(cfg)
	if err != nil {
		return nil, err
	}
	opts = append(opts,
		WithSubscriber(sub),
		WithClientProvider(NewSCMClientProvider(providers)),
	)
	return New(opts...), nil
}

func newFromConfigPath(path, driver string, opts ...Option) (*Worker, error) {
	cfg, err := LoadSubscriberConfig(path)
	if err != nil {
		return nil, err
	}
	if trimmed := strings.TrimSpace(driver); trimmed != "" {
		cfg.Driver = trimmed
		cfg.Drivers = nil
	}
	sub, err := BuildSubscriber(cfg)
	if err != nil {
		return nil, err
	}
	providers, err := LoadProvidersConfig(path)
	if err != nil {
		return nil, err
	}
	opts = append(opts,
		WithSubscriber(sub),
		WithClientProvider(NewSCMClientProvider(providers)),
	)
	return New(opts...), nil
}

func subscriberConfigFromAPI(ctx context.Context, driver string) (SubscriberConfig, error) {
	client := &DriversClient{BaseURL: installationsBaseURL()}
	record, err := client.GetDriver(ctx, driver)
	if err != nil {
		return SubscriberConfig{}, err
	}
	if record == nil {
		return SubscriberConfig{}, fmt.Errorf("driver not found: %s", driver)
	}
	if !record.Enabled {
		return SubscriberConfig{}, fmt.Errorf("driver is disabled: %s", driver)
	}
	return subscriberConfigFromDriver(record.Name, record.ConfigJSON)
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
	if err := w.validateTopics(ctx); err != nil {
		return err
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
	logID := ""
	if msg != nil {
		logID = msg.Metadata.Get("log_id")
	}
	evt, err := w.codec.Decode(topic, msg)
	if err != nil {
		w.logger.Printf("decode failed: %v", err)
		w.updateEventLogStatus(ctx, logID, EventLogStatusFailed, err)
		w.notifyError(ctx, nil, err)
		decision := w.retry.OnError(ctx, nil, err)
		if decision.Retry || decision.Nack {
			msg.Nack()
			return
		}
		msg.Ack()
		return
	}

	w.updateEventLogStatus(ctx, logID, EventLogStatusDelivered, nil)

	if w.clientProvider != nil {
		client, err := w.clientProvider.Client(ctx, evt)
		if err != nil {
			w.logger.Printf("client init failed: %v", err)
			w.updateEventLogStatus(ctx, logID, EventLogStatusFailed, err)
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
		w.updateEventLogStatus(ctx, logID, EventLogStatusSuccess, nil)
		msg.Ack()
		return
	}

	wrapped := w.wrap(handler)
	if err := wrapped(ctx, evt); err != nil {
		w.notifyMessageFinish(ctx, evt, err)
		w.notifyError(ctx, evt, err)
		w.updateEventLogStatus(ctx, logID, EventLogStatusFailed, err)
		decision := w.retry.OnError(ctx, evt, err)
		if decision.Retry || decision.Nack {
			msg.Nack()
			return
		}
		msg.Ack()
		return
	}
	w.notifyMessageFinish(ctx, evt, nil)
	w.updateEventLogStatus(ctx, logID, EventLogStatusSuccess, nil)
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

func (w *Worker) validateTopics(ctx context.Context) error {
	client := &RulesClient{BaseURL: installationsBaseURL()}
	topics, err := client.ListRuleTopics(ctx)
	if err != nil {
		return err
	}
	if len(topics) == 0 {
		return errors.New("no topics available from rules")
	}
	allowed := make(map[string]struct{}, len(topics))
	for _, topic := range topics {
		allowed[topic] = struct{}{}
	}
	for _, topic := range unique(w.topics) {
		if _, ok := allowed[topic]; !ok {
			return fmt.Errorf("unknown topic: %s", topic)
		}
	}
	return nil
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
