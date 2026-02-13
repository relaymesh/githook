package webhook

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/ThreeDotsLabs/watermill"

	"githook/pkg/core"
	"githook/pkg/storage"
)

const (
	eventLogStatusQueued    = "queued"
	eventLogStatusDelivered = "delivered"
	eventLogStatusSuccess   = "success"
	eventLogStatusFailed    = "failed"
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
	if data != nil {
		if provider != "" {
			data["provider"] = provider
		}
		if eventName != "" {
			data["event"] = eventName
		}
	}
	if obj, ok := rawObject.(map[string]interface{}); ok {
		if provider != "" {
			obj["provider"] = provider
		}
		if eventName != "" {
			obj["event"] = eventName
		}
		return obj
	}
	return rawObject
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
				Topic:   topic,
				Drivers: append([]string(nil), rule.Drivers...),
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
		return nil, nil
	}

	records := make([]storage.EventLogRecord, 0, len(rules))
	matched := make([]storage.EventLogRecord, 0, len(rules))
	for _, rule := range rules {
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
				Drivers:        append([]string(nil), rule.Drivers...),
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
