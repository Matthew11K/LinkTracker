package memory

import (
	"context"
	"sync"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
)

type ChatRepository struct {
	chats map[int64]*models.Chat
	mu    sync.RWMutex
}

func NewChatRepository() *ChatRepository {
	return &ChatRepository{
		chats: make(map[int64]*models.Chat),
	}
}

func (r *ChatRepository) Save(ctx context.Context, chat *models.Chat) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if chat.CreatedAt.IsZero() {
		chat.CreatedAt = time.Now()
	}

	chat.UpdatedAt = time.Now()

	r.chats[chat.ID] = chat

	return nil
}

func (r *ChatRepository) FindByID(ctx context.Context, id int64) (*models.Chat, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	chat, exists := r.chats[id]
	if !exists {
		return nil, &errors.ErrChatNotFound{ChatID: id}
	}

	return chat, nil
}

func (r *ChatRepository) Delete(ctx context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.chats[id]; !exists {
		return &errors.ErrChatNotFound{ChatID: id}
	}

	delete(r.chats, id)

	return nil
}

func (r *ChatRepository) Update(ctx context.Context, chat *models.Chat) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.chats[chat.ID]; !exists {
		return &errors.ErrChatNotFound{ChatID: chat.ID}
	}

	chat.UpdatedAt = time.Now()

	r.chats[chat.ID] = chat

	return nil
}

func (r *ChatRepository) AddLink(ctx context.Context, chatID int64, linkID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	chat, exists := r.chats[chatID]
	if !exists {
		return &errors.ErrChatNotFound{ChatID: chatID}
	}

	for _, id := range chat.Links {
		if id == linkID {
			return nil
		}
	}

	chat.Links = append(chat.Links, linkID)
	chat.UpdatedAt = time.Now()

	return nil
}

func (r *ChatRepository) RemoveLink(ctx context.Context, chatID int64, linkID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	chat, exists := r.chats[chatID]
	if !exists {
		return &errors.ErrChatNotFound{ChatID: chatID}
	}

	found := false

	for i, id := range chat.Links {
		if id == linkID {
			chat.Links = append(chat.Links[:i], chat.Links[i+1:]...)
			found = true

			break
		}
	}

	if !found {
		return &errors.ErrLinkNotFound{URL: "ID: " + string(rune(linkID))}
	}

	chat.UpdatedAt = time.Now()

	return nil
}

func (r *ChatRepository) GetAll(ctx context.Context) ([]*models.Chat, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	chats := make([]*models.Chat, 0, len(r.chats))
	for _, chat := range r.chats {
		chats = append(chats, chat)
	}

	return chats, nil
}
