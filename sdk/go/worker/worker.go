package worker

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/ThreeDotsLabs/watermill/message"

	"githook/pkg/auth"
	"githook/pkg/storage"
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

	topicHandlers   map[string]Handler
	topicDrivers    map[string]string
	typeHandlers    map[string]Handler
	middleware      []Middleware
	clientProvider  ClientProvider
	listeners       []Listener
	allowedTopics   map[string]struct{}
	driverSubs      map[string]message.Subscriber
	endpoint        string
	apiKey          string
	oauth2Config    *auth.OAuth2Config
	defaultDriverID string
	tenantID        string
}

// New creates a new Worker with the given options.
func New(opts ...Option) *Worker {
	w := &Worker{
		codec:         DefaultCodec{},
		retry:         NoRetry{},
		logger:        stdLogger{},
		concurrency:   1,
		topicHandlers: make(map[string]Handler),
		topicDrivers:  make(map[string]string),
		typeHandlers:  make(map[string]Handler),
		allowedTopics: make(map[string]struct{}),
		driverSubs:    make(map[string]message.Subscriber),
		tenantID:      envTenantID(),
	}
	for _, opt := range opts {
		opt(w)
	}
	log.Printf("worker tenant context=%q", w.tenantIDValue())
	w.bindClientProvider()
	return w
}

// bindClientProvider propagates the worker's resolved endpoint, API key, and
// OAuth2 config into the SCMClientProvider so callers never have to duplicate
// those values.
func (w *Worker) bindClientProvider() {
	if p, ok := w.clientProvider.(*SCMClientProvider); ok {
		p.bindInstallationsClient(w.installationsClient())
	}
}

// HandleTopic registers a handler for a specific topic and driver.
func (w *Worker) HandleTopic(topic, driverID string, h Handler) {
	if h == nil || topic == "" {
		return
	}
	if len(w.allowedTopics) > 0 {
		if _, ok := w.allowedTopics[topic]; !ok {
			w.logger.Printf("handler topic not subscribed: %s", topic)
			return
		}
	}
	driverID = strings.TrimSpace(driverID)
	if driverID == "" {
		driverID = strings.TrimSpace(w.defaultDriverID)
	}
	if driverID == "" && w.subscriber == nil {
		w.logger.Printf("driver id required for topic: %s", topic)
		return
	}
	w.topicHandlers[topic] = h
	if driverID != "" {
		w.topicDrivers[topic] = driverID
	}
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
	if len(w.topics) == 0 {
		return errors.New("at least one topic is required")
	}
	if w.subscriber != nil {
		if err := w.validateTopics(ctx); err != nil {
			return err
		}
		return w.runWithSubscriber(ctx, w.subscriber, unique(w.topics))
	}

	driverTopics, err := w.topicsByDriver()
	if err != nil {
		return err
	}
	if err := w.validateTopics(ctx); err != nil {
		return err
	}
	if err := w.buildDriverSubscribers(ctx, driverTopics); err != nil {
		return err
	}

	w.notifyStart(ctx)
	defer w.notifyExit(ctx)
	sem := make(chan struct{}, w.concurrency)

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for driverID, topics := range driverTopics {
		sub := w.driverSubs[driverID]
		if sub == nil {
			return fmt.Errorf("subscriber not initialized for driver: %s", driverID)
		}
		for _, topic := range unique(topics) {
			msgs, err := sub.Subscribe(ctx, topic)
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
	}

	<-ctx.Done()
	wg.Wait()
	return nil
}

func (w *Worker) runWithSubscriber(ctx context.Context, sub message.Subscriber, topics []string) error {
	if sub == nil {
		return errors.New("subscriber is required")
	}

	w.notifyStart(ctx)
	defer w.notifyExit(ctx)
	sem := make(chan struct{}, w.concurrency)

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for _, topic := range topics {
		msgs, err := sub.Subscribe(ctx, topic)
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

func (w *Worker) topicsByDriver() (map[string][]string, error) {
	if len(w.topicDrivers) == 0 {
		if defaultDriver := strings.TrimSpace(w.defaultDriverID); defaultDriver != "" {
			return map[string][]string{defaultDriver: unique(w.topics)}, nil
		}
		return nil, errors.New("driver id is required for topics")
	}
	out := make(map[string][]string, len(w.topicDrivers))
	for topic, driverID := range w.topicDrivers {
		trimmed := strings.TrimSpace(driverID)
		if trimmed == "" {
			return nil, fmt.Errorf("driver id is required for topic: %s", topic)
		}
		out[trimmed] = append(out[trimmed], topic)
	}
	return out, nil
}

func (w *Worker) buildDriverSubscribers(ctx context.Context, driverTopics map[string][]string) error {
	for driverID := range driverTopics {
		if _, ok := w.driverSubs[driverID]; ok {
			continue
		}
		tenantID := w.tenantIDValue()
		log.Printf("worker building driver subscriber driver=%s tenant=%s", driverID, tenantID)
		tenantCtx := storage.WithTenant(ctx, tenantID)
		record, err := w.driversClient().GetDriverByID(tenantCtx, driverID)
		if err != nil {
			return err
		}
		if record == nil {
			return fmt.Errorf("driver not found: %s", driverID)
		}
		if !record.Enabled {
			return fmt.Errorf("driver is disabled: %s", driverID)
		}
		cfg, err := subscriberConfigFromDriver(record.Name, record.ConfigJSON)
		if err != nil {
			return err
		}
		sub, err := BuildSubscriber(cfg)
		if err != nil {
			return err
		}
		w.driverSubs[driverID] = sub
	}
	return nil
}

// Close gracefully shuts down the worker and its subscriber.
func (w *Worker) Close() error {
	if w.subscriber == nil {
		for _, sub := range w.driverSubs {
			if sub == nil {
				continue
			}
			if err := sub.Close(); err != nil {
				return err
			}
		}
		return nil
	}
	return w.subscriber.Close()
}

func (w *Worker) handleMessage(ctx context.Context, topic string, msg *message.Message) {
	logID := ""
	if msg != nil {
		logID = msg.Metadata.Get(MetadataKeyLogID)
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

	if reqID := evt.Metadata[MetadataKeyRequestID]; reqID != "" {
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
	client := w.rulesClient()
	rules, err := client.ListRules(ctx)
	if err != nil {
		return err
	}
	if len(rules) == 0 {
		return errors.New("no rules available from api")
	}

	allowedTopics := map[string]struct{}{}
	allowedByDriver := map[string]map[string]struct{}{}
	for _, record := range rules {
		for _, topic := range record.Emit {
			trimmed := strings.TrimSpace(topic)
			if trimmed == "" {
				continue
			}
			allowedTopics[trimmed] = struct{}{}
			driverID := strings.TrimSpace(record.DriverID)
			if driverID == "" {
				continue
			}
			if _, ok := allowedByDriver[driverID]; !ok {
				allowedByDriver[driverID] = map[string]struct{}{}
			}
			allowedByDriver[driverID][trimmed] = struct{}{}
		}
	}
	if len(allowedTopics) == 0 {
		return errors.New("no topics available from rules")
	}

	topics := unique(w.topics)
	if w.subscriber != nil {
		for _, topic := range topics {
			if _, ok := allowedTopics[topic]; !ok {
				return fmt.Errorf("unknown topic: %s", topic)
			}
		}
		return nil
	}

	for _, topic := range topics {
		driverID := strings.TrimSpace(w.topicDrivers[topic])
		if driverID == "" {
			driverID = strings.TrimSpace(w.defaultDriverID)
		}
		if driverID == "" {
			return fmt.Errorf("driver id is required for topic: %s", topic)
		}
		allowed := allowedByDriver[driverID]
		if allowed == nil {
			return fmt.Errorf("driver not configured on any rule: %s", driverID)
		}
		if _, ok := allowed[topic]; !ok {
			return fmt.Errorf("topic %s not configured for driver %s", topic, driverID)
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
