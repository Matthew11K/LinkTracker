package sql

import (
	"context"
	"errors"
	"fmt"

	"github.com/central-university-dev/go-Matthew11K/internal/database"
	customerrors "github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
	"github.com/jackc/pgx/v5"
)

type StackOverflowDetailsRepository struct {
	db *database.PostgresDB
}

func NewStackOverflowDetailsRepository(db *database.PostgresDB) *StackOverflowDetailsRepository {
	return &StackOverflowDetailsRepository{db: db}
}

func (r *StackOverflowDetailsRepository) Save(ctx context.Context, details *models.StackOverflowDetails) error {
	var exists bool
	err := r.db.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM links WHERE id = $1)", details.LinkID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("ошибка при проверке существования ссылки: %w", err)
	}

	if !exists {
		return &customerrors.ErrLinkNotFound{URL: fmt.Sprintf("ID: %d", details.LinkID)}
	}

	_, err = r.db.Pool.Exec(ctx,
		`INSERT INTO stackoverflow_details (link_id, title, author, updated_at, content) 
		VALUES ($1, $2, $3, $4, $5) 
		ON CONFLICT (link_id) DO UPDATE 
		SET title = $2, author = $3, updated_at = $4, content = $5`,
		details.LinkID, details.Title, details.Author, details.UpdatedAt, details.Content)
	if err != nil {
		return fmt.Errorf("ошибка при сохранении деталей StackOverflow: %w", err)
	}

	return nil
}

func (r *StackOverflowDetailsRepository) FindByLinkID(ctx context.Context, linkID int64) (*models.StackOverflowDetails, error) {
	details := &models.StackOverflowDetails{LinkID: linkID}

	err := r.db.Pool.QueryRow(ctx,
		"SELECT title, author, updated_at, content FROM stackoverflow_details WHERE link_id = $1",
		linkID).Scan(&details.Title, &details.Author, &details.UpdatedAt, &details.Content)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &customerrors.ErrDetailsNotFound{LinkID: linkID}
		}
		return nil, fmt.Errorf("ошибка при поиске деталей StackOverflow: %w", err)
	}

	return details, nil
}

func (r *StackOverflowDetailsRepository) Update(ctx context.Context, details *models.StackOverflowDetails) error {
	result, err := r.db.Pool.Exec(ctx,
		"UPDATE stackoverflow_details SET title = $1, author = $2, updated_at = $3, content = $4 WHERE link_id = $5",
		details.Title, details.Author, details.UpdatedAt, details.Content, details.LinkID)
	if err != nil {
		return fmt.Errorf("ошибка при обновлении деталей StackOverflow: %w", err)
	}

	if result.RowsAffected() == 0 {
		return &customerrors.ErrDetailsNotFound{LinkID: details.LinkID}
	}

	return nil
}
