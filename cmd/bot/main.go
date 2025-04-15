package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"log/slog"

	"github.com/central-university-dev/go-Matthew11K/internal/api/openapi/v1/v1_bot"
	"github.com/central-university-dev/go-Matthew11K/internal/bot/clients"
	"github.com/central-university-dev/go-Matthew11K/internal/bot/domain"
	bothandler "github.com/central-university-dev/go-Matthew11K/internal/bot/handler"
	"github.com/central-university-dev/go-Matthew11K/internal/bot/repository"
	botservice "github.com/central-university-dev/go-Matthew11K/internal/bot/service"
	"github.com/central-university-dev/go-Matthew11K/internal/bot/telegram"
	commonservice "github.com/central-university-dev/go-Matthew11K/internal/common"
	"github.com/central-university-dev/go-Matthew11K/internal/config"
	"github.com/central-university-dev/go-Matthew11K/internal/database"
	"github.com/central-university-dev/go-Matthew11K/pkg"
	"github.com/central-university-dev/go-Matthew11K/pkg/txs"
)

func gracefulShutdown(server *http.Server, poller *telegram.Poller, stopCh <-chan struct{}, appLogger *slog.Logger) {
	<-stopCh
	appLogger.Info("Получен сигнал завершения")

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

func setupTelegramCommands(telegramClient domain.TelegramClientAPI, appLogger *slog.Logger) {
	botCommands := []domain.BotCommand{
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

func startHTTPServer(server *http.Server, port int, stopCh chan<- struct{}, appLogger *slog.Logger) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		appLogger.Info("Получен системный сигнал",
			"signal", sig.String(),
		)
		close(stopCh)
	}()

	go func() {
		appLogger.Info("Запуск HTTP сервера бота",
			"port", port,
		)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.Error("Ошибка при запуске HTTP сервера",
				"error", err,
			)
			close(stopCh)
		}
	}()
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка запуска сервиса: %v\n", err)
		os.Exit(1)
	}
}

//nolint:funlen // Длина функции обусловлена необходимостью последовательной инициализации всех компонентов.
func run() error {
	appLogger := pkg.NewLogger(os.Stdout)

	cfg := config.LoadConfig()

	ctx := context.Background()

	db, err := database.NewPostgresDB(ctx, cfg, appLogger)
	if err != nil {
		appLogger.Error("Ошибка при подключении к базе данных",
			"error", err,
		)

		return fmt.Errorf("ошибка подключения к базе данных: %w", err)
	}

	defer db.Close()

	txManager := txs.NewTxManager(db.Pool, appLogger)

	repoFactory := repository.NewFactory(db, cfg, appLogger)

	chatStateRepo, err := repoFactory.CreateChatStateRepository()
	if err != nil {
		appLogger.Error("Ошибка при создании репозитория состояний чата",
			"error", err,
		)

		return fmt.Errorf("ошибка создания репозитория состояний чата: %w", err)
	}

	scrapperClient, err := clients.NewScrapperClient(cfg.ScrapperBaseURL)
	if err != nil {
		appLogger.Error("Ошибка при создании клиента скраппера",
			"error", err,
		)

		return fmt.Errorf("ошибка создания клиента скраппера: %w", err)
	}

	telegramClient := clients.NewTelegramClient(cfg.TelegramBotToken, appLogger)
	setupTelegramCommands(telegramClient, appLogger)

	linkAnalyzer := commonservice.NewLinkAnalyzer()

	botService := botservice.NewBotService(
		chatStateRepo,
		scrapperClient,
		telegramClient,
		linkAnalyzer,
		txManager,
	)

	botHandler := bothandler.NewBotHandler(botService)

	server, err := v1_bot.NewServer(botHandler)
	if err != nil {
		appLogger.Error("Ошибка при создании сервера",
			"error", err,
		)

		return fmt.Errorf("ошибка создания сервера: %w", err)
	}

	poller := telegram.NewPoller(telegramClient, botService, appLogger)
	go poller.Start()

	httpServer := &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.BotServerPort),
		Handler:           server,
		ReadHeaderTimeout: 10 * time.Second,
	}

	stopCh := make(chan struct{})

	startHTTPServer(httpServer, cfg.BotServerPort, stopCh, appLogger)
	gracefulShutdown(httpServer, poller, stopCh, appLogger)

	return nil
}
