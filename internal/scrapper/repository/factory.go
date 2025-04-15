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
)

type ContentDetailsRepository interface {
	Save(ctx context.Context, details *models.ContentDetails) error
	FindByLinkID(ctx context.Context, linkID int64) (*models.ContentDetails, error)
	Update(ctx context.Context, details *models.ContentDetails) error
}

type Factory struct {
	db     *database.PostgresDB
	config *config.Config
	logger *slog.Logger
}

func NewFactory(db *database.PostgresDB, config *config.Config, logger *slog.Logger) *Factory {
	return &Factory{
		db:     db,
		config: config,
		logger: logger,
	}
}

func (f *Factory) CreateLinkRepository() (LinkRepository, error) {
	switch f.config.DatabaseAccessType {
	case config.SquirrelAccess:
		f.logger.Info("Создание ORM (Squirrel) репозитория ссылок")
		return orm.NewLinkRepository(f.db), nil
	case config.SQLAccess:
		f.logger.Info("Создание SQL репозитория ссылок")
		return sqlrepo.NewLinkRepository(f.db), nil
	default:
		return nil, &errors.ErrUnknownDBAccessType{AccessType: string(f.config.DatabaseAccessType)}
	}
}

func (f *Factory) CreateChatRepository() (ChatRepository, error) {
	switch f.config.DatabaseAccessType {
	case config.SquirrelAccess:
		f.logger.Info("Создание ORM (Squirrel) репозитория чатов")
		return orm.NewChatRepository(f.db), nil
	case config.SQLAccess:
		f.logger.Info("Создание SQL репозитория чатов")
		return sqlrepo.NewChatRepository(f.db), nil
	default:
		var repo ChatRepository
		return repo, &errors.ErrUnknownDBAccessType{AccessType: string(f.config.DatabaseAccessType)}
	}
}

func (f *Factory) CreateContentDetailsRepository() (ContentDetailsRepository, error) {
	switch f.config.DatabaseAccessType {
	case config.SquirrelAccess:
		f.logger.Info("Создание ORM (Squirrel) репозитория деталей контента")
		return orm.NewContentDetailsRepository(f.db), nil
	case config.SQLAccess:
		f.logger.Info("Создание SQL репозитория деталей контента")
		return sqlrepo.NewContentDetailsRepository(f.db), nil
	default:
		var repo ContentDetailsRepository
		return repo, &errors.ErrUnknownDBAccessType{AccessType: string(f.config.DatabaseAccessType)}
	}
}
