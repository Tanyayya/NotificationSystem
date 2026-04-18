# Distributed Real-Time Notification System

**Team:** Srimansi Ramesh Kumar, Tanya Shukla, Mackenzie Veazey

When a user triggers an event (like, follow, post, etc.), every relevant subscriber gets a push notification in real time over a **persistent WebSocket** — no polling, no intentional delay.

---

## Local Setup

Requires **Docker** and **Docker Compose v2**. From the repo root:

```bash
docker compose up --build
```

This brings up Redis, Kafka, and the fan-out workers (first Kafka boot can take about a minute). To exercise the fan-out pipeline in detail — optional test services, HTTP producer, and Redis subscriber — see **[fanout/README.md](fanout/README.md)**.

---

## AWS Deployment

All infrastructure is defined in Terraform under `infra/`. Each teammate can deploy an independent stack in their own AWS account.

### Prerequisites

- Terraform >= 1.3.0 (`terraform -version`)
- AWS CLI configured
- Docker Desktop running
- `wscat` for testing (`npm install -g wscat`)

### Account Notes

- **Sandbox account:** Full access including MSK and IAM role creation. Use this for development and experiments.
- **Learner Lab account:** MSK and custom IAM roles are blocked. Use Docker Compose locally instead.

### Step 1 — Configure AWS Credentials

**Sandbox:**

```bash
aws configure
# region: us-east-1
```

**Learner Lab** — paste credentials from "AWS Details" into `~/.aws/credentials`:

```
[default]
aws_access_key_id = ASIA...
aws_secret_access_key = ...
aws_session_token = ...
```

### Step 2 — Deploy Infrastructure

```bash
cd infra
terraform init
terraform plan
terraform apply   # MSK takes 15-25 min — let it run
```

Note the outputs after apply — you will need them:

- `alb_dns` — WebSocket endpoint for clients
- `ecr_gateway_url`, `ecr_ingestion_url`, `ecr_fanout_url` — ECR image URLs
- `kafka_bootstrap_brokers`, `redis_endpoint` — injected automatically into ECS tasks

### Step 3 — Build and Push Docker Images

Run from the **repo root**. Replace `ACCOUNT_ID` with your AWS account ID from the Terraform outputs.

```bash
# authenticate Docker to ECR
aws ecr get-login-password --region us-east-1 \
  | docker login --username AWS --password-stdin ACCOUNT_ID.dkr.ecr.us-east-1.amazonaws.com

# gateway
docker buildx build --platform linux/amd64 --load -t notif-system-gateway -f gateway/Dockerfile .
docker tag notif-system-gateway:latest ACCOUNT_ID.dkr.ecr.us-east-1.amazonaws.com/notif-system-gateway:latest
docker push ACCOUNT_ID.dkr.ecr.us-east-1.amazonaws.com/notif-system-gateway:latest

# ingestion
docker buildx build --platform linux/amd64 --load -t notif-system-ingestion -f ingestion/Dockerfile .
docker tag notif-system-ingestion:latest ACCOUNT_ID.dkr.ecr.us-east-1.amazonaws.com/notif-system-ingestion:latest
docker push ACCOUNT_ID.dkr.ecr.us-east-1.amazonaws.com/notif-system-ingestion:latest

# fanout worker — note --target worker
docker buildx build --platform linux/amd64 --load --target worker -t notif-system-fanout -f fanout/Dockerfile .
docker tag notif-system-fanout:latest ACCOUNT_ID.dkr.ecr.us-east-1.amazonaws.com/notif-system-fanout:latest
docker push ACCOUNT_ID.dkr.ecr.us-east-1.amazonaws.com/notif-system-fanout:latest
```

> **Mac Apple Silicon note:** The `--platform linux/amd64` flag is required. Without it Docker builds an ARM image that ECS Fargate cannot run.

### Step 4 — Deploy Services

```bash
aws ecs update-service --cluster notif-system-cluster --service notif-system-gateway --force-new-deployment --region us-east-1
aws ecs update-service --cluster notif-system-cluster --service notif-system-ingestion --force-new-deployment --region us-east-1
aws ecs update-service --cluster notif-system-cluster --service notif-system-fanout --force-new-deployment --region us-east-1
```

Wait about a minute then verify all three tasks are running:

```bash
aws ecs list-tasks --cluster notif-system-cluster --region us-east-1
```

### Step 5 — Test End-to-End

Find the ingestion task public IP from the ECS Console (Tasks tab → click ingestion task → Network → Public IP).

```bash
# Terminal 1 — connect a WebSocket client
wscat -c "ws://YOUR_ALB_DNS:8080/ws?user_id=alice"

# Terminal 2 — fire a test event immediately (within 60 seconds)
curl -X POST http://INGESTION_TASK_IP:3000/event \
  -H "Content-Type: application/json" \
  -d '{"type":"POST","from_user":"alice","detail":"Alice posted a photo"}'
```

Valid `type` values: `POST`, `LIKE`, `COMMENT` (case-sensitive).

Expected result — on connect, a history envelope appears first, then real-time notifications arrive as events are posted:

```json
{ "type": "history", "unread_count": 0, "has_more": false, "notifications": [] }
{ "type": "notification", "id": ..., "from_user": "alice", "event_type": "POST", "message": "Alice posted a photo", "timestamp": ... }
```

### Step 6 — Tear Down

Always destroy after testing to avoid unnecessary AWS charges:

```bash
cd infra
terraform destroy   # MSK takes a few minutes to delete
```

---

## Architecture

| Layer                        | Role                                                                                                                                                |
| ---------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Event Ingestion API** (Go) | `POST /event`; Snowflake IDs for ordering; events published to **Kafka** partitioned by sender id for per-user ordering.                            |
| **Fan-out Workers** (Go)     | Consume Kafka; fan-out strategy set by `NOTIFICATION_MODE` (`FAN_OUT_WRITE`, `FAN_OUT_READ`, or `FAN_OUT_HYBRID`). Hybrid mode switches between write and read paths based on `FANOUT_THRESHOLD`. |
| **Redis**                    | Pub/Sub for cross-instance delivery (`notif:{userID}`); session keys for user→gateway mapping (`ws:user:{userID}`). |
| **WebSocket Gateway** (Go)   | One goroutine per connection on ECS; user→gateway mapping in Redis; sends notification history from **PostgreSQL** on connect with cursor-based pagination; real-time delivery via Redis Pub/Sub. |

---

## Contracts

Topic names, Kafka message shapes, consumer configuration, and Redis channels for the fan-out path are specified in **[CONTRACTS.md](CONTRACTS.md)**.

---

## Seed Data

The database is preloaded with follower relationships for four test users, designed to exercise different fan-out strategies:

| User | Followers | IDs | Fan-out path (default threshold 1000) |
|------|-----------|-----|---------------------------------------|
| `alice` | 10 | `follower_a_1` … `follower_a_10` | Write |
| `bob` | 100 | `follower_b_1` … `follower_b_100` | Write |
| `carol` | 1,000 | `follower_c_1` … `follower_c_1000` | Write (at threshold) |
| `dave` | 10,000 | `follower_d_1` … `follower_d_10000` | Read |

To test notification history, connect as one of the follower IDs (e.g. `follower_a_1`) and post an event from the corresponding user (`alice`).

---

## Experiments

1. **Fan-out on write vs. read** — Benchmark latency and publisher throughput at 10 / 100 / 1K / 10K subscribers; find the crossover vs. Amdahl-style sequential fan-out limits.
2. **WebSockets per ECS task** — Scale concurrent connections (100 → 5K) with Locust; watch goroutines, heap, CloudWatch, and p99 latency.
3. **Horizontal scale vs. Redis** — Fixed total connections; scale ECS tasks 1 → 2 → 4; measure when **Redis Pub/Sub** becomes the shared bottleneck.
4. **Reconnection storm** _(stretch)_ — Mass disconnect/reconnect; recovery time, loss rate, reordering.
