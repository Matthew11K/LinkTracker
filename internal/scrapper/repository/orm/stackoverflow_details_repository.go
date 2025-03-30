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

type StackOverflowDetailsRepository struct {
	db *database.PostgresDB
	sq sq.StatementBuilderType
}

func NewStackOverflowDetailsRepository(db *database.PostgresDB) *StackOverflowDetailsRepository {
	return &StackOverflowDetailsRepository{
		db: db,
		sq: sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}

func (r *StackOverflowDetailsRepository) Save(ctx context.Context, details *models.StackOverflowDetails) error {
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

	upsertQuery := r.sq.Insert("stackoverflow_details").
		Columns("link_id", "title", "author", "updated_at", "content").
		Values(details.LinkID, details.Title, details.Author, details.UpdatedAt, details.Content).
		Suffix("ON CONFLICT (link_id) DO UPDATE SET title = EXCLUDED.title, author = EXCLUDED.author, updated_at = EXCLUDED.updated_at, content = EXCLUDED.content")

	query, args, err = upsertQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "вставка деталей StackOverflow", Cause: err}
	}

	_, err = tx.Exec(ctx, query, args...)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "сохранение деталей StackOverflow", Cause: err}
	}

	err = tx.Commit(ctx)
	if err != nil {
		return &customerrors.ErrCommitTransaction{Cause: err}
	}

	return nil
}

func (r *StackOverflowDetailsRepository) FindByLinkID(ctx context.Context, linkID int64) (*models.StackOverflowDetails, error) {
	selectQuery := r.sq.Select("title", "author", "updated_at", "content").
		From("stackoverflow_details").
		Where(sq.Eq{"link_id": linkID})

	query, args, err := selectQuery.ToSql()
	if err != nil {
		return nil, &customerrors.ErrBuildSQLQuery{Operation: "поиск деталей StackOverflow", Cause: err}
	}

	details := &models.StackOverflowDetails{LinkID: linkID}
	err = r.db.Pool.QueryRow(ctx, query, args...).
		Scan(&details.Title, &details.Author, &details.UpdatedAt, &details.Content)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &customerrors.ErrDetailsNotFound{LinkID: linkID}
		}
		return nil, &customerrors.ErrSQLExecution{Operation: "поиск деталей StackOverflow", Cause: err}
	}

	return details, nil
}

func (r *StackOverflowDetailsRepository) Update(ctx context.Context, details *models.StackOverflowDetails) error {
	details.UpdatedAt = time.Now()

	updateQuery := r.sq.Update("stackoverflow_details").
		Set("title", details.Title).
		Set("author", details.Author).
		Set("updated_at", details.UpdatedAt).
		Set("content", details.Content).
		Where(sq.Eq{"link_id": details.LinkID})

	query, args, err := updateQuery.ToSql()
	if err != nil {
		return &customerrors.ErrBuildSQLQuery{Operation: "обновление деталей StackOverflow", Cause: err}
	}

	result, err := r.db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return &customerrors.ErrSQLExecution{Operation: "обновление деталей StackOverflow", Cause: err}
	}

	if result.RowsAffected() == 0 {
		return &customerrors.ErrDetailsNotFound{LinkID: details.LinkID}
	}

	return nil
}
