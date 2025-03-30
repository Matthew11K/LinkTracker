package clients

import (
	"context"
	"fmt"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
	"github.com/go-resty/resty/v2"
)

type GitHubClient struct {
	client  *resty.Client
	token   string
	baseURL string
}

type RepositoryUpdateGetter interface {
	GetRepositoryLastUpdate(ctx context.Context, owner, repo string) (time.Time, error)
	GetRepositoryDetails(ctx context.Context, owner, repo string) (*models.GitHubDetails, error)
}

func NewGitHubClient(token, baseURL string) RepositoryUpdateGetter {
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
	UpdatedAt   time.Time `json:"updated_at"`
	Name        string    `json:"name"`
	FullName    string    `json:"full_name"`
	Description string    `json:"description"`
	Owner       struct {
		Login string `json:"login"`
	} `json:"owner"`
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

func (c *GitHubClient) GetRepositoryDetails(ctx context.Context, owner, repo string) (*models.GitHubDetails, error) {
	url := fmt.Sprintf("%s/repos/%s/%s", c.baseURL, owner, repo)

	request := c.client.R().
		SetContext(ctx).
		SetHeader("Accept", "application/vnd.github.v3+json")

	if c.token != "" {
		request.SetHeader("Authorization", "token "+c.token)
	}

	var repository Repository

	resp, err := request.
		SetResult(&repository).
		Get(url)

	if err != nil {
		return nil, err
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf("GitHub API вернул статус: %d", resp.StatusCode())
	}

	details := &models.GitHubDetails{
		Title:       repository.FullName,
		Author:      repository.Owner.Login,
		UpdatedAt:   repository.UpdatedAt,
		Description: repository.Description,
	}

	return details, nil
}
