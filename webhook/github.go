package webhook

import (
	"encoding/json"
	"log"
	"net/http"

	"githooks/internal"

	"github.com/go-playground/webhooks/v6/github"
)

type GitHubHandler struct {
	hook      *github.Webhook
	rules     *internal.RuleEngine
	publisher internal.Publisher
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

func NewGitHubHandler(secret string, rules *internal.RuleEngine, publisher internal.Publisher) (*GitHubHandler, error) {
	hook, err := github.New(github.Options.Secret(secret))
	if err != nil {
		return nil, err
	}

	return &GitHubHandler{hook: hook, rules: rules, publisher: publisher}, nil
}

func (h *GitHubHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	payload, err := h.hook.Parse(r, githubEvents...)
	if err != nil {
		log.Printf("github parse failed: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	eventName := r.Header.Get("X-GitHub-Event")
	switch event := payload.(type) {
	case github.PingPayload:
		w.WriteHeader(http.StatusOK)
		return
	default:
		data, err := flattenPayload(event)
		if err == nil {
			h.emit(r, internal.Event{Provider: "github", Name: eventName, Data: data})
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (h *GitHubHandler) emit(r *http.Request, event internal.Event) {
	topics := h.rules.Evaluate(event)
	log.Printf("event provider=%s name=%s topics=%v", event.Provider, event.Name, topics)
	for _, match := range topics {
		if err := h.publisher.PublishForDrivers(r.Context(), match.Topic, event, match.Drivers); err != nil {
			log.Printf("publish %s failed: %v", match.Topic, err)
		}
	}
}

func jsonToMap(payload interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func flattenPayload(payload interface{}) (map[string]interface{}, error) {
	raw, err := jsonToMap(payload)
	if err != nil {
		return nil, err
	}
	return internal.Flatten(raw), nil
}
