package models

import (
	"time"
)

type Chat struct {
	ID        int64
	Links     []int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ChatState int

const (
	StateIdle ChatState = iota
	StateAwaitingLink
	StateAwaitingTags
	StateAwaitingFilters
	StateAwaitingUntrackLink
)
