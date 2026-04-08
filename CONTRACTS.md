# Contracts: Kafka and Redis

This document is the agreed contract for **topic names**, **message shapes**, and **Redis channels** used by the notification fan-out path.

---

## Kafka

### Topic

| Name | Environment variable | Notes |
|------|----------------------|--------|
| `worker-events` | `KAFKA_TOPIC` | Default; overridden per deployment. Docker Compose creates this topic with **3 partitions** and replication factor **1** (see `docker-compose.yml` `kafka-init`). |

### Consumer group

| Name | Environment variable | Notes |
|------|----------------------|--------|
| `worker-skeleton` | `KAFKA_GROUP_ID` | Default consumer group for the worker process. |

### Message key

- **Preferred:** non-empty UTF-8 string identifying the **recipient user** (the worker routes Redis publishes using this value).
- **If omitted:** the worker uses `NOTIFY_DEFAULT_USER_ID` (default `default`). Producers should set the key whenever possible so routing is explicit.

### Message value (JSON)

Each record **value** must be JSON with the following fields. The **ingestion** service (see below) is the reference producer: it always emits `id` as a **quoted string** and sets the Kafka **key** from the HTTP `from_user` field.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | `string` or `number` | Yes | Notification id. **Ingestion:** string `"<unix_ms>-<seq>"` (e.g. `"1743273600000-1"`) from `ingestion/snowflake.go`. **Consumer:** also accepts a JSON number or a base-10 int64 string; hyphenated ids are mapped to `int64` for Redis (see `fanout/internal/consumer` `parseSnowflakeID`). |
| `type` | `string` | Yes | Notification kind. **Allowed values:** `POST`, `LIKE`, `COMMENT` only (case-sensitive). Ingestion rejects any other value. |
| `detail` | `string` | Yes | Free-text context for the recipient; forwarded to Redis as `message` (see below). May be empty when the producer omits it (ingestion sends `""` if the HTTP body has no `detail`). |
| `timestamp` | `number` | Yes | Unix time in **milliseconds** since epoch. **Ingestion** sets this to `time.Now().UnixMilli()` at publish time. |

Example (**ingestion** shape: string `id`, milliseconds from the server clock):

```json
{
  "id": "1743273600000-1",
  "type": "POST",
  "detail": "Alice posted a photo",
  "timestamp": 1743273600123
}
```

Example (numeric `id`, for other producers):

```json
{
  "id": 1234567890123456789,
  "type": "POST",
  "detail": "Alice posted a photo",
  "timestamp": 1743273600000
}
```

Example (decimal string `id`, accepted by the parser):

```json
{
  "id": "1234567890123456789",
  "type": "COMMENT",
  "detail": "Nice shot!",
  "timestamp": 1743273600000
}
```

Malformed JSON or an invalid `id` is logged; the message is still **committed** (no infinite retry on bad payload).

### HTTP ingestion (`ingestion` service)

The ingestion API accepts **`POST /event`** with JSON matching `ingestion/handler.go` `EventRequest`. It builds `KafkaEvent`, publishes to `KAFKA_TOPIC` (default `worker-events`), and uses the Kafka **message key** = `from_user`.

| Field | JSON | Required | Description |
|-------|------|----------|-------------|
| `Type` | `type` | Yes | Must be `POST`, `LIKE`, or `COMMENT`. Maps to Kafka `type`. |
| `FromUser` | `from_user` | Yes | Kafka **message key** (sender / actor who triggered the event; becomes Redis JSON `from_user` when non-empty). |
| `Details` | `detail` | No | Maps to Kafka `detail`; omitted or empty becomes `""`. |

Kafka fields **`id`** and **`timestamp`** are assigned by the service (`NewSnowflakeID()` and current time in ms); clients do not send them on this endpoint.

Listen port defaults to **`3000`** (`PORT`). **`GET /health`** returns 200 for readiness checks.

### HTTP producer (test harness)

The `kafka-producer` service accepts `POST /messages` with a body that maps to the same Kafka **value** shape, plus optional routing:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | `number` | No | Defaults to `time.Now().UnixNano()` if zero. |
| `type` | `string` | Yes | Must be `POST`, `LIKE`, or `COMMENT`. Maps to `type`. |
| `detail` | `string` | Yes | Maps to `detail`. |
| `timestamp` | `number` | No | Defaults to `time.Now().UnixMilli()` if zero. |
| `key` | `string` | No | If set, becomes the Kafka **message key** (recipient user id). |

---

## Redis (Pub/Sub)

### Channel naming

| Pattern | Example | Purpose |
|---------|---------|---------|
| `notif:{userID}` | `notif:user-42` | One Pub/Sub channel per **recipient**. Gateway clients pass the same string as the WebSocket query `user_id` (`gateway/handler.go`, `gateway/pubsub.go`). |

Subscribers in production typically `SUBSCRIBE` to a single user channel. The test harness `redis-subscriber` uses **`PSUBSCRIBE notif:*`** to observe all notification channels.

### Message payload (JSON)

Published as the **string body** of `PUBLISH`. Shape:

| Field | Type | Description |
|-------|------|-------------|
| `id` | `number` | From Kafka event `id`. |
| `type` | `string` | From Kafka event `type`. |
| `from_user` | `string` | Kafka **message key** when non-empty (`NotificationEvent.FromUser`). |
| `message` | `string` | From Kafka event `detail`. |
| `timestamp` | `number` | From Kafka event `timestamp` (Unix **milliseconds**). |

Example:

```json
{
  "id": 1234567890123456789,
  "type": "POST",
  "from_user": "alice",
  "message": "Alice posted a photo",
  "timestamp": 1743273600000
}
```

---

## Field mapping (Kafka → Redis)

| Kafka | Redis payload |
|-------|----------------|
| Value `id` (parsed per `parseSnowflakeID`) | `id` |
| Value `type` | `type` |
| Value `detail` | `message` |
| Value `timestamp` | `timestamp` |
| Record **key** (bytes as UTF-8), or `NOTIFY_FROM_USER` if key empty | `from_user` |

---

## Versioning

Any change to topic names, channel patterns, or required JSON fields should be updated here and treated as a **breaking** change for producers and subscribers unless backward compatibility is explicitly preserved.
