package sql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/database"
	customerrors "github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
	"github.com/central-university-dev/go-Matthew11K/pkg/txs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type ChatRepository struct {
	db *database.PostgresDB
}

func NewChatRepository(db *database.PostgresDB) *ChatRepository {
	return &ChatRepository{db: db}
}

func (r *ChatRepository) Save(ctx context.Context, chat *models.Chat) error {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	now := time.Now()
	if chat.CreatedAt.IsZero() {
		chat.CreatedAt = now
	}

	if chat.UpdatedAt.IsZero() {
		chat.UpdatedAt = now
	}

	if chat.NotificationMode == "" {
		chat.NotificationMode = models.NotificationModeInstant
	}

	digestTime := time.Date(0, 1, 1, 10, 0, 0, 0, time.UTC)
	if !chat.DigestTime.IsZero() {
		digestTime = chat.DigestTime
	}

	_, err := querier.Exec(ctx,
		"INSERT INTO chats (id, notification_mode, digest_time, created_at, updated_at) VALUES ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING",
		chat.ID, chat.NotificationMode, digestTime, chat.CreatedAt, chat.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return &customerrors.ErrChatAlreadyExists{ChatID: chat.ID}
		}

		return fmt.Errorf("ошибка при сохранении чата: %w", err)
	}

	return nil
}

func (r *ChatRepository) FindByID(ctx context.Context, id int64) (*models.Chat, error) {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	row := querier.QueryRow(ctx, `
		SELECT c.id, c.notification_mode, c.digest_time, c.created_at, c.updated_at,
			COALESCE(array_agg(cl.link_id) FILTER (WHERE cl.link_id IS NOT NULL), '{}') AS links
		FROM chats c
		LEFT JOIN chat_links cl ON c.id = cl.chat_id
		WHERE c.id = $1
		GROUP BY c.id
	`, id)

	var chat models.Chat

	var notificationMode string

	var linksArr []int64

	err := row.Scan(
		&chat.ID,
		&notificationMode,
		&chat.DigestTime,
		&chat.CreatedAt,
		&chat.UpdatedAt,
		&linksArr,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &customerrors.ErrChatNotFound{ChatID: id}
		}

		return nil, fmt.Errorf("ошибка при поиске чата: %w", err)
	}

	chat.NotificationMode = models.NotificationMode(notificationMode)
	chat.Links = linksArr

	return &chat, nil
}

func (r *ChatRepository) Delete(ctx context.Context, id int64) error {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	result, err := querier.Exec(ctx, "DELETE FROM chats WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("ошибка при удалении чата: %w", err)
	}

	if result.RowsAffected() == 0 {
		return &customerrors.ErrChatNotFound{ChatID: id}
	}

	return nil
}

func (r *ChatRepository) Update(ctx context.Context, chat *models.Chat) error {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	chat.UpdatedAt = time.Now()

	result, err := querier.Exec(ctx,
		"UPDATE chats SET notification_mode = $1, digest_time = $2, updated_at = $3 WHERE id = $4",
		chat.NotificationMode, chat.DigestTime, chat.UpdatedAt, chat.ID)
	if err != nil {
		return fmt.Errorf("ошибка при обновлении чата: %w", err)
	}

	if result.RowsAffected() == 0 {
		return &customerrors.ErrChatNotFound{ChatID: chat.ID}
	}

	return nil
}

func (r *ChatRepository) UpdateNotificationSettings(
	ctx context.Context,
	chatID int64,
	mode models.NotificationMode,
	digestTime time.Time,
) error {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	result, err := querier.Exec(ctx,
		"UPDATE chats SET notification_mode = $1, digest_time = $2, updated_at = $3 WHERE id = $4",
		mode, digestTime, time.Now(), chatID)
	if err != nil {
		return fmt.Errorf("ошибка при обновлении настроек уведомлений: %w", err)
	}

	if result.RowsAffected() == 0 {
		return &customerrors.ErrChatNotFound{ChatID: chatID}
	}

	return nil
}

func (r *ChatRepository) FindByDigestTime(ctx context.Context, hour, minute int) ([]*models.Chat, error) {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	timeStr := fmt.Sprintf("%02d:%02d", hour, minute)

	rows, err := querier.Query(ctx, `
		SELECT c.id, c.notification_mode, c.digest_time, c.created_at, c.updated_at,
			COALESCE(array_agg(cl.link_id) FILTER (WHERE cl.link_id IS NOT NULL), '{}') AS links
		FROM chats c
		LEFT JOIN chat_links cl ON c.id = cl.chat_id
		WHERE c.notification_mode = $1 AND 
		      to_char(c.digest_time, 'HH24:MI') = $2
		GROUP BY c.id
		ORDER BY c.id
	`, models.NotificationModeDigest, timeStr)
	if err != nil {
		return nil, fmt.Errorf("ошибка при поиске чатов с временем дайджеста %s: %w", timeStr, err)
	}
	defer rows.Close()

	var chats []*models.Chat

	for rows.Next() {
		var chat models.Chat

		var notificationMode string

		var linksArr []int64

		err := rows.Scan(
			&chat.ID,
			&notificationMode,
			&chat.DigestTime,
			&chat.CreatedAt,
			&chat.UpdatedAt,
			&linksArr,
		)
		if err != nil {
			return nil, fmt.Errorf("ошибка при сканировании чата: %w", err)
		}

		chat.NotificationMode = models.NotificationMode(notificationMode)
		chat.Links = linksArr
		chats = append(chats, &chat)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при обработке результатов запроса чатов: %w", err)
	}

	return chats, nil
}

func (r *ChatRepository) AddLink(ctx context.Context, chatID, linkID int64) error {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	result, err := querier.Exec(ctx,
		"INSERT INTO chat_links (chat_id, link_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
		chatID, linkID)
	if err != nil {
		return fmt.Errorf("ошибка при добавлении связи чата со ссылкой: %w", err)
	}

	if result.RowsAffected() == 0 {
		var chatCount, linkCount int
		err1 := querier.QueryRow(ctx, "SELECT COUNT(*) FROM chats WHERE id = $1", chatID).Scan(&chatCount)
		err2 := querier.QueryRow(ctx, "SELECT COUNT(*) FROM links WHERE id = $1", linkID).Scan(&linkCount)

		if err1 == nil && chatCount == 0 {
			return &customerrors.ErrChatNotFound{ChatID: chatID}
		}

		if err2 == nil && linkCount == 0 {
			return &customerrors.ErrLinkNotFound{URL: fmt.Sprintf("ID: %d", linkID)}
		}

		return &customerrors.ErrLinkNotInChat{ChatID: chatID, LinkID: linkID}
	}

	return nil
}

func (r *ChatRepository) RemoveLink(ctx context.Context, chatID, linkID int64) error {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	result, err := querier.Exec(ctx, "DELETE FROM chat_links WHERE chat_id = $1 AND link_id = $2", chatID, linkID)
	if err != nil {
		return fmt.Errorf("ошибка при удалении связи чата со ссылкой: %w", err)
	}

	if result.RowsAffected() == 0 {
		var chatCount, linkCount int
		err1 := querier.QueryRow(ctx, "SELECT COUNT(*) FROM chats WHERE id = $1", chatID).Scan(&chatCount)
		err2 := querier.QueryRow(ctx, "SELECT COUNT(*) FROM links WHERE id = $1", linkID).Scan(&linkCount)

		if err1 == nil && chatCount == 0 {
			return &customerrors.ErrChatNotFound{ChatID: chatID}
		}

		if err2 == nil && linkCount == 0 {
			return &customerrors.ErrLinkNotFound{URL: fmt.Sprintf("ID: %d", linkID)}
		}

		return &customerrors.ErrLinkNotInChat{ChatID: chatID, LinkID: linkID}
	}

	var count int

	err = querier.QueryRow(ctx, "SELECT COUNT(*) FROM chat_links WHERE link_id = $1", linkID).Scan(&count)
	if err != nil {
		return fmt.Errorf("ошибка при проверке использования ссылки: %w", err)
	}

	if count == 0 {
		_, err = querier.Exec(ctx, "DELETE FROM links WHERE id = $1", linkID)
		if err != nil {
			return fmt.Errorf("ошибка при удалении ссылки: %w", err)
		}
	}

	return nil
}

func (r *ChatRepository) FindByLinkID(ctx context.Context, linkID int64) ([]*models.Chat, error) {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	rows, err := querier.Query(ctx,
		`SELECT c.id, c.notification_mode, c.digest_time, c.created_at, c.updated_at
		FROM chats c
		JOIN chat_links cl ON c.id = cl.chat_id
		WHERE cl.link_id = $1`,
		linkID)
	if err != nil {
		return nil, fmt.Errorf("ошибка при поиске чатов по ID ссылки: %w", err)
	}
	defer rows.Close()

	var chats []*models.Chat

	for rows.Next() {
		chat := &models.Chat{}

		var notificationMode string

		err := rows.Scan(&chat.ID, &notificationMode, &chat.DigestTime, &chat.CreatedAt, &chat.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("ошибка при сканировании чата: %w", err)
		}

		chat.NotificationMode = models.NotificationMode(notificationMode)
		chat.Links = []int64{}
		chats = append(chats, chat)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при обработке результатов запроса чатов: %w", err)
	}

	if len(chats) == 0 {
		var linkCount int

		err := querier.QueryRow(ctx, "SELECT COUNT(*) FROM links WHERE id = $1", linkID).Scan(&linkCount)
		if err == nil && linkCount == 0 {
			return nil, &customerrors.ErrLinkNotFound{URL: fmt.Sprintf("ID: %d", linkID)}
		}
	}

	return chats, nil
}

func (r *ChatRepository) GetAll(ctx context.Context) ([]*models.Chat, error) {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	rows, err := querier.Query(ctx, `
		SELECT c.id, c.notification_mode, c.digest_time, c.created_at, c.updated_at,
			COALESCE(array_agg(cl.link_id) FILTER (WHERE cl.link_id IS NOT NULL), '{}') AS links
		FROM chats c
		LEFT JOIN chat_links cl ON c.id = cl.chat_id
		GROUP BY c.id
		ORDER BY c.id
	`)
	if err != nil {
		return nil, fmt.Errorf("ошибка при запросе всех чатов: %w", err)
	}
	defer rows.Close()

	var chats []*models.Chat

	for rows.Next() {
		var chat models.Chat

		var notificationMode string

		var linksArr []int64

		err := rows.Scan(
			&chat.ID,
			&notificationMode,
			&chat.DigestTime,
			&chat.CreatedAt,
			&chat.UpdatedAt,
			&linksArr,
		)
		if err != nil {
			return nil, fmt.Errorf("ошибка при сканировании чата: %w", err)
		}

		chat.NotificationMode = models.NotificationMode(notificationMode)
		chat.Links = linksArr
		chats = append(chats, &chat)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при обработке результатов запроса чатов: %w", err)
	}

	return chats, nil
}

func (r *ChatRepository) ExistsChatLink(ctx context.Context, chatID, linkID int64) (bool, error) {
	querier := txs.GetQuerier(ctx, r.db.Pool)

	var exists bool

	err := querier.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM chat_links WHERE chat_id = $1 AND link_id = $2)", chatID, linkID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("ошибка при проверке связи чата и ссылки: %w", err)
	}

	return exists, nil
}
