package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/central-university-dev/go-Matthew11K/internal/bot/cache"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
)

type CachedBotService struct {
	botService *BotService
	linkCache  cache.LinkCache
	logger     *slog.Logger
}

func NewCachedBotService(botService *BotService, linkCache cache.LinkCache, logger *slog.Logger) *CachedBotService {
	return &CachedBotService{
		botService: botService,
		linkCache:  linkCache,
		logger:     logger,
	}
}

func (s *CachedBotService) ProcessCommand(ctx context.Context, command *models.Command) (string, error) {
	if command.Type == models.CommandTrack || command.Type == models.CommandUntrack {
		if err := s.linkCache.DeleteLinks(ctx, command.ChatID); err != nil {
			s.logger.Error("Ошибка при инвалидации кэша",
				"error", err,
				"chatID", command.ChatID,
			)
		}
	}

	if command.Type == models.CommandList {
		return s.handleCachedListCommand(ctx, command)
	}

	return s.botService.ProcessCommand(ctx, command)
}

func (s *CachedBotService) ProcessMessage(ctx context.Context, chatID, userID int64, text, username string) (string, error) {
	state, err := s.botService.chatStateRepo.GetState(ctx, chatID)
	if err != nil {
		return "", err
	}

	isLinkModification := (state == models.StateAwaitingFilters || state == models.StateAwaitingUntrackLink)

	response, err := s.botService.ProcessMessage(ctx, chatID, userID, text, username)
	if err != nil {
		return "", err
	}

	if isLinkModification {
		newState, stateErr := s.botService.chatStateRepo.GetState(ctx, chatID)
		if stateErr == nil && newState == models.StateIdle {
			if err := s.linkCache.DeleteLinks(ctx, chatID); err != nil {
				s.logger.Error("Ошибка при инвалидации кэша после модификации списка ссылок",
					"error", err,
					"chatID", chatID,
				)
			}
		}
	}

	return response, nil
}

func (s *CachedBotService) SendLinkUpdate(ctx context.Context, update *models.LinkUpdate) error {
	for _, chatID := range update.TgChatIDs {
		if err := s.linkCache.DeleteLinks(ctx, chatID); err != nil {
			s.logger.Error("Ошибка при инвалидации кэша при обновлении ссылки",
				"error", err,
				"chatID", chatID,
				"linkID", update.ID,
			)
		}
	}

	return s.botService.SendLinkUpdate(ctx, update)
}

func (s *CachedBotService) HandleUpdate(ctx context.Context, update *models.LinkUpdate) error {
	return s.SendLinkUpdate(ctx, update)
}

func (s *CachedBotService) handleCachedListCommand(ctx context.Context, command *models.Command) (string, error) {
	links, err := s.linkCache.GetLinks(ctx, command.ChatID)
	if err == nil && links != nil {
		s.logger.Info("Получены ссылки из кэша",
			"chatID", command.ChatID,
			"count", len(links),
		)

		return s.formatLinksList(links), nil
	}

	response, err := s.botService.handleListCommand(ctx, command)
	if err != nil {
		return "", err
	}

	links, err = s.botService.scrapperClient.GetLinks(ctx, command.ChatID)
	if err != nil {
		s.logger.Error("Ошибка при получении ссылок для кэширования",
			"error", err,
			"chatID", command.ChatID,
		)

		return response, nil
	}

	if err := s.linkCache.SetLinks(ctx, command.ChatID, links); err != nil {
		s.logger.Error("Ошибка при кэшировании ссылок",
			"error", err,
			"chatID", command.ChatID,
		)
	}

	return response, nil
}

func (s *CachedBotService) formatLinksList(links []*models.Link) string {
	if len(links) == 0 {
		return "У вас нет отслеживаемых ссылок."
	}

	var result strings.Builder

	result.WriteString("Список отслеживаемых ссылок:\n\n")

	for i, link := range links {
		result.WriteString(fmt.Sprintf("%d. %s\n", i+1, link.URL))

		if len(link.Tags) > 0 {
			result.WriteString(fmt.Sprintf("   Теги: %s\n", strings.Join(link.Tags, ", ")))
		}

		if len(link.Filters) > 0 {
			result.WriteString(fmt.Sprintf("   Фильтры: %s\n", strings.Join(link.Filters, ", ")))
		}
	}

	return result.String()
}
