package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"log/slog"

	"github.com/central-university-dev/go-Matthew11K/internal/api/handlers"
	"github.com/central-university-dev/go-Matthew11K/internal/api/openapi/v1/v1_bot"
	"github.com/central-university-dev/go-Matthew11K/internal/application/services"
	"github.com/central-university-dev/go-Matthew11K/internal/config"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/clients"
	domainservices "github.com/central-university-dev/go-Matthew11K/internal/domain/services"
	infraclients "github.com/central-university-dev/go-Matthew11K/internal/infrastructure/clients"
	"github.com/central-university-dev/go-Matthew11K/internal/infrastructure/repositories/memory"
	"github.com/central-university-dev/go-Matthew11K/internal/telegram"
	"github.com/central-university-dev/go-Matthew11K/pkg"
	"github.com/joho/godotenv"
)

func gracefulShutdown(server *http.Server, poller *telegram.Poller, appLogger *slog.Logger) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	appLogger.Info("Получен сигнал завершения",
		"signal", sig.String(),
	)

	poller.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		appLogger.Error("Ошибка при остановке HTTP сервера",
			"error", err,
		)
	}

	appLogger.Info("Сервер успешно остановлен")
}

func setupTelegramCommands(telegramClient clients.TelegramClient, appLogger *slog.Logger) {
	botCommands := []clients.BotCommand{
		{Command: "start", Description: "Начать работу с ботом"},
		{Command: "help", Description: "Получить справку о командах"},
		{Command: "track", Description: "Отслеживать ссылку"},
		{Command: "untrack", Description: "Прекратить отслеживание ссылки"},
		{Command: "list", Description: "Список отслеживаемых ссылок"},
	}

	ctx := context.Background()
	if err := telegramClient.SetMyCommands(ctx, botCommands); err != nil {
		appLogger.Error("Ошибка при регистрации команд бота",
			"error", err,
		)
	} else {
		appLogger.Info("Команды бота успешно зарегистрированы")
	}
}

func startHTTPServer(server *http.Server, port int, appLogger *slog.Logger) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		appLogger.Info("Запуск HTTP сервера бота",
			"port", port,
		)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.Error("Ошибка при запуске HTTP сервера",
				"error", err,
			)
			sigCh <- syscall.SIGTERM
		}
	}()
}

func main() {
	appLogger := pkg.NewLogger(os.Stdout)

	if err := godotenv.Load(); err != nil {
		appLogger.Error("Ошибка при загрузке .env файла",
			"error", err,
		)
	}

	cfg := config.LoadConfig()

	chatStateRepo := memory.NewChatStateRepository()

	scrapperClient, err := infraclients.NewScrapperClient(cfg.ScrapperBaseURL)
	if err != nil {
		appLogger.Error("Ошибка при создании клиента скраппера",
			"error", err,
		)
		os.Exit(1)
	}

	telegramClient := infraclients.NewTelegramClient(cfg.TelegramBotToken)
	setupTelegramCommands(telegramClient, appLogger)

	linkAnalyzer := domainservices.NewLinkAnalyzer()

	botService := services.NewBotService(
		chatStateRepo,
		scrapperClient,
		telegramClient,
		linkAnalyzer,
	)

	botHandler := handlers.NewBotHandler(botService)

	server, err := v1_bot.NewServer(botHandler)
	if err != nil {
		appLogger.Error("Ошибка при создании сервера",
			"error", err,
		)
		os.Exit(1)
	}

	poller := telegram.NewPoller(telegramClient, botService, appLogger)
	go poller.Start()

	httpServer := &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.BotServerPort),
		Handler:           server,
		ReadHeaderTimeout: 10 * time.Second,
	}

	startHTTPServer(httpServer, cfg.BotServerPort, appLogger)
	gracefulShutdown(httpServer, poller, appLogger)
}
