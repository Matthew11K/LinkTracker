package clients

import (
	"context"
	"fmt"
	"net/url"

	"github.com/central-university-dev/go-Matthew11K/internal/api/openapi/v1/v1_bot"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
)

type BotAPIClient struct {
	client *v1_bot.Client
}

type BotClient interface {
	SendUpdate(ctx context.Context, update *models.LinkUpdate) error
}

func NewBotAPIClient(baseURL string) (BotClient, error) {
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	client, err := v1_bot.NewClient(baseURL)
	if err != nil {
		return nil, fmt.Errorf("ошибка при создании клиента бота: %w", err)
	}

	return &BotAPIClient{
		client: client,
	}, nil
}

func (c *BotAPIClient) SendUpdate(ctx context.Context, update *models.LinkUpdate) error {
	req := &v1_bot.LinkUpdate{
		ID:          v1_bot.NewOptInt64(update.ID),
		TgChatIds:   update.TgChatIDs,
		Description: v1_bot.NewOptString(update.Description),
	}

	if update.URL != "" {
		parsedURL, err := url.Parse(update.URL)
		if err == nil {
			req.URL = v1_bot.NewOptURI(*parsedURL)
		}
	}

	_, err := c.client.UpdatesPost(ctx, req)

	return err
}
