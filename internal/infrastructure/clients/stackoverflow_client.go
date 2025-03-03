package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/clients"
)

type StackOverflowClient struct {
	httpClient *http.Client
	baseURL    string
	key        string
}

func NewStackOverflowClient(key string) clients.StackOverflowClient {
	return &StackOverflowClient{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL: "https://api.stackexchange.com/2.3",
		key:     key,
	}
}

type StackOverflowResponse struct {
	Items []Question `json:"items"`
}

type Question struct {
	LastActivityDate int64 `json:"last_activity_date"`
}

func (c *StackOverflowClient) GetQuestionLastUpdate(ctx context.Context, questionID int64) (time.Time, error) {
	url := fmt.Sprintf("%s/questions/%d?site=stackoverflow", c.baseURL, questionID)
	if c.key != "" {
		url += "&key=" + c.key
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return time.Time{}, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return time.Time{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return time.Time{}, fmt.Errorf("StackOverflow API вернул статус: %d", resp.StatusCode)
	}

	var response struct {
		Items []struct {
			LastActivityDate int64 `json:"last_activity_date"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return time.Time{}, err
	}

	if len(response.Items) == 0 {
		return time.Time{}, fmt.Errorf("вопрос с ID %d не найден", questionID)
	}

	lastUpdate := time.Unix(response.Items[0].LastActivityDate, 0)

	return lastUpdate, nil
}
