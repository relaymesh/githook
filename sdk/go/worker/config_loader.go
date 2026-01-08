package worker

import (
	"os"
	"strings"

	"githooks/pkg/auth"

	"gopkg.in/yaml.v3"
)

// AppConfig is a partial representation of the main application config,
// used for loading worker-specific configuration.
type AppConfig struct {
	Server    ServerConfig     `yaml:"server"`
	Watermill SubscriberConfig `yaml:"watermill"`
	Providers auth.Config      `yaml:"providers"`
	Auth      auth.AuthConfig  `yaml:"auth"`
}

// ServerConfig is a partial representation of the server config for client resolution.
type ServerConfig struct {
	Port          int    `yaml:"port"`
	PublicBaseURL string `yaml:"public_base_url"`
}

// RulesConfig is a partial representation of the rules configuration,
// used for extracting topic names.
type RulesConfig struct {
	Rules []struct {
		Emit emitList `yaml:"emit"`
	} `yaml:"rules"`
}

type emitList []string

func (e *emitList) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		if value.Value == "" {
			*e = nil
			return nil
		}
		*e = emitList{value.Value}
		return nil
	case yaml.SequenceNode:
		out := make([]string, 0, len(value.Content))
		for _, item := range value.Content {
			if item.Kind != yaml.ScalarNode {
				return &yaml.TypeError{Errors: []string{"emit items must be strings"}}
			}
			out = append(out, item.Value)
		}
		*e = emitList(out)
		return nil
	default:
		return &yaml.TypeError{Errors: []string{"emit must be a string or list of strings"}}
	}
}

func (e emitList) Values() []string {
	out := make([]string, 0, len(e))
	for _, val := range e {
		trimmed := strings.TrimSpace(val)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// LoadSubscriberConfig loads the subscriber configuration from a YAML file.
// It expands environment variables and applies default values.
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

// LoadServerConfig loads the server configuration from a YAML file.
func LoadServerConfig(path string) (ServerConfig, error) {
	var cfg AppConfig
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg.Server, err
	}
	expanded := os.ExpandEnv(string(data))
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return cfg.Server, err
	}
	return cfg.Server, nil
}

// LoadProvidersConfig loads provider configuration from a YAML file.
func LoadProvidersConfig(path string) (auth.Config, error) {
	var cfg AppConfig
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg.Providers, err
	}
	expanded := os.ExpandEnv(string(data))
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return cfg.Providers, err
	}
	return cfg.Providers, nil
}

// LoadTopicsFromConfig extracts a unique list of topic names from the 'emit' fields
// in a rules configuration file.
func LoadTopicsFromConfig(path string) ([]string, error) {
	var cfg RulesConfig
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	expanded := os.ExpandEnv(string(data))
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, err
	}
	topics := make([]string, 0, len(cfg.Rules))
	seen := make(map[string]struct{}, len(cfg.Rules))
	for _, rule := range cfg.Rules {
		for _, topic := range rule.Emit.Values() {
			if _, ok := seen[topic]; ok {
				continue
			}
			seen[topic] = struct{}{}
			topics = append(topics, topic)
		}
	}
	return topics, nil
}

func applySubscriberDefaults(cfg *SubscriberConfig) {
	if cfg.Driver == "" && len(cfg.Drivers) == 0 {
		cfg.Driver = "gochannel"
	}
	if cfg.GoChannel.OutputChannelBuffer == 0 {
		cfg.GoChannel.OutputChannelBuffer = 64
	}
	if cfg.NATS.ClientIDSuffix == "" {
		cfg.NATS.ClientIDSuffix = "-worker"
	}
}
