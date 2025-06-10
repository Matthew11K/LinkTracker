package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	Namespace = "link_tracker"

	BotSubsystem      = "bot"
	ScrapperSubsystem = "scrapper"
)

// Общие метрики для всех сервисов.
var (
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests",
		},
		[]string{"service", "method", "endpoint", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: Namespace,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"service", "method", "endpoint"},
	)
)

// Бот метрики.
var (
	UserMessagesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: BotSubsystem,
			Name:      "user_messages_total",
			Help:      "Total number of user messages processed",
		},
		[]string{"chat_id", "message_type"},
	)

	UserMessagesRate = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: BotSubsystem,
			Name:      "user_messages_rate",
			Help:      "Rate of user messages per second",
		},
		[]string{"message_type"},
	)

	LinkUpdatesProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: BotSubsystem,
			Name:      "link_updates_processed_total",
			Help:      "Total number of link updates processed",
		},
		[]string{"status"},
	)
)

// Scrapper метрики.
var (
	ActiveLinksInDB = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Subsystem: ScrapperSubsystem,
			Name:      "active_links_count",
			Help:      "Number of active links in database by type",
		},
		[]string{"link_type"},
	)

	ScrapeRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: Namespace,
			Subsystem: ScrapperSubsystem,
			Name:      "scrape_request_duration_seconds",
			Help:      "Scrape request duration in seconds (p50, p95, p99)",
			Buckets:   []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 25, 50, 100},
		},
		[]string{"link_type", "status"},
	)

	ScrapeRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: ScrapperSubsystem,
			Name:      "scrape_requests_total",
			Help:      "Total number of scrape requests",
		},
		[]string{"link_type", "status"},
	)

	DatabaseQueriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: ScrapperSubsystem,
			Name:      "database_queries_total",
			Help:      "Total number of database queries",
		},
		[]string{"operation", "status"},
	)

	DatabaseQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: Namespace,
			Subsystem: ScrapperSubsystem,
			Name:      "database_query_duration_seconds",
			Help:      "Database query duration in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"operation"},
	)
)

func RecordHTTPRequest(service, method, endpoint string, statusCode int, duration time.Duration) {
	status := "success"
	if statusCode >= 400 {
		status = "error"
	}

	HTTPRequestsTotal.WithLabelValues(service, method, endpoint, status).Inc()
	HTTPRequestDuration.WithLabelValues(service, method, endpoint).Observe(duration.Seconds())
}

func RecordUserMessage(chatID, messageType string) {
	UserMessagesTotal.WithLabelValues(chatID, messageType).Inc()
	UserMessagesRate.WithLabelValues(messageType).Inc()
}

func RecordScrapeRequest(linkType, status string, duration time.Duration) {
	ScrapeRequestsTotal.WithLabelValues(linkType, status).Inc()
	ScrapeRequestDuration.WithLabelValues(linkType, status).Observe(duration.Seconds())
}

func UpdateActiveLinksCount(linkType string, count float64) {
	ActiveLinksInDB.WithLabelValues(linkType).Set(count)
}

func RecordDatabaseQuery(operation, status string, duration time.Duration) {
	DatabaseQueriesTotal.WithLabelValues(operation, status).Inc()
	DatabaseQueryDuration.WithLabelValues(operation).Observe(duration.Seconds())
}
