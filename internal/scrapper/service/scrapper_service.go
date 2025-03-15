package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/common"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
)

type BotClient interface {
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
}

type LinkRepository interface {
	Save(ctx context.Context, link *models.Link) error

	FindByID(ctx context.Context, id int64) (*models.Link, error)

	FindByURL(ctx context.Context, url string) (*models.Link, error)

	FindByChatID(ctx context.Context, chatID int64) ([]*models.Link, error)

	DeleteByURL(ctx context.Context, url string, chatID int64) error

	GetAll(ctx context.Context) ([]*models.Link, error)

	Update(ctx context.Context, link *models.Link) error

	AddChatLink(ctx context.Context, chatID int64, linkID int64) error
}

type ScrapperService struct {
	linkRepo       LinkRepository
	chatRepo       ChatRepository
	botClient      BotClient
	linkAnalyzer   *common.LinkAnalyzer
	updaterFactory *common.LinkUpdaterFactory
	logger         *slog.Logger
}

func NewScrapperService(
	linkRepo LinkRepository,
	chatRepo ChatRepository,
	botClient BotClient,
	updaterFactory *common.LinkUpdaterFactory,
	linkAnalyzer *common.LinkAnalyzer,
	logger *slog.Logger,
) *ScrapperService {
	return &ScrapperService{
		linkRepo:       linkRepo,
		chatRepo:       chatRepo,
		botClient:      botClient,
		linkAnalyzer:   linkAnalyzer,
		updaterFactory: updaterFactory,
		logger:         logger,
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

//nolint:funlen // Функция логически целостная и не требует разбиения на более мелкие части
func (s *ScrapperService) CheckUpdates(ctx context.Context) error {
	links, err := s.linkRepo.GetAll(ctx)
	if err != nil {
		return err
	}

	s.logger.Info("Проверка обновлений",
		"linksCount", len(links),
	)

	for _, link := range links {
		s.logger.Info("Проверка обновлений для ссылки",
			"url", link.URL,
			"id", link.ID,
			"type", link.Type,
		)

		updated, err := s.checkLinkUpdate(ctx, link)
		if err != nil {
			s.logger.Error("Ошибка при проверке обновлений для ссылки",
				"url", link.URL,
				"error", err,
			)

			continue
		}

		s.logger.Info("Результат проверки",
			"url", link.URL,
			"updated", updated,
		)

		if updated {
			chats, err := s.chatRepo.GetAll(ctx)
			if err != nil {
				s.logger.Error("Ошибка при получении списка чатов",
					"error", err,
				)

				continue
			}

			var chatIDs []int64

			for _, chat := range chats {
				for _, linkID := range chat.Links {
					if linkID == link.ID {
						chatIDs = append(chatIDs, chat.ID)
						break
					}
				}
			}

			s.logger.Info("Отправка уведомления об обновлении",
				"url", link.URL,
				"chatsCount", len(chatIDs),
				"chatIDs", chatIDs,
			)

			update := &models.LinkUpdate{
				ID:          link.ID,
				URL:         link.URL,
				Description: "Обнаружено обновление",
				TgChatIDs:   chatIDs,
			}

			err = s.botClient.SendUpdate(ctx, update)
			if err != nil {
				s.logger.Error("Ошибка при отправке уведомления об обновлении",
					"error", err,
				)
			} else {
				s.logger.Info("Уведомление об обновлении успешно отправлено")
			}
		}
	}

	return nil
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
