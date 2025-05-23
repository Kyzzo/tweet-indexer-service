package kafka

import (
	"context"
	"crypto/tls"
	"time"

	"github.com/segmentio/kafka-go"
)

type Consumer struct {
	reader *kafka.Reader
}

// NewConsumer returns a blank Consumer ready to Connect(...)
func NewConsumer() *Consumer {
	return &Consumer{}
}

// Connect dials your VPC-peered Kafka (:9092) using TLS only
func (c *Consumer) Connect(brokers []string, topic, groupID string) {
	// build a TLS-enabled dialer
	dialer := &kafka.Dialer{
		Timeout:   10 * time.Second,
		DualStack: true,
		TLS: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		// if you ever need SASL/PLAIN you can do:
		// SASLMechanism: plain.Mechanism{
		//     Username: cfg.Kafka.Username,
		//     Password: cfg.Kafka.Password,
		// },
	}

	c.reader = kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers, // e.g. []string{"your-host:9092"}
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       10e3,        // 10KB
		MaxBytes:       10e6,        // 10MB
		CommitInterval: time.Second, // flush commits every sec
		Dialer:         dialer,
	})
}

// Consume starts a background loop and emits each message value on the returned channel.
// It will stop on any reader error (and you can inspect c.reader.Stats() or log).
func (c *Consumer) Consume(ctx context.Context) <-chan []byte {
	ch := make(chan []byte)
	go func() {
		defer close(ch)
		for {
			m, err := c.reader.ReadMessage(ctx)
			if err != nil {
				// you can log err here if you like
				return
			}
			ch <- m.Value
		}
	}()
	return ch
}
