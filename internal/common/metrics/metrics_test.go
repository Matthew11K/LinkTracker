package metrics_test

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/central-university-dev/go-Matthew11K/internal/common/metrics"
)

const (
	statusSuccess = "success"
)

func TestRecordHTTPRequest(t *testing.T) {
	// Arrange
	service := "test-service"
	method := "GET"
	endpoint := "/test"
	statusCode := 200
	duration := 100 * time.Millisecond

	// Act
	metrics.RecordHTTPRequest(service, method, endpoint, statusCode, duration)

	// Assert
	counterValue := testutil.ToFloat64(metrics.HTTPRequestsTotal.WithLabelValues(service, method, endpoint, "success"))
	assert.Equal(t, float64(1), counterValue)

	assert.NotNil(t, metrics.HTTPRequestDuration)
}

func TestRecordHTTPRequestError(t *testing.T) {
	// Arrange
	service := "test-service"
	method := "POST"
	endpoint := "/error"
	statusCode := 500
	duration := 50 * time.Millisecond

	// Act
	metrics.RecordHTTPRequest(service, method, endpoint, statusCode, duration)

	// Assert
	counterValue := testutil.ToFloat64(metrics.HTTPRequestsTotal.WithLabelValues(service, method, endpoint, "error"))
	assert.Equal(t, float64(1), counterValue)
}

func TestRecordUserMessage(t *testing.T) {
	// Arrange
	chatID := "123456"
	messageType := "command"

	// Act
	metrics.RecordUserMessage(chatID, messageType)

	// Assert
	totalValue := testutil.ToFloat64(metrics.UserMessagesTotal.WithLabelValues(chatID, messageType))
	assert.Equal(t, float64(1), totalValue)

	rateValue := testutil.ToFloat64(metrics.UserMessagesRate.WithLabelValues(messageType))
	assert.Equal(t, float64(1), rateValue)
}

func TestRecordScrapeRequest(t *testing.T) {
	// Arrange
	linkType := "github"
	status := statusSuccess
	duration := 200 * time.Millisecond

	// Act
	metrics.RecordScrapeRequest(linkType, status, duration)

	// Assert
	counterValue := testutil.ToFloat64(metrics.ScrapeRequestsTotal.WithLabelValues(linkType, status))
	assert.Equal(t, float64(1), counterValue)

	assert.NotNil(t, metrics.ScrapeRequestDuration)
}

func TestUpdateActiveLinksCount(t *testing.T) {
	// Arrange
	linkType := "stackoverflow"
	count := float64(42)

	// Act
	metrics.UpdateActiveLinksCount(linkType, count)

	// Assert
	gaugeValue := testutil.ToFloat64(metrics.ActiveLinksInDB.WithLabelValues(linkType))
	assert.Equal(t, count, gaugeValue)
}

func TestRecordDatabaseQuery(t *testing.T) {
	// Arrange
	operation := "SELECT"
	status := statusSuccess
	duration := 10 * time.Millisecond

	// Act
	metrics.RecordDatabaseQuery(operation, status, duration)

	// Assert
	counterValue := testutil.ToFloat64(metrics.DatabaseQueriesTotal.WithLabelValues(operation, status))
	assert.Equal(t, float64(1), counterValue)

	assert.NotNil(t, metrics.DatabaseQueryDuration)
}

func TestMetricsExist(t *testing.T) {
	// Arrange & Act & Assert
	metricFamilies, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	metricNames := make(map[string]bool)
	for _, mf := range metricFamilies {
		metricNames[*mf.Name] = true
	}

	expectedMetrics := []string{
		"link_tracker_http_requests_total",
		"link_tracker_http_request_duration_seconds",
		"link_tracker_bot_user_messages_total",
		"link_tracker_bot_user_messages_rate",
		"link_tracker_scrapper_active_links_count",
		"link_tracker_scrapper_scrape_request_duration_seconds",
		"link_tracker_scrapper_scrape_requests_total",
		"link_tracker_scrapper_database_queries_total",
		"link_tracker_scrapper_database_query_duration_seconds",
	}

	for _, metricName := range expectedMetrics {
		assert.True(t, metricNames[metricName], "Метрика %s должна быть зарегистрирована", metricName)
	}
}

func TestScrapeRequestPercentiles(t *testing.T) {
	// Arrange
	linkType := "github_test"
	status := statusSuccess

	durations := []time.Duration{
		10 * time.Millisecond,   // p50
		500 * time.Millisecond,  // p95
		1000 * time.Millisecond, // p99
	}

	initialValue := testutil.ToFloat64(metrics.ScrapeRequestsTotal.WithLabelValues(linkType, status))

	// Act
	for _, duration := range durations {
		metrics.RecordScrapeRequest(linkType, status, duration)
	}

	// Assert
	assert.NotNil(t, metrics.ScrapeRequestDuration)

	finalValue := testutil.ToFloat64(metrics.ScrapeRequestsTotal.WithLabelValues(linkType, status))
	assert.Equal(t, initialValue+float64(len(durations)), finalValue)
}

func TestMultipleMessageTypes(t *testing.T) {
	// Arrange
	chatID := "789_test"
	messageTypes := []string{"command_test", "message_test", "callback_test"}

	// Act & Assert
	for i, messageType := range messageTypes {
		initialTotalValue := testutil.ToFloat64(metrics.UserMessagesTotal.WithLabelValues(chatID, messageType))
		initialRateValue := testutil.ToFloat64(metrics.UserMessagesRate.WithLabelValues(messageType))

		metrics.RecordUserMessage(chatID, messageType)

		finalTotalValue := testutil.ToFloat64(metrics.UserMessagesTotal.WithLabelValues(chatID, messageType))
		assert.Equal(t, initialTotalValue+1, finalTotalValue, "Iteration %d", i)

		finalRateValue := testutil.ToFloat64(metrics.UserMessagesRate.WithLabelValues(messageType))
		assert.Equal(t, initialRateValue+1, finalRateValue, "Iteration %d", i)
	}
}
