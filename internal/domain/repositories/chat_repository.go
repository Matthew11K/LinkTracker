package repositories

import (
	"context"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
)

type ChatRepository interface {
	Save(ctx context.Context, chat *models.Chat) error

	FindByID(ctx context.Context, id int64) (*models.Chat, error)

	Delete(ctx context.Context, id int64) error

	Update(ctx context.Context, chat *models.Chat) error

	AddLink(ctx context.Context, chatID int64, linkID int64) error

	RemoveLink(ctx context.Context, chatID int64, linkID int64) error

	GetAll(ctx context.Context) ([]*models.Chat, error)
}
