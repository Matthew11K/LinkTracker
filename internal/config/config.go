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
}

func getDefaultConfig() *Config {
	return &Config{
		BotServerPort:          8080,
		ScrapperServerPort:     8081,
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
	}
}
