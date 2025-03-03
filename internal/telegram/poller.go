package telegram

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/clients"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
)

type BotService interface {
	ProcessCommand(ctx context.Context, command *models.Command) (string, error)

	ProcessMessage(ctx context.Context, chatID int64, userID int64, text string, username string) (string, error)

	SendLinkUpdate(ctx context.Context, update *models.LinkUpdate) error
}

type Poller struct {
	telegramClient clients.TelegramClient
	botService     BotService
	logger         *slog.Logger
	offset         int
	stopped        bool
}

func NewPoller(telegramClient clients.TelegramClient, botService BotService, logger *slog.Logger) *Poller {
	return &Poller{
		telegramClient: telegramClient,
		botService:     botService,
		logger:         logger,
		offset:         0,
		stopped:        false,
	}
}

func (p *Poller) Start() {
	p.logger.Info("Запуск Telegram поллера")
	p.stopped = false

	for !p.stopped {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		updates, err := p.telegramClient.GetUpdates(ctx, p.offset)

		cancel()

		if err != nil {
			p.logger.Error("Ошибка при получении обновлений", "error", err)
			time.Sleep(5 * time.Second)

			continue
		}

		for _, update := range updates {
			p.processUpdate(update)
			p.offset = int(update.UpdateID) + 1
		}

		time.Sleep(1 * time.Second)
	}
}

func (p *Poller) Stop() {
	p.logger.Info("Остановка Telegram поллера")
	p.stopped = true
}

func (p *Poller) processUpdate(update clients.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID
	text := update.Message.Text
	username := update.Message.From.Username

	p.logger.Info("Получено сообщение",
		"chat_id", chatID,
		"user_id", userID,
		"text", text,
		"username", username,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var response string

	var err error

	if strings.HasPrefix(text, "/") {
		command := &models.Command{
			ChatID:   chatID,
			UserID:   userID,
			Text:     text,
			Username: username,
		}

		command.Type = getCommandType(text)

		response, err = p.botService.ProcessCommand(ctx, command)
	} else {
		response, err = p.botService.ProcessMessage(ctx, chatID, userID, text, username)
	}

	if err != nil {
		p.logger.Error("Ошибка при обработке сообщения",
			"error", err,
			"chat_id", chatID,
			"text", text,
		)

		response = "Произошла ошибка при обработке вашего сообщения. Пожалуйста, попробуйте позже."
	}

	if response != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := p.telegramClient.SendMessage(ctx, chatID, response); err != nil {
			p.logger.Error("Ошибка при отправке ответа",
				"error", err,
				"chat_id", chatID,
				"text", response,
			)
		}
	}
}

func getCommandType(text string) models.CommandType {
	command := strings.Split(text, " ")[0]
	switch command {
	case "/start":
		return models.CommandStart
	case "/help":
		return models.CommandHelp
	case "/track":
		return models.CommandTrack
	case "/untrack":
		return models.CommandUntrack
	case "/list":
		return models.CommandList
	default:
		return models.CommandUnknown
	}
}
