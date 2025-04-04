package repository

import (
	"context"
	"log/slog"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"

	"github.com/central-university-dev/go-Matthew11K/internal/config"
	"github.com/central-university-dev/go-Matthew11K/internal/database"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/central-university-dev/go-Matthew11K/internal/scrapper/repository/orm"
	sqlrepo "github.com/central-university-dev/go-Matthew11K/internal/scrapper/repository/sql"
	"github.com/central-university-dev/go-Matthew11K/pkg/txs"
)

type GitHubDetailsRepository interface {
	Save(ctx context.Context, details *models.GitHubDetails) error
	FindByLinkID(ctx context.Context, linkID int64) (*models.GitHubDetails, error)
	Update(ctx context.Context, details *models.GitHubDetails) error
}

type StackOverflowDetailsRepository interface {
	Save(ctx context.Context, details *models.StackOverflowDetails) error
	FindByLinkID(ctx context.Context, linkID int64) (*models.StackOverflowDetails, error)
	Update(ctx context.Context, details *models.StackOverflowDetails) error
}

type Factory struct {
	db        *database.PostgresDB
	config    *config.Config
	logger    *slog.Logger
	txManager *txs.TxManager
}

func NewFactory(db *database.PostgresDB, config *config.Config, logger *slog.Logger, txManager *txs.TxManager) *Factory {
	return &Factory{
		db:        db,
		config:    config,
		logger:    logger,
		txManager: txManager,
	}
}

func (f *Factory) CreateLinkRepository() (LinkRepository, error) {
	switch f.config.DatabaseAccessType {
	case config.SquirrelAccess:
		f.logger.Info("Создание ORM (Squirrel) репозитория ссылок")
		return orm.NewLinkRepository(f.db, f.txManager), nil
	case config.SQLAccess:
		f.logger.Info("Создание SQL репозитория ссылок")
		return sqlrepo.NewLinkRepository(f.db, f.txManager), nil
	default:
		return nil, &errors.ErrUnknownDBAccessType{AccessType: string(f.config.DatabaseAccessType)}
	}
}

func (f *Factory) CreateChatRepository() (ChatRepository, error) {
	switch f.config.DatabaseAccessType {
	case config.SquirrelAccess:
		f.logger.Info("Создание ORM (Squirrel) репозитория чатов")
		return orm.NewChatRepository(f.db, f.txManager), nil
	case config.SQLAccess:
		f.logger.Info("Создание SQL репозитория чатов")
		return sqlrepo.NewChatRepository(f.db, f.txManager), nil
	default:
		var repo ChatRepository
		return repo, &errors.ErrUnknownDBAccessType{AccessType: string(f.config.DatabaseAccessType)}
	}
}

func (f *Factory) CreateGitHubDetailsRepository() (GitHubDetailsRepository, error) {
	switch f.config.DatabaseAccessType {
	case config.SquirrelAccess:
		f.logger.Info("Создание ORM (Squirrel) репозитория GitHub деталей")
		return orm.NewGitHubDetailsRepository(f.db, f.txManager), nil
	case config.SQLAccess:
		f.logger.Info("Создание SQL репозитория GitHub деталей")
		return sqlrepo.NewGitHubDetailsRepository(f.db, f.txManager), nil
	default:
		var repo GitHubDetailsRepository
		return repo, &errors.ErrUnknownDBAccessType{AccessType: string(f.config.DatabaseAccessType)}
	}
}

func (f *Factory) CreateStackOverflowDetailsRepository() (StackOverflowDetailsRepository, error) {
	switch f.config.DatabaseAccessType {
	case config.SquirrelAccess:
		f.logger.Info("Создание ORM (Squirrel) репозитория StackOverflow деталей")
		return orm.NewStackOverflowDetailsRepository(f.db, f.txManager), nil
	case config.SQLAccess:
		f.logger.Info("Создание SQL репозитория StackOverflow деталей")
		return sqlrepo.NewStackOverflowDetailsRepository(f.db, f.txManager), nil
	default:
		var repo StackOverflowDetailsRepository
		return repo, &errors.ErrUnknownDBAccessType{AccessType: string(f.config.DatabaseAccessType)}
	}
}
