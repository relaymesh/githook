package drivers

import (
	"encoding/json"
	"testing"

	"githook/pkg/core"
	"githook/pkg/storage"
)

func TestRecordsFromConfig(t *testing.T) {
	cfg := core.WatermillConfig{
		Driver: "amqp",
		AMQP: core.AMQPConfig{
			URL:  "amqp://localhost",
			Mode: "durable_queue",
		},
	}
	records, err := RecordsFromConfig(cfg)
	if err != nil {
		t.Fatalf("records from config: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Name != "amqp" || !records[0].Enabled {
		t.Fatalf("unexpected record: %+v", records[0])
	}
}

func TestRecordsFromConfigUnsupported(t *testing.T) {
	cfg := core.WatermillConfig{
		Drivers: []string{"unsupported"},
	}
	if _, err := RecordsFromConfig(cfg); err == nil {
		t.Fatalf("expected unsupported driver error")
	}
}

func TestConfigFromRecords(t *testing.T) {
	base := core.WatermillConfig{
		AMQP: core.AMQPConfig{URL: "amqp://base"},
	}
	raw, err := json.Marshal(core.AMQPConfig{URL: "amqp://custom"})
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	records := []storage.DriverRecord{
		{
			Name:       "amqp",
			ConfigJSON: string(raw),
			Enabled:    true,
		},
		{
			Name:       "nats",
			ConfigJSON: "{}",
			Enabled:    false,
		},
	}
	cfg, err := ConfigFromRecords(base, records)
	if err != nil {
		t.Fatalf("config from records: %v", err)
	}
	if len(cfg.Drivers) != 1 || cfg.Drivers[0] != "amqp" {
		t.Fatalf("unexpected drivers list: %v", cfg.Drivers)
	}
	if cfg.AMQP.URL != "amqp://custom" {
		t.Fatalf("expected updated amqp url, got %q", cfg.AMQP.URL)
	}
}

func TestMarshalDriverConfigUnsupported(t *testing.T) {
	if _, err := marshalDriverConfig("unknown", core.WatermillConfig{}); err == nil {
		t.Fatalf("expected unsupported driver error")
	}
}

func TestApplyDriverConfigNil(t *testing.T) {
	if err := applyDriverConfig(nil, "amqp", "{}"); err == nil {
		t.Fatalf("expected error for nil config")
	}
}
