package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

const (
	riverQueueDefaultQueue      = "default"
	riverQueueDefaultKind       = "githook.event"
	riverQueueDefaultMaxWorkers = 5
	riverQueueDefaultTable      = "river_job"
)

type riverQueueJobArgs struct {
	JobKind string          `json:"-"`
	Raw     json.RawMessage `json:"-"`
}

func (a riverQueueJobArgs) Kind() string { return a.JobKind }

func (a *riverQueueJobArgs) UnmarshalJSON(data []byte) error {
	if a == nil {
		return errors.New("riverqueue args is nil")
	}
	if len(data) == 0 {
		a.Raw = nil
		return nil
	}
	a.Raw = append(a.Raw[:0], data...)
	return nil
}

type riverQueueSubscriber struct {
	cfg    RiverQueueConfig
	logger watermill.LoggerAdapter
	buffer int
	schema string

	mu          sync.Mutex
	topics      map[string]chan *message.Message
	startOnce   sync.Once
	startErr    error
	startCancel context.CancelFunc
	client      *river.Client[pgx.Tx]
	pool        *pgxpool.Pool
	closeOnce   sync.Once
	closed      bool
}

func newRiverQueueSubscriber(cfg SubscriberConfig, logger watermill.LoggerAdapter) (message.Subscriber, error) {
	rcfg := cfg.RiverQueue
	rcfg.DSN = strings.TrimSpace(rcfg.DSN)
	if rcfg.DSN == "" {
		return nil, errors.New("riverqueue dsn is required")
	}
	if rcfg.Queue == "" {
		rcfg.Queue = riverQueueDefaultQueue
	}
	if rcfg.Kind == "" {
		rcfg.Kind = riverQueueDefaultKind
	}
	if rcfg.MaxWorkers <= 0 {
		rcfg.MaxWorkers = riverQueueDefaultMaxWorkers
	}

	schema, err := riverQueueSchemaFromTable(rcfg.Table)
	if err != nil {
		return nil, err
	}

	buffer := int(cfg.GoChannel.OutputChannelBuffer)
	if buffer <= 0 {
		buffer = 64
	}

	return &riverQueueSubscriber{
		cfg:    rcfg,
		logger: logger,
		buffer: buffer,
		schema: schema,
		topics: make(map[string]chan *message.Message),
	}, nil
}

func (s *riverQueueSubscriber) Subscribe(ctx context.Context, topic string) (<-chan *message.Message, error) {
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return nil, errors.New("topic is required")
	}

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, errors.New("subscriber is closed")
	}
	if ch, ok := s.topics[topic]; ok {
		s.mu.Unlock()
		return ch, nil
	}
	ch := make(chan *message.Message, s.buffer)
	s.topics[topic] = ch
	s.mu.Unlock()

	if err := s.ensureStarted(ctx); err != nil {
		s.mu.Lock()
		delete(s.topics, topic)
		close(ch)
		s.mu.Unlock()
		return nil, err
	}

	return ch, nil
}

func (s *riverQueueSubscriber) Close() error {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		client := s.client
		pool := s.pool
		topics := make([]chan *message.Message, 0, len(s.topics))
		for _, ch := range s.topics {
			topics = append(topics, ch)
		}
		cancel := s.startCancel
		s.mu.Unlock()

		if cancel != nil {
			cancel()
		}
		if client != nil {
			ctx, cancelStop := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancelStop()
			if err := client.Stop(ctx); err != nil {
				s.mu.Lock()
				if s.startErr == nil {
					s.startErr = err
				}
				s.mu.Unlock()
			}
		}
		if pool != nil {
			pool.Close()
		}
		for _, ch := range topics {
			close(ch)
		}
	})

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.startErr
}

func (s *riverQueueSubscriber) ensureStarted(ctx context.Context) error {
	s.startOnce.Do(func() {
		startCtx, cancel := context.WithCancel(ctx)
		s.startCancel = cancel

		pool, err := pgxpool.New(startCtx, s.cfg.DSN)
		if err != nil {
			s.startErr = err
			return
		}
		s.pool = pool

		workers := river.NewWorkers()
		river.AddWorkerArgs(workers, riverQueueJobArgs{JobKind: s.cfg.Kind}, &riverQueueWorker{subscriber: s})

		client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
			Queues: map[string]river.QueueConfig{
				s.cfg.Queue: {MaxWorkers: s.cfg.MaxWorkers},
			},
			Workers: workers,
			Schema:  s.schema,
		})
		if err != nil {
			s.startErr = err
			return
		}
		s.client = client

		go func() {
			if err := client.Start(startCtx); err != nil {
				s.mu.Lock()
				if s.startErr == nil {
					s.startErr = err
				}
				s.mu.Unlock()
				if s.logger != nil {
					s.logger.Error("riverqueue subscriber start failed", err, watermill.LogFields{
						"driver": "riverqueue",
					})
				}
			}
		}()
	})

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.startErr
}

func (s *riverQueueSubscriber) handleJob(ctx context.Context, job *river.Job[riverQueueJobArgs]) error {
	provider, eventName, topic, logID, err := riverQueueMetadata(job.Metadata)
	if err != nil {
		return err
	}

	payload := job.Args.Raw
	if len(payload) == 0 && len(job.EncodedArgs) > 0 {
		payload = job.EncodedArgs
	}

	msg := message.NewMessageWithContext(ctx, watermill.NewUUID(), message.Payload(payload))
	if provider != "" {
		msg.Metadata.Set("provider", provider)
	}
	if eventName != "" {
		msg.Metadata.Set("event", eventName)
	}
	if topic != "" {
		msg.Metadata.Set("topic", topic)
	}
	if logID != "" {
		msg.Metadata.Set("log_id", logID)
	}
	msg.Metadata.Set("driver", "riverqueue")
	msg.Metadata.Set("job_id", strconv.FormatInt(job.ID, 10))
	if job.Queue != "" {
		msg.Metadata.Set("queue", job.Queue)
	}
	if job.Kind != "" {
		msg.Metadata.Set("kind", job.Kind)
	}

	ch := s.topicChannel(topic)
	if ch == nil {
		return fmt.Errorf("no subscriber for topic %s", topic)
	}

	select {
	case ch <- msg:
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case <-msg.Acked():
		return nil
	case <-msg.Nacked():
		return fmt.Errorf("riverqueue message nacked: job %d", job.ID)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *riverQueueSubscriber) topicChannel(topic string) chan *message.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.topics[topic]
}

type riverQueueWorker struct {
	river.WorkerDefaults[riverQueueJobArgs]
	subscriber *riverQueueSubscriber
}

func (w *riverQueueWorker) Work(ctx context.Context, job *river.Job[riverQueueJobArgs]) error {
	if w.subscriber == nil {
		return errors.New("riverqueue subscriber is nil")
	}
	return w.subscriber.handleJob(ctx, job)
}

func riverQueueMetadata(raw []byte) (string, string, string, string, error) {
	if len(raw) == 0 {
		return "", "", "", "", errors.New("riverqueue metadata is missing")
	}
	var meta map[string]interface{}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return "", "", "", "", fmt.Errorf("riverqueue metadata decode failed: %w", err)
	}

	var provider, eventName, topic, logID string
	if val, ok := meta["provider"].(string); ok {
		provider = val
	}
	if val, ok := meta["name"].(string); ok {
		eventName = val
	}
	if eventName == "" {
		if val, ok := meta["event"].(string); ok {
			eventName = val
		}
	}
	if val, ok := meta["topic"].(string); ok {
		topic = val
	}
	if val, ok := meta["log_id"].(string); ok {
		logID = val
	}
	if topic == "" {
		return "", "", "", "", errors.New("riverqueue metadata missing topic")
	}
	return provider, eventName, topic, logID, nil
}

func riverQueueSchemaFromTable(table string) (string, error) {
	table = strings.TrimSpace(table)
	if table == "" {
		return "", nil
	}

	parts := strings.Split(table, ".")
	switch len(parts) {
	case 1:
		if strings.ToLower(parts[0]) != riverQueueDefaultTable {
			return "", fmt.Errorf("riverqueue table %q is not supported", table)
		}
		return "", nil
	case 2:
		if strings.ToLower(parts[1]) != riverQueueDefaultTable {
			return "", fmt.Errorf("riverqueue table %q is not supported", table)
		}
		if parts[0] == "" {
			return "", fmt.Errorf("riverqueue table %q is missing schema", table)
		}
		return parts[0], nil
	default:
		return "", fmt.Errorf("riverqueue table %q is not supported", table)
	}
}
