package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"githook/pkg/core"
	"githook/pkg/storage"

	"github.com/go-playground/webhooks/v6/gitlab"
)

// GitLabHandler handles incoming webhooks from GitLab.
type GitLabHandler struct {
	hook        *gitlab.Webhook
	rules       *core.RuleEngine
	publisher   core.Publisher
	logger      *log.Logger
	maxBody     int64
	debugEvents bool
	namespaces  storage.NamespaceStore
	logs        storage.EventLogStore
}

var gitlabEvents = []gitlab.Event{
	gitlab.PushEvents,
	gitlab.TagEvents,
	gitlab.IssuesEvents,
	gitlab.ConfidentialIssuesEvents,
	gitlab.CommentEvents,
	gitlab.ConfidentialCommentEvents,
	gitlab.MergeRequestEvents,
	gitlab.WikiPageEvents,
	gitlab.PipelineEvents,
	gitlab.BuildEvents,
	gitlab.JobEvents,
	gitlab.DeploymentEvents,
	gitlab.SystemHookEvents,
}

// NewGitLabHandler creates a new GitLabHandler.
func NewGitLabHandler(secret string, rules *core.RuleEngine, publisher core.Publisher, logger *log.Logger, maxBody int64, debugEvents bool, namespaces storage.NamespaceStore, logs storage.EventLogStore) (*GitLabHandler, error) {
	options := make([]gitlab.Option, 0, 1)
	if secret != "" {
		options = append(options, gitlab.Options.Secret(secret))
	}
	hook, err := gitlab.New(options...)
	if err != nil {
		return nil, err
	}
	if logger == nil {
		logger = log.Default()
	}
	return &GitLabHandler{hook: hook, rules: rules, publisher: publisher, logger: logger, maxBody: maxBody, debugEvents: debugEvents, namespaces: namespaces, logs: logs}, nil
}

// ServeHTTP handles an incoming HTTP request.
func (h *GitLabHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	if h.debugEvents {
		logDebugEvent(logger, "gitlab", r.Header.Get("X-Gitlab-Event"), rawBody)
	}

	payload, err := h.hook.Parse(r, gitlabEvents...)
	if err != nil {
		logger.Printf("gitlab parse failed: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	eventName := r.Header.Get("X-Gitlab-Event")
	switch payload.(type) {
	default:
		rawObject, data := rawObjectAndFlatten(rawBody)
		namespaceID, namespaceName := gitlabNamespaceInfo(rawBody)
		stateID, installationID := h.resolveStateID(r.Context(), rawBody)
		if installationID == "" {
			logger.Printf("gitlab webhook ignored: missing installation_id")
			w.WriteHeader(http.StatusOK)
			return
		}
		h.emit(r, logger, core.Event{
			Provider:       "gitlab",
			Name:           eventName,
			RequestID:      reqID,
			Data:           data,
			RawPayload:     rawBody,
			RawObject:      rawObject,
			StateID:        stateID,
			InstallationID: installationID,
			NamespaceID:    namespaceID,
			NamespaceName:  namespaceName,
		})
	}

	w.WriteHeader(http.StatusOK)
}

func (h *GitLabHandler) resolveStateID(ctx context.Context, raw []byte) (string, string) {
	if h.namespaces == nil {
		return "", ""
	}
	var payload struct {
		Project struct {
			ID int64 `json:"id"`
		} `json:"project"`
		ProjectID int64 `json:"project_id"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", ""
	}
	repoID := payload.Project.ID
	if repoID == 0 {
		repoID = payload.ProjectID
	}
	if repoID == 0 {
		return "", ""
	}
	record, err := h.namespaces.GetNamespace(ctx, "gitlab", strconv.FormatInt(repoID, 10), "")
	if err != nil || record == nil {
		return "", ""
	}
	return record.AccountID, record.InstallationID
}

func (h *GitLabHandler) emit(r *http.Request, logger *log.Logger, event core.Event) {
	tenantID := storage.TenantFromContext(r.Context())
	rules := h.rules.EvaluateRulesForTenantWithLogger(event, tenantID, logger)
	if h.logs == nil {
		matches := ruleMatchesFromRules(rules)
		logger.Printf("event provider=%s name=%s topics=%v", event.Provider, event.Name, topicsFromMatches(matches))
		for _, match := range matches {
			if err := h.publisher.PublishForDrivers(r.Context(), match.Topic, event, match.Drivers); err != nil {
				logger.Printf("publish %s failed: %v", match.Topic, err)
			}
		}
		return
	}

	matchLogs := logEventMatches(r.Context(), h.logs, logger, event, rules)
	logger.Printf("event provider=%s name=%s topics=%v", event.Provider, event.Name, topicsFromLogRecords(matchLogs))
	for _, record := range matchLogs {
		event.LogID = record.ID
		if err := h.publisher.PublishForDrivers(r.Context(), record.Topic, event, record.Drivers); err != nil {
			logger.Printf("publish %s failed: %v", record.Topic, err)
			if err := h.logs.UpdateEventLogStatus(r.Context(), record.ID, eventLogStatusFailed, err.Error()); err != nil {
				logger.Printf("event log update failed: %v", err)
			}
		}
	}
}

func gitlabNamespaceInfo(raw []byte) (string, string) {
	var payload struct {
		Project struct {
			ID                int64  `json:"id"`
			PathWithNamespace string `json:"path_with_namespace"`
			Path              string `json:"path"`
			Namespace         string `json:"namespace"`
		} `json:"project"`
		ProjectID int64 `json:"project_id"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", ""
	}
	repoID := payload.Project.ID
	if repoID == 0 {
		repoID = payload.ProjectID
	}
	namespaceID := ""
	if repoID > 0 {
		namespaceID = strconv.FormatInt(repoID, 10)
	}
	namespaceName := strings.TrimSpace(payload.Project.PathWithNamespace)
	if namespaceName == "" && payload.Project.Namespace != "" && payload.Project.Path != "" {
		namespaceName = payload.Project.Namespace + "/" + payload.Project.Path
	}
	return namespaceID, namespaceName
}
