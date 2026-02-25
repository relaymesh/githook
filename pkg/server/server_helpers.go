package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/relaymesh/githook/pkg/storage"
	"github.com/relaymesh/githook/pkg/storage/installations"
	"github.com/relaymesh/githook/pkg/storage/namespaces"
	providerinstancestore "github.com/relaymesh/githook/pkg/storage/provider_instances"
)

func resolveRuleDriverName(ctx context.Context, store storage.DriverStore, driverID string) (string, error) {
	if driverID == "" {
		return "", errors.New("driver_id is required")
	}
	if store == nil {
		return "", errors.New("driver store not configured")
	}
	trimmed := strings.TrimSpace(driverID)
	if trimmed == "" {
		return "", errors.New("driver_id is required")
	}
	record, err := store.GetDriverByID(ctx, trimmed)
	if err != nil {
		return "", err
	}
	if record == nil {
		return "", fmt.Errorf("driver not found: %s", trimmed)
	}
	name := strings.TrimSpace(record.Name)
	if name == "" {
		return "", fmt.Errorf("driver %s has empty name", trimmed)
	}
	return name, nil
}

func resolveProviderInstanceHash(
	ctx context.Context,
	store *providerinstancestore.Store,
	record storage.ProviderInstanceRecord,
) (string, string, error) {
	provider := strings.TrimSpace(record.Provider)
	if provider == "" {
		return "", "", errors.New("provider is required")
	}
	tenantID := strings.TrimSpace(record.TenantID)
	records, err := store.ListProviderInstances(ctx, provider)
	if err != nil {
		return "", "", err
	}
	configJSON := strings.TrimSpace(record.ConfigJSON)
	legacyKey := ""
	for _, existing := range records {
		if strings.TrimSpace(existing.TenantID) != tenantID {
			continue
		}
		if strings.TrimSpace(existing.ConfigJSON) != configJSON {
			continue
		}
		key := strings.TrimSpace(existing.Key)
		if key == "" {
			continue
		}
		if isProviderInstanceHash(key) {
			return key, "", nil
		}
		if legacyKey != "" && legacyKey != key {
			return "", "", errors.New("multiple legacy provider instance keys match config")
		}
		legacyKey = key
	}
	hash, err := randomHex(32)
	if err != nil {
		return "", "", err
	}
	return hash, legacyKey, nil
}

func migrateProviderInstanceKey(
	ctx context.Context,
	instanceStore *providerinstancestore.Store,
	installStore *installations.Store,
	namespaceStore *namespaces.Store,
	provider string,
	oldKey string,
	newKey string,
	tenantID string,
) error {
	provider = strings.TrimSpace(provider)
	oldKey = strings.TrimSpace(oldKey)
	newKey = strings.TrimSpace(newKey)
	tenantID = strings.TrimSpace(tenantID)
	if provider == "" || oldKey == "" || newKey == "" {
		return errors.New("provider and keys are required")
	}
	if oldKey == newKey {
		return nil
	}
	if instanceStore == nil {
		return errors.New("provider instance store not configured")
	}
	records, err := instanceStore.ListProviderInstances(ctx, provider)
	if err != nil {
		return err
	}
	hasNewKey := false
	for _, record := range records {
		if strings.TrimSpace(record.TenantID) != tenantID {
			continue
		}
		if strings.TrimSpace(record.Key) == newKey {
			hasNewKey = true
			break
		}
	}
	if installStore != nil {
		if _, err := installStore.UpdateProviderInstanceKey(ctx, provider, oldKey, newKey, tenantID); err != nil {
			return err
		}
	}
	if namespaceStore != nil {
		if _, err := namespaceStore.UpdateProviderInstanceKey(ctx, provider, oldKey, newKey, tenantID); err != nil {
			return err
		}
	}
	if hasNewKey {
		return instanceStore.DeleteProviderInstanceForTenant(ctx, provider, oldKey, tenantID)
	}
	if _, err := instanceStore.UpdateProviderInstanceKey(ctx, provider, oldKey, newKey, tenantID); err != nil {
		return err
	}
	return nil
}

func isProviderInstanceHash(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func randomHex(size int) (string, error) {
	if size <= 0 {
		return "", errors.New("random hex size must be positive")
	}
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func ruleKey(when string, emit []string, driverID string) string {
	emitKey := normalizeRuleSlice(emit)
	driverKey := strings.TrimSpace(driverID)
	return strings.TrimSpace(when) + "|" + emitKey + "|" + driverKey
}

func normalizeRuleSlice(values []string) string {
	clean := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		clean = append(clean, trimmed)
	}
	sort.Strings(clean)
	return strings.Join(clean, ",")
}
