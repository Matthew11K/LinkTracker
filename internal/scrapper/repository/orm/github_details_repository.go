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
)

type GitHubDetailsRepository struct {
	db *database.PostgresDB
	sq sq.StatementBuilderType
}

func NewGitHubDetailsRepository(db *database.PostgresDB) *GitHubDetailsRepository {
	return &GitHubDetailsRepository{
		db: db,
		sq: sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}

func (r *GitHubDetailsRepository) Save(ctx context.Context, details *models.GitHubDetails) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return &customerrors.ErrBeginTransaction{Cause: err}
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	linkExistsQuery := r.sq.Select("1").From("links").Where(sq.Eq{"id": details.LinkID}).Limit(1)

	query, args, err := linkExistsQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "проверка существования ссылки", Cause: err}
	}

	var linkExists bool

	err = tx.QueryRow(ctx, "SELECT EXISTS("+query+")", args...).Scan(&linkExists)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "проверка существования ссылки", Cause: err}
	}

	if !linkExists {
		return &customerrors.ErrLinkNotFound{URL: "ID: " + string(rune(details.LinkID))}
	}

	upsertQuery := r.sq.Insert("github_details").
		Columns("link_id", "title", "author", "updated_at", "description").
		Values(details.LinkID, details.Title, details.Author, details.UpdatedAt, details.Description).
		Suffix(
			"ON CONFLICT (link_id) DO UPDATE SET " +
				"title = EXCLUDED.title, " +
				"author = EXCLUDED.author, " +
				"updated_at = EXCLUDED.updated_at, " +
				"description = EXCLUDED.description",
		)

	query, args, err = upsertQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "вставка деталей GitHub", Cause: err}
	}

	_, err = tx.Exec(ctx, query, args...)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "сохранение деталей GitHub", Cause: err}
	}

	err = tx.Commit(ctx)
	if err != nil {
		return &customerrors.ErrCommitTransaction{Cause: err}
	}

	return nil
}

func (r *GitHubDetailsRepository) FindByLinkID(ctx context.Context, linkID int64) (*models.GitHubDetails, error) {
	selectQuery := r.sq.Select("title", "author", "updated_at", "description").
		From("github_details").
		Where(sq.Eq{"link_id": linkID})

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, &customerrors.ErrBuildSQLQuery{Operation: "поиск деталей GitHub", Cause: err}
	}

	details := &models.GitHubDetails{LinkID: linkID}

	err = r.db.Pool.QueryRow(ctx, query, args...).
		Scan(&details.Title, &details.Author, &details.UpdatedAt, &details.Description)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &customerrors.ErrDetailsNotFound{LinkID: linkID}
		}

		return nil, &customerrors.ErrSQLExecution{Operation: "поиск деталей GitHub", Cause: err}
	}

	return details, nil
}

func (r *GitHubDetailsRepository) Update(ctx context.Context, details *models.GitHubDetails) error {
	details.UpdatedAt = time.Now()

	updateQuery := r.sq.Update("github_details").
		Set("title", details.Title).
		Set("author", details.Author).
		Set("updated_at", details.UpdatedAt).
		Set("description", details.Description).
		Where(sq.Eq{"link_id": details.LinkID})

	query, args, err := updateQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "обновление деталей GitHub", Cause: err}
	}

	result, err := r.db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "обновление деталей GitHub", Cause: err}
	}

	if result.RowsAffected() == 0 {
		return &customerrors.ErrDetailsNotFound{LinkID: details.LinkID}
	}

	return nil
}
