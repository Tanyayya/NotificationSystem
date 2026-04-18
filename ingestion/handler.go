package main

import (
	"encoding/json"
	"log"
	"net/http"
)

// EventRequest is the body the client sends to POST /event.
// Matches Contract 1 (minus the id, which we assign here).
type EventRequest struct {
	Type      string   `json:"type"`
	FromUser  string   `json:"from_user"`
	Recipients []string `json:"recipients"`
}

// KafkaEvent is the full payload published to Kafka.
// fan-out worker consumes this.
type KafkaEvent struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	FromUser   string   `json:"from_user"`
	Recipients []string `json:"recipients"`
}

type Handler struct {
	producer *Producer
}

// This function runs every time someone calls POST /event
func (h *Handler) HandleEvent(w http.ResponseWriter, r *http.Request) {
	// Only allow POST requests. If someone sends a GET, return 405.

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req EventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Type == "" || req.FromUser == "" || len(req.Recipients) == 0 {
		http.Error(w, "type, from_user, and recipients are required", http.StatusBadRequest)
		return
	}

	event := KafkaEvent{
		ID:         NewSnowflakeID(),
		Type:       req.Type,
		FromUser:   req.FromUser,
		Recipients: req.Recipients,
	}

	// Convert the struct to JSON bytes so it can be sent over Kafka.
	payload, err := json.Marshal(event)
	if err != nil {
		log.Printf("marshal error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := h.producer.Publish(payload); err != nil {
		log.Printf("kafka publish error: %v", err)
		http.Error(w, "failed to publish event", http.StatusInternalServerError)
		return
	}

	log.Printf("published event id=%s type=%s from=%s recipients=%v",
		event.ID, event.Type, event.FromUser, event.Recipients)

	w.WriteHeader(http.StatusOK)
}