package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"githook/pkg/core"
	"githook/pkg/storage"

	ghprovider "githook/pkg/providers/github"

	"github.com/go-playground/webhooks/v6/github"
)

// GitHubHandler handles incoming webhooks from GitHub.
type GitHubHandler struct {
	hook         *github.Webhook
	fallbackHook *github.Webhook
	secret       string
	rules        *core.RuleEngine
	publisher    core.Publisher
	logger       *log.Logger
	maxBody      int64
	debugEvents  bool
	store        storage.Store
	namespaces   storage.NamespaceStore
	logs         storage.EventLogStore
}

var githubEvents = []github.Event{
	github.CheckRunEvent,
	github.CheckSuiteEvent,
	github.CommitCommentEvent,
	github.CreateEvent,
	github.DeleteEvent,
	github.DependabotAlertEvent,
	github.DeployKeyEvent,
	github.DeploymentEvent,
	github.DeploymentStatusEvent,
	github.ForkEvent,
	github.GollumEvent,
	github.InstallationEvent,
	github.InstallationRepositoriesEvent,
	github.IntegrationInstallationEvent,
	github.IntegrationInstallationRepositoriesEvent,
	github.IssueCommentEvent,
	github.IssuesEvent,
	github.LabelEvent,
	github.MemberEvent,
	github.MembershipEvent,
	github.MilestoneEvent,
	github.MetaEvent,
	github.OrganizationEvent,
	github.OrgBlockEvent,
	github.PageBuildEvent,
	github.PingEvent,
	github.ProjectCardEvent,
	github.ProjectColumnEvent,
	github.ProjectEvent,
	github.PublicEvent,
	github.PullRequestEvent,
	github.PullRequestReviewEvent,
	github.PullRequestReviewCommentEvent,
	github.PushEvent,
	github.ReleaseEvent,
	github.RepositoryEvent,
	github.RepositoryVulnerabilityAlertEvent,
	github.SecurityAdvisoryEvent,
	github.StatusEvent,
	github.TeamEvent,
	github.TeamAddEvent,
	github.WatchEvent,
	github.WorkflowDispatchEvent,
	github.WorkflowJobEvent,
	github.WorkflowRunEvent,
	github.GitHubAppAuthorizationEvent,
}

// NewGitHubHandler creates a new GitHubHandler.
func NewGitHubHandler(secret string, rules *core.RuleEngine, publisher core.Publisher, logger *log.Logger, maxBody int64, debugEvents bool, store storage.Store, namespaces storage.NamespaceStore, logs storage.EventLogStore) (*GitHubHandler, error) {
	hook, err := github.New(github.Options.Secret(secret))
	if err != nil {
		return nil, err
	}
	fallbackHook, err := github.New()
	if err != nil {
		return nil, err
	}

	if logger == nil {
		logger = log.Default()
	}
	return &GitHubHandler{
		hook:         hook,
		fallbackHook: fallbackHook,
		secret:       secret,
		rules:        rules,
		publisher:    publisher,
		logger:       logger,
		maxBody:      maxBody,
		debugEvents:  debugEvents,
		store:        store,
		namespaces:   namespaces,
		logs:         logs,
	}, nil
}

// ServeHTTP handles an incoming HTTP request.
func (h *GitHubHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
		logDebugEvent(logger, "github", r.Header.Get("X-GitHub-Event"), rawBody)
	}

	payload, err := h.hook.Parse(r, githubEvents...)
	if err != nil {
		if errors.Is(err, github.ErrMissingHubSignatureHeader) && h.secret != "" {
			sha1Header := r.Header.Get("X-Hub-Signature")
			if sha1Header != "" && verifyGitHubSHA1(h.secret, rawBody, sha1Header) {
				logger.Printf("github parse warning: %v; accepted sha1 signature", err)
				r.Body = io.NopCloser(bytes.NewReader(rawBody))
				payload, err = h.fallbackHook.Parse(r, githubEvents...)
			}
		}
		if err != nil {
			logger.Printf("github parse failed: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	eventName := r.Header.Get("X-GitHub-Event")
	switch payload.(type) {
	case github.PingPayload:
		w.WriteHeader(http.StatusOK)
		return
	default:
		rawObject, data := rawObjectAndFlatten(rawBody)
		namespaceID, namespaceName := githubNamespaceInfo(rawBody)
		tenantID, stateID, installationID := h.resolveStateID(r.Context(), rawBody)
		if installationID == "" {
			logger.Printf("github webhook ignored: missing installation_id")
			w.WriteHeader(http.StatusOK)
			return
		}
		ctx := r.Context()
		if tenantID != "" {
			ctx = storage.WithTenant(ctx, tenantID)
		}
		r = r.WithContext(ctx)
		if err := h.applyInstallSystemRules(ctx, eventName, rawBody); err != nil {
			logger.Printf("github install sync failed: %v", err)
		}
		h.emit(r, logger, core.Event{
			Provider:       "github",
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

func (h *GitHubHandler) resolveStateID(ctx context.Context, raw []byte) (string, string, string) {
	installationID, ok, err := ghprovider.InstallationIDFromPayload(raw)
	if err != nil || !ok {
		return "", "", ""
	}
	installationIDStr := strconv.FormatInt(installationID, 10)
	if h.store == nil {
		return "", "", installationIDStr
	}
	record, err := h.store.GetInstallationByInstallationID(ctx, "github", installationIDStr)
	if err != nil || record == nil {
		return "", "", installationIDStr
	}
	return record.TenantID, record.AccountID, installationIDStr
}

func (h *GitHubHandler) applyInstallSystemRules(ctx context.Context, eventName string, raw []byte) error {
	if h.store == nil && h.namespaces == nil {
		return nil
	}
	switch eventName {
	case "installation", "installation_repositories", "integration_installation", "integration_installation_repositories":
	default:
		return nil
	}

	var payload struct {
		Action       string `json:"action"`
		Installation struct {
			ID      int64 `json:"id"`
			Account struct {
				ID    int64  `json:"id"`
				Login string `json:"login"`
			} `json:"account"`
		} `json:"installation"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	if payload.Installation.ID == 0 {
		return fmt.Errorf("installation id missing in webhook")
	}
	action := strings.TrimSpace(payload.Action)
	installationID := strconv.FormatInt(payload.Installation.ID, 10)

	var record *storage.InstallRecord
	storeCtx := ctx
	if h.store != nil {
		found, err := h.store.GetInstallationByInstallationID(ctx, "github", installationID)
		if err != nil {
			return err
		}
		record = found
		if record == nil {
			return nil
		}
		accountID := recordAccountID(record, payload.Installation.Account.ID)
		accountName := recordAccountName(record, payload.Installation.Account.Login)
		tenantID := recordTenantID(record)
		instanceKey := recordInstanceKey(record)
		accessToken := ""
		refreshToken := ""
		var expiresAt *time.Time
		metadataJSON := ""
		if record != nil {
			accessToken = record.AccessToken
			refreshToken = record.RefreshToken
			expiresAt = record.ExpiresAt
			metadataJSON = record.MetadataJSON
		}
		if tenantID != "" {
			storeCtx = storage.WithTenant(ctx, tenantID)
		}

		if action == "deleted" {
			if err := h.store.DeleteInstallation(storeCtx, "github", accountID, installationID, instanceKey); err != nil {
				return err
			}
		} else {
			update := storage.InstallRecord{
				TenantID:            tenantID,
				Provider:            "github",
				AccountID:           accountID,
				AccountName:         accountName,
				InstallationID:      installationID,
				ProviderInstanceKey: instanceKey,
				AccessToken:         accessToken,
				RefreshToken:        refreshToken,
				ExpiresAt:           expiresAt,
				MetadataJSON:        metadataJSON,
			}
			if err := h.store.UpsertInstallation(storeCtx, update); err != nil {
				return err
			}
		}
	}

	if h.namespaces != nil {
		if record == nil {
			return nil
		}
		if action == "deleted" {
			filter := storage.NamespaceFilter{
				Provider:            "github",
				InstallationID:      installationID,
				ProviderInstanceKey: record.ProviderInstanceKey,
			}
			namespaces, err := h.namespaces.ListNamespaces(storeCtx, filter)
			if err != nil {
				return err
			}
			for _, namespace := range namespaces {
				if err := h.namespaces.DeleteNamespace(storeCtx, "github", namespace.RepoID, record.ProviderInstanceKey); err != nil {
					return err
				}
			}
			return nil
		}

		if action == "removed" || action == "repositories_removed" {
			if record == nil {
				return nil
			}
			removed := extractGitHubRemovedRepoIDs(raw, eventName)
			for _, repoID := range removed {
				if err := h.namespaces.DeleteNamespace(storeCtx, "github", repoID, record.ProviderInstanceKey); err != nil {
					return err
				}
			}
		}

		repos := extractGitHubRepos(raw, eventName)
		for _, repo := range repos {
			namespace := storage.NamespaceRecord{
				TenantID:            recordTenantID(record),
				Provider:            "github",
				AccountID:           recordAccountID(record, payload.Installation.Account.ID),
				InstallationID:      installationID,
				ProviderInstanceKey: recordInstanceKey(record),
				RepoID:              repo.ID,
				Owner:               repo.Owner,
				RepoName:            repo.Name,
				FullName:            repo.FullName,
				Visibility:          repo.Visibility,
				DefaultBranch:       repo.DefaultBranch,
				HTTPURL:             repo.HTMLURL,
				SSHURL:              repo.SSHURL,
				WebhooksEnabled:     true,
			}
			if err := h.namespaces.UpsertNamespace(storeCtx, namespace); err != nil {
				return err
			}
		}
	}
	return nil
}

type githubRepo struct {
	ID            string
	Owner         string
	Name          string
	FullName      string
	Visibility    string
	DefaultBranch string
	HTMLURL       string
	SSHURL        string
}

func extractGitHubRepos(raw []byte, eventName string) []githubRepo {
	type repoPayload struct {
		ID            int64  `json:"id"`
		Name          string `json:"name"`
		FullName      string `json:"full_name"`
		Private       bool   `json:"private"`
		DefaultBranch string `json:"default_branch"`
		HTMLURL       string `json:"html_url"`
		SSHURL        string `json:"ssh_url"`
		Owner         struct {
			Login string `json:"login"`
		} `json:"owner"`
	}
	var body struct {
		Repositories        []repoPayload `json:"repositories"`
		RepositoriesAdded   []repoPayload `json:"repositories_added"`
		RepositoriesRemoved []repoPayload `json:"repositories_removed"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil
	}
	candidates := body.Repositories
	if eventName == "installation_repositories" || eventName == "integration_installation_repositories" {
		candidates = body.RepositoriesAdded
	}
	repos := make([]githubRepo, 0, len(candidates))
	for _, repo := range candidates {
		visibility := "public"
		if repo.Private {
			visibility = "private"
		}
		repos = append(repos, githubRepo{
			ID:            strconv.FormatInt(repo.ID, 10),
			Owner:         repo.Owner.Login,
			Name:          repo.Name,
			FullName:      repo.FullName,
			Visibility:    visibility,
			DefaultBranch: repo.DefaultBranch,
			HTMLURL:       repo.HTMLURL,
			SSHURL:        repo.SSHURL,
		})
	}
	return repos
}

func extractGitHubRemovedRepoIDs(raw []byte, eventName string) []string {
	if eventName != "installation_repositories" && eventName != "integration_installation_repositories" {
		return nil
	}
	type repoPayload struct {
		ID int64 `json:"id"`
	}
	var body struct {
		RepositoriesRemoved []repoPayload `json:"repositories_removed"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil
	}
	out := make([]string, 0, len(body.RepositoriesRemoved))
	for _, repo := range body.RepositoriesRemoved {
		if repo.ID == 0 {
			continue
		}
		out = append(out, strconv.FormatInt(repo.ID, 10))
	}
	return out
}

func recordAccountID(record *storage.InstallRecord, providerID int64) string {
	if record != nil && record.AccountID != "" {
		return record.AccountID
	}
	if providerID == 0 {
		return ""
	}
	return strconv.FormatInt(providerID, 10)
}

func recordAccountName(record *storage.InstallRecord, providerName string) string {
	if record != nil && record.AccountName != "" {
		return record.AccountName
	}
	return providerName
}

func recordTenantID(record *storage.InstallRecord) string {
	if record != nil {
		return record.TenantID
	}
	return ""
}

func recordInstanceKey(record *storage.InstallRecord) string {
	if record != nil {
		return record.ProviderInstanceKey
	}
	return ""
}

func (h *GitHubHandler) emit(r *http.Request, logger *log.Logger, event core.Event) {
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

func verifyGitHubSHA1(secret string, body []byte, signature string) bool {
	if secret == "" || len(body) == 0 || signature == "" {
		return false
	}
	signature = strings.TrimPrefix(signature, "sha1=")
	mac := hmac.New(sha1.New, []byte(secret))
	_, _ = mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expected))
}

func githubNamespaceInfo(raw []byte) (string, string) {
	var payload struct {
		Repository struct {
			ID       int64  `json:"id"`
			FullName string `json:"full_name"`
			Name     string `json:"name"`
			Owner    struct {
				Login string `json:"login"`
			} `json:"owner"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", ""
	}
	namespaceID := ""
	if payload.Repository.ID > 0 {
		namespaceID = strconv.FormatInt(payload.Repository.ID, 10)
	}
	namespaceName := strings.TrimSpace(payload.Repository.FullName)
	if namespaceName == "" && payload.Repository.Owner.Login != "" && payload.Repository.Name != "" {
		namespaceName = payload.Repository.Owner.Login + "/" + payload.Repository.Name
	}
	return namespaceID, namespaceName
}
