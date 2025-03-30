package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/common"
	"github.com/central-university-dev/go-Matthew11K/internal/database"
	clients2 "github.com/central-university-dev/go-Matthew11K/internal/scrapper/clients"
	"github.com/central-university-dev/go-Matthew11K/internal/scrapper/notify"
	"github.com/central-university-dev/go-Matthew11K/internal/scrapper/repository"
	"github.com/central-university-dev/go-Matthew11K/internal/scrapper/scheduler"

	"log/slog"

	"github.com/central-university-dev/go-Matthew11K/internal/api/openapi/v1/v1_scrapper"
	"github.com/central-university-dev/go-Matthew11K/internal/config"
	"github.com/central-university-dev/go-Matthew11K/internal/scrapper/handler"
	"github.com/central-university-dev/go-Matthew11K/internal/scrapper/service"
	"github.com/central-university-dev/go-Matthew11K/pkg"
)

func gracefulShutdown(server *http.Server, scheduler interface {
	Stop()
}, stopCh <-chan struct{}, appLogger *slog.Logger) {
	<-stopCh
	appLogger.Info("Получен сигнал завершения")

	scheduler.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		appLogger.Error("Ошибка при остановке HTTP сервера",
			"error", err,
		)
	}

	appLogger.Info("Сервер успешно остановлен")
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
	appLogger := pkg.NewLogger(os.Stdout)

	cfg := config.LoadConfig()

	ctx := context.Background()
	db, err := database.NewPostgresDB(ctx, cfg, appLogger)
	if err != nil {
		appLogger.Error("Ошибка при подключении к базе данных",
			"error", err,
		)
		os.Exit(1)
	}
	defer db.Close()

	repoFactory := repository.NewFactory(db, cfg, appLogger)

	linkRepo, err := repoFactory.CreateLinkRepository()
	if err != nil {
		appLogger.Error("Ошибка при создании репозитория ссылок",
			"error", err,
		)
		os.Exit(1)
	}

	chatRepo, err := repoFactory.CreateChatRepository()
	if err != nil {
		appLogger.Error("Ошибка при создании репозитория чатов",
			"error", err,
		)
		os.Exit(1)
	}

	githubDetailsRepo, err := repoFactory.CreateGitHubDetailsRepository()
	if err != nil {
		appLogger.Error("Ошибка при создании репозитория деталей GitHub",
			"error", err,
		)
		os.Exit(1)
	}

	stackOverflowDetailsRepo, err := repoFactory.CreateStackOverflowDetailsRepository()
	if err != nil {
		appLogger.Error("Ошибка при создании репозитория деталей StackOverflow",
			"error", err,
		)
		os.Exit(1)
	}

	botNotifier, err := notify.NewHTTPBotNotifier(cfg.BotBaseURL, appLogger)
	if err != nil {
		appLogger.Error("Ошибка при создании нотификатора бота",
			"error", err,
		)
		os.Exit(1)
	}

	githubClient := clients2.NewGitHubClient(cfg.GitHubAPIToken, "")
	stackoverflowClient := clients2.NewStackOverflowClient(cfg.StackOverflowAPIToken, "")
	linkAnalyzer := common.NewLinkAnalyzer()

	updaterFactory := common.NewLinkUpdaterFactory(githubClient, stackoverflowClient)

	scrapperService := service.NewScrapperService(
		linkRepo,
		chatRepo,
		botNotifier,
		githubDetailsRepo,
		stackOverflowDetailsRepo,
		updaterFactory,
		linkAnalyzer,
		appLogger,
	)

	tagService := service.NewTagService(linkRepo, chatRepo, appLogger)

	scrapperHandler := handler.NewScrapperHandler(scrapperService, tagService)

	server, err := v1_scrapper.NewServer(scrapperHandler)
	if err != nil {
		appLogger.Error("Ошибка при создании сервера",
			"error", err,
		)
		os.Exit(1)
	}

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
		appLogger.Info("Использование обычного шедулера")
		appLogger.Error("Обычный шедулер больше не поддерживается в текущей конфигурации")
		os.Exit(1)
	}

	sch.Start()

	httpServer := &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.ScrapperServerPort),
		Handler:           server,
		ReadHeaderTimeout: 10 * time.Second,
	}

	stopCh := make(chan struct{})

	startHTTPServer(httpServer, cfg.ScrapperServerPort, stopCh, appLogger)

	gracefulShutdown(httpServer, sch, stopCh, appLogger)
}
