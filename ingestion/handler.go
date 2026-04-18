package main

import (
	"encoding/json"
	"log"
	"net/http"
<<<<<<< HEAD
	"slices"
	"time"
)

var allowedEventTypes = []string{"POST", "LIKE", "COMMENT"}

// EventRequest is the body the client sends to POST /event.
type EventRequest struct {
	Type     string `json:"type"`
	FromUser string `json:"from_user"`
	Details  string `json:"detail"`
}

// KafkaEvent is the full payload published to Kafka.
// Shape matches fanout/internal/consumer.NotificationEvent JSON fields.
type KafkaEvent struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Detail    string `json:"detail"`
	Timestamp int64  `json:"timestamp"`
=======
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
>>>>>>> ts-notifications-read-api
}

type Handler struct {
	producer *Producer
}

<<<<<<< HEAD
func (h *Handler) HandleEvent(w http.ResponseWriter, r *http.Request) {
=======
// This function runs every time someone calls POST /event
func (h *Handler) HandleEvent(w http.ResponseWriter, r *http.Request) {
	// Only allow POST requests. If someone sends a GET, return 405.

>>>>>>> ts-notifications-read-api
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req EventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

<<<<<<< HEAD
	if req.Type == "" || req.FromUser == "" {
		http.Error(w, "type and from_user are required", http.StatusBadRequest)
		return
	}
	if !slices.Contains(allowedEventTypes, req.Type) {
		http.Error(w, "type must be one of POST, LIKE, COMMENT", http.StatusBadRequest)
=======
	if req.Type == "" || req.FromUser == "" || len(req.Recipients) == 0 {
		http.Error(w, "type, from_user, and recipients are required", http.StatusBadRequest)
>>>>>>> ts-notifications-read-api
		return
	}

	event := KafkaEvent{
<<<<<<< HEAD
		ID:        NewSnowflakeID(),
		Type:      req.Type,
		Detail:    req.Details,
		Timestamp: time.Now().UnixMilli(),
	}

=======
		ID:         NewSnowflakeID(),
		Type:       req.Type,
		FromUser:   req.FromUser,
		Recipients: req.Recipients,
	}

	// Convert the struct to JSON bytes so it can be sent over Kafka.
>>>>>>> ts-notifications-read-api
	payload, err := json.Marshal(event)
	if err != nil {
		log.Printf("marshal error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

<<<<<<< HEAD
	if err := h.producer.Publish(req.FromUser, payload); err != nil {
=======
	if err := h.producer.Publish(payload); err != nil {
>>>>>>> ts-notifications-read-api
		log.Printf("kafka publish error: %v", err)
		http.Error(w, "failed to publish event", http.StatusInternalServerError)
		return
	}

<<<<<<< HEAD
	log.Printf("published event id=%d type=%s key=%s detail=%q ts_ms=%d",
		event.ID, event.Type, req.FromUser, event.Detail, event.Timestamp)
=======
	log.Printf("published event id=%s type=%s from=%s recipients=%v",
		event.ID, event.Type, event.FromUser, event.Recipients)
>>>>>>> ts-notifications-read-api

	w.WriteHeader(http.StatusOK)
}