package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"githooks/sdk/go/worker"
)

func main() {
	configPath := flag.String("config", "example/realworld/app.yaml", "Path to app config")
	flag.Parse()

	log.SetPrefix("githooks/worker-release ")
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	subCfg, err := worker.LoadSubscriberConfig(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
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
		worker.WithTopics("release.tag.created"),
		worker.WithConcurrency(5),
	)

	wk.HandleTopic("release.tag.created", func(ctx context.Context, evt *worker.Event) error {
		refType, _ := evt.Normalized["ref_type"].(string)
		ref, _ := evt.Normalized["ref"].(string)
		if refType == "tag" {
			log.Printf("tag created: %s", ref)
		}
		return nil
	})

	if err := wk.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
