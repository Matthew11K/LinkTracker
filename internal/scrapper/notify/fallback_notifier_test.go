package notify_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
	"github.com/central-university-dev/go-Matthew11K/internal/scrapper/notify"
	"github.com/central-university-dev/go-Matthew11K/internal/scrapper/notify/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestFallbackBotNotifier_PrimarySuccess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	primaryMock := mocks.NewBotNotifier(t)
	secondaryMock := mocks.NewBotNotifier(t)

	fallbackNotifier := notify.NewFallbackBotNotifier(primaryMock, secondaryMock, logger)

	update := &models.LinkUpdate{
		ID:        1,
		URL:       "https://example.com",
		TgChatIDs: []int64{123},
	}

	primaryMock.On("SendUpdate", mock.Anything, update).Return(nil)

	err := fallbackNotifier.SendUpdate(context.Background(), update)

	require.NoError(t, err)
	primaryMock.AssertExpectations(t)
	secondaryMock.AssertNotCalled(t, "SendUpdate")
}

func TestFallbackBotNotifier_PrimaryFailsSecondarySuccess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	primaryMock := mocks.NewBotNotifier(t)
	secondaryMock := mocks.NewBotNotifier(t)

	fallbackNotifier := notify.NewFallbackBotNotifier(primaryMock, secondaryMock, logger)

	update := &models.LinkUpdate{
		ID:        1,
		URL:       "https://example.com",
		TgChatIDs: []int64{123},
	}

	primaryError := errors.New("primary transport failed")

	primaryMock.On("SendUpdate", mock.Anything, update).Return(primaryError)
	secondaryMock.On("SendUpdate", mock.Anything, update).Return(nil)

	err := fallbackNotifier.SendUpdate(context.Background(), update)

	require.NoError(t, err)
	primaryMock.AssertExpectations(t)
	secondaryMock.AssertExpectations(t)
}

func TestFallbackBotNotifier_BothFail(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	primaryMock := mocks.NewBotNotifier(t)
	secondaryMock := mocks.NewBotNotifier(t)

	fallbackNotifier := notify.NewFallbackBotNotifier(primaryMock, secondaryMock, logger)

	update := &models.LinkUpdate{
		ID:        1,
		URL:       "https://example.com",
		TgChatIDs: []int64{123},
	}

	primaryError := errors.New("primary transport failed")
	secondaryError := errors.New("secondary transport failed")

	primaryMock.On("SendUpdate", mock.Anything, update).Return(primaryError)
	secondaryMock.On("SendUpdate", mock.Anything, update).Return(secondaryError)

	err := fallbackNotifier.SendUpdate(context.Background(), update)

	require.Error(t, err)
	assert.Equal(t, primaryError, err)
	primaryMock.AssertExpectations(t)
	secondaryMock.AssertExpectations(t)
}
