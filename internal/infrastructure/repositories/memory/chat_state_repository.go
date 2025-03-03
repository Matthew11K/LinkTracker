package memory

import (
	"context"
	"sync"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
)

type ChatStateRepository struct {
	states map[int64]models.ChatState
	data   map[int64]map[string]interface{}
	mu     sync.RWMutex
}

func NewChatStateRepository() *ChatStateRepository {
	return &ChatStateRepository{
		states: make(map[int64]models.ChatState),
		data:   make(map[int64]map[string]interface{}),
	}
}

func (r *ChatStateRepository) GetState(ctx context.Context, chatID int64) (models.ChatState, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state, exists := r.states[chatID]
	if !exists {
		return models.StateIdle, nil
	}

	return state, nil
}

func (r *ChatStateRepository) SetState(ctx context.Context, chatID int64, state models.ChatState) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.states[chatID] = state

	return nil
}

func (r *ChatStateRepository) GetData(ctx context.Context, chatID int64, key string) (interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	chatData, exists := r.data[chatID]
	if !exists {
		return nil, nil
	}

	value, exists := chatData[key]
	if !exists {
		return nil, nil
	}

	return value, nil
}

func (r *ChatStateRepository) SetData(ctx context.Context, chatID int64, key string, value interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.data[chatID]; !exists {
		r.data[chatID] = make(map[string]interface{})
	}

	r.data[chatID][key] = value

	return nil
}

func (r *ChatStateRepository) ClearData(ctx context.Context, chatID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.data, chatID)

	return nil
}
