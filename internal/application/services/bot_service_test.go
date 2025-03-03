package services_test

import (
	"context"
	"testing"

	"github.com/central-university-dev/go-Matthew11K/internal/application/services"
	mockservices "github.com/central-university-dev/go-Matthew11K/internal/application/services/mocks"
	mockclients "github.com/central-university-dev/go-Matthew11K/internal/domain/clients/mocks"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/repositories/mocks"
	domainservices "github.com/central-university-dev/go-Matthew11K/internal/domain/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testUsername = "testuser"
	testRepoURL  = "https://github.com/owner/repo"
)

func TestBotService_ProcessCommand_UnknownCommand(t *testing.T) {
	mockChatStateRepo := new(mocks.ChatStateRepository)
	mockScrapperClient := new(mockservices.ScrapperClient)
	mockTelegramClient := new(mockclients.TelegramClient)
	linkAnalyzer := domainservices.NewLinkAnalyzer()

	botService := services.NewBotService(mockChatStateRepo, mockScrapperClient, mockTelegramClient, linkAnalyzer)

	command := &models.Command{
		ChatID:   123456,
		UserID:   654321,
		Text:     "/unknown",
		Username: testUsername,
		Type:     models.CommandUnknown,
	}

	ctx := context.Background()

	response, err := botService.ProcessCommand(ctx, command)

	assert.Error(t, err)
	assert.IsType(t, &errors.ErrUnknownCommand{}, err)
	assert.Contains(t, response, "Неизвестная команда")
}

func TestBotService_ProcessCommand_StartCommand(t *testing.T) {
	mockChatStateRepo := new(mocks.ChatStateRepository)
	mockScrapperClient := new(mockservices.ScrapperClient)
	mockTelegramClient := new(mockclients.TelegramClient)
	linkAnalyzer := domainservices.NewLinkAnalyzer()

	botService := services.NewBotService(mockChatStateRepo, mockScrapperClient, mockTelegramClient, linkAnalyzer)

	command := &models.Command{
		ChatID:   123456,
		UserID:   654321,
		Text:     "/start",
		Username: testUsername,
		Type:     models.CommandStart,
	}

	ctx := context.Background()

	mockScrapperClient.On("RegisterChat", ctx, int64(123456)).Return(nil)
	mockChatStateRepo.On("SetState", ctx, int64(123456), models.StateIdle).Return(nil)

	response, err := botService.ProcessCommand(ctx, command)

	require.NoError(t, err)
	assert.Contains(t, response, "Привет!")
	mockScrapperClient.AssertExpectations(t)
	mockChatStateRepo.AssertExpectations(t)
}

func TestBotService_ProcessCommand_HelpCommand(t *testing.T) {
	mockChatStateRepo := new(mocks.ChatStateRepository)
	mockScrapperClient := new(mockservices.ScrapperClient)
	mockTelegramClient := new(mockclients.TelegramClient)
	linkAnalyzer := domainservices.NewLinkAnalyzer()

	botService := services.NewBotService(mockChatStateRepo, mockScrapperClient, mockTelegramClient, linkAnalyzer)

	command := &models.Command{
		ChatID:   123456,
		UserID:   654321,
		Text:     "/help",
		Username: testUsername,
		Type:     models.CommandHelp,
	}

	ctx := context.Background()

	mockChatStateRepo.On("SetState", ctx, int64(123456), models.StateIdle).Return(nil)

	response, err := botService.ProcessCommand(ctx, command)

	require.NoError(t, err)
	assert.Contains(t, response, "/start")
	assert.Contains(t, response, "/help")
	assert.Contains(t, response, "/track")
	assert.Contains(t, response, "/untrack")
	assert.Contains(t, response, "/list")
	mockChatStateRepo.AssertExpectations(t)
}

func TestBotService_ProcessCommand_TrackCommand(t *testing.T) {
	mockChatStateRepo := new(mocks.ChatStateRepository)
	mockScrapperClient := new(mockservices.ScrapperClient)
	mockTelegramClient := new(mockclients.TelegramClient)
	linkAnalyzer := domainservices.NewLinkAnalyzer()

	botService := services.NewBotService(mockChatStateRepo, mockScrapperClient, mockTelegramClient, linkAnalyzer)

	command := &models.Command{
		ChatID:   123456,
		UserID:   654321,
		Text:     "/track",
		Username: testUsername,
		Type:     models.CommandTrack,
	}

	ctx := context.Background()

	mockChatStateRepo.On("SetState", ctx, int64(123456), models.StateAwaitingLink).Return(nil)
	mockChatStateRepo.On("ClearData", ctx, int64(123456)).Return(nil)

	response, err := botService.ProcessCommand(ctx, command)

	require.NoError(t, err)
	assert.Contains(t, response, "Введите ссылку")
	mockChatStateRepo.AssertExpectations(t)
}

func TestBotService_ProcessCommand_ListCommand_EmptyList(t *testing.T) {
	mockChatStateRepo := new(mocks.ChatStateRepository)
	mockScrapperClient := new(mockservices.ScrapperClient)
	mockTelegramClient := new(mockclients.TelegramClient)
	linkAnalyzer := domainservices.NewLinkAnalyzer()

	botService := services.NewBotService(mockChatStateRepo, mockScrapperClient, mockTelegramClient, linkAnalyzer)

	command := &models.Command{
		ChatID:   123456,
		UserID:   654321,
		Text:     "/list",
		Username: testUsername,
		Type:     models.CommandList,
	}

	ctx := context.Background()

	mockScrapperClient.On("GetLinks", ctx, int64(123456)).Return([]*models.Link{}, nil)

	response, err := botService.ProcessCommand(ctx, command)

	require.NoError(t, err)
	assert.Contains(t, response, "У вас нет отслеживаемых ссылок")
	mockScrapperClient.AssertExpectations(t)
}

func TestBotService_ProcessCommand_ListCommand_WithLinks(t *testing.T) {
	mockChatStateRepo := new(mocks.ChatStateRepository)
	mockScrapperClient := new(mockservices.ScrapperClient)
	mockTelegramClient := new(mockclients.TelegramClient)
	linkAnalyzer := domainservices.NewLinkAnalyzer()

	botService := services.NewBotService(mockChatStateRepo, mockScrapperClient, mockTelegramClient, linkAnalyzer)

	command := &models.Command{
		ChatID:   123456,
		UserID:   654321,
		Text:     "/list",
		Username: testUsername,
		Type:     models.CommandList,
	}

	ctx := context.Background()

	links := []*models.Link{
		{
			ID:      1,
			URL:     "https://github.com/owner/repo1",
			Type:    models.GitHub,
			Tags:    []string{"tag1", "tag2"},
			Filters: []string{"filter1"},
		},
		{
			ID:      2,
			URL:     "https://stackoverflow.com/questions/12345",
			Type:    models.StackOverflow,
			Tags:    []string{"tag3"},
			Filters: []string{},
		},
	}

	mockScrapperClient.On("GetLinks", ctx, int64(123456)).Return(links, nil)

	response, err := botService.ProcessCommand(ctx, command)

	require.NoError(t, err)
	assert.Contains(t, response, "Список отслеживаемых ссылок")
	assert.Contains(t, response, "https://github.com/owner/repo1")
	assert.Contains(t, response, "https://stackoverflow.com/questions/12345")
	assert.Contains(t, response, "tag1, tag2")
	assert.Contains(t, response, "tag3")
	mockScrapperClient.AssertExpectations(t)
}

func TestBotService_ProcessMessage_AddingLink(t *testing.T) {
	mockChatStateRepo := new(mocks.ChatStateRepository)
	mockScrapperClient := new(mockservices.ScrapperClient)
	mockTelegramClient := new(mockclients.TelegramClient)
	linkAnalyzer := domainservices.NewLinkAnalyzer()

	botService := services.NewBotService(mockChatStateRepo, mockScrapperClient, mockTelegramClient, linkAnalyzer)

	ctx := context.Background()
	chatID := int64(123456)
	userID := int64(654321)
	username := testUsername

	mockChatStateRepo.On("GetState", ctx, chatID).Return(models.StateAwaitingLink, nil).Once()
	mockChatStateRepo.On("SetData", ctx, chatID, "link", testRepoURL).Return(nil).Once()
	mockChatStateRepo.On("SetState", ctx, chatID, models.StateAwaitingTags).Return(nil).Once()

	step1Response, err := botService.ProcessMessage(ctx, chatID, userID, testRepoURL, username)
	require.NoError(t, err)
	assert.Contains(t, step1Response, "Введите теги")
	mockChatStateRepo.AssertExpectations(t)

	mockChatStateRepo = new(mocks.ChatStateRepository)
	mockChatStateRepo.On("GetState", ctx, chatID).Return(models.StateAwaitingTags, nil).Once()
	mockChatStateRepo.On("SetData", ctx, chatID, "tags", []string{"tag1", "tag2"}).Return(nil).Once()
	mockChatStateRepo.On("SetState", ctx, chatID, models.StateAwaitingFilters).Return(nil).Once()

	botService = services.NewBotService(mockChatStateRepo, mockScrapperClient, mockTelegramClient, linkAnalyzer)
	step2Response, err := botService.ProcessMessage(ctx, chatID, userID, "tag1 tag2", username)
	require.NoError(t, err)
	assert.Contains(t, step2Response, "Введите фильтры")
	mockChatStateRepo.AssertExpectations(t)
}

func TestBotService_ProcessMessage_CompleteAddingLink(t *testing.T) {
	mockChatStateRepo := new(mocks.ChatStateRepository)
	mockScrapperClient := new(mockservices.ScrapperClient)
	mockTelegramClient := new(mockclients.TelegramClient)
	linkAnalyzer := domainservices.NewLinkAnalyzer()

	botService := services.NewBotService(mockChatStateRepo, mockScrapperClient, mockTelegramClient, linkAnalyzer)

	ctx := context.Background()
	chatID := int64(123456)
	userID := int64(654321)
	username := testUsername
	linkURL := testRepoURL
	tags := []string{"tag1", "tag2"}
	filters := []string{"filter1", "filter2"}

	mockChatStateRepo.On("GetState", ctx, chatID).Return(models.StateAwaitingLink, nil).Once()
	mockChatStateRepo.On("SetData", ctx, chatID, "link", linkURL).Return(nil).Once()
	mockChatStateRepo.On("SetState", ctx, chatID, models.StateAwaitingTags).Return(nil).Once()

	step1Response, err := botService.ProcessMessage(ctx, chatID, userID, linkURL, username)
	require.NoError(t, err)
	assert.Contains(t, step1Response, "Введите теги")
	mockChatStateRepo.AssertExpectations(t)

	mockChatStateRepo = new(mocks.ChatStateRepository)
	mockChatStateRepo.On("GetState", ctx, chatID).Return(models.StateAwaitingTags, nil).Once()
	mockChatStateRepo.On("SetData", ctx, chatID, "tags", tags).Return(nil).Once()
	mockChatStateRepo.On("SetState", ctx, chatID, models.StateAwaitingFilters).Return(nil).Once()

	botService = services.NewBotService(mockChatStateRepo, mockScrapperClient, mockTelegramClient, linkAnalyzer)
	step2Response, err := botService.ProcessMessage(ctx, chatID, userID, "tag1 tag2", username)
	require.NoError(t, err)
	assert.Contains(t, step2Response, "Введите фильтры")
	mockChatStateRepo.AssertExpectations(t)

	mockChatStateRepo = new(mocks.ChatStateRepository)
	mockScrapperClient = new(mockservices.ScrapperClient)

	mockChatStateRepo.On("GetState", ctx, chatID).Return(models.StateAwaitingFilters, nil).Once()
	mockChatStateRepo.On("GetData", ctx, chatID, "link").Return(linkURL, nil).Once()
	mockChatStateRepo.On("GetData", ctx, chatID, "tags").Return(tags, nil).Once()
	mockScrapperClient.On("AddLink", ctx, chatID, linkURL, tags, filters).Return(&models.Link{
		ID:      1,
		URL:     linkURL,
		Type:    models.GitHub,
		Tags:    tags,
		Filters: filters,
	}, nil).Once()
	mockChatStateRepo.On("SetState", ctx, chatID, models.StateIdle).Return(nil).Once()
	mockChatStateRepo.On("ClearData", ctx, chatID).Return(nil).Once()

	botService = services.NewBotService(mockChatStateRepo, mockScrapperClient, mockTelegramClient, linkAnalyzer)
	step3Response, err := botService.ProcessMessage(ctx, chatID, userID, "filter1 filter2", username)
	require.NoError(t, err)
	assert.Contains(t, step3Response, "Ссылка")
	assert.Contains(t, step3Response, "добавлена для отслеживания")
	mockChatStateRepo.AssertExpectations(t)
	mockScrapperClient.AssertExpectations(t)
}

func TestBotService_SendLinkUpdate(t *testing.T) {
	mockChatStateRepo := new(mocks.ChatStateRepository)
	mockScrapperClient := new(mockservices.ScrapperClient)
	mockTelegramClient := new(mockclients.TelegramClient)
	linkAnalyzer := domainservices.NewLinkAnalyzer()

	botService := services.NewBotService(mockChatStateRepo, mockScrapperClient, mockTelegramClient, linkAnalyzer)

	ctx := context.Background()
	update := &models.LinkUpdate{
		ID:          1,
		URL:         testRepoURL,
		Description: "Репозиторий был обновлен",
		TgChatIDs:   []int64{123456, 654321},
	}

	mockTelegramClient.On("SendMessage", ctx, int64(123456), mock.AnythingOfType("string")).Return(nil).Once()
	mockTelegramClient.On("SendMessage", ctx, int64(654321), mock.AnythingOfType("string")).Return(nil).Once()

	err := botService.SendLinkUpdate(ctx, update)

	require.NoError(t, err)
	mockTelegramClient.AssertExpectations(t)
}

func TestBotService_ProcessCommand_UntrackCommand(t *testing.T) {
	mockChatStateRepo := new(mocks.ChatStateRepository)
	mockScrapperClient := new(mockservices.ScrapperClient)
	mockTelegramClient := new(mockclients.TelegramClient)
	linkAnalyzer := domainservices.NewLinkAnalyzer()

	botService := services.NewBotService(mockChatStateRepo, mockScrapperClient, mockTelegramClient, linkAnalyzer)

	command := &models.Command{
		ChatID:   123456,
		UserID:   654321,
		Text:     "/untrack",
		Username: testUsername,
		Type:     models.CommandUntrack,
	}

	ctx := context.Background()

	mockChatStateRepo.On("SetState", ctx, int64(123456), models.StateAwaitingUntrackLink).Return(nil)
	mockChatStateRepo.On("ClearData", ctx, int64(123456)).Return(nil)

	response, err := botService.ProcessCommand(ctx, command)

	require.NoError(t, err)
	assert.Contains(t, response, "Введите ссылку для прекращения отслеживания")
	mockChatStateRepo.AssertExpectations(t)
}

func TestBotService_ProcessMessage_UntrackLink(t *testing.T) {
	mockChatStateRepo := new(mocks.ChatStateRepository)
	mockScrapperClient := new(mockservices.ScrapperClient)
	mockTelegramClient := new(mockclients.TelegramClient)
	linkAnalyzer := domainservices.NewLinkAnalyzer()

	botService := services.NewBotService(mockChatStateRepo, mockScrapperClient, mockTelegramClient, linkAnalyzer)

	ctx := context.Background()
	chatID := int64(123456)
	userID := int64(654321)
	username := testUsername
	linkURL := testRepoURL

	mockChatStateRepo.On("GetState", ctx, chatID).Return(models.StateAwaitingUntrackLink, nil)
	mockScrapperClient.On("RemoveLink", ctx, chatID, linkURL).Return(&models.Link{
		ID:      1,
		URL:     linkURL,
		Type:    models.GitHub,
		Tags:    []string{"tag1", "tag2"},
		Filters: []string{"filter1"},
	}, nil)
	mockChatStateRepo.On("SetState", ctx, chatID, models.StateIdle).Return(nil)

	response, err := botService.ProcessMessage(ctx, chatID, userID, linkURL, username)

	require.NoError(t, err)
	assert.Contains(t, response, "Прекращено отслеживание ссылки")
	assert.Contains(t, response, linkURL)
	mockChatStateRepo.AssertExpectations(t)
	mockScrapperClient.AssertExpectations(t)
}

func TestBotService_ProcessMessage_UntrackLink_NotFound(t *testing.T) {
	mockChatStateRepo := new(mocks.ChatStateRepository)
	mockScrapperClient := new(mockservices.ScrapperClient)
	mockTelegramClient := new(mockclients.TelegramClient)
	linkAnalyzer := domainservices.NewLinkAnalyzer()

	botService := services.NewBotService(mockChatStateRepo, mockScrapperClient, mockTelegramClient, linkAnalyzer)

	ctx := context.Background()
	chatID := int64(123456)
	userID := int64(654321)
	username := testUsername
	linkURL := testRepoURL

	mockChatStateRepo.On("GetState", ctx, chatID).Return(models.StateAwaitingUntrackLink, nil)
	mockScrapperClient.On("RemoveLink", ctx, chatID, linkURL).Return(nil, &errors.ErrLinkNotFound{URL: linkURL})

	response, err := botService.ProcessMessage(ctx, chatID, userID, linkURL, username)

	require.NoError(t, err)
	assert.Contains(t, response, "Ссылка не найдена или не отслеживается")
	mockChatStateRepo.AssertExpectations(t)
	mockScrapperClient.AssertExpectations(t)
}

func TestBotService_ProcessMessage_AddingDuplicateLink(t *testing.T) {
	mockChatStateRepo := new(mocks.ChatStateRepository)
	mockScrapperClient := new(mockservices.ScrapperClient)
	mockTelegramClient := new(mockclients.TelegramClient)
	linkAnalyzer := domainservices.NewLinkAnalyzer()

	botService := services.NewBotService(mockChatStateRepo, mockScrapperClient, mockTelegramClient, linkAnalyzer)

	ctx := context.Background()
	chatID := int64(123456)
	userID := int64(654321)
	username := testUsername
	linkURL := testRepoURL
	tags := []string{"tag1", "tag2"}
	filters := []string{"filter1", "filter2"}

	mockChatStateRepo.On("GetState", ctx, chatID).Return(models.StateAwaitingFilters, nil).Once()
	mockChatStateRepo.On("GetData", ctx, chatID, "link").Return(linkURL, nil).Once()
	mockChatStateRepo.On("GetData", ctx, chatID, "tags").Return(tags, nil).Once()
	mockScrapperClient.On("AddLink", ctx, chatID, linkURL, tags, filters).Return(nil, &errors.ErrLinkAlreadyExists{URL: linkURL}).Once()

	response, err := botService.ProcessMessage(ctx, chatID, userID, "filter1 filter2", username)

	require.NoError(t, err)
	assert.Contains(t, response, "Эта ссылка уже отслеживается")
	mockChatStateRepo.AssertExpectations(t)
	mockScrapperClient.AssertExpectations(t)
}

func TestBotService_ProcessMessage_LinkParsing(t *testing.T) {
	mockChatStateRepo := new(mocks.ChatStateRepository)
	mockScrapperClient := new(mockservices.ScrapperClient)
	mockTelegramClient := new(mockclients.TelegramClient)
	linkAnalyzer := domainservices.NewLinkAnalyzer()

	botService := services.NewBotService(mockChatStateRepo, mockScrapperClient, mockTelegramClient, linkAnalyzer)

	ctx := context.Background()
	chatID := int64(123456)
	userID := int64(654321)
	username := testUsername

	testCases := []struct {
		name          string
		link          string
		expectedType  models.LinkType
		shouldProceed bool
	}{
		{
			name:          "Valid GitHub link",
			link:          "https://github.com/owner/repo",
			expectedType:  models.GitHub,
			shouldProceed: true,
		},
		{
			name:          "Valid GitHub link with path",
			link:          "https://github.com/owner/repo/tree/main",
			expectedType:  models.GitHub,
			shouldProceed: true,
		},
		{
			name:          "Valid StackOverflow link",
			link:          "https://stackoverflow.com/questions/12345/how-to-test",
			expectedType:  models.StackOverflow,
			shouldProceed: true,
		},
		{
			name:          "Invalid link",
			link:          "https://example.com",
			expectedType:  models.Unknown,
			shouldProceed: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockChatStateRepo = new(mocks.ChatStateRepository)
			mockScrapperClient = new(mockservices.ScrapperClient)

			botService = services.NewBotService(mockChatStateRepo, mockScrapperClient, mockTelegramClient, linkAnalyzer)

			mockChatStateRepo.On("GetState", ctx, chatID).Return(models.StateAwaitingLink, nil)

			if tc.shouldProceed {
				mockChatStateRepo.On("SetData", ctx, chatID, "link", tc.link).Return(nil)
				mockChatStateRepo.On("SetState", ctx, chatID, models.StateAwaitingTags).Return(nil)
			}

			response, err := botService.ProcessMessage(ctx, chatID, userID, tc.link, username)

			require.NoError(t, err)

			if tc.shouldProceed {
				assert.Contains(t, response, "Введите теги")
			} else {
				assert.Contains(t, response, "Неподдерживаемый тип ссылки")
			}

			mockChatStateRepo.AssertExpectations(t)
		})
	}
}
