package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/clients"
	domainerrors "github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/repositories"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/services"
)

type ScrapperClient interface {
	RegisterChat(ctx context.Context, chatID int64) error

	DeleteChat(ctx context.Context, chatID int64) error

	AddLink(ctx context.Context, chatID int64, url string, tags []string, filters []string) (*models.Link, error)

	RemoveLink(ctx context.Context, chatID int64, url string) (*models.Link, error)

	GetLinks(ctx context.Context, chatID int64) ([]*models.Link, error)
}

type BotService struct {
	chatStateRepo  repositories.ChatStateRepository
	scrapperClient ScrapperClient
	telegramClient clients.TelegramClient
	linkAnalyzer   *services.LinkAnalyzer
}

func NewBotService(
	chatStateRepo repositories.ChatStateRepository,
	scrapperClient ScrapperClient,
	telegramClient clients.TelegramClient,
	linkAnalyzer *services.LinkAnalyzer,
) *BotService {
	return &BotService{
		chatStateRepo:  chatStateRepo,
		scrapperClient: scrapperClient,
		telegramClient: telegramClient,
		linkAnalyzer:   linkAnalyzer,
	}
}

func (s *BotService) ProcessCommand(ctx context.Context, command *models.Command) (string, error) {
	switch command.Type {
	case models.CommandStart:
		return s.handleStartCommand(ctx, command)
	case models.CommandHelp:
		return s.handleHelpCommand(ctx, command)
	case models.CommandTrack:
		return s.handleTrackCommand(ctx, command)
	case models.CommandUntrack:
		return s.handleUntrackCommand(ctx, command)
	case models.CommandList:
		return s.handleListCommand(ctx, command)
	case models.CommandUnknown:
		return "Неизвестная команда. Введите /help для просмотра доступных команд.", &domainerrors.ErrUnknownCommand{Command: string(command.Type)}
	default:
		return "Неизвестная команда. Введите /help для просмотра доступных команд.", &domainerrors.ErrUnknownCommand{Command: string(command.Type)}
	}
}

func (s *BotService) ProcessMessage(ctx context.Context, chatID, userID int64, text, username string) (string, error) {
	state, err := s.chatStateRepo.GetState(ctx, chatID)
	if err != nil {
		return "", err
	}

	switch state {
	case models.StateIdle:
		if strings.HasPrefix(text, "/") {
			command := &models.Command{
				ChatID:   chatID,
				UserID:   userID,
				Text:     text,
				Username: username,
			}

			command.Type = getCommandType(text)

			return s.ProcessCommand(ctx, command)
		}

		return "Введите команду или /help для просмотра доступных команд.", nil
	case models.StateAwaitingLink:
		return s.handleLinkInput(ctx, chatID, text)
	case models.StateAwaitingTags:
		return s.handleTagsInput(ctx, chatID, text)
	case models.StateAwaitingFilters:
		return s.handleFiltersInput(ctx, chatID, text)
	case models.StateAwaitingUntrackLink:
		return s.handleUntrackLinkInput(ctx, chatID, text)
	default:
		return "", fmt.Errorf("неизвестное состояние чата: %d", state)
	}
}

func (s *BotService) SendLinkUpdate(ctx context.Context, update *models.LinkUpdate) error {
	for _, chatID := range update.TgChatIDs {
		message := fmt.Sprintf("🔔 *Обновление ссылки*\n\n🔗 [%s](%s)\n\n📝 %s", update.URL, update.URL, update.Description)
		if err := s.telegramClient.SendMessage(ctx, chatID, message); err != nil {
			return err
		}
	}

	return nil
}

func (s *BotService) handleStartCommand(ctx context.Context, command *models.Command) (string, error) {
	if err := s.scrapperClient.RegisterChat(ctx, command.ChatID); err != nil {
		return "", err
	}

	if err := s.chatStateRepo.SetState(ctx, command.ChatID, models.StateIdle); err != nil {
		return "", err
	}

	return "Привет! Я бот для отслеживания изменений по ссылкам. Введите /help для просмотра доступных команд.", nil
}

func (s *BotService) handleHelpCommand(ctx context.Context, command *models.Command) (string, error) {
	if err := s.chatStateRepo.SetState(ctx, command.ChatID, models.StateIdle); err != nil {
		return "", err
	}

	return `Доступные команды:
/start - регистрация
/help - список команд
/track - начать отслеживание ссылки
/untrack - прекратить отслеживание ссылки
/list - показать список отслеживаемых ссылок`, nil
}

func (s *BotService) handleTrackCommand(ctx context.Context, command *models.Command) (string, error) {
	if err := s.chatStateRepo.SetState(ctx, command.ChatID, models.StateAwaitingLink); err != nil {
		return "", err
	}

	if err := s.chatStateRepo.ClearData(ctx, command.ChatID); err != nil {
		return "", err
	}

	return "Введите ссылку для отслеживания:", nil
}

func (s *BotService) handleUntrackCommand(ctx context.Context, command *models.Command) (string, error) {
	if err := s.chatStateRepo.SetState(ctx, command.ChatID, models.StateAwaitingUntrackLink); err != nil {
		return "", err
	}

	if err := s.chatStateRepo.ClearData(ctx, command.ChatID); err != nil {
		return "", err
	}

	return "Введите ссылку для прекращения отслеживания:", nil
}

func (s *BotService) handleListCommand(ctx context.Context, command *models.Command) (string, error) {
	links, err := s.scrapperClient.GetLinks(ctx, command.ChatID)
	if err != nil {
		return "", err
	}

	if len(links) == 0 {
		return "У вас нет отслеживаемых ссылок.", nil
	}

	var sb strings.Builder

	sb.WriteString("Список отслеживаемых ссылок:\n\n")

	for i, link := range links {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, link.URL))

		if len(link.Tags) > 0 {
			sb.WriteString(fmt.Sprintf("   Теги: %s\n", strings.Join(link.Tags, ", ")))
		}
	}

	return sb.String(), nil
}

func (s *BotService) handleLinkInput(ctx context.Context, chatID int64, text string) (string, error) {
	linkType := s.linkAnalyzer.AnalyzeLink(text)
	if linkType == models.Unknown {
		return "Неподдерживаемый тип ссылки. Пожалуйста, введите ссылку на GitHub репозиторий или вопрос StackOverflow:", nil
	}

	if err := s.chatStateRepo.SetData(ctx, chatID, "link", text); err != nil {
		return "", err
	}

	if err := s.chatStateRepo.SetState(ctx, chatID, models.StateAwaitingTags); err != nil {
		return "", err
	}

	return "Введите теги для ссылки (разделите пробелами) или просто напишите 'нет' для пропуска:", nil
}

func (s *BotService) handleTagsInput(ctx context.Context, chatID int64, text string) (string, error) {
	var tags []string

	if !strings.EqualFold(text, "нет") {
		tags = strings.Fields(text)
	}

	if err := s.chatStateRepo.SetData(ctx, chatID, "tags", tags); err != nil {
		return "", err
	}

	if err := s.chatStateRepo.SetState(ctx, chatID, models.StateAwaitingFilters); err != nil {
		return "", err
	}

	return "Введите фильтры для уведомлений (разделите пробелами) или просто напишите 'нет' для пропуска:", nil
}

func (s *BotService) handleFiltersInput(ctx context.Context, chatID int64, text string) (string, error) {
	var filters []string

	if !strings.EqualFold(text, "нет") {
		filters = strings.Fields(text)
	}

	linkInterface, err := s.chatStateRepo.GetData(ctx, chatID, "link")
	if err != nil {
		return "", err
	}

	link, ok := linkInterface.(string)
	if !ok {
		return "", fmt.Errorf("некорректный тип данных для ссылки")
	}

	tagsInterface, err := s.chatStateRepo.GetData(ctx, chatID, "tags")
	if err != nil {
		return "", err
	}

	tags, ok := tagsInterface.([]string)
	if !ok {
		tags = []string{}
	}

	_, err = s.scrapperClient.AddLink(ctx, chatID, link, tags, filters)
	if err != nil {
		var linkExistsErr *domainerrors.ErrLinkAlreadyExists
		if errors.As(err, &linkExistsErr) {
			return "Эта ссылка уже отслеживается.", nil
		}

		return "", err
	}

	if err := s.chatStateRepo.SetState(ctx, chatID, models.StateIdle); err != nil {
		return "", err
	}

	if err := s.chatStateRepo.ClearData(ctx, chatID); err != nil {
		return "", err
	}

	return fmt.Sprintf("Ссылка %s добавлена для отслеживания!", link), nil
}

func (s *BotService) handleUntrackLinkInput(ctx context.Context, chatID int64, text string) (string, error) {
	_, err := s.scrapperClient.RemoveLink(ctx, chatID, text)
	if err != nil {
		var linkNotFoundErr *domainerrors.ErrLinkNotFound
		if errors.As(err, &linkNotFoundErr) {
			return "Ссылка не найдена или не отслеживается.", nil
		}

		return "", err
	}

	if err := s.chatStateRepo.SetState(ctx, chatID, models.StateIdle); err != nil {
		return "", err
	}

	return fmt.Sprintf("Прекращено отслеживание ссылки %s.", text), nil
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
