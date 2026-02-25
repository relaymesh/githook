package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	worker "github.com/relaymesh/githook/sdk/go/worker"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	endpoint := os.Getenv("GITHOOK_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:8080"
	}

	wk := worker.New(
		worker.WithEndpoint(endpoint),
	)

	wk.HandleRule("<rule-id>", func(ctx context.Context, evt *worker.Event) error {
		if evt == nil {
			return nil
		}
		log.Printf("topic=%s provider=%s type=%s", evt.Topic, evt.Provider, evt.Type)
		return nil
	})

	if err := wk.Run(ctx); err != nil {
		log.Fatalf("worker run failed: %v", err)
	}
}
