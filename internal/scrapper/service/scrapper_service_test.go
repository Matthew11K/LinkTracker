package service_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/central-university-dev/go-Matthew11K/internal/common"
	commonmocks "github.com/central-university-dev/go-Matthew11K/internal/common/mocks"
	domainErrors "github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
	repomocks "github.com/central-university-dev/go-Matthew11K/internal/scrapper/repository/mocks"
	"github.com/central-university-dev/go-Matthew11K/internal/scrapper/service"
	servicemocks "github.com/central-university-dev/go-Matthew11K/internal/scrapper/service/mocks"
	txsmocks "github.com/central-university-dev/go-Matthew11K/pkg/txs/mocks"
)

const (
	testRepoURL = "https://github.com/owner/repo"
)

func TestScrapperService_AddLink(t *testing.T) {
	t.Parallel()

	mockLinkRepo := new(repomocks.LinkRepository)
	mockChatRepo := new(repomocks.ChatRepository)
	mockBotNotifier := new(servicemocks.BotNotifier)
	mockDetailsRepo := new(repomocks.ContentDetailsRepository)
	mockTxManager := new(txsmocks.TxManager)

	mockGithubClient := new(commonmocks.GitHubClient)
	mockStackOverflowClient := new(commonmocks.StackOverflowClient)

	linkAnalyzer := common.NewLinkAnalyzer()
	updaterFactory := common.NewLinkUpdaterFactory(mockGithubClient, mockStackOverflowClient)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	ctx := context.Background()
	chatID := int64(123)
	url := testRepoURL
	tags := []string{"tag1", "tag2"}
	filters := []string{"filter1", "filter2"}

	mockChat := &models.Chat{
		ID:        chatID,
		Links:     []int64{},
		CreatedAt: time.Now(),
	}
	mockChatRepo.On("FindByID", ctx, chatID).Return(mockChat, nil)

	notFoundErr := &domainErrors.ErrLinkNotFound{URL: url}
	mockLinkRepo.On("FindByURL", ctx, url).Return(nil, notFoundErr)

	mockLinkRepo.On("Save", ctx, mock.MatchedBy(func(link *models.Link) bool {
		return link.URL == url &&
			assert.ElementsMatch(t, tags, link.Tags) &&
			assert.ElementsMatch(t, filters, link.Filters) &&
			link.Type == models.GitHub
	})).Return(nil).Run(func(args mock.Arguments) {
		link := args.Get(1).(*models.Link)
		link.ID = 1
	})

	mockChatRepo.On("AddLink", ctx, chatID, int64(1)).Return(nil)
	mockLinkRepo.On("AddChatLink", ctx, chatID, int64(1)).Return(nil)
	mockLinkRepo.On("GetAll", ctx).Return([]*models.Link{}, nil).Maybe()

	mockTxManager.On("WithTransaction", mock.Anything, mock.AnythingOfType("func(context.Context) error")).
		Run(func(args mock.Arguments) {
			fn := args.Get(1).(func(context.Context) error)
			err := fn(ctx)
			require.NoError(t, err)
		}).Return(nil)

	scrapperService := service.NewScrapperService(
		mockLinkRepo,
		mockChatRepo,
		mockBotNotifier,
		mockDetailsRepo,
		updaterFactory,
		linkAnalyzer,
		logger,
		mockTxManager,
	)

	link, err := scrapperService.AddLink(ctx, chatID, url, tags, filters)

	require.NoError(t, err)
	assert.NotNil(t, link)
	assert.Equal(t, url, link.URL)
	assert.ElementsMatch(t, tags, link.Tags)
	assert.ElementsMatch(t, filters, link.Filters)
	assert.Equal(t, models.GitHub, link.Type)

	mockLinkRepo.AssertExpectations(t)
	mockChatRepo.AssertExpectations(t)
	mockTxManager.AssertExpectations(t)
}

func TestScrapperService_AddDuplicateLink(t *testing.T) {
	t.Parallel()

	mockLinkRepo := new(repomocks.LinkRepository)
	mockChatRepo := new(repomocks.ChatRepository)
	mockBotNotifier := new(servicemocks.BotNotifier)
	mockDetailsRepo := new(repomocks.ContentDetailsRepository)
	mockTxManager := new(txsmocks.TxManager)

	mockGithubClient := new(commonmocks.GitHubClient)
	mockStackOverflowClient := new(commonmocks.StackOverflowClient)

	linkAnalyzer := common.NewLinkAnalyzer()
	updaterFactory := common.NewLinkUpdaterFactory(mockGithubClient, mockStackOverflowClient)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	ctx := context.Background()

	chatID := int64(123)
	url := testRepoURL
	tags := []string{"tag1", "tag2"}
	filters := []string{"filter1", "filter2"}
	existingLinkID := int64(1)

	existingLink := &models.Link{
		ID:        existingLinkID,
		URL:       url,
		Type:      models.GitHub,
		Tags:      []string{"existing"},
		Filters:   []string{"existing"},
		CreatedAt: time.Now(),
	}

	mockChat := &models.Chat{
		ID:        chatID,
		Links:     []int64{existingLinkID},
		CreatedAt: time.Now(),
	}
	mockChatRepo.On("FindByID", ctx, chatID).Return(mockChat, nil)
	mockLinkRepo.On("FindByURL", ctx, url).Return(existingLink, nil)
	mockLinkRepo.On("GetAll", ctx).Return([]*models.Link{}, nil).Maybe()

	mockTxManager.On("WithTransaction", mock.Anything, mock.AnythingOfType("func(context.Context) error")).
		Run(func(args mock.Arguments) {
			fn := args.Get(1).(func(context.Context) error)
			err := fn(ctx)
			assert.IsType(t, &domainErrors.ErrLinkAlreadyExists{}, err)
		}).Return(&domainErrors.ErrLinkAlreadyExists{URL: url})

	scrapperService := service.NewScrapperService(
		mockLinkRepo,
		mockChatRepo,
		mockBotNotifier,
		mockDetailsRepo,
		updaterFactory,
		linkAnalyzer,
		logger,
		mockTxManager,
	)

	link, err := scrapperService.AddLink(ctx, chatID, url, tags, filters)

	require.Error(t, err)

	var linkExistsErr *domainErrors.ErrLinkAlreadyExists

	assert.True(t, errors.As(err, &linkExistsErr))
	assert.Nil(t, link)

	mockLinkRepo.AssertExpectations(t)
	mockChatRepo.AssertExpectations(t)
	mockTxManager.AssertExpectations(t)
}

func TestScrapperService_ProcessLink_Preview(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	ctx := context.Background()

	linkID := int64(123)
	chatIDs := []int64{10, 20, 30}
	longText := strings.Repeat("a", 250)
	shortText := strings.Repeat("b", 100)
	expectedPreviewLong := strings.Repeat("a", 200) + "..."
	now := time.Now()
	updateTime := now.Add(time.Minute)
	lastCheckTime := now.Add(-time.Hour)

	linkAnalyzer := common.NewLinkAnalyzer()

	t.Run("GitHub Preview Truncation", func(t *testing.T) {
		mockLinkRepo := new(repomocks.LinkRepository)
		mockChatRepo := new(repomocks.ChatRepository)
		mockBotNotifier := new(servicemocks.BotNotifier)
		mockDetailsRepo := new(repomocks.ContentDetailsRepository)
		mockGithubClient := new(commonmocks.GitHubClient)
		mockStackOverflowClient := new(commonmocks.StackOverflowClient)
		mockTxManager := new(txsmocks.TxManager)

		updaterFactory := common.NewLinkUpdaterFactory(mockGithubClient, mockStackOverflowClient)

		contentDetails := &models.ContentDetails{
			LinkID:      linkID,
			Title:       "Repository Title",
			Author:      "owner",
			UpdatedAt:   updateTime,
			ContentText: longText,
			LinkType:    models.GitHub,
		}

		githubLink := &models.Link{
			ID:          linkID,
			URL:         "https://github.com/owner/repo",
			Type:        models.GitHub,
			LastChecked: lastCheckTime,
			LastUpdated: time.Time{},
		}

		mockGithubClient.On("GetRepositoryLastUpdate", ctx, "owner", "repo").Return(updateTime, nil).Once()
		mockLinkRepo.On("Update", ctx, mock.Anything).Return(nil).Once()
		mockLinkRepo.On("FindByID", ctx, linkID).Return(githubLink, nil).Once()
		mockChatRepo.On("FindByLinkID", ctx, linkID).Return([]*models.Chat{{ID: chatIDs[0]}, {ID: chatIDs[1]}, {ID: chatIDs[2]}}, nil).Once()
		mockGithubClient.On("GetRepositoryDetails", ctx, "owner", "repo").Return(contentDetails, nil).Once()
		mockDetailsRepo.On("Save", ctx, mock.MatchedBy(func(details *models.ContentDetails) bool {
			return details.LinkID == linkID && details.ContentText == longText
		})).Return(nil).Once()
		mockBotNotifier.On("SendUpdate", ctx, mock.MatchedBy(func(update *models.LinkUpdate) bool {
			return update.UpdateInfo != nil && update.UpdateInfo.TextPreview == expectedPreviewLong
		})).Return(nil).Once()

		mockTxManager.On("WithTransaction", ctx, mock.AnythingOfType("func(context.Context) error")).Return(nil).
			Run(func(args mock.Arguments) {
				fn := args.Get(1).(func(context.Context) error)
				err := fn(ctx)
				require.NoError(t, err)
			})

		svc := service.NewScrapperService(
			mockLinkRepo,
			mockChatRepo,
			mockBotNotifier,
			mockDetailsRepo,
			updaterFactory,
			linkAnalyzer,
			logger,
			mockTxManager,
		)

		processed, err := svc.ProcessLink(ctx, githubLink)
		require.NoError(t, err)
		assert.True(t, processed)

		mockLinkRepo.AssertExpectations(t)
		mockBotNotifier.AssertExpectations(t)
		mockChatRepo.AssertExpectations(t)
		mockGithubClient.AssertExpectations(t)
		mockDetailsRepo.AssertExpectations(t)
		mockTxManager.AssertExpectations(t)
	})

	t.Run("StackOverflow No Truncation", func(t *testing.T) {
		mockLinkRepo := new(repomocks.LinkRepository)
		mockChatRepo := new(repomocks.ChatRepository)
		mockBotNotifier := new(servicemocks.BotNotifier)
		mockDetailsRepo := new(repomocks.ContentDetailsRepository)
		mockGithubClient := new(commonmocks.GitHubClient)
		mockStackOverflowClient := new(commonmocks.StackOverflowClient)
		mockTxManager := new(txsmocks.TxManager)

		updaterFactory := common.NewLinkUpdaterFactory(mockGithubClient, mockStackOverflowClient)

		contentDetails := &models.ContentDetails{
			LinkID:      linkID,
			Title:       "Question Title",
			Author:      "user123",
			UpdatedAt:   updateTime,
			ContentText: shortText,
			LinkType:    models.StackOverflow,
		}

		soLink := &models.Link{
			ID:          linkID,
			URL:         "https://stackoverflow.com/questions/12345",
			Type:        models.StackOverflow,
			LastChecked: lastCheckTime,
			LastUpdated: time.Time{},
		}

		mockStackOverflowClient.On("GetQuestionLastUpdate", ctx, int64(12345)).Return(updateTime, nil).Once()
		mockLinkRepo.On("Update", ctx, mock.Anything).Return(nil).Once()
		mockLinkRepo.On("FindByID", ctx, linkID).Return(soLink, nil).Once()
		mockChatRepo.On("FindByLinkID", ctx, linkID).Return([]*models.Chat{{ID: chatIDs[0]}, {ID: chatIDs[1]}, {ID: chatIDs[2]}}, nil).Once()
		mockStackOverflowClient.On("GetQuestionDetails", ctx, int64(12345)).Return(contentDetails, nil).Once()
		mockDetailsRepo.On("Save", ctx, mock.MatchedBy(func(details *models.ContentDetails) bool {
			return details.LinkID == linkID && details.ContentText == shortText
		})).Return(nil).Once()
		mockBotNotifier.On("SendUpdate", ctx, mock.MatchedBy(func(update *models.LinkUpdate) bool {
			return update.UpdateInfo != nil && update.UpdateInfo.TextPreview == shortText
		})).Return(nil).Once()

		mockTxManager.On("WithTransaction", ctx, mock.AnythingOfType("func(context.Context) error")).Return(nil).
			Run(func(args mock.Arguments) {
				fn := args.Get(1).(func(context.Context) error)
				err := fn(ctx)
				require.NoError(t, err)
			})

		svc := service.NewScrapperService(
			mockLinkRepo,
			mockChatRepo,
			mockBotNotifier,
			mockDetailsRepo,
			updaterFactory,
			linkAnalyzer,
			logger,
			mockTxManager,
		)

		processed, err := svc.ProcessLink(ctx, soLink)
		require.NoError(t, err)
		assert.True(t, processed)

		mockLinkRepo.AssertExpectations(t)
		mockBotNotifier.AssertExpectations(t)
		mockChatRepo.AssertExpectations(t)
		mockStackOverflowClient.AssertExpectations(t)
		mockDetailsRepo.AssertExpectations(t)
		mockTxManager.AssertExpectations(t)
	})
}

func TestScrapperService_ProcessLink_Scenarios(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	t.Run("Обработка ссылки на GitHub без обновлений", func(t *testing.T) {
		mockLinkRepo := new(repomocks.LinkRepository)
		mockChatRepo := new(repomocks.ChatRepository)
		mockBotNotifier := new(servicemocks.BotNotifier)
		mockDetailsRepo := new(repomocks.ContentDetailsRepository)
		mockGithubClient := new(commonmocks.GitHubClient)
		mockStackOverflowClient := new(commonmocks.StackOverflowClient)
		mockTxManager := new(txsmocks.TxManager)

		linkAnalyzer := common.NewLinkAnalyzer()
		updaterFactory := common.NewLinkUpdaterFactory(mockGithubClient, mockStackOverflowClient)

		now := time.Now()
		lastUpdate := now.Add(-time.Hour)

		githubLink := &models.Link{
			ID:          1,
			URL:         "https://github.com/owner/repo",
			Type:        models.GitHub,
			LastChecked: now.Add(-time.Hour * 24),
			LastUpdated: lastUpdate,
		}

		mockGithubClient.On("GetRepositoryLastUpdate", ctx, "owner", "repo").Return(lastUpdate, nil).Once()

		mockTxManager.On("WithTransaction", ctx, mock.AnythingOfType("func(context.Context) error")).Return(nil).
			Run(func(args mock.Arguments) {
				fn := args.Get(1).(func(context.Context) error)
				err := fn(ctx)
				require.NoError(t, err)
			})

		mockLinkRepo.On("Update", ctx, mock.AnythingOfType("*models.Link")).Return(nil).Once()

		svc := service.NewScrapperService(
			mockLinkRepo,
			mockChatRepo,
			mockBotNotifier,
			mockDetailsRepo,
			updaterFactory,
			linkAnalyzer,
			logger,
			mockTxManager,
		)

		updated, err := svc.ProcessLink(ctx, githubLink)
		require.NoError(t, err)
		assert.False(t, updated)

		mockGithubClient.AssertExpectations(t)
		mockLinkRepo.AssertExpectations(t)
		mockTxManager.AssertExpectations(t)
	})

	t.Run("Обработка ссылки на StackOverflow с обновлениями, но без подписчиков", func(t *testing.T) {
		mockLinkRepo := new(repomocks.LinkRepository)
		mockChatRepo := new(repomocks.ChatRepository)
		mockBotNotifier := new(servicemocks.BotNotifier)
		mockDetailsRepo := new(repomocks.ContentDetailsRepository)
		mockGithubClient := new(commonmocks.GitHubClient)
		mockStackOverflowClient := new(commonmocks.StackOverflowClient)
		mockTxManager := new(txsmocks.TxManager)

		linkAnalyzer := common.NewLinkAnalyzer()
		updaterFactory := common.NewLinkUpdaterFactory(mockGithubClient, mockStackOverflowClient)

		now := time.Now()
		lastUpdate := now.Add(-time.Hour)
		newUpdate := now.Add(-time.Minute)

		soLink := &models.Link{
			ID:          2,
			URL:         "https://stackoverflow.com/questions/12345",
			Type:        models.StackOverflow,
			LastChecked: now.Add(-time.Hour * 24),
			LastUpdated: lastUpdate,
		}

		mockStackOverflowClient.On("GetQuestionLastUpdate", ctx, int64(12345)).Return(newUpdate, nil).Once()

		mockTxManager.On("WithTransaction", ctx, mock.AnythingOfType("func(context.Context) error")).Return(nil).
			Run(func(args mock.Arguments) {
				fn := args.Get(1).(func(context.Context) error)
				err := fn(ctx)
				require.NoError(t, err)
			})

		mockLinkRepo.On("Update", ctx, mock.AnythingOfType("*models.Link")).Return(nil).Once()
		mockLinkRepo.On("FindByID", ctx, int64(2)).Return(soLink, nil).Once()
		mockChatRepo.On("FindByLinkID", ctx, soLink.ID).Return([]*models.Chat{}, nil).Once()

		svc := service.NewScrapperService(
			mockLinkRepo,
			mockChatRepo,
			mockBotNotifier,
			mockDetailsRepo,
			updaterFactory,
			linkAnalyzer,
			logger,
			mockTxManager,
		)

		updated, err := svc.ProcessLink(ctx, soLink)
		require.NoError(t, err)
		assert.True(t, updated)

		mockStackOverflowClient.AssertExpectations(t)
		mockLinkRepo.AssertExpectations(t)
		mockChatRepo.AssertExpectations(t)
		mockTxManager.AssertExpectations(t)
	})

	t.Run("Ошибка при получении обновлений", func(t *testing.T) {
		mockLinkRepo := new(repomocks.LinkRepository)
		mockChatRepo := new(repomocks.ChatRepository)
		mockBotNotifier := new(servicemocks.BotNotifier)
		mockDetailsRepo := new(repomocks.ContentDetailsRepository)
		mockGithubClient := new(commonmocks.GitHubClient)
		mockStackOverflowClient := new(commonmocks.StackOverflowClient)
		mockTxManager := new(txsmocks.TxManager)

		linkAnalyzer := common.NewLinkAnalyzer()
		updaterFactory := common.NewLinkUpdaterFactory(mockGithubClient, mockStackOverflowClient)

		now := time.Now()
		expectedErr := errors.New("API error")

		githubLink := &models.Link{
			ID:          3,
			URL:         "https://github.com/owner/repo",
			Type:        models.GitHub,
			LastChecked: now.Add(-time.Hour * 24),
			LastUpdated: now.Add(-time.Hour),
		}

		mockGithubClient.On("GetRepositoryLastUpdate", ctx, "owner", "repo").Return(time.Time{}, expectedErr).Once()

		svc := service.NewScrapperService(
			mockLinkRepo,
			mockChatRepo,
			mockBotNotifier,
			mockDetailsRepo,
			updaterFactory,
			linkAnalyzer,
			logger,
			mockTxManager,
		)

		updated, err := svc.ProcessLink(ctx, githubLink)
		require.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.False(t, updated)

		mockGithubClient.AssertExpectations(t)
	})
}
