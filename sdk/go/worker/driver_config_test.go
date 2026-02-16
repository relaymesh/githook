package worker

import "testing"

func TestSubscriberConfigFromDriver(t *testing.T) {
	if _, err := SubscriberConfigFromDriver("", "{}"); err == nil {
		t.Fatalf("expected driver required error")
	}
	cfg, err := SubscriberConfigFromDriver("amqp", `{"url":"amqp://localhost"}`)
	if err != nil {
		t.Fatalf("subscriber config: %v", err)
	}
	if cfg.Driver != "amqp" || cfg.AMQP.URL != "amqp://localhost" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestApplyDriverConfigErrors(t *testing.T) {
	if err := applyDriverConfig(nil, "amqp", "{}"); err == nil {
		t.Fatalf("expected nil config error")
	}
	if err := applyDriverConfig(&SubscriberConfig{}, "unknown", "{}"); err == nil {
		t.Fatalf("expected unsupported driver error")
	}
}

func TestUnmarshalJSONEmpty(t *testing.T) {
	cfg := SubscriberConfig{Driver: "amqp"}
	if err := unmarshalJSON("", &cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
