package main

import (
	"log"
	"net/http"

	"github.com/Tanyayya/NotificationSystem/gateway/internal/history"
)

// historySvc is nil when PostgreSQL is unavailable — history is disabled but
// real-time WebSocket delivery continues to work normally.
var historySvc *history.Service

func main() {
	// Initialize Redis connection before accepting any WebSocket connections.
	// This will fatal if Redis is unreachable — better to fail fast on startup
	// than to discover Redis is down mid-connection.
	initRedis()

	// Initialize PostgreSQL for notification history.
	// A failure here is non-fatal: the gateway degrades gracefully by skipping
	// history on connect rather than refusing all connections.
	db, err := history.OpenDB()
	if err != nil {
		log.Printf("warning: PostgreSQL unavailable, notification history disabled: %v", err)
	} else {
		historySvc = history.NewService(db)
		log.Println("connected to PostgreSQL for notification history")
	}

	// /ws is the WebSocket endpoint — clients connect here to receive real-time notifications
	// HandleWS is defined in handler.go
	http.HandleFunc("/ws", HandleWS)

	// /health is a simple health check endpoint
	// ECS will ping this to know the task is alive — must return 200
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	log.Println("Gateway listening on :8080")

	// ListenAndServe blocks forever — if it returns, something went wrong
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
