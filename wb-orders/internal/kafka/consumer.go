package kafka

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/segmentio/kafka-go"
)

type Consumer struct {
	reader *kafka.Reader
}

func NewConsumer() *Consumer {
	brokers := os.Getenv("KAFKA_BROKERS")
	topic := os.Getenv("KAFKA_TOPIC_ORDERS")
	groupID := os.Getenv("KAFKA_GROUP_ORDERS")

	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        []string{brokers},
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       1,           // минимум 1 байт
		MaxBytes:       10e6,        // максимум ~10Мб
		CommitInterval: time.Second, // как часто фиксировать офсеты
	})

	return &Consumer{reader: r}
}

func (c *Consumer) Run(ctx context.Context) error {
	for {
		m, err := c.reader.ReadMessage(ctx)
		if err != nil {
			return fmt.Errorf("read message: %w", err)
		}
		log.Printf("got message at offset %d: key=%s value=%s",
			m.Offset, string(m.Key), string(m.Value))
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
