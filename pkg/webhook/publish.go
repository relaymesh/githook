package webhook

import (
	"context"
	"log"
	"strings"

	"githook/pkg/core"
	"githook/pkg/drivers"
	"githook/pkg/storage"
)

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
