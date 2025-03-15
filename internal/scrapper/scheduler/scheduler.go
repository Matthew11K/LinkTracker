package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/go-co-op/gocron"
)

type CheckUpdater interface {
	CheckUpdates(ctx context.Context) error
}

type Scheduler struct {
	scheduler    *gocron.Scheduler
	checkUpdater CheckUpdater
	logger       *slog.Logger
	interval     time.Duration
}

func NewScheduler(checkUpdater CheckUpdater, interval time.Duration, logger *slog.Logger) *Scheduler {
	scheduler := gocron.NewScheduler(time.UTC)

	return &Scheduler{
		scheduler:    scheduler,
		checkUpdater: checkUpdater,
		logger:       logger,
		interval:     interval,
	}
}

func (s *Scheduler) Start() {
	s.logger.Info("Запуск планировщика",
		"interval", s.interval.String(),
	)

	_, err := s.scheduler.Every(s.interval).Do(func() {
		s.logger.Info("Запуск проверки обновлений")

		ctx := context.Background()
		if err := s.checkUpdater.CheckUpdates(ctx); err != nil {
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
