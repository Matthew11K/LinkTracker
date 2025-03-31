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
	domainErrors "github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"

	clientmocks "github.com/central-university-dev/go-Matthew11K/internal/scrapper/clients/mocks"
	repomocks "github.com/central-university-dev/go-Matthew11K/internal/scrapper/repository/mocks"
	"github.com/central-university-dev/go-Matthew11K/internal/scrapper/service"
	servicemocks "github.com/central-university-dev/go-Matthew11K/internal/scrapper/service/mocks"
)

const (
	testRepoURL = "https://github.com/owner/repo"
)

func TestScrapperService_AddLink(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	chatID := int64(123)
	url := testRepoURL
	tags := []string{"tag1", "tag2"}
	filters := []string{"filter1", "filter2"}

	mockLinkRepo := repomocks.NewLinkRepository(t)
	mockChatRepo := repomocks.NewChatRepository(t)
	mockBotNotifier := servicemocks.NewBotNotifier(t)
	mockGithubClient := clientmocks.NewGitHubClient(t)
	mockStackOverflowClient := clientmocks.NewStackOverflowClient(t)
	mockGithubRepo := repomocks.NewGitHubDetailsRepository(t)
	mockStackOverflowRepo := repomocks.NewStackOverflowDetailsRepository(t)

	linkAnalyzer := common.NewLinkAnalyzer()
	updaterFactory := common.NewLinkUpdaterFactory(mockGithubClient, mockStackOverflowClient)

	mockChat := &models.Chat{
		ID:        chatID,
		Links:     []int64{},
		CreatedAt: time.Now(),
	}
	mockChatRepo.EXPECT().FindByID(ctx, chatID).Return(mockChat, nil)

	notFoundErr := &domainErrors.ErrLinkNotFound{URL: url}
	mockLinkRepo.EXPECT().FindByURL(ctx, url).Return(nil, notFoundErr)

	mockLinkRepo.EXPECT().Save(ctx, mock.MatchedBy(func(link *models.Link) bool {
		link.ID = 1
		return link.URL == url &&
			assert.ElementsMatch(t, tags, link.Tags) &&
			assert.ElementsMatch(t, filters, link.Filters) &&
			link.Type == models.GitHub
	})).Return(nil)

	mockChatRepo.EXPECT().AddLink(ctx, chatID, int64(1)).Return(nil)
	mockLinkRepo.EXPECT().AddChatLink(ctx, chatID, int64(1)).Return(nil)

	scrapperService := service.NewScrapperService(
		mockLinkRepo,
		mockChatRepo,
		mockBotNotifier,
		mockGithubRepo,
		mockStackOverflowRepo,
		updaterFactory,
		linkAnalyzer,
		logger,
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
}

func TestScrapperService_AddDuplicateLink(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	chatID := int64(123)
	url := testRepoURL
	tags := []string{"tag1", "tag2"}
	filters := []string{"filter1", "filter2"}
	existingLinkID := int64(1)

	mockLinkRepo := repomocks.NewLinkRepository(t)
	mockChatRepo := repomocks.NewChatRepository(t)
	mockBotNotifier := servicemocks.NewBotNotifier(t)
	mockGithubClient := clientmocks.NewGitHubClient(t)
	mockStackOverflowClient := clientmocks.NewStackOverflowClient(t)
	mockGithubRepo := repomocks.NewGitHubDetailsRepository(t)
	mockStackOverflowRepo := repomocks.NewStackOverflowDetailsRepository(t)

	linkAnalyzer := common.NewLinkAnalyzer()
	updaterFactory := common.NewLinkUpdaterFactory(mockGithubClient, mockStackOverflowClient)

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
	mockChatRepo.EXPECT().FindByID(ctx, chatID).Return(mockChat, nil)
	mockLinkRepo.EXPECT().FindByURL(ctx, url).Return(existingLink, nil)

	scrapperService := service.NewScrapperService(
		mockLinkRepo,
		mockChatRepo,
		mockBotNotifier,
		mockGithubRepo,
		mockStackOverflowRepo,
		updaterFactory,
		linkAnalyzer,
		logger,
	)

	link, err := scrapperService.AddLink(ctx, chatID, url, tags, filters)

	require.Error(t, err)

	var linkExistsErr *domainErrors.ErrLinkAlreadyExists

	assert.True(t, errors.As(err, &linkExistsErr))
	assert.Nil(t, link)

	mockLinkRepo.AssertExpectations(t)
	mockChatRepo.AssertExpectations(t)
}

func TestScrapperService_ProcessLink_Preview(t *testing.T) {
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
		mockLinkRepo := repomocks.NewLinkRepository(t)
		mockChatRepo := repomocks.NewChatRepository(t)
		mockBotNotifier := servicemocks.NewBotNotifier(t)
		mockGithubRepo := repomocks.NewGitHubDetailsRepository(t)
		mockStackOverflowRepo := repomocks.NewStackOverflowDetailsRepository(t)
		mockGithubClient := clientmocks.NewGitHubClient(t)
		updaterFactory := common.NewLinkUpdaterFactory(mockGithubClient, clientmocks.NewStackOverflowClient(t))

		svc := service.NewScrapperService(mockLinkRepo, mockChatRepo, mockBotNotifier, mockGithubRepo,
			mockStackOverflowRepo, updaterFactory, linkAnalyzer, logger)

		githubLink := &models.Link{
			ID:          linkID,
			URL:         testRepoURL,
			Type:        models.GitHub,
			LastUpdated: lastCheckTime,
		}
		mockGithubDetails := &models.GitHubDetails{
			LinkID:      linkID,
			Title:       "user/repo",
			Author:      "user",
			Description: longText,
			UpdatedAt:   updateTime,
		}
		expectedUpdateInfo := &models.UpdateInfo{
			Title:       mockGithubDetails.Title,
			Author:      mockGithubDetails.Author,
			UpdatedAt:   updateTime,
			ContentType: "repository",
			TextPreview: expectedPreviewLong,
		}

		mockGithubClient.EXPECT().GetRepositoryLastUpdate(ctx, "owner", "repo").Return(updateTime, nil).Once()
		mockGithubClient.EXPECT().GetRepositoryDetails(ctx, "owner", "repo").Return(mockGithubDetails, nil).Once()
		mockChatRepo.EXPECT().FindByLinkID(ctx, linkID).Return([]*models.Chat{{ID: chatIDs[0]}, {ID: chatIDs[1]}, {ID: chatIDs[2]}}, nil).Once()
		mockGithubRepo.EXPECT().FindByLinkID(ctx, linkID).Return(nil, errors.New("details not found")).Once()
		mockGithubRepo.EXPECT().Save(ctx, mock.MatchedBy(func(details *models.GitHubDetails) bool {
			return details.LinkID == linkID && details.Description == longText
		})).Return(nil).Once()

		var capturedUpdate *models.LinkUpdate

		mockBotNotifier.EXPECT().
			SendUpdate(ctx, mock.MatchedBy(func(update *models.LinkUpdate) bool {
				capturedUpdate = update
				return update.ID == linkID &&
					update.URL == githubLink.URL &&
					len(update.TgChatIDs) == 3 &&
					update.UpdateInfo != nil &&
					update.UpdateInfo.TextPreview == expectedPreviewLong
			})).
			Return(nil).
			Once()

		mockLinkRepo.EXPECT().Update(ctx, mock.MatchedBy(func(link *models.Link) bool {
			return link.ID == linkID && link.LastUpdated.Equal(updateTime)
		})).Return(nil).Once()

		t.Log("--- Запуск теста для GitHub --- ")

		processed, err := svc.ProcessLink(ctx, githubLink)
		require.NoError(t, err)
		assert.True(t, processed, "Link should have been processed")

		mockLinkRepo.AssertExpectations(t)
		mockChatRepo.AssertExpectations(t)
		mockBotNotifier.AssertExpectations(t)
		mockGithubRepo.AssertExpectations(t)
		mockGithubClient.AssertExpectations(t)
		require.NotNil(t, capturedUpdate, "Update should have been captured")
		require.NotNil(t, capturedUpdate.UpdateInfo)
		assert.Equal(t, expectedPreviewLong, capturedUpdate.UpdateInfo.TextPreview, "GitHub preview should be truncated to 200 chars")
		assert.Len(t, capturedUpdate.UpdateInfo.TextPreview, 203, "GitHub preview length should be 200 + 3 ellipsis")
		assert.Equal(t, expectedUpdateInfo.Title, capturedUpdate.UpdateInfo.Title)
		assert.Equal(t, expectedUpdateInfo.Author, capturedUpdate.UpdateInfo.Author)
		assert.ElementsMatch(t, chatIDs, capturedUpdate.TgChatIDs)
	})

	t.Run("StackOverflow Preview No Truncation", func(t *testing.T) {
		mockLinkRepo := repomocks.NewLinkRepository(t)
		mockChatRepo := repomocks.NewChatRepository(t)
		mockBotNotifier := servicemocks.NewBotNotifier(t)
		mockGithubRepo := repomocks.NewGitHubDetailsRepository(t)
		mockStackOverflowRepo := repomocks.NewStackOverflowDetailsRepository(t)
		mockStackOverflowClient := clientmocks.NewStackOverflowClient(t)
		updaterFactory := common.NewLinkUpdaterFactory(clientmocks.NewGitHubClient(t), mockStackOverflowClient)

		svc := service.NewScrapperService(mockLinkRepo, mockChatRepo, mockBotNotifier, mockGithubRepo,
			mockStackOverflowRepo, updaterFactory, linkAnalyzer, logger)

		soLink := &models.Link{
			ID:          linkID,
			URL:         "https://stackoverflow.com/questions/12345/title",
			Type:        models.StackOverflow,
			LastUpdated: lastCheckTime,
		}
		mockSODetails := &models.StackOverflowDetails{
			LinkID:    linkID,
			Title:     "StackOverflow Question Title",
			Author:    "so_user",
			Content:   shortText,
			UpdatedAt: updateTime,
		}
		expectedUpdateInfo := &models.UpdateInfo{
			Title:       mockSODetails.Title,
			Author:      mockSODetails.Author,
			UpdatedAt:   updateTime,
			ContentType: "question",
			TextPreview: shortText,
		}

		mockStackOverflowClient.EXPECT().GetQuestionLastUpdate(ctx, int64(12345)).Return(updateTime, nil).Once()
		mockStackOverflowClient.EXPECT().GetQuestionDetails(ctx, int64(12345)).Return(mockSODetails, nil).Once()
		mockChatRepo.EXPECT().FindByLinkID(ctx, linkID).Return([]*models.Chat{{ID: chatIDs[0]}}, nil).Once()
		mockStackOverflowRepo.EXPECT().FindByLinkID(ctx, linkID).Return(nil, errors.New("details not found")).Once()
		mockStackOverflowRepo.EXPECT().Save(ctx, mock.MatchedBy(func(details *models.StackOverflowDetails) bool {
			return details.LinkID == linkID && details.Content == shortText
		})).Return(nil).Once()

		var capturedUpdate *models.LinkUpdate

		mockBotNotifier.EXPECT().
			SendUpdate(ctx, mock.MatchedBy(func(update *models.LinkUpdate) bool {
				capturedUpdate = update
				return update.ID == linkID &&
					len(update.TgChatIDs) == 1 &&
					update.UpdateInfo != nil &&
					update.UpdateInfo.TextPreview == shortText
			})).
			Return(nil).
			Once()

		mockLinkRepo.EXPECT().Update(ctx, mock.MatchedBy(func(link *models.Link) bool {
			return link.ID == linkID && link.LastUpdated.Equal(updateTime)
		})).Return(nil).Once()

		t.Log("--- Запуск теста для StackOverflow --- ")

		processed, err := svc.ProcessLink(ctx, soLink)
		require.NoError(t, err)
		assert.True(t, processed, "Link should have been processed")

		mockLinkRepo.AssertExpectations(t)
		mockChatRepo.AssertExpectations(t)
		mockBotNotifier.AssertExpectations(t)
		mockStackOverflowRepo.AssertExpectations(t)
		mockStackOverflowClient.AssertExpectations(t)
		require.NotNil(t, capturedUpdate, "Update should have been captured")
		require.NotNil(t, capturedUpdate.UpdateInfo)
		assert.Equal(t, shortText, capturedUpdate.UpdateInfo.TextPreview, "StackOverflow preview should NOT be truncated")
		assert.Len(t, capturedUpdate.UpdateInfo.TextPreview, len(shortText), "StackOverflow preview length should match original short text")
		assert.Equal(t, expectedUpdateInfo.Title, capturedUpdate.UpdateInfo.Title)
		assert.Equal(t, expectedUpdateInfo.Author, capturedUpdate.UpdateInfo.Author)
		assert.ElementsMatch(t, []int64{chatIDs[0]}, capturedUpdate.TgChatIDs)
	})
}

func TestScrapperService_ProcessLink_Scenarios(t *testing.T) {
	linkID := int64(1)
	ctx := context.Background()
	updatedAt := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	lastChecked := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	mockGithubDetails := &models.GitHubDetails{
		LinkID:      linkID,
		Description: "Repo description",
		Title:       "Repo title",
		Author:      "Repo author",
	}

	mockChatInfos := []*models.Chat{
		{ID: 123},
		{ID: 456},
		{ID: 789},
	}

	linkAnalyzer := &common.LinkAnalyzer{}

	t.Run("ChatRepo_FindByLinkID_Error", func(t *testing.T) {
		mockLinkRepo := repomocks.NewLinkRepository(t)
		mockLinkRepo.ExpectedCalls = nil

		mockChatRepo := repomocks.NewChatRepository(t)
		mockBotNotifier := servicemocks.NewBotNotifier(t)
		mockGithubRepo := repomocks.NewGitHubDetailsRepository(t)
		mockStackOverflowRepo := repomocks.NewStackOverflowDetailsRepository(t)
		mockGithubClient := clientmocks.NewGitHubClient(t)

		mockStackOverflowClient := clientmocks.NewStackOverflowClient(t)
		updaterFactory := common.NewLinkUpdaterFactory(mockGithubClient, mockStackOverflowClient)

		svc := service.NewScrapperService(
			mockLinkRepo, mockChatRepo, mockBotNotifier,
			mockGithubRepo, mockStackOverflowRepo,
			updaterFactory, linkAnalyzer, logger,
		)

		githubLink := &models.Link{
			ID:          linkID,
			URL:         testRepoURL,
			Type:        models.GitHub,
			LastChecked: lastChecked,
		}

		mockGithubClient.EXPECT().GetRepositoryLastUpdate(ctx, "owner", "repo").Return(updatedAt, nil).Once()

		expectedErr := errors.New("db error on find chats")
		mockChatRepo.EXPECT().FindByLinkID(ctx, linkID).Return(nil, expectedErr).Once()

		mockLinkRepo.On("Update", ctx, mock.AnythingOfType("*models.Link")).Return(nil)

		processed, err := svc.ProcessLink(ctx, githubLink)

		assert.True(t, processed, "Link should be processed even when ChatRepo.FindByLinkID returns error")
		assert.Error(t, err, "Error should be propagated")
		assert.ErrorContains(t, err, expectedErr.Error())
	})

	t.Run("GetUpdateDetails_Error", func(t *testing.T) {
		mockLinkRepo := repomocks.NewLinkRepository(t)
		mockLinkRepo.ExpectedCalls = nil

		mockChatRepo := repomocks.NewChatRepository(t)
		mockBotNotifier := servicemocks.NewBotNotifier(t)
		mockGithubRepo := repomocks.NewGitHubDetailsRepository(t)
		mockStackOverflowRepo := repomocks.NewStackOverflowDetailsRepository(t)
		mockGithubClient := clientmocks.NewGitHubClient(t)

		mockStackOverflowClient := clientmocks.NewStackOverflowClient(t)
		updaterFactory := common.NewLinkUpdaterFactory(mockGithubClient, mockStackOverflowClient)

		svc := service.NewScrapperService(
			mockLinkRepo, mockChatRepo, mockBotNotifier,
			mockGithubRepo, mockStackOverflowRepo,
			updaterFactory, linkAnalyzer, logger,
		)

		githubLink := &models.Link{
			ID:          linkID,
			URL:         testRepoURL,
			Type:        models.GitHub,
			LastChecked: lastChecked,
		}

		mockGithubClient.EXPECT().GetRepositoryLastUpdate(ctx, "owner", "repo").Return(updatedAt, nil).Once()

		expectedErr := errors.New("github API error on get details")
		mockGithubClient.EXPECT().GetRepositoryDetails(ctx, "owner", "repo").Return(nil, expectedErr).Once()

		mockChatRepo.EXPECT().FindByLinkID(ctx, linkID).Return(mockChatInfos, nil).Once()

		mockBotNotifier.EXPECT().SendUpdate(ctx, mock.AnythingOfType("*models.LinkUpdate")).Return(nil).Once()

		mockLinkRepo.On("Update", ctx, mock.AnythingOfType("*models.Link")).Return(nil)

		processed, err := svc.ProcessLink(ctx, githubLink)

		assert.True(t, processed, "Link should be processed despite GetRepositoryDetails error")
		assert.NoError(t, err, "No error should be propagated")
	})

	t.Run("DetailsRepo_Save_Error", func(t *testing.T) {
		mockLinkRepo := repomocks.NewLinkRepository(t)
		mockLinkRepo.ExpectedCalls = nil

		mockChatRepo := repomocks.NewChatRepository(t)
		mockBotNotifier := servicemocks.NewBotNotifier(t)
		mockGithubRepo := repomocks.NewGitHubDetailsRepository(t)
		mockStackOverflowRepo := repomocks.NewStackOverflowDetailsRepository(t)
		mockGithubClient := clientmocks.NewGitHubClient(t)

		mockStackOverflowClient := clientmocks.NewStackOverflowClient(t)
		updaterFactory := common.NewLinkUpdaterFactory(mockGithubClient, mockStackOverflowClient)

		svc := service.NewScrapperService(
			mockLinkRepo, mockChatRepo, mockBotNotifier,
			mockGithubRepo, mockStackOverflowRepo,
			updaterFactory, linkAnalyzer, logger,
		)

		githubLink := &models.Link{
			ID:          linkID,
			URL:         testRepoURL,
			Type:        models.GitHub,
			LastChecked: lastChecked,
		}

		mockGithubClient.EXPECT().GetRepositoryLastUpdate(ctx, "owner", "repo").Return(updatedAt, nil).Once()

		mockGithubClient.EXPECT().GetRepositoryDetails(ctx, "owner", "repo").Return(mockGithubDetails, nil).Once()

		mockChatRepo.EXPECT().FindByLinkID(ctx, linkID).Return(mockChatInfos, nil).Once()

		mockGithubRepo.EXPECT().FindByLinkID(ctx, linkID).Return(nil, errors.New("not found")).Once()

		expectedErr := errors.New("db error on save details")
		mockGithubRepo.EXPECT().Save(ctx, mock.MatchedBy(func(details *models.GitHubDetails) bool {
			return details.LinkID == linkID && details.Description == mockGithubDetails.Description
		})).Return(expectedErr).Once()

		mockBotNotifier.EXPECT().SendUpdate(ctx, mock.AnythingOfType("*models.LinkUpdate")).Return(nil).Once()

		mockLinkRepo.On("Update", ctx, mock.AnythingOfType("*models.Link")).Return(nil)

		processed, err := svc.ProcessLink(ctx, githubLink)

		assert.True(t, processed, "Link should be processed despite GitHubDetailsRepo.Save error")
		assert.NoError(t, err, "No error should be propagated")
	})

	t.Run("No_Update_Found", func(t *testing.T) {
		mockLinkRepo := repomocks.NewLinkRepository(t)
		mockLinkRepo.ExpectedCalls = nil

		mockChatRepo := repomocks.NewChatRepository(t)
		mockBotNotifier := servicemocks.NewBotNotifier(t)
		mockGithubRepo := repomocks.NewGitHubDetailsRepository(t)
		mockStackOverflowRepo := repomocks.NewStackOverflowDetailsRepository(t)
		mockGithubClient := clientmocks.NewGitHubClient(t)

		mockStackOverflowClient := clientmocks.NewStackOverflowClient(t)
		updaterFactory := common.NewLinkUpdaterFactory(mockGithubClient, mockStackOverflowClient)

		svc := service.NewScrapperService(
			mockLinkRepo, mockChatRepo, mockBotNotifier,
			mockGithubRepo, mockStackOverflowRepo,
			updaterFactory, linkAnalyzer, logger,
		)

		githubLink := &models.Link{
			ID:          linkID,
			URL:         testRepoURL,
			Type:        models.GitHub,
			LastChecked: updatedAt,
			LastUpdated: updatedAt,
		}

		mockGithubClient.EXPECT().GetRepositoryLastUpdate(ctx, "owner", "repo").Return(updatedAt, nil).Once()

		mockLinkRepo.On("Update", ctx, mock.AnythingOfType("*models.Link")).Return(nil)

		processed, err := svc.ProcessLink(ctx, githubLink)

		assert.False(t, processed, "Link should not be processed when no update is found")
		assert.NoError(t, err, "No error should be returned")
	})
}

func TestScrapperService_RemoveLink(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	chatID := int64(123)
	url := testRepoURL
	linkID := int64(1)

	t.Run("Success", func(t *testing.T) {
		mockLinkRepo := repomocks.NewLinkRepository(t)
		mockChatRepo := repomocks.NewChatRepository(t)
		mockBotNotifier := servicemocks.NewBotNotifier(t)
		mockGithubRepo := repomocks.NewGitHubDetailsRepository(t)
		mockStackOverflowRepo := repomocks.NewStackOverflowDetailsRepository(t)
		mockGithubClient := clientmocks.NewGitHubClient(t)
		mockStackOverflowClient := clientmocks.NewStackOverflowClient(t)
		linkAnalyzer := common.NewLinkAnalyzer()
		updaterFactory := common.NewLinkUpdaterFactory(mockGithubClient, mockStackOverflowClient)
		svc := service.NewScrapperService(mockLinkRepo, mockChatRepo, mockBotNotifier, mockGithubRepo,
			mockStackOverflowRepo, updaterFactory, linkAnalyzer, logger)

		existingLink := &models.Link{ID: linkID, URL: url, Type: models.GitHub}
		mockChat := &models.Chat{ID: chatID, Links: []int64{linkID}}

		mockChatRepo.EXPECT().FindByID(ctx, chatID).Return(mockChat, nil).Once()
		mockLinkRepo.EXPECT().FindByURL(ctx, url).Return(existingLink, nil).Once()
		mockLinkRepo.EXPECT().DeleteByURL(ctx, url, chatID).Return(nil).Once()

		link, err := svc.RemoveLink(ctx, chatID, url)

		require.NoError(t, err)
		require.NotNil(t, link)
		assert.Equal(t, linkID, link.ID)
		mockChatRepo.AssertExpectations(t)
		mockLinkRepo.AssertExpectations(t)
	})

	t.Run("Chat Not Found", func(t *testing.T) {
		mockLinkRepo := repomocks.NewLinkRepository(t)
		mockChatRepo := repomocks.NewChatRepository(t)
		svc := service.NewScrapperService(mockLinkRepo, mockChatRepo, nil, nil, nil, nil, nil, logger)
		expectedErr := &domainErrors.ErrChatNotFound{}

		mockChatRepo.EXPECT().FindByID(ctx, chatID).Return(nil, expectedErr).Once()

		link, err := svc.RemoveLink(ctx, chatID, url)

		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Nil(t, link)
		mockChatRepo.AssertExpectations(t)
		mockLinkRepo.AssertNotCalled(t, "FindByURL")
	})

	t.Run("Link Not Found", func(t *testing.T) {
		mockLinkRepo := repomocks.NewLinkRepository(t)
		mockChatRepo := repomocks.NewChatRepository(t)
		svc := service.NewScrapperService(mockLinkRepo, mockChatRepo, nil, nil, nil, nil, nil, logger)
		mockChat := &models.Chat{ID: chatID}
		expectedErr := &domainErrors.ErrLinkNotFound{URL: url}

		mockChatRepo.EXPECT().FindByID(ctx, chatID).Return(mockChat, nil).Once()
		mockLinkRepo.EXPECT().FindByURL(ctx, url).Return(nil, expectedErr).Once()

		link, err := svc.RemoveLink(ctx, chatID, url)

		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Nil(t, link)
		mockChatRepo.AssertExpectations(t)
		mockLinkRepo.AssertExpectations(t)
		mockLinkRepo.AssertNotCalled(t, "DeleteByURL")
	})

	t.Run("Chat Not Tracking Link", func(t *testing.T) {
		mockLinkRepo := repomocks.NewLinkRepository(t)
		mockChatRepo := repomocks.NewChatRepository(t)
		svc := service.NewScrapperService(mockLinkRepo, mockChatRepo, nil, nil, nil, nil, nil, logger)
		existingLink := &models.Link{ID: linkID, URL: url, Type: models.GitHub}
		mockChat := &models.Chat{ID: chatID, Links: []int64{999}}
		expectedErr := errors.New("chat not tracking link")

		mockChatRepo.EXPECT().FindByID(ctx, chatID).Return(mockChat, nil).Once()
		mockLinkRepo.EXPECT().FindByURL(ctx, url).Return(existingLink, nil).Once()
		mockLinkRepo.EXPECT().DeleteByURL(ctx, url, chatID).Return(expectedErr).Once()

		link, err := svc.RemoveLink(ctx, chatID, url)

		require.Error(t, err)
		assert.ErrorContains(t, err, "chat not tracking link")
		assert.Nil(t, link)
		mockChatRepo.AssertExpectations(t)
		mockLinkRepo.AssertExpectations(t)
	})

	t.Run("Link Tracked By Others (RemoveLink only)", func(t *testing.T) {
		mockLinkRepo := repomocks.NewLinkRepository(t)
		mockChatRepo := repomocks.NewChatRepository(t)
		svc := service.NewScrapperService(mockLinkRepo, mockChatRepo, nil, nil, nil, nil, nil, logger)
		existingLink := &models.Link{ID: linkID, URL: url, Type: models.GitHub}
		mockChat := &models.Chat{ID: chatID, Links: []int64{linkID}}

		mockChatRepo.EXPECT().FindByID(ctx, chatID).Return(mockChat, nil).Once()
		mockLinkRepo.EXPECT().FindByURL(ctx, url).Return(existingLink, nil).Once()
		mockLinkRepo.EXPECT().DeleteByURL(ctx, url, chatID).Return(nil).Once()

		link, err := svc.RemoveLink(ctx, chatID, url)

		require.NoError(t, err)
		require.NotNil(t, link)
		assert.Equal(t, linkID, link.ID)
		mockChatRepo.AssertExpectations(t)
		mockLinkRepo.AssertExpectations(t)
		mockChatRepo.AssertNotCalled(t, "FindByLinkID", mock.Anything, linkID)
	})
}
