package clients

import (
	"context"
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/clients"
)

type GitHubClient struct {
	client  *resty.Client
	token   string
	baseURL string
}

func NewGitHubClient(token, baseURL string) clients.GitHubClient {
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}

	client := resty.New()
	client.SetTimeout(10 * time.Second)

	return &GitHubClient{
		client:  client,
		token:   token,
		baseURL: baseURL,
	}
}

type Repository struct {
	UpdatedAt time.Time `json:"updated_at"`
}

func (c *GitHubClient) GetRepositoryLastUpdate(ctx context.Context, owner, repo string) (time.Time, error) {
	url := fmt.Sprintf("%s/repos/%s/%s", c.baseURL, owner, repo)

	request := c.client.R().
		SetContext(ctx).
		SetHeader("Accept", "application/vnd.github.v3+json")

	if c.token != "" {
		request.SetHeader("Authorization", "token "+c.token)
	}

	var repository struct {
		UpdatedAt time.Time `json:"updated_at"`
	}

	resp, err := request.
		SetResult(&repository).
		Get(url)

	if err != nil {
		return time.Time{}, err
	}

	if !resp.IsSuccess() {
		return time.Time{}, fmt.Errorf("GitHub API вернул статус: %d", resp.StatusCode())
	}

	return repository.UpdatedAt, nil
}
