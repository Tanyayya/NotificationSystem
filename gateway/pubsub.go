package main

import (
	"context"
	"log"
)

// subscribeNotifications subscribes to the Redis Pub/Sub channel for a specific user
// and forwards any incoming messages to their WebSocket writeCh.
//
// Channel naming convention: notif:{userID}
// e.g. for user123 → subscribes to "notif:user123"
//
// This runs as its own goroutine per connection (spawned in handler.go).
// It exits cleanly when ctx is cancelled (i.e. the user disconnects).
//
// writeCh is the same channel the writer goroutine drains —
// we never call ws.WriteMessage directly from here, always go through writeCh
// to respect the single-writer rule.
func subscribeNotifications(ctx context.Context, userID string, writeCh chan<- []byte) {
	channel := "notif:" + userID

	// Subscribe to the user's personal notification channel in Redis Pub/Sub.
	// Any fan-out worker that publishes to "notif:user123" will be received here.
	sub := rdb.Subscribe(ctx, channel)
	defer sub.Close()

	log.Printf("subscribed to Redis channel %s", channel)

	// sub.Channel() returns a Go channel that delivers incoming Pub/Sub messages
	ch := sub.Channel()

	for {
		select {
		case <-ctx.Done():
			// User disconnected — ctx was cancelled, exit cleanly
			log.Printf("unsubscribed from channel %s", channel)
			return

		case msg, ok := <-ch:
			if !ok {
				// Redis channel was closed — exit
				log.Printf("Redis channel closed for user %s", userID)
				return
			}

			// Forward the message payload to the writer goroutine via writeCh.
			// Using a non-blocking select so a slow/stuck client doesn't block
			// the subscriber — we just drop the message and log it instead.
			select {
			case writeCh <- []byte(msg.Payload):
			default:
				log.Printf("writeCh full for user %s, dropping message", userID)
			}
		}
	}
}