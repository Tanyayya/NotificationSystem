package main

import (
	"github.com/IBM/sarama"
)

const topic = "notification-events"

type Producer struct {
	sp sarama.SyncProducer
}

func NewProducer(broker string) (*Producer, error) {
	cfg := sarama.NewConfig()
	cfg.Producer.Return.Successes = true
	// Partition by recipient so per-user ordering is preserved.
	// Person B (fan-out worker) consumes from this topic.
	cfg.Producer.Partitioner = sarama.NewHashPartitioner

	sp, err := sarama.NewSyncProducer([]string{broker}, cfg)
	if err != nil {
		return nil, err
	}
	return &Producer{sp: sp}, nil
}

func (p *Producer) Publish(payload []byte) error {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(payload),
	}
	_, _, err := p.sp.SendMessage(msg)
	return err
}

func (p *Producer) Close() {
	_ = p.sp.Close()
}