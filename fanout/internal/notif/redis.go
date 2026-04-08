package notif

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Tanyayya/NotificationSystem/fanout/internal/consumer"
	"github.com/redis/go-redis/v9"
)

// Payload is the JSON body published to Redis channels notif:{userID}.
type Payload struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	FromUser  string `json:"from_user"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"` // Unix milliseconds
}

// Publisher sends fixed-shape notifications to Redis Pub/Sub.
type Publisher struct {
	rdb     *redis.Client
	payload Payload
}

// NewPublisher connects to Redis and builds the notification template from config fields.
func NewPublisher(redisAddr, notifyType, fromUser, message string) *Publisher {
	return &Publisher{
		rdb: redis.NewClient(&redis.Options{Addr: redisAddr}),
		payload: Payload{
			Type:     notifyType,
			FromUser: fromUser,
			Message:  message,
		},
	}
}

// Close releases the Redis client.
func (p *Publisher) Close() error {
	return p.rdb.Close()
}

// Publish sends a notification derived from the Kafka event to channel notif:{userID}.
func (p *Publisher) Publish(ctx context.Context, userID string, ev consumer.NotificationEvent) error {
	fromUser := ev.FromUser
	if fromUser == "" {
		fromUser = p.payload.FromUser
	}
	body, err := json.Marshal(Payload{
		ID:        ev.ID,
		Type:      ev.Type,
		FromUser:  fromUser,
		Message:   ev.Detail,
		Timestamp: ev.Timestamp,
	})
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}
	ch := "notif:" + userID
	return p.rdb.Publish(ctx, ch, body).Err()
}
