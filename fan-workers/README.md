# Fan-out workers (Go)

Consumer workers live under `src/worker`. After each Kafka message, the worker publishes a JSON notification to **Redis Pub/Sub** on channel `notif:{userID}` (Kafka message key, or `NOTIFY_DEFAULT_USER_ID` when the key is empty). The optional Kafka HTTP producer for load and manual publishes lives under `test/kafka-producer`. A small **Redis subscriber** under `test/redis-subscriber` pattern-subscribes to `notif:*` and logs incoming messages (same Go module and `internal/` packages).

Run Docker Compose from the [`fan-workers`](.) directory (or pass `-f fan-workers/docker-compose.yml` from the repo root).

## Setup (3 workers, core stack)

Requires Docker and Docker Compose v2.

```bash
cd fan-workers
docker compose up --build --scale worker=3
```

This starts **Redis** (`redis`), Kafka, and workers. Wait until Kafka is healthy (first boot can take about a minute). `kafka-init` creates the topic with **3 partitions** so each worker can consume a different partition in the same consumer group. Redis is exposed on **6379** on the host for debugging.

### Worker and Redis environment (Compose defaults)

| Variable | Purpose |
| -------- | ------- |
| `REDIS_ADDR` | Redis address (default in code: `localhost:6379`; Compose sets `redis:6379`) |
| `NOTIFY_DEFAULT_USER_ID` | User id for `notif:{userID}` when the Kafka message has no key |
| `NOTIFY_TYPE` | JSON `type` field (default `new_post`) |
| `NOTIFY_FROM_USER` | JSON `from_user` field |
| `NOTIFY_MESSAGE` | JSON `message` field |

## Setup with test helpers (kafka-producer, redis-subscriber)

To also start the **kafka-producer** service on port **8081** and the **redis-subscriber** container (pattern-subscribes to `notif:*` and logs messages), enable the `test` profile:

```bash
cd fan-workers
docker compose --profile test up --build --scale worker=3
```

The **redis-subscriber** service sets `REDIS_ADDR=redis:6379` only.

## Testing

The **kafka-producer** HTTP API at `http://localhost:8081` and the **redis-subscriber** subscriber are only started when you use **`--profile test`** (see above).

**Health**

```bash
curl -s http://localhost:8081/health
```

**Publish a message** (expect `202`, workers logging the Kafka payload, then Redis `PUBLISH` to `notif:{userID}` with body `{"type":"new_post","from_user":"…","message":"…"}`. The **redis-subscriber** logs `channel=notif:…` and the payload.)

Use `key` as the recipient user id (e.g. `bob` → channel `notif:bob`):

```bash
curl -s -X POST http://localhost:8081/messages \
  -H 'Content-Type: application/json' \
  -d '{"message":"hello from curl","key":"bob"}'
```

Without `key`, the worker uses `NOTIFY_DEFAULT_USER_ID` (Compose default `default` → `notif:default`).

**Load ramp** (~30 seconds; linear rate increase to `peak_mps`)

```bash
curl -s -X POST "http://localhost:8081/load/ramp?peak_mps=30"
```

**Stop**

```bash
docker compose down
```
