package orm

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/central-university-dev/go-Matthew11K/internal/database"
	customerrors "github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
	"github.com/central-university-dev/go-Matthew11K/pkg/txs"
	"github.com/jackc/pgx/v5"
)

type ChatStateRepository struct {
	db *database.PostgresDB
	sq sq.StatementBuilderType
}

func NewChatStateRepository(db *database.PostgresDB) *ChatStateRepository {
	return &ChatStateRepository{
		db: db,
		sq: sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}

func (r *ChatStateRepository) GetState(ctx context.Context, chatID int64) (models.ChatState, error) {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	selectQuery := r.sq.Select("state").
		From("chat_states").
		Where(sq.Eq{"chat_id": chatID})

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return models.StateIdle, &customerrors.ErrBuildSQLQuery{Operation: "получение состояния чата", Cause: err}
	}

	var state int

	err = querier.QueryRow(ctx, query, args...).Scan(&state)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.StateIdle, &customerrors.ErrChatStateNotFound{ChatID: chatID}
		}

		return models.StateIdle, &customerrors.ErrSQLExecution{Operation: "получение состояния чата", Cause: err}
	}

	return models.ChatState(state), nil
}

func (r *ChatStateRepository) SetState(ctx context.Context, chatID int64, state models.ChatState) error {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	now := time.Now()
	upsertQuery := r.sq.Insert("chat_states").
		Columns("chat_id", "state", "created_at", "updated_at").
		Values(chatID, int(state), now, now).
		Suffix("ON CONFLICT (chat_id) DO UPDATE SET state = EXCLUDED.state, updated_at = EXCLUDED.updated_at")

	query, args, err := upsertQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "вставка/обновление состояния", Cause: err}
	}

	_, err = querier.Exec(ctx, query, args...)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "сохранение состояния чата", Cause: err}
	}

	return nil
}

func (r *ChatStateRepository) GetData(ctx context.Context, chatID int64, key string) (any, error) {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	selectQuery := r.sq.Select("value").
		From("chat_state_data").
		Where(sq.And{
			sq.Eq{"chat_id": chatID},
			sq.Eq{"key": key},
		})

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, &customerrors.ErrBuildSQLQuery{Operation: "получение данных чата", Cause: err}
	}

	var valueStr string

	err = querier.QueryRow(ctx, query, args...).Scan(&valueStr)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &customerrors.ErrChatStateNotFound{ChatID: chatID}
		}

		return nil, &customerrors.ErrSQLExecution{Operation: "получение данных чата", Cause: err}
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
	upsertQuery := r.sq.Insert("chat_state_data").
		Columns("chat_id", "key", "value", "created_at", "updated_at").
		Values(chatID, key, string(valueJSON), now, now).
		Suffix("ON CONFLICT (chat_id, key) DO UPDATE SET value = EXCLUDED.value, updated_at = EXCLUDED.updated_at")

	query, args, err := upsertQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "вставка/обновление данных", Cause: err}
	}

	_, err = querier.Exec(ctx, query, args...)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "сохранение данных чата", Cause: err}
	}

	return nil
}

func (r *ChatStateRepository) ClearData(ctx context.Context, chatID int64) error {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	deleteQuery := r.sq.Delete("chat_state_data").
		Where(sq.Eq{"chat_id": chatID})

	query, args, err := deleteQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "удаление данных чата", Cause: err}
	}

	_, err = querier.Exec(ctx, query, args...)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "удаление данных чата", Cause: err}
	}

	return nil
}
