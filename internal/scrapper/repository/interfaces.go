package repository

import (
	"context"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
)

type LinkRepository interface {
	Save(ctx context.Context, link *models.Link) error
	FindByID(ctx context.Context, id int64) (*models.Link, error)
	FindByURL(ctx context.Context, url string) (*models.Link, error)
	FindByChatID(ctx context.Context, chatID int64) ([]*models.Link, error)
	DeleteByURL(ctx context.Context, url string, chatID int64) error
	Update(ctx context.Context, link *models.Link) error
	AddChatLink(ctx context.Context, chatID, linkID int64) error
	FindDue(ctx context.Context, limit, offset int) ([]*models.Link, error)
	Count(ctx context.Context) (int, error)
	SaveTags(ctx context.Context, linkID int64, tags []string) error
	SaveFilters(ctx context.Context, linkID int64, filters []string) error
	GetAll(ctx context.Context) ([]*models.Link, error)
}

type ChatRepository interface {
	Save(ctx context.Context, chat *models.Chat) error
	FindByID(ctx context.Context, id int64) (*models.Chat, error)
	Delete(ctx context.Context, id int64) error
	Update(ctx context.Context, chat *models.Chat) error
	AddLink(ctx context.Context, chatID, linkID int64) error
	RemoveLink(ctx context.Context, chatID, linkID int64) error
	FindByLinkID(ctx context.Context, linkID int64) ([]*models.Chat, error)
	GetAll(ctx context.Context) ([]*models.Chat, error)
	ExistsChatLink(ctx context.Context, chatID, linkID int64) (bool, error)
	UpdateNotificationSettings(ctx context.Context, chatID int64, mode models.NotificationMode, digestTime time.Time) error
	FindByDigestTime(ctx context.Context, hour, minute int) ([]*models.Chat, error)
}
