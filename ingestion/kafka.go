package main

import (
	"github.com/IBM/sarama"
)

<<<<<<< HEAD
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

func (p *Producer) Publish(key string, payload []byte) error {
	msg := &sarama.ProducerMessage{
		Topic: p.topic,
		Value: sarama.ByteEncoder(payload),
	}
	if key != "" {
		msg.Key = sarama.StringEncoder(key)
	}
=======
const topic = "notification-events"

// A wrapper around Sarama's producer
type Producer struct {
	sp sarama.SyncProducer
}

func NewProducer(broker string) (*Producer, error) {
	cfg := sarama.NewConfig()
	cfg.Producer.Return.Successes = true
	// Partition by recipient so per-user ordering is preserved.
	// (fan-out worker) consumes from this topic.
	cfg.Producer.Partitioner = sarama.NewHashPartitioner

	// opens the connection
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
>>>>>>> ts-notifications-read-api
	_, _, err := p.sp.SendMessage(msg)
	return err
}

func (p *Producer) Close() {
	_ = p.sp.Close()
}