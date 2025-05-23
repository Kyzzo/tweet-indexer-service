package kafka

import (
	"context"
	"crypto/tls"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	"github.com/jeetdotso/tweet-indexer-service/stream"
)

type Producer struct {
	writer *kafka.Writer
}

// NewProducer builds and returns a stream.Producer backed by kafka-go
func NewProducer(cfg config.Stream, topic string) stream.Producer {
	p := &Producer{}
	p.Connect(cfg, topic)
	return p
}

// Connect dials your VPC‐peered Kafka (:9092) using TLS only
func (p *Producer) Connect(cfg config.Stream, topic string) {
	logg.Log.Info().
		Msgf("Connecting to Kafka brokers (private, TLS only): %v", cfg.Kafka.Brokers)

	// configure a TLS‐enabled dialer
	dialer := &kafka.Dialer{
		Timeout:   10 * time.Second,
		DualStack: true,
		TLS: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	// build the writer
	p.writer = kafka.NewWriter(kafka.WriterConfig{
		Brokers:      cfg.Kafka.Brokers, // e.g. ["<your-host>:9092"]
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 1 * time.Millisecond,
		Dialer:       dialer,
	})
}

// Produce sends a single message (with optional key) to Kafka
func (p *Producer) Produce(key, msg []byte) error {
	m := kafka.Message{Key: key, Value: msg}
	if err := p.writer.WriteMessages(context.Background(), m); err != nil {
		logg.Log.Error().Err(err).Msg("Failed to produce message to Kafka")
		return err
	}
	return nil
}
