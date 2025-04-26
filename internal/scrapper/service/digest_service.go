package service

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/config"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
	"github.com/central-university-dev/go-Matthew11K/internal/scrapper/notify"
	"github.com/central-university-dev/go-Matthew11K/internal/scrapper/repository"
	"github.com/go-co-op/gocron"
)

type DigestCache interface {
	AddUpdate(ctx context.Context, chatID int64, update *models.LinkUpdate) error
	GetUpdates(ctx context.Context, chatID int64) ([]*models.LinkUpdate, error)
	ClearUpdates(ctx context.Context, chatID int64) error
	GetAllChatIDs(ctx context.Context) ([]int64, error)
	Close() error
}

type DigestService struct {
	config        *config.Config
	notifier      notify.BotNotifier
	digestCache   DigestCache
	chatRepo      repository.ChatRepository
	logger        *slog.Logger
	scheduler     *gocron.Scheduler
	schedulerDone chan struct{}
}

func NewDigestService(
	config *config.Config,
	notifier notify.BotNotifier,
	digestCache DigestCache,
	chatRepo repository.ChatRepository,
	logger *slog.Logger,
) *DigestService {
	return &DigestService{
		config:        config,
		notifier:      notifier,
		digestCache:   digestCache,
		chatRepo:      chatRepo,
		logger:        logger,
		scheduler:     gocron.NewScheduler(time.UTC),
		schedulerDone: make(chan struct{}),
	}
}

func (s *DigestService) Start(ctx context.Context) {
	s.logger.Info("Запуск планировщика дайджестов")

	_, err := s.scheduler.Every(1).Minute().Do(func() {
		now := time.Now()
		hour, minute := now.Hour(), now.Minute()

		if err := s.sendDigests(ctx, hour, minute); err != nil {
			s.logger.Error("Ошибка при отправке дайджестов",
				"error", err,
				"time", fmt.Sprintf("%02d:%02d", hour, minute),
			)
		}
	})

	if err != nil {
		s.logger.Error("Ошибка при настройке планировщика дайджестов",
			"error", err,
		)

		return
	}

	s.scheduler.StartAsync()
}

func (s *DigestService) Stop() {
	s.logger.Info("Остановка планировщика дайджестов")
	s.scheduler.Stop()
	close(s.schedulerDone)
}

func (s *DigestService) AddUpdate(ctx context.Context, update *models.LinkUpdate) error {
	s.logger.Debug("Обработка обновления для дайджеста",
		"totalChats", len(update.TgChatIDs),
	)

	for _, chatID := range update.TgChatIDs {
		chat, err := s.chatRepo.FindByID(ctx, chatID)
		if err != nil {
			s.logger.Error("Ошибка при получении чата",
				"error", err,
				"chatID", chatID,
			)

			continue
		}

		if chat.NotificationMode == models.NotificationModeDigest {
			chatUpdate := &models.LinkUpdate{
				ID:          update.ID,
				URL:         update.URL,
				Description: update.Description,
				TgChatIDs:   []int64{chatID},
				UpdateInfo:  update.UpdateInfo,
			}

			if err := s.digestCache.AddUpdate(ctx, chatID, chatUpdate); err != nil {
				s.logger.Error("Ошибка при добавлении обновления в дайджест",
					"error", err,
					"chatID", chatID,
				)

				continue
			}

			s.logger.Info("Обновление добавлено в дайджест",
				"chatID", chatID,
				"url", update.URL,
			)
		}
	}

	return nil
}

func (s *DigestService) sendDigests(ctx context.Context, hour, minute int) error {
	chats, err := s.chatRepo.FindByDigestTime(ctx, hour, minute)
	if err != nil {
		return fmt.Errorf("ошибка при поиске чатов с временем дайджеста %02d:%02d: %w", hour, minute, err)
	}

	if len(chats) == 0 {
		return nil
	}

	s.logger.Info("Отправка дайджестов",
		"time", fmt.Sprintf("%02d:%02d", hour, minute),
		"totalChats", len(chats),
	)

	for _, chat := range chats {
		updates, err := s.digestCache.GetUpdates(ctx, chat.ID)
		if err != nil {
			s.logger.Error("Ошибка при получении обновлений для дайджеста",
				"error", err,
				"chatID", chat.ID,
			)

			continue
		}

		if len(updates) == 0 {
			s.logger.Debug("Нет обновлений для дайджеста",
				"chatID", chat.ID,
			)

			continue
		}

		message := s.createDigestMessage(updates)

		digestUpdate := &models.LinkUpdate{
			Description: message,
			TgChatIDs:   []int64{chat.ID},
		}

		if err := s.notifier.SendUpdate(ctx, digestUpdate); err != nil {
			s.logger.Error("Ошибка при отправке дайджеста",
				"error", err,
				"chatID", chat.ID,
			)

			continue
		}

		if err := s.digestCache.ClearUpdates(ctx, chat.ID); err != nil {
			s.logger.Error("Ошибка при очистке обновлений после отправки дайджеста",
				"error", err,
				"chatID", chat.ID,
			)
		}

		s.logger.Info("Дайджест успешно отправлен",
			"chatID", chat.ID,
			"updates", len(updates),
		)
	}

	return nil
}

func (s *DigestService) createDigestMessage(updates []*models.LinkUpdate) string {
	dateFormat := "02.01.2006"

	var message strings.Builder

	message.WriteString(fmt.Sprintf("📋 *Дайджест обновлений за %s*\n\n", time.Now().Format(dateFormat)))

	count := 0
	maxUpdates := 10

	for _, update := range updates {
		if count >= maxUpdates {
			message.WriteString(fmt.Sprintf("\n...и еще %d обновлений", len(updates)-maxUpdates))
			break
		}

		message.WriteString(fmt.Sprintf("🔔 *Обновление*: [%s](%s)\n", update.URL, update.URL))

		if update.UpdateInfo != nil {
			info := update.UpdateInfo
			message.WriteString(fmt.Sprintf("📄 *%s* от *%s*\n",
				info.Title,
				info.Author,
			))
			message.WriteString(fmt.Sprintf("⏱️ %s\n", info.UpdatedAt.Format("02.01.2006 15:04")))

			preview := info.TextPreview
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}

			message.WriteString(fmt.Sprintf("📝 %s\n", preview))
		} else if update.Description != "" {
			message.WriteString(fmt.Sprintf("%s\n", update.Description))
		}

		message.WriteString("\n")

		count++
	}

	return message.String()
}

func (s *DigestService) GetDigestChatsByTime(ctx context.Context, hourStr, minuteStr string) ([]*models.Chat, error) {
	hour, err := strconv.Atoi(hourStr)
	if err != nil {
		return nil, fmt.Errorf("некорректный формат часа: %w", err)
	}

	minute, err := strconv.Atoi(minuteStr)
	if err != nil {
		return nil, fmt.Errorf("некорректный формат минуты: %w", err)
	}

	return s.chatRepo.FindByDigestTime(ctx, hour, minute)
}
