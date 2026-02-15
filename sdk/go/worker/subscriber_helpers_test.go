package worker

import (
	"testing"

	wmsql "github.com/ThreeDotsLabs/watermill-sql/pkg/sql"
)

func TestUniqueStrings(t *testing.T) {
	values := uniqueStrings([]string{"AMQP", "amqp", "  nats ", ""})
	if len(values) != 2 || values[0] != "amqp" || values[1] != "nats" {
		t.Fatalf("unexpected unique values: %v", values)
	}
}

func TestSubscriberDriverSupported(t *testing.T) {
	if !isSubscriberDriverSupported("amqp") {
		t.Fatalf("expected amqp to be supported")
	}
	if isSubscriberDriverSupported("unknown") {
		t.Fatalf("expected unknown to be unsupported")
	}
}

func TestAMQPSubscriberConfigFromMode(t *testing.T) {
	url := "amqp://localhost"
	cfg, err := amqpSubscriberConfigFromMode(url, "durable_queue")
	if err != nil {
		t.Fatalf("amqp config: %v", err)
	}
	if cfg.Connection.AmqpURI != url || !cfg.Queue.Durable {
		t.Fatalf("unexpected amqp config")
	}
	if _, err := amqpSubscriberConfigFromMode(url, "invalid"); err == nil {
		t.Fatalf("expected unsupported mode error")
	}
}

func TestSQLAdapters(t *testing.T) {
	schema, offsets, err := sqlAdapters("postgres")
	if err != nil {
		t.Fatalf("sql adapters: %v", err)
	}
	if schema == nil || offsets == nil {
		t.Fatalf("expected adapters")
	}
	if _, ok := offsets.(wmsql.DefaultPostgreSQLOffsetsAdapter); !ok {
		t.Fatalf("unexpected offsets adapter type")
	}
	if _, _, err := sqlAdapters("unknown"); err == nil {
		t.Fatalf("expected unsupported dialect error")
	}
}
