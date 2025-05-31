package httputil_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/common/httputil"
	"github.com/central-university-dev/go-Matthew11K/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCircuitBreaker_FastFailure(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount++

		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &config.Config{
		ExternalRequestTimeout:     1 * time.Second,
		RetryCount:                 1,
		RetryBackoff:               50 * time.Millisecond,
		RetryableStatusCodes:       []int{500, 502, 503, 504},
		CBSlidingWindowSize:        1,
		CBMinimumRequiredCalls:     1,
		CBFailureRateThreshold:     100,
		CBPermittedCallsInHalfOpen: 1,
		CBWaitDurationInOpenState:  2 * time.Second,
	}

	client := httputil.CreateResilientHTTPClient(cfg, logger, "test_service")

	_, err := client.R().Get(server.URL + "/test")
	require.Error(t, err)

	initialRequestCount := requestCount

	start := time.Now()
	_, err = client.R().Get(server.URL + "/test")
	duration := time.Since(start)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker is open", "Ошибка должна указывать на открытый circuit breaker")

	assert.Less(t, duration, 200*time.Millisecond, "Circuit breaker должен отвечать быстро")

	finalRequestCount := requestCount
	assert.LessOrEqual(t, finalRequestCount, initialRequestCount+1,
		"Circuit breaker должен предотвратить дополнительные запросы к серверу")
}

func TestRetryWithBackoff(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount++
		if requestCount <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("success"))
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

	client := httputil.CreateResilientHTTPClient(cfg, logger, "test_service")

	resp, err := client.R().Get(server.URL + "/test")

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode())
	assert.Equal(t, 3, requestCount, "Должно быть 3 запроса: 2 неудачных + 1 успешный")
}

func TestNonRetryableStatus(t *testing.T) {
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

	client := httputil.CreateResilientHTTPClient(cfg, logger, "test_service")

	resp, err := client.R().Get(server.URL + "/test")

	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode())
	assert.Equal(t, 1, requestCount, "Должен быть только 1 запрос, retry не должен произойти")
}

func TestRetryStatefulBehavior(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	var state int32

	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)

		if count >= 3 {
			atomic.StoreInt32(&state, 1)
		}

		currentState := atomic.LoadInt32(&state)
		if currentState == 0 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error": "server error"}`))
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status": "success"}`))
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		ExternalRequestTimeout:     5 * time.Second,
		RetryCount:                 3,
		RetryBackoff:               50 * time.Millisecond,
		RetryableStatusCodes:       []int{500, 502, 503, 504},
		CBSlidingWindowSize:        100,
		CBMinimumRequiredCalls:     100,
		CBFailureRateThreshold:     100,
		CBPermittedCallsInHalfOpen: 10,
		CBWaitDurationInOpenState:  10 * time.Second,
	}

	client := httputil.CreateResilientHTTPClient(cfg, logger, "stateful_test")

	resp, err := client.R().Get(server.URL + "/api/test")

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode())
	assert.Contains(t, string(resp.Body()), "success")

	finalCount := atomic.LoadInt32(&requestCount)
	assert.Equal(t, int32(3), finalCount, "Должно быть ровно 3 запроса: 2 неудачных + 1 успешный")
}

func TestRetryNonRetryableStatusStateful(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "unauthorized"}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		ExternalRequestTimeout:     5 * time.Second,
		RetryCount:                 3,
		RetryBackoff:               50 * time.Millisecond,
		RetryableStatusCodes:       []int{500, 502, 503, 504, 429},
		CBSlidingWindowSize:        100,
		CBMinimumRequiredCalls:     100,
		CBFailureRateThreshold:     100,
		CBPermittedCallsInHalfOpen: 10,
		CBWaitDurationInOpenState:  10 * time.Second,
	}

	client := httputil.CreateResilientHTTPClient(cfg, logger, "non_retryable_test")

	resp, err := client.R().Get(server.URL + "/api/protected")

	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode())
	assert.Contains(t, string(resp.Body()), "unauthorized")

	finalCount := atomic.LoadInt32(&requestCount)
	assert.Equal(t, int32(1), finalCount, "Должен быть толко 1 запрос, retry не должен произойти для 401")
}

func TestCircuitBreakerWithDelayedResponse(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&requestCount, 1)

		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &config.Config{
		ExternalRequestTimeout:     2 * time.Second,
		RetryCount:                 1,
		RetryBackoff:               50 * time.Millisecond,
		RetryableStatusCodes:       []int{500, 502, 503, 504},
		CBSlidingWindowSize:        1,
		CBMinimumRequiredCalls:     1,
		CBFailureRateThreshold:     100,
		CBPermittedCallsInHalfOpen: 1,
		CBWaitDurationInOpenState:  300 * time.Millisecond,
	}

	client := httputil.CreateResilientHTTPClient(cfg, logger, "delay_test")

	start := time.Now()
	_, err := client.R().Get(server.URL + "/slow")
	firstDuration := time.Since(start)

	require.Error(t, err)

	start = time.Now()
	_, err = client.R().Get(server.URL + "/slow")
	secondDuration := time.Since(start)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker is open")

	assert.Less(t, secondDuration, 100*time.Millisecond,
		"Circuit breaker должен отвечать быстро без обращения к серверу")
	assert.Less(t, secondDuration, firstDuration/2,
		"Второй запрос должен быть быстрее первого минимум в 2 раза")

	finalCount := atomic.LoadInt32(&requestCount)
	assert.LessOrEqual(t, finalCount, int32(2),
		"Circuit breaker должен предотвратить лишние запросы")
}
