package main

import (
	"log"
	"net/http"
)

func main() {
	// Initialize Redis connection before accepting any WebSocket connections.
	// This will fatal if Redis is unreachable — better to fail fast on startup
	// than to discover Redis is down mid-connection.
	initRedis()

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