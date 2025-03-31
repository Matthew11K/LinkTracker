package orm

import (
	"context"
	"errors"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/central-university-dev/go-Matthew11K/internal/database"
	customerrors "github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
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
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return &customerrors.ErrBeginTransaction{Cause: err}
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	now := time.Now()
	if chat.CreatedAt.IsZero() {
		chat.CreatedAt = now
	}

	if chat.UpdatedAt.IsZero() {
		chat.UpdatedAt = now
	}

	existsQuery := r.sq.Select("1").From("chats").Where(sq.Eq{"id": chat.ID}).Limit(1)

	query, args, err := existsQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "проверка существования чата", Cause: err}
	}

	var exists bool

	err = tx.QueryRow(ctx, "SELECT EXISTS("+query+")", args...).Scan(&exists)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "проверка существования чата", Cause: err}
	}

	if exists {
		err = r.updateExistingChat(ctx, tx, chat)
	} else {
		err = r.insertNewChat(ctx, tx, chat)
	}

	if err != nil {
		return err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return &customerrors.ErrCommitTransaction{Cause: err}
	}

	return nil
}

func (r *ChatRepository) updateExistingChat(ctx context.Context, tx pgx.Tx, chat *models.Chat) error {
	updateQuery := r.sq.Update("chats").
		Set("updated_at", chat.UpdatedAt).
		Where(sq.Eq{"id": chat.ID})

	query, args, err := updateQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "обновление чата", Cause: err}
	}

	_, err = tx.Exec(ctx, query, args...)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "обновление чата", Cause: err}
	}

	return nil
}

func (r *ChatRepository) insertNewChat(ctx context.Context, tx pgx.Tx, chat *models.Chat) error {
	insertQuery := r.sq.Insert("chats").
		Columns("id", "created_at", "updated_at").
		Values(chat.ID, chat.CreatedAt, chat.UpdatedAt)

	query, args, err := insertQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "вставка чата", Cause: err}
	}

	_, err = tx.Exec(ctx, query, args...)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "сохранение чата", Cause: err}
	}

	return nil
}

func (r *ChatRepository) FindByID(ctx context.Context, id int64) (*models.Chat, error) {
	selectQuery := r.sq.Select("created_at", "updated_at").
		From("chats").
		Where(sq.Eq{"id": id})

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, &customerrors.ErrBuildSQLQuery{Operation: "поиск чата по ID", Cause: err}
	}

	chat := &models.Chat{ID: id}

	err = r.db.Pool.QueryRow(ctx, query, args...).
		Scan(&chat.CreatedAt, &chat.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &customerrors.ErrChatNotFound{ChatID: id}
		}

		return nil, &customerrors.ErrSQLExecution{Operation: "поиск чата по ID", Cause: err}
	}

	linkIDs, err := r.getLinkIDs(ctx, id)
	if err != nil {
		return nil, err
	}

	chat.Links = linkIDs

	return chat, nil
}

func (r *ChatRepository) getLinkIDs(ctx context.Context, chatID int64) ([]int64, error) {
	selectQuery := r.sq.Select("link_id").
		From("chat_links").
		Where(sq.Eq{"chat_id": chatID})

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, &customerrors.ErrBuildSQLQuery{Operation: "получение ID ссылок", Cause: err}
	}

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "запрос ID ссылок", Cause: err}
	}
	defer rows.Close()

	var linkIDs []int64

	for rows.Next() {
		var linkID int64
		if err := rows.Scan(&linkID); err != nil {
			return nil, &customerrors.ErrSQLScan{Entity: "ID ссылки", Cause: err}
		}

		linkIDs = append(linkIDs, linkID)
	}

	if err := rows.Err(); err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "обработка результатов запроса ID ссылок", Cause: err}
	}

	return linkIDs, nil
}

func (r *ChatRepository) Delete(ctx context.Context, id int64) error {
	existsQuery := r.sq.Select("1").From("chats").Where(sq.Eq{"id": id}).Limit(1)

	query, args, err := existsQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "проверка существования чата", Cause: err}
	}

	var exists bool

	err = r.db.Pool.QueryRow(ctx, "SELECT EXISTS("+query+")", args...).Scan(&exists)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "проверка существования чата", Cause: err}
	}

	if !exists {
		return &customerrors.ErrChatNotFound{ChatID: id}
	}

	deleteQuery := r.sq.Delete("chats").Where(sq.Eq{"id": id})

	query, args, err = deleteQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "удаление чата", Cause: err}
	}

	_, err = r.db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "удаление чата", Cause: err}
	}

	return nil
}

func (r *ChatRepository) Update(ctx context.Context, chat *models.Chat) error {
	chat.UpdatedAt = time.Now()

	updateQuery := r.sq.Update("chats").
		Set("updated_at", chat.UpdatedAt).
		Where(sq.Eq{"id": chat.ID})

	query, args, err := updateQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "обновление чата", Cause: err}
	}

	result, err := r.db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "обновление чата", Cause: err}
	}

	if result.RowsAffected() == 0 {
		return &customerrors.ErrChatNotFound{ChatID: chat.ID}
	}

	return nil
}

//nolint:funlen // Длина функции обусловлена транзакционной обработкой, проверками существования и последовательными операциями добавления.
func (r *ChatRepository) AddLink(ctx context.Context, chatID, linkID int64) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return &customerrors.ErrBeginTransaction{Cause: err}
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	chatExistsQuery := r.sq.Select("1").From("chats").Where(sq.Eq{"id": chatID}).Limit(1)

	query, args, err := chatExistsQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "проверка существования чата", Cause: err}
	}

	var chatExists bool

	err = tx.QueryRow(ctx, "SELECT EXISTS("+query+")", args...).Scan(&chatExists)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "проверка существования чата", Cause: err}
	}

	if !chatExists {
		return &customerrors.ErrChatNotFound{ChatID: chatID}
	}

	linkExistsQuery := r.sq.Select("1").From("links").Where(sq.Eq{"id": linkID}).Limit(1)

	query, args, err = linkExistsQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "проверка существования ссылки", Cause: err}
	}

	var linkExists bool

	err = tx.QueryRow(ctx, "SELECT EXISTS("+query+")", args...).Scan(&linkExists)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "проверка существования ссылки", Cause: err}
	}

	if !linkExists {
		return &customerrors.ErrLinkNotFound{URL: "ID: " + string(rune(linkID))}
	}

	insertQuery := r.sq.Insert("chat_links").
		Columns("chat_id", "link_id").
		Values(chatID, linkID).
		Suffix("ON CONFLICT DO NOTHING")

	query, args, err = insertQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "добавление связи", Cause: err}
	}

	_, err = tx.Exec(ctx, query, args...)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "связывание чата и ссылки", Cause: err}
	}

	updateQuery := r.sq.Update("chats").
		Set("updated_at", time.Now()).
		Where(sq.Eq{"id": chatID})

	query, args, err = updateQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "обновление времени чата", Cause: err}
	}

	_, err = tx.Exec(ctx, query, args...)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "обновление времени чата", Cause: err}
	}

	err = tx.Commit(ctx)
	if err != nil {
		return &customerrors.ErrCommitTransaction{Cause: err}
	}

	return nil
}

func (r *ChatRepository) RemoveLink(ctx context.Context, chatID, linkID int64) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return &customerrors.ErrBeginTransaction{Cause: err}
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	deleteQuery := r.sq.Delete("chat_links").
		Where(sq.And{
			sq.Eq{"chat_id": chatID},
			sq.Eq{"link_id": linkID},
		})

	query, args, err := deleteQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "удаление связи", Cause: err}
	}

	result, err := tx.Exec(ctx, query, args...)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "удаление связи чата и ссылки", Cause: err}
	}

	if result.RowsAffected() == 0 {
		chatExistsQuery := r.sq.Select("1").From("chats").Where(sq.Eq{"id": chatID}).Limit(1)

		query, args, err = chatExistsQuery.ToSql()
		if err != nil {
			return &customerrors.ErrBuildSQLQuery{Operation: "проверка существования чата", Cause: err}
		}

		var chatExists bool

		err = tx.QueryRow(ctx, "SELECT EXISTS("+query+")", args...).Scan(&chatExists)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "проверка существования чата", Cause: err}
		}

		if !chatExists {
			return &customerrors.ErrChatNotFound{ChatID: chatID}
		}

		return &customerrors.ErrLinkNotFound{URL: "ID: " + string(rune(linkID))}
	}

	updateQuery := r.sq.Update("chats").
		Set("updated_at", time.Now()).
		Where(sq.Eq{"id": chatID})

	query, args, err = updateQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "обновление времени чата", Cause: err}
	}

	_, err = tx.Exec(ctx, query, args...)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "обновление времени чата", Cause: err}
	}

	err = tx.Commit(ctx)
	if err != nil {
		return &customerrors.ErrCommitTransaction{Cause: err}
	}

	return nil
}

func (r *ChatRepository) FindByLinkID(ctx context.Context, linkID int64) ([]*models.Chat, error) {
	linkExistsQuery := r.sq.Select("1").From("links").Where(sq.Eq{"id": linkID}).Limit(1)

	query, args, err := linkExistsQuery.ToSql()
	if err != nil {
		return nil, &customerrors.ErrBuildSQLQuery{Operation: "проверка существования ссылки", Cause: err}
	}

	var linkExists bool

	err = r.db.Pool.QueryRow(ctx, "SELECT EXISTS("+query+")", args...).Scan(&linkExists)
	if err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "проверка существования ссылки", Cause: err}
	}

	if !linkExists {
		return nil, &customerrors.ErrLinkNotFound{URL: "ID: " + string(rune(linkID))}
	}

	selectQuery := r.sq.Select("c.id", "c.created_at", "c.updated_at").
		From("chats c").
		Join("chat_links cl ON c.id = cl.chat_id").
		Where(sq.Eq{"cl.link_id": linkID})

	query, args, err = selectQuery.ToSql()
	if err != nil {
		return nil, &customerrors.ErrBuildSQLQuery{Operation: "поиск чатов по link_id", Cause: err}
	}

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "запрос чатов по link_id", Cause: err}
	}
	defer rows.Close()

	var chats []*models.Chat

	for rows.Next() {
		chat := &models.Chat{}
		if err := rows.Scan(&chat.ID, &chat.CreatedAt, &chat.UpdatedAt); err != nil {
			return nil, &customerrors.ErrSQLScan{Entity: "чат", Cause: err}
		}

		linkIDs, err := r.getLinkIDs(ctx, chat.ID)
		if err != nil {
			return nil, err
		}

		chat.Links = linkIDs

		chats = append(chats, chat)
	}

	if err := rows.Err(); err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "обработка результатов запроса чатов", Cause: err}
	}

	return chats, nil
}

func (r *ChatRepository) GetAll(ctx context.Context) ([]*models.Chat, error) {
	selectQuery := r.sq.Select("id", "created_at", "updated_at").
		From("chats").
		OrderBy("id")

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, &customerrors.ErrBuildSQLQuery{Operation: "получение всех чатов", Cause: err}
	}

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "получение всех чатов", Cause: err}
	}
	defer rows.Close()

	chats := make([]*models.Chat, 0)

	for rows.Next() {
		chat := &models.Chat{}

		err := rows.Scan(&chat.ID, &chat.CreatedAt, &chat.UpdatedAt)
		if err != nil {
			return nil, &customerrors.ErrSQLExecution{Operation: "сканирование чата", Cause: err}
		}

		linkIDs, err := r.getLinkIDs(ctx, chat.ID)
		if err != nil {
			return nil, err
		}

		chat.Links = linkIDs

		chats = append(chats, chat)
	}

	if err := rows.Err(); err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "итерация по чатам", Cause: err}
	}

	return chats, nil
}
