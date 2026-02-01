package providerinstance

import (
	"encoding/json"
	"errors"
	"strings"

	"githooks/pkg/auth"
	"githooks/pkg/storage"
)

// RecordsFromConfig converts provider config into instance records.
func RecordsFromConfig(cfg auth.Config) ([]storage.ProviderInstanceRecord, error) {
	out := make([]storage.ProviderInstanceRecord, 0, 3)
	if record, ok, err := instanceRecordFromConfig("github", cfg.GitHub); err != nil {
		return nil, err
	} else if ok {
		out = append(out, record)
	}
	if record, ok, err := instanceRecordFromConfig("gitlab", cfg.GitLab); err != nil {
		return nil, err
	} else if ok {
		out = append(out, record)
	}
	if record, ok, err := instanceRecordFromConfig("bitbucket", cfg.Bitbucket); err != nil {
		return nil, err
	} else if ok {
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
	cfg.Key = ""
	raw, err := json.Marshal(cfg)
	if err != nil {
		return storage.ProviderInstanceRecord{}, err
	}
	return storage.ProviderInstanceRecord{
		Provider:   provider,
		Key:        "",
		ConfigJSON: string(raw),
		Enabled:    cfg.Enabled,
	}, nil
}

func instanceRecordFromConfig(provider string, cfg auth.ProviderConfig) (storage.ProviderInstanceRecord, bool, error) {
	if !hasProviderConfig(cfg) {
		return storage.ProviderInstanceRecord{}, false, nil
	}
	record, err := instanceRecord(provider, cfg)
	if err != nil {
		return storage.ProviderInstanceRecord{}, false, err
	}
	return record, true, nil
}

func hasProviderConfig(cfg auth.ProviderConfig) bool {
	if cfg.Enabled {
		return true
	}
	if strings.TrimSpace(cfg.Webhook.Path) != "" || strings.TrimSpace(cfg.Webhook.Secret) != "" {
		return true
	}
	if cfg.App.AppID != 0 || strings.TrimSpace(cfg.App.PrivateKeyPath) != "" || strings.TrimSpace(cfg.App.AppSlug) != "" {
		return true
	}
	if strings.TrimSpace(cfg.OAuth.ClientID) != "" || strings.TrimSpace(cfg.OAuth.ClientSecret) != "" || len(cfg.OAuth.Scopes) > 0 {
		return true
	}
	if strings.TrimSpace(cfg.API.BaseURL) != "" || strings.TrimSpace(cfg.API.WebBaseURL) != "" {
		return true
	}
	return false
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
