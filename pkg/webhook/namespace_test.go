package webhook

import "testing"

func TestGitlabNamespaceInfo(t *testing.T) {
	raw := []byte(`{"project":{"id":123,"path_with_namespace":"org/repo"}}`)
	id, name := gitlabNamespaceInfo(raw)
	if id != "123" || name != "org/repo" {
		t.Fatalf("unexpected gitlab namespace info: %q %q", id, name)
	}
	raw = []byte(`{"project_id":45,"project":{"namespace":"org","path":"repo"}}`)
	id, name = gitlabNamespaceInfo(raw)
	if id != "45" || name != "org/repo" {
		t.Fatalf("unexpected gitlab namespace info: %q %q", id, name)
	}
}

func TestBitbucketNamespaceInfo(t *testing.T) {
	raw := []byte(`{"repository":{"uuid":"{id}","full_name":"org/repo"}}`)
	id, name := bitbucketNamespaceInfo(raw)
	if id != "{id}" || name != "org/repo" {
		t.Fatalf("unexpected bitbucket namespace info: %q %q", id, name)
	}
	raw = []byte(`{"repository":{"workspace":{"slug":"org"},"name":"repo"}}`)
	_, name = bitbucketNamespaceInfo(raw)
	if name != "org/repo" {
		t.Fatalf("unexpected bitbucket namespace info: %q", name)
	}
}
