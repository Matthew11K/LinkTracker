package cache_test

import (
	"context"
	"testing"
	"time"

	"log/slog"
	"os"

	"github.com/central-university-dev/go-Matthew11K/internal/bot/cache"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestRedisCache(t *testing.T) {
	if testing.Short() {
		t.Skip("Пропускаем интеграционный тест в коротком режиме")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	redisC, redisPort := startRedisContainer(t)
	defer func() {
		if err := redisC.Terminate(context.Background()); err != nil {
			t.Logf("Ошибка при остановке Redis контейнера: %v", err)
		}
	}()

	redisURL := "localhost:" + redisPort
	ttl := 30 * time.Second
	redisCache, err := cache.NewRedisLinkCache(redisURL, "", 0, ttl, logger)
	require.NoError(t, err)

	defer redisCache.Close()

	ctx := context.Background()
	chatID := int64(123456789)

	links := []*models.Link{
		{
			ID:          1,
			URL:         "https://github.com/user1/repo1",
			Type:        models.GitHub,
			Tags:        []string{"go", "backend"},
			Filters:     []string{"user:user1"},
			LastChecked: time.Now(),
			LastUpdated: time.Now(),
			CreatedAt:   time.Now(),
		},
		{
			ID:          2,
			URL:         "https://stackoverflow.com/questions/12345",
			Type:        models.StackOverflow,
			Tags:        []string{"golang", "database"},
			Filters:     []string{"user:user2"},
			LastChecked: time.Now(),
			LastUpdated: time.Now(),
			CreatedAt:   time.Now(),
		},
	}

	cachedLinks, err := redisCache.GetLinks(ctx, chatID)
	require.NoError(t, err)
	assert.Nil(t, cachedLinks)

	err = redisCache.SetLinks(ctx, chatID, links)
	require.NoError(t, err)

	cachedLinks, err = redisCache.GetLinks(ctx, chatID)
	require.NoError(t, err)
	require.NotNil(t, cachedLinks)
	require.Len(t, cachedLinks, 2)

	assert.Equal(t, links[0].ID, cachedLinks[0].ID)
	assert.Equal(t, links[0].URL, cachedLinks[0].URL)
	assert.Equal(t, links[0].Type, cachedLinks[0].Type)
	assert.Equal(t, links[0].Tags, cachedLinks[0].Tags)
	assert.Equal(t, links[0].Filters, cachedLinks[0].Filters)

	assert.Equal(t, links[1].ID, cachedLinks[1].ID)
	assert.Equal(t, links[1].URL, cachedLinks[1].URL)
	assert.Equal(t, links[1].Type, cachedLinks[1].Type)
	assert.Equal(t, links[1].Tags, cachedLinks[1].Tags)
	assert.Equal(t, links[1].Filters, cachedLinks[1].Filters)

	err = redisCache.DeleteLinks(ctx, chatID)
	require.NoError(t, err)

	cachedLinks, err = redisCache.GetLinks(ctx, chatID)
	require.NoError(t, err)
	assert.Nil(t, cachedLinks)

	err = redisCache.SetLinks(ctx, chatID, links)
	require.NoError(t, err)

	shortTTLCache, err := cache.NewRedisLinkCache(redisURL, "", 0, 1*time.Second, logger)
	require.NoError(t, err)
	defer shortTTLCache.Close()

	err = shortTTLCache.SetLinks(ctx, chatID+1, links)
	require.NoError(t, err)

	cachedLinks, err = shortTTLCache.GetLinks(ctx, chatID+1)
	require.NoError(t, err)
	require.NotNil(t, cachedLinks)
	require.Len(t, cachedLinks, 2)

	time.Sleep(2 * time.Second)

	cachedLinks, err = shortTTLCache.GetLinks(ctx, chatID+1)
	require.NoError(t, err)
	assert.Nil(t, cachedLinks)
}

func startRedisContainer(t *testing.T) (container testcontainers.Container, port string) {
	ctx := context.Background()

	redisC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "redis:alpine",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForLog("Ready to accept connections"),
		},
		Started: true,
	})
	require.NoError(t, err)

	mappedPort, err := redisC.MappedPort(ctx, "6379")
	require.NoError(t, err)

	return redisC, mappedPort.Port()
}
