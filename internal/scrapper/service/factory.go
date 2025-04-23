package service

import (
	"log/slog"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/common"
	"github.com/central-university-dev/go-Matthew11K/internal/config"
	"github.com/central-university-dev/go-Matthew11K/internal/scrapper/notify"
	"github.com/central-university-dev/go-Matthew11K/internal/scrapper/repository"
	"github.com/central-university-dev/go-Matthew11K/pkg/txs"
)

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

	return NewScrapperService(
		linkRepo,
		chatRepo,
		botClient,
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

	deliveryTime, err := time.Parse("15:04", f.config.DigestDeliveryTime)
	if err != nil {
		return nil, err
	}

	notifierFactory := notify.NewNotifierFactory(f.config, f.logger)

	botClient, err := notifierFactory.CreateNotifier()
	if err != nil {
		return nil, err
	}

	return NewDigestService(
		f.config,
		botClient,
		f.logger,
		deliveryTime.Hour(),
		deliveryTime.Minute(),
	), nil
}
