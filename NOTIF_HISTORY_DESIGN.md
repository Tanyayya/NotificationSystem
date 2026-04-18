# Notification History on Connect — Design Plan

## Overview

When a user first connects via WebSocket, they receive their recent notification history (last 50) along with an unread count. Subsequent pages can be requested by the client over the same WebSocket connection. Notifications are marked as delivered when sent.

---

## Architecture

```
Client WebSocket connect
  → Gateway (handler.go)
      → history.Service.GetHistory()
            → Query notifications table (fan-out-write records)
            → Query events table JOIN followers (fan-out-read records)
            → Write-through resolved events → notifications table
            → Compute unread_count
            → Mark batch as delivered
      → Send "history" envelope over WebSocket
  → Begin Redis Pub/Sub for real-time delivery

Client sends { "type": "fetch_history", "before_id": "..." }
  → Reader goroutine detects inbound message
  → Gateway calls history.Service.GetHistory(beforeID)
  → Send next "history" envelope, mark delivered
```

---

## New Package: `gateway/internal/history/`

```
gateway/
  internal/
    history/
      service.go   ← HistoryService struct, GetHistory()
      db.go        ← PostgreSQL queries (fetch, write-through, mark delivered)
```

### `service.go`

```go
type HistoryService struct { db *sql.DB }

type Result struct {
    UnreadCount   int
    Notifications []Notification
    HasMore       bool
}

func (s *HistoryService) GetHistory(ctx context.Context, recipientID string, beforeID int64, limit int) (Result, error)
// 1. Fetch last `limit` notifications from both sources (beforeID as pagination cursor; 0 = from latest)
// 2. Write-through any events-table records → notifications table (INSERT ON CONFLICT DO NOTHING)
// 3. Compute unread_count from the fetched batch before marking delivered
// 4. Mark all returned notifications as delivered
// 5. Return Result
```

---

## Query Logic (Merged Fan-out Modes)

A single SQL query merges both the `notifications` table (fan-out-on-write) and the `events` table (fan-out-on-read), followed by a write-through and mark-delivered step.

```sql
-- Step 1: Fetch merged history
SELECT id, recipient_id, from_user, type, message, delivered, created_at
FROM notifications
WHERE recipient_id = $1
  AND ($2 = 0 OR id < $2)        -- cursor: 0 means "from latest"

UNION ALL

SELECT e.id, $1 AS recipient_id, e.from_user, e.type, e.message,
       FALSE AS delivered, e.created_at
FROM events e
JOIN followers f ON f.following_id = e.from_user
WHERE f.follower_id = $1
  AND ($2 = 0 OR e.id < $2)
  AND NOT EXISTS (
    SELECT 1 FROM notifications n
    WHERE n.id = e.id AND n.recipient_id = $1
  )

ORDER BY created_at DESC
LIMIT $3;

-- Step 2: Write-through fan-out-on-read records not already in notifications
INSERT INTO notifications (id, recipient_id, from_user, type, message, delivered)
  SELECT id, recipient_id, from_user, type, message, FALSE
  FROM <above result set where source = events>
ON CONFLICT (id, recipient_id) DO NOTHING;

-- Step 3: Count unread before marking delivered
SELECT COUNT(*) FROM notifications
WHERE recipient_id = $1 AND delivered = FALSE;

-- Step 4: Mark batch as delivered
UPDATE notifications
SET delivered = TRUE
WHERE recipient_id = $1
  AND id = ANY($ids);
```

**Why write-through?** Once a fan-out-on-read event is resolved and shown to a user, writing it to `notifications` ensures future fetches see it with the correct `delivered = true` status and do not re-surface it as unread.

---

## Gateway Changes

### `handler.go` — on connect

After `registerUser()` and before `subscribeNotifications()`, fetch and send history:

```go
result, err := h.history.GetHistory(ctx, userID, 0, 50)
if err != nil {
    // log and continue — history failure should not block real-time delivery
}
// marshal result as "history" envelope and send over writeCh
```

### `handler.go` — reader goroutine

The reader goroutine currently discards all inbound client messages. It must be updated to handle pagination requests:

```go
// Parse inbound message
// If type == "fetch_history":
//   call h.history.GetHistory(ctx, userID, beforeID, 50)
//   send result over writeCh
```

### `main.go` — initialization

```go
db := connectPostgres(os.Getenv("DB_DSN"))
historySvc := history.NewService(db)
handler := NewHandler(redisClient, historySvc)
```

---

## WebSocket Message Shapes

### Server → Client: history envelope

Sent once on connect, and again for each pagination request.

```json
{
  "type": "history",
  "unread_count": 12,
  "has_more": true,
  "notifications": [
    {
      "id": 1234567890,
      "from_user": "alice",
      "event_type": "POST",
      "message": "Alice posted a photo",
      "delivered": false,
      "timestamp": 1743273600123
    },
    {
      "id": 1234567800,
      "from_user": "carol",
      "event_type": "LIKE",
      "message": "Carol liked your post",
      "delivered": true,
      "timestamp": 1743273500000
    }
  ]
}
```

### Server → Client: real-time notification

A `type` field is added to distinguish live notifications from history frames.

```json
{
  "type": "notification",
  "id": 1234567999,
  "from_user": "bob",
  "event_type": "COMMENT",
  "message": "Bob commented on your post",
  "timestamp": 1743273700000
}
```

### Client → Server: request next page

```json
{
  "type": "fetch_history",
  "before_id": 1234567800,
  "limit": 50
}
```

`before_id` is the `id` of the oldest notification in the last received batch, used as a cursor for the next page.

---

## Shared DB Code

`GetUndeliveredNotifications()` and `MarkDelivered()` in `fanout/internal/db/postgres.go` remain in place — the fanout worker continues to use them for real-time delivery. The history package maintains its own DB layer since its query pattern (merged union, write-through, cursor-paginated) is distinct from the fanout worker's needs.

---

## Summary of Changes

| File / Package | Change |
|---|---|
| `gateway/internal/history/service.go` | **New** — `HistoryService`, `GetHistory()` |
| `gateway/internal/history/db.go` | **New** — merged query, write-through, mark delivered |
| `gateway/handler.go` | Call `GetHistory()` on connect; handle `fetch_history` in reader goroutine |
| `gateway/main.go` | Initialize DB connection and `HistoryService`, inject into handler |
| `db/init.sql` | Verify `created_at` exists on `notifications` and `events` tables; add if missing |

---

## Open Questions / Future Considerations

- **Error handling on history failure:** If the DB is unavailable at connect time, the gateway should degrade gracefully — skip history and continue with real-time delivery only.
- **`has_more` calculation:** Fetch `limit + 1` rows and return only `limit`; if `limit + 1` rows exist, set `has_more = true`.
- **`DB_DSN` environment variable:** The gateway does not currently read this variable. It must be added to the gateway's deployment configuration (Docker Compose, ECS task definition, etc.).
