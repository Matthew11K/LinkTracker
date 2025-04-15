package scheduler_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
	"github.com/central-university-dev/go-Matthew11K/internal/scrapper/scheduler"
	"github.com/central-university-dev/go-Matthew11K/internal/scrapper/scheduler/mocks"
)

func TestParallelScheduler_ProcessBatches(t *testing.T) {
	mockLinkProcessor := mocks.NewLinkProcessor(t)
	mockLinkRepo := mocks.NewLinkRepository(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	batchSize := 2
	workers := 2
	ctx := context.Background()

	link1 := &models.Link{ID: 1, URL: "url1"}
	link2 := &models.Link{ID: 2, URL: "url2"}
	link3 := &models.Link{ID: 3, URL: "url3"}
	linksBatch1 := []*models.Link{link1, link2}
	linksBatch2 := []*models.Link{link3}

	mockLinkRepo.EXPECT().
		FindDue(ctx, batchSize, 0).
		Return(linksBatch1, nil).
		Once()
	mockLinkRepo.EXPECT().
		FindDue(ctx, batchSize, batchSize).
		Return(linksBatch2, nil).
		Once()
	mockLinkRepo.EXPECT().
		FindDue(ctx, batchSize, batchSize+len(linksBatch2)).
		Return([]*models.Link{}, nil).
		Once()

	mockLinkProcessor.EXPECT().ProcessLink(ctx, link1).Return(true, nil).Once()
	mockLinkProcessor.EXPECT().ProcessLink(ctx, link2).Return(false, nil).Once()
	mockLinkProcessor.EXPECT().ProcessLink(ctx, link3).Return(true, nil).Once()

	parallelScheduler := scheduler.NewParallelScheduler(
		mockLinkProcessor,
		mockLinkRepo,
		1*time.Hour,
		batchSize,
		workers,
		logger,
	)

	parallelScheduler.ProcessBatches(ctx)

	mockLinkRepo.AssertExpectations(t)
	mockLinkProcessor.AssertExpectations(t)
}
