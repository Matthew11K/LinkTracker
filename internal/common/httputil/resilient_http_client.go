package httputil

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/config"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/go-resty/resty/v2"
	"github.com/sony/gobreaker"
)

type ResilientHTTPClient struct {
	client         *resty.Client
	circuitBreaker *gobreaker.CircuitBreaker
	logger         *slog.Logger
	serviceName    string
}

func CreateResilientHTTPClient(cfg *config.Config, logger *slog.Logger, serviceName string) *resty.Client {
	client := resty.New()

	client.SetTimeout(cfg.ExternalRequestTimeout)

	client.SetRetryCount(cfg.RetryCount)
	client.SetRetryWaitTime(cfg.RetryBackoff)
	client.SetRetryMaxWaitTime(cfg.RetryBackoff * 5)

	client.AddRetryCondition(func(r *resty.Response, err error) bool {
		if err != nil {
			return true
		}

		for _, status := range cfg.RetryableStatusCodes {
			if r.StatusCode() == status {
				return true
			}
		}

		return false
	})

	circuitBreakerSettings := gobreaker.Settings{
		Name:        serviceName + "_circuit_breaker",
		MaxRequests: uint32(cfg.CBPermittedCallsInHalfOpen), //nolint:gosec // G115: Значение из конфига
		Interval:    time.Duration(cfg.CBSlidingWindowSize) * time.Second,
		Timeout:     cfg.CBWaitDurationInOpenState,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= uint32(cfg.CBMinimumRequiredCalls) && //nolint:gosec // G115: Значение из конфига
				failureRatio >= float64(cfg.CBFailureRateThreshold)/100.0
		},
	}

	circuitBreaker := gobreaker.NewCircuitBreaker(circuitBreakerSettings)

	resilientClient := &ResilientHTTPClient{
		client:         client,
		circuitBreaker: circuitBreaker,
		logger:         logger,
		serviceName:    serviceName,
	}

	client.SetTransport(&CircuitBreakerTransport{
		resilientClient:   resilientClient,
		originalTransport: http.DefaultTransport,
	})

	if logger != nil {
		client.OnAfterResponse(func(_ *resty.Client, resp *resty.Response) error {
			if resp.Request.Attempt > 1 {
				logger.Info("HTTP client retry attempt",
					"service", serviceName,
					"url", resp.Request.URL,
					"attempt", resp.Request.Attempt,
					"status", resp.StatusCode(),
				)
			}

			return nil
		})
	}

	return client
}

type CircuitBreakerTransport struct {
	resilientClient   *ResilientHTTPClient
	originalTransport http.RoundTripper
}

func (t *CircuitBreakerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	result, err := t.resilientClient.circuitBreaker.Execute(func() (interface{}, error) {
		resp, err := t.originalTransport.RoundTrip(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode >= 500 {
			resp.Body.Close()
			return nil, &errors.HTTPError{StatusCode: resp.StatusCode}
		}

		return resp, nil
	})

	if err != nil {
		if err == gobreaker.ErrOpenState {
			if t.resilientClient.logger != nil {
				t.resilientClient.logger.Warn("Circuit breaker is open",
					"service", t.resilientClient.serviceName,
					"url", req.URL.String(),
				)
			}
		}

		return nil, err
	}

	return result.(*http.Response), nil
}
