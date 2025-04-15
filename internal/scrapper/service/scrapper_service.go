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

	GetAll(ctx context.Context) ([]*models.Link, error)
}

type ContentDetailsRepository interface {
	Save(ctx context.Context, details *models.ContentDetails) error
	FindByLinkID(ctx context.Context, linkID int64) (*models.ContentDetails, error)
	Update(ctx context.Context, details *models.ContentDetails) error
}

type Transactor interface {
	WithTransaction(ctx context.Context, txFunc func(ctx context.Context) error) error
}

type ScrapperService struct {
	linkRepo       LinkRepository
	chatRepo       ChatRepository
	botClient      BotNotifier
	detailsRepo    ContentDetailsRepository
	linkAnalyzer   *common.LinkAnalyzer
	updaterFactory *common.LinkUpdaterFactory
	logger         *slog.Logger
	txManager      Transactor
}

func NewScrapperService(
	linkRepo LinkRepository,
	chatRepo ChatRepository,
	botClient BotNotifier,
	detailsRepo ContentDetailsRepository,
	updaterFactory *common.LinkUpdaterFactory,
	linkAnalyzer *common.LinkAnalyzer,
	logger *slog.Logger,
	txManager Transactor,
) *ScrapperService {
	return &ScrapperService{
		linkRepo:       linkRepo,
		chatRepo:       chatRepo,
		botClient:      botClient,
		detailsRepo:    detailsRepo,
		linkAnalyzer:   linkAnalyzer,
		updaterFactory: updaterFactory,
		logger:         logger,
		txManager:      txManager,
	}
}

func (s *ScrapperService) RegisterChat(ctx context.Context, chatID int64) error {
	return s.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		chat := &models.Chat{
			ID:        chatID,
			Links:     []int64{},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		return s.chatRepo.Save(ctx, chat)
	})
}

func (s *ScrapperService) DeleteChat(ctx context.Context, chatID int64) error {
	return s.txManager.WithTransaction(ctx, func(ctx context.Context) error {
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
	})
}

func (s *ScrapperService) AddLink(ctx context.Context, chatID int64, url string, tags, filters []string) (*models.Link, error) {
	var result *models.Link

	err := s.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		chat, err := s.chatRepo.FindByID(ctx, chatID)
		if err != nil {
			return err
		}

		linkType := s.linkAnalyzer.AnalyzeLink(url)
		if linkType == models.Unknown {
			return &errors.ErrUnsupportedLinkType{URL: url}
		}

		existingLink, err := s.linkRepo.FindByURL(ctx, url)
		if err == nil {
			for _, linkID := range chat.Links {
				if linkID == existingLink.ID {
					return &errors.ErrLinkAlreadyExists{URL: url}
				}
			}

			if err := s.chatRepo.AddLink(ctx, chatID, existingLink.ID); err != nil {
				return err
			}

			if err := s.linkRepo.AddChatLink(ctx, chatID, existingLink.ID); err != nil {
				return err
			}

			result = existingLink

			return nil
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
			return err
		}

		if err := s.chatRepo.AddLink(ctx, chatID, link.ID); err != nil {
			return err
		}

		if err := s.linkRepo.AddChatLink(ctx, chatID, link.ID); err != nil {
			return err
		}

		result = link

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *ScrapperService) RemoveLink(ctx context.Context, chatID int64, url string) (*models.Link, error) {
	var result *models.Link

	err := s.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		_, err := s.chatRepo.FindByID(ctx, chatID)
		if err != nil {
			return err
		}

		link, err := s.linkRepo.FindByURL(ctx, url)
		if err != nil {
			return err
		}

		if err := s.linkRepo.DeleteByURL(ctx, url, chatID); err != nil {
			return err
		}

		result = link

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *ScrapperService) GetLinks(ctx context.Context, chatID int64) ([]*models.Link, error) {
	var result []*models.Link

	err := s.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		_, err := s.chatRepo.FindByID(ctx, chatID)
		if err != nil {
			return err
		}

		links, err := s.linkRepo.FindByChatID(ctx, chatID)
		if err != nil {
			return err
		}

		result = links

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

//nolint:funlen // Функция целостная
func (s *ScrapperService) CheckUpdates(ctx context.Context) error {
	links, err := s.linkRepo.GetAll(ctx)
	if err != nil {
		return err
	}

	s.logger.Info("Начало проверки обновлений ссылок",
		"count", len(links),
	)

	for _, link := range links {
		func() {
			s.logger.Info("Проверка ссылки",
				"linkId", link.ID,
				"url", link.URL,
			)

			isUpdated, err := s.checkLinkUpdate(ctx, link)
			if err != nil {
				s.logger.Error("Ошибка при проверке обновлений",
					"error", err,
				)

				return
			}

			if !isUpdated {
				s.logger.Info("Нет обновлений для ссылки",
					"linkId", link.ID,
					"url", link.URL,
				)

				return
			}

			s.logger.Info("Найдены обновления для ссылки",
				"linkId", link.ID,
				"url", link.URL,
			)

			chats, err := s.chatRepo.FindByLinkID(ctx, link.ID)
			if err != nil {
				s.logger.Error("Ошибка при получении чатов для ссылки",
					"error", err,
					"linkId", link.ID,
				)

				return
			}

			if len(chats) == 0 {
				s.logger.Info("Нет чатов для уведомления",
					"linkId", link.ID,
				)

				return
			}

			var chatIDs []int64

			for _, chat := range chats {
				chatIDs = append(chatIDs, chat.ID)
			}

			_, err = s.notifyChatsAboutUpdate(ctx, link, chatIDs)
			if err != nil {
				s.logger.Error("Ошибка при отправке уведомлений",
					"error", err,
					"linkId", link.ID,
				)
			}
		}()
	}

	return nil
}

func (s *ScrapperService) notifyChatsAboutUpdate(ctx context.Context, link *models.Link, chatIDs []int64) (bool, error) {
	s.logger.Info("Отправка уведомления об обновлении",
		"linkId", link.ID,
		"chatsCount", len(chatIDs),
	)

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
		err = s.saveDetailsToRepository(ctx, link, updateInfo)
		if err != nil {
			return true, err
		}
	}

	var description string
	//nolint:exhaustive // Unknown обрабатывается в блоке default
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
	}

	s.logger.Info("Уведомление об обновлении успешно отправлено",
		"linkID", link.ID,
		"chatsCount", len(chatIDs),
	)

	return true, nil
}

func (s *ScrapperService) saveDetailsToRepository(ctx context.Context, link *models.Link, info *models.UpdateInfo) error {
	return s.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		details := &models.ContentDetails{
			LinkID:      link.ID,
			Title:       info.Title,
			Author:      info.Author,
			UpdatedAt:   info.UpdatedAt,
			ContentText: info.FullText,
			LinkType:    link.Type,
		}

		if err := s.detailsRepo.Save(ctx, details); err != nil {
			s.logger.Error("Ошибка при сохранении деталей контента",
				"error", err,
				"linkID", link.ID,
				"type", link.Type,
			)

			return err
		}

		return nil
	})
}

//nolint:funlen // Функция целостная
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

		err := s.txManager.WithTransaction(ctx, func(ctx context.Context) error {
			return s.linkRepo.Update(ctx, link)
		})
		if err != nil {
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

		err := s.txManager.WithTransaction(ctx, func(ctx context.Context) error {
			return s.linkRepo.Update(ctx, link)
		})
		if err != nil {
			return false, err
		}

		return true, nil
	}

	s.logger.Info("Обновлений не обнаружено",
		"url", link.URL,
	)

	err = s.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		return s.linkRepo.Update(ctx, link)
	})
	if err != nil {
		return false, err
	}

	return false, nil
}

func (s *ScrapperService) ProcessLink(ctx context.Context, link *models.Link) (bool, error) {
	updated, err := s.checkLinkUpdate(ctx, link)
	if err != nil {
		return false, err
	}

	if !updated {
		return false, nil
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

	return s.notifyChatsAboutUpdate(ctx, link, chatIDs)
}
