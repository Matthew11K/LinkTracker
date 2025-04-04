package repository

import (
	"log/slog"

	"github.com/central-university-dev/go-Matthew11K/internal/bot/repository/orm"
	sqlrepo "github.com/central-university-dev/go-Matthew11K/internal/bot/repository/sql"
	"github.com/central-university-dev/go-Matthew11K/internal/bot/service"
	"github.com/central-university-dev/go-Matthew11K/internal/config"
	"github.com/central-university-dev/go-Matthew11K/internal/database"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/central-university-dev/go-Matthew11K/pkg/txs"
)

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

func (f *Factory) CreateChatStateRepository() (service.ChatStateRepository, error) {
	switch f.config.DatabaseAccessType {
	case config.SquirrelAccess:
		f.logger.Info("Создание ORM (Squirrel) репозитория состояний чатов")
		return orm.NewChatStateRepository(f.db, f.txManager), nil
	case config.SQLAccess:
		f.logger.Info("Создание SQL репозитория состояний чатов")
		return sqlrepo.NewChatStateRepository(f.db, f.txManager), nil
	default:
		return nil, &errors.ErrUnknownDBAccessType{AccessType: string(f.config.DatabaseAccessType)}
	}
}
