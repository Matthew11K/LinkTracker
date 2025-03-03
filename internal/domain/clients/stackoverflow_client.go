package clients

import (
	"context"
	"time"
)

type StackOverflowClient interface {
	GetQuestionLastUpdate(ctx context.Context, questionID int64) (time.Time, error)
}
