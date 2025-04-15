package orm

import (
	"context"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/central-university-dev/go-Matthew11K/internal/database"
	customerrors "github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
	"github.com/central-university-dev/go-Matthew11K/pkg/txs"
	"github.com/jackc/pgx/v5"
)

type ChatRepository struct {
	db *database.PostgresDB
	sq sq.StatementBuilderType
}

func NewChatRepository(db *database.PostgresDB) *ChatRepository {
	return &ChatRepository{
		db: db,
		sq: sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}

func (r *ChatRepository) Save(ctx context.Context, chat *models.Chat) error {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	now := time.Now()
	if chat.CreatedAt.IsZero() {
		chat.CreatedAt = now
	}

	if chat.UpdatedAt.IsZero() {
		chat.UpdatedAt = now
	}

	insertQuery := r.sq.Insert("chats").
		Columns("id", "created_at", "updated_at").
		Values(chat.ID, chat.CreatedAt, chat.UpdatedAt).
		Suffix("ON CONFLICT (id) DO UPDATE SET updated_at = EXCLUDED.updated_at")

	query, args, err := insertQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "вставка/обновление чата", Cause: err}
	}

	_, err = querier.Exec(ctx, query, args...)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "сохранение чата", Cause: err}
	}

	return nil
}

func (r *ChatRepository) FindByID(ctx context.Context, id int64) (*models.Chat, error) {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	selectQuery := r.sq.Select(
		"c.id", "c.created_at", "c.updated_at",
		"COALESCE(array_agg(cl.link_id) FILTER (WHERE cl.link_id IS NOT NULL), '{}') AS links",
	).
		From("chats c").
		LeftJoin("chat_links cl ON c.id = cl.chat_id").
		Where(sq.Eq{"c.id": id}).
		GroupBy("c.id")

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, &customerrors.ErrBuildSQLQuery{Operation: "поиск чата по ID", Cause: err}
	}

	row := querier.QueryRow(ctx, query, args...)

	var chat models.Chat

	var linksArr []int64

	err = row.Scan(
		&chat.ID,
		&chat.CreatedAt,
		&chat.UpdatedAt,
		&linksArr,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &customerrors.ErrChatNotFound{ChatID: id}
		}

		return nil, &customerrors.ErrSQLExecution{Operation: "поиск чата по ID", Cause: err}
	}

	chat.Links = linksArr

	return &chat, nil
}

func (r *ChatRepository) Delete(ctx context.Context, id int64) error {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	deleteChatQuery := r.sq.Delete("chats").Where(sq.Eq{"id": id})

	query, args, err := deleteChatQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "удаление чата", Cause: err}
	}

	result, err := querier.Exec(ctx, query, args...)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "удаление чата", Cause: err}
	}

	if result.RowsAffected() == 0 {
		return &customerrors.ErrChatNotFound{ChatID: id}
	}

	return nil
}

func (r *ChatRepository) Update(ctx context.Context, chat *models.Chat) error {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	chat.UpdatedAt = time.Now()

	updateQuery := r.sq.Update("chats").
		Set("updated_at", chat.UpdatedAt).
		Where(sq.Eq{"id": chat.ID})

	query, args, err := updateQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "обновление чата", Cause: err}
	}

	result, err := querier.Exec(ctx, query, args...)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "обновление чата", Cause: err}
	}

	if result.RowsAffected() == 0 {
		return &customerrors.ErrChatNotFound{ChatID: chat.ID}
	}

	return nil
}

func (r *ChatRepository) AddLink(ctx context.Context, chatID, linkID int64) error {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	insertQuery := r.sq.Insert("chat_links").
		Columns("chat_id", "link_id").
		Values(chatID, linkID).
		Suffix("ON CONFLICT (chat_id, link_id) DO NOTHING")

	query, args, err := insertQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "добавление связи", Cause: err}
	}

	result, err := querier.Exec(ctx, query, args...)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "добавление связи", Cause: err}
	}

	if result.RowsAffected() == 0 {
		var chatCount, linkCount int

		chatCountQuery := r.sq.Select("COUNT(*)").From("chats").Where(sq.Eq{"id": chatID})
		linkCountQuery := r.sq.Select("COUNT(*)").From("links").Where(sq.Eq{"id": linkID})
		chatQuery, chatArgs, _ := chatCountQuery.ToSql()
		linkQuery, linkArgs, _ := linkCountQuery.ToSql()
		_ = querier.QueryRow(ctx, chatQuery, chatArgs...).Scan(&chatCount)
		_ = querier.QueryRow(ctx, linkQuery, linkArgs...).Scan(&linkCount)

		if chatCount == 0 {
			return &customerrors.ErrChatNotFound{ChatID: chatID}
		}

		if linkCount == 0 {
			return &customerrors.ErrLinkNotFound{URL: fmt.Sprintf("ID: %d", linkID)}
		}

		return &customerrors.ErrLinkNotInChat{ChatID: chatID, LinkID: linkID}
	}

	return nil
}

func (r *ChatRepository) RemoveLink(ctx context.Context, chatID, linkID int64) error {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	deleteQuery := r.sq.Delete("chat_links").
		Where(sq.And{
			sq.Eq{"chat_id": chatID},
			sq.Eq{"link_id": linkID},
		})

	query, args, err := deleteQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "удаление связи", Cause: err}
	}

	result, err := querier.Exec(ctx, query, args...)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "удаление связи", Cause: err}
	}

	if result.RowsAffected() == 0 {
		return &customerrors.ErrLinkNotInChat{ChatID: chatID, LinkID: linkID}
	}

	return nil
}

func (r *ChatRepository) FindByLinkID(ctx context.Context, linkID int64) ([]*models.Chat, error) {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	selectQuery := r.sq.Select("c.id", "c.created_at", "c.updated_at").
		From("chats c").
		Join("chat_links cl ON c.id = cl.chat_id").
		Where(sq.Eq{"cl.link_id": linkID})

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, &customerrors.ErrBuildSQLQuery{Operation: "поиск чатов по ссылке", Cause: err}
	}

	rows, err := querier.Query(ctx, query, args...)
	if err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "поиск чатов по ссылке", Cause: err}
	}
	defer rows.Close()

	var chats []*models.Chat

	for rows.Next() {
		chat := &models.Chat{}

		err := rows.Scan(&chat.ID, &chat.CreatedAt, &chat.UpdatedAt)
		if err != nil {
			return nil, &customerrors.ErrSQLExecution{Operation: "чтение чата", Cause: err}
		}

		chat.Links = []int64{}
		chats = append(chats, chat)
	}

	if err := rows.Err(); err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "обработка результатов", Cause: err}
	}

	if len(chats) == 0 {
		linkCountQuery := r.sq.Select("COUNT(*)").From("links").Where(sq.Eq{"id": linkID})
		linkQuery, linkArgs, _ := linkCountQuery.ToSql()

		var linkCount int
		_ = querier.QueryRow(ctx, linkQuery, linkArgs...).Scan(&linkCount)

		if linkCount == 0 {
			return nil, &customerrors.ErrLinkNotFound{URL: fmt.Sprintf("ID: %d", linkID)}
		}
	}

	return chats, nil
}

func (r *ChatRepository) GetAll(ctx context.Context) ([]*models.Chat, error) {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	selectQuery := r.sq.Select(
		"c.id", "c.created_at", "c.updated_at",
		"COALESCE(array_agg(cl.link_id) FILTER (WHERE cl.link_id IS NOT NULL), '{}') AS links",
	).
		From("chats c").
		LeftJoin("chat_links cl ON c.id = cl.chat_id").
		GroupBy("c.id").
		OrderBy("c.id")

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, &customerrors.ErrBuildSQLQuery{Operation: "получение всех чатов", Cause: err}
	}

	rows, err := querier.Query(ctx, query, args...)
	if err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "получение всех чатов", Cause: err}
	}
	defer rows.Close()

	var chats []*models.Chat

	for rows.Next() {
		var chat models.Chat

		var linksArr []int64

		err := rows.Scan(
			&chat.ID,
			&chat.CreatedAt,
			&chat.UpdatedAt,
			&linksArr,
		)
		if err != nil {
			return nil, &customerrors.ErrSQLExecution{Operation: "сканирование чата", Cause: err}
		}

		chat.Links = linksArr
		chats = append(chats, &chat)
	}

	if err := rows.Err(); err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "обработка результатов", Cause: err}
	}

	return chats, nil
}

func (r *ChatRepository) ExistsChatLink(ctx context.Context, chatID, linkID int64) (bool, error) {
	querier := txs.GetQuerier(ctx, r.db.Pool)
	selectQuery := r.sq.Select("1").From("chat_links").Where(sq.And{sq.Eq{"chat_id": chatID}, sq.Eq{"link_id": linkID}}).Limit(1)

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return false, &customerrors.ErrBuildSQLQuery{Operation: "проверка связи чата и ссылки", Cause: err}
	}

	var exists bool

	err = querier.QueryRow(ctx, "SELECT EXISTS("+query+")", args...).Scan(&exists)
	if err != nil {
		return false, &customerrors.ErrSQLExecution{Operation: "проверка связи чата и ссылки", Cause: err}
	}

	return exists, nil
}
