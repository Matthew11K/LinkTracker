package repository

import (
	"context"
	"sync"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
)

type LinkRepository struct {
	links      map[int64]*models.Link
	linksByURL map[string]int64
	chatLinks  map[int64][]int64
	mu         sync.RWMutex
	nextID     int64
}

func NewLinkRepository() *LinkRepository {
	return &LinkRepository{
		links:      make(map[int64]*models.Link),
		linksByURL: make(map[string]int64),
		chatLinks:  make(map[int64][]int64),
		nextID:     1,
	}
}

func (r *LinkRepository) Save(_ context.Context, link *models.Link) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.linksByURL[link.URL]; exists {
		return &errors.ErrLinkAlreadyExists{URL: link.URL}
	}

	if link.ID == 0 {
		link.ID = r.nextID
		r.nextID++
	}

	if link.CreatedAt.IsZero() {
		link.CreatedAt = time.Now()
	}

	r.links[link.ID] = link
	r.linksByURL[link.URL] = link.ID

	return nil
}

func (r *LinkRepository) FindByID(_ context.Context, id int64) (*models.Link, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	link, exists := r.links[id]
	if !exists {
		return nil, &errors.ErrLinkNotFound{URL: "ID: " + string(rune(id))}
	}

	return link, nil
}

func (r *LinkRepository) FindByURL(_ context.Context, url string) (*models.Link, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, exists := r.linksByURL[url]
	if !exists {
		return nil, &errors.ErrLinkNotFound{URL: url}
	}

	return r.links[id], nil
}

func (r *LinkRepository) FindByChatID(_ context.Context, chatID int64) ([]*models.Link, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	linkIDs, exists := r.chatLinks[chatID]
	if !exists {
		return []*models.Link{}, nil
	}

	links := make([]*models.Link, 0, len(linkIDs))
	for _, id := range linkIDs {
		if link, ok := r.links[id]; ok {
			links = append(links, link)
		}
	}

	return links, nil
}

func (r *LinkRepository) DeleteByURL(_ context.Context, url string, chatID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	id, exists := r.linksByURL[url]
	if !exists {
		return &errors.ErrLinkNotFound{URL: url}
	}

	linkIDs, exists := r.chatLinks[chatID]
	if !exists {
		return &errors.ErrChatNotFound{ChatID: chatID}
	}

	found := false

	for i, linkID := range linkIDs {
		if linkID == id {
			r.chatLinks[chatID] = append(linkIDs[:i], linkIDs[i+1:]...)
			found = true

			break
		}
	}

	if !found {
		return &errors.ErrLinkNotFound{URL: url}
	}

	inUse := false

	for _, chatLinks := range r.chatLinks {
		for _, linkID := range chatLinks {
			if linkID == id {
				inUse = true
				break
			}
		}

		if inUse {
			break
		}
	}

	if !inUse {
		delete(r.links, id)
		delete(r.linksByURL, url)
	}

	return nil
}

func (r *LinkRepository) GetAll(_ context.Context) ([]*models.Link, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	links := make([]*models.Link, 0, len(r.links))
	for _, link := range r.links {
		links = append(links, link)
	}

	return links, nil
}

func (r *LinkRepository) Update(_ context.Context, link *models.Link) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, exists := r.links[link.ID]
	if !exists {
		return &errors.ErrLinkNotFound{URL: link.URL}
	}

	r.links[link.ID] = link

	return nil
}

func (r *LinkRepository) AddChatLink(_ context.Context, chatID, linkID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.links[linkID]; !exists {
		return &errors.ErrLinkNotFound{URL: "ID: " + string(rune(linkID))}
	}

	linkIDs, exists := r.chatLinks[chatID]
	if !exists {
		r.chatLinks[chatID] = []int64{linkID}
		return nil
	}

	for _, id := range linkIDs {
		if id == linkID {
			return nil
		}
	}

	r.chatLinks[chatID] = append(linkIDs, linkID)

	return nil
}
