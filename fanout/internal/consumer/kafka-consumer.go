package consumer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"strconv"
	"time"

	"github.com/segmentio/kafka-go"
)

// NotificationEvent combines the parsed Kafka message key and JSON value body.
// For mode="write" messages the key is the recipient ID (pre-resolved by the ingester).
// For mode="read" messages the key is the sender ID; the fan-out worker stores one event record.
type NotificationEvent struct {
	ID          int64
	Type        string
	Detail      string
	Timestamp   int64
	FromUser    string
	RecipientID string // populated from the Kafka message key
	Mode        string // "write" or "read"
}

// parseSnowflakeID decodes a JSON id field as a number or a base-10 string.
func parseSnowflakeID(raw json.RawMessage) (int64, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return 0, errors.New("missing id")
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return 0, err
		}
		return strconv.ParseInt(s, 10, 64)
	}
	var n int64
	if err := json.Unmarshal(raw, &n); err != nil {
		return 0, err
	}
	return n, nil
}

// Notifier processes a notification event after it is read from Kafka.
type Notifier interface {
	Publish(ctx context.Context, keyID string, ev NotificationEvent) error
}

// Run reads messages from the given topic until the context is cancelled.
// keyID passed to Publish is the raw Kafka message key — recipient ID for write-mode
// messages, sender ID for read-mode messages.
func Run(ctx context.Context, brokers []string, topic, groupID string, n Notifier) error {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 1,
		MaxBytes: 10e6, // 10MB
		MaxWait:  2 * time.Second,
	})
	defer func() {
		if err := r.Close(); err != nil {
			log.Printf("kafka reader close: %v", err)
		}
	}()

	log.Printf("kafka consumer started topic=%q group=%q brokers=%v", topic, groupID, brokers)

	for {
		m, err := r.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}

		var raw struct {
			ID       json.RawMessage `json:"id"`
			Type     string          `json:"type"`
			Detail   string          `json:"detail"`
			FromUser string          `json:"from_user"`
			Timestamp int64          `json:"timestamp"`
			Mode     string          `json:"mode"`
		}
		if err := json.Unmarshal(m.Value, &raw); err != nil {
			log.Printf("kafka message json error partition=%d offset=%d: %v", m.Partition, m.Offset, err)
		} else {
			id, err := parseSnowflakeID(raw.ID)
			if err != nil {
				log.Printf("kafka message id partition=%d offset=%d: %v", m.Partition, m.Offset, err)
			} else {
				keyID := string(m.Key)
				ev := NotificationEvent{
					ID:          id,
					Type:        raw.Type,
					Detail:      raw.Detail,
					Timestamp:   raw.Timestamp,
					FromUser:    raw.FromUser,
					RecipientID: keyID,
					Mode:        raw.Mode,
				}
				log.Printf(
					"kafka message partition=%d offset=%d key=%q mode=%q id=%d type=%q from=%q",
					m.Partition, m.Offset, keyID, ev.Mode, ev.ID, ev.Type, ev.FromUser,
				)

				if err := n.Publish(ctx, keyID, ev); err != nil {
					log.Printf("publish keyID=%q: %v", keyID, err)
				}
			}
		}

		if err := r.CommitMessages(ctx, m); err != nil {
			log.Printf("kafka commit: %v", err)
		}
	}
}
