package config

import (
	"time"

	"github.com/spf13/viper"
)

type AccessType string

const (
	SQLAccess      AccessType = "SQL"
	SquirrelAccess AccessType = "SQUIRREL" // Вместо ORM
)

type Config struct {
	TelegramBotToken       string        `mapstructure:"TELEGRAM_BOT_TOKEN"`
	BotServerPort          int           `mapstructure:"BOT_SERVER_PORT"`
	ScrapperServerPort     int           `mapstructure:"SCRAPPER_SERVER_PORT"`
	BotMetricsPort         int           `mapstructure:"BOT_METRICS_PORT"`
	ScrapperMetricsPort    int           `mapstructure:"SCRAPPER_METRICS_PORT"`
	ScrapperBaseURL        string        `mapstructure:"SCRAPPER_BASE_URL"`
	BotBaseURL             string        `mapstructure:"BOT_BASE_URL"`
	SchedulerCheckInterval time.Duration `mapstructure:"SCHEDULER_CHECK_INTERVAL"`
	GitHubAPIToken         string        `mapstructure:"GITHUB_API_TOKEN"`
	StackOverflowAPIToken  string        `mapstructure:"STACKOVERFLOW_API_TOKEN"`

	DatabaseURL        string     `mapstructure:"DATABASE_URL"`
	DatabaseAccessType AccessType `mapstructure:"DATABASE_ACCESS_TYPE"`
	DatabaseBatchSize  int        `mapstructure:"DATABASE_BATCH_SIZE"`
	DatabaseMaxConn    int        `mapstructure:"DATABASE_MAX_CONNECTIONS"`

	UseParallelScheduler bool `mapstructure:"USE_PARALLEL_SCHEDULER"`
	SchedulerWorkers     int  `mapstructure:"SCHEDULER_WORKERS"`

	KafkaBrokers         string `mapstructure:"KAFKA_BROKERS"`
	MessageTransport     string `mapstructure:"MESSAGE_TRANSPORT"`
	TopicLinkUpdates     string `mapstructure:"TOPIC_LINK_UPDATES"`
	TopicDeadLetterQueue string `mapstructure:"TOPIC_DEAD_LETTER_QUEUE"`

	RedisURL      string        `mapstructure:"REDIS_URL"`
	RedisPassword string        `mapstructure:"REDIS_PASSWORD"`
	RedisDB       int           `mapstructure:"REDIS_DB"`
	RedisCacheTTL time.Duration `mapstructure:"REDIS_CACHE_TTL"`

	DigestEnabled      bool   `mapstructure:"DIGEST_ENABLED"`
	DigestDeliveryTime string `mapstructure:"DIGEST_DELIVERY_TIME"`
	NotificationMode   string `mapstructure:"NOTIFICATION_MODE"`

	HTTPRequestTimeout     time.Duration `mapstructure:"HTTP_REQUEST_TIMEOUT"`
	ExternalRequestTimeout time.Duration `mapstructure:"EXTERNAL_REQUEST_TIMEOUT"`

	RateLimitRequests int           `mapstructure:"RATE_LIMIT_REQUESTS"`
	RateLimitWindow   time.Duration `mapstructure:"RATE_LIMIT_WINDOW"`

	RetryCount           int           `mapstructure:"RETRY_COUNT"`
	RetryBackoff         time.Duration `mapstructure:"RETRY_BACKOFF"`
	RetryableStatusCodes []int         `mapstructure:"RETRYABLE_STATUS_CODES"`

	CBSlidingWindowSize        int           `mapstructure:"CB_SLIDING_WINDOW_SIZE"`
	CBMinimumRequiredCalls     int           `mapstructure:"CB_MINIMUM_REQUIRED_CALLS"`
	CBFailureRateThreshold     int           `mapstructure:"CB_FAILURE_RATE_THRESHOLD"`
	CBPermittedCallsInHalfOpen int           `mapstructure:"CB_PERMITTED_CALLS_IN_HALF_OPEN"`
	CBWaitDurationInOpenState  time.Duration `mapstructure:"CB_WAIT_DURATION_IN_OPEN_STATE"`

	FallbackEnabled   bool   `mapstructure:"FALLBACK_ENABLED"`
	FallbackTransport string `mapstructure:"FALLBACK_TRANSPORT"`
}

func LoadConfig() *Config {
	setDefaults()

	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")

	viper.AutomaticEnv()

	_ = viper.ReadInConfig()

	config := &Config{}

	if err := viper.Unmarshal(config); err != nil {
		return getDefaultConfig()
	}

	return config
}

func setDefaults() {
	viper.SetDefault("BOT_SERVER_PORT", 8080)
	viper.SetDefault("SCRAPPER_SERVER_PORT", 8081)
	viper.SetDefault("BOT_METRICS_PORT", 9094)
	viper.SetDefault("SCRAPPER_METRICS_PORT", 9095)
	viper.SetDefault("SCRAPPER_BASE_URL", "http://link_tracker_scrapper:8081")
	viper.SetDefault("BOT_BASE_URL", "http://link_tracker_bot:8080")
	viper.SetDefault("SCHEDULER_CHECK_INTERVAL", "1m")

	viper.SetDefault("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/link_tracker")
	viper.SetDefault("DATABASE_ACCESS_TYPE", string(SQLAccess))
	viper.SetDefault("DATABASE_BATCH_SIZE", 100)
	viper.SetDefault("DATABASE_MAX_CONNECTIONS", 10)

	viper.SetDefault("USE_PARALLEL_SCHEDULER", true)
	viper.SetDefault("SCHEDULER_WORKERS", 4)

	viper.SetDefault("KAFKA_BROKERS", "kafka:9092")
	viper.SetDefault("MESSAGE_TRANSPORT", "HTTP")
	viper.SetDefault("TOPIC_LINK_UPDATES", "link-updates")
	viper.SetDefault("TOPIC_DEAD_LETTER_QUEUE", "link-updates-dlq")

	viper.SetDefault("REDIS_URL", "redis:6379")
	viper.SetDefault("REDIS_PASSWORD", "")
	viper.SetDefault("REDIS_DB", 0)
	viper.SetDefault("REDIS_CACHE_TTL", "30m")

	viper.SetDefault("DIGEST_ENABLED", false)
	viper.SetDefault("DIGEST_DELIVERY_TIME", "10:00")
	viper.SetDefault("NOTIFICATION_MODE", "instant")

	viper.SetDefault("HTTP_REQUEST_TIMEOUT", "5s")
	viper.SetDefault("EXTERNAL_REQUEST_TIMEOUT", "10s")

	viper.SetDefault("RATE_LIMIT_REQUESTS", 100)
	viper.SetDefault("RATE_LIMIT_WINDOW", "1m")

	viper.SetDefault("RETRY_COUNT", 3)
	viper.SetDefault("RETRY_BACKOFF", "1s")
	viper.SetDefault("RETRYABLE_STATUS_CODES", []int{408, 429, 500, 502, 503, 504})

	viper.SetDefault("CB_SLIDING_WINDOW_SIZE", 10)
	viper.SetDefault("CB_MINIMUM_REQUIRED_CALLS", 5)
	viper.SetDefault("CB_FAILURE_RATE_THRESHOLD", 50)
	viper.SetDefault("CB_PERMITTED_CALLS_IN_HALF_OPEN", 2)
	viper.SetDefault("CB_WAIT_DURATION_IN_OPEN_STATE", "10s")

	viper.SetDefault("FALLBACK_ENABLED", true)
	viper.SetDefault("FALLBACK_TRANSPORT", "Kafka") // HTTP -> Kafka
}

func getDefaultConfig() *Config {
	return &Config{
		BotServerPort:          8080,
		ScrapperServerPort:     8081,
		BotMetricsPort:         9094,
		ScrapperMetricsPort:    9095,
		ScrapperBaseURL:        "http://link_tracker_scrapper:8081",
		BotBaseURL:             "http://link_tracker_bot:8080",
		SchedulerCheckInterval: 1 * time.Minute,
		DatabaseURL:            "postgres://postgres:postgres@localhost:5432/link_tracker",
		DatabaseAccessType:     SQLAccess,
		DatabaseBatchSize:      100,
		DatabaseMaxConn:        10,
		UseParallelScheduler:   true,
		SchedulerWorkers:       4,

		KafkaBrokers:         "kafka:9092",
		MessageTransport:     "HTTP",
		TopicLinkUpdates:     "link-updates",
		TopicDeadLetterQueue: "link-updates-dlq",

		RedisURL:      "redis:6379",
		RedisPassword: "",
		RedisDB:       0,
		RedisCacheTTL: 30 * time.Minute,

		DigestEnabled:      false,
		DigestDeliveryTime: "10:00",
		NotificationMode:   "instant",

		HTTPRequestTimeout:     5 * time.Second,
		ExternalRequestTimeout: 10 * time.Second,

		RateLimitRequests: 100,
		RateLimitWindow:   1 * time.Minute,

		RetryCount:           3,
		RetryBackoff:         1 * time.Second,
		RetryableStatusCodes: []int{408, 429, 500, 502, 503, 504},

		CBSlidingWindowSize:        10,
		CBMinimumRequiredCalls:     5,
		CBFailureRateThreshold:     50,
		CBPermittedCallsInHalfOpen: 2,
		CBWaitDurationInOpenState:  10 * time.Second,

		FallbackEnabled:   true,
		FallbackTransport: "Kafka",
	}
}
