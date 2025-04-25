package models

import (
	"time"
)

type NotificationMode string

const (
	NotificationModeInstant NotificationMode = "instant"

	NotificationModeDigest NotificationMode = "digest"
)

type Chat struct {
	ID               int64
	Links            []int64
	NotificationMode NotificationMode
	DigestTime       time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type ChatState int

const (
	StateIdle ChatState = iota
	StateAwaitingLink
	StateAwaitingTags
	StateAwaitingFilters
	StateAwaitingUntrackLink
	StateAwaitingNotificationMode
	StateAwaitingDigestTime
)
