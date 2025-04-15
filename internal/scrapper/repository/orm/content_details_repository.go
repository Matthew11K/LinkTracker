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

type ContentDetailsRepository struct {
	db *database.PostgresDB
	sq sq.StatementBuilderType
}

func NewContentDetailsRepository(db *database.PostgresDB) *ContentDetailsRepository {
	return &ContentDetailsRepository{
		db: db,
		sq: sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}

func (r *ContentDetailsRepository) Save(ctx context.Context, details *models.ContentDetails) error {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	now := time.Now()

	insertQuery := r.sq.Insert("content_details").
		Columns("link_id", "title", "author", "updated_at", "content_text", "link_type", "created_at").
		Values(details.LinkID, details.Title, details.Author, details.UpdatedAt, details.ContentText, string(details.LinkType), now).
		Suffix("ON CONFLICT (link_id) DO UPDATE SET title = EXCLUDED.title, author = EXCLUDED.author, " +
			"updated_at = EXCLUDED.updated_at, content_text = EXCLUDED.content_text")

	query, args, err := insertQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "вставка/обновление деталей контента", Cause: err}
	}

	_, err = querier.Exec(ctx, query, args...)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "сохранение деталей контента", Cause: err}
	}

	return nil
}

func (r *ContentDetailsRepository) Update(ctx context.Context, details *models.ContentDetails) error {
	return r.Save(ctx, details)
}

func (r *ContentDetailsRepository) FindByLinkID(ctx context.Context, linkID int64) (*models.ContentDetails, error) {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	selectQuery := r.sq.Select("title", "author", "updated_at", "content_text", "link_type").
		From("content_details").
		Where(sq.Eq{"link_id": linkID})

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, &customerrors.ErrBuildSQLQuery{Operation: "получение деталей контента", Cause: err}
	}

	details := &models.ContentDetails{
		LinkID: linkID,
	}

	var linkTypeStr string

	err = querier.QueryRow(ctx, query, args...).
		Scan(&details.Title, &details.Author, &details.UpdatedAt, &details.ContentText, &linkTypeStr)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &customerrors.ErrLinkNotFound{URL: fmt.Sprintf("ID: %d", linkID)}
		}

		return nil, &customerrors.ErrSQLExecution{Operation: "получение деталей контента", Cause: err}
	}

	details.LinkType = models.LinkType(linkTypeStr)

	return details, nil
}
