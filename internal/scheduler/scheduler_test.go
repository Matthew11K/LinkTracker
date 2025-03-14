package scheduler_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/scheduler"
	"github.com/central-university-dev/go-Matthew11K/internal/scheduler/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestScheduler_Start(t *testing.T) {
	mockCheckUpdater := new(mocks.CheckUpdater)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	interval := 100 * time.Millisecond
	//nolint //тест
	mockCheckUpdater.On("CheckUpdates", mock.MatchedBy(func(ctx context.Context) bool {
		return true
	})).Return(nil)

	scheduler := scheduler.NewScheduler(mockCheckUpdater, interval, logger)
	scheduler.Start()

	time.Sleep(150 * time.Millisecond)
	scheduler.Stop()

	mockCheckUpdater.AssertExpectations(t)
}

func TestScheduler_Stop(t *testing.T) {
	mockCheckUpdater := new(mocks.CheckUpdater)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	interval := 1 * time.Second

	scheduler := scheduler.NewScheduler(mockCheckUpdater, interval, logger)

	scheduler.Start()
	scheduler.Stop()

	mockCheckUpdater.AssertNotCalled(t, "CheckUpdates", mock.Anything)
}

func TestScheduler_CheckUpdatesWithError(t *testing.T) {
	mockCheckUpdater := new(mocks.CheckUpdater)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	interval := 100 * time.Millisecond
	//nolint //тест
	mockCheckUpdater.On("CheckUpdates", mock.MatchedBy(func(ctx context.Context) bool {
		return true
	})).Return(assert.AnError)

	scheduler := scheduler.NewScheduler(mockCheckUpdater, interval, logger)
	scheduler.Start()

	time.Sleep(150 * time.Millisecond)
	scheduler.Stop()

	mockCheckUpdater.AssertExpectations(t)
}
