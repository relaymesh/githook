package core

import (
	"context"
	"errors"
	"testing"
	"time"

	relaymessage "github.com/relaymesh/relaybus/sdk/core/go/message"
)

type errPublisher struct {
	err error
}

func (e *errPublisher) Publish(context.Context, string, Event) error { return e.err }
func (e *errPublisher) PublishForDrivers(ctx context.Context, topic string, event Event, drivers []string) error {
	return e.Publish(ctx, topic, event)
}
func (e *errPublisher) Close() error { return e.err }

func TestValidatePublisherDriverAndConfig(t *testing.T) {
	if err := ValidatePublisherConfig(RelaybusConfig{}); err != nil {
		t.Fatalf("empty config should validate: %v", err)
	}

	if err := validatePublisherDriver(RelaybusConfig{}, "amqp"); err == nil {
		t.Fatalf("expected amqp url required")
	}
	if err := validatePublisherDriver(RelaybusConfig{}, "nats"); err == nil {
		t.Fatalf("expected nats url required")
	}
	if err := validatePublisherDriver(RelaybusConfig{}, "kafka"); err == nil {
		t.Fatalf("expected kafka brokers required")
	}
	if err := validatePublisherDriver(RelaybusConfig{Kafka: KafkaConfig{Broker: "localhost:9092"}}, "kafka"); err != nil {
		t.Fatalf("expected kafka broker fallback validation success: %v", err)
	}
	if err := validatePublisherDriver(RelaybusConfig{}, "http"); err == nil {
		t.Fatalf("expected http endpoint required")
	}
	if err := validatePublisherDriver(RelaybusConfig{HTTP: HTTPConfig{Endpoint: "http://localhost:8088/{topic}"}}, "http"); err != nil {
		t.Fatalf("expected valid http config: %v", err)
	}
	if err := validatePublisherDriver(RelaybusConfig{}, "unknown"); err == nil {
		t.Fatalf("expected unknown driver error")
	}

	if err := ValidatePublisherConfig(RelaybusConfig{Drivers: []string{"kafka"}, Kafka: KafkaConfig{Brokers: []string{"b1:9092"}}}); err != nil {
		t.Fatalf("expected valid kafka config: %v", err)
	}
	if err := ValidatePublisherConfig(RelaybusConfig{Drivers: []string{"http"}, HTTP: HTTPConfig{Endpoint: "http://localhost:8088/{topic}"}}); err != nil {
		t.Fatalf("expected valid http config: %v", err)
	}
}

func TestRelaybusRetryPolicyDefaults(t *testing.T) {
	policy := relaybusRetryPolicy(PublishRetryConfig{Attempts: 0, DelayMS: -1})
	if policy.MaxAttempts != 1 {
		t.Fatalf("expected default max attempts=1, got %d", policy.MaxAttempts)
	}
	if policy.BaseDelay < 0 || policy.MaxDelay < 0 {
		t.Fatalf("expected non-negative delays")
	}
}

func TestDriverRetryConfig(t *testing.T) {
	base := RelaybusConfig{
		PublishRetry: PublishRetryConfig{Attempts: 3, DelayMS: 50},
		AMQP:         AMQPConfig{RetryCount: 7},
		NATS:         NATSConfig{RetryCount: 6},
		Kafka:        KafkaConfig{RetryCount: 5},
		HTTP:         HTTPConfig{RetryCount: 4},
	}
	if got := driverRetryConfig(base, "amqp"); got.Attempts != 7 {
		t.Fatalf("expected amqp retry override, got %+v", got)
	}
	if got := driverRetryConfig(base, "nats"); got.Attempts != 6 {
		t.Fatalf("expected nats retry override, got %+v", got)
	}
	if got := driverRetryConfig(base, "kafka"); got.Attempts != 5 {
		t.Fatalf("expected kafka retry override, got %+v", got)
	}
	if got := driverRetryConfig(base, "http"); got.Attempts != 4 {
		t.Fatalf("expected http retry override, got %+v", got)
	}
	if got := driverRetryConfig(base, "unknown"); got.Attempts != 3 {
		t.Fatalf("expected base retry for unknown, got %+v", got)
	}
}

func TestRetryingPublisher(t *testing.T) {
	t.Run("eventual success", func(t *testing.T) {
		calls := 0
		base := &publisherRecorder{delegate: &stubPublisher{}, onPublish: func() { calls++ }}
		r := &retryingPublisher{
			base: Publisher(&publisherRecorder{delegate: PublisherFunc(func(ctx context.Context, topic string, event Event) error {
				calls++
				if calls < 3 {
					return errors.New("fail")
				}
				return nil
			})}),
			attempts: 3,
			delay:    0,
		}
		_ = base
		if err := r.Publish(context.Background(), "topic", Event{}); err != nil {
			t.Fatalf("expected eventual success, got %v", err)
		}
	})

	t.Run("context cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		r := &retryingPublisher{base: &errPublisher{err: errors.New("always")}, attempts: 3, delay: time.Second}
		if err := r.Publish(ctx, "topic", Event{}); err == nil {
			t.Fatalf("expected canceled error")
		}
	})
}

type PublisherFunc func(ctx context.Context, topic string, event Event) error

func (f PublisherFunc) Publish(ctx context.Context, topic string, event Event) error {
	return f(ctx, topic, event)
}

func (f PublisherFunc) PublishForDrivers(ctx context.Context, topic string, event Event, drivers []string) error {
	return f(ctx, topic, event)
}

func (f PublisherFunc) Close() error { return nil }

func TestPublisherMuxUnknownDriverAndDLQ(t *testing.T) {
	primaryErr := errors.New("publish failed")
	primary := &errPublisher{err: primaryErr}
	dlqCalled := 0
	dlq := &stubPublisher{}

	mux := &publisherMux{
		publishers: map[string]Publisher{
			"primary": primary,
			"dlq":     Publisher(&publisherRecorder{onPublish: func() { dlqCalled++ }, delegate: dlq}),
		},
		defaultDrivers: []string{"primary", "missing"},
		dlqDriver:      "dlq",
	}

	err := mux.Publish(context.Background(), "topic", Event{Provider: "github"})
	if err == nil {
		t.Fatalf("expected aggregated publish error")
	}
	if dlqCalled != 1 {
		t.Fatalf("expected dlq publish once, got %d", dlqCalled)
	}
}

func TestPublisherMuxCloseAggregates(t *testing.T) {
	errA := errors.New("close-a")
	errB := errors.New("close-b")
	mux := &publisherMux{publishers: map[string]Publisher{
		"a": &errPublisher{err: errA},
		"b": &errPublisher{err: errB},
	}}
	err := mux.Close()
	if err == nil {
		t.Fatalf("expected close error")
	}
}

func TestRelaybusPublisherCloseNil(t *testing.T) {
	pub := &relaybusPublisher{}
	if err := pub.Close(); err != nil {
		t.Fatalf("close nil publisher should be nil: %v", err)
	}
}

type publisherRecorder struct {
	onPublish func()
	delegate  Publisher
}

func (p *publisherRecorder) Publish(ctx context.Context, topic string, event Event) error {
	if p.onPublish != nil {
		p.onPublish()
	}
	if p.delegate != nil {
		return p.delegate.Publish(ctx, topic, event)
	}
	return nil
}

func (p *publisherRecorder) PublishForDrivers(ctx context.Context, topic string, event Event, drivers []string) error {
	return p.Publish(ctx, topic, event)
}

func (p *publisherRecorder) Close() error {
	if p.delegate != nil {
		return p.delegate.Close()
	}
	return nil
}

type relayNoopPublisher struct{}

func (relayNoopPublisher) Publish(context.Context, string, relaymessage.Message) error { return nil }
func (relayNoopPublisher) PublishBatch(context.Context, string, []relaymessage.Message) error {
	return nil
}
func (relayNoopPublisher) Close() error { return nil }
