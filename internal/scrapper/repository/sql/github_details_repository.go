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

type GitHubDetailsRepository struct {
	db        *database.PostgresDB
	txManager *txs.TxManager
}

func NewGitHubDetailsRepository(db *database.PostgresDB, txManager *txs.TxManager) *GitHubDetailsRepository {
	return &GitHubDetailsRepository{db: db, txManager: txManager}
}

func (r *GitHubDetailsRepository) Save(ctx context.Context, details *models.GitHubDetails) error {
	return r.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		querier := txs.GetQuerier(ctx, r.db.Pool)

		_, err := querier.Exec(ctx, `
			INSERT INTO github_details (link_id, title, author, updated_at, description) 
			VALUES ($1, $2, $3, $4, $5) 
			ON CONFLICT (link_id) DO UPDATE SET 
				title = $2, 
				author = $3, 
				updated_at = $4, 
				description = $5
			`,
			details.LinkID, details.Title, details.Author, details.UpdatedAt, details.Description)
		if err != nil {
			return fmt.Errorf("ошибка при сохранении деталей GitHub: %w", err)
		}

		return nil
	})
}

func (r *GitHubDetailsRepository) Update(ctx context.Context, details *models.GitHubDetails) error {
	return r.txManager.WithTransaction(ctx, func(ctx context.Context) error {
		querier := txs.GetQuerier(ctx, r.db.Pool)

		result, err := querier.Exec(ctx,
			"UPDATE github_details SET title = $1, author = $2, updated_at = $3, description = $4 WHERE link_id = $5",
			details.Title, details.Author, details.UpdatedAt, details.Description, details.LinkID)
		if err != nil {
			return fmt.Errorf("ошибка при обновлении деталей GitHub: %w", err)
		}

		if result.RowsAffected() == 0 {
			return &customerrors.ErrLinkNotFound{URL: fmt.Sprintf("ID: %d", details.LinkID)}
		}

		return nil
	})
}

func (r *GitHubDetailsRepository) FindByLinkID(ctx context.Context, linkID int64) (*models.GitHubDetails, error) {
	details := &models.GitHubDetails{LinkID: linkID}

	err := r.db.Pool.QueryRow(ctx,
		"SELECT title, author, updated_at, description FROM github_details WHERE link_id = $1",
		linkID).Scan(&details.Title, &details.Author, &details.UpdatedAt, &details.Description)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &customerrors.ErrLinkNotFound{URL: fmt.Sprintf("ID: %d", linkID)}
		}

		return nil, fmt.Errorf("ошибка при поиске деталей GitHub: %w", err)
	}

	return details, nil
}
