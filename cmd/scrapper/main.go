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
	"github.com/central-university-dev/go-Matthew11K/internal/api/openapi/v1/v1_scrapper"
	appservices "github.com/central-university-dev/go-Matthew11K/internal/application/services"
	"github.com/central-university-dev/go-Matthew11K/internal/config"
	domainservices "github.com/central-university-dev/go-Matthew11K/internal/domain/services"
	"github.com/central-university-dev/go-Matthew11K/internal/infrastructure/clients"
	"github.com/central-university-dev/go-Matthew11K/internal/infrastructure/repositories/memory"
	"github.com/central-university-dev/go-Matthew11K/internal/scheduler"
	"github.com/central-university-dev/go-Matthew11K/pkg"
	"github.com/joho/godotenv"
)

func gracefulShutdown(server *http.Server, scheduler *scheduler.Scheduler, appLogger *slog.Logger) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	appLogger.Info("Получен сигнал завершения",
		"signal", sig.String(),
	)

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

func startInitialCheck(scrapperService *appservices.ScrapperService, appLogger *slog.Logger) {
	go func() {
		time.Sleep(10 * time.Second)
		appLogger.Info("Запуск первоначальной проверки обновлений")

		ctx := context.Background()
		if err := scrapperService.CheckUpdates(ctx); err != nil {
			appLogger.Error("Ошибка при первоначальной проверке обновлений",
				"error", err,
			)
		}
	}()
}

func startHTTPServer(server *http.Server, port int, appLogger *slog.Logger) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		appLogger.Info("Запуск HTTP сервера скраппера",
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

	linkRepo := memory.NewLinkRepository()
	chatRepo := memory.NewChatRepository()

	botClient, err := clients.NewBotAPIClient(cfg.BotBaseURL)
	if err != nil {
		appLogger.Error("Ошибка при создании клиента бота",
			"error", err,
		)
		os.Exit(1)
	}

	githubClient := clients.NewGitHubClient(cfg.GitHubAPIToken, "")
	stackoverflowClient := clients.NewStackOverflowClient(cfg.StackOverflowAPIToken, "")
	linkAnalyzer := domainservices.NewLinkAnalyzer()

	scrapperService := appservices.NewScrapperService(
		linkRepo,
		chatRepo,
		botClient,
		githubClient,
		stackoverflowClient,
		linkAnalyzer,
		appLogger,
	)

	scrapperHandler := handlers.NewScrapperHandler(scrapperService)

	server, err := v1_scrapper.NewServer(scrapperHandler)
	if err != nil {
		appLogger.Error("Ошибка при создании сервера",
			"error", err,
		)
		os.Exit(1)
	}

	sch := scheduler.NewScheduler(scrapperService, cfg.SchedulerCheckInterval, appLogger)
	startInitialCheck(scrapperService, appLogger)

	httpServer := &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.ScrapperServerPort),
		Handler:           server,
		ReadHeaderTimeout: 10 * time.Second,
	}

	startHTTPServer(httpServer, cfg.ScrapperServerPort, appLogger)
	sch.Start()

	gracefulShutdown(httpServer, sch, appLogger)
}
