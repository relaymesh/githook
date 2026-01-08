package controllers

import (
	"context"
	"log"

	"githooks/sdk/go/worker"
)

func HandlePullRequestReady(ctx context.Context, evt *worker.Event) error {
	if gh, ok := worker.GitHubClient(evt); ok {
		_ = gh
	}
	log.Printf("topic=%s provider=%s", evt.Topic, evt.Provider)
	return nil
}

func HandlePullRequestMerged(ctx context.Context, evt *worker.Event) error {
	if gh, ok := worker.GitHubClient(evt); ok {
		_ = gh
	}
	log.Printf("topic=%s provider=%s", evt.Topic, evt.Provider)
	return nil
}
