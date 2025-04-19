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
	db *database.PostgresDB
}

func NewChatStateRepository(db *database.PostgresDB) *ChatStateRepository {
	return &ChatStateRepository{db: db}
}

func (r *ChatStateRepository) GetState(ctx context.Context, chatID int64) (models.ChatState, error) {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	var state int

	err := querier.QueryRow(ctx, "SELECT state FROM chat_states WHERE chat_id = $1", chatID).Scan(&state)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.StateIdle, &customerrors.ErrChatStateNotFound{ChatID: chatID}
		}

		return models.StateIdle, &customerrors.ErrSQLExecution{Operation: customerrors.OpGetChatState, Cause: err}
	}

	return models.ChatState(state), nil
}

func (r *ChatStateRepository) SetState(ctx context.Context, chatID int64, state models.ChatState) error {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	now := time.Now()

	_, err := querier.Exec(ctx, `
		INSERT INTO chat_states (chat_id, state, created_at, updated_at) 
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (chat_id) DO UPDATE SET
			state = $2,
			updated_at = $4
	`, chatID, int(state), now, now)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: customerrors.OpSetChatState, Cause: err}
	}

	return nil
}

func (r *ChatStateRepository) GetData(ctx context.Context, chatID int64, key string) (any, error) {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	var valueStr string

	err := querier.QueryRow(ctx, "SELECT value FROM chat_state_data WHERE chat_id = $1 AND key = $2",
		chatID, key).Scan(&valueStr)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &customerrors.ErrChatStateNotFound{ChatID: chatID}
		}

		return nil, &customerrors.ErrSQLExecution{Operation: customerrors.OpGetChatStateData, Cause: err}
	}

	var value any

	err = json.Unmarshal([]byte(valueStr), &value)
	if err != nil {
		return nil, &customerrors.ErrSQLScan{Entity: "данные чата", Cause: err}
	}

	return value, nil
}

func (r *ChatStateRepository) SetData(ctx context.Context, chatID int64, key string, value any) error {
	querier := txs.GetQuerier(ctx, r.db.Pool)

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
		return &customerrors.ErrSQLExecution{Operation: customerrors.OpSetChatStateData, Cause: err}
	}

	return nil
}

func (r *ChatStateRepository) ClearData(ctx context.Context, chatID int64) error {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	_, err := querier.Exec(ctx, "DELETE FROM chat_state_data WHERE chat_id = $1", chatID)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: customerrors.OpClearChatStateData, Cause: err}
	}

	return nil
}
