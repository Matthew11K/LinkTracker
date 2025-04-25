package service

import (
	"log/slog"

	"github.com/central-university-dev/go-Matthew11K/internal/common"
	"github.com/central-university-dev/go-Matthew11K/internal/config"
	"github.com/central-university-dev/go-Matthew11K/internal/scrapper/cache"
	"github.com/central-university-dev/go-Matthew11K/internal/scrapper/notify"
	"github.com/central-university-dev/go-Matthew11K/internal/scrapper/repository"
	"github.com/central-university-dev/go-Matthew11K/pkg/txs"
)

//nolint:revive // Имя ServiceFactory используется для ясности
type ServiceFactory struct {
	config         *config.Config
	logger         *slog.Logger
	txManager      *txs.TxManager
	repoFactory    *repository.Factory
	linkAnalyzer   *common.LinkAnalyzer
	updaterFactory *common.LinkUpdaterFactory
}

func NewServiceFactory(
	config *config.Config,
	logger *slog.Logger,
	txManager *txs.TxManager,
	repoFactory *repository.Factory,
	linkAnalyzer *common.LinkAnalyzer,
	updaterFactory *common.LinkUpdaterFactory,
) *ServiceFactory {
	return &ServiceFactory{
		config:         config,
		logger:         logger,
		txManager:      txManager,
		repoFactory:    repoFactory,
		linkAnalyzer:   linkAnalyzer,
		updaterFactory: updaterFactory,
	}
}

func (f *ServiceFactory) CreateScrapperService() (*ScrapperService, error) {
	linkRepo, err := f.repoFactory.CreateLinkRepository()
	if err != nil {
		return nil, err
	}

	chatRepo, err := f.repoFactory.CreateChatRepository()
	if err != nil {
		return nil, err
	}

	detailsRepo, err := f.repoFactory.CreateContentDetailsRepository()
	if err != nil {
		return nil, err
	}

	notifierFactory := notify.NewNotifierFactory(f.config, f.logger)

	botClient, err := notifierFactory.CreateNotifier()
	if err != nil {
		return nil, err
	}

	var digestService *DigestService
	if f.config.DigestEnabled {
		digestService, err = f.CreateDigestService()
		if err != nil {
			f.logger.Warn("Не удалось создать сервис дайджестов, продолжаем без него", "error", err)
		}
	}

	return NewScrapperService(
		linkRepo,
		chatRepo,
		botClient,
		digestService,
		detailsRepo,
		f.updaterFactory,
		f.linkAnalyzer,
		f.logger,
		f.txManager,
	), nil
}

func (f *ServiceFactory) CreateDigestService() (*DigestService, error) {
	if !f.config.DigestEnabled {
		return nil, nil
	}

	notifierFactory := notify.NewNotifierFactory(f.config, f.logger)

	botClient, err := notifierFactory.CreateNotifier()
	if err != nil {
		return nil, err
	}

	chatRepo, err := f.repoFactory.CreateChatRepository()
	if err != nil {
		return nil, err
	}

	redisCache, err := cache.NewRedisDigestCache(
		f.config.RedisURL,
		f.config.RedisPassword,
		f.config.RedisDB,
		f.config.RedisCacheTTL,
		f.logger,
	)
	if err != nil {
		return nil, err
	}

	return NewDigestService(
		f.config,
		botClient,
		redisCache,
		chatRepo,
		f.logger,
	), nil
}
