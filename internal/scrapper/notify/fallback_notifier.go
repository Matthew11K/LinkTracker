package notify

import (
	"context"
	"log/slog"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
)

type FallbackBotNotifier struct {
	primary   BotNotifier
	secondary BotNotifier
	logger    *slog.Logger
}

func NewFallbackBotNotifier(primary, secondary BotNotifier, logger *slog.Logger) *FallbackBotNotifier {
	return &FallbackBotNotifier{
		primary:   primary,
		secondary: secondary,
		logger:    logger,
	}
}

func (n *FallbackBotNotifier) SendUpdate(ctx context.Context, update *models.LinkUpdate) error {
	err := n.primary.SendUpdate(ctx, update)
	if err == nil {
		return nil
	}

	n.logger.Warn("Основной транспорт недоступен, переключаемся на резервный",
		"primaryError", err,
		"linkID", update.ID,
	)

	fallbackErr := n.secondary.SendUpdate(ctx, update)
	if fallbackErr != nil {
		return err
	}

	n.logger.Info("Уведомление успешно отправлено через резервный транспорт",
		"linkID", update.ID,
	)

	return nil
}
