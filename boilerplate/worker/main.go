package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"githooks/boilerplate/worker/controllers"
	"githooks/pkg/core"
	"githooks/sdk/go/worker"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to app config")
	flag.Parse()

	log.SetPrefix("githooks/worker-boilerplate ")
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

	topics, err := worker.LoadTopicsFromConfig(*configPath)
	if err != nil {
		log.Fatalf("load topics: %v", err)
	}
	if len(topics) == 0 {
		topics = []string{"pr.opened.ready", "pr.merged"}
	}

	wk := worker.New(
		worker.WithSubscriber(sub),
		worker.WithTopics(topics...),
		worker.WithConcurrency(5),
		worker.WithClientProvider(worker.NewSCMClientProvider(appCfg.Providers)),
	)

	wk.HandleTopic("pr.opened.ready", controllers.HandlePullRequestReady)
	wk.HandleTopic("pr.merged", controllers.HandlePullRequestMerged)

	if err := wk.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
