package main

import (
	"github.com/IBM/sarama"
)

type Producer struct {
	sp    sarama.SyncProducer
	topic string
}

// NewProducer creates a sync producer for the given bootstrap brokers and topic.
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

// Publish sends payload to Kafka with the given key.
// For write-mode messages the key is the recipient ID; for read-mode it is the sender ID.
func (p *Producer) Publish(key string, payload []byte) error {
	msg := &sarama.ProducerMessage{
		Topic: p.topic,
		Value: sarama.ByteEncoder(payload),
	}
	if key != "" {
		msg.Key = sarama.StringEncoder(key)
	}
	_, _, err := p.sp.SendMessage(msg)
	return err
}

func (p *Producer) Close() {
	_ = p.sp.Close()
}
