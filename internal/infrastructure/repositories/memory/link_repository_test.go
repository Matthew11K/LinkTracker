package memory_test

import (
	"context"
	"testing"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
	"github.com/central-university-dev/go-Matthew11K/internal/infrastructure/repositories/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLinkRepository_Save(t *testing.T) {
	repo := memory.NewLinkRepository()
	ctx := context.Background()

	t.Run("Save new link", func(t *testing.T) {
		link := &models.Link{
			URL:     "https://github.com/owner/repo",
			Type:    models.GitHub,
			Tags:    []string{"tag1", "tag2"},
			Filters: []string{"filter1", "filter2"},
		}

		err := repo.Save(ctx, link)

		require.NoError(t, err)
		assert.NotZero(t, link.ID)
		assert.False(t, link.CreatedAt.IsZero())
	})

	t.Run("Save duplicate link", func(t *testing.T) {
		link := &models.Link{
			URL:     "https://github.com/owner/repo",
			Type:    models.GitHub,
			Tags:    []string{"tag1", "tag2"},
			Filters: []string{"filter1", "filter2"},
		}

		err := repo.Save(ctx, link)

		require.Error(t, err)
		assert.IsType(t, &errors.ErrLinkAlreadyExists{}, err)
	})
}

func TestLinkRepository_FindByURL(t *testing.T) {
	repo := memory.NewLinkRepository()
	ctx := context.Background()

	originalLink := &models.Link{
		URL:     "https://github.com/owner/repo",
		Type:    models.GitHub,
		Tags:    []string{"tag1", "tag2"},
		Filters: []string{"filter1", "filter2"},
	}
	require.NoError(t, repo.Save(ctx, originalLink))

	t.Run("Find existing link", func(t *testing.T) {
		link, err := repo.FindByURL(ctx, "https://github.com/owner/repo")

		require.NoError(t, err)
		assert.Equal(t, originalLink.ID, link.ID)
		assert.Equal(t, originalLink.URL, link.URL)
		assert.Equal(t, originalLink.Type, link.Type)
		assert.Equal(t, originalLink.Tags, link.Tags)
		assert.Equal(t, originalLink.Filters, link.Filters)
	})

	t.Run("Find non-existing link", func(t *testing.T) {
		link, err := repo.FindByURL(ctx, "https://github.com/nonexistent/repo")

		require.Error(t, err)
		assert.Nil(t, link)
		assert.IsType(t, &errors.ErrLinkNotFound{}, err)
	})
}

func TestLinkRepository_FindByChatID(t *testing.T) {
	repo := memory.NewLinkRepository()
	ctx := context.Background()
	chatID := int64(12345)

	link1 := &models.Link{
		URL:     "https://github.com/owner/repo1",
		Type:    models.GitHub,
		Tags:    []string{"tag1", "tag2"},
		Filters: []string{"filter1", "filter2"},
	}
	link2 := &models.Link{
		URL:     "https://github.com/owner/repo2",
		Type:    models.GitHub,
		Tags:    []string{"tag3", "tag4"},
		Filters: []string{"filter3", "filter4"},
	}

	require.NoError(t, repo.Save(ctx, link1))
	require.NoError(t, repo.Save(ctx, link2))

	require.NoError(t, repo.AddChatLink(ctx, chatID, link1.ID))
	require.NoError(t, repo.AddChatLink(ctx, chatID, link2.ID))

	t.Run("Find links by chat ID", func(t *testing.T) {
		links, err := repo.FindByChatID(ctx, chatID)

		require.NoError(t, err)
		assert.Len(t, links, 2)

		foundLink1 := false
		foundLink2 := false

		for _, link := range links {
			if link.ID == link1.ID {
				foundLink1 = true
			}

			if link.ID == link2.ID {
				foundLink2 = true
			}
		}

		assert.True(t, foundLink1, "Link 1 should be found")
		assert.True(t, foundLink2, "Link 2 should be found")
	})

	t.Run("Find links for non-existing chat", func(t *testing.T) {
		links, err := repo.FindByChatID(ctx, 99999)

		require.NoError(t, err)
		assert.Empty(t, links)
	})
}

func TestLinkRepository_DeleteByURL(t *testing.T) {
	repo := memory.NewLinkRepository()
	ctx := context.Background()
	chatID := int64(12345)

	link := &models.Link{
		URL:     "https://github.com/owner/repo",
		Type:    models.GitHub,
		Tags:    []string{"tag1", "tag2"},
		Filters: []string{"filter1", "filter2"},
	}
	require.NoError(t, repo.Save(ctx, link))
	require.NoError(t, repo.AddChatLink(ctx, chatID, link.ID))

	t.Run("Delete existing link", func(t *testing.T) {
		err := repo.DeleteByURL(ctx, "https://github.com/owner/repo", chatID)

		require.NoError(t, err)

		links, err := repo.FindByChatID(ctx, chatID)
		require.NoError(t, err)
		assert.Empty(t, links)
	})

	t.Run("Delete non-existing link", func(t *testing.T) {
		err := repo.DeleteByURL(ctx, "https://github.com/nonexistent/repo", chatID)

		require.Error(t, err)
		assert.IsType(t, &errors.ErrLinkNotFound{}, err)
	})
}

func TestLinkRepository_Update(t *testing.T) {
	repo := memory.NewLinkRepository()
	ctx := context.Background()

	link := &models.Link{
		URL:     "https://github.com/owner/repo",
		Type:    models.GitHub,
		Tags:    []string{"tag1", "tag2"},
		Filters: []string{"filter1", "filter2"},
	}
	require.NoError(t, repo.Save(ctx, link))

	t.Run("Update link", func(t *testing.T) {
		updatedLink := &models.Link{
			ID:          link.ID,
			URL:         link.URL,
			Type:        link.Type,
			Tags:        []string{"updated-tag1", "updated-tag2"},
			Filters:     []string{"updated-filter1", "updated-filter2"},
			LastChecked: time.Now(),
			LastUpdated: time.Now(),
		}

		err := repo.Update(ctx, updatedLink)

		require.NoError(t, err)

		retrievedLink, err := repo.FindByID(ctx, link.ID)
		require.NoError(t, err)
		assert.Equal(t, updatedLink.Tags, retrievedLink.Tags)
		assert.Equal(t, updatedLink.Filters, retrievedLink.Filters)
		assert.Equal(t, updatedLink.LastChecked.Unix(), retrievedLink.LastChecked.Unix())
		assert.Equal(t, updatedLink.LastUpdated.Unix(), retrievedLink.LastUpdated.Unix())
	})
}
