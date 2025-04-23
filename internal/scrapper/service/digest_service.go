package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/config"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
	"github.com/central-university-dev/go-Matthew11K/internal/scrapper/notify"
)

type DigestService struct {
	config        *config.Config
	notifier      notify.BotNotifier
	logger        *slog.Logger
	hour          int
	minute        int
	updates       map[int64][]*models.LinkUpdate
	updatesMutex  sync.RWMutex
	schedulerDone chan struct{}
}

func NewDigestService(
	config *config.Config,
	notifier notify.BotNotifier,
	logger *slog.Logger,
	hour, minute int,
) *DigestService {
	return &DigestService{
		config:        config,
		notifier:      notifier,
		logger:        logger,
		hour:          hour,
		minute:        minute,
		updates:       make(map[int64][]*models.LinkUpdate),
		updatesMutex:  sync.RWMutex{},
		schedulerDone: make(chan struct{}),
	}
}

func (s *DigestService) Start(ctx context.Context) {
	go s.runScheduler(ctx)
}

func (s *DigestService) Stop() {
	close(s.schedulerDone)
}

func (s *DigestService) AddUpdate(update *models.LinkUpdate) {
	s.updatesMutex.Lock()
	defer s.updatesMutex.Unlock()

	for _, chatID := range update.TgChatIDs {
		s.updates[chatID] = append(s.updates[chatID], update)
	}

	s.logger.Debug("Обновление добавлено в дайджест",
		"totalChats", len(update.TgChatIDs),
	)
}

func (s *DigestService) runScheduler(ctx context.Context) {
	s.logger.Info("Запуск планировщика дайджеста",
		"hour", s.hour,
		"minute", s.minute,
	)

	now := time.Now()
	nextRun := time.Date(now.Year(), now.Month(), now.Day(), s.hour, s.minute, 0, 0, now.Location())

	if nextRun.Before(now) {
		nextRun = nextRun.Add(24 * time.Hour)
	}

	delay := nextRun.Sub(now)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Остановка планировщика дайджеста по контексту")
			return
		case <-s.schedulerDone:
			s.logger.Info("Остановка планировщика дайджеста")
			return
		case <-time.After(delay):
			if err := s.sendDigest(ctx); err != nil {
				s.logger.Error("Ошибка при отправке дайджеста",
					"error", err,
				)
			}

			nextRun = nextRun.Add(24 * time.Hour)
			delay = time.Until(nextRun)
		}
	}
}

func (s *DigestService) sendDigest(ctx context.Context) error {
	s.updatesMutex.Lock()
	updates := s.updates
	s.updates = make(map[int64][]*models.LinkUpdate)
	s.updatesMutex.Unlock()

	s.logger.Info("Отправка дайджеста",
		"totalChats", len(updates),
	)

	for chatID, chatUpdates := range updates {
		if len(chatUpdates) == 0 {
			continue
		}

		message := s.createDigestMessage(chatUpdates)

		update := &models.LinkUpdate{
			Description: message,
			TgChatIDs:   []int64{chatID},
		}

		if err := s.notifier.SendUpdate(ctx, update); err != nil {
			s.logger.Error("Ошибка при отправке дайджеста в чат",
				"error", err,
				"chatID", chatID,
			)
		}
	}

	return nil
}

func (s *DigestService) createDigestMessage(updates []*models.LinkUpdate) string {
	var message string

	message = fmt.Sprintf("📋 *Дайджест обновлений за %s*\n\n", time.Now().Format("02.01.2006"))

	count := 0
	maxUpdates := 10

	for _, update := range updates {
		if count >= maxUpdates {
			message += fmt.Sprintf("\n...и еще %d обновлений", len(updates)-maxUpdates)
			break
		}

		updateInfo := fmt.Sprintf("🔔 *Обновление*: [%s](%s)\n", update.URL, update.URL)

		if update.UpdateInfo != nil {
			info := update.UpdateInfo
			updateInfo += fmt.Sprintf("📄 *%s* от *%s*\n", info.Title, info.Author)
			updateInfo += fmt.Sprintf("⏱️ %s\n", info.UpdatedAt.Format("02.01.2006 15:04"))
			updateInfo += fmt.Sprintf("📝 %s\n", info.TextPreview)
		} else {
			updateInfo += fmt.Sprintf("%s\n", update.Description)
		}

		message += updateInfo + "\n"
		count++
	}

	return message
}
