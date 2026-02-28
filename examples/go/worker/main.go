package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"

	worker "github.com/relaymesh/githook/sdk/go/worker"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	endpoint := os.Getenv("GITHOOK_ENDPOINT")
	if endpoint == "" {
		endpoint = "https://relaymesh.vercel.app/api/connect"
	}
	ruleID := os.Getenv("GITHOOK_RULE_ID")
	if ruleID == "" {
		ruleID = "85101e9f-3bcf-4ed0-b561-750c270ef6c3"
	}

	wk := worker.New(
		worker.WithEndpoint(endpoint),
		worker.WithClientProvider(worker.NewRemoteSCMClientProvider()),
	)

	var attempts atomic.Uint64

	wk.HandleRule(ruleID, func(ctx context.Context, evt *worker.Event) (err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				err = fmt.Errorf("panic recovered: %v", recovered)
			}
		}()

		if evt == nil {
			return nil
		}

		seq := attempts.Add(1)
		if seq%2 == 0 {
			err = fmt.Errorf("intentional failure for status test (seq=%d)", seq)
			log.Printf("handler failed seq=%d err=%v", seq, err)
			return err
		}

		log.Printf("handler success seq=%d topic=%s provider=%s type=%s", seq, evt.Topic, evt.Provider, evt.Type)
		log.Printf("topic=%s provider=%s type=%s", evt.Topic, evt.Provider, evt.Type)

		ghClient, ok := worker.GitHubClient(evt)
		if !ok {
			log.Printf("github client not available for provider=%s (installation may not be configured)", evt.Provider)
			return nil
		}

		owner, repo := repositoryFromEvent(evt)
		if owner == "" || repo == "" {
			log.Printf("repository info missing in payload; skipping github read")
			return nil
		}

		repository, _, err := ghClient.Repositories.Get(ctx, owner, repo)
		if err != nil {
			log.Printf("github read failed owner=%s repo=%s err=%v", owner, repo, err)
			return nil
		}

		log.Printf("github read ok full_name=%s private=%t default_branch=%s", repository.GetFullName(), repository.GetPrivate(), repository.GetDefaultBranch())
		return nil
	})

	if err := wk.Run(ctx); err != nil {
		log.Fatalf("worker run failed: %v", err)
	}
}

func repositoryFromEvent(evt *worker.Event) (string, string) {
	if evt == nil || evt.Normalized == nil {
		return "", ""
	}

	repoValue, ok := evt.Normalized["repository"]
	if !ok {
		return "", ""
	}
	repoMap, ok := repoValue.(map[string]interface{})
	if !ok {
		return "", ""
	}

	if fullName, ok := repoMap["full_name"].(string); ok {
		parts := strings.SplitN(strings.TrimSpace(fullName), "/", 2)
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			return parts[0], parts[1]
		}
	}

	name, _ := repoMap["name"].(string)
	ownerMap, _ := repoMap["owner"].(map[string]interface{})
	owner, _ := ownerMap["login"].(string)

	return strings.TrimSpace(owner), strings.TrimSpace(name)
}
