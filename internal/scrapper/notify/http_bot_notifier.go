package notify

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/central-university-dev/go-Matthew11K/internal/api/openapi/v1/v1_bot"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
)

type HTTPBotNotifier struct {
	client *v1_bot.Client
	logger *slog.Logger
}

func NewHTTPBotNotifier(baseURL string, logger *slog.Logger) (*HTTPBotNotifier, error) {
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	client, err := v1_bot.NewClient(baseURL)
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

		n.logger.Info("Сформировано детализированное уведомление",
			"linkID", update.ID,
			"title", info.Title,
			"type", info.ContentType,
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
