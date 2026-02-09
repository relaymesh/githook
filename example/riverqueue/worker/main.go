package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

var jobKind = "my_job"

type WebhookArgs map[string]interface{}

func (WebhookArgs) Kind() string { return jobKind }

type WebhookWorker struct {
	river.WorkerDefaults[WebhookArgs]
}

func (w *WebhookWorker) Work(ctx context.Context, job *river.Job[WebhookArgs]) error {
	log.Printf("job=%d queue=%s kind=%s args=%v", job.ID, job.Queue, job.Kind, job.Args)
	return nil
}

func main() {
	dsn := flag.String("dsn", "postgres://githook:githook@localhost:5433/githook?sslmode=disable", "Postgres DSN")
	queue := flag.String("queue", "my_custom_queue", "River queue")
	kind := flag.String("kind", "my_job", "River job kind")
	maxWorkers := flag.Int("max-workers", 5, "Max workers for the queue")
	flag.Parse()

	log.SetPrefix("githook/riverqueue-worker ")
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	jobKind = *kind

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	dbPool, err := pgxpool.New(ctx, *dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer dbPool.Close()

	workers := river.NewWorkers()
	river.AddWorker(workers, &WebhookWorker{})

	client, err := river.NewClient(riverpgxv5.New(dbPool), &river.Config{
		Logger: slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})),
		Queues: map[string]river.QueueConfig{
			*queue: {MaxWorkers: *maxWorkers},
		},
		Workers: workers,
	})
	if err != nil {
		log.Fatalf("river client: %v", err)
	}

	if err := client.Start(ctx); err != nil {
		log.Fatalf("river start: %v", err)
	}

	<-ctx.Done()
	stopCtx, cancelStop := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelStop()
	if err := client.Stop(stopCtx); err != nil {
		log.Printf("river stop: %v", err)
	}
}
