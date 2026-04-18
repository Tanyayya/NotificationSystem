# History Query: Mode-Aware UNION ALL

## Background

The history query in `gateway/internal/history/db.go` (`fetchRows`) always runs a `UNION ALL` merging the `notifications` table (fan-out-on-write) and the `events` table joined against `followers` (fan-out-on-read), regardless of the active `NOTIFICATION_MODE`.

The fanout worker writes exclusively to one table depending on mode:
- `FAN_OUT_WRITE` → only `notifications`
- `FAN_OUT_READ` → only `events`
- `FAN_OUT_HYBRID` → `notifications` for users below `FANOUT_THRESHOLD`, `events` for users above it

## Problem

In `FAN_OUT_WRITE` mode the `events` table is always empty, but the query still plans and executes the `JOIN followers` and a correlated `NOT EXISTS` subquery. In `FAN_OUT_READ` mode the inverse applies.

PostgreSQL short-circuits empty table scans quickly, so the performance impact is modest today. The more meaningful costs are:

- The `NOT EXISTS` correlated subquery is compiled into the plan regardless of whether `events` has data
- A single-table scan against `idx_notifications_recipient` is a simpler, cheaper plan than a `UNION ALL` with a join
- If the unused table accumulates rows during a migration or mixed deployment, the UNION silently starts doing real work

## Proposed Change

Pass `NOTIFICATION_MODE` into `history.Service` and select the query branch at runtime:

| Mode | Query |
|------|-------|
| `FAN_OUT_WRITE` | `SELECT` from `notifications` only |
| `FAN_OUT_READ` | `SELECT` from `events JOIN followers` only |
| `FAN_OUT_HYBRID` | Current `UNION ALL` (both branches needed) |

### Files to change

| File | Change |
|------|--------|
| `gateway/internal/history/service.go` | Add `mode` field to `Service`; update `NewService` to accept it |
| `gateway/internal/history/db.go` | Split `fetchRows` into three query paths based on mode, or add a `mode` parameter and build the query conditionally |
| `gateway/main.go` | Read `NOTIFICATION_MODE` env var and pass it to `history.NewService` |

## Expected Outcome

Structural improvement and correctness signal rather than a dramatic latency win. Primary benefit is a simpler query plan for pure `FAN_OUT_WRITE` and `FAN_OUT_READ` deployments, and protection against silent performance regression if the unused table grows during a migration.
