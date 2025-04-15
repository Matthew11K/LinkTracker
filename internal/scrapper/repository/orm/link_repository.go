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
	querier := txs.GetQuerier(ctx, r.db.Pool)

	now := time.Now()
	if link.CreatedAt.IsZero() {
		link.CreatedAt = now
	}

	insertQuery := r.sq.Insert("links").
		Columns("url", "type", "last_checked", "last_updated", "created_at").
		Values(link.URL, link.Type, link.LastChecked, link.LastUpdated, link.CreatedAt).
		Suffix("RETURNING id")

	query, args, err := insertQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "вставка ссылки", Cause: err}
	}

	var id int64

	err = querier.QueryRow(ctx, query, args...).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return &customerrors.ErrLinkAlreadyExists{URL: link.URL}
		}

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
}

//nolint:funlen // Функция целостная
func (r *LinkRepository) saveTags(ctx context.Context, querier txs.Querier, linkID int64, tags []string) error {
	if len(tags) == 0 {
		return nil
	}

	{
		insert := r.sq.Insert("tags").Columns("name")
		for _, tag := range tags {
			insert = insert.Values(tag)
		}

		insert = insert.Suffix("ON CONFLICT (name) DO NOTHING")

		query, args, err := insert.ToSql()
		if err != nil {
			return &customerrors.ErrBuildSQLQuery{Operation: "batch-сохранение тегов", Cause: err}
		}

		_, err = querier.Exec(ctx, query, args...)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "batch-сохранение тегов", Cause: err}
		}
	}

	{
		selectQuery := r.sq.Select("id", "name").From("tags").Where(sq.Eq{"name": tags})

		query, args, err := selectQuery.ToSql()
		if err != nil {
			return &customerrors.ErrBuildSQLQuery{Operation: "получение id тегов", Cause: err}
		}

		rows, err := querier.Query(ctx, query, args...)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "получение id тегов", Cause: err}
		}

		defer rows.Close()

		ids := make(map[string]int64, len(tags))

		for rows.Next() {
			var id int64

			var name string
			if err := rows.Scan(&id, &name); err != nil {
				return &customerrors.ErrSQLExecution{Operation: "сканирование id тегов", Cause: err}
			}

			ids[name] = id
		}

		if err := rows.Err(); err != nil {
			return &customerrors.ErrSQLExecution{Operation: "чтение id тегов", Cause: err}
		}

		insert := r.sq.Insert("link_tags").Columns("link_id", "tag_id")

		for _, tag := range tags {
			id, ok := ids[tag]
			if !ok {
				return &customerrors.ErrSQLExecution{Operation: "batch-связывание ссылки с тегом", Cause: fmt.Errorf("id не найден для тега %s", tag)}
			}

			insert = insert.Values(linkID, id)
		}

		insert = insert.Suffix("ON CONFLICT DO NOTHING")

		query, args, err = insert.ToSql()
		if err != nil {
			return &customerrors.ErrBuildSQLQuery{Operation: "batch-связывание ссылки с тегами", Cause: err}
		}

		_, err = querier.Exec(ctx, query, args...)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "batch-связывание ссылки с тегами", Cause: err}
		}
	}

	return nil
}

func (r *LinkRepository) saveFilters(ctx context.Context, querier txs.Querier, linkID int64, filters []string) error {
	if len(filters) == 0 {
		return nil
	}

	insert := r.sq.Insert("filters").Columns("value", "link_id")
	for _, filter := range filters {
		insert = insert.Values(filter, linkID)
	}

	query, args, err := insert.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "batch-сохранение фильтров", Cause: err}
	}

	_, err = querier.Exec(ctx, query, args...)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "batch-сохранение фильтров", Cause: err}
	}

	return nil
}

func (r *LinkRepository) FindByID(ctx context.Context, id int64) (*models.Link, error) {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	selectQuery := r.sq.Select(
		"l.id", "l.url", "l.type", "l.last_checked", "l.last_updated", "l.created_at",
		"COALESCE(array_agg(DISTINCT t.name) FILTER (WHERE t.name IS NOT NULL), '{}') AS tags",
		"COALESCE(array_agg(DISTINCT f.value) FILTER (WHERE f.value IS NOT NULL), '{}') AS filters",
	).
		From("links l").
		LeftJoin("link_tags lt ON l.id = lt.link_id").
		LeftJoin("tags t ON lt.tag_id = t.id").
		LeftJoin("filters f ON l.id = f.link_id").
		Where(sq.Eq{"l.id": id}).
		GroupBy("l.id")

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, &customerrors.ErrBuildSQLQuery{Operation: "поиск ссылки по ID", Cause: err}
	}

	row := querier.QueryRow(ctx, query, args...)

	var link models.Link

	var tagsArr, filtersArr []string

	err = row.Scan(
		&link.ID,
		&link.URL,
		&link.Type,
		&link.LastChecked,
		&link.LastUpdated,
		&link.CreatedAt,
		&tagsArr,
		&filtersArr,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &customerrors.ErrLinkNotFound{URL: fmt.Sprintf("ID: %d", id)}
		}

		return nil, &customerrors.ErrSQLExecution{Operation: "поиск ссылки по ID", Cause: err}
	}

	link.Tags = tagsArr
	link.Filters = filtersArr

	return &link, nil
}

func (r *LinkRepository) FindByURL(ctx context.Context, url string) (*models.Link, error) {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	selectQuery := r.sq.Select(
		"l.id", "l.url", "l.type", "l.last_checked", "l.last_updated", "l.created_at",
		"COALESCE(array_agg(DISTINCT t.name) FILTER (WHERE t.name IS NOT NULL), '{}') AS tags",
		"COALESCE(array_agg(DISTINCT f.value) FILTER (WHERE f.value IS NOT NULL), '{}') AS filters",
	).
		From("links l").
		LeftJoin("link_tags lt ON l.id = lt.link_id").
		LeftJoin("tags t ON lt.tag_id = t.id").
		LeftJoin("filters f ON l.id = f.link_id").
		Where(sq.Eq{"l.url": url}).
		GroupBy("l.id").
		Limit(1)

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, &customerrors.ErrBuildSQLQuery{Operation: "поиск ссылки по URL", Cause: err}
	}

	row := querier.QueryRow(ctx, query, args...)

	var link models.Link

	var tagsArr, filtersArr []string

	err = row.Scan(
		&link.ID,
		&link.URL,
		&link.Type,
		&link.LastChecked,
		&link.LastUpdated,
		&link.CreatedAt,
		&tagsArr,
		&filtersArr,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &customerrors.ErrLinkNotFound{URL: url}
		}

		return nil, &customerrors.ErrSQLExecution{Operation: "поиск ссылки по URL", Cause: err}
	}

	link.Tags = tagsArr
	link.Filters = filtersArr

	return &link, nil
}

func (r *LinkRepository) FindByChatID(ctx context.Context, chatID int64) ([]*models.Link, error) {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	selectQuery := r.sq.Select(
		"l.id", "l.url", "l.type", "l.last_checked", "l.last_updated", "l.created_at",
		"COALESCE(array_agg(DISTINCT t.name) FILTER (WHERE t.name IS NOT NULL), '{}') AS tags",
		"COALESCE(array_agg(DISTINCT f.value) FILTER (WHERE f.value IS NOT NULL), '{}') AS filters",
	).
		From("links l").
		Join("chat_links cl ON l.id = cl.link_id").
		LeftJoin("link_tags lt ON l.id = lt.link_id").
		LeftJoin("tags t ON lt.tag_id = t.id").
		LeftJoin("filters f ON l.id = f.link_id").
		Where(sq.Eq{"cl.chat_id": chatID}).
		GroupBy("l.id")

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

		var tagsArr, filtersArr []string

		err := rows.Scan(
			&link.ID,
			&link.URL,
			&link.Type,
			&link.LastChecked,
			&link.LastUpdated,
			&link.CreatedAt,
			&tagsArr,
			&filtersArr,
		)
		if err != nil {
			return nil, &customerrors.ErrSQLExecution{Operation: "сканирование ссылки", Cause: err}
		}

		link.Tags = tagsArr
		link.Filters = filtersArr
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

func (r *LinkRepository) Update(ctx context.Context, link *models.Link) error {
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
}

func (r *LinkRepository) AddChatLink(ctx context.Context, chatID, linkID int64) error {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	insertQuery := r.sq.Insert("chat_links").
		Columns("chat_id", "link_id").
		Values(chatID, linkID).
		Suffix("ON CONFLICT DO NOTHING")

	query, args, err := insertQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "добавление связи", Cause: err}
	}

	_, err = querier.Exec(ctx, query, args...)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "добавление связи чата и ссылки", Cause: err}
	}

	return nil
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
}

func (r *LinkRepository) SaveFilters(ctx context.Context, linkID int64, filters []string) error {
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
}

func (r *LinkRepository) GetAll(ctx context.Context) ([]*models.Link, error) {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	selectQuery := r.sq.Select(
		"l.id", "l.url", "l.type", "l.last_checked", "l.last_updated", "l.created_at",
		"COALESCE(array_agg(DISTINCT t.name) FILTER (WHERE t.name IS NOT NULL), '{}') AS tags",
		"COALESCE(array_agg(DISTINCT f.value) FILTER (WHERE f.value IS NOT NULL), '{}') AS filters",
	).
		From("links l").
		LeftJoin("link_tags lt ON l.id = lt.link_id").
		LeftJoin("tags t ON lt.tag_id = t.id").
		LeftJoin("filters f ON l.id = f.link_id").
		GroupBy("l.id").
		OrderBy("l.id")

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, &customerrors.ErrBuildSQLQuery{Operation: "получение всех ссылок", Cause: err}
	}

	rows, err := querier.Query(ctx, query, args...)
	if err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "получение всех ссылок", Cause: err}
	}
	defer rows.Close()

	var links []*models.Link

	for rows.Next() {
		var link models.Link

		var tagsArr, filtersArr []string

		err := rows.Scan(
			&link.ID,
			&link.URL,
			&link.Type,
			&link.LastChecked,
			&link.LastUpdated,
			&link.CreatedAt,
			&tagsArr,
			&filtersArr,
		)
		if err != nil {
			return nil, &customerrors.ErrSQLExecution{Operation: "сканирование ссылки", Cause: err}
		}

		link.Tags = tagsArr
		link.Filters = filtersArr
		links = append(links, &link)
	}

	if err = rows.Err(); err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "обработка результатов", Cause: err}
	}

	return links, nil
}

func (r *LinkRepository) DeleteByURL(ctx context.Context, url string, chatID int64) error {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	linkID, err := r.getLinkIDByURL(ctx, querier, url)
	if err != nil {
		return err
	}

	err = r.deleteChatLink(ctx, querier, chatID, linkID, url)
	if err != nil {
		return err
	}

	return nil
}
