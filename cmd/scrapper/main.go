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

	"github.com/central-university-dev/go-Matthew11K/internal/api/openapi/v1_scrapper"
	"github.com/central-university-dev/go-Matthew11K/internal/common"
	"github.com/central-university-dev/go-Matthew11K/internal/common/middleware"
	"github.com/central-university-dev/go-Matthew11K/internal/config"
	"github.com/central-university-dev/go-Matthew11K/internal/database"
	"github.com/central-university-dev/go-Matthew11K/internal/scrapper/cache"
	"github.com/central-university-dev/go-Matthew11K/internal/scrapper/clients"
	"github.com/central-university-dev/go-Matthew11K/internal/scrapper/handler"
	"github.com/central-university-dev/go-Matthew11K/internal/scrapper/notify"
	"github.com/central-university-dev/go-Matthew11K/internal/scrapper/repository"
	"github.com/central-university-dev/go-Matthew11K/internal/scrapper/scheduler"
	"github.com/central-university-dev/go-Matthew11K/internal/scrapper/service"
	"github.com/central-university-dev/go-Matthew11K/pkg"
	"github.com/central-university-dev/go-Matthew11K/pkg/txs"
)

type Scheduler interface {
	Start()
	Stop()
}

func gracefulShutdown(
	ctx context.Context,
	server *http.Server,
	updateScheduler Scheduler,
	digestService *service.DigestService,
	stopCh <-chan struct{},
	appLogger *slog.Logger,
) {
	<-stopCh
	appLogger.Info("Получен сигнал завершения")

	updateScheduler.Stop()

	if digestService != nil {
		digestService.Stop()
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		appLogger.Error("Ошибка при остановке HTTP сервера",
			"error", err,
		)
	}

	appLogger.Info("Сервер успешно остановлен")
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
		appLogger.Info("Запуск HTTP сервера скраппера",
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

	linkRepo, err := repoFactory.CreateLinkRepository()
	if err != nil {
		appLogger.Error("Ошибка при создании репозитория ссылок",
			"error", err,
		)

		return err
	}

	chatRepo, err := repoFactory.CreateChatRepository()
	if err != nil {
		appLogger.Error("Ошибка при создании репозитория чатов",
			"error", err,
		)

		return err
	}

	detailsRepo, err := repoFactory.CreateContentDetailsRepository()
	if err != nil {
		appLogger.Error("Ошибка при создании репозитория деталей контента",
			"error", err,
		)

		return err
	}

	updaterFactory := common.NewLinkUpdaterFactory(
		clients.NewGitHubClient(cfg.GitHubAPIToken, "", cfg, appLogger),
		clients.NewStackOverflowClient(cfg.StackOverflowAPIToken, "", cfg, appLogger),
	)

	linkAnalyzer := common.NewLinkAnalyzer()

	notifierFactory := notify.NewNotifierFactory(cfg, appLogger)

	botNotifier, err := notifierFactory.CreateNotifier()
	if err != nil {
		appLogger.Error("Ошибка при создании нотификатора бота",
			"error", err,
		)

		return err
	}

	var digestCache service.DigestCache

	var digestService *service.DigestService

	if cfg.DigestEnabled {
		redisTTL := cfg.RedisCacheTTL

		digestCache, err = cache.NewRedisDigestCache(ctx, cfg.RedisURL, cfg.RedisPassword, cfg.RedisDB, redisTTL, appLogger)
		if err != nil {
			appLogger.Error("Ошибка при подключении к Redis для дайджестов",
				"error", err,
			)

			appLogger.Warn("Продолжаем без дайджестов")
		} else {
			digestService = service.NewDigestService(cfg, botNotifier, digestCache, chatRepo, appLogger)
			digestService.Start(ctx)
			appLogger.Info("Сервис дайджестов успешно запущен")
		}
	} else {
		appLogger.Info("Дайджесты отключены в конфигурации")
	}

	scrapperService := service.NewScrapperService(
		linkRepo,
		chatRepo,
		botNotifier,
		digestService,
		detailsRepo,
		updaterFactory,
		linkAnalyzer,
		appLogger,
		txManager,
	)

	tagService := service.NewTagService(linkRepo, chatRepo, appLogger)

	scrapperHandler := handler.NewScrapperHandler(scrapperService, tagService)

	server, err := v1_scrapper.NewServer(scrapperHandler)
	if err != nil {
		appLogger.Error("Ошибка при создании сервера",
			"error", err,
		)

		return err
	}

	rateLimiter := middleware.NewRateLimiterMiddleware(
		cfg.RateLimitRequests,
		cfg.RateLimitWindow,
		appLogger,
	)

	serverWithMiddleware := rateLimiter.Middleware(server)

	var sch interface {
		Start()
		Stop()
	}

	if cfg.UseParallelScheduler {
		appLogger.Info("Использование параллельного шедулера",
			"batchSize", cfg.DatabaseBatchSize,
			"workers", cfg.SchedulerWorkers,
		)

		sch = scheduler.NewParallelScheduler(
			scrapperService,
			linkRepo,
			cfg.SchedulerCheckInterval,
			cfg.DatabaseBatchSize,
			cfg.SchedulerWorkers,
			appLogger,
		)
	} else {
		return fmt.Errorf("обычный шедулер больше не поддерживается в текущей конфигурации")
	}

	sch.Start()

	httpServer := &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.ScrapperServerPort),
		Handler:           serverWithMiddleware,
		ReadHeaderTimeout: 10 * time.Second,
	}

	stopCh := make(chan struct{})

	startHTTPServer(ctx, httpServer, cfg.ScrapperServerPort, stopCh, appLogger)

	gracefulShutdown(ctx, httpServer, sch, digestService, stopCh, appLogger)

	return nil
}
