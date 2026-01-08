package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"githooks/pkg/core"
	"githooks/sdk/go/worker"

	gh "github.com/google/go-github/v57/github"
	bb "github.com/ktrysmt/go-bitbucket"
	gl "github.com/xanzy/go-gitlab"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to app config")
	flag.Parse()

	log.SetPrefix("githooks/vercel-worker ")
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	appCfg, err := core.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	subCfg, err := worker.LoadSubscriberConfig(*configPath)
	if err != nil {
		log.Fatalf("load subscriber config: %v", err)
	}

	sub, err := worker.BuildSubscriber(subCfg)
	if err != nil {
		log.Fatalf("subscriber: %v", err)
	}
	defer func() {
		if err := sub.Close(); err != nil {
			log.Printf("subscriber close: %v", err)
		}
	}()

	wk := worker.New(
		worker.WithSubscriber(sub),
		worker.WithTopics("vercel.preview", "vercel.production"),
		worker.WithConcurrency(2),
		worker.WithClientProvider(worker.NewSCMClientProvider(appCfg.Providers)),
		worker.WithListener(worker.Listener{
			OnStart: func(ctx context.Context) { log.Println("worker started") },
			OnExit:  func(ctx context.Context) { log.Println("worker stopped") },
			OnError: func(ctx context.Context, evt *worker.Event, err error) {
				log.Printf("worker error: %v", err)
			},
		}),
	)

	wk.HandleTopic("vercel.preview", func(ctx context.Context, evt *worker.Event) error {
		return handlePreviewComment(ctx, evt)
	})

	wk.HandleTopic("vercel.production", func(ctx context.Context, evt *worker.Event) error {
		log.Printf("intent: trigger production deploy (provider=%s topic=%s)", evt.Provider, evt.Topic)
		return nil
	})

	if err := wk.Run(ctx); err != nil {
		log.Fatal(err)
	}
}

func handlePreviewComment(ctx context.Context, evt *worker.Event) error {
	if evt == nil {
		return nil
	}
	switch evt.Provider {
	case "github":
		client, ok := worker.GitHubClient(evt)
		if !ok {
			log.Printf("intent: github client unavailable")
			return nil
		}
		owner, repo, number := githubPullRequestRef(evt)
		if owner == "" || repo == "" || number == 0 {
			log.Printf("intent: missing github repo or pr number")
			return nil
		}
		body := "Preview deploy queued (dummy)."
		_, _, err := client.Issues.CreateComment(ctx, owner, repo, number, &gh.IssueComment{
			Body: &body,
		})
		return err
	case "gitlab":
		client, ok := worker.GitLabClient(evt)
		if !ok {
			log.Printf("intent: gitlab client unavailable")
			return nil
		}
		project, iid := gitlabMergeRequestRef(evt)
		if project == "" || iid == 0 {
			log.Printf("intent: missing gitlab project or mr iid")
			return nil
		}
		body := "Preview deploy queued (dummy)."
		_, _, err := client.Notes.CreateMergeRequestNote(project, iid, &gl.CreateMergeRequestNoteOptions{
			Body: &body,
		})
		return err
	case "bitbucket":
		client, ok := worker.BitbucketClient(evt)
		if !ok {
			log.Printf("intent: bitbucket client unavailable")
			return nil
		}
		owner, repo, prID := bitbucketPullRequestRef(evt)
		if owner == "" || repo == "" || prID == "" {
			log.Printf("intent: missing bitbucket repo or pr id")
			return nil
		}
		body := "Preview deploy queued (dummy)."
		opts := (&bb.PullRequestCommentOptions{
			Owner:         owner,
			RepoSlug:      repo,
			PullRequestID: prID,
			Content:       body,
		}).WithContext(ctx)
		_, err := client.Repositories.PullRequests.AddComment(opts)
		return err
	default:
		log.Printf("intent: preview comment not supported for provider=%s", evt.Provider)
		return nil
	}
}

func githubPullRequestRef(evt *worker.Event) (string, string, int) {
	repo, _ := evt.Normalized["repository"].(map[string]interface{})
	full, _ := repo["full_name"].(string)
	if full != "" {
		parts := strings.Split(full, "/")
		if len(parts) == 2 {
			number := intFromMap(evt.Normalized, "pull_request", "number")
			return parts[0], parts[1], number
		}
	}
	return "", "", 0
}

func gitlabMergeRequestRef(evt *worker.Event) (string, int) {
	project, _ := stringFromMap(evt.Normalized, "project", "path_with_namespace")
	iid := intFromMap(evt.Normalized, "object_attributes", "iid")
	return project, iid
}

func bitbucketPullRequestRef(evt *worker.Event) (string, string, string) {
	repo, _ := evt.Normalized["repository"].(map[string]interface{})
	full, _ := repo["full_name"].(string)
	if full != "" {
		parts := strings.Split(full, "/")
		if len(parts) == 2 {
			id, _ := stringFromMap(evt.Normalized, "pullrequest", "id")
			return parts[0], parts[1], id
		}
	}
	return "", "", ""
}

func stringFromMap(root map[string]interface{}, path ...string) (string, bool) {
	current := root
	for i, key := range path {
		value, ok := current[key]
		if !ok {
			return "", false
		}
		if i == len(path)-1 {
			str, ok := value.(string)
			return str, ok
		}
		next, ok := value.(map[string]interface{})
		if !ok {
			return "", false
		}
		current = next
	}
	return "", false
}

func intFromMap(root map[string]interface{}, path ...string) int {
	current := root
	for i, key := range path {
		value, ok := current[key]
		if !ok {
			return 0
		}
		if i == len(path)-1 {
			switch v := value.(type) {
			case int:
				return v
			case int64:
				return int(v)
			case float64:
				return int(v)
			}
			return 0
		}
		next, ok := value.(map[string]interface{})
		if !ok {
			return 0
		}
		current = next
	}
	return 0
}
