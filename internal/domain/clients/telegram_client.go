package clients

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramClient interface {
	SendMessage(ctx context.Context, chatID int64, text string) error

	GetUpdates(ctx context.Context, offset int) ([]Update, error)

	SendUpdate(ctx context.Context, update interface{}) error

	SetMyCommands(ctx context.Context, commands []BotCommand) error

	GetBot() *tgbotapi.BotAPI
}

type BotCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message,omitempty"`
}

type Message struct {
	MessageID int64  `json:"message_id"`
	Text      string `json:"text"`
	Chat      Chat   `json:"chat"`
	From      User   `json:"from"`
}

type Chat struct {
	ID int64 `json:"id"`
}

type User struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}
