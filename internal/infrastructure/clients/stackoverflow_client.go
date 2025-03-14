package clients

import (
	"context"
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/clients"
)

type StackOverflowClient struct {
	client  *resty.Client
	baseURL string
	key     string
}

func NewStackOverflowClient(key, baseURL string) clients.StackOverflowClient {
	if baseURL == "" {
		baseURL = "https://api.stackexchange.com/2.3"
	}

	client := resty.New()
	client.SetTimeout(10 * time.Second)

	return &StackOverflowClient{
		client:  client,
		baseURL: baseURL,
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
	url := fmt.Sprintf("%s/questions/%d", c.baseURL, questionID)

	request := c.client.R().
		SetContext(ctx).
		SetQueryParam("site", "stackoverflow")

	if c.key != "" {
		request.SetQueryParam("key", c.key)
	}

	var response struct {
		Items []struct {
			LastActivityDate int64 `json:"last_activity_date"`
		} `json:"items"`
	}

	resp, err := request.
		SetResult(&response).
		Get(url)

	if err != nil {
		return time.Time{}, err
	}

	if !resp.IsSuccess() {
		return time.Time{}, fmt.Errorf("StackOverflow API вернул статус: %d", resp.StatusCode())
	}

	if len(response.Items) == 0 {
		return time.Time{}, fmt.Errorf("вопрос с ID %d не найден", questionID)
	}

	lastUpdate := time.Unix(response.Items[0].LastActivityDate, 0)

	return lastUpdate, nil
}
