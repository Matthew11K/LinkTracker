package common

import (
	"context"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
)

type LinkUpdater interface {
	GetLastUpdate(ctx context.Context, url string) (time.Time, error)
	GetUpdateDetails(ctx context.Context, url string) (*models.UpdateInfo, error)
}

type GitHubClient interface {
	GetRepositoryLastUpdate(ctx context.Context, owner, repo string) (time.Time, error)
	GetRepositoryDetails(ctx context.Context, owner, repo string) (*models.ContentDetails, error)
}

type GitHubUpdater struct {
	client GitHubClient
}

func NewGitHubUpdater(client GitHubClient) *GitHubUpdater {
	return &GitHubUpdater{
		client: client,
	}
}

func (u *GitHubUpdater) GetLastUpdate(ctx context.Context, url string) (time.Time, error) {
	owner, repo, err := ParseGitHubURL(url)
	if err != nil {
		return time.Time{}, err
	}

	return u.client.GetRepositoryLastUpdate(ctx, owner, repo)
}

func (u *GitHubUpdater) GetUpdateDetails(ctx context.Context, url string) (*models.UpdateInfo, error) {
	owner, repo, err := ParseGitHubURL(url)
	if err != nil {
		return nil, err
	}

	details, err := u.client.GetRepositoryDetails(ctx, owner, repo)
	if err != nil {
		return nil, err
	}

	return &models.UpdateInfo{
		Title:       details.Title,
		Author:      details.Author,
		UpdatedAt:   details.UpdatedAt,
		ContentType: "repository",
		TextPreview: models.TextPreview(details.ContentText, 200),
		FullText:    details.ContentText,
	}, nil
}

type StackOverflowClient interface {
	GetQuestionLastUpdate(ctx context.Context, questionID int64) (time.Time, error)
	GetQuestionDetails(ctx context.Context, questionID int64) (*models.ContentDetails, error)
}

type StackOverflowUpdater struct {
	client StackOverflowClient
}

func NewStackOverflowUpdater(client StackOverflowClient) *StackOverflowUpdater {
	return &StackOverflowUpdater{
		client: client,
	}
}

func (u *StackOverflowUpdater) GetLastUpdate(ctx context.Context, url string) (time.Time, error) {
	questionID, err := ParseStackOverflowURL(url)
	if err != nil {
		return time.Time{}, err
	}

	return u.client.GetQuestionLastUpdate(ctx, questionID)
}

func (u *StackOverflowUpdater) GetUpdateDetails(ctx context.Context, url string) (*models.UpdateInfo, error) {
	questionID, err := ParseStackOverflowURL(url)
	if err != nil {
		return nil, err
	}

	details, err := u.client.GetQuestionDetails(ctx, questionID)
	if err != nil {
		return nil, err
	}

	return &models.UpdateInfo{
		Title:       details.Title,
		Author:      details.Author,
		UpdatedAt:   details.UpdatedAt,
		ContentType: "question",
		TextPreview: models.TextPreview(details.ContentText, 200),
		FullText:    details.ContentText,
	}, nil
}

type LinkUpdaterFactory struct {
	githubUpdater        *GitHubUpdater
	stackOverflowUpdater *StackOverflowUpdater
}

func NewLinkUpdaterFactory(githubClient GitHubClient, stackOverflowClient StackOverflowClient) *LinkUpdaterFactory {
	return &LinkUpdaterFactory{
		githubUpdater:        NewGitHubUpdater(githubClient),
		stackOverflowUpdater: NewStackOverflowUpdater(stackOverflowClient),
	}
}

func (f *LinkUpdaterFactory) CreateUpdater(linkType models.LinkType) (LinkUpdater, error) {
	//nolint:exhaustive // Unknown обрабатывается в default
	switch linkType {
	case models.GitHub:
		return f.githubUpdater, nil
	case models.StackOverflow:
		return f.stackOverflowUpdater, nil
	default:
		return nil, &errors.ErrUnsupportedLinkType{URL: string(linkType)}
	}
}
