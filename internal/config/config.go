package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	TelegramBotToken       string
	BotServerPort          int
	ScrapperServerPort     int
	ScrapperBaseURL        string
	BotBaseURL             string
	SchedulerCheckInterval time.Duration
	GitHubAPIToken         string
	StackOverflowAPIToken  string
}

func LoadConfig() *Config {
	config := &Config{
		BotServerPort:          8080,
		ScrapperServerPort:     8081,
		ScrapperBaseURL:        "http://localhost:8081",
		BotBaseURL:             "http://localhost:8080",
		SchedulerCheckInterval: 1 * time.Minute,
	}

	if token := os.Getenv("TELEGRAM_BOT_TOKEN"); token != "" {
		config.TelegramBotToken = token
	}

	if portStr := os.Getenv("BOT_SERVER_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			config.BotServerPort = port
		}
	}

	if portStr := os.Getenv("SCRAPPER_SERVER_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			config.ScrapperServerPort = port
		}
	}

	if url := os.Getenv("SCRAPPER_BASE_URL"); url != "" {
		config.ScrapperBaseURL = url
	}

	if url := os.Getenv("BOT_BASE_URL"); url != "" {
		config.BotBaseURL = url
	}

	if intervalStr := os.Getenv("SCHEDULER_CHECK_INTERVAL"); intervalStr != "" {
		if interval, err := time.ParseDuration(intervalStr); err == nil {
			config.SchedulerCheckInterval = interval
		}
	}

	if token := os.Getenv("GITHUB_API_TOKEN"); token != "" {
		config.GitHubAPIToken = token
	}

	if token := os.Getenv("STACKOVERFLOW_API_TOKEN"); token != "" {
		config.StackOverflowAPIToken = token
	}

	return config
}
