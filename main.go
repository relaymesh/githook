package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"githooks/internal"
	"githooks/webhook"
)

func main() {
	config, err := internal.LoadConfig("app.yaml")
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ruleEngine, err := internal.NewRuleEngine(internal.RulesConfig{Rules: config.Rules})
	if err != nil {
		log.Fatalf("compile rules: %v", err)
	}

	publisher, err := internal.NewPublisher(config.Watermill)
	if err != nil {
		log.Fatalf("publisher: %v", err)
	}
	defer publisher.Close()

	mux := http.NewServeMux()

	if config.Providers.GitHub.Enabled {
		ghHandler, err := webhook.NewGitHubHandler(
			config.Providers.GitHub.Secret,
			ruleEngine,
			publisher,
		)
		if err != nil {
			log.Fatalf("github handler: %v", err)
		}
		mux.Handle(config.Providers.GitHub.Path, ghHandler)
		log.Printf("github webhook enabled on %s", config.Providers.GitHub.Path)
	}

	addr := ":" + strconv.Itoa(config.Server.Port)
	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
