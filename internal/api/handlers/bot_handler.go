package handlers

import (
	"context"

	"github.com/central-university-dev/go-Matthew11K/internal/api/openapi/v1/v1_bot"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
)

type BotService interface {
	ProcessCommand(ctx context.Context, command *models.Command) (string, error)
	ProcessMessage(ctx context.Context, chatID int64, userID int64, text string, username string) (string, error)
	SendLinkUpdate(ctx context.Context, update *models.LinkUpdate) error
}

type BotHandler struct {
	botService BotService
}

func NewBotHandler(botService BotService) *BotHandler {
	return &BotHandler{
		botService: botService,
	}
}

func (h *BotHandler) UpdatesPost(ctx context.Context, req *v1_bot.LinkUpdate) (v1_bot.UpdatesPostRes, error) {
	update := &models.LinkUpdate{
		TgChatIDs: req.TgChatIds,
	}

	if req.ID.IsSet() {
		update.ID = req.ID.Value
	}

	if req.URL.IsSet() {
		update.URL = req.URL.Value.String()
	}

	if req.Description.IsSet() {
		update.Description = req.Description.Value
	}

	if err := h.botService.SendLinkUpdate(ctx, update); err != nil {
		errResp := &v1_bot.ApiErrorResponse{
			Description: v1_bot.NewOptString("Ошибка при отправке обновления"),
		}

		return errResp, err
	}

	return &v1_bot.UpdatesPostOK{}, nil
}
