package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
)

type LinkCache interface {
	GetLinks(ctx context.Context, chatID int64) ([]*models.Link, error)
	SetLinks(ctx context.Context, chatID int64, links []*models.Link) error
	DeleteLinks(ctx context.Context, chatID int64) error
}

type RedisLinkCache struct {
	client *redis.Client
	ttl    time.Duration
	logger *slog.Logger
}

func NewRedisLinkCache(redisURL, password string, db int, ttl time.Duration, logger *slog.Logger) (*RedisLinkCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     redisURL,
		Password: password,
		DB:       db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ошибка при подключении к Redis: %w", err)
	}

	logger.Info("Соединение с Redis успешно установлено")

	return &RedisLinkCache{
		client: client,
		ttl:    ttl,
		logger: logger,
	}, nil
}

func (c *RedisLinkCache) GetLinks(ctx context.Context, chatID int64) ([]*models.Link, error) {
	key := fmt.Sprintf("links:%d", chatID)

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			c.logger.Debug("Кэш не найден",
				"chatID", chatID,
			)

			return nil, nil
		}

		c.logger.Error("Ошибка при получении данных из Redis",
			"error", err,
			"chatID", chatID,
		)

		return nil, fmt.Errorf("ошибка при получении данных из Redis: %w", err)
	}

	var links []*models.Link
	if err := json.Unmarshal(data, &links); err != nil {
		c.logger.Error("Ошибка при десериализации данных из Redis",
			"error", err,
			"chatID", chatID,
		)

		return nil, fmt.Errorf("ошибка при десериализации данных из Redis: %w", err)
	}

	c.logger.Info("Данные успешно получены из кэша",
		"chatID", chatID,
		"count", len(links),
	)

	return links, nil
}

func (c *RedisLinkCache) SetLinks(ctx context.Context, chatID int64, links []*models.Link) error {
	key := fmt.Sprintf("links:%d", chatID)

	data, err := json.Marshal(links)
	if err != nil {
		c.logger.Error("Ошибка при сериализации данных для Redis",
			"error", err,
			"chatID", chatID,
		)

		return fmt.Errorf("ошибка при сериализации данных для Redis: %w", err)
	}

	if err := c.client.Set(ctx, key, data, c.ttl).Err(); err != nil {
		c.logger.Error("Ошибка при сохранении данных в Redis",
			"error", err,
			"chatID", chatID,
		)

		return fmt.Errorf("ошибка при сохранении данных в Redis: %w", err)
	}

	c.logger.Info("Данные успешно сохранены в кэш",
		"chatID", chatID,
		"count", len(links),
		"ttl", c.ttl,
	)

	return nil
}

func (c *RedisLinkCache) DeleteLinks(ctx context.Context, chatID int64) error {
	key := fmt.Sprintf("links:%d", chatID)

	if err := c.client.Del(ctx, key).Err(); err != nil {
		c.logger.Error("Ошибка при удалении данных из Redis",
			"error", err,
			"chatID", chatID,
		)

		return fmt.Errorf("ошибка при удалении данных из Redis: %w", err)
	}

	c.logger.Info("Данные успешно удалены из кэша",
		"chatID", chatID,
	)

	return nil
}

func (c *RedisLinkCache) Close() error {
	return c.client.Close()
}
