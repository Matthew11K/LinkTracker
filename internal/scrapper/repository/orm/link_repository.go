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

type LinkRepository struct {
	db        *database.PostgresDB
	sq        sq.StatementBuilderType
	txManager *txs.TxManager
}

func NewLinkRepository(db *database.PostgresDB, txManager *txs.TxManager) *LinkRepository {
	return &LinkRepository{
		db:        db,
		sq:        sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
		txManager: txManager,
	}
}

func (r *LinkRepository) Save(ctx context.Context, link *models.Link) error {
	return r.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		querier := txs.GetQuerier(ctx, r.db.Pool)

		existsQuery := r.sq.Select("1").From("links").Where(sq.Eq{"url": link.URL}).Limit(1)

		query, args, err := existsQuery.ToSql()
		if err != nil {
			return &customerrors.ErrBuildSQLQuery{Operation: "проверка существования ссылки", Cause: err}
		}

		var exists bool

		err = querier.QueryRow(ctx, "SELECT EXISTS("+query+")", args...).Scan(&exists)
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

		err = querier.QueryRow(ctx, query, args...).Scan(&id)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "сохранение ссылки", Cause: err}
		}

		link.ID = id

		if len(link.Tags) > 0 {
			err = r.saveTags(ctx, querier, id, link.Tags)
			if err != nil {
				return err
			}
		}

		if len(link.Filters) > 0 {
			err = r.saveFilters(ctx, querier, id, link.Filters)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *LinkRepository) saveTags(ctx context.Context, querier txs.Querier, linkID int64, tags []string) error {
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

		err = querier.QueryRow(ctx, query, args...).Scan(&tagID)
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

		_, err = querier.Exec(ctx, query, args...)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "связывание ссылки с тегом", Cause: err}
		}
	}

	return nil
}

func (r *LinkRepository) saveFilters(ctx context.Context, querier txs.Querier, linkID int64, filters []string) error {
	for _, filter := range filters {
		insertQuery := r.sq.Insert("filters").
			Columns("value", "link_id").
			Values(filter, linkID)

		query, args, err := insertQuery.ToSql()
		if err != nil {
			return &customerrors.ErrBuildSQLQuery{Operation: "создание фильтра", Cause: err}
		}

		_, err = querier.Exec(ctx, query, args...)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "сохранение фильтра " + filter, Cause: err}
		}
	}

	return nil
}

func (r *LinkRepository) FindByID(ctx context.Context, id int64) (*models.Link, error) {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	selectQuery := r.sq.Select("id", "url", "type", "last_checked", "last_updated", "created_at").
		From("links").
		Where(sq.Eq{"id": id})

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, &customerrors.ErrBuildSQLQuery{Operation: "поиск ссылки по ID", Cause: err}
	}

	var link models.Link

	err = querier.QueryRow(ctx, query, args...).Scan(
		&link.ID,
		&link.URL,
		&link.Type,
		&link.LastChecked,
		&link.LastUpdated,
		&link.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &customerrors.ErrLinkNotFound{URL: fmt.Sprintf("ID: %d", id)}
		}

		return nil, &customerrors.ErrSQLExecution{Operation: "поиск ссылки по ID", Cause: err}
	}

	tags, err := r.GetTags(ctx, link.ID)
	if err != nil {
		return nil, err
	}

	link.Tags = tags

	filters, err := r.GetFilters(ctx, link.ID)
	if err != nil {
		return nil, err
	}

	link.Filters = filters

	return &link, nil
}

func (r *LinkRepository) GetTags(ctx context.Context, linkID int64) ([]string, error) {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	selectQuery := r.sq.Select("t.name").
		From("tags t").
		Join("link_tags lt ON t.id = lt.tag_id").
		Where(sq.Eq{"lt.link_id": linkID})

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, &customerrors.ErrBuildSQLQuery{Operation: "получение тегов", Cause: err}
	}

	rows, err := querier.Query(ctx, query, args...)
	if err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "получение тегов", Cause: err}
	}
	defer rows.Close()

	var tags []string

	for rows.Next() {
		var tag string

		err = rows.Scan(&tag)
		if err != nil {
			return nil, &customerrors.ErrSQLExecution{Operation: "чтение тега", Cause: err}
		}

		tags = append(tags, tag)
	}

	if err = rows.Err(); err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "обработка результатов", Cause: err}
	}

	return tags, nil
}

func (r *LinkRepository) GetFilters(ctx context.Context, linkID int64) ([]string, error) {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	selectQuery := r.sq.Select("value").
		From("filters").
		Where(sq.Eq{"link_id": linkID})

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, &customerrors.ErrBuildSQLQuery{Operation: "получение фильтров", Cause: err}
	}

	rows, err := querier.Query(ctx, query, args...)
	if err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "получение фильтров", Cause: err}
	}
	defer rows.Close()

	var filters []string

	for rows.Next() {
		var filter string

		err = rows.Scan(&filter)
		if err != nil {
			return nil, &customerrors.ErrSQLExecution{Operation: "чтение фильтра", Cause: err}
		}

		filters = append(filters, filter)
	}

	if err = rows.Err(); err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "обработка результатов", Cause: err}
	}

	return filters, nil
}

func (r *LinkRepository) FindByURL(ctx context.Context, url string) (*models.Link, error) {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	selectQuery := r.sq.Select("id", "url", "type", "last_checked", "last_updated", "created_at").
		From("links").
		Where(sq.Eq{"url": url}).
		Limit(1)

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, &customerrors.ErrBuildSQLQuery{Operation: "поиск ссылки", Cause: err}
	}

	var link models.Link

	err = querier.QueryRow(ctx, query, args...).Scan(
		&link.ID,
		&link.URL,
		&link.Type,
		&link.LastChecked,
		&link.LastUpdated,
		&link.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &customerrors.ErrLinkNotFound{URL: url}
		}

		return nil, &customerrors.ErrSQLExecution{Operation: "поиск ссылки", Cause: err}
	}

	tags, err := r.GetTags(ctx, link.ID)
	if err != nil {
		return nil, err
	}

	link.Tags = tags

	filters, err := r.GetFilters(ctx, link.ID)
	if err != nil {
		return nil, err
	}

	link.Filters = filters

	return &link, nil
}

func (r *LinkRepository) FindAllByChatID(ctx context.Context, chatID int64) ([]*models.Link, error) {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	selectQuery := r.sq.Select("l.id", "l.url", "l.type", "l.last_checked", "l.last_updated", "l.created_at").
		From("links l").
		Join("chat_links cl ON l.id = cl.link_id").
		Where(sq.Eq{"cl.chat_id": chatID})

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, &customerrors.ErrBuildSQLQuery{Operation: "поиск всех ссылок", Cause: err}
	}

	rows, err := querier.Query(ctx, query, args...)
	if err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "поиск всех ссылок", Cause: err}
	}
	defer rows.Close()

	var links []*models.Link

	for rows.Next() {
		var link models.Link

		err = rows.Scan(
			&link.ID,
			&link.URL,
			&link.Type,
			&link.LastChecked,
			&link.LastUpdated,
			&link.CreatedAt,
		)
		if err != nil {
			return nil, &customerrors.ErrSQLExecution{Operation: "чтение ссылки", Cause: err}
		}

		tags, err := r.GetTags(ctx, link.ID)
		if err != nil {
			return nil, err
		}

		link.Tags = tags

		filters, err := r.GetFilters(ctx, link.ID)
		if err != nil {
			return nil, err
		}

		link.Filters = filters

		links = append(links, &link)
	}

	if err = rows.Err(); err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "обработка результатов", Cause: err}
	}

	return links, nil
}

func (r *LinkRepository) FindByChatID(ctx context.Context, chatID int64) ([]*models.Link, error) {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	selectQuery := r.sq.Select("l.id", "l.url", "l.type", "l.last_checked", "l.last_updated", "l.created_at").
		From("links l").
		Join("chat_links cl ON l.id = cl.link_id").
		Where(sq.Eq{"cl.chat_id": chatID})

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, &customerrors.ErrBuildSQLQuery{Operation: "поиск ссылок по ID чата", Cause: err}
	}

	rows, err := querier.Query(ctx, query, args...)
	if err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "поиск ссылок", Cause: err}
	}
	defer rows.Close()

	var links []*models.Link

	for rows.Next() {
		var link models.Link

		err = rows.Scan(
			&link.ID,
			&link.URL,
			&link.Type,
			&link.LastChecked,
			&link.LastUpdated,
			&link.CreatedAt,
		)
		if err != nil {
			return nil, &customerrors.ErrSQLExecution{Operation: "чтение данных ссылки", Cause: err}
		}

		tags, err := r.GetTags(ctx, link.ID)
		if err != nil {
			return nil, err
		}

		link.Tags = tags

		filters, err := r.GetFilters(ctx, link.ID)
		if err != nil {
			return nil, err
		}

		link.Filters = filters

		links = append(links, &link)
	}

	if err = rows.Err(); err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "обработка результатов", Cause: err}
	}

	return links, nil
}

func (r *LinkRepository) getLinkIDByURL(ctx context.Context, querier txs.Querier, url string) (int64, error) {
	linkQuery := r.sq.Select("id").From("links").Where(sq.Eq{"url": url}).Limit(1)

	query, args, err := linkQuery.ToSql()
	if err != nil {
		return 0, &customerrors.ErrBuildSQLQuery{Operation: "получение ID ссылки", Cause: err}
	}

	var linkID int64

	err = querier.QueryRow(ctx, query, args...).Scan(&linkID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, &customerrors.ErrLinkNotFound{URL: url}
		}

		return 0, &customerrors.ErrSQLExecution{Operation: "получение ID ссылки", Cause: err}
	}

	return linkID, nil
}

func (r *LinkRepository) deleteLinkWithDependencies(ctx context.Context, querier txs.Querier, linkID int64) error {
	tablesInfo := []struct {
		tableName string
		operation string
	}{
		{"filters", "удаление фильтров"},
		{"link_tags", "удаление тегов"},
		{"github_details", "удаление деталей GitHub"},
		{"stackoverflow_details", "удаление деталей StackOverflow"},
	}

	for _, tableInfo := range tablesInfo {
		deleteQuery := r.sq.Delete(tableInfo.tableName).Where(sq.Eq{"link_id": linkID})

		query, args, err := deleteQuery.ToSql()
		if err != nil {
			return &customerrors.ErrBuildSQLQuery{Operation: tableInfo.operation, Cause: err}
		}

		_, err = querier.Exec(ctx, query, args...)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: tableInfo.operation, Cause: err}
		}
	}

	deleteLinkQuery := r.sq.Delete("links").Where(sq.Eq{"id": linkID})

	query, args, err := deleteLinkQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "удаление ссылки", Cause: err}
	}

	_, err = querier.Exec(ctx, query, args...)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "удаление ссылки", Cause: err}
	}

	return nil
}

func (r *LinkRepository) deleteChatLink(ctx context.Context, querier txs.Querier, chatID, linkID int64, url string) error {
	deleteQuery := r.sq.Delete("chat_links").Where(sq.And{sq.Eq{"chat_id": chatID}, sq.Eq{"link_id": linkID}})

	query, args, err := deleteQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "удаление связи чата и ссылки", Cause: err}
	}

	result, err := querier.Exec(ctx, query, args...)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "удаление связи чата и ссылки", Cause: err}
	}

	if result.RowsAffected() == 0 {
		return &customerrors.ErrLinkNotFound{URL: url}
	}

	return nil
}

func (r *LinkRepository) countLinkedChats(ctx context.Context, querier txs.Querier, linkID int64) (int, error) {
	countQuery := r.sq.Select("COUNT(*)").From("chat_links").Where(sq.Eq{"link_id": linkID})

	query, args, err := countQuery.ToSql()
	if err != nil {
		return 0, &customerrors.ErrBuildSQLQuery{Operation: "подсчет связанных чатов", Cause: err}
	}

	var count int

	err = querier.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, &customerrors.ErrSQLExecution{Operation: "подсчет связанных чатов", Cause: err}
	}

	return count, nil
}

func (r *LinkRepository) DeleteByURL(ctx context.Context, url string, chatID int64) error {
	return r.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		querier := txs.GetQuerier(ctx, r.db.Pool)

		linkID, err := r.getLinkIDByURL(ctx, querier, url)
		if err != nil {
			return err
		}

		err = r.deleteChatLink(ctx, querier, chatID, linkID, url)
		if err != nil {
			return err
		}

		count, err := r.countLinkedChats(ctx, querier, linkID)
		if err != nil {
			return err
		}

		if count == 0 {
			return r.deleteLinkWithDependencies(ctx, querier, linkID)
		}

		return nil
	})
}

func (r *LinkRepository) Update(ctx context.Context, link *models.Link) error {
	return r.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		querier := txs.GetQuerier(ctx, r.db.Pool)

		updateQuery := r.sq.Update("links").
			Set("url", link.URL).
			Set("type", link.Type).
			Set("last_checked", link.LastChecked).
			Set("last_updated", link.LastUpdated).
			Where(sq.Eq{"id": link.ID})

		query, args, err := updateQuery.ToSql()
		if err != nil {
			return &customerrors.ErrBuildSQLQuery{Operation: "обновление ссылки", Cause: err}
		}

		_, err = querier.Exec(ctx, query, args...)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "обновление ссылки", Cause: err}
		}

		if len(link.Tags) > 0 {
			deleteTagsQuery := r.sq.Delete("link_tags").Where(sq.Eq{"link_id": link.ID})

			query, args, err = deleteTagsQuery.ToSql()
			if err != nil {
				return &customerrors.ErrBuildSQLQuery{Operation: "удаление тегов", Cause: err}
			}

			_, err = querier.Exec(ctx, query, args...)
			if err != nil {
				return &customerrors.ErrSQLExecution{Operation: "удаление тегов", Cause: err}
			}

			err = r.saveTags(ctx, querier, link.ID, link.Tags)
			if err != nil {
				return err
			}
		}

		if len(link.Filters) > 0 {
			deleteFiltersQuery := r.sq.Delete("filters").Where(sq.Eq{"link_id": link.ID})

			query, args, err = deleteFiltersQuery.ToSql()
			if err != nil {
				return &customerrors.ErrBuildSQLQuery{Operation: "удаление фильтров", Cause: err}
			}

			_, err = querier.Exec(ctx, query, args...)
			if err != nil {
				return &customerrors.ErrSQLExecution{Operation: "удаление фильтров", Cause: err}
			}

			err = r.saveFilters(ctx, querier, link.ID, link.Filters)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *LinkRepository) AddChatLink(ctx context.Context, chatID, linkID int64) error {
	return r.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		querier := txs.GetQuerier(ctx, r.db.Pool)

		existsQuery := r.sq.Select("1").
			From("chat_links").
			Where(sq.And{sq.Eq{"chat_id": chatID}, sq.Eq{"link_id": linkID}}).
			Limit(1)

		query, args, err := existsQuery.ToSql()
		if err != nil {
			return &customerrors.ErrBuildSQLQuery{Operation: "проверка существования связи", Cause: err}
		}

		var exists bool

		err = querier.QueryRow(ctx, "SELECT EXISTS("+query+")", args...).Scan(&exists)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "проверка существования связи", Cause: err}
		}

		if exists {
			return nil
		}

		insertQuery := r.sq.Insert("chat_links").
			Columns("chat_id", "link_id").
			Values(chatID, linkID)

		query, args, err = insertQuery.ToSql()
		if err != nil {
			return &customerrors.ErrBuildSQLQuery{Operation: "добавление связи", Cause: err}
		}

		_, err = querier.Exec(ctx, query, args...)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "добавление связи чата и ссылки", Cause: err}
		}

		return nil
	})
}

func (r *LinkRepository) FindDue(ctx context.Context, limit, offset int) ([]*models.Link, error) {
	selectQuery := r.sq.Select("id", "url", "type", "last_checked", "last_updated", "created_at").
		From("links").
		OrderBy("last_checked NULLS FIRST, last_updated NULLS FIRST, created_at ASC, id ASC")

	if limit > 0 {
		selectQuery = selectQuery.Limit(uint64(limit))
	}

	if offset > 0 {
		selectQuery = selectQuery.Offset(uint64(offset))
	}

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, &customerrors.ErrBuildSQLQuery{Operation: "поиск ссылок для проверки", Cause: err}
	}

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "запрос ссылок для проверки", Cause: err}
	}
	defer rows.Close()

	links := make([]*models.Link, 0)

	for rows.Next() {
		var link models.Link

		err = rows.Scan(&link.ID, &link.URL, &link.Type, &link.LastChecked, &link.LastUpdated, &link.CreatedAt)
		if err != nil {
			return nil, &customerrors.ErrSQLExecution{Operation: "сканирование ссылки", Cause: err}
		}

		tags, err := r.GetTags(ctx, link.ID)
		if err != nil {
			return nil, err
		}

		link.Tags = tags

		filters, err := r.GetFilters(ctx, link.ID)
		if err != nil {
			return nil, err
		}

		link.Filters = filters

		links = append(links, &link)
	}

	if err = rows.Err(); err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "обработка результатов", Cause: err}
	}

	return links, nil
}

func (r *LinkRepository) Count(ctx context.Context) (int, error) {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	selectQuery := r.sq.Select("COUNT(*)").
		From("links")

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return 0, &customerrors.ErrBuildSQLQuery{Operation: "подсчет ссылок", Cause: err}
	}

	var count int

	err = querier.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, &customerrors.ErrSQLExecution{Operation: "подсчет ссылок", Cause: err}
	}

	return count, nil
}

func (r *LinkRepository) SaveTags(ctx context.Context, linkID int64, tags []string) error {
	return r.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		querier := txs.GetQuerier(ctx, r.db.Pool)

		deleteQuery := r.sq.Delete("link_tags").
			Where(sq.Eq{"link_id": linkID})

		query, args, err := deleteQuery.ToSql()
		if err != nil {
			return &customerrors.ErrBuildSQLQuery{Operation: "удаление тегов", Cause: err}
		}

		_, err = querier.Exec(ctx, query, args...)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "удаление существующих тегов", Cause: err}
		}

		if len(tags) > 0 {
			err = r.saveTags(ctx, querier, linkID, tags)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *LinkRepository) SaveFilters(ctx context.Context, linkID int64, filters []string) error {
	return r.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		querier := txs.GetQuerier(ctx, r.db.Pool)

		deleteQuery := r.sq.Delete("filters").
			Where(sq.Eq{"link_id": linkID})

		query, args, err := deleteQuery.ToSql()
		if err != nil {
			return &customerrors.ErrBuildSQLQuery{Operation: "удаление фильтров", Cause: err}
		}

		_, err = querier.Exec(ctx, query, args...)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "удаление существующих фильтров", Cause: err}
		}

		if len(filters) > 0 {
			err = r.saveFilters(ctx, querier, linkID, filters)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *LinkRepository) GetAll(ctx context.Context) ([]*models.Link, error) {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	selectQuery := r.sq.Select("id", "url", "type", "last_checked", "last_updated", "created_at").
		From("links").
		OrderBy("id")

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, &customerrors.ErrBuildSQLQuery{Operation: "получение всех ссылок", Cause: err}
	}

	rows, err := querier.Query(ctx, query, args...)
	if err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "получение всех ссылок", Cause: err}
	}
	defer rows.Close()

	links := make([]*models.Link, 0)

	for rows.Next() {
		var link models.Link

		err := rows.Scan(&link.ID, &link.URL, &link.Type, &link.LastChecked, &link.LastUpdated, &link.CreatedAt)
		if err != nil {
			return nil, &customerrors.ErrSQLExecution{Operation: "сканирование ссылки", Cause: err}
		}

		tags, err := r.GetTags(ctx, link.ID)
		if err != nil {
			return nil, err
		}

		link.Tags = tags

		filters, err := r.GetFilters(ctx, link.ID)
		if err != nil {
			return nil, err
		}

		link.Filters = filters

		links = append(links, &link)
	}

	if err = rows.Err(); err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "итерация по ссылкам", Cause: err}
	}

	return links, nil
}
