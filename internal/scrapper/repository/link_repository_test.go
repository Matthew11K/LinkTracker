package repository_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/config"
	"github.com/central-university-dev/go-Matthew11K/internal/database"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
	"github.com/central-university-dev/go-Matthew11K/internal/scrapper/repository"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	testDB *database.PostgresDB
	logger *slog.Logger
)

func setupTestDatabase(ctx context.Context) (*database.PostgresDB, func(), error) {
	dbName := "testdb"
	dbUser := "testuser"
	dbPassword := "testpassword"

	container, err := postgres.Run(ctx,
		"postgres:16",
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPassword),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(10*time.Second),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("не удалось запустить контейнер postgres: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("не удалось получить хост контейнера: %w", err)
	}

	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		return nil, nil, fmt.Errorf("не удалось получить порт контейнера: %w", err)
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", dbUser, dbPassword, host, port.Port(), dbName)

	migrationsPath, _ := filepath.Abs("../../../migrations")
	migrateURL := fmt.Sprintf("file://%s", migrationsPath)

	m, err := migrate.New(migrateURL, dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("не удалось создать экземпляр migrate: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return nil, nil, fmt.Errorf("не удалось применить миграции: %w", err)
	}

	sourceErr, dbErr := m.Close()
	if sourceErr != nil {
		return nil, nil, fmt.Errorf("ошибка закрытия источника миграций: %w", sourceErr)
	}

	if dbErr != nil {
		return nil, nil, fmt.Errorf("ошибка закрытия подключения БД миграций: %w", dbErr)
	}

	logger.Info("Миграции успешно применены к тестовой БД")

	testCfg := &config.Config{
		DatabaseURL:     dsn,
		DatabaseMaxConn: 5,
	}

	db, err := database.NewPostgresDB(ctx, testCfg, logger)
	if err != nil {
		return nil, nil, fmt.Errorf("не удалось подключиться к тестовой БД: %w", err)
	}

	cleanup := func() {
		if err := container.Terminate(ctx); err != nil {
			logger.Error("Не удалось остановить контейнер postgres", "error", err)
		}

		logger.Info("Контейнер postgres остановлен")
	}

	return db, cleanup, nil
}

func TestMain(m *testing.M) {
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	exitCode := func() int {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		var cleanup func()

		var err error

		testDB, cleanup, err = setupTestDatabase(ctx)
		if err != nil {
			logger.Error("Ошибка при настройке тестовой БД", "error", err)
			return 1
		}

		code := m.Run()

		cleanup()

		return code
	}()

	os.Exit(exitCode)
}

func clearTables(ctx context.Context, t *testing.T) {
	tables := []string{
		"chat_state_data",
		"chat_states",
		"stackoverflow_details",
		"github_details",
		"chat_links",
		"filters",
		"link_tags",
		"links",
		"tags",
		"chats",
	}
	for _, table := range tables {
		query := fmt.Sprintf("DELETE FROM %s", table)
		_, err := testDB.Pool.Exec(ctx, query)

		require.NoErrorf(t, err, "Failed to clear table %s", table)
	}

	sequences := []string{
		"links_id_seq",
		"tags_id_seq",
		"filters_id_seq",
		"github_details_id_seq",
		"stackoverflow_details_id_seq",
		"chat_state_data_id_seq",
	}
	for _, seq := range sequences {
		query := fmt.Sprintf("ALTER SEQUENCE %s RESTART WITH 1", seq)

		_, err := testDB.Pool.Exec(ctx, query)
		if err != nil {
			t.Logf("Warning: failed to restart sequence %s: %v", seq, err)
		}
	}
}

func runTestsForConfig(t *testing.T, accessType config.AccessType) {
	ctx := context.Background()

	testCfg := &config.Config{
		DatabaseAccessType: accessType,
	}

	factory := repository.NewFactory(testDB, testCfg, logger)

	linkRepo, err := factory.CreateLinkRepository()
	require.NoError(t, err, "Ошибка создания LinkRepository для %s", accessType)

	chatRepo, err := factory.CreateChatRepository()
	require.NoError(t, err, "Ошибка создания ChatRepository для %s", accessType)

	t.Run("LinkRepository Save and FindByURL", func(t *testing.T) {
		clearTables(ctx, t)

		linkURL := fmt.Sprintf("https://test.com/link-%s-%d", accessType, time.Now().UnixNano())
		link := &models.Link{
			URL:         linkURL,
			Type:        models.GitHub,
			Tags:        []string{"tag1", string(accessType)},
			Filters:     []string{"filter1"},
			LastChecked: time.Now().Truncate(time.Microsecond),
			LastUpdated: time.Now().Truncate(time.Microsecond),
		}

		err = linkRepo.Save(ctx, link)
		require.NoError(t, err, "Save failed for %s", accessType)
		require.NotZero(t, link.ID, "Link ID should be set after save for %s", accessType)
		require.False(t, link.CreatedAt.IsZero(), "CreatedAt should be set for %s", accessType)

		foundLink, err := linkRepo.FindByURL(ctx, linkURL)
		require.NoError(t, err, "FindByURL failed for %s", accessType)
		require.NotNil(t, foundLink, "Found link should not be nil for %s", accessType)
		assert.Equal(t, link.ID, foundLink.ID, "ID mismatch for %s", accessType)
		assert.Equal(t, link.URL, foundLink.URL, "URL mismatch for %s", accessType)
		assert.Equal(t, link.Type, foundLink.Type, "Type mismatch for %s", accessType)
		assert.ElementsMatch(t, link.Tags, foundLink.Tags, "Tags mismatch for %s", accessType)
		assert.ElementsMatch(t, link.Filters, foundLink.Filters, "Filters mismatch for %s", accessType)

		assert.WithinDuration(t, link.LastChecked, foundLink.LastChecked, time.Second, "LastChecked mismatch for %s", accessType)
		assert.WithinDuration(t, link.LastUpdated, foundLink.LastUpdated, time.Second, "LastUpdated mismatch for %s", accessType)
		assert.WithinDuration(t, link.CreatedAt, foundLink.CreatedAt, time.Second, "CreatedAt mismatch for %s", accessType)

		duplicateLink := &models.Link{URL: linkURL, Type: models.GitHub}
		err = linkRepo.Save(ctx, duplicateLink)
		require.Error(t, err, "Saving duplicate should fail for %s", accessType)
		assert.IsType(t, &errors.ErrLinkAlreadyExists{}, err, "Error type should be ErrLinkAlreadyExists for %s", accessType)
	})

	t.Run("LinkRepository FindByID not found", func(t *testing.T) {
		clearTables(ctx, t)
		_, err := linkRepo.FindByID(ctx, -1)
		require.Error(t, err, "FindByID for non-existent ID should fail for %s", accessType)
		assert.IsType(t, &errors.ErrLinkNotFound{}, err, "Error type should be ErrLinkNotFound for %s", accessType)
	})

	t.Run("LinkRepository Update", func(t *testing.T) {
		clearTables(ctx, t)

		linkURL := fmt.Sprintf("https://update-test.com/link-%s-%d", accessType, time.Now().UnixNano())
		linkToUpdate := &models.Link{URL: linkURL, Type: models.StackOverflow, CreatedAt: time.Now()}
		err = linkRepo.Save(ctx, linkToUpdate)
		require.NoError(t, err)

		newCheckedTime := time.Now().Add(-1 * time.Hour).Truncate(time.Microsecond)
		newUpdatedTime := time.Now().Add(-2 * time.Hour).Truncate(time.Microsecond)
		linkToUpdate.LastChecked = newCheckedTime
		linkToUpdate.LastUpdated = newUpdatedTime

		err = linkRepo.Update(ctx, linkToUpdate)
		require.NoError(t, err, "Update failed for %s", accessType)

		updatedLink, err := linkRepo.FindByID(ctx, linkToUpdate.ID)
		require.NoError(t, err)
		assert.WithinDuration(t, newCheckedTime, updatedLink.LastChecked, time.Second, "Updated LastChecked mismatch for %s", accessType)
		assert.WithinDuration(t, newUpdatedTime, updatedLink.LastUpdated, time.Second, "Updated LastUpdated mismatch for %s", accessType)
	})

	t.Run("LinkRepository AddChatLink, FindByChatID, DeleteByURL", func(t *testing.T) {
		clearTables(ctx, t)

		chatID := time.Now().UnixNano()
		chat := &models.Chat{ID: chatID}
		err = chatRepo.Save(ctx, chat)
		require.NoError(t, err, "Save chat failed for %s", accessType)

		linkURL1 := fmt.Sprintf("https://chatlink1-%s.com/", accessType)
		linkURL2 := fmt.Sprintf("https://chatlink2-%s.com/", accessType)
		link1 := &models.Link{URL: linkURL1, Type: models.GitHub}
		link2 := &models.Link{URL: linkURL2, Type: models.StackOverflow}
		err = linkRepo.Save(ctx, link1)
		require.NoError(t, err)
		err = linkRepo.Save(ctx, link2)
		require.NoError(t, err)

		err = linkRepo.AddChatLink(ctx, chatID, link1.ID)
		require.NoError(t, err, "AddChatLink 1 failed for %s", accessType)
		err = linkRepo.AddChatLink(ctx, chatID, link2.ID)
		require.NoError(t, err, "AddChatLink 2 failed for %s", accessType)

		links, err := linkRepo.FindByChatID(ctx, chatID)
		require.NoError(t, err, "FindByChatID failed for %s", accessType)
		assert.Len(t, links, 2, "Should find 2 links for chat for %s", accessType)

		foundIDs := []int64{}
		for _, l := range links {
			foundIDs = append(foundIDs, l.ID)
		}

		assert.Contains(t, foundIDs, link1.ID, "Link1 ID not found for %s", accessType)
		assert.Contains(t, foundIDs, link2.ID, "Link2 ID not found for %s", accessType)

		err = linkRepo.DeleteByURL(ctx, linkURL1, chatID)
		require.NoError(t, err, "DeleteByURL failed for %s", accessType)

		links, err = linkRepo.FindByChatID(ctx, chatID)
		require.NoError(t, err, "FindByChatID after delete failed for %s", accessType)
		assert.Len(t, links, 1, "Should find 1 link after delete for %s", accessType)
		assert.Equal(t, link2.ID, links[0].ID, "Remaining link should be link2 for %s", accessType)

		err = linkRepo.DeleteByURL(ctx, "https://nonexistent.com", chatID)
		require.Error(t, err, "DeleteByURL for non-existent link should fail for %s", accessType)
		assert.IsType(t, &errors.ErrLinkNotFound{}, err, "Error type should be ErrLinkNotFound for %s", accessType)
	})

	t.Run("LinkRepository FindDue (Pagination)", func(t *testing.T) {
		clearTables(ctx, t)

		linksToCreate := []*models.Link{
			{URL: fmt.Sprintf("due1-%s", accessType), Type: models.GitHub, LastChecked: time.Time{}},
			{URL: fmt.Sprintf("due2-%s", accessType), Type: models.GitHub, LastChecked: time.Now().Add(-2 * time.Hour)},
			{URL: fmt.Sprintf("due3-%s", accessType), Type: models.StackOverflow, LastChecked: time.Now().Add(-1 * time.Hour)},
			{URL: fmt.Sprintf("due4-%s", accessType), Type: models.GitHub, LastChecked: time.Now()},
		}
		createdLinkIDs := make([]int64, 0, len(linksToCreate))

		for _, link := range linksToCreate {
			err := linkRepo.Save(ctx, link)
			require.NoError(t, err)

			createdLinkIDs = append(createdLinkIDs, link.ID)
		}

		dueLinks1, err := linkRepo.FindDue(ctx, 2, 0)
		require.NoError(t, err, "FindDue page 1 failed for %s", accessType)
		require.Len(t, dueLinks1, 2, "Page 1 should have 2 links for %s", accessType)
		assert.Equal(t, createdLinkIDs[0], dueLinks1[0].ID, "Page 1 first link ID mismatch for %s", accessType)  // due1 (NULL last_checked)
		assert.Equal(t, createdLinkIDs[1], dueLinks1[1].ID, "Page 1 second link ID mismatch for %s", accessType) // due2 (-2h)

		dueLinks2, err := linkRepo.FindDue(ctx, 2, 2)
		require.NoError(t, err, "FindDue page 2 failed for %s", accessType)
		require.Len(t, dueLinks2, 2, "Page 2 should have 2 links for %s", accessType)
		assert.Equal(t, createdLinkIDs[2], dueLinks2[0].ID, "Page 2 first link ID mismatch for %s", accessType)  // due3 (-1h)
		assert.Equal(t, createdLinkIDs[3], dueLinks2[1].ID, "Page 2 second link ID mismatch for %s", accessType) // due4 (now)

		dueLinks3, err := linkRepo.FindDue(ctx, 2, 4)
		require.NoError(t, err, "FindDue page 3 failed for %s", accessType)
		assert.Empty(t, dueLinks3, "Page 3 should be empty for %s", accessType)
	})

	t.Run("LinkRepository SaveTags and SaveFilters", func(t *testing.T) {
		clearTables(ctx, t)

		linkURL := fmt.Sprintf("tags-filters-%s.com", accessType)
		link := &models.Link{
			URL:     linkURL,
			Type:    models.GitHub,
			Tags:    []string{"initial_tag"},
			Filters: []string{"initial_filter"},
		}
		err := linkRepo.Save(ctx, link)
		require.NoError(t, err)
		require.NotZero(t, link.ID)

		foundLink, err := linkRepo.FindByID(ctx, link.ID)
		require.NoError(t, err)
		assert.Equal(t, []string{"initial_tag"}, foundLink.Tags, "Initial tags mismatch for %s", accessType)
		assert.Equal(t, []string{"initial_filter"}, foundLink.Filters, "Initial filters mismatch for %s", accessType)

		newTags := []string{"updated_tag1", "updated_tag2"}
		err = linkRepo.SaveTags(ctx, link.ID, newTags)
		require.NoError(t, err, "SaveTags failed for %s", accessType)

		newFilters := []string{"updated_filter1"}
		err = linkRepo.SaveFilters(ctx, link.ID, newFilters)
		require.NoError(t, err, "SaveFilters failed for %s", accessType)

		foundLinkUpdated, err := linkRepo.FindByID(ctx, link.ID)
		require.NoError(t, err)
		sort.Strings(newTags)
		sort.Strings(foundLinkUpdated.Tags)
		assert.Equal(t, newTags, foundLinkUpdated.Tags, "Updated tags mismatch for %s", accessType)
		assert.Equal(t, newFilters, foundLinkUpdated.Filters, "Updated filters mismatch for %s", accessType)

		err = linkRepo.SaveTags(ctx, link.ID, []string{})
		require.NoError(t, err)
		err = linkRepo.SaveFilters(ctx, link.ID, []string{})
		require.NoError(t, err)

		foundLinkEmpty, err := linkRepo.FindByID(ctx, link.ID)
		require.NoError(t, err)
		assert.Empty(t, foundLinkEmpty.Tags, "Tags should be empty after delete for %s", accessType)
		assert.Empty(t, foundLinkEmpty.Filters, "Filters should be empty after delete for %s", accessType)
	})

	t.Run("ChatRepository Save, FindByID, Delete", func(t *testing.T) {
		clearTables(ctx, t)

		chatID := time.Now().UnixNano() + 1
		chat := &models.Chat{ID: chatID}

		err = chatRepo.Save(ctx, chat)
		require.NoError(t, err, "Save chat failed for %s", accessType)
		require.False(t, chat.CreatedAt.IsZero(), "Chat CreatedAt should be set for %s", accessType)
		require.False(t, chat.UpdatedAt.IsZero(), "Chat UpdatedAt should be set for %s", accessType)

		foundChat, err := chatRepo.FindByID(ctx, chatID)
		require.NoError(t, err, "FindByID chat failed for %s", accessType)
		require.NotNil(t, foundChat)
		assert.Equal(t, chat.ID, foundChat.ID, "Chat ID mismatch for %s", accessType)
		assert.WithinDuration(t, chat.CreatedAt, foundChat.CreatedAt, time.Second)
		assert.WithinDuration(t, chat.UpdatedAt, foundChat.UpdatedAt, time.Second)

		_, err = chatRepo.FindByID(ctx, -999)
		require.Error(t, err, "FindByID for non-existent chat should fail for %s", accessType)
		assert.IsType(t, &errors.ErrChatNotFound{}, err, "Error type should be ErrChatNotFound for %s", accessType)

		err = chatRepo.Delete(ctx, chatID)
		require.NoError(t, err, "Delete chat failed for %s", accessType)

		_, err = chatRepo.FindByID(ctx, chatID)
		require.Error(t, err, "FindByID after delete should fail for %s", accessType)
		assert.IsType(t, &errors.ErrChatNotFound{}, err, "Error type should be ErrChatNotFound after delete for %s", accessType)
	})

	t.Run("ChatRepository FindByLinkID", func(t *testing.T) {
		clearTables(ctx, t)

		chatID1 := time.Now().UnixNano() + 2
		chatID2 := time.Now().UnixNano() + 3
		chat1 := &models.Chat{ID: chatID1}
		chat2 := &models.Chat{ID: chatID2}
		err = chatRepo.Save(ctx, chat1)
		require.NoError(t, err)
		err = chatRepo.Save(ctx, chat2)
		require.NoError(t, err)

		linkURL := fmt.Sprintf("find-by-link-%s.com", accessType)
		link := &models.Link{URL: linkURL, Type: models.GitHub}
		err = linkRepo.Save(ctx, link)
		require.NoError(t, err)

		err = linkRepo.AddChatLink(ctx, chatID1, link.ID)
		require.NoError(t, err)
		err = linkRepo.AddChatLink(ctx, chatID2, link.ID)
		require.NoError(t, err)

		foundChats, err := chatRepo.FindByLinkID(ctx, link.ID)
		require.NoError(t, err, "FindByLinkID failed for %s", accessType)
		assert.Len(t, foundChats, 2, "Should find 2 chats for link for %s", accessType)

		foundChatIDs := []int64{}
		for _, c := range foundChats {
			foundChatIDs = append(foundChatIDs, c.ID)
		}

		assert.Contains(t, foundChatIDs, chatID1, "Chat1 not found by link ID for %s", accessType)
		assert.Contains(t, foundChatIDs, chatID2, "Chat2 not found by link ID for %s", accessType)

		emptyChats, err := chatRepo.FindByLinkID(ctx, -1)
		require.Error(t, err, "FindByLinkID for non-existent link should return error for %s", accessType)
		assert.IsType(t, &errors.ErrLinkNotFound{}, err, "Error type should be ErrLinkNotFound for %s", accessType)
		assert.Nil(t, emptyChats, "Result slice should be nil when error occurs for %s", accessType)
	})
}

func TestLinkRepository_Implementations(t *testing.T) {
	t.Run("SQL Implementation", func(t *testing.T) {
		runTestsForConfig(t, config.SQLAccess)
	})
	t.Run("Squirrel Implementation", func(t *testing.T) {
		runTestsForConfig(t, config.SquirrelAccess)
	})
}
