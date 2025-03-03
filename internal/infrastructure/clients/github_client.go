package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/clients"
)

type GitHubClient struct {
	httpClient *http.Client
	token      string
	baseURL    string
}

func NewGitHubClient(token, baseURL string) clients.GitHubClient {
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}

	return &GitHubClient{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		token:   token,
		baseURL: baseURL,
	}
}

type Repository struct {
	UpdatedAt time.Time `json:"updated_at"`
}

func (c *GitHubClient) GetRepositoryLastUpdate(ctx context.Context, owner, repo string) (time.Time, error) {
	url := fmt.Sprintf("%s/repos/%s/%s", c.baseURL, owner, repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return time.Time{}, err
	}

	if c.token != "" {
		req.Header.Set("Authorization", "token "+c.token)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return time.Time{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return time.Time{}, fmt.Errorf("GitHub API вернул статус: %d", resp.StatusCode)
	}

	var repository struct {
		UpdatedAt time.Time `json:"updated_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&repository); err != nil {
		return time.Time{}, err
	}

	return repository.UpdatedAt, nil
}
