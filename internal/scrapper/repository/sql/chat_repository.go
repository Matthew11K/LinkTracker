package sql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/database"
	customerrors "github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
	"github.com/jackc/pgx/v5"
)

type ChatRepository struct {
	db *database.PostgresDB
}

func NewChatRepository(db *database.PostgresDB) *ChatRepository {
	return &ChatRepository{db: db}
}

func (r *ChatRepository) Save(ctx context.Context, chat *models.Chat) error {
	now := time.Now()

	if chat.CreatedAt.IsZero() {
		chat.CreatedAt = now
	}

	chat.UpdatedAt = now

	_, err := r.db.Pool.Exec(ctx,
		"INSERT INTO chats (id, created_at, updated_at) VALUES ($1, $2, $3) ON CONFLICT (id) DO UPDATE SET updated_at = $3",
		chat.ID, chat.CreatedAt, chat.UpdatedAt)
	if err != nil {
		return fmt.Errorf("ошибка при сохранении чата: %w", err)
	}

	return nil
}

func (r *ChatRepository) FindByID(ctx context.Context, id int64) (*models.Chat, error) {
	chat := &models.Chat{ID: id}

	err := r.db.Pool.QueryRow(ctx,
		"SELECT created_at, updated_at FROM chats WHERE id = $1",
		id).Scan(&chat.CreatedAt, &chat.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &customerrors.ErrChatNotFound{ChatID: id}
		}
		return nil, fmt.Errorf("ошибка при поиске чата: %w", err)
	}

	links, err := r.getLinkIDs(ctx, id)
	if err != nil {
		return nil, err
	}
	chat.Links = links

	return chat, nil
}

func (r *ChatRepository) getLinkIDs(ctx context.Context, chatID int64) ([]int64, error) {
	rows, err := r.db.Pool.Query(ctx,
		"SELECT link_id FROM chat_links WHERE chat_id = $1",
		chatID)
	if err != nil {
		return nil, fmt.Errorf("ошибка при запросе ссылок чата: %w", err)
	}
	defer rows.Close()

	var linkIDs []int64
	for rows.Next() {
		var linkID int64
		if err := rows.Scan(&linkID); err != nil {
			return nil, fmt.Errorf("ошибка при сканировании ID ссылки: %w", err)
		}
		linkIDs = append(linkIDs, linkID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при обработке результатов запроса ссылок чата: %w", err)
	}

	return linkIDs, nil
}

func (r *ChatRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.Pool.Exec(ctx, "DELETE FROM chats WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("ошибка при удалении чата: %w", err)
	}

	if result.RowsAffected() == 0 {
		return &customerrors.ErrChatNotFound{ChatID: id}
	}

	return nil
}

func (r *ChatRepository) Update(ctx context.Context, chat *models.Chat) error {
	chat.UpdatedAt = time.Now()

	result, err := r.db.Pool.Exec(ctx,
		"UPDATE chats SET updated_at = $1 WHERE id = $2",
		chat.UpdatedAt, chat.ID)
	if err != nil {
		return fmt.Errorf("ошибка при обновлении чата: %w", err)
	}

	if result.RowsAffected() == 0 {
		return &customerrors.ErrChatNotFound{ChatID: chat.ID}
	}

	return nil
}

func (r *ChatRepository) AddLink(ctx context.Context, chatID, linkID int64) error {
	var exists bool
	err := r.db.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM chats WHERE id = $1)", chatID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("ошибка при проверке существования чата: %w", err)
	}

	if !exists {
		return &customerrors.ErrChatNotFound{ChatID: chatID}
	}

	err = r.db.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM links WHERE id = $1)", linkID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("ошибка при проверке существования ссылки: %w", err)
	}

	if !exists {
		return &customerrors.ErrLinkNotFound{URL: fmt.Sprintf("ID: %d", linkID)}
	}

	_, err = r.db.Pool.Exec(ctx,
		"INSERT INTO chat_links (chat_id, link_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
		chatID, linkID)
	if err != nil {
		return fmt.Errorf("ошибка при связывании чата и ссылки: %w", err)
	}

	_, err = r.db.Pool.Exec(ctx, "UPDATE chats SET updated_at = $1 WHERE id = $2", time.Now(), chatID)
	if err != nil {
		return fmt.Errorf("ошибка при обновлении времени чата: %w", err)
	}

	return nil
}

func (r *ChatRepository) RemoveLink(ctx context.Context, chatID, linkID int64) error {
	result, err := r.db.Pool.Exec(ctx,
		"DELETE FROM chat_links WHERE chat_id = $1 AND link_id = $2",
		chatID, linkID)
	if err != nil {
		return fmt.Errorf("ошибка при удалении связи чата и ссылки: %w", err)
	}

	if result.RowsAffected() == 0 {
		var exists bool
		err := r.db.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM chats WHERE id = $1)", chatID).Scan(&exists)
		if err != nil {
			return fmt.Errorf("ошибка при проверке существования чата: %w", err)
		}

		if !exists {
			return &customerrors.ErrChatNotFound{ChatID: chatID}
		}

		return &customerrors.ErrLinkNotFound{URL: fmt.Sprintf("ID: %d", linkID)}
	}

	_, err = r.db.Pool.Exec(ctx, "UPDATE chats SET updated_at = $1 WHERE id = $2", time.Now(), chatID)
	if err != nil {
		return fmt.Errorf("ошибка при обновлении времени чата: %w", err)
	}

	return nil
}

func (r *ChatRepository) FindByLinkID(ctx context.Context, linkID int64) ([]*models.Chat, error) {
	var exists bool
	err := r.db.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM links WHERE id = $1)", linkID).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("ошибка при проверке существования ссылки: %w", err)
	}

	if !exists {
		return nil, &customerrors.ErrLinkNotFound{URL: fmt.Sprintf("ID: %d", linkID)}
	}

	rows, err := r.db.Pool.Query(ctx,
		`SELECT c.id, c.created_at, c.updated_at 
		FROM chats c 
		JOIN chat_links cl ON c.id = cl.chat_id 
		WHERE cl.link_id = $1`,
		linkID)
	if err != nil {
		return nil, fmt.Errorf("ошибка при запросе чатов по link_id: %w", err)
	}
	defer rows.Close()

	var chats []*models.Chat
	for rows.Next() {
		chat := &models.Chat{}
		if err := rows.Scan(&chat.ID, &chat.CreatedAt, &chat.UpdatedAt); err != nil {
			return nil, fmt.Errorf("ошибка при сканировании чата: %w", err)
		}

		links, err := r.getLinkIDs(ctx, chat.ID)
		if err != nil {
			return nil, err
		}
		chat.Links = links

		chats = append(chats, chat)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при обработке результатов запроса чатов: %w", err)
	}

	return chats, nil
}

func (r *ChatRepository) GetAll(ctx context.Context) ([]*models.Chat, error) {
	rows, err := r.db.Pool.Query(ctx,
		"SELECT id, created_at, updated_at FROM chats ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении всех чатов: %w", err)
	}
	defer rows.Close()

	chats := make([]*models.Chat, 0)
	for rows.Next() {
		chat := &models.Chat{}
		err := rows.Scan(&chat.ID, &chat.CreatedAt, &chat.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("ошибка при сканировании чата: %w", err)
		}

		linkIDs, err := r.getLinkIDs(ctx, chat.ID)
		if err != nil {
			return nil, err
		}
		chat.Links = linkIDs

		chats = append(chats, chat)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при итерации по чатам: %w", err)
	}

	return chats, nil
}
