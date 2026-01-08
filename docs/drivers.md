# Driver Configuration

Githooks uses [Watermill](https://watermill.io/) for publishing and subscribing. This document describes supported driver settings and common patterns.

## Publisher vs Worker

- **Publisher (server)**: Use `driver` or `drivers` for fan-out.
- **Subscriber (worker)**: Use `driver` or `drivers` for fan-in.

### Publisher (Fan-Out)

To publish events to multiple backends simultaneously, use the `drivers` list.

```yaml
# Githooks Server Configuration
watermill:
  drivers: [amqp, http]
  amqp:
    url: amqp://guest:guest@localhost:5672/
    mode: durable_queue
  http:
    mode: base_url
    base_url: http://some-other-service/hooks
```

### Worker (Fan-In)

A worker can consume from multiple drivers, creating a unified stream of messages. Unsupported drivers for subscribers (like `http`) are automatically ignored.

```yaml
# Worker Configuration
watermill:
  drivers: [amqp, nats, kafka]
  # ... individual driver configs
```

---

## Driver Reference

Below are configuration examples for each supported driver.

### GoChannel

In-memory pub/sub, ideal for local development and testing. **This is the default driver if none is specified.**

-   **`persistent`**: If `true`, the channel is not closed when all subscribers are gone.
-   **`output_buffer`**: The size of the output channel buffer.

```yaml
watermill:
  driver: gochannel
  gochannel:
    output_buffer: 64
    persistent: false
    block_publish_until_subscriber_ack: false
```

### Kafka

Forwards events to a Kafka topic.

-   **`brokers`**: A list of Kafka broker addresses.
-   **`consumer_group`**: (Worker-only) The consumer group for the worker.

```yaml
watermill:
  driver: kafka
  kafka:
    brokers: ["kafka-broker-1:9092", "kafka-broker-2:9092"]
    consumer_group: "my-githooks-worker-group" # for workers
```

### NATS Streaming

Forwards events to a NATS Streaming (STAN) channel.

-   **`cluster_id`**: The NATS Streaming cluster ID.
-   **`client_id`**: A unique client ID for the connection.
-   **`client_id_suffix`**: (Worker-only) An optional suffix added to the `client_id` to ensure uniqueness across worker replicas.
-   **`url`**: The address of the NATS server.

```yaml
watermill:
  driver: nats
  nats:
    cluster_id: test-cluster
    client_id: githooks-publisher
    client_id_suffix: "-worker-1" # for workers
    url: nats://localhost:4222
```

### AMQP (RabbitMQ)

Forwards events to an AMQP exchange/queue.

-   **`url`**: The AMQP connection string.
-   **`mode`**: Can be `durable_queue`, `nondurable_queue`, `durable_pubsub`, or `nondurable_pubsub`.

```yaml
watermill:
  driver: amqp
  amqp:
    url: amqp://guest:guest@rabbitmq:5672/
    mode: durable_queue
```

### SQL (Postgres/MySQL)

Uses a database table as a message queue.

-   **`driver`**: The Go database driver name (`postgres` or `mysql`).
-   **`dsn`**: The database connection string.
-   **`dialect`**: The SQL dialect (`postgres` or `mysql`).
-   **`auto_initialize_schema`**: If `true`, Githooks will create the necessary table if it doesn't exist.

**Note**: Your application must blank-import the required database driver (e.g., `_ "github.com/lib/pq"`).

```yaml
watermill:
  driver: sql
  sql:
    driver: postgres
    dsn: postgres://user:pass@localhost:5432/dbname?sslmode=disable
    dialect: postgres
    auto_initialize_schema: true
```

### RiverQueue (Postgres)

Publishes events as jobs to a [River](https://github.com/riverqueue/river) job queue. This is a **publish-only** driver.

-   **`driver`**: The Go database driver for River (`postgres`).
-   **`dsn`**: The database connection string.
-   **`table`**: The River jobs table name (default: `river_job`).
-   **`queue`**: The queue to insert jobs into (default: `default`).
-   **`kind`**: The `kind` of job to insert. This should match a registered River worker.
-   **`tags`**: (Optional) Tags to add to the job.

```yaml
watermill:
  driver: riverqueue
  riverqueue:
    driver: postgres
    dsn: postgres://user:pass@localhost:5432/dbname?sslmode=disable
    table: river_job
    queue: high_priority
    kind: githooks.event
    max_attempts: 25
    priority: 1
    tags: ["githooks", "webhook"]
```

### HTTP (Publish-only)

Publishes events via an HTTP POST request. This is a **publish-only** driver and cannot be used by workers.

-   **`mode`**:
    -   `topic_url`: The topic name is treated as the full URL to POST to.
    -   `base_url`: The topic name is appended to the `base_url`.
-   **`base_url`**: The base URL for the webhook endpoint.

```yaml
watermill:
  driver: http
  http:
    mode: base_url
    base_url: http://another-service:8080/webhooks
```

## Publish Failure Handling

Configure retry and optional DLQ routing:

```yaml
watermill:
  publish_retry:
    attempts: 3
    delay_ms: 500
  dlq_driver: amqp
```
