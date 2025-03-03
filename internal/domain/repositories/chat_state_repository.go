package repositories

import (
	"context"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
)

type ChatStateRepository interface {
	GetState(ctx context.Context, chatID int64) (models.ChatState, error)

	SetState(ctx context.Context, chatID int64, state models.ChatState) error

	GetData(ctx context.Context, chatID int64, key string) (interface{}, error)

	SetData(ctx context.Context, chatID int64, key string, value interface{}) error

	ClearData(ctx context.Context, chatID int64) error
}
