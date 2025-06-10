package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
	"github.com/central-university-dev/go-Matthew11K/internal/scrapper/repository"
	"github.com/go-co-op/gocron"
)

type LinkProcessor interface {
	ProcessLink(ctx context.Context, link *models.Link) (bool, error)
}

type ParallelScheduler struct {
	scheduler     *gocron.Scheduler
	linkProcessor LinkProcessor
	linkRepo      repository.LinkRepository
	logger        *slog.Logger
	interval      time.Duration
	batchSize     int
	workers       int
}

func NewParallelScheduler(
	linkProcessor LinkProcessor,
	linkRepo repository.LinkRepository,
	interval time.Duration,
	batchSize int,
	workers int,
	logger *slog.Logger,
) *ParallelScheduler {
	scheduler := gocron.NewScheduler(time.UTC)

	if workers <= 0 {
		workers = 4
	}

	if batchSize <= 0 {
		batchSize = 100
	}

	return &ParallelScheduler{
		scheduler:     scheduler,
		linkProcessor: linkProcessor,
		linkRepo:      linkRepo,
		logger:        logger,
		interval:      interval,
		batchSize:     batchSize,
		workers:       workers,
	}
}

func (s *ParallelScheduler) Start() {
	s.logger.Info("Запуск параллельного планировщика",
		"interval", s.interval.String(),
		"workers", s.workers,
		"batchSize", s.batchSize,
	)

	_, err := s.scheduler.Every(s.interval).Do(func() {
		s.logger.Info("Запуск параллельной обработки обновлений")

		ctx := context.Background()
		s.ProcessBatches(ctx)
	})

	if err != nil {
		s.logger.Error("Ошибка при настройке планировщика",
			"error", err,
		)

		return
	}

	s.scheduler.StartAsync()
}

func (s *ParallelScheduler) Stop() {
	s.logger.Info("Остановка параллельного планировщика")
	s.scheduler.Stop()
}

func (s *ParallelScheduler) ProcessBatches(ctx context.Context) {
	s.logger.Info("Начало обработки ссылок")

	if linkService, ok := s.linkProcessor.(interface{ UpdateActiveLinksMetrics(ctx context.Context) }); ok {
		linkService.UpdateActiveLinksMetrics(ctx)
	}

	offset := 0
	batchNum := 1
	processedCount := 0

	for {
		s.logger.Debug("Запрос очередной порции ссылок", "batchSize", s.batchSize, "offset", offset)

		links, err := s.linkRepo.FindDue(ctx, s.batchSize, offset)
		if err != nil {
			s.logger.Error("Ошибка при получении порции ссылок",
				"error", err,
				"offset", offset,
			)

			break
		}

		if len(links) == 0 {
			s.logger.Info("Больше нет ссылок для обработки")
			break
		}

		batchSize := len(links)
		s.logger.Info("Обработка батча",
			"batch", batchNum,
			"size", batchSize,
			"offset", offset,
		)

		s.processOneBatch(ctx, links, batchNum)

		processedCount += batchSize
		offset += batchSize
		batchNum++
	}

	s.logger.Info("Обработка ссылок завершена",
		"processed", processedCount,
	)
}

func (s *ParallelScheduler) processOneBatch(ctx context.Context, batch []*models.Link, batchNum int) {
	linkCh := make(chan *models.Link)
	wg := sync.WaitGroup{}

	for i := 0; i < s.workers; i++ {
		workerID := i + 1

		wg.Add(1)

		go func(workerID int) {
			defer wg.Done()
			s.worker(ctx, linkCh, workerID, batchNum)
		}(workerID)
	}

	go func() {
		for _, link := range batch {
			linkCh <- link
		}

		close(linkCh)
	}()

	wg.Wait()
}

func (s *ParallelScheduler) worker(ctx context.Context, linkCh <-chan *models.Link, workerID, batchNum int) {
	for link := range linkCh {
		s.logger.Debug("Воркер обрабатывает ссылку",
			"worker", workerID,
			"batch", batchNum,
			"link", link.ID,
			"url", link.URL,
		)

		updated, err := s.linkProcessor.ProcessLink(ctx, link)
		if err != nil {
			s.logger.Error("Ошибка при обработке ссылки",
				"worker", workerID,
				"batch", batchNum,
				"link", link.ID,
				"url", link.URL,
				"error", err,
			)

			continue
		}

		if updated {
			s.logger.Info("Ссылка обновлена",
				"worker", workerID,
				"batch", batchNum,
				"link", link.ID,
				"url", link.URL,
			)
		}
	}

	s.logger.Debug("Воркер завершил работу",
		"worker", workerID,
		"batch", batchNum,
	)
}
