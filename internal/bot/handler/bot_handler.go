package handler

import (
	"context"

	"github.com/central-university-dev/go-Matthew11K/internal/api/openapi/v1_bot"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
)

type linkUpdater interface {
	SendLinkUpdate(ctx context.Context, update *models.LinkUpdate) error
}

type BotHandler struct {
	linkUpdater linkUpdater
}

func NewBotHandler(linkUpdater linkUpdater) *BotHandler {
	return &BotHandler{
		linkUpdater: linkUpdater,
	}
}

func (h *BotHandler) UpdatesPost(ctx context.Context, req *v1_bot.LinkUpdate) (v1_bot.UpdatesPostRes, error) {
	update := &models.LinkUpdate{
		TgChatIDs: req.TgChatIds,
	}

	if !req.URL.IsSet() && !req.Description.IsSet() {
		errResp := &v1_bot.ApiErrorResponse{
			Description: v1_bot.NewOptString("Отсутствует обязательное поле URL или Description"),
		}

		return errResp, &errors.ErrMissingRequiredField{FieldName: "URL or Description"}
	}

	if !req.Description.IsSet() {
		errResp := &v1_bot.ApiErrorResponse{
			Description: v1_bot.NewOptString("Отсутствует обязательное поле Description"),
		}

		return errResp, &errors.ErrMissingRequiredField{FieldName: "Description"}
	}

	if req.ID.IsSet() {
		update.ID = req.ID.Value
	}

	if req.URL.IsSet() {
		update.URL = req.URL.Value.String()
	}

	update.Description = req.Description.Value

	if err := h.linkUpdater.SendLinkUpdate(ctx, update); err != nil {
		errResp := &v1_bot.ApiErrorResponse{
			Description: v1_bot.NewOptString("Ошибка при отправке обновления"),
		}

		return errResp, err
	}

	return &v1_bot.UpdatesPostOK{}, nil
}
