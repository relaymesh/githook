package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"githook/pkg/storage"

	ghprovider "githook/pkg/providers/github"
)

func (h *GitHubHandler) resolveStateID(ctx context.Context, raw []byte) (string, string, string, string) {
	installationID, ok, err := ghprovider.InstallationIDFromPayload(raw)
	if err != nil || !ok {
		return "", "", "", ""
	}
	installationIDStr := strconv.FormatInt(installationID, 10)
	if h.store == nil {
		return "", "", installationIDStr, ""
	}
	record, err := h.store.GetInstallationByInstallationID(ctx, "github", installationIDStr)
	if err != nil || record == nil {
		return "", "", installationIDStr, ""
	}
	return record.TenantID, record.AccountID, installationIDStr, record.ProviderInstanceKey
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

		enterpriseID, enterpriseSlug, enterpriseName := recordEnterprise(record)
		if enterprise, ok := extractGitHubEnterprise(raw); ok {
			enterpriseID = enterprise.ID
			enterpriseSlug = enterprise.Slug
			enterpriseName = enterprise.Name
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
				EnterpriseID:        enterpriseID,
				EnterpriseSlug:      enterpriseSlug,
				EnterpriseName:      enterpriseName,
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

type githubEnterprise struct {
	ID   string
	Slug string
	Name string
}

func extractGitHubEnterprise(raw []byte) (githubEnterprise, bool) {
	var payload struct {
		Enterprise struct {
			ID   int64  `json:"id"`
			Slug string `json:"slug"`
			Name string `json:"name"`
		} `json:"enterprise"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return githubEnterprise{}, false
	}
	if payload.Enterprise.ID == 0 && strings.TrimSpace(payload.Enterprise.Slug) == "" && strings.TrimSpace(payload.Enterprise.Name) == "" {
		return githubEnterprise{}, false
	}
	return githubEnterprise{
		ID:   strconv.FormatInt(payload.Enterprise.ID, 10),
		Slug: strings.TrimSpace(payload.Enterprise.Slug),
		Name: strings.TrimSpace(payload.Enterprise.Name),
	}, true
}

func recordEnterprise(record *storage.InstallRecord) (string, string, string) {
	if record == nil {
		return "", "", ""
	}
	return strings.TrimSpace(record.EnterpriseID), strings.TrimSpace(record.EnterpriseSlug), strings.TrimSpace(record.EnterpriseName)
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
