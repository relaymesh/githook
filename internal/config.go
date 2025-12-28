package internal

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type AppConfig struct {
	Server struct {
		Port int `yaml:"port"`
	} `yaml:"server"`
	Providers struct {
		GitHub ProviderConfig `yaml:"github"`
	} `yaml:"providers"`
	Watermill WatermillConfig `yaml:"watermill"`
}

type ProviderConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
	Secret  string `yaml:"secret"`
}

type WatermillConfig struct {
	Driver    string          `yaml:"driver"`
	Drivers   []string        `yaml:"drivers"`
	GoChannel GoChannelConfig `yaml:"gochannel"`
	Kafka     KafkaConfig     `yaml:"kafka"`
	NATS      NATSConfig      `yaml:"nats"`
	AMQP      AMQPConfig      `yaml:"amqp"`
	SQL       SQLConfig       `yaml:"sql"`
	HTTP      HTTPConfig      `yaml:"http"`
}

type GoChannelConfig struct {
	OutputChannelBuffer            int64 `yaml:"output_buffer"`
	Persistent                     bool  `yaml:"persistent"`
	BlockPublishUntilSubscriberAck bool  `yaml:"block_publish_until_subscriber_ack"`
}

type KafkaConfig struct {
	Brokers []string `yaml:"brokers"`
}

type NATSConfig struct {
	ClusterID string `yaml:"cluster_id"`
	ClientID  string `yaml:"client_id"`
	URL       string `yaml:"url"`
}

type AMQPConfig struct {
	URL  string `yaml:"url"`
	Mode string `yaml:"mode"`
}

type SQLConfig struct {
	Driver               string `yaml:"driver"`
	DSN                  string `yaml:"dsn"`
	Dialect              string `yaml:"dialect"`
	AutoInitializeSchema bool   `yaml:"auto_initialize_schema"`
}

type HTTPConfig struct {
	BaseURL string `yaml:"base_url"`
	Mode    string `yaml:"mode"`
}

type RulesConfig struct {
	Rules []Rule `yaml:"rules"`
}

func LoadAppConfig(path string) (AppConfig, error) {
	var cfg AppConfig
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}

	expanded := os.ExpandEnv(string(data))
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return cfg, err
	}

	applyDefaults(&cfg)
	return cfg, nil
}

func LoadRulesConfig(path string) (RulesConfig, error) {
	var cfg RulesConfig
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}

	for i := range cfg.Rules {
		cfg.Rules[i].When = strings.TrimSpace(cfg.Rules[i].When)
		cfg.Rules[i].Emit = strings.TrimSpace(cfg.Rules[i].Emit)
		if cfg.Rules[i].When == "" || cfg.Rules[i].Emit == "" {
			return cfg, fmt.Errorf("rule %d is missing when or emit", i)
		}
		if len(cfg.Rules[i].Drivers) > 0 {
			drivers := make([]string, 0, len(cfg.Rules[i].Drivers))
			for _, driver := range cfg.Rules[i].Drivers {
				trimmed := strings.TrimSpace(driver)
				if trimmed != "" {
					drivers = append(drivers, trimmed)
				}
			}
			cfg.Rules[i].Drivers = drivers
		}
	}

	return cfg, nil
}

func applyDefaults(cfg *AppConfig) {
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Providers.GitHub.Path == "" {
		cfg.Providers.GitHub.Path = "/webhooks/github"
	}
	if cfg.Watermill.Driver == "" {
		cfg.Watermill.Driver = "gochannel"
	}
	if cfg.Watermill.GoChannel.OutputChannelBuffer == 0 {
		cfg.Watermill.GoChannel.OutputChannelBuffer = 64
	}
	if cfg.Watermill.HTTP.Mode == "" {
		cfg.Watermill.HTTP.Mode = "topic_url"
	}
}
