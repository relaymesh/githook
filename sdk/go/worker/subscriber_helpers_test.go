package worker

import "testing"

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
