package config

import (
	"time"

	"github.com/spf13/viper"
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
		// В случае ошибки используем значения по умолчанию
		return getDefaultConfig()
	}

	return config
}

func setDefaults() {
	viper.SetDefault("BOT_SERVER_PORT", 8080)
	viper.SetDefault("SCRAPPER_SERVER_PORT", 8081)
	viper.SetDefault("SCRAPPER_BASE_URL", "http://localhost:8081")
	viper.SetDefault("BOT_BASE_URL", "http://localhost:8080")
	viper.SetDefault("SCHEDULER_CHECK_INTERVAL", "1m")
}

func getDefaultConfig() *Config {
	return &Config{
		BotServerPort:          8080,
		ScrapperServerPort:     8081,
		ScrapperBaseURL:        "http://localhost:8081",
		BotBaseURL:             "http://localhost:8080",
		SchedulerCheckInterval: 1 * time.Minute,
	}
}
