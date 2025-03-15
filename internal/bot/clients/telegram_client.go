package clients

import (
	"context"
	"fmt"
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/central-university-dev/go-Matthew11K/internal/bot/domain"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
)

type TelegramClient struct {
	bot    *tgbotapi.BotAPI
	logger *slog.Logger
}

func NewTelegramClient(token string, logger *slog.Logger) domain.TelegramClientAPI {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		logger.Error("Ошибка при создании Telegram клиента", "error", err)
	}

	return &TelegramClient{
		bot:    bot,
		logger: logger,
	}
}

// SetBaseURL устанавливает базовый URL для API Telegram (используется в тестах).
func (c *TelegramClient) SetBaseURL(url string) {
	if c.bot != nil {
		c.bot.SetAPIEndpoint(url)
	}
}

func (c *TelegramClient) SendUpdate(_ context.Context, update interface{}) error {
	if c.bot == nil {
		return fmt.Errorf("telegram клиент не инициализирован")
	}

	linkUpdate, ok := update.(*models.LinkUpdate)
	if !ok {
		return fmt.Errorf("неправильный тип данных для обновления")
	}

	for _, chatID := range linkUpdate.TgChatIDs {
		message := fmt.Sprintf("🔔 *Обновление ссылки*\n\n🔗 [%s](%s)\n\n📝 %s", linkUpdate.URL, linkUpdate.URL, linkUpdate.Description)

		msg := tgbotapi.NewMessage(chatID, message)
		msg.ParseMode = tgbotapi.ModeMarkdown

		_, err := c.bot.Send(msg)
		if err != nil {
			return fmt.Errorf("ошибка при отправке обновления: %w", err)
		}
	}

	return nil
}

func (c *TelegramClient) SendMessage(_ context.Context, chatID int64, text string) error {
	if c.bot == nil {
		return fmt.Errorf("telegram клиент не инициализирован")
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML

	_, err := c.bot.Send(msg)
	if err != nil {
		return fmt.Errorf("ошибка при отправке сообщения: %w", err)
	}

	return nil
}

func (c *TelegramClient) GetUpdates(_ context.Context, offset int) ([]domain.Update, error) {
	if c.bot == nil {
		return nil, fmt.Errorf("telegram клиент не инициализирован")
	}

	updateConfig := tgbotapi.NewUpdate(offset)
	updateConfig.Timeout = 30

	updates, err := c.bot.GetUpdates(updateConfig)
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении обновлений: %w", err)
	}

	domainUpdates := make([]domain.Update, 0, len(updates))

	for _, update := range updates {
		var domainMessage *domain.Message
		if update.Message != nil {
			domainMessage = &domain.Message{
				MessageID: int64(update.Message.MessageID),
				Text:      update.Message.Text,
				Chat: domain.Chat{
					ID: update.Message.Chat.ID,
				},
				From: domain.User{
					ID:        update.Message.From.ID,
					Username:  update.Message.From.UserName,
					FirstName: update.Message.From.FirstName,
					LastName:  update.Message.From.LastName,
				},
			}
		}

		domainUpdates = append(domainUpdates, domain.Update{
			UpdateID: int64(update.UpdateID),
			Message:  domainMessage,
		})
	}

	return domainUpdates, nil
}

func (c *TelegramClient) SetMyCommands(_ context.Context, commands []domain.BotCommand) error {
	if c.bot == nil {
		return fmt.Errorf("telegram клиент не инициализирован")
	}

	botAPICommands := make([]tgbotapi.BotCommand, 0, len(commands))
	for _, cmd := range commands {
		botAPICommands = append(botAPICommands, tgbotapi.BotCommand{
			Command:     cmd.Command,
			Description: cmd.Description,
		})
	}

	setCommandsConfig := tgbotapi.NewSetMyCommands(botAPICommands...)

	_, err := c.bot.Request(setCommandsConfig)
	if err != nil {
		return fmt.Errorf("ошибка при установке команд бота: %w", err)
	}

	return nil
}

func (c *TelegramClient) GetBot() *tgbotapi.BotAPI {
	return c.bot
}
