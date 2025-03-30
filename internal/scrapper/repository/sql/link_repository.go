package sql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/database"
	customerrors "github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
	"github.com/jackc/pgx/v5"
)

type LinkRepository struct {
	db *database.PostgresDB
}

func NewLinkRepository(db *database.PostgresDB) *LinkRepository {
	return &LinkRepository{db: db}
}

func (r *LinkRepository) Save(ctx context.Context, link *models.Link) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("ошибка при начале транзакции: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	var exists bool
	err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM links WHERE url = $1)", link.URL).Scan(&exists)
	if err != nil {
		return fmt.Errorf("ошибка при проверке существования ссылки: %w", err)
	}

	if exists {
		return &customerrors.ErrLinkAlreadyExists{URL: link.URL}
	}

	var id int64
	err = tx.QueryRow(ctx,
		"INSERT INTO links (url, type, last_checked, last_updated, created_at) VALUES ($1, $2, $3, $4, $5) RETURNING id",
		link.URL, link.Type, link.LastChecked, link.LastUpdated, time.Now()).Scan(&id)
	if err != nil {
		return fmt.Errorf("ошибка при сохранении ссылки: %w", err)
	}

	link.ID = id

	if len(link.Tags) > 0 {
		err = r.saveTags(ctx, tx, id, link.Tags)
		if err != nil {
			return fmt.Errorf("ошибка при сохранении тегов: %w", err)
		}
	}

	if len(link.Filters) > 0 {
		err = r.saveFilters(ctx, tx, id, link.Filters)
		if err != nil {
			return fmt.Errorf("ошибка при сохранении фильтров: %w", err)
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("ошибка при фиксации транзакции: %w", err)
	}

	return nil
}

func (r *LinkRepository) saveTags(ctx context.Context, tx pgx.Tx, linkID int64, tags []string) error {
	for _, tag := range tags {
		var tagID int64
		err := tx.QueryRow(ctx,
			"INSERT INTO tags (name) VALUES ($1) ON CONFLICT (name) DO UPDATE SET name = $1 RETURNING id",
			tag).Scan(&tagID)
		if err != nil {
			return fmt.Errorf("ошибка при сохранении тега %s: %w", tag, err)
		}

		_, err = tx.Exec(ctx,
			"INSERT INTO link_tags (link_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
			linkID, tagID)
		if err != nil {
			return fmt.Errorf("ошибка при связывании ссылки с тегом: %w", err)
		}
	}

	return nil
}

func (r *LinkRepository) saveFilters(ctx context.Context, tx pgx.Tx, linkID int64, filters []string) error {
	for _, filter := range filters {
		_, err := tx.Exec(ctx,
			"INSERT INTO filters (value, link_id) VALUES ($1, $2)",
			filter, linkID)
		if err != nil {
			return fmt.Errorf("ошибка при сохранении фильтра %s: %w", filter, err)
		}
	}

	return nil
}

func (r *LinkRepository) FindByID(ctx context.Context, id int64) (*models.Link, error) {
	link := &models.Link{ID: id}

	err := r.db.Pool.QueryRow(ctx,
		"SELECT url, type, last_checked, last_updated, created_at FROM links WHERE id = $1",
		id).Scan(&link.URL, &link.Type, &link.LastChecked, &link.LastUpdated, &link.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &customerrors.ErrLinkNotFound{URL: "ID: " + fmt.Sprint(id)}
		}
		return nil, fmt.Errorf("ошибка при поиске ссылки по ID: %w", err)
	}

	tags, err := r.getTagsByLinkID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("ошибка при загрузке тегов: %w", err)
	}
	link.Tags = tags

	filters, err := r.getFiltersByLinkID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("ошибка при загрузке фильтров: %w", err)
	}
	link.Filters = filters

	return link, nil
}

func (r *LinkRepository) getTagsByLinkID(ctx context.Context, linkID int64) ([]string, error) {
	rows, err := r.db.Pool.Query(ctx,
		"SELECT t.name FROM tags t JOIN link_tags lt ON t.id = lt.tag_id WHERE lt.link_id = $1",
		linkID)
	if err != nil {
		return nil, fmt.Errorf("ошибка при запросе тегов: %w", err)
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, fmt.Errorf("ошибка при сканировании тега: %w", err)
		}
		tags = append(tags, tag)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при обработке результатов запроса тегов: %w", err)
	}

	return tags, nil
}

func (r *LinkRepository) getFiltersByLinkID(ctx context.Context, linkID int64) ([]string, error) {
	rows, err := r.db.Pool.Query(ctx,
		"SELECT value FROM filters WHERE link_id = $1",
		linkID)
	if err != nil {
		return nil, fmt.Errorf("ошибка при запросе фильтров: %w", err)
	}
	defer rows.Close()

	var filters []string
	for rows.Next() {
		var filter string
		if err := rows.Scan(&filter); err != nil {
			return nil, fmt.Errorf("ошибка при сканировании фильтра: %w", err)
		}
		filters = append(filters, filter)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при обработке результатов запроса фильтров: %w", err)
	}

	return filters, nil
}

func (r *LinkRepository) FindByURL(ctx context.Context, url string) (*models.Link, error) {
	var id int64
	err := r.db.Pool.QueryRow(ctx, "SELECT id FROM links WHERE url = $1", url).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &customerrors.ErrLinkNotFound{URL: url}
		}
		return nil, fmt.Errorf("ошибка при поиске ссылки по URL: %w", err)
	}

	return r.FindByID(ctx, id)
}

func (r *LinkRepository) FindByChatID(ctx context.Context, chatID int64) ([]*models.Link, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT l.id, l.url, l.type, l.last_checked, l.last_updated, l.created_at 
		FROM links l 
		JOIN chat_links cl ON l.id = cl.link_id 
		WHERE cl.chat_id = $1`,
		chatID)
	if err != nil {
		return nil, fmt.Errorf("ошибка при запросе ссылок по chat_id: %w", err)
	}
	defer rows.Close()

	var links []*models.Link
	for rows.Next() {
		link := &models.Link{}
		if err := rows.Scan(&link.ID, &link.URL, &link.Type, &link.LastChecked, &link.LastUpdated, &link.CreatedAt); err != nil {
			return nil, fmt.Errorf("ошибка при сканировании ссылки: %w", err)
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
		return nil, fmt.Errorf("ошибка при обработке результатов запроса ссылок: %w", err)
	}

	return links, nil
}

func (r *LinkRepository) DeleteByURL(ctx context.Context, url string, chatID int64) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("ошибка при начале транзакции: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	var linkID int64
	err = tx.QueryRow(ctx, "SELECT id FROM links WHERE url = $1", url).Scan(&linkID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &customerrors.ErrLinkNotFound{URL: url}
		}
		return fmt.Errorf("ошибка при поиске ссылки: %w", err)
	}

	result, err := tx.Exec(ctx,
		"DELETE FROM chat_links WHERE chat_id = $1 AND link_id = $2",
		chatID, linkID)
	if err != nil {
		return fmt.Errorf("ошибка при удалении связи чата и ссылки: %w", err)
	}

	if result.RowsAffected() == 0 {
		return &customerrors.ErrLinkNotFound{URL: url}
	}

	var count int
	err = tx.QueryRow(ctx,
		"SELECT COUNT(*) FROM chat_links WHERE link_id = $1",
		linkID).Scan(&count)
	if err != nil {
		return fmt.Errorf("ошибка при проверке использования ссылки: %w", err)
	}

	if count == 0 {
		_, err = tx.Exec(ctx, "DELETE FROM links WHERE id = $1", linkID)
		if err != nil {
			return fmt.Errorf("ошибка при удалении ссылки: %w", err)
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("ошибка при фиксации транзакции: %w", err)
	}

	return nil
}

func (r *LinkRepository) Update(ctx context.Context, link *models.Link) error {
	_, err := r.db.Pool.Exec(ctx,
		"UPDATE links SET last_checked = $1, last_updated = $2 WHERE id = $3",
		link.LastChecked, link.LastUpdated, link.ID)
	if err != nil {
		return fmt.Errorf("ошибка при обновлении ссылки: %w", err)
	}

	return nil
}

func (r *LinkRepository) AddChatLink(ctx context.Context, chatID, linkID int64) error {
	_, err := r.db.Pool.Exec(ctx,
		"INSERT INTO chat_links (chat_id, link_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
		chatID, linkID)
	if err != nil {
		return fmt.Errorf("ошибка при связывании чата и ссылки: %w", err)
	}

	return nil
}

func (r *LinkRepository) FindDue(ctx context.Context, limit, offset int) ([]*models.Link, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, url, type, last_checked, last_updated, created_at 
		FROM links 
		ORDER BY last_checked NULLS FIRST, id 
		LIMIT $1 OFFSET $2`,
		limit, offset)
	if err != nil {
		return nil, fmt.Errorf("ошибка при запросе ссылок для проверки: %w", err)
	}
	defer rows.Close()

	var links []*models.Link
	for rows.Next() {
		link := &models.Link{}
		if err := rows.Scan(&link.ID, &link.URL, &link.Type, &link.LastChecked, &link.LastUpdated, &link.CreatedAt); err != nil {
			return nil, fmt.Errorf("ошибка при сканировании ссылки: %w", err)
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
		return nil, fmt.Errorf("ошибка при обработке результатов запроса ссылок: %w", err)
	}

	return links, nil
}

func (r *LinkRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM links").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("ошибка при подсчете ссылок: %w", err)
	}

	return count, nil
}

func (r *LinkRepository) SaveTags(ctx context.Context, linkID int64, tags []string) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("ошибка при начале транзакции: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	_, err = tx.Exec(ctx, "DELETE FROM link_tags WHERE link_id = $1", linkID)
	if err != nil {
		return fmt.Errorf("ошибка при удалении существующих тегов: %w", err)
	}

	if len(tags) > 0 {
		err = r.saveTags(ctx, tx, linkID, tags)
		if err != nil {
			return fmt.Errorf("ошибка при сохранении тегов: %w", err)
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("ошибка при фиксации транзакции: %w", err)
	}

	return nil
}

func (r *LinkRepository) SaveFilters(ctx context.Context, linkID int64, filters []string) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("ошибка при начале транзакции: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	_, err = tx.Exec(ctx, "DELETE FROM filters WHERE link_id = $1", linkID)
	if err != nil {
		return fmt.Errorf("ошибка при удалении существующих фильтров: %w", err)
	}

	if len(filters) > 0 {
		err = r.saveFilters(ctx, tx, linkID, filters)
		if err != nil {
			return fmt.Errorf("ошибка при сохранении фильтров: %w", err)
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("ошибка при фиксации транзакции: %w", err)
	}

	return nil
}

func (r *LinkRepository) GetAll(ctx context.Context) ([]*models.Link, error) {
	rows, err := r.db.Pool.Query(ctx,
		"SELECT id, url, type, last_checked, last_updated, created_at FROM links ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении всех ссылок: %w", err)
	}
	defer rows.Close()

	links := make([]*models.Link, 0)
	for rows.Next() {
		link := &models.Link{}
		err := rows.Scan(&link.ID, &link.URL, &link.Type, &link.LastChecked, &link.LastUpdated, &link.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("ошибка при сканировании ссылки: %w", err)
		}

		tags, err := r.getTagsByLinkID(ctx, link.ID)
		if err != nil {
			return nil, fmt.Errorf("ошибка при загрузке тегов: %w", err)
		}
		link.Tags = tags

		filters, err := r.getFiltersByLinkID(ctx, link.ID)
		if err != nil {
			return nil, fmt.Errorf("ошибка при загрузке фильтров: %w", err)
		}
		link.Filters = filters

		links = append(links, link)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при итерации по ссылкам: %w", err)
	}

	return links, nil
}
