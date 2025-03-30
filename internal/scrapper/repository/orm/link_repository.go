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
	"github.com/jackc/pgx/v5/pgconn"
)

type LinkRepository struct {
	db *database.PostgresDB
	sq sq.StatementBuilderType
}

func NewLinkRepository(db *database.PostgresDB) *LinkRepository {
	return &LinkRepository{
		db: db,
		sq: sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}

func (r *LinkRepository) Save(ctx context.Context, link *models.Link) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return &customerrors.ErrBeginTransaction{Cause: err}
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	existsQuery := r.sq.Select("1").From("links").Where(sq.Eq{"url": link.URL}).Limit(1)
	query, args, err := existsQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "проверка существования ссылки", Cause: err}
	}

	var exists bool
	err = tx.QueryRow(ctx, "SELECT EXISTS("+query+")", args...).Scan(&exists)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "проверка существования ссылки", Cause: err}
	}

	if exists {
		return &customerrors.ErrLinkAlreadyExists{URL: link.URL}
	}

	now := time.Now()
	if link.CreatedAt.IsZero() {
		link.CreatedAt = now
	}

	insertQuery := r.sq.Insert("links").
		Columns("url", "type", "last_checked", "last_updated", "created_at").
		Values(link.URL, link.Type, link.LastChecked, link.LastUpdated, link.CreatedAt).
		Suffix("RETURNING id")

	query, args, err = insertQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "вставка ссылки", Cause: err}
	}

	var id int64
	err = tx.QueryRow(ctx, query, args...).Scan(&id)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "сохранение ссылки", Cause: err}
	}

	link.ID = id

	if len(link.Tags) > 0 {
		err = r.saveTags(ctx, tx, id, link.Tags)
		if err != nil {
			return err
		}
	}

	if len(link.Filters) > 0 {
		err = r.saveFilters(ctx, tx, id, link.Filters)
		if err != nil {
			return err
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		return &customerrors.ErrCommitTransaction{Cause: err}
	}

	return nil
}

func (r *LinkRepository) saveTags(ctx context.Context, tx pgx.Tx, linkID int64, tags []string) error {
	for _, tag := range tags {
		insertQuery := r.sq.Insert("tags").
			Columns("name").
			Values(tag).
			Suffix("ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name RETURNING id")

		query, args, err := insertQuery.ToSql()
		if err != nil {
			return &customerrors.ErrBuildSQLQuery{Operation: "создание тега", Cause: err}
		}

		var tagID int64
		err = tx.QueryRow(ctx, query, args...).Scan(&tagID)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "сохранение тега " + tag, Cause: err}
		}

		linkTagQuery := r.sq.Insert("link_tags").
			Columns("link_id", "tag_id").
			Values(linkID, tagID).
			Suffix("ON CONFLICT DO NOTHING")

		query, args, err = linkTagQuery.ToSql()
		if err != nil {
			return &customerrors.ErrBuildSQLQuery{Operation: "связь тега", Cause: err}
		}

		_, err = tx.Exec(ctx, query, args...)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "связывание ссылки с тегом", Cause: err}
		}
	}

	return nil
}

func (r *LinkRepository) saveFilters(ctx context.Context, tx pgx.Tx, linkID int64, filters []string) error {
	for _, filter := range filters {
		insertQuery := r.sq.Insert("filters").
			Columns("value", "link_id").
			Values(filter, linkID)

		query, args, err := insertQuery.ToSql()
		if err != nil {
			return &customerrors.ErrBuildSQLQuery{Operation: "создание фильтра", Cause: err}
		}

		_, err = tx.Exec(ctx, query, args...)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "сохранение фильтра " + filter, Cause: err}
		}
	}

	return nil
}

func (r *LinkRepository) FindByID(ctx context.Context, id int64) (*models.Link, error) {
	selectQuery := r.sq.Select("url", "type", "last_checked", "last_updated", "created_at").
		From("links").
		Where(sq.Eq{"id": id})

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, &customerrors.ErrBuildSQLQuery{Operation: "поиск по ID", Cause: err}
	}

	link := &models.Link{ID: id}
	err = r.db.Pool.QueryRow(ctx, query, args...).
		Scan(&link.URL, &link.Type, &link.LastChecked, &link.LastUpdated, &link.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &customerrors.ErrLinkNotFound{URL: "ID: " + string(rune(id))}
		}
		return nil, &customerrors.ErrSQLExecution{Operation: "поиск ссылки по ID", Cause: err}
	}

	tags, err := r.getTagsByLinkID(ctx, id)
	if err != nil {
		return nil, err
	}
	link.Tags = tags

	filters, err := r.getFiltersByLinkID(ctx, id)
	if err != nil {
		return nil, err
	}
	link.Filters = filters

	return link, nil
}

func (r *LinkRepository) getTagsByLinkID(ctx context.Context, linkID int64) ([]string, error) {
	selectQuery := r.sq.Select("t.name").
		From("tags t").
		Join("link_tags lt ON t.id = lt.tag_id").
		Where(sq.Eq{"lt.link_id": linkID})

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, &customerrors.ErrBuildSQLQuery{Operation: "получение тегов", Cause: err}
	}

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "запрос тегов", Cause: err}
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, &customerrors.ErrSQLScan{Entity: "тег", Cause: err}
		}
		tags = append(tags, tag)
	}

	if err := rows.Err(); err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "обработка результатов запроса тегов", Cause: err}
	}

	return tags, nil
}

func (r *LinkRepository) getFiltersByLinkID(ctx context.Context, linkID int64) ([]string, error) {
	selectQuery := r.sq.Select("value").
		From("filters").
		Where(sq.Eq{"link_id": linkID})

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, &customerrors.ErrBuildSQLQuery{Operation: "получение фильтров", Cause: err}
	}

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "запрос фильтров", Cause: err}
	}
	defer rows.Close()

	var filters []string
	for rows.Next() {
		var filter string
		if err := rows.Scan(&filter); err != nil {
			return nil, &customerrors.ErrSQLScan{Entity: "фильтр", Cause: err}
		}
		filters = append(filters, filter)
	}

	if err := rows.Err(); err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "обработка результатов запроса фильтров", Cause: err}
	}

	return filters, nil
}

func (r *LinkRepository) FindByURL(ctx context.Context, url string) (*models.Link, error) {
	selectQuery := r.sq.Select("id").
		From("links").
		Where(sq.Eq{"url": url})

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, &customerrors.ErrBuildSQLQuery{Operation: "поиск по URL", Cause: err}
	}

	var id int64
	err = r.db.Pool.QueryRow(ctx, query, args...).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &customerrors.ErrLinkNotFound{URL: url}
		}
		return nil, &customerrors.ErrSQLExecution{Operation: "поиск ссылки по URL", Cause: err}
	}

	return r.FindByID(ctx, id)
}

func (r *LinkRepository) FindByChatID(ctx context.Context, chatID int64) ([]*models.Link, error) {
	selectQuery := r.sq.Select("l.id", "l.url", "l.type", "l.last_checked", "l.last_updated", "l.created_at").
		From("links l").
		Join("chat_links cl ON l.id = cl.link_id").
		Where(sq.Eq{"cl.chat_id": chatID})

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, &customerrors.ErrBuildSQLQuery{Operation: "поиск по chat_id", Cause: err}
	}

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "запрос ссылок по chat_id", Cause: err}
	}
	defer rows.Close()

	var links []*models.Link
	for rows.Next() {
		link := &models.Link{}
		if err := rows.Scan(&link.ID, &link.URL, &link.Type, &link.LastChecked, &link.LastUpdated, &link.CreatedAt); err != nil {
			return nil, &customerrors.ErrSQLScan{Entity: "ссылка", Cause: err}
		}

		tags, err := r.getTagsByLinkID(ctx, link.ID)
		if err != nil {
			return nil, err
		}
		link.Tags = tags

		filters, err := r.getFiltersByLinkID(ctx, link.ID)
		if err != nil {
			return nil, err
		}
		link.Filters = filters

		links = append(links, link)
	}

	if err := rows.Err(); err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "обработка результатов запроса ссылок", Cause: err}
	}

	return links, nil
}

func (r *LinkRepository) DeleteByURL(ctx context.Context, url string, chatID int64) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return &customerrors.ErrBeginTransaction{Cause: err}
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	selectQuery := r.sq.Select("id").
		From("links").
		Where(sq.Eq{"url": url})

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "получение ID ссылки", Cause: err}
	}

	var linkID int64
	err = tx.QueryRow(ctx, query, args...).Scan(&linkID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &customerrors.ErrLinkNotFound{URL: url}
		}
		return &customerrors.ErrSQLExecution{Operation: "поиск ссылки", Cause: err}
	}

	deleteQuery := r.sq.Delete("chat_links").
		Where(sq.And{
			sq.Eq{"chat_id": chatID},
			sq.Eq{"link_id": linkID},
		})

	query, args, err = deleteQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "удаление связи", Cause: err}
	}

	cmdTag, err := tx.Exec(ctx, query, args...)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "удаление связи чата и ссылки", Cause: err}
	}

	if cmdTag.RowsAffected() == 0 {
		return &customerrors.ErrLinkNotFound{URL: url}
	}

	countQuery := r.sq.Select("COUNT(*)").
		From("chat_links").
		Where(sq.Eq{"link_id": linkID})

	query, args, err = countQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "подсчет связей", Cause: err}
	}

	var count int
	err = tx.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "проверка использования ссылки", Cause: err}
	}

	if count == 0 {
		deleteQuery := r.sq.Delete("links").
			Where(sq.Eq{"id": linkID})

		query, args, err = deleteQuery.ToSql()
		if err != nil {
			return &customerrors.ErrBuildSQLQuery{Operation: "удаление ссылки", Cause: err}
		}

		_, err = tx.Exec(ctx, query, args...)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "удаление ссылки", Cause: err}
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		return &customerrors.ErrCommitTransaction{Cause: err}
	}

	return nil
}

func (r *LinkRepository) Update(ctx context.Context, link *models.Link) error {
	updateQuery := r.sq.Update("links").
		Set("last_checked", link.LastChecked).
		Set("last_updated", link.LastUpdated).
		Where(sq.Eq{"id": link.ID})

	query, args, err := updateQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "обновление ссылки", Cause: err}
	}

	_, err = r.db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "обновление ссылки", Cause: err}
	}

	return nil
}

func (r *LinkRepository) AddChatLink(ctx context.Context, chatID, linkID int64) error {
	insertQuery := r.sq.Insert("chat_links").
		Columns("chat_id", "link_id").
		Values(chatID, linkID).
		Suffix("ON CONFLICT DO NOTHING")

	query, args, err := insertQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "связывание чата и ссылки", Cause: err}
	}

	_, err = r.db.Pool.Exec(ctx, query, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation
			if pgErr.ConstraintName == "chat_links_chat_id_fkey" {
				return &customerrors.ErrChatNotFound{ChatID: chatID}
			} else if pgErr.ConstraintName == "chat_links_link_id_fkey" {
				return &customerrors.ErrLinkNotFound{URL: "ID: " + string(rune(linkID))}
			}
		}
		return &customerrors.ErrSQLExecution{Operation: "связывание чата и ссылки", Cause: err}
	}

	return nil
}

func (r *LinkRepository) FindDue(ctx context.Context, limit, offset int) ([]*models.Link, error) {
	selectQuery := r.sq.Select("id", "url", "type", "last_checked", "last_updated", "created_at").
		From("links").
		OrderBy("last_checked NULLS FIRST, id").
		Limit(uint64(limit)).
		Offset(uint64(offset))

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, &customerrors.ErrBuildSQLQuery{Operation: "поиск ссылок для проверки", Cause: err}
	}

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "запрос ссылок для проверки", Cause: err}
	}
	defer rows.Close()

	var links []*models.Link
	for rows.Next() {
		link := &models.Link{}
		if err := rows.Scan(&link.ID, &link.URL, &link.Type, &link.LastChecked, &link.LastUpdated, &link.CreatedAt); err != nil {
			return nil, &customerrors.ErrSQLScan{Entity: "ссылка", Cause: err}
		}

		tags, err := r.getTagsByLinkID(ctx, link.ID)
		if err != nil {
			return nil, err
		}
		link.Tags = tags

		filters, err := r.getFiltersByLinkID(ctx, link.ID)
		if err != nil {
			return nil, err
		}
		link.Filters = filters

		links = append(links, link)
	}

	if err := rows.Err(); err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "обработка результатов запроса ссылок", Cause: err}
	}

	return links, nil
}

func (r *LinkRepository) Count(ctx context.Context) (int, error) {
	selectQuery := r.sq.Select("COUNT(*)").
		From("links")

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return 0, &customerrors.ErrBuildSQLQuery{Operation: "подсчет ссылок", Cause: err}
	}

	var count int
	err = r.db.Pool.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, &customerrors.ErrSQLExecution{Operation: "подсчет ссылок", Cause: err}
	}

	return count, nil
}

func (r *LinkRepository) SaveTags(ctx context.Context, linkID int64, tags []string) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return &customerrors.ErrBeginTransaction{Cause: err}
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	deleteQuery := r.sq.Delete("link_tags").
		Where(sq.Eq{"link_id": linkID})

	query, args, err := deleteQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "удаление тегов", Cause: err}
	}

	_, err = tx.Exec(ctx, query, args...)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "удаление существующих тегов", Cause: err}
	}

	if len(tags) > 0 {
		err = r.saveTags(ctx, tx, linkID, tags)
		if err != nil {
			return err
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		return &customerrors.ErrCommitTransaction{Cause: err}
	}

	return nil
}

func (r *LinkRepository) SaveFilters(ctx context.Context, linkID int64, filters []string) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return &customerrors.ErrBeginTransaction{Cause: err}
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	deleteQuery := r.sq.Delete("filters").
		Where(sq.Eq{"link_id": linkID})

	query, args, err := deleteQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "удаление фильтров", Cause: err}
	}

	_, err = tx.Exec(ctx, query, args...)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "удаление существующих фильтров", Cause: err}
	}

	if len(filters) > 0 {
		err = r.saveFilters(ctx, tx, linkID, filters)
		if err != nil {
			return err
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		return &customerrors.ErrCommitTransaction{Cause: err}
	}

	return nil
}

func (r *LinkRepository) GetAll(ctx context.Context) ([]*models.Link, error) {
	selectQuery := r.sq.Select("id", "url", "type", "last_checked", "last_updated", "created_at").
		From("links").
		OrderBy("id")

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, &customerrors.ErrBuildSQLQuery{Operation: "получение всех ссылок", Cause: err}
	}

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "получение всех ссылок", Cause: err}
	}
	defer rows.Close()

	links := make([]*models.Link, 0)
	for rows.Next() {
		link := &models.Link{}
		err := rows.Scan(&link.ID, &link.URL, &link.Type, &link.LastChecked, &link.LastUpdated, &link.CreatedAt)
		if err != nil {
			return nil, &customerrors.ErrSQLExecution{Operation: "сканирование ссылки", Cause: err}
		}

		tags, err := r.getTagsByLinkID(ctx, link.ID)
		if err != nil {
			return nil, err
		}
		link.Tags = tags

		filters, err := r.getFiltersByLinkID(ctx, link.ID)
		if err != nil {
			return nil, err
		}
		link.Filters = filters

		links = append(links, link)
	}

	if err := rows.Err(); err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "итерация по ссылкам", Cause: err}
	}

	return links, nil
}
