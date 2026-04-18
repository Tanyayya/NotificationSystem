package fanout

import (
	"context"
	"log"

	"github.com/Tanyayya/NotificationSystem/fanout/internal/consumer"
	"github.com/Tanyayya/NotificationSystem/fanout/internal/db"
	"github.com/Tanyayya/NotificationSystem/fanout/internal/notif"
)

// FanOuter sits between the Kafka consumer and the Redis publisher.
// It implements consumer.Notifier so it can be passed directly to consumer.Run.
//
// Fan-out mode is now decided by the ingester and encoded in each message:
//   - mode="write": keyID is the pre-resolved recipient; write one notification + publish to Redis.
//   - mode="read":  keyID is the sender; store a single event record for read-time resolution.
type FanOuter struct {
	db        *db.DB
	publisher *notif.Publisher
}

// New creates a FanOuter with the given DB and Redis publisher.
func New(database *db.DB, publisher *notif.Publisher) *FanOuter {
	return &FanOuter{
		db:        database,
		publisher: publisher,
	}
}

// Publish implements consumer.Notifier.
// keyID is the raw Kafka message key — recipient ID for write-mode, sender ID for read-mode.
func (f *FanOuter) Publish(ctx context.Context, keyID string, ev consumer.NotificationEvent) error {
	switch ev.Mode {
	case "write":
		log.Printf("fanout: mode=write recipient=%q event_id=%d", keyID, ev.ID)
		return f.writeForRecipient(ctx, keyID, ev)
	case "read":
		log.Printf("fanout: mode=read sender=%q event_id=%d", keyID, ev.ID)
		return f.fanoutOnRead(ctx, ev)
	default:
		log.Printf("fanout: unknown mode=%q event_id=%d, skipping", ev.Mode, ev.ID)
		return nil
	}
}

// writeForRecipient persists one notification record and publishes to Redis Pub/Sub.
// Called once per pre-addressed write-mode message.
func (f *FanOuter) writeForRecipient(ctx context.Context, recipientID string, ev consumer.NotificationEvent) error {
	if err := f.db.InsertNotification(ctx, recipientID, ev); err != nil {
		return err
	}

	if err := f.publisher.Publish(ctx, recipientID, ev); err != nil {
		// Notification is already persisted; log and continue.
		log.Printf("fanout: redis publish recipient=%q: %v", recipientID, err)
	}

	return nil
}

// fanoutOnRead stores a single event record for high-follower accounts.
// Recipients are resolved at read time when they open their notification feed.
func (f *FanOuter) fanoutOnRead(ctx context.Context, ev consumer.NotificationEvent) error {
	log.Printf("fanout-on-read: storing event id=%d from=%q (recipients resolved at read time)", ev.ID, ev.FromUser)

	if err := f.db.InsertEvent(ctx, ev); err != nil {
		return err
	}

	log.Printf("fanout-on-read: event id=%d stored", ev.ID)
	return nil
}
