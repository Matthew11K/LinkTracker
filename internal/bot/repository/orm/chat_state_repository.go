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
	db        *database.PostgresDB
	sq        sq.StatementBuilderType
	txManager *txs.TxManager
}

func NewChatStateRepository(db *database.PostgresDB, txManager *txs.TxManager) *ChatStateRepository {
	return &ChatStateRepository{
		db:        db,
		sq:        sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
		txManager: txManager,
	}
}

func (r *ChatStateRepository) GetState(ctx context.Context, chatID int64) (models.ChatState, error) {
	selectQuery := r.sq.Select("state").
		From("chat_states").
		Where(sq.Eq{"chat_id": chatID})

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return models.StateIdle, &customerrors.ErrBuildSQLQuery{Operation: "получение состояния чата", Cause: err}
	}

	var state int

	err = r.db.Pool.QueryRow(ctx, query, args...).Scan(&state)
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

		stateExists, err := r.checkStateExists(ctx, querier, chatID)
		if err != nil {
			return err
		}

		now := time.Now()
		if stateExists {
			err = r.updateState(ctx, querier, chatID, state, now)
		} else {
			err = r.insertState(ctx, querier, chatID, state, now)
		}

		if err != nil {
			return err
		}

		return nil
	})
}

func (r *ChatStateRepository) checkStateExists(ctx context.Context, querier txs.Querier, chatID int64) (bool, error) {
	existsQuery := r.sq.Select("1").From("chat_states").Where(sq.Eq{"chat_id": chatID}).Limit(1)

	query, args, err := existsQuery.ToSql()
	if err != nil {
		return false, &customerrors.ErrBuildSQLQuery{Operation: "проверка существования состояния", Cause: err}
	}

	var stateExists bool

	err = querier.QueryRow(ctx, "SELECT EXISTS("+query+")", args...).Scan(&stateExists)
	if err != nil {
		return false, &customerrors.ErrSQLExecution{Operation: "проверка существования состояния", Cause: err}
	}

	return stateExists, nil
}

func (r *ChatStateRepository) updateState(
	ctx context.Context,
	querier txs.Querier,
	chatID int64,
	state models.ChatState,
	now time.Time,
) error {
	updateQuery := r.sq.Update("chat_states").
		Set("state", int(state)).
		Set("updated_at", now).
		Where(sq.Eq{"chat_id": chatID})

	query, args, err := updateQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "обновление состояния", Cause: err}
	}

	_, err = querier.Exec(ctx, query, args...)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "обновление состояния чата", Cause: err}
	}

	return nil
}

func (r *ChatStateRepository) insertState(
	ctx context.Context,
	querier txs.Querier,
	chatID int64,
	state models.ChatState,
	now time.Time,
) error {
	insertQuery := r.sq.Insert("chat_states").
		Columns("chat_id", "state", "created_at", "updated_at").
		Values(chatID, int(state), now, now)

	query, args, err := insertQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "вставка состояния", Cause: err}
	}

	_, err = querier.Exec(ctx, query, args...)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "создание состояния чата", Cause: err}
	}

	return nil
}

func (r *ChatStateRepository) GetData(ctx context.Context, chatID int64, key string) (interface{}, error) {
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

	err = r.db.Pool.QueryRow(ctx, query, args...).Scan(&valueStr)
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

		chatExistsQuery := r.sq.Select("1").From("chats").Where(sq.Eq{"id": chatID}).Limit(1)

		query, args, err := chatExistsQuery.ToSql()
		if err != nil {
			return &customerrors.ErrBuildSQLQuery{Operation: "проверка существования чата", Cause: err}
		}

		var chatExists bool

		err = querier.QueryRow(ctx, "SELECT EXISTS("+query+")", args...).Scan(&chatExists)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "проверка существования чата", Cause: err}
		}

		if !chatExists {
			now := time.Now()
			insertChatQuery := r.sq.Insert("chats").
				Columns("id", "created_at", "updated_at").
				Values(chatID, now, now)

			query, args, err = insertChatQuery.ToSql()
			if err != nil {
				return &customerrors.ErrBuildSQLQuery{Operation: "вставка чата", Cause: err}
			}

			_, err = querier.Exec(ctx, query, args...)
			if err != nil {
				return &customerrors.ErrSQLExecution{Operation: "создание чата", Cause: err}
			}
		}

		valueJSON, err := json.Marshal(value)
		if err != nil {
			return &customerrors.ErrInvalidArgument{Message: "не удалось сериализовать значение в JSON"}
		}

		now := time.Now()
		upsertQuery := r.sq.Insert("chat_state_data").
			Columns("chat_id", "key", "value", "created_at", "updated_at").
			Values(chatID, key, string(valueJSON), now, now).
			Suffix("ON CONFLICT (chat_id, key) DO UPDATE SET value = EXCLUDED.value, updated_at = EXCLUDED.updated_at")

		query, args, err = upsertQuery.ToSql()
		if err != nil {
			return &customerrors.ErrBuildSQLQuery{Operation: "вставка/обновление данных", Cause: err}
		}

		_, err = querier.Exec(ctx, query, args...)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "сохранение данных чата", Cause: err}
		}

		return nil
	})
}

func (r *ChatStateRepository) ClearData(ctx context.Context, chatID int64) error {
	deleteQuery := r.sq.Delete("chat_state_data").
		Where(sq.Eq{"chat_id": chatID})

	query, args, err := deleteQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "удаление данных чата", Cause: err}
	}

	_, err = r.db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "удаление данных чата", Cause: err}
	}

	return nil
}
