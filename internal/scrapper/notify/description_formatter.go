package notify

import (
	"fmt"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
)

func formatDescription(update *models.LinkUpdate) string {
	description := update.Description

	if update.UpdateInfo != nil {
		info := update.UpdateInfo

		switch info.ContentType {
		case "repository", "issue", "pull_request":
			description = fmt.Sprintf("%s\n\n🔷 GitHub обновление 🔷\n"+
				"📎 Название: %s\n"+
				"👤 Автор: %s\n"+
				"⏱️ Время: %s\n"+
				"📄 Тип: %s\n"+
				"📝 Превью:\n%s",
				description, info.Title, info.Author,
				info.UpdatedAt.Format("2006-01-02 15:04:05"),
				info.ContentType, info.TextPreview)
		case "question", "answer", "comment":
			description = fmt.Sprintf("%s\n\n🔶 StackOverflow обновление 🔶\n"+
				"📎 Тема вопроса: %s\n"+
				"👤 Пользователь: %s\n"+
				"⏱️ Время: %s\n"+
				"📄 Тип: %s\n"+
				"📝 Превью:\n%s",
				description, info.Title, info.Author,
				info.UpdatedAt.Format("2006-01-02 15:04:05"),
				info.ContentType, info.TextPreview)
		default:
			description = fmt.Sprintf("%s\n\n🔹 Обновление ресурса 🔹\n"+
				"📎 Заголовок: %s\n"+
				"👤 Автор: %s\n"+
				"⏱️ Время: %s\n"+
				"📄 Тип: %s\n"+
				"📝 Превью:\n%s",
				description, info.Title, info.Author,
				info.UpdatedAt.Format("2006-01-02 15:04:05"),
				info.ContentType, info.TextPreview)
		}
	}

	return description
}
