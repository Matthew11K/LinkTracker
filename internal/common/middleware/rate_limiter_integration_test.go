package middleware_test

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	postgresdriver "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/central-university-dev/go-Matthew11K/internal/api/openapi/v1_bot"
	"github.com/central-university-dev/go-Matthew11K/internal/bot/cache"
	"github.com/central-university-dev/go-Matthew11K/internal/bot/clients"
	"github.com/central-university-dev/go-Matthew11K/internal/bot/domain"
	"github.com/central-university-dev/go-Matthew11K/internal/bot/domain/mocks"
	bothandler "github.com/central-university-dev/go-Matthew11K/internal/bot/handler"
	"github.com/central-university-dev/go-Matthew11K/internal/bot/repository"
	botservice "github.com/central-university-dev/go-Matthew11K/internal/bot/service"
	"github.com/central-university-dev/go-Matthew11K/internal/common"
	"github.com/central-university-dev/go-Matthew11K/internal/common/middleware"
	"github.com/central-university-dev/go-Matthew11K/internal/config"
	"github.com/central-university-dev/go-Matthew11K/internal/database"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
	"github.com/central-university-dev/go-Matthew11K/pkg/txs"
)

func TestRateLimiterRealIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Пропуск интеграционного теста (используйте -short=false)")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	logger.Info("Запуск Postgres контейнера")

	postgresContainer, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err)

	defer func() {
		if err := postgresContainer.Terminate(ctx); err != nil {
			t.Logf("Ошибка при остановке PostgreSQL контейнера: %v", err)
		}
	}()

	postgresEndpoint, err := postgresContainer.Endpoint(ctx, "")
	require.NoError(t, err)

	dbURL := fmt.Sprintf("postgres://testuser:testpass@%s/testdb?sslmode=disable", postgresEndpoint)

	logger.Info("Применение миграций")

	err = runMigrations(dbURL)
	require.NoError(t, err)

	logger.Info("Запуск Redis контейнера")

	redisContainer, err := redis.Run(ctx,
		"redis:7-alpine",
		redis.WithLogLevel(redis.LogLevelVerbose),
	)
	require.NoError(t, err)

	defer func() {
		if err := redisContainer.Terminate(ctx); err != nil {
			t.Logf("Ошибка при остановке Redis контейнера: %v", err)
		}
	}()

	redisEndpoint, err := redisContainer.Endpoint(ctx, "")
	require.NoError(t, err)

	cfg := &config.Config{
		DatabaseURL:        dbURL,
		DatabaseAccessType: config.SQLAccess,
		DatabaseBatchSize:  100,
		DatabaseMaxConn:    10,
		ScrapperBaseURL:    "http://localhost:8080",
		RedisURL:           redisEndpoint,
		RedisPassword:      "",
		RedisDB:            0,
		RedisCacheTTL:      time.Hour,
		RateLimitRequests:  2,
		RateLimitWindow:    time.Second,
		HTTPRequestTimeout: 10 * time.Second,
	}

	logger.Info("Инициализация тестового приложения")
	app, err := initializeTestApplication(ctx, t, cfg, logger)
	require.NoError(t, err)

	defer app.Close()

	logger.Info("Запуск тестового HTTP сервера")

	serverURL, stopServer := startTestServer(t, app.handler)
	defer stopServer()

	testEndpoint := serverURL + "/updates"
	client := &http.Client{Timeout: 30 * time.Second}

	logger.Info("Выполнение первого запроса (должен пройти)")

	resp, err := makeRealTestRequest(client, testEndpoint)
	require.NoError(t, err)
	assert.NotEqual(t, http.StatusTooManyRequests, resp.StatusCode,
		"Первый запрос должен проходить")
	resp.Body.Close()

	logger.Info("Выполнение второго запроса (должен пройти)")

	resp, err = makeRealTestRequest(client, testEndpoint)
	require.NoError(t, err)
	assert.NotEqual(t, http.StatusTooManyRequests, resp.StatusCode,
		"Второй запрос должен проходить")
	resp.Body.Close()

	logger.Info("Выполнение третьего запроса (должен быть заблокирован)")

	resp, err = makeRealTestRequest(client, testEndpoint)
	require.NoError(t, err)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode,
		"Третий запрос должен быть заблокирован rate limiter")

	retryAfter := resp.Header.Get("Retry-After")
	assert.NotEmpty(t, retryAfter, "Должен быть заголовок Retry-After")

	retrySeconds, parseErr := strconv.Atoi(retryAfter)
	assert.NoError(t, parseErr, "Retry-After должен быть числом")
	assert.True(t, retrySeconds > 0 && retrySeconds <= 2,
		"Retry-After должен быть в разумных пределах (1-2 сек)")
	resp.Body.Close()

	time.Sleep(time.Second + 200*time.Millisecond)

	resp, err = makeRealTestRequest(client, testEndpoint)
	require.NoError(t, err)
	assert.NotEqual(t, http.StatusTooManyRequests, resp.StatusCode,
		"После сброса лимита запрос должен проходить")
	resp.Body.Close()

	logger.Info("Интеграционный тест rate limiting успешно завершен")
}

type testApplication struct {
	handler     http.Handler
	rateLimiter *middleware.RateLimiterMiddleware
	closers     []func() error
}

func (app *testApplication) Close() {
	for _, closer := range app.closers {
		_ = closer()
	}
}

func initializeTestApplication(ctx context.Context, t *testing.T, cfg *config.Config, logger *slog.Logger) (*testApplication, error) {
	t.Helper()

	var closers []func() error

	db, err := database.NewPostgresDB(ctx, cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к базе данных: %w", err)
	}

	closers = append(closers, func() error {
		db.Close()
		return nil
	})

	txManager := txs.NewTxManager(db.Pool, logger)

	repoFactory := repository.NewFactory(db, cfg, logger)

	chatStateRepo, err := repoFactory.CreateChatStateRepository()
	if err != nil {
		return nil, fmt.Errorf("ошибка создания репозитория состояний чата: %w", err)
	}

	scrapperClient, err := clients.NewScrapperClient(cfg.ScrapperBaseURL, cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания клиента скраппера: %w", err)
	}

	telegramClient := mocks.NewTelegramClientAPI(t)

	telegramClient.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	telegramClient.On("GetUpdates", mock.Anything, mock.Anything).Return([]domain.Update{}, nil).Maybe()
	telegramClient.On("SendUpdate", mock.Anything, mock.Anything).Return(nil).Maybe()
	telegramClient.On("SetMyCommands", mock.Anything, mock.Anything).Return(nil).Maybe()
	telegramClient.On("GetBot").Return(nil).Maybe()

	linkAnalyzer := common.NewLinkAnalyzer()

	baseBotService := botservice.NewBotService(
		chatStateRepo,
		scrapperClient,
		telegramClient,
		linkAnalyzer,
		txManager,
	)

	var linkUpdater interface {
		SendLinkUpdate(ctx context.Context, update *models.LinkUpdate) error
	} = baseBotService

	if cfg.RedisURL != "" {
		redisCache, err := cache.NewRedisLinkCache(
			ctx,
			cfg.RedisURL,
			cfg.RedisPassword,
			cfg.RedisDB,
			cfg.RedisCacheTTL,
			logger,
		)
		if err != nil {
			logger.Warn("Не удалось подключиться к Redis, продолжаем без кеша", "error", err)
		} else {
			closers = append(closers, func() error {
				return redisCache.Close()
			})

			cachedService := botservice.NewCachedBotService(baseBotService, redisCache, logger)
			linkUpdater = cachedService
		}
	}

	botHandler := bothandler.NewBotHandler(linkUpdater)

	server, err := v1_bot.NewServer(botHandler)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания OpenAPI сервера: %w", err)
	}

	rateLimiter := middleware.NewRateLimiterMiddleware(
		cfg.RateLimitRequests,
		cfg.RateLimitWindow,
		logger,
	)

	closers = append(closers, func() error {
		return rateLimiter.Close()
	})

	handler := rateLimiter.Middleware(server)

	return &testApplication{
		handler:     handler,
		rateLimiter: rateLimiter,
		closers:     closers,
	}, nil
}

func startTestServer(t *testing.T, handler http.Handler) (serverURL string, stopFunc func()) {
	t.Helper()

	listener, err := net.Listen("tcp", ":0") //nolint:gosec // G102: Использование localhost:0 безопасно для тестов
	require.NoError(t, err)

	port := listener.Addr().(*net.TCPAddr).Port
	serverURL = fmt.Sprintf("http://localhost:%d", port)

	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			t.Errorf("Ошибка HTTP сервера: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	stopFunc = func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_ = server.Shutdown(ctx)
		_ = listener.Close()
	}

	return serverURL, stopFunc
}

func runMigrations(databaseURL string) error {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return fmt.Errorf("не удалось открыть соединение с базой данных: %w", err)
	}
	defer db.Close()

	driver, err := postgresdriver.WithInstance(db, &postgresdriver.Config{})
	if err != nil {
		return fmt.Errorf("не удалось создать драйвер миграций: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://../../../migrations",
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("не удалось создать инстанс миграций: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("не удалось применить миграции: %w", err)
	}

	return nil
}

func makeRealTestRequest(client *http.Client, url string) (*http.Response, error) {
	body := `{
		"id": 1,
		"url": "https://github.com/example/repo",
		"tgChatIds": [123456789],
		"description": "Integration test update"
	}`

	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "integration-test")

	return client.Do(req)
}
