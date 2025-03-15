package common

import (
	"context"
	"time"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
)

type LinkUpdater interface {
	GetLastUpdate(ctx context.Context, url string) (time.Time, error)
}

type GitHubClient interface {
	GetRepositoryLastUpdate(ctx context.Context, owner, repo string) (time.Time, error)
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

type StackOverflowClient interface {
	GetQuestionLastUpdate(ctx context.Context, questionID int64) (time.Time, error)
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

type LinkUpdaterFactory struct {
	githubUpdater        LinkUpdater
	stackOverflowUpdater LinkUpdater
}

func NewLinkUpdaterFactory(
	githubClient GitHubClient,
	stackOverflowClient StackOverflowClient,
) *LinkUpdaterFactory {
	return &LinkUpdaterFactory{
		githubUpdater:        NewGitHubUpdater(githubClient),
		stackOverflowUpdater: NewStackOverflowUpdater(stackOverflowClient),
	}
}

func (f *LinkUpdaterFactory) CreateUpdater(linkType models.LinkType) (LinkUpdater, error) {
	//nolint:exhaustive // обработка models.Unknown находится в блоке default
	switch linkType {
	case models.GitHub:
		return f.githubUpdater, nil
	case models.StackOverflow:
		return f.stackOverflowUpdater, nil
	default:
		return nil, &errors.ErrUnsupportedLinkType{URL: ""}
	}
}
