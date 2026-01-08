package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"

	"githooks/pkg/core"
	"githooks/pkg/server"
	"githooks/sdk/go/worker"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to app config")
	flag.Parse()

	log.SetPrefix("githooks/inprocess ")
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	cfg, err := core.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	cfg.Watermill.Driver = "shared_gochannel"
	cfg.Watermill.Drivers = nil
	if cfg.Watermill.GoChannel.OutputChannelBuffer == 0 {
		cfg.Watermill.GoChannel.OutputChannelBuffer = 64
	}

	wmLogger := watermill.NewStdLogger(false, false)
	bus := gochannel.NewGoChannel(gochannel.Config{
		OutputChannelBuffer:            cfg.Watermill.GoChannel.OutputChannelBuffer,
		Persistent:                     cfg.Watermill.GoChannel.Persistent,
		BlockPublishUntilSubscriberAck: cfg.Watermill.GoChannel.BlockPublishUntilSubscriberAck,
	}, wmLogger)

	core.RegisterPublisherDriver("shared_gochannel", func(_ core.WatermillConfig, _ watermill.LoggerAdapter) (message.Publisher, func() error, error) {
		return bus, bus.Close, nil
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	serverLogger := core.NewLogger("server")
	go func() {
		if err := server.Run(ctx, cfg, serverLogger); err != nil {
			log.Printf("server stopped: %v", err)
			cancel()
		}
	}()

	previewWorker := worker.New(
		worker.WithSubscriber(bus),
		worker.WithTopics("pr.opened.ready"),
		worker.WithLogger(core.NewLogger("worker-preview")),
	)
	previewWorker.HandleTopic("pr.opened.ready", func(ctx context.Context, evt *worker.Event) error {
		log.Printf("intent: preview worker received topic=%s provider=%s type=%s", evt.Topic, evt.Provider, evt.Type)
		return nil
	})

	mergeWorker := worker.New(
		worker.WithSubscriber(bus),
		worker.WithTopics("pr.merged"),
		worker.WithLogger(core.NewLogger("worker-merge")),
	)
	mergeWorker.HandleTopic("pr.merged", func(ctx context.Context, evt *worker.Event) error {
		log.Printf("intent: merge worker received topic=%s provider=%s type=%s", evt.Topic, evt.Provider, evt.Type)
		return nil
	})

	go func() {
		if err := previewWorker.Run(ctx); err != nil {
			log.Printf("preview worker stopped: %v", err)
			cancel()
		}
	}()
	go func() {
		if err := mergeWorker.Run(ctx); err != nil {
			log.Printf("merge worker stopped: %v", err)
			cancel()
		}
	}()

	<-ctx.Done()
}
