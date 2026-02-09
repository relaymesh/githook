package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"

	"githook/pkg/core"
	"githook/pkg/storage"

	"github.com/go-playground/webhooks/v6/bitbucket"
)

// BitbucketHandler handles incoming webhooks from Bitbucket.
type BitbucketHandler struct {
	hook        *bitbucket.Webhook
	rules       *core.RuleEngine
	publisher   core.Publisher
	logger      *log.Logger
	maxBody     int64
	debugEvents bool
	namespaces  storage.NamespaceStore
}

var bitbucketEvents = []bitbucket.Event{
	bitbucket.RepoPushEvent,
	bitbucket.RepoForkEvent,
	bitbucket.RepoUpdatedEvent,
	bitbucket.RepoCommitCommentCreatedEvent,
	bitbucket.RepoCommitStatusCreatedEvent,
	bitbucket.RepoCommitStatusUpdatedEvent,
	bitbucket.IssueCreatedEvent,
	bitbucket.IssueUpdatedEvent,
	bitbucket.IssueCommentCreatedEvent,
	bitbucket.PullRequestCreatedEvent,
	bitbucket.PullRequestUpdatedEvent,
	bitbucket.PullRequestApprovedEvent,
	bitbucket.PullRequestUnapprovedEvent,
	bitbucket.PullRequestMergedEvent,
	bitbucket.PullRequestDeclinedEvent,
	bitbucket.PullRequestCommentCreatedEvent,
	bitbucket.PullRequestCommentUpdatedEvent,
	bitbucket.PullRequestCommentDeletedEvent,
}

// NewBitbucketHandler creates a new BitbucketHandler.
func NewBitbucketHandler(secret string, rules *core.RuleEngine, publisher core.Publisher, logger *log.Logger, maxBody int64, debugEvents bool, namespaces storage.NamespaceStore) (*BitbucketHandler, error) {
	options := make([]bitbucket.Option, 0, 1)
	if secret != "" {
		options = append(options, bitbucket.Options.UUID(secret))
	}
	hook, err := bitbucket.New(options...)
	if err != nil {
		return nil, err
	}
	if logger == nil {
		logger = log.Default()
	}
	return &BitbucketHandler{hook: hook, rules: rules, publisher: publisher, logger: logger, maxBody: maxBody, debugEvents: debugEvents, namespaces: namespaces}, nil
}

// ServeHTTP handles an incoming HTTP request.
func (h *BitbucketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
		logDebugEvent(logger, "bitbucket", r.Header.Get("X-Event-Key"), rawBody)
	}

	payload, err := h.hook.Parse(r, bitbucketEvents...)
	if err != nil {
		if errors.Is(err, bitbucket.ErrMissingHookUUIDHeader) {
			logger.Printf("bitbucket parse warning: %v; skipping UUID verification", err)
			r.Body = io.NopCloser(bytes.NewReader(rawBody))
			unverified, fallbackErr := bitbucket.New()
			if fallbackErr == nil {
				payload, err = unverified.Parse(r, bitbucketEvents...)
			} else {
				err = fallbackErr
			}
		}
		if err != nil {
			logger.Printf("bitbucket parse failed: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	eventName := r.Header.Get("X-Event-Key")
	switch payload.(type) {
	default:
		rawObject, data := rawObjectAndFlatten(rawBody)
		stateID, installationID := h.resolveStateID(r.Context(), rawBody)
		if installationID == "" {
			logger.Printf("bitbucket webhook ignored: missing installation_id")
			w.WriteHeader(http.StatusOK)
			return
		}
		h.emit(r, logger, core.Event{
			Provider:       "bitbucket",
			Name:           eventName,
			RequestID:      reqID,
			Data:           data,
			RawPayload:     rawBody,
			RawObject:      rawObject,
			StateID:        stateID,
			InstallationID: installationID,
		})
	}

	w.WriteHeader(http.StatusOK)
}

func (h *BitbucketHandler) resolveStateID(ctx context.Context, raw []byte) (string, string) {
	if h.namespaces == nil {
		return "", ""
	}
	var payload struct {
		Repository struct {
			UUID string `json:"uuid"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", ""
	}
	repoID := strings.TrimSpace(payload.Repository.UUID)
	if repoID == "" {
		return "", ""
	}
	record, err := h.namespaces.GetNamespace(ctx, "bitbucket", repoID, "")
	if err != nil || record == nil {
		return "", ""
	}
	return record.AccountID, record.InstallationID
}

func (h *BitbucketHandler) emit(r *http.Request, logger *log.Logger, event core.Event) {
	tenantID := storage.TenantFromContext(r.Context())
	topics := h.rules.EvaluateForTenantWithLogger(event, tenantID, logger)
	logger.Printf("event provider=%s name=%s topics=%v", event.Provider, event.Name, topics)
	for _, match := range topics {
		if err := h.publisher.PublishForDrivers(r.Context(), match.Topic, event, match.Drivers); err != nil {
			logger.Printf("publish %s failed: %v", match.Topic, err)
		}
	}
}
