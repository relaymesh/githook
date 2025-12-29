package worker

import (
	"os"

	"gopkg.in/yaml.v3"
)

type AppConfig struct {
	Watermill SubscriberConfig `yaml:"watermill"`
}

func LoadSubscriberConfig(path string) (SubscriberConfig, error) {
	var cfg AppConfig
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg.Watermill, err
	}
	expanded := os.ExpandEnv(string(data))
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return cfg.Watermill, err
	}
	applySubscriberDefaults(&cfg.Watermill)
	return cfg.Watermill, nil
}

func applySubscriberDefaults(cfg *SubscriberConfig) {
	if cfg.Driver == "" && len(cfg.Drivers) == 0 {
		cfg.Driver = "gochannel"
	}
	if cfg.GoChannel.OutputChannelBuffer == 0 {
		cfg.GoChannel.OutputChannelBuffer = 64
	}
}
