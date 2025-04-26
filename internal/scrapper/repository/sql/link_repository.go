package sql

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/database"
	customerrors "github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
	"github.com/central-university-dev/go-Matthew11K/pkg/txs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type LinkRepository struct {
	db *database.PostgresDB
}

func NewLinkRepository(db *database.PostgresDB) *LinkRepository {
	return &LinkRepository{db: db}
}

func (r *LinkRepository) Save(ctx context.Context, link *models.Link) error {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	now := time.Now()
	if link.CreatedAt.IsZero() {
		link.CreatedAt = now
	}

	var id int64

	err := querier.QueryRow(ctx,
		"INSERT INTO links (url, type, last_checked, last_updated, created_at) VALUES ($1, $2, $3, $4, $5) RETURNING id",
		link.URL, link.Type, link.LastChecked, link.LastUpdated, link.CreatedAt).Scan(&id)
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
		values := make([]any, 0, len(tags))

		placeholders := make([]string, 0, len(tags))
		for i, tag := range tags {
			placeholders = append(placeholders, fmt.Sprintf("($%d)", i+1))
			values = append(values, tag)
		}

		query := fmt.Sprintf("INSERT INTO tags (name) VALUES %s ON CONFLICT (name) DO NOTHING", strings.Join(placeholders, ", "))

		_, err := querier.Exec(ctx, query, values...)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "batch-сохранение тегов", Cause: err}
		}
	}

	{
		var values []any

		placeholders := make([]string, 0, len(tags))
		for i, tag := range tags {
			placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
			values = append(values, tag)
		}

		query := fmt.Sprintf("SELECT id, name FROM tags WHERE name IN (%s)", strings.Join(placeholders, ", "))

		rows, err := querier.Query(ctx, query, values...)
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

		var linkTagValues []any

		linkTagPlaceholders := make([]string, 0, len(tags))

		for i, tag := range tags {
			id, ok := ids[tag]
			if !ok {
				return &customerrors.ErrSQLExecution{Operation: "batch-связывание ссылки с тегом", Cause: fmt.Errorf("id не найден для тега %s", tag)}
			}

			linkTagPlaceholders = append(linkTagPlaceholders, fmt.Sprintf("($%d, $%d)", 2*i+1, 2*i+2))
			linkTagValues = append(linkTagValues, linkID, id)
		}

		query = fmt.Sprintf("INSERT INTO link_tags (link_id, tag_id) VALUES %s ON CONFLICT DO NOTHING", strings.Join(linkTagPlaceholders, ", "))

		_, err = querier.Exec(ctx, query, linkTagValues...)
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

	values := make([]any, 0, len(filters)*2)

	placeholders := make([]string, 0, len(filters))
	for i, filter := range filters {
		placeholders = append(placeholders, fmt.Sprintf("($%d, $%d)", 2*i+1, 2*i+2))
		values = append(values, filter, linkID)
	}

	query := fmt.Sprintf("INSERT INTO filters (value, link_id) VALUES %s", strings.Join(placeholders, ", "))

	_, err := querier.Exec(ctx, query, values...)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "batch-сохранение фильтров", Cause: err}
	}

	return nil
}

func (r *LinkRepository) FindByID(ctx context.Context, id int64) (*models.Link, error) {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	row := querier.QueryRow(ctx, `
		SELECT l.id, l.url, l.type, l.last_checked, l.last_updated, l.created_at,
			COALESCE(array_agg(DISTINCT t.name) FILTER (WHERE t.name IS NOT NULL), '{}') AS tags,
			COALESCE(array_agg(DISTINCT f.value) FILTER (WHERE f.value IS NOT NULL), '{}') AS filters
		FROM links l
		LEFT JOIN link_tags lt ON l.id = lt.link_id
		LEFT JOIN tags t ON lt.tag_id = t.id
		LEFT JOIN filters f ON l.id = f.link_id
		WHERE l.id = $1
		GROUP BY l.id
	`, id)

	var link models.Link

	var tagsArr, filtersArr []string

	err := row.Scan(
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
			return nil, &customerrors.ErrLinkNotFound{URL: "ID: " + fmt.Sprint(id)}
		}

		return nil, &customerrors.ErrSQLExecution{Operation: "поиск ссылки по ID", Cause: err}
	}

	link.Tags = tagsArr
	link.Filters = filtersArr

	return &link, nil
}

func (r *LinkRepository) FindByURL(ctx context.Context, url string) (*models.Link, error) {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	var id int64

	err := querier.QueryRow(ctx, "SELECT id FROM links WHERE url = $1", url).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &customerrors.ErrLinkNotFound{URL: url}
		}

		return nil, &customerrors.ErrSQLExecution{Operation: "поиск ссылки по URL", Cause: err}
	}

	return r.FindByID(ctx, id)
}

func (r *LinkRepository) FindByChatID(ctx context.Context, chatID int64) ([]*models.Link, error) {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	rows, err := querier.Query(ctx, `
		SELECT l.id, l.url, l.type, l.last_checked, l.last_updated, l.created_at,
			COALESCE(array_agg(DISTINCT t.name) FILTER (WHERE t.name IS NOT NULL), '{}') AS tags,
			COALESCE(array_agg(DISTINCT f.value) FILTER (WHERE f.value IS NOT NULL), '{}') AS filters
		FROM links l
		JOIN chat_links cl ON l.id = cl.link_id
		LEFT JOIN link_tags lt ON l.id = lt.link_id
		LEFT JOIN tags t ON lt.tag_id = t.id
		LEFT JOIN filters f ON l.id = f.link_id
		WHERE cl.chat_id = $1
		GROUP BY l.id
	`, chatID)
	if err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "запрос ссылок по ID чата", Cause: err}
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
		return nil, &customerrors.ErrSQLExecution{Operation: "обработка результатов запроса ссылок", Cause: err}
	}

	return links, nil
}

func (r *LinkRepository) DeleteByURL(ctx context.Context, url string, chatID int64) error {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	var linkID int64

	err := querier.QueryRow(ctx, "SELECT id FROM links WHERE url = $1", url).Scan(&linkID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &customerrors.ErrLinkNotFound{URL: url}
		}

		return &customerrors.ErrSQLExecution{Operation: "получение ID ссылки", Cause: err}
	}

	result, err := querier.Exec(ctx, "DELETE FROM chat_links WHERE chat_id = $1 AND link_id = $2", chatID, linkID)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "удаление связи чата с ссылкой", Cause: err}
	}

	if result.RowsAffected() == 0 {
		return &customerrors.ErrLinkNotInChat{ChatID: chatID, LinkID: linkID}
	}

	var count int

	err = querier.QueryRow(ctx, "SELECT COUNT(*) FROM chat_links WHERE link_id = $1", linkID).Scan(&count)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "проверка использования ссылки", Cause: err}
	}

	if count == 0 {
		_, err = querier.Exec(ctx, "DELETE FROM links WHERE id = $1", linkID)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "удаление ссылки", Cause: err}
		}
	}

	return nil
}

func (r *LinkRepository) Update(ctx context.Context, link *models.Link) error {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	result, err := querier.Exec(ctx, `
		UPDATE links 
		SET url = $1, type = $2, last_checked = $3, last_updated = $4, created_at = $5
		WHERE id = $6
	`, link.URL, link.Type, link.LastChecked, link.LastUpdated, link.CreatedAt, link.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return &customerrors.ErrLinkAlreadyExists{URL: link.URL}
		}

		return &customerrors.ErrSQLExecution{Operation: "обновление ссылки", Cause: err}
	}

	if result.RowsAffected() == 0 {
		return &customerrors.ErrLinkNotFound{URL: link.URL}
	}

	if len(link.Tags) > 0 {
		if err := r.SaveTags(ctx, link.ID, link.Tags); err != nil {
			return err
		}
	}

	if len(link.Filters) > 0 {
		if err := r.SaveFilters(ctx, link.ID, link.Filters); err != nil {
			return err
		}
	}

	return nil
}

func (r *LinkRepository) AddChatLink(ctx context.Context, chatID, linkID int64) error {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	_, err := querier.Exec(ctx,
		"INSERT INTO chat_links (chat_id, link_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
		chatID, linkID)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "добавление связи чата с ссылкой", Cause: err}
	}

	return nil
}

func (r *LinkRepository) FindDue(ctx context.Context, limit, offset int) ([]*models.Link, error) {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	query := `
		SELECT id, url, type, last_checked, last_updated, created_at 
		FROM links 
		ORDER BY last_checked NULLS FIRST, last_updated NULLS FIRST, created_at ASC, id ASC`

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	if offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", offset)
	}

	rows, err := querier.Query(ctx, query)
	if err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "запрос ссылок для проверки", Cause: err}
	}
	defer rows.Close()

	var links []*models.Link

	for rows.Next() {
		link := &models.Link{}

		err := rows.Scan(
			&link.ID,
			&link.URL,
			&link.Type,
			&link.LastChecked,
			&link.LastUpdated,
			&link.CreatedAt,
		)
		if err != nil {
			return nil, &customerrors.ErrSQLExecution{Operation: "сканирование ссылки", Cause: err}
		}

		links = append(links, link)
	}

	if err = rows.Err(); err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "обработка результатов запроса ссылок", Cause: err}
	}

	return links, nil
}

func (r *LinkRepository) Count(ctx context.Context) (int, error) {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	var count int

	err := querier.QueryRow(ctx, "SELECT COUNT(*) FROM links").Scan(&count)
	if err != nil {
		return 0, &customerrors.ErrSQLExecution{Operation: "подсчёт ссылок", Cause: err}
	}

	return count, nil
}

func (r *LinkRepository) SaveTags(ctx context.Context, linkID int64, tags []string) error {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	_, err := querier.Exec(ctx, "DELETE FROM link_tags WHERE link_id = $1", linkID)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "удаление существующих тегов", Cause: err}
	}

	if len(tags) == 0 {
		return nil
	}

	return r.saveTags(ctx, querier, linkID, tags)
}

func (r *LinkRepository) SaveFilters(ctx context.Context, linkID int64, filters []string) error {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	_, err := querier.Exec(ctx, "DELETE FROM filters WHERE link_id = $1", linkID)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "удаление существующих фильтров", Cause: err}
	}

	if len(filters) == 0 {
		return nil
	}

	return r.saveFilters(ctx, querier, linkID, filters)
}

func (r *LinkRepository) GetAll(ctx context.Context) ([]*models.Link, error) {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	rows, err := querier.Query(ctx, `
		SELECT l.id, l.url, l.type, l.last_checked, l.last_updated, l.created_at,
			COALESCE(array_agg(DISTINCT t.name) FILTER (WHERE t.name IS NOT NULL), '{}') AS tags,
			COALESCE(array_agg(DISTINCT f.value) FILTER (WHERE f.value IS NOT NULL), '{}') AS filters
		FROM links l
		LEFT JOIN link_tags lt ON l.id = lt.link_id
		LEFT JOIN tags t ON lt.tag_id = t.id
		LEFT JOIN filters f ON l.id = f.link_id
		GROUP BY l.id
		ORDER BY l.id
	`)
	if err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "запрос всех ссылок", Cause: err}
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
		return nil, &customerrors.ErrSQLExecution{Operation: "обработка результатов запроса ссылок", Cause: err}
	}

	return links, nil
}
