package clients

import (
	"context"
	"time"
)

type GitHubClient interface {
	GetRepositoryLastUpdate(ctx context.Context, owner, repo string) (time.Time, error)
}
