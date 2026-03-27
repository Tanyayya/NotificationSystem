# Kafka fan-out workers (Go)

Consumer workers and an HTTP producer. Run the stack with Docker Compose from the **repository root**.

## Setup (3 workers)

Requires Docker and Docker Compose v2.

```bash
docker compose up --build --scale worker=3
```

Wait until Kafka is healthy (first boot can take about a minute). `kafka-init` creates the topic with **3 partitions** so each worker can consume a different partition in the same consumer group.

## Testing

Producer API: `http://localhost:8081`.

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
