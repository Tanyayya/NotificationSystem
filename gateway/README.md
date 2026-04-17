# WebSocket Gateway

The gateway accepts WebSocket connections on **`/ws`**, registers the client in Redis, and forwards **Redis Pub/Sub** messages on channel `notif:{userID}` to the connected client as WebSocket text frames.


## Test a connection with websocat

Install [websocat](https://github.com/vi/websocat), then open a connection (the `user_id` query parameter must match the Redis channel you publish to — case-sensitive):

```bash
websocat 'ws://localhost:8080/ws?user_id=alice'
```

On connect, the gateway replays any undelivered notifications from PostgreSQL (notifications the user missed while offline), then subscribes to `notif:{userID}` on Redis for real-time delivery.

## Health check

```bash
curl -sf http://localhost:8080/health
```
