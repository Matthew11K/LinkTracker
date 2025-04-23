package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
	"github.com/segmentio/kafka-go"
)

type KafkaBotNotifier struct {
	producer    *kafka.Writer
	dlqProducer *kafka.Writer
	logger      *slog.Logger
	linkTopic   string
	dlqTopic    string
}

type LinkUpdateMessage struct {
	ID          int64              `json:"id"`
	URL         string             `json:"url"`
	Description string             `json:"description"`
	TgChatIDs   []int64            `json:"tgChatIds"`
	UpdateInfo  *models.UpdateInfo `json:"updateInfo,omitempty"`
}

func NewKafkaBotNotifier(brokers []string, linkTopic, dlqTopic string, logger *slog.Logger) *KafkaBotNotifier {
	producer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        linkTopic,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
		Logger:       kafka.LoggerFunc(logger.Debug),
		ErrorLogger:  kafka.LoggerFunc(logger.Error),
	}

	dlqProducer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        dlqTopic,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
		Logger:       kafka.LoggerFunc(logger.Debug),
		ErrorLogger:  kafka.LoggerFunc(logger.Error),
	}

	return &KafkaBotNotifier{
		producer:    producer,
		dlqProducer: dlqProducer,
		logger:      logger,
		linkTopic:   linkTopic,
		dlqTopic:    dlqTopic,
	}
}

func (n *KafkaBotNotifier) SendUpdate(ctx context.Context, update *models.LinkUpdate) error {
	n.logger.Info("Отправка уведомления в Kafka",
		"linkID", update.ID,
		"url", update.URL,
		"chats", len(update.TgChatIDs),
		"topic", n.linkTopic,
	)

	message := LinkUpdateMessage{
		ID:          update.ID,
		URL:         update.URL,
		Description: n.formatDescription(update),
		TgChatIDs:   update.TgChatIDs,
		UpdateInfo:  update.UpdateInfo,
	}

	value, err := json.Marshal(message)
	if err != nil {
		n.logger.Error("Ошибка при сериализации сообщения",
			"error", err,
		)

		return fmt.Errorf("ошибка при сериализации сообщения: %w", err)
	}

	err = n.producer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(fmt.Sprintf("%d", update.ID)),
		Value: value,
		Time:  time.Now(),
	})

	if err != nil {
		n.logger.Error("Ошибка при отправке сообщения в Kafka",
			"error", err,
		)

		return fmt.Errorf("ошибка при отправке сообщения в Kafka: %w", err)
	}

	n.logger.Info("Уведомление успешно отправлено в Kafka")

	return nil
}

func (n *KafkaBotNotifier) SendToDLQ(ctx context.Context, message []byte, errMsg string) error {
	n.logger.Info("Отправка сообщения в DLQ",
		"error", errMsg,
		"topic", n.dlqTopic,
	)

	err := n.dlqProducer.WriteMessages(ctx, kafka.Message{
		Key:   []byte("error"),
		Value: message,
		Headers: []kafka.Header{
			{Key: "error", Value: []byte(errMsg)},
			{Key: "timestamp", Value: []byte(time.Now().Format(time.RFC3339))},
		},
		Time: time.Now(),
	})

	if err != nil {
		n.logger.Error("Ошибка при отправке сообщения в DLQ",
			"error", err,
		)

		return fmt.Errorf("ошибка при отправке сообщения в DLQ: %w", err)
	}

	n.logger.Info("Сообщение успешно отправлено в DLQ")

	return nil
}

func (n *KafkaBotNotifier) Close() error {
	if err := n.producer.Close(); err != nil {
		return err
	}

	return n.dlqProducer.Close()
}

func (n *KafkaBotNotifier) formatDescription(update *models.LinkUpdate) string {
	description := update.Description

	if update.UpdateInfo != nil {
		info := update.UpdateInfo

		switch info.ContentType {
		case "repository", "issue", "pull_request":
			description = fmt.Sprintf("%s\n\n🔷 GitHub обновление 🔷\n"+
				"📎 Название: %s\n"+
				"👤 Автор: %s\n"+
				"⏱️ Время: %s\n"+
				"📄 Тип: %s\n"+
				"📝 Превью:\n%s",
				description, info.Title, info.Author,
				info.UpdatedAt.Format("2006-01-02 15:04:05"),
				info.ContentType, info.TextPreview)
		case "question", "answer", "comment":
			description = fmt.Sprintf("%s\n\n🔶 StackOverflow обновление 🔶\n"+
				"📎 Тема вопроса: %s\n"+
				"👤 Пользователь: %s\n"+
				"⏱️ Время: %s\n"+
				"📄 Тип: %s\n"+
				"📝 Превью:\n%s",
				description, info.Title, info.Author,
				info.UpdatedAt.Format("2006-01-02 15:04:05"),
				info.ContentType, info.TextPreview)
		default:
			description = fmt.Sprintf("%s\n\n🔹 Обновление ресурса 🔹\n"+
				"📎 Заголовок: %s\n"+
				"👤 Автор: %s\n"+
				"⏱️ Время: %s\n"+
				"📄 Тип: %s\n"+
				"📝 Превью:\n%s",
				description, info.Title, info.Author,
				info.UpdatedAt.Format("2006-01-02 15:04:05"),
				info.ContentType, info.TextPreview)
		}
	}

	return description
}
