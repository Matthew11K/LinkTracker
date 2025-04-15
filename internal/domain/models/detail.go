package models

import "time"

type ContentDetails struct {
	LinkID      int64
	Title       string
	Author      string
	UpdatedAt   time.Time
	ContentText string
	LinkType    LinkType
}

func TextPreview(text string, length int) string {
	if len(text) <= length {
		return text
	}

	return text[:length] + "..."
}
