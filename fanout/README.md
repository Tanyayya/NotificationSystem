# Fan-out workers (Go)

Fan out workers live under `/worker`. Each Kafka **message value** must be JSON (see [Kafka message format](#kafka-message-format)). After a message is parsed, the worker publishes to **Redis Pub/Sub** on channel `notif:{userID}` (Kafka message key, or `NOTIFY_DEFAULT_USER_ID` when the key is empty). For testing, there is an optional Kafka HTTP producer for load and manual publishes under `test/kafka-producer`. A small **Redis subscriber** under `test/redis-subscriber` pattern-subscribes to `notif:*` and logs all incoming messages.

### Kafka message format

The consumer expects the **value** (payload) to be JSON with:

| Field | Type | Meaning |
| ----- | ---- | ------- |
| `id` | number or decimal string | Snowflake id for the notification |
| `type` | string | Notification kind (e.g. `Like`, `Comment`, `Post`) |
| `detail` | string | Extra context for the recipient |
| `timestamp` | number | Unix time in **milliseconds** |

Invalid JSON or a missing/invalid `id` is logged and the message is still committed; no Redis publish occurs for that record.

### Redis notification shape

Published JSON on `notif:{userID}` looks like:

`{"id":…,"type":…,"from_user":…,"message":…,"timestamp":…}`

Here `id`, `type`, `message` (from Kafka `detail`), and `timestamp` come from the Kafka payload. `from_user` is set from the worker env `NOTIFY_FROM_USER` only.

## Regular Setup (3 workers, core stack)

Requires Docker and Docker Compose v2.

```bash
docker compose up --build
```

This starts **Redis** (`redis`), Kafka, and **3 workers**. Wait until Kafka is healthy (first boot can take about a minute). `kafka-init` creates the topic with **3 partitions** so each worker can consume a different partition in the same consumer group. Redis is exposed on **6379** on the host for debugging.

### Worker and Redis environment (Compose defaults)

| Variable | Purpose |
| -------- | ------- |
| `REDIS_ADDR` | Redis address (default in code: `localhost:6379`; Compose sets `redis:6379`) |
| `NOTIFY_DEFAULT_USER_ID` | User id for `notif:{userID}` when the Kafka message has no key |
| `NOTIFY_FROM_USER` | Redis JSON `from_user` (default `alice`) |
| `NOTIFY_TYPE` | Loaded by the worker but **not** used for Redis when consuming Kafka (Kafka supplies `type`) |
| `NOTIFY_MESSAGE` | Loaded by the worker but **not** used for Redis when consuming Kafka (Kafka `detail` becomes `message`) |

## Test Setup (include kafka-producer, redis-subscriber)

To also start the **kafka-producer** service on port **8081** and the **redis-subscriber** container (pattern-subscribes to `notif:*` and logs messages), enable the `test-fanout` profile:

```bash
docker compose --profile test-fanout up --build
```

The **redis-subscriber** service sets `REDIS_ADDR=redis:6379` only.

## Testing

The **kafka-producer** HTTP API at `http://localhost:8081` and the **redis-subscriber** subscriber are only started when you use **`--profile test-fanout`** (see above).

**Health**

```bash
curl -s http://localhost:8081/health
```

**Publish a message** (expect `202`, workers logging the parsed Kafka fields, then Redis `PUBLISH` to `notif:{userID}` with the [Redis notification shape](#redis-notification-shape). The **redis-subscriber** logs `channel=notif:…` and the payload.)

The HTTP body is JSON. **`type`** and **`detail`** are required. **`id`**, **`timestamp`** (Unix ms), and **`key`** are optional; if `id` or `timestamp` is omitted, the producer fills them. Use **`key`** as the recipient user id (e.g. `bob` → channel `notif:bob`).

```bash
curl -s -X POST http://localhost:8081/messages \
  -H 'Content-Type: application/json' \
  -d '{"type":"Like","detail":"Someone liked your photo","key":"bob"}'
```

With explicit id and timestamp:

```bash
curl -s -X POST http://localhost:8081/messages \
  -H 'Content-Type: application/json' \
  -d '{"id":1234567890123456789,"type":"Comment","detail":"New reply","timestamp":1743273600000,"key":"bob"}'
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
