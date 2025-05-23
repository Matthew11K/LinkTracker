package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"

	"github.com/central-university-dev/go-Matthew11K/internal/bot/cache"
	"github.com/central-university-dev/go-Matthew11K/internal/bot/domain"
	commonservice "github.com/central-university-dev/go-Matthew11K/internal/common"
	domainerrors "github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
)

type ChatStateRepository interface {
	GetState(ctx context.Context, chatID int64) (models.ChatState, error)

	SetState(ctx context.Context, chatID int64, state models.ChatState) error

	GetData(ctx context.Context, chatID int64, key string) (any, error)

	SetData(ctx context.Context, chatID int64, key string, value any) error

	ClearData(ctx context.Context, chatID int64) error
}

type ScrapperClient interface {
	RegisterChat(ctx context.Context, chatID int64) error

	DeleteChat(ctx context.Context, chatID int64) error

	AddLink(ctx context.Context, chatID int64, url string, tags []string, filters []string) (*models.Link, error)

	RemoveLink(ctx context.Context, chatID int64, url string) (*models.Link, error)

	GetLinks(ctx context.Context, chatID int64) ([]*models.Link, error)

	UpdateNotificationSettings(ctx context.Context, chatID int64, mode models.NotificationMode, digestTime time.Time) error
}

type Transactor interface {
	WithTransaction(ctx context.Context, txFunc func(ctx context.Context) error) error
}

type BotService struct {
	chatStateRepo  ChatStateRepository
	scrapperClient ScrapperClient
	telegramClient domain.TelegramClientAPI
	linkAnalyzer   *commonservice.LinkAnalyzer
	txManager      Transactor
}

func NewBotService(
	chatStateRepo ChatStateRepository,
	scrapperClient ScrapperClient,
	telegramClient domain.TelegramClientAPI,
	linkAnalyzer *commonservice.LinkAnalyzer,
	txManager Transactor,
) *BotService {
	return &BotService{
		chatStateRepo:  chatStateRepo,
		scrapperClient: scrapperClient,
		telegramClient: telegramClient,
		linkAnalyzer:   linkAnalyzer,
		txManager:      txManager,
	}
}

func (s *BotService) WithCache(linkCache cache.LinkCache) *CachedBotService {
	return NewCachedBotService(s, linkCache, slog.Default())
}

func (s *BotService) ProcessCommand(ctx context.Context, command *models.Command) (string, error) {
	//nolint:exhaustive // CommandUnknown обрабатывается в блоке default
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
	case models.CommandMode:
		return s.handleModeCommand(ctx, command)
	case models.CommandTime:
		return s.handleTimeCommand(ctx, command)
	default:
		return "Неизвестная команда. Введите /help для просмотра доступных команд.",
			&domainerrors.ErrUnknownCommand{Command: string(command.Type)}
	}
}

func (s *BotService) ProcessMessage(ctx context.Context, chatID, _ int64, text, _ string) (string, error) {
	state, err := s.chatStateRepo.GetState(ctx, chatID)
	if err != nil {
		return "", err
	}

	switch state {
	case models.StateIdle:
		return "Введите команду или /help для просмотра доступных команд.", nil
	case models.StateAwaitingLink:
		return s.handleLinkInput(ctx, chatID, text)
	case models.StateAwaitingTags:
		return s.handleTagsInput(ctx, chatID, text)
	case models.StateAwaitingFilters:
		return s.handleFiltersInput(ctx, chatID, text)
	case models.StateAwaitingUntrackLink:
		return s.handleUntrackLinkInput(ctx, chatID, text)
	case models.StateAwaitingNotificationMode:
		return s.handleNotificationModeInput(ctx, chatID, text)
	case models.StateAwaitingDigestTime:
		return s.handleDigestTimeInput(ctx, chatID, text)
	default:
		return "", fmt.Errorf("неизвестное состояние чата: %d", state)
	}
}

func (s *BotService) SendLinkUpdate(ctx context.Context, update *models.LinkUpdate) error {
	return s.telegramClient.SendUpdate(ctx, update)
}

func (s *BotService) HandleUpdate(ctx context.Context, update *models.LinkUpdate) error {
	return s.SendLinkUpdate(ctx, update)
}

func (s *BotService) handleStartCommand(ctx context.Context, command *models.Command) (string, error) {
	err := s.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		if err := s.scrapperClient.RegisterChat(ctx, command.ChatID); err != nil {
			return err
		}

		if err := s.chatStateRepo.SetState(ctx, command.ChatID, models.StateIdle); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	return "Привет! Я бот для отслеживания изменений по ссылкам. Введите /help для просмотра доступных команд.", nil
}

func (s *BotService) handleHelpCommand(ctx context.Context, command *models.Command) (string, error) {
	err := s.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		return s.setStateWithEnsureChat(ctx, command.ChatID, models.StateIdle)
	})

	if err != nil {
		return "", err
	}

	return `Доступные команды:
/start - регистрация
/help - список команд
/track - начать отслеживание ссылки
/untrack - прекратить отслеживание ссылки
/list - показать список отслеживаемых ссылок
/mode - изменить режим уведомлений (мгновенный/дайджест)
/time - установить время доставки дайджеста`, nil
}

func (s *BotService) handleTrackCommand(ctx context.Context, command *models.Command) (string, error) {
	err := s.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		if err := s.setStateWithEnsureChat(ctx, command.ChatID, models.StateAwaitingLink); err != nil {
			return err
		}

		if err := s.clearDataWithEnsureChat(ctx, command.ChatID); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	return "Введите ссылку для отслеживания:", nil
}

func (s *BotService) handleUntrackCommand(ctx context.Context, command *models.Command) (string, error) {
	err := s.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		if err := s.setStateWithEnsureChat(ctx, command.ChatID, models.StateAwaitingUntrackLink); err != nil {
			return err
		}

		if err := s.clearDataWithEnsureChat(ctx, command.ChatID); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
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

		if len(link.Filters) > 0 {
			sb.WriteString(fmt.Sprintf("   Фильтры: %s\n", strings.Join(link.Filters, ", ")))
		}
	}

	return sb.String(), nil
}

func (s *BotService) handleLinkInput(ctx context.Context, chatID int64, text string) (string, error) {
	linkType := s.linkAnalyzer.AnalyzeLink(text)
	if linkType == models.Unknown {
		return "Неподдерживаемый тип ссылки. Пожалуйста, введите ссылку на GitHub репозиторий или вопрос StackOverflow:", nil
	}

	err := s.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		if err := s.setDataWithEnsureChat(ctx, chatID, "link", text); err != nil {
			return err
		}

		if err := s.setStateWithEnsureChat(ctx, chatID, models.StateAwaitingTags); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	return "Введите теги для ссылки (разделите пробелами) или просто напишите 'нет' для пропуска:", nil
}

func (s *BotService) handleTagsInput(ctx context.Context, chatID int64, text string) (string, error) {
	var tags []string

	if !strings.EqualFold(text, "нет") {
		tags = strings.Fields(text)
	}

	err := s.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		if err := s.setDataWithEnsureChat(ctx, chatID, "tags", tags); err != nil {
			return err
		}

		if err := s.setStateWithEnsureChat(ctx, chatID, models.StateAwaitingFilters); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	return "Введите фильтры для уведомлений (разделите пробелами) или просто напишите 'нет' для пропуска:", nil
}

func (s *BotService) handleFiltersInput(ctx context.Context, chatID int64, text string) (string, error) {
	var filters []string

	var link string

	var tags []string

	if !strings.EqualFold(text, "нет") {
		filters = strings.Fields(text)
	}

	err := s.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		linkInterface, err := s.chatStateRepo.GetData(ctx, chatID, "link")
		if err != nil {
			return err
		}

		linkStr, ok := linkInterface.(string)
		if !ok {
			return fmt.Errorf("некорректный тип данных для ссылки")
		}

		link = linkStr

		tagsInterface, err := s.chatStateRepo.GetData(ctx, chatID, "tags")
		if err != nil {
			return err
		}

		tagsSlice, ok := tagsInterface.([]string)
		if !ok {
			tagsSlice = []string{}
		}

		tags = tagsSlice

		if err := s.setStateWithEnsureChat(ctx, chatID, models.StateIdle); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	_, err = s.scrapperClient.AddLink(ctx, chatID, link, tags, filters)
	if err != nil {
		var linkExistsErr *domainerrors.ErrLinkAlreadyExists
		if errors.As(err, &linkExistsErr) {
			return "Эта ссылка уже отслеживается.", nil
		}

		return "", err
	}

	return "Ссылка успешно добавлена для отслеживания!", nil
}

func (s *BotService) handleUntrackLinkInput(ctx context.Context, chatID int64, text string) (string, error) {
	err := s.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		return s.setStateWithEnsureChat(ctx, chatID, models.StateIdle)
	})

	if err != nil {
		return "", err
	}

	_, err = s.scrapperClient.RemoveLink(ctx, chatID, text)
	if err != nil {
		var linkNotFoundErr *domainerrors.ErrLinkNotFound
		if errors.As(err, &linkNotFoundErr) {
			return "Указанная ссылка не отслеживается.", nil
		}

		return "", err
	}

	return "Отслеживание ссылки прекращено.", nil
}

func (s *BotService) handleModeCommand(ctx context.Context, command *models.Command) (string, error) {
	err := s.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		if err := s.setStateWithEnsureChat(ctx, command.ChatID, models.StateAwaitingNotificationMode); err != nil {
			return err
		}

		if err := s.clearDataWithEnsureChat(ctx, command.ChatID); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	return "Выберите режим уведомлений:\n1 - Мгновенные уведомления\n2 - Дайджест (раз в день)", nil
}

func (s *BotService) handleTimeCommand(ctx context.Context, command *models.Command) (string, error) {
	err := s.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		if err := s.setStateWithEnsureChat(ctx, command.ChatID, models.StateAwaitingDigestTime); err != nil {
			return err
		}

		if err := s.clearDataWithEnsureChat(ctx, command.ChatID); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	return "Введите время доставки дайджеста в формате ЧЧ:ММ (например, 09:00)", nil
}

func (s *BotService) handleNotificationModeInput(ctx context.Context, chatID int64, text string) (string, error) {
	var mode models.NotificationMode

	switch strings.TrimSpace(text) {
	case "1":
		mode = models.NotificationModeInstant
	case "2":
		mode = models.NotificationModeDigest
	default:
		return "Неверный ввод. Введите 1 для мгновенных уведомлений или 2 для дайджеста.", nil
	}

	var digestTime time.Time
	if mode == models.NotificationModeDigest {
		digestTime = time.Date(0, 1, 1, 10, 0, 0, 0, time.UTC)
	}

	if err := s.scrapperClient.UpdateNotificationSettings(ctx, chatID, mode, digestTime); err != nil {
		return "", fmt.Errorf("ошибка при обновлении режима уведомлений: %w", err)
	}

	err := s.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		return s.setStateWithEnsureChat(ctx, chatID, models.StateIdle)
	})

	if err != nil {
		return "", err
	}

	if mode == models.NotificationModeInstant {
		return "Режим мгновенных уведомлений успешно установлен.", nil
	}

	return "Режим дайджеста успешно установлен. Используйте команду /time для установки времени доставки.", nil
}

func (s *BotService) handleDigestTimeInput(ctx context.Context, chatID int64, text string) (string, error) {
	parts := strings.Split(strings.TrimSpace(text), ":")
	if len(parts) != 2 {
		return "Неверный формат времени. Используйте формат ЧЧ:ММ (например, 09:00).", nil
	}

	hour, errHour := strconv.Atoi(parts[0])
	minute, errMinute := strconv.Atoi(parts[1])

	if errHour != nil || errMinute != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return "Неверное время. Часы должны быть от 0 до 23, минуты от 0 до 59.", nil
	}

	digestTime := time.Date(0, 1, 1, hour, minute, 0, 0, time.UTC)

	if err := s.scrapperClient.UpdateNotificationSettings(ctx, chatID, models.NotificationModeDigest, digestTime); err != nil {
		return "", fmt.Errorf("ошибка при обновлении времени доставки дайджеста: %w", err)
	}

	err := s.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		return s.setStateWithEnsureChat(ctx, chatID, models.StateIdle)
	})

	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Время доставки дайджеста установлено на %02d:%02d.", hour, minute), nil
}

func (s *BotService) setStateWithEnsureChat(ctx context.Context, chatID int64, state models.ChatState) error {
	err := s.chatStateRepo.SetState(ctx, chatID, state)
	if isForeignKeyOrNotFoundErr(err) {
		if regErr := s.scrapperClient.RegisterChat(ctx, chatID); regErr != nil {
			return regErr
		}

		err = s.chatStateRepo.SetState(ctx, chatID, state)
	}

	return err
}

func (s *BotService) setDataWithEnsureChat(ctx context.Context, chatID int64, key string, value any) error {
	err := s.chatStateRepo.SetData(ctx, chatID, key, value)
	if isForeignKeyOrNotFoundErr(err) {
		if regErr := s.scrapperClient.RegisterChat(ctx, chatID); regErr != nil {
			return regErr
		}

		err = s.chatStateRepo.SetData(ctx, chatID, key, value)
	}

	return err
}

func (s *BotService) clearDataWithEnsureChat(ctx context.Context, chatID int64) error {
	err := s.chatStateRepo.ClearData(ctx, chatID)
	if isForeignKeyOrNotFoundErr(err) {
		if regErr := s.scrapperClient.RegisterChat(ctx, chatID); regErr != nil {
			return regErr
		}

		err = s.chatStateRepo.ClearData(ctx, chatID)
	}

	return err
}

func isForeignKeyOrNotFoundErr(err error) bool {
	if err == nil {
		return false
	}

	var sqlErr *domainerrors.ErrSQLExecution
	if errors.As(err, &sqlErr) {
		switch sqlErr.Operation {
		case domainerrors.OpSetChatState, domainerrors.OpSetChatStateData, domainerrors.OpClearChatStateData:
			return true
		}
	}

	return false
}
