package main

import (
	"context"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// upgrader upgrades a regular HTTP connection to a WebSocket connection.
// CheckOrigin is set to allow all origins for now — tighten this in production.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// conn represents a single connected WebSocket client.
// It holds everything needed to manage that client's lifecycle:
//   - userID:  who this connection belongs to
//   - ws:      the actual WebSocket connection
//   - writeCh: a channel for safely sending messages (gorilla/websocket
//              does NOT allow concurrent writes — all writes must go
//              through this channel to a single dedicated writer goroutine)
//   - cancel:  cancels the context to shut down all goroutines for this connection
type conn struct {
	userID  string
	ws      *websocket.Conn
	writeCh chan []byte
	cancel  context.CancelFunc
}

// HandleWS is the entry point for all WebSocket connections.
// It is called by main.go whenever a client hits the /ws endpoint.
//
// Flow:
//  1. Read user_id from the query param (?user_id=123)
//  2. Upgrade the HTTP connection to WebSocket
//  3. Spawn 2 goroutines: one reader, one writer
//  4. Block until both goroutines exit (i.e. client disconnects)
//  5. Clean up (Redis deregistration added in Task 2)
func HandleWS(w http.ResponseWriter, r *http.Request) {
	// Extract user_id from query params — e.g. ws://localhost:8080/ws?user_id=123
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "missing user_id", http.StatusBadRequest)
		return
	}

	// Upgrade the HTTP connection to a persistent WebSocket connection
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade error for user %s: %v", userID, err)
		return
	}

	// ctx is tied to this specific connection's lifetime.
	// When cancel() is called (by reader or writer on error/disconnect),
	// it signals all goroutines for this connection to shut down.
	ctx, cancel := context.WithCancel(context.Background())

	c := &conn{
		userID:  userID,
		ws:      ws,
		writeCh: make(chan []byte, 64), // buffered so senders don't block on slow clients
		cancel:  cancel,
	}

	log.Printf("user %s connected", userID)

	// WaitGroup ensures HandleWS blocks until both goroutines have fully exited
	// before we run cleanup (deregister from Redis in Task 2)
	var wg sync.WaitGroup
	wg.Add(2)

	// Writer goroutine — the ONLY goroutine allowed to call ws.WriteMessage.
	// It drains writeCh and sends each message to the client.
	go func() {
		defer wg.Done()
		c.writer(ctx)
	}()

	// Reader goroutine — listens for incoming messages from the client.
	// Its main job is detecting disconnects and triggering cancel()
	// so the writer and any future goroutines (Task 3) also shut down.
	go func() {
		defer wg.Done()
		c.reader(ctx)
	}()

	// Block here until both goroutines exit
	wg.Wait()
	log.Printf("user %s disconnected, cleaning up", userID)
	// TODO Task 2: deregister user from Redis here
}

// writer drains writeCh and sends each message to the WebSocket client.
// This is the single writer goroutine — no other goroutine should ever
// call ws.WriteMessage directly.
// Exits when ctx is cancelled or writeCh is closed.
func (c *conn) writer(ctx context.Context) {
	defer c.ws.Close()
	for {
		select {
		case <-ctx.Done():
			// Connection is shutting down — exit cleanly
			return
		case msg, ok := <-c.writeCh:
			if !ok {
				// writeCh was closed — exit
				return
			}
			if err := c.ws.WriteMessage(websocket.TextMessage, msg); err != nil {
				log.Printf("write error for user %s: %v", c.userID, err)
				c.cancel() // signal reader and other goroutines to shut down too
				return
			}
		}
	}
}

// reader blocks waiting for messages from the client.
// In Week 1 clients don't send us anything meaningful — we just drain reads.
// Its real job is detecting when the client disconnects and calling cancel()
// to tear down the whole connection cleanly.
func (c *conn) reader(ctx context.Context) {
	defer c.cancel() // always trigger shutdown when reader exits
	for {
		_, _, err := c.ws.ReadMessage()
		if err != nil {
			// IsUnexpectedCloseError filters out normal closure codes (e.g. browser tab closed)
			// and only logs truly unexpected disconnects
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseNormalClosure,
			) {
				log.Printf("unexpected close for user %s: %v", c.userID, err)
			}
			return
		}
		// Drain any messages the client sends — not used in Week 1
	}
}