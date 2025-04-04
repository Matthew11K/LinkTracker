package sql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/database"
	customerrors "github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
	"github.com/central-university-dev/go-Matthew11K/pkg/txs"
	"github.com/jackc/pgx/v5"
)

type LinkRepository struct {
	db        *database.PostgresDB
	txManager *txs.TxManager
}

func NewLinkRepository(db *database.PostgresDB, txManager *txs.TxManager) *LinkRepository {
	return &LinkRepository{db: db, txManager: txManager}
}

func (r *LinkRepository) Save(ctx context.Context, link *models.Link) error {
	return r.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		querier := txs.GetQuerier(ctx, r.db.Pool)

		var exists bool

		err := querier.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM links WHERE url = $1)", link.URL).Scan(&exists)
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

		var id int64

		err = querier.QueryRow(ctx,
			"INSERT INTO links (url, type, last_checked, last_updated, created_at) VALUES ($1, $2, $3, $4, $5) RETURNING id",
			link.URL, link.Type, link.LastChecked, link.LastUpdated, link.CreatedAt).Scan(&id)
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
		var tagID int64

		err := querier.QueryRow(ctx,
			"INSERT INTO tags (name) VALUES ($1) ON CONFLICT (name) DO UPDATE SET name = $1 RETURNING id",
			tag).Scan(&tagID)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "сохранение тега " + tag, Cause: err}
		}

		_, err = querier.Exec(ctx,
			"INSERT INTO link_tags (link_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
			linkID, tagID)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "связывание ссылки с тегом", Cause: err}
		}
	}

	return nil
}

func (r *LinkRepository) saveFilters(ctx context.Context, querier txs.Querier, linkID int64, filters []string) error {
	for _, filter := range filters {
		_, err := querier.Exec(ctx,
			"INSERT INTO filters (value, link_id) VALUES ($1, $2)",
			filter, linkID)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "сохранение фильтра " + filter, Cause: err}
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
	rows, err := r.db.Pool.Query(ctx,
		"SELECT t.name FROM tags t JOIN link_tags lt ON t.id = lt.tag_id WHERE lt.link_id = $1",
		linkID)
	if err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "запрос тегов", Cause: err}
	}
	defer rows.Close()

	var tags []string

	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, &customerrors.ErrSQLExecution{Operation: "сканирование тега", Cause: err}
		}

		tags = append(tags, tag)
	}

	if err := rows.Err(); err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "обработка результатов запроса тегов", Cause: err}
	}

	return tags, nil
}

func (r *LinkRepository) getFiltersByLinkID(ctx context.Context, linkID int64) ([]string, error) {
	rows, err := r.db.Pool.Query(ctx,
		"SELECT value FROM filters WHERE link_id = $1",
		linkID)
	if err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "запрос фильтров", Cause: err}
	}
	defer rows.Close()

	var filters []string

	for rows.Next() {
		var filter string
		if err := rows.Scan(&filter); err != nil {
			return nil, &customerrors.ErrSQLExecution{Operation: "сканирование фильтра", Cause: err}
		}

		filters = append(filters, filter)
	}

	if err := rows.Err(); err != nil {
		return nil, &customerrors.ErrSQLExecution{Operation: "обработка результатов запроса фильтров", Cause: err}
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

		return nil, &customerrors.ErrSQLExecution{Operation: "поиск ссылки по URL", Cause: err}
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
		return nil, &customerrors.ErrSQLExecution{Operation: "запрос ссылок по chat_id", Cause: err}
	}
	defer rows.Close()

	var links []*models.Link

	for rows.Next() {
		link := &models.Link{}
		if err := rows.Scan(&link.ID, &link.URL, &link.Type, &link.LastChecked, &link.LastUpdated, &link.CreatedAt); err != nil {
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
		return nil, &customerrors.ErrSQLExecution{Operation: "обработка результатов запроса ссылок", Cause: err}
	}

	return links, nil
}

func (r *LinkRepository) DeleteByURL(ctx context.Context, url string, chatID int64) error {
	return r.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		querier := txs.GetQuerier(ctx, r.db.Pool)

		var linkID int64

		err := querier.QueryRow(ctx, "SELECT id FROM links WHERE url = $1", url).Scan(&linkID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return &customerrors.ErrLinkNotFound{URL: url}
			}

			return &customerrors.ErrSQLExecution{Operation: "поиск ссылки по URL", Cause: err}
		}

		var chatLinkExists bool

		err = querier.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM chat_links WHERE chat_id = $1 AND link_id = $2)",
			chatID, linkID).Scan(&chatLinkExists)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "проверка связи чата и ссылки", Cause: err}
		}

		if !chatLinkExists {
			return &customerrors.ErrLinkNotFound{URL: url}
		}

		_, err = querier.Exec(ctx,
			"DELETE FROM chat_links WHERE chat_id = $1 AND link_id = $2",
			chatID, linkID)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "удаление связи чата и ссылки", Cause: err}
		}

		var chatCount int

		err = querier.QueryRow(ctx,
			"SELECT COUNT(*) FROM chat_links WHERE link_id = $1",
			linkID).Scan(&chatCount)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "подсчёт чатов для ссылки", Cause: err}
		}

		if chatCount == 0 {
			_, err = querier.Exec(ctx,
				"DELETE FROM link_tags WHERE link_id = $1",
				linkID)
			if err != nil {
				return &customerrors.ErrSQLExecution{Operation: "удаление тегов ссылки", Cause: err}
			}

			_, err = querier.Exec(ctx,
				"DELETE FROM filters WHERE link_id = $1",
				linkID)
			if err != nil {
				return &customerrors.ErrSQLExecution{Operation: "удаление фильтров ссылки", Cause: err}
			}

			_, err = querier.Exec(ctx,
				"DELETE FROM links WHERE id = $1",
				linkID)
			if err != nil {
				return &customerrors.ErrSQLExecution{Operation: "удаление ссылки", Cause: err}
			}
		}

		return nil
	})
}

func (r *LinkRepository) Update(ctx context.Context, link *models.Link) error {
	return r.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		querier := txs.GetQuerier(ctx, r.db.Pool)

		_, err := querier.Exec(ctx,
			"UPDATE links SET url = $1, type = $2, last_checked = $3, last_updated = $4 WHERE id = $5",
			link.URL, link.Type, link.LastChecked, link.LastUpdated, link.ID)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "обновление ссылки", Cause: err}
		}

		if len(link.Tags) > 0 {
			_, err = querier.Exec(ctx, "DELETE FROM link_tags WHERE link_id = $1", link.ID)
			if err != nil {
				return &customerrors.ErrSQLExecution{Operation: "удаление существующих тегов", Cause: err}
			}

			err = r.saveTags(ctx, querier, link.ID, link.Tags)
			if err != nil {
				return err
			}
		}

		if len(link.Filters) > 0 {
			_, err = querier.Exec(ctx, "DELETE FROM filters WHERE link_id = $1", link.ID)
			if err != nil {
				return &customerrors.ErrSQLExecution{Operation: "удаление существующих фильтров", Cause: err}
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

		var exists bool

		err := querier.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM chat_links WHERE chat_id = $1 AND link_id = $2)",
			chatID, linkID).Scan(&exists)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "проверка существования связи", Cause: err}
		}

		if exists {
			// Связь уже существует, ничего не делаем
			return nil
		}

		_, err = querier.Exec(ctx,
			"INSERT INTO chat_links (chat_id, link_id) VALUES ($1, $2)",
			chatID, linkID)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "добавление связи чата и ссылки", Cause: err}
		}

		return nil
	})
}

func (r *LinkRepository) FindDue(ctx context.Context, limit, offset int) ([]*models.Link, error) {
	var result []*models.Link

	err := r.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		querier := txs.GetQuerier(ctx, r.db.Pool)

		rows, err := querier.Query(ctx,
			`SELECT id, url, type, last_checked, last_updated, created_at 
			FROM links 
			ORDER BY last_checked NULLS FIRST, last_updated NULLS FIRST, created_at ASC, id ASC
			LIMIT $1 OFFSET $2`,
			limit, offset)
		if err != nil {
			return &customerrors.ErrSQLExecution{Operation: "запрос ссылок для проверки", Cause: err}
		}
		defer rows.Close()

		links := make([]*models.Link, 0)

		for rows.Next() {
			link := &models.Link{}
			if err := rows.Scan(&link.ID, &link.URL, &link.Type, &link.LastChecked, &link.LastUpdated, &link.CreatedAt); err != nil {
				return &customerrors.ErrSQLExecution{Operation: "сканирование ссылки", Cause: err}
			}

			tags, err := r.getTagsByLinkID(ctx, link.ID)
			if err != nil {
				return err
			}

			link.Tags = tags

			filters, err := r.getFiltersByLinkID(ctx, link.ID)
			if err != nil {
				return err
			}

			link.Filters = filters

			links = append(links, link)
		}

		if err := rows.Err(); err != nil {
			return &customerrors.ErrSQLExecution{Operation: "обработка результатов запроса ссылок", Cause: err}
		}

		result = links

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (r *LinkRepository) Count(ctx context.Context) (int, error) {
	var count int

	err := r.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM links").Scan(&count)
	if err != nil {
		return 0, &customerrors.ErrSQLExecution{Operation: "подсчет ссылок", Cause: err}
	}

	return count, nil
}

func (r *LinkRepository) SaveTags(ctx context.Context, linkID int64, tags []string) error {
	return r.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		querier := txs.GetQuerier(ctx, r.db.Pool)

		_, err := querier.Exec(ctx, "DELETE FROM link_tags WHERE link_id = $1", linkID)
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

		_, err := querier.Exec(ctx, "DELETE FROM filters WHERE link_id = $1", linkID)
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
	rows, err := r.db.Pool.Query(ctx,
		"SELECT id, url, type, last_checked, last_updated, created_at FROM links ORDER BY id")
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
