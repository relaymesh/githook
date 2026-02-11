package worker

import (
	"encoding/json"
	"errors"
	"strings"
)

func subscriberConfigFromDriver(driver, raw string) (SubscriberConfig, error) {
	driver = strings.ToLower(strings.TrimSpace(driver))
	if driver == "" {
		return SubscriberConfig{}, errors.New("driver is required")
	}
	cfg := SubscriberConfig{Driver: driver}
	if err := applyDriverConfig(&cfg, driver, raw); err != nil {
		return SubscriberConfig{}, err
	}
	return cfg, nil
}

func applyDriverConfig(cfg *SubscriberConfig, name, raw string) error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "gochannel":
		return unmarshalJSON(raw, &cfg.GoChannel)
	case "amqp":
		return unmarshalJSON(raw, &cfg.AMQP)
	case "nats":
		return unmarshalJSON(raw, &cfg.NATS)
	case "kafka":
		return unmarshalJSON(raw, &cfg.Kafka)
	case "sql":
		return unmarshalJSON(raw, &cfg.SQL)
	default:
		return errors.New("unsupported driver: " + name)
	}
}

func unmarshalJSON(raw string, target interface{}) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	return json.Unmarshal([]byte(raw), target)
}
