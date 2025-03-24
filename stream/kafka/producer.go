package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	"github.com/jeetdotso/tweet-indexer-service/stream"
	"github.com/segmentio/kafka-go"
)

type Producer struct {
	writer kafka.Writer
}

func NewProducer(cfg config.Stream, topic string) stream.Producer {
	cli := Producer{}
	cli.Connect(cfg, topic)
	return &cli
}

func (c *Producer) Connect(cfg config.Stream, topic string) {
	logg.Log.Info().Msg(fmt.Sprintf("Connecting to kafka broker: %v", cfg.Kafka.Brokers))

	w := kafka.Writer{
		Addr:                   kafka.TCP(cfg.Kafka.Brokers...),
		Topic:                  topic,
		BatchTimeout:           time.Millisecond * 1,
		AllowAutoTopicCreation: true,
	}

	c.writer = w
}

func (c *Producer) Produce(msg []byte) error {
	m := kafka.Message{
		Value: msg,
	}

	if err := c.writer.WriteMessages(context.Background(), m); err != nil {
		logg.Log.Error().Err(err).Msg("Failed to produce message to kafka.")
		return err
	}

	return nil
}
