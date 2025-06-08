package notify

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/central-university-dev/go-Matthew11K/internal/api/openapi/v1_bot"
	"github.com/central-university-dev/go-Matthew11K/internal/common/httputil"
	"github.com/central-university-dev/go-Matthew11K/internal/config"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
)

type HTTPBotNotifier struct {
	client *v1_bot.Client
	logger *slog.Logger
}

func NewHTTPBotNotifier(baseURL string, cfg *config.Config, logger *slog.Logger) (*HTTPBotNotifier, error) {
	if baseURL == "" {
		baseURL = "http://link_tracker_bot:8080"
	}

	resilientClient := httputil.CreateResilientOpenAPIClient(cfg, logger, "bot_service")

	client, err := v1_bot.NewClient(baseURL, v1_bot.WithClient(resilientClient))
	if err != nil {
		return nil, fmt.Errorf("ошибка при создании клиента бота: %w", err)
	}

	return &HTTPBotNotifier{
		client: client,
		logger: logger,
	}, nil
}

func (n *HTTPBotNotifier) SendUpdate(ctx context.Context, update *models.LinkUpdate) error {
	n.logger.Info("Отправка уведомления в бота",
		"linkID", update.ID,
		"url", update.URL,
		"chats", len(update.TgChatIDs),
	)

	description := formatDescription(update)

	if update.UpdateInfo != nil {
		n.logger.Info("Сформировано детализированное уведомление",
			"linkID", update.ID,
			"title", update.UpdateInfo.Title,
			"type", update.UpdateInfo.ContentType,
		)
	}

	req := &v1_bot.LinkUpdate{
		ID:          v1_bot.NewOptInt64(update.ID),
		TgChatIds:   update.TgChatIDs,
		Description: v1_bot.NewOptString(description),
	}

	if update.URL != "" {
		parsedURL, err := url.Parse(update.URL)
		if err == nil {
			req.URL = v1_bot.NewOptURI(*parsedURL)
		}
	}

	_, err := n.client.UpdatesPost(ctx, req)
	if err != nil {
		n.logger.Error("Ошибка при отправке уведомления в бота",
			"error", err,
		)

		return fmt.Errorf("ошибка при отправке уведомления в бота: %w", err)
	}

	n.logger.Info("Уведомление успешно отправлено")

	return nil
}
