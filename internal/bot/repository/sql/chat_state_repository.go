package sql

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/database"
	customerrors "github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
	"github.com/central-university-dev/go-Matthew11K/pkg/txs"
	"github.com/jackc/pgx/v5"
)

type ChatStateRepository struct {
	db        *database.PostgresDB
	txManager *txs.TxManager
}

func NewChatStateRepository(db *database.PostgresDB, txManager *txs.TxManager) *ChatStateRepository {
	return &ChatStateRepository{db: db, txManager: txManager}
}

func (r *ChatStateRepository) GetState(ctx context.Context, chatID int64) (models.ChatState, error) {
	var state int

	err := r.db.Pool.QueryRow(ctx, "SELECT state FROM chat_states WHERE chat_id = $1", chatID).Scan(&state)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Если состояние не найдено, возвращаем StateIdle (0) по умолчанию
			return models.StateIdle, nil
		}

		return models.StateIdle, &customerrors.ErrSQLExecution{Operation: "получение состояния чата", Cause: err}
	}

	return models.ChatState(state), nil
}

func (r *ChatStateRepository) SetState(ctx context.Context, chatID int64, state models.ChatState) error {
	return r.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		querier := txs.GetQuerier(ctx, r.db.Pool)

		var chatExists bool

		err := querier.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM chats WHERE id = $1)", chatID).Scan(&chatExists)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "проверка существования чата", Cause: err}
		}

		if !chatExists {
			now := time.Now()

			_, err = querier.Exec(ctx, "INSERT INTO chats (id, created_at, updated_at) VALUES ($1, $2, $3)",
				chatID, now, now)
			if err != nil {
				return &customerrors.ErrSQLExecution{Operation: "создание чата", Cause: err}
			}
		}

		var stateExists bool

		err = querier.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM chat_states WHERE chat_id = $1)", chatID).Scan(&stateExists)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "проверка существования состояния", Cause: err}
		}

		now := time.Now()
		if stateExists {
			_, err = querier.Exec(ctx, "UPDATE chat_states SET state = $1, updated_at = $2 WHERE chat_id = $3",
				int(state), now, chatID)
			if err != nil {
				return &customerrors.ErrSQLExecution{Operation: "обновление состояния чата", Cause: err}
			}
		} else {
			_, err = querier.Exec(ctx, "INSERT INTO chat_states (chat_id, state, created_at, updated_at) VALUES ($1, $2, $3, $4)",
				chatID, int(state), now, now)
			if err != nil {
				return &customerrors.ErrSQLExecution{Operation: "создание состояния чата", Cause: err}
			}
		}

		return nil
	})
}

func (r *ChatStateRepository) GetData(ctx context.Context, chatID int64, key string) (interface{}, error) {
	var valueStr string

	err := r.db.Pool.QueryRow(ctx, "SELECT value FROM chat_state_data WHERE chat_id = $1 AND key = $2",
		chatID, key).Scan(&valueStr)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Возвращаем nil, если данные не найдены
		}

		return nil, &customerrors.ErrSQLExecution{Operation: "получение данных чата", Cause: err}
	}

	var value interface{}

	err = json.Unmarshal([]byte(valueStr), &value)
	if err != nil {
		return nil, &customerrors.ErrSQLScan{Entity: "данные чата", Cause: err}
	}

	return value, nil
}

func (r *ChatStateRepository) SetData(ctx context.Context, chatID int64, key string, value interface{}) error {
	return r.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		querier := txs.GetQuerier(ctx, r.db.Pool)

		var chatExists bool

		err := querier.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM chats WHERE id = $1)", chatID).Scan(&chatExists)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "проверка существования чата", Cause: err}
		}

		if !chatExists {
			now := time.Now()

			_, err = querier.Exec(ctx, "INSERT INTO chats (id, created_at, updated_at) VALUES ($1, $2, $3)",
				chatID, now, now)
			if err != nil {
				return &customerrors.ErrSQLExecution{Operation: "создание чата", Cause: err}
			}
		}

		valueJSON, err := json.Marshal(value)
		if err != nil {
			return &customerrors.ErrInvalidArgument{Message: "не удалось сериализовать значение в JSON"}
		}

		now := time.Now()

		_, err = querier.Exec(ctx, `
			INSERT INTO chat_state_data (chat_id, key, value, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (chat_id, key) DO UPDATE SET
				value = EXCLUDED.value,
				updated_at = EXCLUDED.updated_at
		`, chatID, key, string(valueJSON), now, now)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "сохранение данных чата", Cause: err}
		}

		return nil
	})
}

func (r *ChatStateRepository) ClearData(ctx context.Context, chatID int64) error {
	_, err := r.db.Pool.Exec(ctx, "DELETE FROM chat_state_data WHERE chat_id = $1", chatID)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "удаление данных чата", Cause: err}
	}

	return nil
}
