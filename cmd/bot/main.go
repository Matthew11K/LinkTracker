package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"log/slog"

	"github.com/central-university-dev/go-Matthew11K/internal/api/openapi/v1_bot"
	"github.com/central-university-dev/go-Matthew11K/internal/bot/cache"
	"github.com/central-university-dev/go-Matthew11K/internal/bot/clients"
	"github.com/central-university-dev/go-Matthew11K/internal/bot/clients/kafka"
	"github.com/central-university-dev/go-Matthew11K/internal/bot/domain"
	bothandler "github.com/central-university-dev/go-Matthew11K/internal/bot/handler"
	"github.com/central-university-dev/go-Matthew11K/internal/bot/repository"
	botservice "github.com/central-university-dev/go-Matthew11K/internal/bot/service"
	"github.com/central-university-dev/go-Matthew11K/internal/bot/telegram"
	commonservice "github.com/central-university-dev/go-Matthew11K/internal/common"
	"github.com/central-university-dev/go-Matthew11K/internal/common/middleware"
	"github.com/central-university-dev/go-Matthew11K/internal/config"
	"github.com/central-university-dev/go-Matthew11K/internal/database"
	"github.com/central-university-dev/go-Matthew11K/pkg"
	"github.com/central-university-dev/go-Matthew11K/pkg/txs"
)

const shutdownTimeout = 5 * time.Second

type httpCloser struct {
	*http.Server
	ctx context.Context
}

func newHTTPCloser(ctx context.Context, server *http.Server) httpCloser {
	return httpCloser{
		Server: server,
		ctx:    ctx,
	}
}

func (c httpCloser) Close() error {
	shutdownCtx, cancel := context.WithTimeout(c.ctx, shutdownTimeout)
	defer cancel()

	return c.Shutdown(shutdownCtx)
}

func gracefulShutdown(closers []io.Closer, stopCh <-chan struct{}, appLogger *slog.Logger) {
	<-stopCh
	appLogger.Info("Получен сигнал завершения")

	for _, c := range closers {
		if err := c.Close(); err != nil {
			appLogger.Error("Ошибка при закрытии ресурса",
				"error", err,
			)
		}
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
		{Command: "mode", Description: "Изменить режим уведомлений (мгновенный/дайджест)"},
		{Command: "time", Description: "Установить время доставки дайджеста"},
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

func startHTTPServer(_ context.Context, server *http.Server, port int, stopCh chan<- struct{}, appLogger *slog.Logger) {
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

	scrapperClient, err := clients.NewScrapperClient(cfg.ScrapperBaseURL, cfg, appLogger)
	if err != nil {
		appLogger.Error("Ошибка при создании клиента скраппера",
			"error", err,
		)

		return fmt.Errorf("ошибка создания клиента скраппера: %w", err)
	}

	telegramClient := clients.NewTelegramClient(cfg.TelegramBotToken, appLogger)
	setupTelegramCommands(telegramClient, appLogger)

	linkAnalyzer := commonservice.NewLinkAnalyzer()

	baseBotService := botservice.NewBotService(
		chatStateRepo,
		scrapperClient,
		telegramClient,
		linkAnalyzer,
		txManager,
	)

	var botHandler bothandler.BotHandler

	var messageHandler kafka.MessageHandler

	var botService telegram.BotService

	botService = baseBotService
	messageHandler = baseBotService

	var redisCache *cache.RedisLinkCache

	if cfg.RedisURL != "" {
		cacheTTL := cfg.RedisCacheTTL
		if cacheTTL <= 0 {
			cacheTTL = 30 * time.Minute
		}

		redisCache, err = cache.NewRedisLinkCache(ctx, cfg.RedisURL, cfg.RedisPassword, cfg.RedisDB, cacheTTL, appLogger)
		if err != nil {
			appLogger.Error("Ошибка при подключении к Redis",
				"error", err,
			)
		} else {
			appLogger.Info("Кэш Redis успешно инициализирован")

			cachedService := baseBotService.WithCache(redisCache)
			botService = cachedService
			messageHandler = cachedService
		}
	}

	var kafkaConsumer *kafka.Consumer

	if strings.EqualFold(cfg.MessageTransport, "KAFKA") {
		brokers := strings.Split(cfg.KafkaBrokers, ",")
		kafkaConsumer = kafka.NewConsumer(
			brokers,
			"bot-group",
			cfg.TopicLinkUpdates,
			cfg.TopicDeadLetterQueue,
			messageHandler,
			appLogger,
		)

		kafkaConsumer.Start(ctx)
		appLogger.Info("Kafka консьюмер успешно запущен")
	}

	botHandler = *bothandler.NewBotHandler(botService)

	server, err := v1_bot.NewServer(&botHandler)
	if err != nil {
		appLogger.Error("Ошибка при создании сервера",
			"error", err,
		)

		return fmt.Errorf("ошибка создания сервера: %w", err)
	}

	rateLimiter := middleware.NewRateLimiterMiddleware(
		cfg.RateLimitRequests,
		cfg.RateLimitWindow,
		appLogger,
	)

	serverWithMiddleware := rateLimiter.Middleware(server)

	poller := telegram.NewPoller(telegramClient, botService, appLogger)
	go poller.Start()

	httpServer := &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.BotServerPort),
		Handler:           serverWithMiddleware,
		ReadHeaderTimeout: 10 * time.Second,
	}

	stopCh := make(chan struct{})

	closers := []io.Closer{
		poller,
		newHTTPCloser(ctx, httpServer),
		rateLimiter,
	}

	if kafkaConsumer != nil {
		closers = append(closers, kafkaConsumer)
	}

	if redisCache != nil {
		closers = append(closers, redisCache)
	}

	startHTTPServer(ctx, httpServer, cfg.BotServerPort, stopCh, appLogger)
	gracefulShutdown(closers, stopCh, appLogger)

	return nil
}
