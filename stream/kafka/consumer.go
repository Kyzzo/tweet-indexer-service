package kafka

import (
	"context"
	"time"

	"github.com/segmentio/kafka-go"
)

type Consumer struct {
	reader *kafka.Reader
}

func NewConsumer() *Consumer {
	return &Consumer{}
}

func (c *Consumer) Connect(brokers []string, topic string, cgroup string) {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		GroupID:        cgroup,
		Topic:          topic,
		MinBytes:       10e3,        // 10KB
		MaxBytes:       10e6,        // 10MB
		CommitInterval: time.Second, // flushes commits to Kafka every second
	})
	c.reader = r
}

func (c *Consumer) Consume() <-chan []byte {
	ch := make(chan []byte)
	go func() {
		for {
			m, err := c.reader.ReadMessage(context.Background())
			if err != nil {
				break
			}
			ch <- m.Value
		}
	}()
	return ch
}
