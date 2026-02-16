package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	worker "githook/sdk/go/worker"
)

func main() {
	endpoint := flag.String("endpoint", os.Getenv("GITHOOK_ENDPOINT"), "Githook API endpoint (required)")
	apiKey := flag.String("api-key", os.Getenv("GITHOOK_API_KEY"), "Githook API key (required)")
	topic := flag.String("topic", "github.dev", "topic to subscribe")
	driverID := flag.String("driver-id", "default:amqp", "Driver ID to use for the topic (required)")
	tenantID := flag.String("tenant-id", os.Getenv("GITHOOK_TENANT_ID"), "Tenant ID for driver lookups (default \"default\")")
	flag.Parse()

	if *endpoint == "" {
		log.Fatal("endpoint is required")
	}
	if *apiKey == "" {
		log.Fatal("api-key is required")
	}
	if *driverID == "" {
		log.Fatal("driver-id is required (fetch it from the Drivers API)")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	wk := worker.New(
		worker.WithEndpoint(*endpoint),
		worker.WithAPIKey(*apiKey),
		worker.WithTenant(*tenantID),
		worker.WithDefaultDriver(*driverID),
	)

	wk.HandleTopic(*topic, *driverID, func(ctx context.Context, evt *worker.Event) error {
		if evt == nil {
			return nil
		}
		log.Printf("topic=%s provider=%s type=%s installation=%s", evt.Topic, evt.Provider, evt.Type, evt.Metadata["installation_id"])
		if len(evt.Payload) > 0 {
			log.Printf("payload bytes=%d", len(evt.Payload))
		}
		return nil
	})

	if err := wk.Run(ctx); err != nil {
		log.Fatalf("worker run failed: %v", err)
	}
}
