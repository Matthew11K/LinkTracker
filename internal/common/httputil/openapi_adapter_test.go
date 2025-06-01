package httputil_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/common/httputil"
	"github.com/central-university-dev/go-Matthew11K/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAPIAdapter_HeadersAndContext(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	receivedHeaders := make(map[string]string)

	var receivedUserAgent string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders["Authorization"] = r.Header.Get("Authorization")
		receivedHeaders["Content-Type"] = r.Header.Get("Content-Type")
		receivedUserAgent = r.Header.Get("User-Agent")

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"received": "ok"}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		ExternalRequestTimeout:     5 * time.Second,
		RetryCount:                 1,
		RetryBackoff:               50 * time.Millisecond,
		RetryableStatusCodes:       []int{500},
		CBSlidingWindowSize:        100,
		CBMinimumRequiredCalls:     10,
		CBFailureRateThreshold:     90,
		CBPermittedCallsInHalfOpen: 3,
		CBWaitDurationInOpenState:  10 * time.Second,
	}

	adapter := httputil.CreateResilientOpenAPIClient(cfg, logger, "test_service")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", server.URL+"/api/headers", strings.NewReader(`{"test": "data"}`))
	require.NoError(t, err)

	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "test-client/1.0")

	resp, err := adapter.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "Bearer test-token", receivedHeaders["Authorization"])
	assert.Equal(t, "application/json", receivedHeaders["Content-Type"])
	assert.Contains(t, receivedUserAgent, "test-client/1.0")
}
