package worker

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/google/go-github/v57/github"
	"github.com/ktrysmt/go-bitbucket"
	"github.com/xanzy/go-gitlab"
	"golang.org/x/oauth2"
)

// GitHubClient returns the GitHub SDK client from an event, if available.
func GitHubClient(evt *Event) (*github.Client, bool) {
	if evt == nil || evt.Client == nil {
		return nil, false
	}
	client, ok := evt.Client.(*github.Client)
	return client, ok
}

// GitLabClient returns the GitLab SDK client from an event, if available.
//
//nolint:staticcheck // legacy go-gitlab client retained for compatibility
func GitLabClient(evt *Event) (*gitlab.Client, bool) {
	if evt == nil || evt.Client == nil {
		return nil, false
	}
	client, ok := evt.Client.(*gitlab.Client)
	return client, ok
}

// BitbucketClient returns the Bitbucket SDK client from an event, if available.
func BitbucketClient(evt *Event) (*bitbucket.Client, bool) {
	if evt == nil || evt.Client == nil {
		return nil, false
	}
	client, ok := evt.Client.(*bitbucket.Client)
	return client, ok
}

func GitHubClientFromEvent(evt *Event) (*github.Client, bool) {
	return GitHubClient(evt)
}

//nolint:staticcheck // legacy go-gitlab client retained for compatibility
func GitLabClientFromEvent(evt *Event) (*gitlab.Client, bool) {
	return GitLabClient(evt)
}

func BitbucketClientFromEvent(evt *Event) (*bitbucket.Client, bool) {
	return BitbucketClient(evt)
}

func NewProviderClient(provider, token, baseURL string) (interface{}, error) {
	return newProviderClient(provider, token, baseURL)
}

func newProviderClient(provider, token, baseURL string) (interface{}, error) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "github":
		return newGitHubClient(token, baseURL)
	case "gitlab":
		return newGitLabClient(token, baseURL)
	case "bitbucket":
		return newBitbucketClient(token, baseURL)
	default:
		return nil, errors.New("unsupported provider")
	}
}

func newGitHubClient(token, baseURL string) (*github.Client, error) {
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("github token is required")
	}
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	httpClient := oauth2.NewClient(context.Background(), ts)
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	client := github.NewClient(httpClient)
	if baseURL == "" || baseURL == defaultGitHubAPIBase {
		return client, nil
	}
	uploadURL := enterpriseUploadURL(baseURL)
	return client.WithEnterpriseURLs(baseURL, uploadURL)
}

//nolint:staticcheck
func newGitLabClient(token, baseURL string) (*gitlab.Client, error) {
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("gitlab token is required")
	}
	opts := []gitlab.ClientOptionFunc{}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = defaultGitLabAPIBase
	}
	if baseURL != "" {
		opts = append(opts, gitlab.WithBaseURL(baseURL))
	}
	return gitlab.NewClient(token, opts...) //nolint:staticcheck // legacy go-gitlab client retained for compatibility
}

func newBitbucketClient(token, baseURL string) (*bitbucket.Client, error) {
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("bitbucket token is required")
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = defaultBitbucketAPIBase
	}
	if baseURL != "" {
		_ = os.Setenv("BITBUCKET_API_BASE_URL", baseURL)
	}
	return bitbucket.NewOAuthbearerToken(token)
}

func enterpriseUploadURL(apiBase string) string {
	base := strings.TrimRight(apiBase, "/")
	switch {
	case strings.HasSuffix(base, "/api/v3"):
		return strings.TrimSuffix(base, "/api/v3") + "/api/uploads"
	case strings.HasSuffix(base, "/api"):
		return strings.TrimSuffix(base, "/api") + "/api/uploads"
	default:
		return base
	}
}

const (
	defaultGitHubAPIBase    = "https://api.github.com"
	defaultGitLabAPIBase    = "https://gitlab.com/api/v4"
	defaultBitbucketAPIBase = "https://api.bitbucket.org/2.0"
)
