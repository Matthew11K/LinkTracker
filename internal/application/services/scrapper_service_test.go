package services_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/central-university-dev/go-Matthew11K/internal/application/services"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/clients/mocks"
	domainErrors "github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
	repoMocks "github.com/central-university-dev/go-Matthew11K/internal/domain/repositories/mocks"
	domainServices "github.com/central-university-dev/go-Matthew11K/internal/domain/services"
)

type MockBotClient struct {
	mock.Mock
}

func (m *MockBotClient) SendUpdate(ctx context.Context, update *models.LinkUpdate) error {
	return m.Called(ctx, update).Error(0)
}

func TestScrapperService_AddLink(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	chatID := int64(123)
	url := "https://github.com/owner/repo"
	tags := []string{"tag1", "tag2"}
	filters := []string{"filter1", "filter2"}

	mockLinkRepo := repoMocks.NewLinkRepository(t)
	mockChatRepo := repoMocks.NewChatRepository(t)
	mockBotClient := new(MockBotClient)
	mockGithubClient := mocks.NewGitHubClient(t)
	mockStackOverflowClient := mocks.NewStackOverflowClient(t)
	linkAnalyzer := domainServices.NewLinkAnalyzer()

	updaterFactory := domainServices.NewLinkUpdaterFactory(mockGithubClient, mockStackOverflowClient)

	mockChat := &models.Chat{
		ID:        chatID,
		Links:     []int64{},
		CreatedAt: time.Now(),
	}
	mockChatRepo.On("FindByID", ctx, chatID).Return(mockChat, nil)

	notFoundErr := &domainErrors.ErrLinkNotFound{URL: url}
	mockLinkRepo.On("FindByURL", ctx, url).Return(nil, notFoundErr)

	mockLinkRepo.On("Save", ctx, mock.MatchedBy(func(link *models.Link) bool {
		link.ID = 1
		return link.URL == url &&
			assert.ElementsMatch(t, tags, link.Tags) &&
			assert.ElementsMatch(t, filters, link.Filters) &&
			link.Type == models.GitHub
	})).Return(nil)

	mockChatRepo.On("AddLink", ctx, chatID, int64(1)).Return(nil)

	mockLinkRepo.On("AddChatLink", ctx, chatID, int64(1)).Return(nil)

	scrapperService := services.NewScrapperService(
		mockLinkRepo,
		mockChatRepo,
		mockBotClient,
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
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	chatID := int64(123)
	url := "https://github.com/owner/repo"
	tags := []string{"tag1", "tag2"}
	filters := []string{"filter1", "filter2"}
	existingLinkID := int64(1)

	mockLinkRepo := repoMocks.NewLinkRepository(t)
	mockChatRepo := repoMocks.NewChatRepository(t)
	mockBotClient := new(MockBotClient)
	mockGithubClient := mocks.NewGitHubClient(t)
	mockStackOverflowClient := mocks.NewStackOverflowClient(t)
	linkAnalyzer := domainServices.NewLinkAnalyzer()

	updaterFactory := domainServices.NewLinkUpdaterFactory(mockGithubClient, mockStackOverflowClient)

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

	scrapperService := services.NewScrapperService(
		mockLinkRepo,
		mockChatRepo,
		mockBotClient,
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

func TestScrapperService_CheckUpdates_GitHubError(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	mockLinkRepo := repoMocks.NewLinkRepository(t)
	mockChatRepo := repoMocks.NewChatRepository(t)
	mockBotClient := new(MockBotClient)
	mockGithubClient := mocks.NewGitHubClient(t)
	mockStackOverflowClient := mocks.NewStackOverflowClient(t)
	linkAnalyzer := domainServices.NewLinkAnalyzer()

	updaterFactory := domainServices.NewLinkUpdaterFactory(mockGithubClient, mockStackOverflowClient)

	githubLink := &models.Link{
		ID:        1,
		URL:       "https://github.com/owner/repo",
		Type:      models.GitHub,
		Tags:      []string{"github"},
		Filters:   []string{},
		CreatedAt: time.Now(),
	}

	links := []*models.Link{githubLink}
	mockLinkRepo.On("GetAll", ctx).Return(links, nil)

	apiError := fmt.Errorf("GitHub API вернул статус: 500")
	mockGithubClient.On("GetRepositoryLastUpdate", ctx, "owner", "repo").Return(time.Time{}, apiError)

	scrapperService := services.NewScrapperService(
		mockLinkRepo,
		mockChatRepo,
		mockBotClient,
		updaterFactory,
		linkAnalyzer,
		logger,
	)

	err := scrapperService.CheckUpdates(ctx)

	require.NoError(t, err)

	mockLinkRepo.AssertExpectations(t)
	mockGithubClient.AssertExpectations(t)
}

func TestScrapperService_CheckUpdates_StackOverflowError(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	mockLinkRepo := repoMocks.NewLinkRepository(t)
	mockChatRepo := repoMocks.NewChatRepository(t)
	mockBotClient := new(MockBotClient)
	mockGithubClient := mocks.NewGitHubClient(t)
	mockStackOverflowClient := mocks.NewStackOverflowClient(t)
	linkAnalyzer := domainServices.NewLinkAnalyzer()

	updaterFactory := domainServices.NewLinkUpdaterFactory(mockGithubClient, mockStackOverflowClient)

	stackOverflowLink := &models.Link{
		ID:        1,
		URL:       "https://stackoverflow.com/questions/12345",
		Type:      models.StackOverflow,
		Tags:      []string{"stackoverflow"},
		Filters:   []string{},
		CreatedAt: time.Now(),
	}

	links := []*models.Link{stackOverflowLink}
	mockLinkRepo.On("GetAll", ctx).Return(links, nil)

	apiError := fmt.Errorf("StackOverflow API вернул статус: 429 Too Many Requests")
	mockStackOverflowClient.On("GetQuestionLastUpdate", ctx, int64(12345)).Return(time.Time{}, apiError)

	scrapperService := services.NewScrapperService(
		mockLinkRepo,
		mockChatRepo,
		mockBotClient,
		updaterFactory,
		linkAnalyzer,
		logger,
	)

	err := scrapperService.CheckUpdates(ctx)

	require.NoError(t, err)

	mockLinkRepo.AssertExpectations(t)
	mockStackOverflowClient.AssertExpectations(t)
}

func TestScrapperService_CheckUpdates_SendUpdateToSubscribedUsers(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	mockLinkRepo := repoMocks.NewLinkRepository(t)
	mockChatRepo := repoMocks.NewChatRepository(t)
	mockBotClient := new(MockBotClient)
	mockGithubClient := mocks.NewGitHubClient(t)
	mockStackOverflowClient := mocks.NewStackOverflowClient(t)
	linkAnalyzer := domainServices.NewLinkAnalyzer()

	updaterFactory := domainServices.NewLinkUpdaterFactory(mockGithubClient, mockStackOverflowClient)

	chatID1 := int64(123)
	chatID2 := int64(456)

	now := time.Now()
	lastUpdate := now.Add(-time.Hour)
	newUpdate := now

	githubLink := &models.Link{
		ID:          1,
		URL:         "https://github.com/owner/repo",
		Type:        models.GitHub,
		Tags:        []string{"github"},
		Filters:     []string{},
		LastUpdated: lastUpdate,
		CreatedAt:   now.Add(-2 * time.Hour),
	}

	subscribedChat := &models.Chat{
		ID:        chatID1,
		Links:     []int64{1},
		CreatedAt: time.Now(),
	}

	unsubscribedChat := &models.Chat{
		ID:        chatID2,
		Links:     []int64{},
		CreatedAt: time.Now(),
	}

	links := []*models.Link{githubLink}
	mockLinkRepo.On("GetAll", ctx).Return(links, nil)

	mockGithubClient.On("GetRepositoryLastUpdate", ctx, "owner", "repo").Return(newUpdate, nil)

	mockLinkRepo.On("Update", ctx, mock.MatchedBy(func(link *models.Link) bool {
		return link.ID == githubLink.ID && link.LastUpdated.Equal(newUpdate)
	})).Return(nil)

	chats := []*models.Chat{subscribedChat, unsubscribedChat}
	mockChatRepo.On("GetAll", ctx).Return(chats, nil)

	mockBotClient.On("SendUpdate", ctx, mock.MatchedBy(func(update *models.LinkUpdate) bool {
		return len(update.TgChatIDs) == 1 && update.TgChatIDs[0] == chatID1 && update.URL == githubLink.URL
	})).Return(nil)

	scrapperService := services.NewScrapperService(
		mockLinkRepo,
		mockChatRepo,
		mockBotClient,
		updaterFactory,
		linkAnalyzer,
		logger,
	)

	err := scrapperService.CheckUpdates(ctx)

	require.NoError(t, err)

	mockLinkRepo.AssertExpectations(t)
	mockGithubClient.AssertExpectations(t)
	mockBotClient.AssertExpectations(t)
	mockChatRepo.AssertExpectations(t)
}

func TestScrapperService_GitHub_HTTP_Errors(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	mockLinkRepo := repoMocks.NewLinkRepository(t)
	mockChatRepo := repoMocks.NewChatRepository(t)
	mockBotClient := new(MockBotClient)
	mockGithubClient := mocks.NewGitHubClient(t)
	mockStackOverflowClient := mocks.NewStackOverflowClient(t)
	linkAnalyzer := domainServices.NewLinkAnalyzer()

	updaterFactory := domainServices.NewLinkUpdaterFactory(mockGithubClient, mockStackOverflowClient)

	githubLink := &models.Link{
		ID:        1,
		URL:       "https://github.com/owner/repo",
		Type:      models.GitHub,
		Tags:      []string{"github"},
		Filters:   []string{},
		CreatedAt: time.Now(),
	}

	httpErrors := []struct {
		statusCode int
		errorMsg   string
	}{
		{400, "Bad Request"},
		{401, "Unauthorized"},
		{403, "Forbidden"},
		{404, "Not Found"},
		{429, "Too Many Requests"},
		{500, "Internal Server Error"},
		{503, "Service Unavailable"},
	}

	for _, httpErr := range httpErrors {
		t.Run(fmt.Sprintf("GitHub_HTTP_%d", httpErr.statusCode), func(t *testing.T) {
			mockLinkRepo.On("GetAll", ctx).Return([]*models.Link{githubLink}, nil).Once()

			apiError := fmt.Errorf("GitHub API вернул статус: %d %s", httpErr.statusCode, httpErr.errorMsg)
			mockGithubClient.On("GetRepositoryLastUpdate", ctx, "owner", "repo").Return(time.Time{}, apiError).Once()

			scrapperService := services.NewScrapperService(
				mockLinkRepo,
				mockChatRepo,
				mockBotClient,
				updaterFactory,
				linkAnalyzer,
				logger,
			)

			err := scrapperService.CheckUpdates(ctx)

			require.NoError(t, err)

			mockLinkRepo.AssertExpectations(t)
			mockGithubClient.AssertExpectations(t)
		})
	}
}

func TestScrapperService_StackOverflow_HTTP_Errors(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	mockLinkRepo := repoMocks.NewLinkRepository(t)
	mockChatRepo := repoMocks.NewChatRepository(t)
	mockBotClient := new(MockBotClient)
	mockGithubClient := mocks.NewGitHubClient(t)
	mockStackOverflowClient := mocks.NewStackOverflowClient(t)
	linkAnalyzer := domainServices.NewLinkAnalyzer()

	updaterFactory := domainServices.NewLinkUpdaterFactory(mockGithubClient, mockStackOverflowClient)

	stackOverflowLink := &models.Link{
		ID:        1,
		URL:       "https://stackoverflow.com/questions/12345",
		Type:      models.StackOverflow,
		Tags:      []string{"stackoverflow"},
		Filters:   []string{},
		CreatedAt: time.Now(),
	}

	httpErrors := []struct {
		statusCode int
		errorMsg   string
	}{
		{400, "Bad Request"},
		{401, "Unauthorized"},
		{403, "Forbidden"},
		{404, "Not Found"},
		{429, "Too Many Requests"},
		{500, "Internal Server Error"},
		{503, "Service Unavailable"},
	}

	for _, httpErr := range httpErrors {
		t.Run(fmt.Sprintf("StackOverflow_HTTP_%d", httpErr.statusCode), func(t *testing.T) {
			mockLinkRepo.On("GetAll", ctx).Return([]*models.Link{stackOverflowLink}, nil).Once()

			apiError := fmt.Errorf("StackOverflow API вернул статус: %d %s", httpErr.statusCode, httpErr.errorMsg)
			mockStackOverflowClient.On("GetQuestionLastUpdate", ctx, int64(12345)).Return(time.Time{}, apiError).Once()

			scrapperService := services.NewScrapperService(
				mockLinkRepo,
				mockChatRepo,
				mockBotClient,
				updaterFactory,
				linkAnalyzer,
				logger,
			)

			err := scrapperService.CheckUpdates(ctx)

			require.NoError(t, err)

			mockLinkRepo.AssertExpectations(t)
			mockStackOverflowClient.AssertExpectations(t)
		})
	}
}

func TestScrapperService_GitHub_InvalidJSON(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	mockLinkRepo := repoMocks.NewLinkRepository(t)
	mockChatRepo := repoMocks.NewChatRepository(t)
	mockBotClient := new(MockBotClient)
	mockGithubClient := mocks.NewGitHubClient(t)
	mockStackOverflowClient := mocks.NewStackOverflowClient(t)
	linkAnalyzer := domainServices.NewLinkAnalyzer()

	updaterFactory := domainServices.NewLinkUpdaterFactory(mockGithubClient, mockStackOverflowClient)

	githubLink := &models.Link{
		ID:        1,
		URL:       "https://github.com/owner/repo",
		Type:      models.GitHub,
		Tags:      []string{"github"},
		Filters:   []string{},
		CreatedAt: time.Now(),
	}

	mockLinkRepo.On("GetAll", ctx).Return([]*models.Link{githubLink}, nil)

	jsonError := fmt.Errorf("ошибка при декодировании JSON: unexpected EOF")
	mockGithubClient.On("GetRepositoryLastUpdate", ctx, "owner", "repo").Return(time.Time{}, jsonError)

	scrapperService := services.NewScrapperService(
		mockLinkRepo,
		mockChatRepo,
		mockBotClient,
		updaterFactory,
		linkAnalyzer,
		logger,
	)

	err := scrapperService.CheckUpdates(ctx)

	require.NoError(t, err)

	mockLinkRepo.AssertExpectations(t)
	mockGithubClient.AssertExpectations(t)
}

func TestScrapperService_StackOverflow_InvalidJSON(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	mockLinkRepo := repoMocks.NewLinkRepository(t)
	mockChatRepo := repoMocks.NewChatRepository(t)
	mockBotClient := new(MockBotClient)
	mockGithubClient := mocks.NewGitHubClient(t)
	mockStackOverflowClient := mocks.NewStackOverflowClient(t)
	linkAnalyzer := domainServices.NewLinkAnalyzer()

	updaterFactory := domainServices.NewLinkUpdaterFactory(mockGithubClient, mockStackOverflowClient)

	stackOverflowLink := &models.Link{
		ID:        1,
		URL:       "https://stackoverflow.com/questions/12345",
		Type:      models.StackOverflow,
		Tags:      []string{"stackoverflow"},
		Filters:   []string{},
		CreatedAt: time.Now(),
	}

	mockLinkRepo.On("GetAll", ctx).Return([]*models.Link{stackOverflowLink}, nil)

	jsonError := fmt.Errorf("ошибка при декодировании JSON: invalid character '}' after object key")
	mockStackOverflowClient.On("GetQuestionLastUpdate", ctx, int64(12345)).Return(time.Time{}, jsonError)

	scrapperService := services.NewScrapperService(
		mockLinkRepo,
		mockChatRepo,
		mockBotClient,
		updaterFactory,
		linkAnalyzer,
		logger,
	)

	err := scrapperService.CheckUpdates(ctx)

	require.NoError(t, err)

	mockLinkRepo.AssertExpectations(t)
	mockStackOverflowClient.AssertExpectations(t)
}
