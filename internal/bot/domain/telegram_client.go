package domain

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramClientAPI interface {
	SendMessage(ctx context.Context, chatID int64, text string) error

	GetUpdates(ctx context.Context, offset int) ([]Update, error)

	SendUpdate(ctx context.Context, update any) error

	SetMyCommands(ctx context.Context, commands []BotCommand) error

	GetBot() *tgbotapi.BotAPI
}
