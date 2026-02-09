package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"githooks/pkg/core"
	"githooks/pkg/providers/github"
	worker "githooks/sdk/go/worker"

	_ "github.com/lib/pq"
)

type retryOnce struct{}

type attemptKey struct{}

type attempts struct {
	count int
}

func (retryOnce) OnError(ctx context.Context, evt *worker.Event, err error) worker.RetryDecision {
	if evt == nil {
		return worker.RetryDecision{Retry: false, Nack: true}
	}
	if value := ctx.Value(attemptKey{}); value != nil {
		if state, ok := value.(*attempts); ok && state.count > 0 {
			return worker.RetryDecision{Retry: false, Nack: false}
		}
		if state, ok := value.(*attempts); ok {
			state.count++
		}
	}
	return worker.RetryDecision{Retry: true, Nack: true}
}

// splitRepoFullName splits "owner/repo" into ["owner", "repo"]
func splitRepoFullName(fullName string) []string {
	return strings.SplitN(fullName, "/", 2)
}

func main() {
	configPath := flag.String("config", "config.yaml", "Path to app config")
	driver := flag.String("driver", "", "Override subscriber driver (amqp|nats|kafka|sql|gochannel)")
	flag.Parse()

	log.SetPrefix("githooks/worker-example ")
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
	if *driver != "" {
		subCfg.Driver = *driver
		subCfg.Drivers = nil
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
		worker.WithTopics("pr.opened.ready", "pr.merged", "github.commit.created"),
		worker.WithConcurrency(5),
		worker.WithRetry(retryOnce{}),
		worker.WithClientProvider(worker.NewSCMClientProvider(appCfg.Providers)),
		worker.WithListener(worker.Listener{
			OnStart: func(ctx context.Context) { log.Println("worker started") },
			OnExit:  func(ctx context.Context) { log.Println("worker stopped") },
			OnError: func(ctx context.Context, evt *worker.Event, err error) {
				log.Printf("worker error: %v", err)
			},
			OnMessageFinish: func(ctx context.Context, evt *worker.Event, err error) {
				log.Printf("finished provider=%s type=%s err=%v", evt.Provider, evt.Type, err)
			},
		}),
	)

	wk.HandleTopic("pr.merged", func(ctx context.Context, evt *worker.Event) error {
		if evt.Provider != "github" {
			return nil
		}

		if driver := evt.Metadata["driver"]; driver != "" {
			log.Printf("driver=%s topic=%s provider=%s", driver, evt.Topic, evt.Provider)
		}

		if evt.Client != nil {
			gh := evt.Client.(*github.Client)
			_ = gh
		}

		action, _ := evt.Normalized["action"].(string)
		pr, _ := evt.Normalized["pull_request"].(map[string]interface{})
		draft, _ := pr["draft"].(bool)
		if action == "opened" && !draft {
			log.Printf("ready PR: topic=%s", evt.Topic)
		}
		return nil
	})

	wk.HandleTopic("pr.opened.ready", func(ctx context.Context, evt *worker.Event) error {
		if evt.Provider != "github" {
			return nil
		}

		if driver := evt.Metadata["driver"]; driver != "" {
			log.Printf("driver=%s topic=%s provider=%s", driver, evt.Topic, evt.Provider)
		}

		if evt.Client != nil {
			gh := evt.Client.(*github.Client)
			_ = gh
		}

		action, _ := evt.Normalized["action"].(string)
		pr, _ := evt.Normalized["pull_request"].(map[string]interface{})
		draft, _ := pr["draft"].(bool)
		if action == "opened" && !draft {
			log.Printf("ready PR: topic=%s", evt.Topic)
		}
		return nil
	})

	wk.HandleTopic("github.commit.created", func(ctx context.Context, evt *worker.Event) error {
		if evt.Provider != "github" {
			return nil
		}

		if driver := evt.Metadata["driver"]; driver != "" {
			log.Printf("driver=%s topic=%s provider=%s", driver, evt.Topic, evt.Provider)
		}

		// Extract repository and commit information from the event
		// Handle different event types (push, check_suite, etc.)
		repository, _ := evt.Normalized["repository"].(map[string]interface{})
		var headCommit map[string]interface{}

		// Try to get head_commit from top level (push events)
		headCommit, _ = evt.Normalized["head_commit"].(map[string]interface{})

		// If not found, try check_suite.head_commit (check_suite events)
		if headCommit == nil {
			checkSuite, _ := evt.Normalized["check_suite"].(map[string]interface{})
			if checkSuite != nil {
				headCommit, _ = checkSuite["head_commit"].(map[string]interface{})
			}
		}

		if repository == nil {
			log.Printf("github.commit.created: missing repository data (event type: %s)", evt.Type)
			return nil
		}

		if headCommit == nil {
			log.Printf("github.commit.created: missing head_commit data (event type: %s)", evt.Type)
			return nil
		}

		repoFullName, _ := repository["full_name"].(string)
		commitID, _ := headCommit["id"].(string)
		commitMessage, _ := headCommit["message"].(string)

		log.Printf("github.commit.created: repo=%s commit=%s message=%q", repoFullName, commitID, commitMessage)

		// Test GitHub client functionality
		if evt.Client != nil {
			ghClient := evt.Client.(*github.Client)

			// Parse owner and repo from full_name (e.g., "owner/repo")
			parts := splitRepoFullName(repoFullName)
			if len(parts) != 2 {
				log.Printf("github.commit.created: invalid repo full_name format: %s", repoFullName)
				return nil
			}
			owner, repo := parts[0], parts[1]

			// Test 1: Get repository information using GitHub SDK
			log.Printf("Testing GitHub client: fetching repository %s/%s", owner, repo)
			repoInfo, _, err := ghClient.Repositories.Get(ctx, owner, repo)
			if err != nil {
				log.Printf("github.commit.created: failed to get repository: %v", err)
				return fmt.Errorf("get repository: %w", err)
			}
			log.Printf("Repository info: name=%s stars=%d forks=%d",
				repoInfo.GetName(), repoInfo.GetStargazersCount(), repoInfo.GetForksCount())

			// Test 2: Get commit details using GitHub SDK
			if commitID != "" {
				log.Printf("Testing GitHub client: fetching commit %s", commitID[:7])
				commit, _, err := ghClient.Repositories.GetCommit(ctx, owner, repo, commitID, nil)
				if err != nil {
					log.Printf("github.commit.created: failed to get commit: %v", err)
					return fmt.Errorf("get commit: %w", err)
				}

				author := "unknown"
				if commit.GetAuthor() != nil && commit.GetAuthor().GetLogin() != "" {
					author = commit.GetAuthor().GetLogin()
				}

				log.Printf("Commit details: sha=%s author=%s files_changed=%d",
					commit.GetSHA()[:7], author, len(commit.Files))

				// Log stats
				stats := commit.GetStats()
				if stats != nil {
					log.Printf("Commit stats: additions=%d deletions=%d total=%d",
						stats.GetAdditions(), stats.GetDeletions(), stats.GetTotal())
				}
			}

			log.Printf("✅ GitHub client test passed for commit %s", commitID[:7])
		} else {
			log.Printf("⚠️  No GitHub client available for this event")
		}

		return nil
	})

	exampleCtx := context.WithValue(ctx, attemptKey{}, &attempts{})
	if err := wk.Run(exampleCtx); err != nil {
		log.Fatal(err)
	}
}
