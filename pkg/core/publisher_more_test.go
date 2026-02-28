package core

import (
	"context"
	"errors"
	"testing"

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
	if err := validatePublisherDriver(RelaybusConfig{}, "unknown"); err == nil {
		t.Fatalf("expected unknown driver error")
	}

	if err := ValidatePublisherConfig(RelaybusConfig{Drivers: []string{"kafka"}, Kafka: KafkaConfig{Brokers: []string{"b1:9092"}}}); err != nil {
		t.Fatalf("expected valid kafka config: %v", err)
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
