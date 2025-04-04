package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/config"

	// file driver необходим для миграций базы данных.
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"

	// pgx/stdlib нужен для регистрации драйвера pgx в базе данных.
	_ "github.com/jackc/pgx/v5/stdlib"
)

const maxInt32 = 1<<31 - 1

type PostgresDB struct {
	Pool   *pgxpool.Pool
	Config *config.Config
	Logger *slog.Logger
}

func NewPostgresDB(ctx context.Context, cfg *config.Config, logger *slog.Logger) (*PostgresDB, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("ошибка при парсинге строки подключения к PostgreSQL: %w", err)
	}

	var maxConns int32

	switch {
	case cfg.DatabaseMaxConn <= 0:
		maxConns = 0
	case cfg.DatabaseMaxConn >= maxInt32:
		maxConns = maxInt32
	default:
		maxConns = int32(cfg.DatabaseMaxConn)
	}

	poolConfig.MaxConns = maxConns

	poolConfig.ConnConfig.ConnectTimeout = 5 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("ошибка при создании пула соединений PostgreSQL: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ошибка при проверке соединения с PostgreSQL: %w", err)
	}

	logger.Info("Соединение с PostgreSQL успешно установлено")

	return &PostgresDB{
		Pool:   pool,
		Config: cfg,
		Logger: logger,
	}, nil
}

func (db *PostgresDB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
		db.Logger.Info("Соединение с PostgreSQL закрыто")
	}
}
