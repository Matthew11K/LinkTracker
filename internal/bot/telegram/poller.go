package telegram

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/bot/domain"
	"github.com/central-university-dev/go-Matthew11K/internal/common/metrics"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
)

type BotService interface {
	ProcessCommand(ctx context.Context, command *models.Command) (string, error)

	ProcessMessage(ctx context.Context, chatID int64, userID int64, text string, username string) (string, error)

	SendLinkUpdate(ctx context.Context, update *models.LinkUpdate) error
}

type Poller struct {
	telegramClient domain.TelegramClientAPI
	botService     BotService
	logger         *slog.Logger
	stopped        bool
	updatesChan    tgbotapi.UpdatesChannel
	stopChan       chan struct{}
}

func NewPoller(telegramClient domain.TelegramClientAPI, botService BotService, logger *slog.Logger) *Poller {
	return &Poller{
		telegramClient: telegramClient,
		botService:     botService,
		logger:         logger,
		stopped:        true,
		stopChan:       make(chan struct{}),
	}
}

func (p *Poller) Start() {
	p.logger.Info("Запуск Telegram поллера")

	bot := p.telegramClient.GetBot()
	if bot == nil {
		p.logger.Error("Не удалось получить доступ к API бота")
		return
	}

	p.stopped = false

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	p.updatesChan = bot.GetUpdatesChan(u)

	go func() {
		for {
			select {
			case <-p.stopChan:
				p.logger.Info("Получен сигнал остановки поллера")
				return
			case update := <-p.updatesChan:
				p.processUpdate(&update)
			}
		}
	}()
}

func (p *Poller) Stop() {
	p.logger.Info("Остановка Telegram поллера")
	p.stopped = true
	close(p.stopChan)
}

func (p *Poller) Close() error {
	p.logger.Info("Остановка Telegram поллера")
	p.stopped = true
	close(p.stopChan)

	return nil
}

func (p *Poller) processUpdate(update *tgbotapi.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID
	text := update.Message.Text
	username := update.Message.From.UserName

	p.logger.Info("Получено сообщение",
		"chat_id", chatID,
		"user_id", userID,
		"text", text,
		"username", username,
	)

	messageType := "message"
	if update.Message.IsCommand() {
		messageType = "command"
	}

	metrics.RecordUserMessage(strconv.FormatInt(chatID, 10), messageType)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var response string

	var err error

	if update.Message.IsCommand() {
		commandName := "/" + update.Message.Command()

		command := &models.Command{
			ChatID:   chatID,
			UserID:   userID,
			Text:     text,
			Username: username,
			Type:     getCommandType(commandName),
		}

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

func getCommandType(commandName string) models.CommandType {
	switch commandName {
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
	case "/mode":
		return models.CommandMode
	case "/time":
		return models.CommandTime
	default:
		return models.CommandUnknown
	}
}
