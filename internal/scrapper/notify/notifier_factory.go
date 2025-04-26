package notify

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/central-university-dev/go-Matthew11K/internal/config"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
)

type NotifierType string

const (
	HTTPNotifier  NotifierType = "HTTP"
	KafkaNotifier NotifierType = "Kafka"
)

type NotifierFactory struct {
	config *config.Config
	logger *slog.Logger
}

func NewNotifierFactory(config *config.Config, logger *slog.Logger) *NotifierFactory {
	return &NotifierFactory{
		config: config,
		logger: logger,
	}
}

func (f *NotifierFactory) CreateNotifier() (BotNotifier, error) {
	notifierType := NotifierType(strings.ToUpper(f.config.MessageTransport))

	f.logger.Info("Создание нотификатора",
		"type", notifierType,
	)

	switch notifierType {
	case HTTPNotifier:
		return NewHTTPBotNotifier(f.config.BotBaseURL, f.logger)
	case KafkaNotifier:
		brokers := strings.Split(f.config.KafkaBrokers, ",")
		return NewKafkaBotNotifier(brokers, f.config.TopicLinkUpdates, f.config.TopicDeadLetterQueue, f.logger), nil
	default:
		return nil, fmt.Errorf("неизвестный тип нотификатора: %s", notifierType)
	}
}

type BotNotifier interface {
	SendUpdate(ctx context.Context, update *models.LinkUpdate) error
}
