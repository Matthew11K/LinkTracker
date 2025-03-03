package repositories

import (
	"context"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
)

type LinkRepository interface {
	Save(ctx context.Context, link *models.Link) error

	FindByID(ctx context.Context, id int64) (*models.Link, error)

	FindByURL(ctx context.Context, url string) (*models.Link, error)

	FindByChatID(ctx context.Context, chatID int64) ([]*models.Link, error)

	DeleteByURL(ctx context.Context, url string, chatID int64) error

	GetAll(ctx context.Context) ([]*models.Link, error)

	Update(ctx context.Context, link *models.Link) error

	AddChatLink(ctx context.Context, chatID int64, linkID int64) error
}
