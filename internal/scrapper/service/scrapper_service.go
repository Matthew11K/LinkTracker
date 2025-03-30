package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/common"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
)

type BotNotifier interface {
	SendUpdate(ctx context.Context, update *models.LinkUpdate) error
}

type ChatRepository interface {
	Save(ctx context.Context, chat *models.Chat) error

	FindByID(ctx context.Context, id int64) (*models.Chat, error)

	Delete(ctx context.Context, id int64) error

	Update(ctx context.Context, chat *models.Chat) error

	AddLink(ctx context.Context, chatID int64, linkID int64) error

	RemoveLink(ctx context.Context, chatID int64, linkID int64) error

	GetAll(ctx context.Context) ([]*models.Chat, error)

	FindByLinkID(ctx context.Context, linkID int64) ([]*models.Chat, error)
}

type LinkRepository interface {
	Save(ctx context.Context, link *models.Link) error

	FindByID(ctx context.Context, id int64) (*models.Link, error)

	FindByURL(ctx context.Context, url string) (*models.Link, error)

	FindByChatID(ctx context.Context, chatID int64) ([]*models.Link, error)

	DeleteByURL(ctx context.Context, url string, chatID int64) error

	Update(ctx context.Context, link *models.Link) error

	AddChatLink(ctx context.Context, chatID int64, linkID int64) error
}

type GitHubDetailsRepository interface {
	Save(ctx context.Context, details *models.GitHubDetails) error
	FindByLinkID(ctx context.Context, linkID int64) (*models.GitHubDetails, error)
	Update(ctx context.Context, details *models.GitHubDetails) error
}

type StackOverflowDetailsRepository interface {
	Save(ctx context.Context, details *models.StackOverflowDetails) error
	FindByLinkID(ctx context.Context, linkID int64) (*models.StackOverflowDetails, error)
	Update(ctx context.Context, details *models.StackOverflowDetails) error
}

type ScrapperService struct {
	linkRepo          LinkRepository
	chatRepo          ChatRepository
	botClient         BotNotifier
	githubRepo        GitHubDetailsRepository
	stackOverflowRepo StackOverflowDetailsRepository
	linkAnalyzer      *common.LinkAnalyzer
	updaterFactory    *common.LinkUpdaterFactory
	logger            *slog.Logger
}

func NewScrapperService(
	linkRepo LinkRepository,
	chatRepo ChatRepository,
	botClient BotNotifier,
	githubRepo GitHubDetailsRepository,
	stackOverflowRepo StackOverflowDetailsRepository,
	updaterFactory *common.LinkUpdaterFactory,
	linkAnalyzer *common.LinkAnalyzer,
	logger *slog.Logger,
) *ScrapperService {
	return &ScrapperService{
		linkRepo:          linkRepo,
		chatRepo:          chatRepo,
		botClient:         botClient,
		githubRepo:        githubRepo,
		stackOverflowRepo: stackOverflowRepo,
		linkAnalyzer:      linkAnalyzer,
		updaterFactory:    updaterFactory,
		logger:            logger,
	}
}

func (s *ScrapperService) RegisterChat(ctx context.Context, chatID int64) error {
	_, err := s.chatRepo.FindByID(ctx, chatID)
	if err == nil {
		return nil
	}

	chat := &models.Chat{
		ID:        chatID,
		Links:     []int64{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	return s.chatRepo.Save(ctx, chat)
}

func (s *ScrapperService) DeleteChat(ctx context.Context, chatID int64) error {
	chat, err := s.chatRepo.FindByID(ctx, chatID)
	if err != nil {
		return err
	}

	for _, linkID := range chat.Links {
		link, err := s.linkRepo.FindByID(ctx, linkID)
		if err != nil {
			continue
		}

		_ = s.linkRepo.DeleteByURL(ctx, link.URL, chatID)
	}

	return s.chatRepo.Delete(ctx, chatID)
}

func (s *ScrapperService) AddLink(ctx context.Context, chatID int64, url string, tags, filters []string) (*models.Link, error) {
	chat, err := s.chatRepo.FindByID(ctx, chatID)
	if err != nil {
		return nil, err
	}

	linkType := s.linkAnalyzer.AnalyzeLink(url)
	if linkType == models.Unknown {
		return nil, &errors.ErrUnsupportedLinkType{URL: url}
	}

	existingLink, err := s.linkRepo.FindByURL(ctx, url)
	if err == nil {
		for _, linkID := range chat.Links {
			if linkID == existingLink.ID {
				return nil, &errors.ErrLinkAlreadyExists{URL: url}
			}
		}

		if err := s.chatRepo.AddLink(ctx, chatID, existingLink.ID); err != nil {
			return nil, err
		}

		if err := s.linkRepo.AddChatLink(ctx, chatID, existingLink.ID); err != nil {
			return nil, err
		}

		return existingLink, nil
	}

	link := &models.Link{
		URL:         url,
		Type:        linkType,
		Tags:        tags,
		Filters:     filters,
		LastChecked: time.Now(),
		LastUpdated: time.Now(),
		CreatedAt:   time.Now(),
	}

	if err := s.linkRepo.Save(ctx, link); err != nil {
		return nil, err
	}

	if err := s.chatRepo.AddLink(ctx, chatID, link.ID); err != nil {
		return nil, err
	}

	if err := s.linkRepo.AddChatLink(ctx, chatID, link.ID); err != nil {
		return nil, err
	}

	return link, nil
}

func (s *ScrapperService) RemoveLink(ctx context.Context, chatID int64, url string) (*models.Link, error) {
	_, err := s.chatRepo.FindByID(ctx, chatID)
	if err != nil {
		return nil, err
	}

	link, err := s.linkRepo.FindByURL(ctx, url)
	if err != nil {
		return nil, err
	}

	if err := s.linkRepo.DeleteByURL(ctx, url, chatID); err != nil {
		return nil, err
	}

	return link, nil
}

func (s *ScrapperService) GetLinks(ctx context.Context, chatID int64) ([]*models.Link, error) {
	_, err := s.chatRepo.FindByID(ctx, chatID)
	if err != nil {
		return nil, err
	}

	return s.linkRepo.FindByChatID(ctx, chatID)
}

func (s *ScrapperService) ProcessLink(ctx context.Context, link *models.Link) (bool, error) {
	updated, err := s.checkLinkUpdate(ctx, link)
	if err != nil {
		// Ошибка уже залогирована в checkLinkUpdate
		return false, err // Возвращаем ошибку, чтобы планировщик знал о проблеме
	}

	if !updated {
		return false, nil // Обновлений нет
	}

	s.logger.Info("Обнаружено обновление ссылки, отправка уведомлений",
		"url", link.URL,
		"updatedAt", link.LastUpdated,
	)

	chats, err := s.chatRepo.FindByLinkID(ctx, link.ID)
	if err != nil {
		s.logger.Error("Ошибка при получении списка чатов для ссылки",
			"error", err,
			"linkID", link.ID,
		)
		return true, err
	}

	chatIDs := make([]int64, 0, len(chats))
	for _, chat := range chats {
		chatIDs = append(chatIDs, chat.ID)
	}

	if len(chatIDs) == 0 {
		s.logger.Info("Нет подписчиков для уведомления об обновлении ссылки", "linkID", link.ID)
		return true, nil
	}

	updater, err := s.updaterFactory.CreateUpdater(link.Type)
	if err != nil {
		s.logger.Error("Ошибка при создании updater",
			"error", err,
			"linkType", link.Type,
		)
		return true, err
	}

	var updateInfo *models.UpdateInfo
	updateInfo, err = updater.GetUpdateDetails(ctx, link.URL)
	if err != nil {
		s.logger.Error("Ошибка при получении деталей обновления",
			"error", err,
			"url", link.URL,
		)
	}

	if updateInfo != nil {
		s.saveDetailsToRepository(ctx, link, updateInfo)
	}

	var description string
	switch link.Type {
	case models.GitHub:
		description = "Обнаружено обновление GitHub репозитория"
	case models.StackOverflow:
		description = "Обнаружено обновление вопроса StackOverflow"
	default:
		description = "Обнаружено обновление ссылки"
	}

	update := &models.LinkUpdate{
		ID:          link.ID,
		URL:         link.URL,
		Description: description,
		TgChatIDs:   chatIDs,
		UpdateInfo:  updateInfo,
	}

	err = s.botClient.SendUpdate(ctx, update)
	if err != nil {
		s.logger.Error("Ошибка при отправке уведомления об обновлении",
			"error", err,
		)
		return true, err
	} else {
		s.logger.Info("Уведомление об обновлении успешно отправлено",
			"linkID", link.ID,
			"chatsCount", len(chatIDs),
		)
	}

	return true, nil
}

func (s *ScrapperService) saveDetailsToRepository(ctx context.Context, link *models.Link, info *models.UpdateInfo) {
	switch link.Type {
	case models.GitHub:
		details := &models.GitHubDetails{
			LinkID:      link.ID,
			Title:       info.Title,
			Author:      info.Author,
			UpdatedAt:   info.UpdatedAt,
			Description: info.TextPreview,
		}

		existingDetails, err := s.githubRepo.FindByLinkID(ctx, link.ID)
		if err == nil {
			details.LinkID = existingDetails.LinkID
			if err := s.githubRepo.Update(ctx, details); err != nil {
				s.logger.Error("Ошибка при обновлении деталей GitHub",
					"error", err,
					"linkID", link.ID,
				)
			}
		} else {
			if err := s.githubRepo.Save(ctx, details); err != nil {
				s.logger.Error("Ошибка при сохранении деталей GitHub",
					"error", err,
					"linkID", link.ID,
				)
			}
		}

	case models.StackOverflow:
		details := &models.StackOverflowDetails{
			LinkID:    link.ID,
			Title:     info.Title,
			Author:    info.Author,
			UpdatedAt: info.UpdatedAt,
			Content:   info.TextPreview,
		}

		existingDetails, err := s.stackOverflowRepo.FindByLinkID(ctx, link.ID)
		if err == nil {
			details.LinkID = existingDetails.LinkID
			if err := s.stackOverflowRepo.Update(ctx, details); err != nil {
				s.logger.Error("Ошибка при обновлении деталей StackOverflow",
					"error", err,
					"linkID", link.ID,
				)
			}
		} else {
			if err := s.stackOverflowRepo.Save(ctx, details); err != nil {
				s.logger.Error("Ошибка при сохранении деталей StackOverflow",
					"error", err,
					"linkID", link.ID,
				)
			}
		}
	}
}

func (s *ScrapperService) checkLinkUpdate(ctx context.Context, link *models.Link) (bool, error) {
	updater, err := s.updaterFactory.CreateUpdater(link.Type)
	if err != nil {
		return false, err
	}

	s.logger.Info("Проверка обновлений для ссылки",
		"url", link.URL,
		"type", link.Type,
	)

	lastUpdate, err := updater.GetLastUpdate(ctx, link.URL)
	if err != nil {
		s.logger.Error("Ошибка при запросе обновлений",
			"url", link.URL,
			"error", err,
		)

		return false, err
	}

	s.logger.Info("Время последнего обновления ресурса",
		"url", link.URL,
		"lastUpdate", lastUpdate,
		"savedLastUpdate", link.LastUpdated,
	)

	link.LastChecked = time.Now()

	if link.LastUpdated.IsZero() {
		s.logger.Info("Первая проверка ссылки",
			"url", link.URL,
		)

		link.LastUpdated = lastUpdate

		if err := s.linkRepo.Update(ctx, link); err != nil {
			return false, err
		}

		return true, nil
	}

	if lastUpdate.After(link.LastUpdated) {
		s.logger.Info("Обнаружено обновление",
			"url", link.URL,
			"newTime", lastUpdate,
			"oldTime", link.LastUpdated,
		)

		link.LastUpdated = lastUpdate

		if err := s.linkRepo.Update(ctx, link); err != nil {
			return false, err
		}

		return true, nil
	}

	s.logger.Info("Обновлений не обнаружено",
		"url", link.URL,
	)

	if err := s.linkRepo.Update(ctx, link); err != nil {
		return false, err
	}

	return false, nil
}
