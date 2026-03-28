# Kafka fan-out workers (Go)

Consumer workers live under `src/worker`. The optional Kafka HTTP producer for load and manual publishes lives under `test/producer` (same Go module and `internal/` packages).

Run Docker Compose from the [`fan-workers`](.) directory (or pass `-f fan-workers/docker-compose.yml` from the repo root).

## Setup (3 workers, no producer)

Requires Docker and Docker Compose v2.

```bash
cd fan-workers
docker compose up --build --scale worker=3
```

Wait until Kafka is healthy (first boot can take about a minute). `kafka-init` creates the topic with **3 partitions** so each worker can consume a different partition in the same consumer group.

## Setup with the Kafka producer (test profile)

To also start the HTTP producer on port **8081**, enable the `test` profile:

```bash
cd fan-workers
docker compose --profile test up --build --scale worker=3
```

## Testing

The producer API at `http://localhost:8081` is only available when you started the stack with **`--profile test`** (see above).

**Health**

```bash
curl -s http://localhost:8081/health
```

**Publish a message** (expect `202` and workers logging the payload)

```bash
curl -s -X POST http://localhost:8081/messages \
  -H 'Content-Type: application/json' \
  -d '{"message":"hello from curl","key":"optional-partition-key"}'
```

**Load ramp** (~30 seconds; linear rate increase to `peak_mps`)

```bash
curl -s -X POST "http://localhost:8081/load/ramp?peak_mps=30"
```

**Stop**

```bash
docker compose down
```
