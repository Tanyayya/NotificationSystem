package main

import (
	"encoding/json"
	"log"
	"net/http"
	"slices"
	"time"
)

var allowedEventTypes = []string{"POST", "LIKE", "COMMENT"}

// EventRequest is the body the client sends to POST /event.
type EventRequest struct {
	Type     string `json:"type"`
	FromUser string `json:"from_user"`
	Detail   string `json:"detail"`
}

// KafkaEvent is the full payload published to Kafka.
// Mode is "write" for pre-addressed messages (key = recipient_id)
// and "read" for single-event messages consumed by fan-out-on-read (key = from_user).
type KafkaEvent struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Detail    string `json:"detail"`
	FromUser  string `json:"from_user"`
	Timestamp int64  `json:"timestamp"`
	Mode      string `json:"mode"`
}

type Handler struct {
	producer  *Producer
	db        *FollowerDB
	mode      string // FAN_OUT_WRITE, FAN_OUT_READ, or FAN_OUT_HYBRID
	threshold int    // follower count above which HYBRID switches to fan-out-on-read
}

func (h *Handler) HandleEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req EventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Type == "" || req.FromUser == "" {
		http.Error(w, "type and from_user are required", http.StatusBadRequest)
		return
	}
	if !slices.Contains(allowedEventTypes, req.Type) {
		http.Error(w, "type must be one of POST, LIKE, COMMENT", http.StatusBadRequest)
		return
	}

	event := KafkaEvent{
		ID:        NewSnowflakeID(),
		Type:      req.Type,
		Detail:    req.Detail,
		FromUser:  req.FromUser,
		Timestamp: time.Now().UnixMilli(),
	}

	ctx := r.Context()

	switch h.mode {
	case "FAN_OUT_READ":
		if err := h.publishRead(event); err != nil {
			log.Printf("kafka publish error: %v", err)
			http.Error(w, "failed to publish event", http.StatusInternalServerError)
			return
		}
	case "FAN_OUT_WRITE":
		followers, err := h.db.GetFollowers(ctx, req.FromUser)
		if err != nil {
			log.Printf("follower lookup error user=%s: %v", req.FromUser, err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if err := h.publishWrite(event, followers); err != nil {
			log.Printf("kafka publish error: %v", err)
			http.Error(w, "failed to publish event", http.StatusInternalServerError)
			return
		}
	default: // FAN_OUT_HYBRID
		followers, err := h.db.GetFollowers(ctx, req.FromUser)
		if err != nil {
			log.Printf("follower lookup error user=%s: %v", req.FromUser, err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if len(followers) > h.threshold {
			if err := h.publishRead(event); err != nil {
				log.Printf("kafka publish error: %v", err)
				http.Error(w, "failed to publish event", http.StatusInternalServerError)
				return
			}
		} else {
			if err := h.publishWrite(event, followers); err != nil {
				log.Printf("kafka publish error: %v", err)
				http.Error(w, "failed to publish event", http.StatusInternalServerError)
				return
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}

// publishWrite publishes one Kafka message per follower, keyed by follower ID.
// With no followers, nothing would reach the worker for DB persistence; use the
// read path (one message, one events row) instead.
func (h *Handler) publishWrite(event KafkaEvent, followers []string) error {
	if len(followers) == 0 {
		return h.publishRead(event)
	}
	event.Mode = "write"
	for _, followerID := range followers {
		payload, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if err := h.producer.Publish(followerID, payload); err != nil {
			return err
		}
		log.Printf("published event id=%d type=%s from=%s to=%s", event.ID, event.Type, event.FromUser, followerID)
	}
	return nil
}

// publishRead publishes a single Kafka message keyed by the sender ID.
// The fan-out worker will store it as one event record; recipients are resolved at read time.
func (h *Handler) publishRead(event KafkaEvent) error {
	event.Mode = "read"
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if err := h.producer.Publish(event.FromUser, payload); err != nil {
		return err
	}
	log.Printf("published read-mode event id=%d type=%s from=%s", event.ID, event.Type, event.FromUser)
	return nil
}
