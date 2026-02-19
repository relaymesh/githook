package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"githook/pkg/auth"
	"githook/pkg/core"
	"githook/pkg/drivers"
	"githook/pkg/providerinstance"
	"githook/pkg/storage"
)

// SlackHandler handles incoming webhooks from Slack.
type SlackHandler struct {
	config         auth.ProviderConfig
	rules          *core.RuleEngine
	publisher      core.Publisher
	logger         *log.Logger
	maxBody        int64
	debugEvents    bool
	store          storage.Store
	logs           storage.EventLogStore
	ruleStore      storage.RuleStore
	driverStore    storage.DriverStore
	rulesStrict    bool
	dynamicDrivers *drivers.DynamicPublisherCache
	instanceStore  storage.ProviderInstanceStore
	instanceCache  *providerinstance.Cache
}

type slackEnvelope struct {
	Type      string `json:"type"`
	TeamID    string `json:"team_id"`
	APIAppID  string `json:"api_app_id"`
	Challenge string `json:"challenge"`
	Event     struct {
		Type    string `json:"type"`
		Subtype string `json:"subtype"`
		TeamID  string `json:"team"`
	} `json:"event"`
}

// NewSlackHandler creates a new SlackHandler.
func NewSlackHandler(cfg auth.ProviderConfig, opts HandlerOptions) (*SlackHandler, error) {
	logger := opts.Logger
	if logger == nil {
		logger = log.Default()
	}
	return &SlackHandler{
		config:         cfg,
		rules:          opts.Rules,
		publisher:      opts.Publisher,
		logger:         logger,
		maxBody:        opts.MaxBodyBytes,
		debugEvents:    opts.DebugEvents,
		store:          opts.InstallStore,
		logs:           opts.EventLogStore,
		ruleStore:      opts.RuleStore,
		driverStore:    opts.DriverStore,
		rulesStrict:    opts.RulesStrict,
		dynamicDrivers: opts.DynamicDriverCache,
		instanceStore:  opts.ProviderInstanceStore,
		instanceCache:  opts.ProviderInstanceCache,
	}, nil
}

// ServeHTTP handles an incoming HTTP request.
func (h *SlackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.maxBody > 0 {
		r.Body = http.MaxBytesReader(w, r.Body, h.maxBody)
	}
	reqID := requestID(r)
	w.Header().Set("X-Request-Id", reqID)
	logger := core.WithRequestID(h.logger, reqID)
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(rawBody))

	var payload slackEnvelope
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		logger.Printf("slack parse failed: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	eventName := slackEventName(payload)
	if h.debugEvents {
		logDebugEvent(logger, "slack", eventName, rawBody)
	}

	ctx := r.Context()
	teamID := slackTeamID(payload)
	appID := strings.TrimSpace(payload.APIAppID)
	instanceKey := slackInstanceKeyFromRequest(r)
	record := h.findSlackInstall(ctx, appID, teamID)
	if record != nil && instanceKey == "" {
		instanceKey = record.ProviderInstanceKey
	}

	cfg := h.config
	if resolved, ok := h.resolveSlackConfig(ctx, instanceKey); ok {
		cfg = resolved
	}
	secret := strings.TrimSpace(cfg.Webhook.Secret)
	timestamp := r.Header.Get("X-Slack-Request-Timestamp")
	signature := r.Header.Get("X-Slack-Signature")
	if err := validateSlackTimestamp(timestamp, time.Now().UTC()); err != nil {
		logger.Printf("slack signature rejected: %v", err)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	verified := false
	if secret != "" && slackSignatureMatches(secret, timestamp, signature, rawBody) {
		verified = true
	}
	if !verified && instanceKey == "" {
		var matched bool
		cfg, instanceKey, matched = h.matchSlackConfigBySignature(ctx, timestamp, signature, rawBody)
		if matched {
			secret = strings.TrimSpace(cfg.Webhook.Secret)
			verified = true
		}
	}
	if !verified {
		logger.Printf("slack signature rejected: invalid signature")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if strings.EqualFold(payload.Type, "url_verification") {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"challenge": payload.Challenge})
		return
	}

	installationID := appID
	stateID := teamID
	tenantID := ""
	if record != nil {
		tenantID = record.TenantID
		if record.AccountID != "" {
			stateID = record.AccountID
		}
		if record.InstallationID != "" {
			installationID = record.InstallationID
		}
	}
	if installationID == "" {
		logger.Printf("slack webhook ignored: missing api_app_id")
		w.WriteHeader(http.StatusOK)
		return
	}
	if tenantID != "" {
		ctx = storage.WithTenant(ctx, tenantID)
		r = r.WithContext(ctx)
	}

	rawObject, data := rawObjectAndFlatten(rawBody)
	rawObject = annotatePayload(rawObject, data, "slack", eventName)
	h.emit(r, logger, core.Event{
		Provider:       "slack",
		Name:           eventName,
		RequestID:      reqID,
		Headers:        cloneHeaders(r.Header),
		Data:           data,
		RawPayload:     rawBody,
		RawObject:      rawObject,
		StateID:        stateID,
		InstallationID: installationID,
	})

	w.WriteHeader(http.StatusOK)
}

func (h *SlackHandler) resolveSlackConfig(ctx context.Context, instanceKey string) (auth.ProviderConfig, bool) {
	instanceKey = strings.TrimSpace(instanceKey)
	if instanceKey == "" {
		return auth.ProviderConfig{}, false
	}
	if h.instanceCache != nil {
		cfg, ok, err := h.instanceCache.ConfigFor(ctx, "slack", instanceKey)
		if err == nil && ok {
			return cfg, true
		}
	}
	if h.instanceStore == nil {
		return auth.ProviderConfig{}, false
	}
	record, err := h.instanceStore.GetProviderInstance(ctx, "slack", instanceKey)
	if err != nil || record == nil {
		return auth.ProviderConfig{}, false
	}
	cfg, err := providerinstance.ProviderConfigFromRecord(*record)
	if err != nil {
		return auth.ProviderConfig{}, false
	}
	return cfg, true
}

func (h *SlackHandler) matchSlackConfigBySignature(ctx context.Context, timestamp, signature string, body []byte) (auth.ProviderConfig, string, bool) {
	if h.instanceStore == nil {
		return auth.ProviderConfig{}, "", false
	}
	records, err := h.instanceStore.ListProviderInstances(ctx, "slack")
	if err != nil || len(records) == 0 {
		return auth.ProviderConfig{}, "", false
	}
	for _, record := range records {
		cfg, err := providerinstance.ProviderConfigFromRecord(record)
		if err != nil {
			continue
		}
		secret := strings.TrimSpace(cfg.Webhook.Secret)
		if secret == "" {
			continue
		}
		if slackSignatureMatches(secret, timestamp, signature, body) {
			return cfg, record.Key, true
		}
	}
	return auth.ProviderConfig{}, "", false
}

func (h *SlackHandler) findSlackInstall(ctx context.Context, appID, teamID string) *storage.InstallRecord {
	if h.store == nil {
		return nil
	}
	appID = strings.TrimSpace(appID)
	teamID = strings.TrimSpace(teamID)
	if teamID != "" {
		records, err := h.store.ListInstallations(ctx, "slack", teamID)
		if err != nil || len(records) == 0 {
			return nil
		}
		best := records[0]
		for _, record := range records[1:] {
			if record.UpdatedAt.After(best.UpdatedAt) {
				best = record
			}
		}
		return &best
	}
	if appID == "" {
		return nil
	}
	record, err := h.store.GetInstallationByInstallationID(ctx, "slack", appID)
	if err != nil || record == nil {
		return nil
	}
	return record
}

func (h *SlackHandler) matchRules(ctx context.Context, event core.Event, tenantID string, logger *log.Logger) []core.MatchedRule {
	if h.ruleStore != nil {
		return matchRulesFromStore(ctx, event, tenantID, h.ruleStore, h.driverStore, h.rulesStrict, logger)
	}
	if h.rules == nil {
		return nil
	}
	return h.rules.EvaluateRulesForTenantWithLogger(event, tenantID, logger)
}

func (h *SlackHandler) emit(r *http.Request, logger *log.Logger, event core.Event) {
	if logger != nil {
		logger.Printf("event received provider=%s name=%s installation_id=%s request_id=%s", event.Provider, event.Name, event.InstallationID, event.RequestID)
	}
	tenantID := storage.TenantFromContext(r.Context())
	matches := h.matchRules(r.Context(), event, tenantID, logger)
	if h.logs == nil {
		matchRules := ruleMatchesFromRules(matches)
		logger.Printf("event provider=%s name=%s topics=%v", event.Provider, event.Name, topicsFromMatches(matchRules))
		publishMatchesWithFallback(r.Context(), event, matchRules, nil, h.dynamicDrivers, h.publisher, logger, nil)
		return
	}

	matchLogs := logEventMatches(r.Context(), h.logs, logger, event, matches)
	logger.Printf("event provider=%s name=%s topics=%v", event.Provider, event.Name, topicsFromLogRecords(matchLogs))
	matchRules := ruleMatchesFromRules(matches)
	statusUpdater := func(recordID, status, message string) {
		if recordID == "" {
			return
		}
		if err := h.logs.UpdateEventLogStatus(r.Context(), recordID, status, message); err != nil {
			logger.Printf("event log update failed: %v", err)
		}
	}
	publishMatchesWithFallback(r.Context(), event, matchRules, matchLogs, h.dynamicDrivers, h.publisher, logger, statusUpdater)
}

func slackEventName(payload slackEnvelope) string {
	eventType := strings.TrimSpace(payload.Type)
	if eventType != "event_callback" {
		return eventType
	}
	name := strings.TrimSpace(payload.Event.Type)
	if name == "" {
		return eventType
	}
	if subtype := strings.TrimSpace(payload.Event.Subtype); subtype != "" {
		return name + "." + subtype
	}
	return name
}

func slackTeamID(payload slackEnvelope) string {
	if strings.TrimSpace(payload.TeamID) != "" {
		return strings.TrimSpace(payload.TeamID)
	}
	return strings.TrimSpace(payload.Event.TeamID)
}

func slackInstanceKeyFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	key := strings.TrimSpace(r.URL.Query().Get("instance"))
	if key == "" {
		key = strings.TrimSpace(r.URL.Query().Get("provider_instance_key"))
	}
	return key
}
