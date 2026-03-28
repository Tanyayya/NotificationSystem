package consumer

import (
	"context"
	"errors"
	"io"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

// Run reads messages from the given topic until the context is cancelled.
func Run(ctx context.Context, brokers []string, topic, groupID string) error {
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

		log.Printf(
			"kafka message partition=%d offset=%d key=%q value=%s",
			m.Partition, m.Offset, string(m.Key), string(m.Value),
		)

		if err := r.CommitMessages(ctx, m); err != nil {
			log.Printf("kafka commit: %v", err)
		}
	}
}
