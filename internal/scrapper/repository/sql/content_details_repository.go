package sql

import (
	"context"
	"errors"
	"fmt"

	"github.com/central-university-dev/go-Matthew11K/internal/database"
	customerrors "github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
	"github.com/central-university-dev/go-Matthew11K/pkg/txs"
	"github.com/jackc/pgx/v5"
)

type ContentDetailsRepository struct {
	db *database.PostgresDB
}

func NewContentDetailsRepository(db *database.PostgresDB) *ContentDetailsRepository {
	return &ContentDetailsRepository{db: db}
}

func (r *ContentDetailsRepository) Save(ctx context.Context, details *models.ContentDetails) error {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	_, err := querier.Exec(ctx, `
		INSERT INTO content_details (link_id, title, author, updated_at, content_text, link_type) 
		VALUES ($1, $2, $3, $4, $5, $6) 
		ON CONFLICT (link_id) DO UPDATE SET 
			title = $2, 
			author = $3, 
			updated_at = $4, 
			content_text = $5,
			link_type = $6
		`,
		details.LinkID, details.Title, details.Author, details.UpdatedAt,
		details.ContentText, string(details.LinkType))

	if err != nil {
		return fmt.Errorf("ошибка при сохранении деталей контента: %w", err)
	}

	return nil
}

func (r *ContentDetailsRepository) Update(ctx context.Context, details *models.ContentDetails) error {
	return r.Save(ctx, details)
}

func (r *ContentDetailsRepository) FindByLinkID(ctx context.Context, linkID int64) (*models.ContentDetails, error) {
	details := &models.ContentDetails{
		LinkID: linkID,
	}

	querier := txs.GetQuerier(ctx, r.db.Pool)

	var linkTypeStr string
	err := querier.QueryRow(ctx,
		`SELECT title, author, updated_at, content_text, link_type 
		 FROM content_details 
		 WHERE link_id = $1`, linkID).
		Scan(&details.Title, &details.Author, &details.UpdatedAt, &details.ContentText, &linkTypeStr)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &customerrors.ErrLinkNotFound{URL: fmt.Sprintf("ID: %d", linkID)}
		}

		return nil, fmt.Errorf("ошибка при поиске деталей контента: %w", err)
	}

	details.LinkType = models.LinkType(linkTypeStr)

	return details, nil
}
