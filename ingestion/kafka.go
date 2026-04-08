package main

import (
	"github.com/IBM/sarama"
)

type Producer struct {
	sp    sarama.SyncProducer
	topic string
}

// NewProducer creates a sync producer for the given bootstrap brokers and topic.
// Brokers must match the listener clients use (e.g. kafka:29092 on Docker Compose
// for the PLAINTEXT listener; localhost:9092 from the host for PLAINTEXT_HOST).
func NewProducer(brokers []string, topic string) (*Producer, error) {
	cfg := sarama.NewConfig()
	cfg.Producer.Return.Successes = true
	cfg.Producer.Partitioner = sarama.NewHashPartitioner

	sp, err := sarama.NewSyncProducer(brokers, cfg)
	if err != nil {
		return nil, err
	}
	return &Producer{sp: sp, topic: topic}, nil
}

func (p *Producer) Publish(payload []byte) error {
	msg := &sarama.ProducerMessage{
		Topic: p.topic,
		Value: sarama.ByteEncoder(payload),
	}
	_, _, err := p.sp.SendMessage(msg)
	return err
}

func (p *Producer) Close() {
	_ = p.sp.Close()
}