package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/ThreeDotsLabs/watermill"

	"githook/pkg/core"
	"githook/pkg/drivers"
	"githook/pkg/storage"
)

const (
	eventLogStatusQueued    = "queued"
	eventLogStatusDelivered = "delivered"
	eventLogStatusSuccess   = "success"
	eventLogStatusFailed    = "failed"
	eventLogStatusUnmatched = "unmatched"
)

// rawObjectAndFlatten unmarshals a raw JSON byte slice into both an interface{}
// and a flattened map[string]interface{}. This is useful for both preserving the
// original structure and for easy access to nested fields.
func rawObjectAndFlatten(raw []byte) (interface{}, map[string]interface{}) {
	var out interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, map[string]interface{}{}
	}
	objectMap, ok := out.(map[string]interface{})
	if !ok {
		return out, map[string]interface{}{}
	}
	return out, core.Flatten(objectMap)
}

func annotatePayload(rawObject interface{}, data map[string]interface{}, provider, eventName string) interface{} {
	provider = strings.TrimSpace(provider)
	eventName = strings.TrimSpace(eventName)
	var refValue string
	var hasRef bool
	if data != nil {
		if provider != "" {
			data["provider"] = provider
		}
		if eventName != "" {
			data["event"] = eventName
		}
		if ref, ok := deriveGitRef(data); ok {
			refValue = ref
			hasRef = true
			data["ref"] = ref
		}
	}
	if obj, ok := rawObject.(map[string]interface{}); ok {
		if provider != "" {
			obj["provider"] = provider
		}
		if eventName != "" {
			obj["event"] = eventName
		}
		if hasRef {
			obj["ref"] = refValue
		} else if ref, ok := deriveGitRef(data); ok {
			obj["ref"] = ref
		}
		return obj
	}
	return rawObject
}

func deriveGitRef(data map[string]interface{}) (string, bool) {
	if data == nil {
		return "", false
	}
	if value, ok := data["ref"]; ok {
		if normalized, valid := normalizeGitRef(fmt.Sprintf("%v", value)); valid {
			return normalized, true
		}
	}
	candidates := []string{
		"check_suite.head_branch",
		"check_suite.head_ref",
		"workflow_run.head_branch",
		"workflow_run.head_ref",
		"push.ref",
	}
	for _, key := range candidates {
		if value := data[key]; value != nil {
			if normalized, valid := normalizeGitRef(fmt.Sprintf("%v", value)); valid {
				return normalized, true
			}
		}
	}
	return "", false
}

func normalizeGitRef(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	if strings.HasPrefix(value, "refs/") {
		return value, true
	}
	return fmt.Sprintf("refs/heads/%s", strings.TrimPrefix(value, "refs/heads/")), true
}

func requestID(r *http.Request) string {
	if r == nil {
		return watermill.NewUUID()
	}
	if id := r.Header.Get("X-Request-Id"); id != "" {
		return id
	}
	if id := r.Header.Get("X-Request-ID"); id != "" {
		return id
	}
	if id := r.Header.Get("X-Correlation-Id"); id != "" {
		return id
	}
	return watermill.NewUUID()
}

func logDebugEvent(logger *log.Logger, provider string, event string, body []byte) {
	if logger == nil {
		logger = log.Default()
	}
	logger.Printf("debug event provider=%s name=%s payload=%s", provider, event, string(body))
}

func ruleMatchesFromRules(rules []core.MatchedRule) []core.RuleMatch {
	matches := make([]core.RuleMatch, 0, len(rules))
	for _, rule := range rules {
		for _, topic := range rule.Emit {
			matches = append(matches, core.RuleMatch{
				Topic:            topic,
				DriverID:         rule.DriverID,
				DriverName:       rule.DriverName,
				DriverConfigJSON: rule.DriverConfigJSON,
				DriverEnabled:    rule.DriverEnabled,
				RuleID:           rule.ID,
				RuleWhen:         rule.When,
			})
		}
	}
	return matches
}

func topicsFromMatches(matches []core.RuleMatch) []string {
	topics := make([]string, 0, len(matches))
	for _, match := range matches {
		if match.Topic == "" {
			continue
		}
		topics = append(topics, match.Topic)
	}
	return topics
}

func logEventFailure(ctx context.Context, store storage.EventLogStore, logger *log.Logger, event core.Event, reason string) {
	if store == nil {
		return
	}
	record := storage.EventLogRecord{
		ID:             watermill.NewUUID(),
		Provider:       event.Provider,
		Name:           event.Name,
		RequestID:      event.RequestID,
		StateID:        event.StateID,
		InstallationID: event.InstallationID,
		NamespaceID:    event.NamespaceID,
		NamespaceName:  event.NamespaceName,
		Status:         eventLogStatusFailed,
		ErrorMessage:   reason,
		Matched:        false,
	}
	if err := store.CreateEventLogs(ctx, []storage.EventLogRecord{record}); err != nil && logger != nil {
		logger.Printf("event log write failed: %v", err)
	}
}

func logEventMatches(ctx context.Context, store storage.EventLogStore, logger *log.Logger, event core.Event, rules []core.MatchedRule) []storage.EventLogRecord {
	if store == nil {
		return nil
	}
	records, matched := buildEventLogRecords(event, rules)
	if len(records) == 0 {
		return nil
	}
	if err := store.CreateEventLogs(ctx, records); err != nil && logger != nil {
		logger.Printf("event log write failed: %v", err)
	}
	return matched
}

func buildEventLogRecords(event core.Event, rules []core.MatchedRule) ([]storage.EventLogRecord, []storage.EventLogRecord) {
	if len(rules) == 0 {
		record := storage.EventLogRecord{
			ID:             watermill.NewUUID(),
			Provider:       event.Provider,
			Name:           event.Name,
			RequestID:      event.RequestID,
			StateID:        event.StateID,
			InstallationID: event.InstallationID,
			NamespaceID:    event.NamespaceID,
			NamespaceName:  event.NamespaceName,
			Status:         eventLogStatusUnmatched,
			Matched:        false,
		}
		return []storage.EventLogRecord{record}, nil
	}

	records := make([]storage.EventLogRecord, 0, len(rules))
	matched := make([]storage.EventLogRecord, 0, len(rules))
	for _, rule := range rules {
		driverName := strings.TrimSpace(rule.DriverName)
		if driverName == "" {
			driverName = strings.TrimSpace(rule.DriverID)
		}
		var drivers []string
		if driverName != "" {
			drivers = []string{driverName}
		}
		for _, topic := range rule.Emit {
			record := storage.EventLogRecord{
				ID:             watermill.NewUUID(),
				Provider:       event.Provider,
				Name:           event.Name,
				RequestID:      event.RequestID,
				StateID:        event.StateID,
				InstallationID: event.InstallationID,
				NamespaceID:    event.NamespaceID,
				NamespaceName:  event.NamespaceName,
				Topic:          topic,
				RuleID:         rule.ID,
				RuleWhen:       rule.When,
				Drivers:        append([]string(nil), drivers...),
				Status:         eventLogStatusQueued,
				Matched:        true,
			}
			records = append(records, record)
			matched = append(matched, record)
		}
	}
	return records, matched
}

func topicsFromLogRecords(records []storage.EventLogRecord) []string {
	topics := make([]string, 0, len(records))
	for _, record := range records {
		if record.Topic == "" {
			continue
		}
		topics = append(topics, record.Topic)
	}
	return topics
}

func driverListFromMatch(match core.RuleMatch) []string {
	driverName := strings.TrimSpace(match.DriverName)
	if driverName == "" {
		driverName = strings.TrimSpace(match.DriverID)
	}
	if driverName == "" {
		return nil
	}
	return []string{driverName}
}

func publishMatchesWithFallback(ctx context.Context, event core.Event, matches []core.RuleMatch, logs []storage.EventLogRecord, dynamic *drivers.DynamicPublisherCache, fallback core.Publisher, logger *log.Logger, statusUpdater func(string, string, string)) {
	if len(matches) == 0 {
		return
	}
	if logger != nil {
		logger.Printf("publishing event request_id=%s provider=%s name=%s matches=%d logs=%d tenant=%s namespace=%s",
			event.RequestID, event.Provider, event.Name, len(matches), len(logs), event.StateID, event.NamespaceName)
	}
	for idx, match := range matches {
		if idx < len(logs) {
			event.LogID = logs[idx].ID
		} else {
			event.LogID = ""
		}
		matchDriver := strings.TrimSpace(match.DriverName)
		if matchDriver == "" {
			matchDriver = strings.TrimSpace(match.DriverID)
		}
		if logger != nil {
			logger.Printf("publishing match topic=%s driver=%s driver_id=%s", match.Topic, matchDriver, match.DriverID)
		}
		ok, err := publishDynamicMatch(ctx, event, match, dynamic, logger)
		if err != nil && statusUpdater != nil && idx < len(logs) {
			statusUpdater(logs[idx].ID, eventLogStatusFailed, err.Error())
		}
		if ok {
			if statusUpdater != nil && idx < len(logs) {
				statusUpdater(logs[idx].ID, eventLogStatusDelivered, "")
			}
			continue
		}
		if err != nil {
			continue
		}
		drivers := driverListFromMatch(match)
		if len(drivers) == 0 {
			if logger != nil {
				logger.Printf("publish skipped: no driver configured for topic=%s", match.Topic)
			}
			continue
		}
		if logger != nil {
			logger.Printf("fallback publish topic=%s drivers=%v", match.Topic, drivers)
		}
		if logger != nil {
			logger.Printf("fallback publish attempt topic=%s drivers=%v driver_ids=%v", match.Topic, drivers, match.DriverID)
		}
		if err := fallback.PublishForDrivers(ctx, match.Topic, event, drivers); err != nil {
			if logger != nil {
				logger.Printf("publish %s failed: %v", match.Topic, err)
			}
			if statusUpdater != nil && idx < len(logs) {
				statusUpdater(logs[idx].ID, eventLogStatusFailed, err.Error())
			}
		} else if statusUpdater != nil && idx < len(logs) {
			statusUpdater(logs[idx].ID, eventLogStatusDelivered, "")
			if logger != nil {
				logger.Printf("fallback publish delivered topic=%s drivers=%v", match.Topic, drivers)
			}
		}
	}
}

func publishDynamicMatch(ctx context.Context, event core.Event, match core.RuleMatch, cache *drivers.DynamicPublisherCache, logger *log.Logger) (bool, error) {
	if cache == nil {
		return false, nil
	}
	driverName := strings.TrimSpace(match.DriverName)
	if driverName == "" || strings.TrimSpace(match.DriverConfigJSON) == "" {
		if logger != nil {
			logger.Printf("dynamic publish skipped: missing driver config topic=%s driver=%q config_present=%t", match.Topic, driverName, strings.TrimSpace(match.DriverConfigJSON) != "")
		}
		return false, nil
	}
	if !match.DriverEnabled {
		if logger != nil {
			logger.Printf("dynamic driver disabled: %s", driverName)
		}
		return false, nil
	}
	if logger != nil {
		logger.Printf("dynamic publish init driver=%s topic=%s provider=%s config_len=%d", driverName, match.Topic, event.Provider, len(match.DriverConfigJSON))
	}
	pub, err := cache.Publisher(driverName, match.DriverConfigJSON)
	if err != nil {
		if logger != nil {
			logger.Printf("dynamic publisher init failed driver=%s err=%v", driverName, err)
		}
		return false, err
	}
	if err := pub.Publish(ctx, match.Topic, event); err != nil {
		if logger != nil {
			logger.Printf("dynamic publish failed topic=%s driver=%s err=%v", match.Topic, driverName, err)
		}
		return false, err
	}
	if logger != nil {
		logger.Printf("dynamic publish success topic=%s driver=%s", match.Topic, driverName)
	}
	return true, nil
}

func matchRulesFromStore(ctx context.Context, event core.Event, tenantID string, ruleStore storage.RuleStore, driverStore storage.DriverStore, strict bool, logger *log.Logger) []core.MatchedRule {
	if ruleStore == nil {
		return nil
	}
	tenantCtx := storage.WithTenant(ctx, tenantID)
	if logger != nil {
		logger.Printf("rule load requested tenant=%s provider=%s event=%s", tenantID, event.Provider, event.Name)
		logger.Printf("rule engine start tenant=%s provider=%s event=%s", tenantID, event.Provider, event.Name)
	}
	rules, err := loadRulesForTenant(tenantCtx, ruleStore, driverStore, logger)
	if err != nil {
		if logger != nil {
			logger.Printf("rule load failed: %v", err)
		}
		return nil
	}
	ruleMap := make(map[string]core.Rule, len(rules))
	for _, rule := range rules {
		ruleMap[rule.ID] = rule
	}
	if logger != nil {
		logger.Printf("rule engine loaded %d rules tenant=%s provider=%s", len(rules), tenantID, event.Provider)
	}
	if len(rules) == 0 {
		return nil
	}
	engine, err := core.NewRuleEngine(core.RulesConfig{
		Rules:  rules,
		Strict: strict,
		Logger: logger,
	})
	if err != nil {
		if logger != nil {
			logger.Printf("rule engine compile failed: %v", err)
		}
		return nil
	}
	matches := engine.EvaluateRulesWithLogger(event, logger)
	if logger != nil {
		logger.Printf("rule evaluation complete matches=%d tenant=%s provider=%s event=%s", len(matches), tenantID, event.Provider, event.Name)
	}
	matchMap := make(map[string]struct{}, len(matches))
	if logger != nil && len(matches) > 0 {
		for _, match := range matches {
			matchMap[match.ID] = struct{}{}
			rule := ruleMap[match.ID]
			emit := rule.Emit
			if len(emit) == 0 {
				emit = []string{}
			}
			for _, topic := range emit {
				logger.Printf("rule match id=%s when=%q emit=%v topic=%s driver_id=%s driver_name=%s",
					match.ID,
					rule.When,
					emit,
					topic,
					match.DriverID,
					match.DriverName,
				)
			}
			if len(emit) == 0 {
				logger.Printf("rule match id=%s when=%q emit=%v topic= driver_id=%s driver_name=%s",
					match.ID,
					rule.When,
					emit,
					match.DriverID,
					match.DriverName,
				)
			}
		}
	}
	if logger != nil && len(matches) == 0 {
		logger.Printf("rule match none tenant=%s provider=%s event=%s", tenantID, event.Provider, event.Name)
	}
	if logger != nil {
		for _, rule := range rules {
			if _, ok := matchMap[rule.ID]; ok {
				continue
			}
			logger.Printf("rule unmatched id=%s when=%q emit=%v", rule.ID, rule.When, rule.Emit)
		}
	}
	return matches
}

func loadRulesForTenant(ctx context.Context, store storage.RuleStore, driverStore storage.DriverStore, logger *log.Logger) ([]core.Rule, error) {
	if store == nil {
		return nil, nil
	}
	records, err := store.ListRules(ctx)
	if err != nil {
		return nil, err
	}
	rules := make([]core.Rule, 0, len(records))
	for _, record := range records {
		if logger != nil {
			logger.Printf("rule record raw id=%s tenant=%s when=%q emit=%v driver_id=%s driver_name=%s enabled=%t config=%s",
				record.ID,
				record.TenantID,
				record.When,
				record.Emit,
				record.DriverID,
				record.DriverName,
				record.DriverEnabled,
				record.DriverConfigJSON,
			)
		}
		if strings.TrimSpace(record.When) == "" {
			continue
		}
		if len(record.Emit) == 0 {
			continue
		}
		driverID := strings.TrimSpace(record.DriverID)
		if driverID == "" {
			continue
		}
		driverName := strings.TrimSpace(record.DriverName)
		if driverName == "" {
			var err error
			driverName, err = driverNameForID(ctx, driverStore, driverID)
			if err != nil {
				if logger != nil {
					logger.Printf("rule driver resolve failed: %v", err)
				}
				continue
			}
		}
		if logger != nil {
			logger.Printf(
				"rule fetched id=%s driver_id=%s driver_name=%s enabled=%t emit=%v config=%s",
				record.ID,
				driverID,
				driverName,
				record.DriverEnabled,
				record.Emit,
				record.DriverConfigJSON,
			)
		}
		rules = append(rules, core.Rule{
			ID:               record.ID,
			When:             record.When,
			Emit:             core.EmitList(record.Emit),
			DriverID:         driverID,
			DriverName:       driverName,
			DriverConfigJSON: strings.TrimSpace(record.DriverConfigJSON),
			DriverEnabled:    record.DriverEnabled,
		})
	}
	return rules, nil
}

func driverNameForID(ctx context.Context, store storage.DriverStore, driverID string) (string, error) {
	if strings.TrimSpace(driverID) == "" {
		return "", errors.New("driver_id is required")
	}
	if store == nil {
		return "", errors.New("driver store not configured")
	}
	record, err := store.GetDriverByID(ctx, driverID)
	if err != nil {
		return "", err
	}
	if record == nil {
		return "", fmt.Errorf("driver not found: %s", driverID)
	}
	name := strings.TrimSpace(record.Name)
	if name == "" {
		return "", fmt.Errorf("driver %s has empty name", driverID)
	}
	return name, nil
}
