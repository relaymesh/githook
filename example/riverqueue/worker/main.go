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
	configPath := flag.String("config", "example/riverqueue/app.yaml", "Path to config")
	driver := flag.String("driver", "riverqueue", "Subscriber driver name")
	flag.Parse()

	log.SetPrefix("githook/worker ")
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	wk, err := worker.NewFromConfigPathWithDriverFromAPI(*configPath, *driver)
	if err != nil {
		log.Fatalf("init worker: %v", err)
	}

	wk.HandleTopic("pr.opened.ready", func(ctx context.Context, evt *worker.Event) error {
		log.Printf("topic=%s provider=%s type=%s", evt.Topic, evt.Provider, evt.Type)
		return nil
	})

	if err := wk.Run(ctx); err != nil {
		log.Fatalf("worker run: %v", err)
	}
}
