package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"githook/sdk/go/worker"
)

func main() {
	configPath := flag.String("config", "example/realworld/app.yaml", "Path to app config")
	flag.Parse()

	log.SetPrefix("githook/worker-pr ")
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
		worker.WithTopics("pr.opened.ready", "pr.merged"),
		worker.WithConcurrency(10),
	)

	wk.HandleTopic("pr.opened.ready", func(ctx context.Context, evt *worker.Event) error {
		action, _ := evt.Normalized["action"].(string)
		pr, _ := evt.Normalized["pull_request"].(map[string]interface{})
		draft, _ := pr["draft"].(bool)
		if action == "opened" && !draft {
			log.Printf("PR ready: topic=%s", evt.Topic)
		}
		return nil
	})

	wk.HandleTopic("pr.merged", func(ctx context.Context, evt *worker.Event) error {
		pr, ok := evt.Normalized["pull_request"].(map[string]interface{})
		if !ok {
			return nil
		}
		if merged, ok := pr["merged"].(bool); ok && merged {
			log.Printf("PR merged: topic=%s", evt.Topic)
		}
		return nil
	})

	if err := wk.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
