# Distributed Real-Time Notification System

**Team:** Srimansi Ramesh Kumar, Tanya Shukla, Mackenzie Veazey

When a user triggers an event (like, follow, post, etc.), every relevant subscriber gets a push notification in real time over a **persistent WebSocket**—no polling, no intentional delay.

## Setup

Requires **Docker** and **Docker Compose v2**. From the repo root:

```bash
docker compose up --build
```

This brings up Redis, Kafka, and the fan-out workers (first Kafka boot can take about a minute). To exercise the fan-out pipeline in detail—optional test services, HTTP producer, and Redis subscriber—see **[fanout/README.md](fanout/README.md)**.

## Architecture

| Layer | Role |
|--------|------|
| **Event Ingestion API** (Go) | `POST /event`; Snowflake IDs for ordering; events published to **Kafka** partitioned by sender id for per-user ordering. |
| **Fan-out Workers** (Go) | Consume Kafka; **hybrid fan-out**—write path for typical users, read path for high-follower accounts; crossover threshold tuned empirically. |
| **Redis** | Pub/Sub for cross-instance delivery; sorted sets for paginated history (`ZADD` / `ZREVRANGE`); atomic counters for unread badges (`INCR` / `DECR`). |
| **WebSocket Gateway** (Go) | One goroutine per connection on ECS; user→gateway mapping in Redis; offline users get backlog replay from **PostgreSQL** on reconnect. |

## Experiments

1. **Fan-out on write vs. read** — Benchmark latency and publisher throughput at 10 / 100 / 1K / 10K subscribers; find the crossover vs. Amdahl-style sequential fan-out limits.
2. **WebSockets per ECS task** — Scale concurrent connections (100 → 5K) with Locust; watch goroutines, heap, CloudWatch, and p99 latency.
3. **Horizontal scale vs. Redis** — Fixed total connections; scale ECS tasks 1 → 2 → 4; measure when **Redis Pub/Sub** becomes the shared bottleneck.
4. **Reconnection storm** *(stretch)* — Mass disconnect/reconnect; recovery time, loss rate, reordering.