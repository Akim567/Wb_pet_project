// internal/kafka/consumer.go
package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/segmentio/kafka-go"

	"wb-orders/internal/cache"
	"wb-orders/internal/models"
	"wb-orders/internal/storage"
)

type Consumer struct {
	reader *kafka.Reader
	repo   *storage.Repo
	cache  *cache.LRU
}

// Теперь создаём Consumer с зависимостями
func NewConsumer(repo *storage.Repo, c *cache.LRU) *Consumer {
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

	return &Consumer{reader: r, repo: repo, cache: c}
}

func (c *Consumer) Run(ctx context.Context) error {
	for {
		m, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				// нормальное завершение по отмене контекста
				return nil
			}
			return fmt.Errorf("read message: %w", err)
		}

		// Парсим JSON в структуру заказа
		var ord models.Order
		if err := json.Unmarshal(m.Value, &ord); err != nil {
			log.Printf("[kafka] skip: bad json (offset=%d): %v", m.Offset, err)
			continue
		}

		// Валидация
		if err := storage.ValidateOrder(ord); err != nil {
			log.Printf("[kafka] skip: invalid order (offset=%d): %v", m.Offset, err)
			continue
		}

		// Сохраняем в БД (идемпотентно)
		if err := c.repo.UpsertOrder(ctx, ord); err != nil {
			log.Printf("[kafka] store error offset=%d id=%s: %v",
				m.Offset, ord.OrderUID, err)
			continue
		}

		// Обновляем кэш
		c.cache.Set(ord.OrderUID, ord)

		// Краткий лог
		log.Printf("[kafka] stored order: id=%s items=%d offset=%d",
			ord.OrderUID, len(ord.Items), m.Offset)
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
