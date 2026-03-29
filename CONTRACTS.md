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

Each record **value** must be JSON with the following fields. The consumer unmarshals flexibly: `id` may be a JSON number **or** a decimal string in quotes.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | `number` or `string` | Yes | Notification id (snowflake-style); string form must be parseable as base-10 int64. |
| `type` | `string` | Yes | Notification kind (e.g. `Like`, `Comment`, `Post`, `new_post`). |
| `detail` | `string` | Yes | Free-text context for the recipient; forwarded to Redis as `message` (see below). |
| `timestamp` | `number` | Yes | Unix time in **milliseconds** since epoch. |

Example (numeric `id`):

```json
{
  "id": 1234567890123456789,
  "type": "Post",
  "detail": "Alice posted a photo",
  "timestamp": 1743273600000
}
```

Example (string `id`, accepted by the parser):

```json
{
  "id": "1234567890123456789",
  "type": "Comment",
  "detail": "Nice shot!",
  "timestamp": 1743273600000
}
```

Malformed JSON or an invalid `id` is logged; the message is still **committed** (no infinite retry on bad payload).

### HTTP producer (test harness)

The `kafka-producer` service accepts `POST /messages` with a body that maps to the same value shape, plus optional routing:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | `number` | No | Defaults to `time.Now().UnixNano()` if zero. |
| `type` | `string` | Yes | Maps to `type`. |
| `detail` | `string` | Yes | Maps to `detail`. |
| `timestamp` | `number` | No | Defaults to `time.Now().UnixMilli()` if zero. |
| `key` | `string` | No | If set, becomes the Kafka **message key** (recipient user id). |

---

## Redis (Pub/Sub)

### Channel naming

| Pattern | Example | Purpose |
|---------|---------|---------|
| `notif:{userID}` | `notif:user-42` | One channel per recipient; `userID` is the same string used as the Kafka message key (or the default user id when the key is empty). |

Subscribers in production typically `SUBSCRIBE` to a single user channel. The test harness `redis-subscriber` uses **`PSUBSCRIBE notif:*`** to observe all notification channels.

### Message payload (JSON)

Published as the **string body** of `PUBLISH`. Shape:

| Field | Type | Description |
|-------|------|-------------|
| `id` | `number` | From Kafka event `id`. |
| `type` | `string` | From Kafka event `type`. |
| `from_user` | `string` | From worker config `NOTIFY_FROM_USER` (not from Kafka). |
| `message` | `string` | From Kafka event `detail`. |
| `timestamp` | `number` | From Kafka event `timestamp` (Unix **milliseconds**). |

Example:

```json
{
  "id": 1234567890123456789,
  "type": "new_post",
  "from_user": "alice",
  "message": "Alice posted a photo",
  "timestamp": 1743273600000
}
```

---

## Field mapping (Kafka → Redis)

| Kafka value | Redis payload |
|-------------|----------------|
| `id` | `id` |
| `type` | `type` |
| `detail` | `message` |
| `timestamp` | `timestamp` |
| — | `from_user` (worker env `NOTIFY_FROM_USER`) |

---

## Versioning

Any change to topic names, channel patterns, or required JSON fields should be updated here and treated as a **breaking** change for producers and subscribers unless backward compatibility is explicitly preserved.
