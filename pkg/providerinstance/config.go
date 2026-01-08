package providerinstance

import (
	"encoding/json"
	"errors"
	"strings"

	"githooks/pkg/auth"
	"githooks/pkg/storage"
)

// DefaultKey is the instance key used for config-defined providers.
const DefaultKey = "default"

// RecordsFromConfig converts provider config into default instance records.
func RecordsFromConfig(cfg auth.Config) ([]storage.ProviderInstanceRecord, error) {
	out := make([]storage.ProviderInstanceRecord, 0, 3)
	if record, err := instanceRecord("github", cfg.GitHub); err != nil {
		return nil, err
	} else {
		out = append(out, record)
	}
	if record, err := instanceRecord("gitlab", cfg.GitLab); err != nil {
		return nil, err
	} else {
		out = append(out, record)
	}
	if record, err := instanceRecord("bitbucket", cfg.Bitbucket); err != nil {
		return nil, err
	} else {
		out = append(out, record)
	}
	return out, nil
}

// ProviderConfigFromRecord returns a provider config from an instance record.
func ProviderConfigFromRecord(record storage.ProviderInstanceRecord) (auth.ProviderConfig, error) {
	var cfg auth.ProviderConfig
	if err := unmarshalConfig(record.ConfigJSON, &cfg); err != nil {
		return auth.ProviderConfig{}, err
	}
	cfg.Enabled = record.Enabled
	return cfg, nil
}

func instanceRecord(provider string, cfg auth.ProviderConfig) (storage.ProviderInstanceRecord, error) {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return storage.ProviderInstanceRecord{}, err
	}
	key := strings.TrimSpace(cfg.Key)
	if key == "" {
		key = DefaultKey
	}
	return storage.ProviderInstanceRecord{
		Provider:   provider,
		Key:        key,
		ConfigJSON: string(raw),
		Enabled:    cfg.Enabled,
	}, nil
}

func unmarshalConfig(raw string, target interface{}) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if target == nil {
		return errors.New("target is nil")
	}
	return json.Unmarshal([]byte(raw), target)
}
