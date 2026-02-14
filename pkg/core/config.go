package core

import (
	"fmt"
	"log"
	"os"
	"strings"

	"githook/pkg/auth"

	"gopkg.in/yaml.v3"
)

// AppConfig represents the main application configuration.
type AppConfig struct {
	// Server holds server-specific configuration.
	Server struct {
		Port           int    `yaml:"port"`
		PublicBaseURL  string `yaml:"public_base_url"` // Deprecated: use Endpoint.
		ReadTimeoutMS  int64  `yaml:"read_timeout_ms"`
		WriteTimeoutMS int64  `yaml:"write_timeout_ms"`
		IdleTimeoutMS  int64  `yaml:"idle_timeout_ms"`
		ReadHeaderMS   int64  `yaml:"read_header_timeout_ms"`
		MaxBodyBytes   int64  `yaml:"max_body_bytes"`
		DebugEvents    bool   `yaml:"debug_events"`
	} `yaml:"server"`
	// Providers contains configuration for each Git provider.
	Providers auth.Config `yaml:"providers"`
	// Watermill holds configuration for the message router.
	Watermill WatermillConfig `yaml:"watermill"`
	// Storage holds configuration for installation storage.
	Storage StorageConfig `yaml:"storage"`
	// RedirectBaseURL is where users are redirected after OAuth completion.
	RedirectBaseURL string `yaml:"redirect_base_url"`
	// OAuth holds legacy callback configuration for provider integrations.
	OAuth OAuthConfig `yaml:"oauth"`
	// Auth holds API authentication configuration.
	Auth auth.AuthConfig `yaml:"auth"`
	// Endpoint is the base URL for Connect RPC calls.
	Endpoint string `yaml:"endpoint"`
}

// Config represents the application configuration including rules.
type Config struct {
	AppConfig   `yaml:",inline"`
	Rules       []Rule `yaml:"rules"`
	RulesStrict bool   `yaml:"rules_strict"`
}

// ProviderConfig represents the configuration for a single Git provider.
type ProviderConfig = auth.ProviderConfig

// WatermillConfig holds the configuration for Watermill, which handles messaging.
type WatermillConfig struct {
	Driver       string             `yaml:"driver"`
	Drivers      []string           `yaml:"drivers"`
	GoChannel    GoChannelConfig    `yaml:"gochannel"`
	Kafka        KafkaConfig        `yaml:"kafka"`
	NATS         NATSConfig         `yaml:"nats"`
	AMQP         AMQPConfig         `yaml:"amqp"`
	SQL          SQLConfig          `yaml:"sql"`
	HTTP         HTTPConfig         `yaml:"http"`
	RiverQueue   RiverQueueConfig   `yaml:"riverqueue"`
	PublishRetry PublishRetryConfig `yaml:"publish_retry"`
	DLQDriver    string             `yaml:"dlq_driver"`
}

// GoChannelConfig holds configuration for the GoChannel pub/sub.
type GoChannelConfig struct {
	OutputChannelBuffer            int64 `yaml:"output_buffer"`
	Persistent                     bool  `yaml:"persistent"`
	BlockPublishUntilSubscriberAck bool  `yaml:"block_publish_until_subscriber_ack"`
}

// KafkaConfig holds configuration for the Kafka pub/sub.
type KafkaConfig struct {
	Brokers []string `yaml:"brokers"`
}

// NATSConfig holds configuration for the NATS pub/sub.
type NATSConfig struct {
	ClusterID string `yaml:"cluster_id"`
	ClientID  string `yaml:"client_id"`
	URL       string `yaml:"url"`
}

// AMQPConfig holds configuration for the AMQP pub/sub.
type AMQPConfig struct {
	URL  string `yaml:"url"`
	Mode string `yaml:"mode"`
}

// SQLConfig holds configuration for the SQL pub/sub.
type SQLConfig struct {
	Driver               string `yaml:"driver"`
	DSN                  string `yaml:"dsn"`
	Dialect              string `yaml:"dialect"`
	InitializeSchema     bool   `yaml:"initialize_schema"`
	AutoInitializeSchema bool   `yaml:"auto_initialize_schema"`
}

// HTTPConfig holds configuration for the HTTP publisher.
type HTTPConfig struct {
	BaseURL string `yaml:"base_url"`
	Mode    string `yaml:"mode"`
}

// RiverQueueConfig holds configuration for the RiverQueue publisher.
type RiverQueueConfig struct {
	Driver      string   `yaml:"driver"`
	DSN         string   `yaml:"dsn"`
	Table       string   `yaml:"table"`
	Queue       string   `yaml:"queue"`
	Kind        string   `yaml:"kind"`
	MaxAttempts int      `yaml:"max_attempts"`
	Priority    int      `yaml:"priority"`
	Tags        []string `yaml:"tags"`
}

type PublishRetryConfig struct {
	Attempts int `yaml:"attempts"`
	DelayMS  int `yaml:"delay_ms"`
}

// StorageConfig holds configuration for SQL-backed installation storage.
type StorageConfig struct {
	Driver      string `yaml:"driver"`
	DSN         string `yaml:"dsn"`
	Dialect     string `yaml:"dialect"`
	AutoMigrate bool   `yaml:"auto_migrate"`
}

// OAuthConfig holds configuration for OAuth callbacks.
type OAuthConfig struct {
	RedirectBaseURL string `yaml:"redirect_base_url"`
}

// LoadAppConfig loads the main application configuration from a YAML file.
// It applies default values.
func LoadAppConfig(path string) (AppConfig, error) {
	var cfg AppConfig
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}

	applyDefaults(&cfg)
	return cfg, nil
}

// LoadConfig loads the full application configuration, including rules, from a YAML file.
// It applies defaults and normalizes rules.
func LoadConfig(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}

	applyDefaults(&cfg.AppConfig)
	normalized, err := normalizeRules(cfg.Rules)
	if err != nil {
		return cfg, err
	}
	cfg.Rules = normalized
	cfg.RulesStrict = cfg.RulesStrict || false

	return cfg, nil
}

// RulesConfig represents the rule-specific parts of the configuration.
type RulesConfig struct {
	Rules    []Rule `yaml:"rules"`
	Strict   bool   `yaml:"rules_strict"`
	TenantID string `yaml:"-"`
	Logger   *log.Logger
}

// LoadRulesConfig loads only the rules from a YAML configuration file.
func LoadRulesConfig(path string) (RulesConfig, error) {
	var cfg RulesConfig
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	normalized, err := normalizeRules(cfg.Rules)
	if err != nil {
		return cfg, err
	}
	cfg.Rules = normalized
	return cfg, nil
}

// NormalizeRules trims and validates rule definitions.
func NormalizeRules(rules []Rule) ([]Rule, error) {
	return normalizeRules(rules)
}

func applyDefaults(cfg *AppConfig) {
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.ReadTimeoutMS == 0 {
		cfg.Server.ReadTimeoutMS = 5000
	}
	if cfg.Server.WriteTimeoutMS == 0 {
		cfg.Server.WriteTimeoutMS = 10000
	}
	if cfg.Server.IdleTimeoutMS == 0 {
		cfg.Server.IdleTimeoutMS = 60000
	}
	if cfg.Server.ReadHeaderMS == 0 {
		cfg.Server.ReadHeaderMS = 5000
	}
	if cfg.Server.MaxBodyBytes == 0 {
		cfg.Server.MaxBodyBytes = 1 << 20
	}
	if cfg.Endpoint == "" && cfg.Server.PublicBaseURL != "" {
		cfg.Endpoint = cfg.Server.PublicBaseURL
	}
	if cfg.RedirectBaseURL == "" && cfg.OAuth.RedirectBaseURL != "" {
		cfg.RedirectBaseURL = cfg.OAuth.RedirectBaseURL
	}
	if cfg.Providers.GitHub.Webhook.Path == "" {
		cfg.Providers.GitHub.Webhook.Path = "/webhooks/github"
	}
	if cfg.Providers.GitLab.Webhook.Path == "" {
		cfg.Providers.GitLab.Webhook.Path = "/webhooks/gitlab"
	}
	if cfg.Providers.Bitbucket.Webhook.Path == "" {
		cfg.Providers.Bitbucket.Webhook.Path = "/webhooks/bitbucket"
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
	if cfg.Watermill.RiverQueue.Table == "" {
		cfg.Watermill.RiverQueue.Table = "river_job"
	}
	if cfg.Watermill.RiverQueue.Queue == "" {
		cfg.Watermill.RiverQueue.Queue = "default"
	}
	if cfg.Watermill.RiverQueue.Kind == "" {
		cfg.Watermill.RiverQueue.Kind = "githook.event"
	}
	if cfg.Watermill.RiverQueue.MaxAttempts == 0 {
		cfg.Watermill.RiverQueue.MaxAttempts = 25
	}
	if cfg.Watermill.PublishRetry.Attempts == 0 {
		cfg.Watermill.PublishRetry.Attempts = 3
	}
	if cfg.Watermill.PublishRetry.DelayMS == 0 {
		cfg.Watermill.PublishRetry.DelayMS = 500
	}
	applyAuthDefaults(cfg)
}

func applyAuthDefaults(cfg *AppConfig) {
	if !cfg.Auth.OAuth2.Enabled {
		return
	}
	oauth2 := &cfg.Auth.OAuth2
	if oauth2.Mode == "" {
		oauth2.Mode = "auto"
	}
	if oauth2.RedirectURL == "" && cfg.Endpoint != "" {
		oauth2.RedirectURL = strings.TrimRight(cfg.Endpoint, "/") + "/auth/callback"
	}
	if len(oauth2.Scopes) == 0 {
		oauth2.Scopes = []string{"openid", "profile", "email"}
	}
}

func normalizeRules(rules []Rule) ([]Rule, error) {
	out := make([]Rule, 0, len(rules))
	for i := range rules {
		rule := rules[i]
		rule.When = strings.TrimSpace(rule.When)
		rule.Emit = EmitList(rule.Emit.Values())
		if rule.When == "" || len(rule.Emit) == 0 {
			return nil, fmt.Errorf("rule %d is missing when or emit", i)
		}
		if len(rule.Drivers) > 0 {
			drivers := make([]string, 0, len(rule.Drivers))
			for _, driver := range rule.Drivers {
				trimmed := strings.TrimSpace(driver)
				if trimmed != "" {
					drivers = append(drivers, trimmed)
				}
			}
			rule.Drivers = drivers
		}
		out = append(out, rule)
	}
	return out, nil
}
