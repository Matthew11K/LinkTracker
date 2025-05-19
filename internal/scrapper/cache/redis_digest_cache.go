package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
)

type RedisDigestCache struct {
	client     *redis.Client
	ttl        time.Duration
	logger     *slog.Logger
	keyPattern string
}

func NewRedisDigestCache(
	ctx context.Context,
	redisURL, password string,
	db int,
	ttl time.Duration,
	logger *slog.Logger,
) (*RedisDigestCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     redisURL,
		Password: password,
		DB:       db,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ошибка при подключении к Redis: %w", err)
	}

	logger.Info("Соединение с Redis для кэша дайджестов успешно установлено")

	return &RedisDigestCache{
		client:     client,
		ttl:        ttl,
		logger:     logger,
		keyPattern: "digest:updates:%d",
	}, nil
}

func (c *RedisDigestCache) AddUpdate(ctx context.Context, chatID int64, update *models.LinkUpdate) error {
	key := fmt.Sprintf(c.keyPattern, chatID)

	updates, err := c.GetUpdates(ctx, chatID)
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("ошибка при получении текущих обновлений: %w", err)
	}

	updates = append(updates, update)

	data, err := json.Marshal(updates)
	if err != nil {
		return fmt.Errorf("ошибка при сериализации данных для Redis: %w", err)
	}

	if err := c.client.Set(ctx, key, data, c.ttl).Err(); err != nil {
		return fmt.Errorf("ошибка при сохранении обновлений в Redis: %w", err)
	}

	c.logger.Info("Обновление добавлено в дайджест",
		"chatID", chatID,
		"linkID", update.ID,
		"url", update.URL,
		"count", len(updates),
	)

	return nil
}

func (c *RedisDigestCache) GetUpdates(ctx context.Context, chatID int64) ([]*models.LinkUpdate, error) {
	key := fmt.Sprintf(c.keyPattern, chatID)

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return []*models.LinkUpdate{}, nil
		}

		return nil, fmt.Errorf("ошибка при получении обновлений из Redis: %w", err)
	}

	var updates []*models.LinkUpdate
	if err := json.Unmarshal(data, &updates); err != nil {
		return nil, fmt.Errorf("ошибка при десериализации данных из Redis: %w", err)
	}

	c.logger.Info("Накопленные обновления получены",
		"chatID", chatID,
		"count", len(updates),
	)

	return updates, nil
}

func (c *RedisDigestCache) ClearUpdates(ctx context.Context, chatID int64) error {
	key := fmt.Sprintf(c.keyPattern, chatID)

	if err := c.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("ошибка при удалении обновлений из Redis: %w", err)
	}

	c.logger.Info("Накопленные обновления удалены",
		"chatID", chatID,
	)

	return nil
}

func (c *RedisDigestCache) GetAllChatIDs(ctx context.Context) ([]int64, error) {
	pattern := "digest:updates:*"
	keys, err := c.client.Keys(ctx, pattern).Result()

	if err != nil {
		return nil, fmt.Errorf("ошибка при получении ключей из Redis: %w", err)
	}

	var chatIDs []int64

	for _, key := range keys {
		var chatID int64
		if _, err := fmt.Sscanf(key, "digest:updates:%d", &chatID); err == nil {
			chatIDs = append(chatIDs, chatID)
		}
	}

	c.logger.Info("Получены ID чатов с накопленными обновлениями",
		"count", len(chatIDs),
	)

	return chatIDs, nil
}

func (c *RedisDigestCache) Close() error {
	return c.client.Close()
}
