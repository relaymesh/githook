package worker

import (
	"testing"

	"githook/pkg/providers/bitbucket"
	"githook/pkg/providers/github"
	"githook/pkg/providers/gitlab"
)

func TestProviderClientHelpers(t *testing.T) {
	if _, ok := GitHubClient(nil); ok {
		t.Fatalf("expected false for nil event")
	}
	if _, ok := GitLabClient(nil); ok {
		t.Fatalf("expected false for nil event")
	}
	if _, ok := BitbucketClient(nil); ok {
		t.Fatalf("expected false for nil event")
	}

	evt := &Event{Client: &github.Client{}}
	if _, ok := GitHubClient(evt); !ok {
		t.Fatalf("expected github client")
	}
	evt.Client = &gitlab.Client{}
	if _, ok := GitLabClient(evt); !ok {
		t.Fatalf("expected gitlab client")
	}
	evt.Client = &bitbucket.Client{}
	if _, ok := BitbucketClient(evt); !ok {
		t.Fatalf("expected bitbucket client")
	}
}
