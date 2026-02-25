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

	wk := worker.New(
		worker.WithEndpoint("https://githook-app.vercel.app/api/connect"),
	)

	wk.HandleRule("85101e9f-3bcf-4ed0-b561-750c270ef6c3", func(ctx context.Context, evt *worker.Event) error {
		if evt == nil {
			return nil
		}
		log.Printf("topic=%s provider=%s type=%s installation=%s",
			evt.Topic, evt.Provider, evt.Type, evt.Metadata["installation_id"])
		if len(evt.Payload) > 0 {
			log.Printf("payload bytes=%d", len(evt.Payload))
		}
		return nil
	})

	if err := wk.Run(ctx); err != nil {
		log.Fatalf("worker run failed: %v", err)
	}
}
