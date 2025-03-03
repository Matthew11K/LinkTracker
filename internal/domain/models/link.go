package models

import (
	"time"
)

type LinkType string

const (
	GitHub        LinkType = "github"
	StackOverflow LinkType = "stackoverflow"
	Unknown       LinkType = "unknown"
)

type Link struct {
	ID          int64
	URL         string
	Type        LinkType
	Tags        []string
	Filters     []string
	LastChecked time.Time
	LastUpdated time.Time
	CreatedAt   time.Time
}

type LinkUpdate struct {
	ID          int64
	URL         string
	Description string
	TgChatIDs   []int64
}
