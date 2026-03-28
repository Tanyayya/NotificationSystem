package notif

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// Payload is the JSON body published to Redis channels notif:{userID}.
type Payload struct {
	Type     string `json:"type"`
	FromUser string `json:"from_user"`
	Message  string `json:"message"`
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

// Publish sends the configured payload to channel notif:{userID}.
func (p *Publisher) Publish(ctx context.Context, userID string) error {
	body, err := json.Marshal(p.payload)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}
	ch := "notif:" + userID
	return p.rdb.Publish(ctx, ch, body).Err()
}
