package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"githook/pkg/auth"
	"githook/pkg/storage"
)

// SyncGitLabNamespaces fetches repositories and upserts them into the namespace store.
func SyncGitLabNamespaces(ctx context.Context, store storage.NamespaceStore, cfg auth.ProviderConfig, accessToken, accountID, installationID, instanceKey string) error {
	if !namespaceStoreAvailable(store) {
		return nil
	}
	if accessToken == "" {
		return nil
	}
	baseURL := strings.TrimRight(cfg.API.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://gitlab.com/api/v4"
	}

	page := 1
	for {
		endpoint := fmt.Sprintf("%s/projects?membership=true&per_page=100&page=%d", baseURL, page)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		var payload []struct {
			ID                int64  `json:"id"`
			Name              string `json:"name"`
			PathWithNamespace string `json:"path_with_namespace"`
			Visibility        string `json:"visibility"`
			DefaultBranch     string `json:"default_branch"`
			WebURL            string `json:"web_url"`
			SSHURL            string `json:"ssh_url_to_repo"`
			HTTPURL           string `json:"http_url_to_repo"`
			Namespace         struct {
				Name string `json:"name"`
				Path string `json:"path"`
			} `json:"namespace"`
		}
		err = json.NewDecoder(resp.Body).Decode(&payload)
		resp.Body.Close()
		if err != nil {
			return err
		}
		for _, repo := range payload {
			existing, err := store.GetNamespace(ctx, "gitlab", strconv.FormatInt(repo.ID, 10), instanceKey)
			if err != nil {
				return err
			}
			record := storage.NamespaceRecord{
				Provider:            "gitlab",
				ProviderInstanceKey: instanceKey,
				AccountID:           accountID,
				InstallationID:      installationID,
				RepoID:              strconv.FormatInt(repo.ID, 10),
				Owner:               repo.Namespace.Path,
				RepoName:            repo.Name,
				FullName:            repo.PathWithNamespace,
				Visibility:          repo.Visibility,
				DefaultBranch:       repo.DefaultBranch,
				HTTPURL:             repo.HTTPURL,
				SSHURL:              repo.SSHURL,
				WebhooksEnabled:     existingWebhooks(existing, false),
			}
			if err := store.UpsertNamespace(ctx, record); err != nil {
				return err
			}
		}
		if resp.Header.Get("X-Next-Page") == "" {
			break
		}
		page++
	}
	return nil
}

// SyncBitbucketNamespaces fetches repositories and upserts them into the namespace store.
func SyncBitbucketNamespaces(ctx context.Context, store storage.NamespaceStore, cfg auth.ProviderConfig, accessToken, accountID, installationID, instanceKey string) error {
	if !namespaceStoreAvailable(store) {
		return nil
	}
	if accessToken == "" {
		return nil
	}
	baseURL := strings.TrimRight(cfg.API.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.bitbucket.org/2.0"
	}
	nextURL := baseURL + "/repositories?role=member"
	for nextURL != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, nextURL, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		var payload struct {
			Next   string `json:"next"`
			Values []struct {
				UUID     string `json:"uuid"`
				Name     string `json:"name"`
				FullName string `json:"full_name"`
				Owner    struct {
					Username    string `json:"username"`
					DisplayName string `json:"display_name"`
				} `json:"owner"`
				MainBranch struct {
					Name string `json:"name"`
				} `json:"mainbranch"`
				Links struct {
					HTML struct {
						Href string `json:"href"`
					} `json:"html"`
					Clone []struct {
						Href string `json:"href"`
						Name string `json:"name"`
					} `json:"clone"`
				} `json:"links"`
				IsPrivate bool `json:"is_private"`
			} `json:"values"`
		}
		err = json.NewDecoder(resp.Body).Decode(&payload)
		resp.Body.Close()
		if err != nil {
			return err
		}
		for _, repo := range payload.Values {
			owner := repo.Owner.Username
			if owner == "" {
				owner = repo.Owner.DisplayName
			}
			visibility := "public"
			if repo.IsPrivate {
				visibility = "private"
			}
			sshURL := ""
			for _, clone := range repo.Links.Clone {
				if clone.Name == "ssh" {
					sshURL = clone.Href
					break
				}
			}
			existing, err := store.GetNamespace(ctx, "bitbucket", repo.UUID, instanceKey)
			if err != nil {
				return err
			}
			record := storage.NamespaceRecord{
				Provider:            "bitbucket",
				ProviderInstanceKey: instanceKey,
				AccountID:           accountID,
				InstallationID:      installationID,
				RepoID:              repo.UUID,
				Owner:               owner,
				RepoName:            repo.Name,
				FullName:            repo.FullName,
				Visibility:          visibility,
				DefaultBranch:       repo.MainBranch.Name,
				HTTPURL:             repo.Links.HTML.Href,
				SSHURL:              sshURL,
				WebhooksEnabled:     existingWebhooks(existing, false),
			}
			if err := store.UpsertNamespace(ctx, record); err != nil {
				return err
			}
		}
		nextURL = payload.Next
	}
	return nil
}

func existingWebhooks(record *storage.NamespaceRecord, defaultValue bool) bool {
	if record == nil {
		return defaultValue
	}
	return record.WebhooksEnabled
}
