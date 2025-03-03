package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/clients"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
)

type TelegramClient struct {
	httpClient *http.Client
	token      string
	baseURL    string
}

func NewTelegramClient(token string) clients.TelegramClient {
	return &TelegramClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		token:   token,
		baseURL: fmt.Sprintf("https://api.telegram.org/bot%s", token),
	}
}

func (c *TelegramClient) sanitizeError(err error) error {
	if err == nil {
		return nil
	}

	errorMsg := err.Error()

	re := regexp.MustCompile(`https://api\.telegram\.org/bot([^/\s]+)`)

	sanitized := re.ReplaceAllString(errorMsg, "https://api.telegram.org/bot[MASKED_TOKEN]")

	return fmt.Errorf("%s", sanitized)
}

func (c *TelegramClient) SendUpdate(ctx context.Context, update interface{}) error {
	linkUpdate, ok := update.(*models.LinkUpdate)
	if !ok {
		return fmt.Errorf("неправильный тип данных для обновления")
	}

	for _, chatID := range linkUpdate.TgChatIDs {
		message := fmt.Sprintf("🔔 *Обновление ссылки*\n\n🔗 [%s](%s)\n\n📝 %s", linkUpdate.URL, linkUpdate.URL, linkUpdate.Description)
		if err := c.SendMessage(ctx, chatID, message); err != nil {
			return err
		}
	}

	return nil
}

func (c *TelegramClient) SendMessage(ctx context.Context, chatID int64, text string) error {
	url := fmt.Sprintf("%s/sendMessage", c.baseURL)

	data := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "HTML",
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return c.sanitizeError(err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(jsonData)))
	if err != nil {
		return c.sanitizeError(err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return c.sanitizeError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorResponse struct {
			Description string `json:"description"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&errorResponse); err != nil {
			return fmt.Errorf("ошибка при отправке сообщения: статус %d", resp.StatusCode)
		}

		return fmt.Errorf("ошибка при отправке сообщения: %s", errorResponse.Description)
	}

	return nil
}

func (c *TelegramClient) GetUpdates(ctx context.Context, offset int) ([]clients.Update, error) {
	url := fmt.Sprintf("%s/getUpdates", c.baseURL)

	data := map[string]interface{}{
		"offset":  offset,
		"timeout": 30,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, c.sanitizeError(err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, c.sanitizeError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorResponse struct {
			Description string `json:"description"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&errorResponse); err != nil {
			return nil, fmt.Errorf("ошибка при получении обновлений: статус %d", resp.StatusCode)
		}

		return nil, fmt.Errorf("ошибка при получении обновлений: %s", errorResponse.Description)
	}

	var updateResponse struct {
		OK     bool             `json:"ok"`
		Result []clients.Update `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&updateResponse); err != nil {
		return nil, fmt.Errorf("ошибка при декодировании ответа: %v", err)
	}

	if !updateResponse.OK {
		return nil, fmt.Errorf("ошибка при получении обновлений: запрос не выполнен")
	}

	return updateResponse.Result, nil
}
