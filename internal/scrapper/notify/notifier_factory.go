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
		"fallbackEnabled", f.config.FallbackEnabled,
	)

	if f.config.FallbackEnabled {
		return f.createFallbackNotifier(notifierType)
	}

	return f.createSingleNotifier(notifierType)
}

func (f *NotifierFactory) createFallbackNotifier(primaryType NotifierType) (BotNotifier, error) {
	primary, err := f.createSingleNotifier(primaryType)
	if err != nil {
		return nil, fmt.Errorf("ошибка при создании основного нотификатора: %w", err)
	}

	var secondaryType NotifierType
	if strings.EqualFold(f.config.FallbackTransport, string(HTTPNotifier)) {
		secondaryType = HTTPNotifier
	} else {
		secondaryType = KafkaNotifier
	}

	if primaryType == secondaryType {
		f.logger.Warn("Основной и резервный транспорты одинаковые, fallback отключен")
		return primary, nil
	}

	secondary, err := f.createSingleNotifier(secondaryType)
	if err != nil {
		f.logger.Warn("Не удалось создать резервный нотификатор, продолжаем без fallback",
			"error", err,
		)

		return primary, nil
	}

	f.logger.Info("Создан комбинированный нотификатор с fallback",
		"primary", primaryType,
		"secondary", secondaryType,
	)

	return NewFallbackBotNotifier(primary, secondary, f.logger), nil
}

func (f *NotifierFactory) createSingleNotifier(notifierType NotifierType) (BotNotifier, error) {
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
