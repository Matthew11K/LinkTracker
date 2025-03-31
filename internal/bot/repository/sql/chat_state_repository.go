package sql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/database"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
	"github.com/jackc/pgx/v5"
)

type ChatStateRepository struct {
	db *database.PostgresDB
}

func NewChatStateRepository(db *database.PostgresDB) *ChatStateRepository {
	return &ChatStateRepository{db: db}
}

func (r *ChatStateRepository) GetState(ctx context.Context, chatID int64) (models.ChatState, error) {
	var state int

	err := r.db.Pool.QueryRow(ctx, "SELECT state FROM chat_states WHERE chat_id = $1", chatID).Scan(&state)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Если состояние не найдено, возвращаем StateIdle (0) по умолчанию
			return models.StateIdle, nil
		}

		return models.StateIdle, fmt.Errorf("ошибка при получении состояния чата: %w", err)
	}

	return models.ChatState(state), nil
}

func (r *ChatStateRepository) SetState(ctx context.Context, chatID int64, state models.ChatState) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("ошибка при начале транзакции: %w", err)
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	var chatExists bool

	err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM chats WHERE id = $1)", chatID).Scan(&chatExists)
	if err != nil {
		return fmt.Errorf("ошибка при проверке существования чата: %w", err)
	}

	if !chatExists {
		now := time.Now()

		_, err = tx.Exec(ctx, "INSERT INTO chats (id, created_at, updated_at) VALUES ($1, $2, $3)",
			chatID, now, now)
		if err != nil {
			return fmt.Errorf("ошибка при создании чата: %w", err)
		}
	}

	var stateExists bool

	err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM chat_states WHERE chat_id = $1)", chatID).Scan(&stateExists)
	if err != nil {
		return fmt.Errorf("ошибка при проверке существования состояния: %w", err)
	}

	now := time.Now()
	if stateExists {
		_, err = tx.Exec(ctx, "UPDATE chat_states SET state = $1, updated_at = $2 WHERE chat_id = $3",
			int(state), now, chatID)
		if err != nil {
			return fmt.Errorf("ошибка при обновлении состояния чата: %w", err)
		}
	} else {
		_, err = tx.Exec(ctx, "INSERT INTO chat_states (chat_id, state, created_at, updated_at) VALUES ($1, $2, $3, $4)",
			chatID, int(state), now, now)
		if err != nil {
			return fmt.Errorf("ошибка при создании состояния чата: %w", err)
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("ошибка при фиксации транзакции: %w", err)
	}

	return nil
}

func (r *ChatStateRepository) GetData(ctx context.Context, chatID int64, key string) (interface{}, error) {
	var valueStr string

	err := r.db.Pool.QueryRow(ctx, "SELECT value FROM chat_state_data WHERE chat_id = $1 AND key = $2",
		chatID, key).Scan(&valueStr)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Возвращаем nil, если данные не найдены
		}

		return nil, fmt.Errorf("ошибка при получении данных чата: %w", err)
	}

	var value interface{}

	err = json.Unmarshal([]byte(valueStr), &value)
	if err != nil {
		return nil, fmt.Errorf("ошибка при десериализации данных: %w", err)
	}

	return value, nil
}

func (r *ChatStateRepository) SetData(ctx context.Context, chatID int64, key string, value interface{}) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("ошибка при начале транзакции: %w", err)
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	var chatExists bool

	err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM chats WHERE id = $1)", chatID).Scan(&chatExists)
	if err != nil {
		return fmt.Errorf("ошибка при проверке существования чата: %w", err)
	}

	if !chatExists {
		now := time.Now()

		_, err = tx.Exec(ctx, "INSERT INTO chats (id, created_at, updated_at) VALUES ($1, $2, $3)",
			chatID, now, now)
		if err != nil {
			return fmt.Errorf("ошибка при создании чата: %w", err)
		}
	}

	valueJSON, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("ошибка при сериализации значения в JSON: %w", err)
	}

	now := time.Now()

	_, err = tx.Exec(ctx, `
		INSERT INTO chat_state_data (chat_id, key, value, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (chat_id, key) DO UPDATE SET
			value = EXCLUDED.value,
			updated_at = EXCLUDED.updated_at
	`, chatID, key, string(valueJSON), now, now)
	if err != nil {
		return fmt.Errorf("ошибка при сохранении данных чата: %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("ошибка при фиксации транзакции: %w", err)
	}

	return nil
}

func (r *ChatStateRepository) ClearData(ctx context.Context, chatID int64) error {
	_, err := r.db.Pool.Exec(ctx, "DELETE FROM chat_state_data WHERE chat_id = $1", chatID)
	if err != nil {
		return fmt.Errorf("ошибка при удалении данных чата: %w", err)
	}

	return nil
}
