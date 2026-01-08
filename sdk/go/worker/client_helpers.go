package worker

import (
	"githooks/pkg/providers/bitbucket"
	"githooks/pkg/providers/github"
	"githooks/pkg/providers/gitlab"
)

// GitHubClient returns the GitHub client from an event if available.
func GitHubClient(evt *Event) (*github.Client, bool) {
	if evt == nil {
		return nil, false
	}
	client, ok := evt.Client.(*github.Client)
	return client, ok
}

// GitLabClient returns the GitLab client from an event if available.
func GitLabClient(evt *Event) (*gitlab.Client, bool) {
	if evt == nil {
		return nil, false
	}
	client, ok := evt.Client.(*gitlab.Client)
	return client, ok
}

// BitbucketClient returns the Bitbucket client from an event if available.
func BitbucketClient(evt *Event) (*bitbucket.Client, bool) {
	if evt == nil {
		return nil, false
	}
	client, ok := evt.Client.(*bitbucket.Client)
	return client, ok
}
