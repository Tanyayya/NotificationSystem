package fanout

import (
	"context"
	"log"

	"github.com/Tanyayya/NotificationSystem/fanout/internal/consumer"
	"github.com/Tanyayya/NotificationSystem/fanout/internal/db"
	"github.com/Tanyayya/NotificationSystem/fanout/internal/notif"
)

// Mode controls which fan-out strategy the worker uses.
type Mode string

const (
	ModeRead   Mode = "FAN_OUT_READ"
	ModeWrite  Mode = "FAN_OUT_WRITE"
	ModeHybrid Mode = "FAN_OUT_HYBRID"
)

// FanOuter sits between the Kafka consumer and the Redis publisher.
// It implements consumer.Notifier so it can be passed directly to consumer.Run.
//
// For each event it:
//  1. Looks up the follower list from PostgreSQL
//  2. Chooses fan-out on write or fan-out on read based on mode (and threshold for HYBRID)
//  3. Fan-out on write: persists one notification per follower + publishes to Redis
//  4. Fan-out on read:  persists one event record (stub — read path not yet implemented)
type FanOuter struct {
	db        *db.DB
	publisher *notif.Publisher
	threshold int  // follower count above which HYBRID switches to fan-out on read
	mode      Mode // fan-out strategy: FAN_OUT_READ, FAN_OUT_WRITE, or HYBRID
}

// New creates a FanOuter with the given DB, Redis publisher, threshold, and mode.
func New(database *db.DB, publisher *notif.Publisher, threshold int, mode Mode) *FanOuter {
	return &FanOuter{
		db:        database,
		publisher: publisher,
		threshold: threshold,
		mode:      mode,
	}
}

// Publish implements consumer.Notifier.
// fromUser is the Kafka message key — the person who triggered the event.
// The fan-out worker calls this once per Kafka message.
func (f *FanOuter) Publish(ctx context.Context, fromUser string, ev consumer.NotificationEvent) error {
	// FAN_OUT_READ skips the follower lookup — one event record stored, no per-follower writes.
	if f.mode == ModeRead {
		log.Printf("fanout: mode=%s user=%q", f.mode, fromUser)
		return f.fanoutOnRead(ctx, ev)
	}

	// All other modes need the follower list.
	followers, err := f.db.GetFollowers(ctx, fromUser)
	if err != nil {
		return err
	}

	if len(followers) == 0 {
		log.Printf("fanout: no followers for user=%q, skipping", fromUser)
		return nil
	}

	switch f.mode {
	case ModeWrite:
		log.Printf("fanout: mode=%s user=%q followers=%d", f.mode, fromUser, len(followers))
		return f.fanoutOnWrite(ctx, followers, ev)
	default: // ModeHybrid
		log.Printf("fanout: mode=%s user=%q followers=%d threshold=%d", f.mode, fromUser, len(followers), f.threshold)
		if len(followers) <= f.threshold {
			return f.fanoutOnWrite(ctx, followers, ev)
		}
		return f.fanoutOnRead(ctx, ev)
	}
}

// fanoutOnWrite is the write path — used for normal users under the threshold.
// For each follower:
//   - writes a notification record to PostgreSQL (delivered=false)
//   - publishes to notif:{followerID} on Redis Pub/Sub
//
// Redis publish errors are logged but do not stop the loop —
// the notification is already persisted and will be replayed on reconnect.
func (f *FanOuter) fanoutOnWrite(ctx context.Context, followers []string, ev consumer.NotificationEvent) error {
	log.Printf("fanout-on-write: event id=%d type=%q from=%q recipients=%d",
		ev.ID, ev.Type, ev.FromUser, len(followers))

	var publishErrors int
	for _, followerID := range followers {
		// persist to PostgreSQL — ensures offline users get it on reconnect
		if err := f.db.InsertNotification(ctx, followerID, ev); err != nil {
			log.Printf("fanout-on-write: insert notification follower=%q: %v", followerID, err)
			continue
		}

		// publish to Redis Pub/Sub — delivers to online users instantly
		if err := f.publisher.Publish(ctx, followerID, ev); err != nil {
			log.Printf("fanout-on-write: redis publish follower=%q: %v", followerID, err)
			publishErrors++
			// don't return — notification is persisted, continue to next follower
		}
	}

	if publishErrors > 0 {
		log.Printf("fanout-on-write: %d redis publish errors (notifications persisted, will replay on reconnect)", publishErrors)
	}

	log.Printf("fanout-on-write: done event id=%d recipients=%d errors=%d",
		ev.ID, len(followers), publishErrors)
	return nil
}

// fanoutOnRead is the read path — used for high-follower accounts above the threshold.
// Instead of writing one notification per follower, we store the event once.
// Recipients are resolved at read time when they open their notification feed.
//
// NOTE: This is a stub — the full read path (resolving recipients at delivery time)
// will be implemented in Week 3 as part of Experiment 1.
func (f *FanOuter) fanoutOnRead(ctx context.Context, ev consumer.NotificationEvent) error {
	log.Printf("fanout-on-read: high-follower account user=%q, storing event id=%d (read path stub)",
		ev.FromUser, ev.ID)

	// store one event record instead of N notification records
	if err := f.db.InsertEvent(ctx, ev); err != nil {
		return err
	}

	log.Printf("fanout-on-read: event id=%d stored, recipients resolved at read time", ev.ID)
	return nil
}