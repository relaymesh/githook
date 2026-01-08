package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
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
		worker.WithTopics("pr.opened.ready", "pr.merged"),
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

	exampleCtx := context.WithValue(ctx, attemptKey{}, &attempts{})
	if err := wk.Run(exampleCtx); err != nil {
		log.Fatal(err)
	}
}
