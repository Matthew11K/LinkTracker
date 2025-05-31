package clients_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/config"
	"github.com/central-university-dev/go-Matthew11K/internal/scrapper/clients"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitHubClient_RetryBehavior(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount++
		if requestCount <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")

			response := `{"updated_at": "2023-01-01T10:00:00Z"}`
			if _, err := w.Write([]byte(response)); err != nil {
				t.Errorf("Failed to write response: %v", err)
			}
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		ExternalRequestTimeout:     5 * time.Second,
		RetryCount:                 3,
		RetryBackoff:               100 * time.Millisecond,
		RetryableStatusCodes:       []int{500, 502, 503, 504},
		CBSlidingWindowSize:        100,
		CBMinimumRequiredCalls:     10,
		CBFailureRateThreshold:     90,
		CBPermittedCallsInHalfOpen: 3,
		CBWaitDurationInOpenState:  10 * time.Second,
	}

	client := clients.NewGitHubClient("", server.URL, cfg, logger)

	ctx := context.Background()

	lastUpdate, err := client.GetRepositoryLastUpdate(ctx, "owner", "repo")

	require.NoError(t, err)

	assert.Equal(t, 3, requestCount)

	_ = lastUpdate
}

func TestGitHubClient_NonRetryableStatus(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount++

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		ExternalRequestTimeout:     5 * time.Second,
		RetryCount:                 3,
		RetryBackoff:               100 * time.Millisecond,
		RetryableStatusCodes:       []int{500, 502, 503, 504},
		CBSlidingWindowSize:        100,
		CBMinimumRequiredCalls:     10,
		CBFailureRateThreshold:     90,
		CBPermittedCallsInHalfOpen: 3,
		CBWaitDurationInOpenState:  10 * time.Second,
	}

	client := clients.NewGitHubClient("", server.URL, cfg, logger)

	ctx := context.Background()

	_, err := client.GetRepositoryLastUpdate(ctx, "owner", "repo")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")

	assert.Equal(t, 1, requestCount)
}
