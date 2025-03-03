package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"

	"github.com/go-co-op/gocron"
)

type ScrapperService interface {
	RegisterChat(ctx context.Context, chatID int64) error

	DeleteChat(ctx context.Context, chatID int64) error

	AddLink(ctx context.Context, chatID int64, url string, tags []string, filters []string) (*models.Link, error)

	RemoveLink(ctx context.Context, chatID int64, url string) (*models.Link, error)

	GetLinks(ctx context.Context, chatID int64) ([]*models.Link, error)

	CheckUpdates(ctx context.Context) error
}

type Scheduler struct {
	scheduler       *gocron.Scheduler
	scrapperService ScrapperService
	logger          *slog.Logger
	interval        time.Duration
}

func NewScheduler(scrapperService ScrapperService, interval time.Duration, logger *slog.Logger) *Scheduler {
	scheduler := gocron.NewScheduler(time.UTC)

	return &Scheduler{
		scheduler:       scheduler,
		scrapperService: scrapperService,
		logger:          logger,
		interval:        interval,
	}
}

func (s *Scheduler) Start() {
	s.logger.Info("Запуск планировщика",
		"interval", s.interval.String(),
	)

	_, err := s.scheduler.Every(s.interval).Do(func() {
		s.logger.Info("Запуск проверки обновлений")

		ctx := context.Background()
		if err := s.scrapperService.CheckUpdates(ctx); err != nil {
			s.logger.Error("Ошибка при проверке обновлений",
				"error", err,
			)
		}
	})

	if err != nil {
		s.logger.Error("Ошибка при настройке планировщика",
			"error", err,
		)

		return
	}

	s.scheduler.StartAsync()
}

func (s *Scheduler) Stop() {
	s.logger.Info("Остановка планировщика")
	s.scheduler.Stop()
}
