package worker

import (
	"encoding/json"
	"errors"
	"strings"
)

func SubscriberConfigFromDriver(driver, raw string) (SubscriberConfig, error) {
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

// ValidateSubscriber builds and closes a subscriber from the driver JSON to ensure
// the configuration is usable.
func ValidateSubscriber(driver, raw string) error {
	cfg, err := SubscriberConfigFromDriver(driver, raw)
	if err != nil {
		return err
	}
	sub, err := BuildSubscriber(cfg)
	if err != nil {
		return err
	}
	return sub.Close()
}

func applyDriverConfig(cfg *SubscriberConfig, name, raw string) error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "amqp":
		return unmarshalJSON(raw, &cfg.AMQP)
	case "nats":
		return unmarshalJSON(raw, &cfg.NATS)
	case "kafka":
		return unmarshalJSON(raw, &cfg.Kafka)
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
