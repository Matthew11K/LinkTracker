package models

import "time"

type GitHubDetails struct {
	LinkID      int64
	Title       string
	Author      string
	UpdatedAt   time.Time
	Description string
}

type StackOverflowDetails struct {
	LinkID    int64
	Title     string
	Author    string
	UpdatedAt time.Time
	Content   string
}

func TextPreview(text string, length int) string {
	if len(text) <= length {
		return text
	}

	return text[:length] + "..."
}
