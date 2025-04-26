package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	boterrors "github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
	"github.com/segmentio/kafka-go"
)

type LinkUpdateMessage struct {
	ID          int64              `json:"id"`
	URL         string             `json:"url"`
	Description string             `json:"description"`
	TgChatIDs   []int64            `json:"tgChatIds"`
	UpdateInfo  *models.UpdateInfo `json:"updateInfo,omitempty"`
}

type MessageHandler interface {
	HandleUpdate(ctx context.Context, update *models.LinkUpdate) error
}

type Consumer struct {
	reader         *kafka.Reader
	dlqWriter      *kafka.Writer
	messageHandler MessageHandler
	logger         *slog.Logger
	linkTopic      string
	dlqTopic       string
}

func NewConsumer(
	brokers []string,
	groupID string,
	linkTopic string,
	dlqTopic string,
	messageHandler MessageHandler,
	logger *slog.Logger,
) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		GroupID:        groupID,
		Topic:          linkTopic,
		MinBytes:       10e3,
		MaxBytes:       10e6,
		CommitInterval: 1 * time.Second,
		Logger:         kafka.LoggerFunc(logger.Debug),
		ErrorLogger:    kafka.LoggerFunc(logger.Error),
	})

	dlqWriter := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        dlqTopic,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
		Logger:       kafka.LoggerFunc(logger.Debug),
		ErrorLogger:  kafka.LoggerFunc(logger.Error),
	}

	return &Consumer{
		reader:         reader,
		dlqWriter:      dlqWriter,
		messageHandler: messageHandler,
		logger:         logger,
		linkTopic:      linkTopic,
		dlqTopic:       dlqTopic,
	}
}

func (c *Consumer) Start(ctx context.Context) {
	c.logger.Info("Запуск потребления сообщений из Kafka",
		"topic", c.linkTopic,
	)

	go func() {
		for {
			select {
			case <-ctx.Done():
				c.logger.Info("Остановка потребления сообщений из Kafka")
				return
			default:
				msg, err := c.reader.ReadMessage(ctx)
				if err != nil {
					c.logger.Error("Ошибка при чтении сообщения из Kafka",
						"error", err,
					)

					continue
				}

				c.logger.Info("Получено сообщение из Kafka",
					"topic", msg.Topic,
					"partition", msg.Partition,
					"offset", msg.Offset,
				)

				if err := c.processMessage(ctx, &msg); err != nil {
					c.logger.Error("Ошибка при обработке сообщения",
						"error", err,
					)
				}
			}
		}
	}()
}

func (c *Consumer) processMessage(ctx context.Context, msg *kafka.Message) error {
	var linkUpdateMessage LinkUpdateMessage

	if err := json.Unmarshal(msg.Value, &linkUpdateMessage); err != nil {
		c.logger.Error("Ошибка при десериализации сообщения",
			"error", err,
		)

		if sendErr := c.sendToDLQ(ctx, msg.Value, fmt.Sprintf("Ошибка десериализации: %s", err)); sendErr != nil {
			c.logger.Error("Ошибка при отправке сообщения в DLQ",
				"error", sendErr,
			)
		}

		return fmt.Errorf("ошибка при десериализации сообщения: %w", err)
	}

	if linkUpdateMessage.URL == "" {
		errMsg := "отсутствует обязательное поле URL"
		c.logger.Error(errMsg)

		newErr := &boterrors.ErrMissingURLInUpdate{}

		if sendErr := c.sendToDLQ(ctx, msg.Value, newErr.Error()); sendErr != nil {
			c.logger.Error("Ошибка при отправке сообщения в DLQ",
				"error", sendErr,
			)
		}

		return newErr
	}

	update := &models.LinkUpdate{
		ID:          linkUpdateMessage.ID,
		URL:         linkUpdateMessage.URL,
		Description: linkUpdateMessage.Description,
		TgChatIDs:   linkUpdateMessage.TgChatIDs,
		UpdateInfo:  linkUpdateMessage.UpdateInfo,
	}

	if err := c.messageHandler.HandleUpdate(ctx, update); err != nil {
		c.logger.Error("Ошибка при обработке обновления",
			"error", err,
		)

		return fmt.Errorf("ошибка при обработке обновления: %w", err)
	}

	c.logger.Info("Сообщение успешно обработано")

	return nil
}

func (c *Consumer) sendToDLQ(ctx context.Context, message []byte, errMsg string) error {
	c.logger.Info("Отправка сообщения в DLQ",
		"error", errMsg,
		"topic", c.dlqTopic,
	)

	err := c.dlqWriter.WriteMessages(ctx, kafka.Message{
		Key:   []byte("error"),
		Value: message,
		Headers: []kafka.Header{
			{Key: "error", Value: []byte(errMsg)},
			{Key: "timestamp", Value: []byte(time.Now().Format(time.RFC3339))},
		},
		Time: time.Now(),
	})

	if err != nil {
		c.logger.Error("Ошибка при отправке сообщения в DLQ",
			"error", err,
		)

		return fmt.Errorf("ошибка при отправке сообщения в DLQ: %w", err)
	}

	c.logger.Info("Сообщение успешно отправлено в DLQ")

	return nil
}

func (c *Consumer) Close() error {
	if err := c.reader.Close(); err != nil {
		return err
	}

	return c.dlqWriter.Close()
}
