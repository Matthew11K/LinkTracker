package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/api/handlers"
	"github.com/central-university-dev/go-Matthew11K/internal/api/openapi/v1/v1_bot"
	"github.com/central-university-dev/go-Matthew11K/internal/application/services"
	"github.com/central-university-dev/go-Matthew11K/internal/config"
	domainservices "github.com/central-university-dev/go-Matthew11K/internal/domain/services"
	"github.com/central-university-dev/go-Matthew11K/internal/infrastructure/clients"
	"github.com/central-university-dev/go-Matthew11K/internal/infrastructure/repositories/memory"
	"github.com/central-university-dev/go-Matthew11K/internal/telegram"
	"github.com/central-university-dev/go-Matthew11K/pkg"
	"github.com/joho/godotenv"
)

func main() {
	appLogger := pkg.NewLogger(os.Stdout)

	if err := godotenv.Load(); err != nil {
		appLogger.Error("Ошибка при загрузке .env файла",
			"error", err,
		)
	}

	cfg := config.LoadConfig()

	chatStateRepo := memory.NewChatStateRepository()
	scrapperClient, err := clients.NewScrapperClient(cfg.ScrapperBaseURL)
	if err != nil {
		appLogger.Error("Ошибка при создании клиента скраппера",
			"error", err,
		)
		os.Exit(1)
	}
	telegramClient := clients.NewTelegramClient(cfg.TelegramBotToken)
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
		Addr:    ":" + strconv.Itoa(cfg.BotServerPort),
		Handler: server,
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		appLogger.Info("Запуск HTTP сервера бота",
			"port", cfg.BotServerPort,
		)

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.Error("Ошибка при запуске HTTP сервера",
				"error", err,
			)
			sigCh <- syscall.SIGTERM
		}
	}()

	sig := <-sigCh
	appLogger.Info("Получен сигнал завершения",
		"signal", sig.String(),
	)

	poller.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		appLogger.Error("Ошибка при остановке HTTP сервера",
			"error", err,
		)
	}

	appLogger.Info("Сервер успешно остановлен")
}
