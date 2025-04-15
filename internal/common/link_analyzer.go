package common

import (
	"regexp"
	"strconv"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
)

type LinkAnalyzer struct {
	githubRegex        *regexp.Regexp
	stackoverflowRegex *regexp.Regexp
}

func NewLinkAnalyzer() *LinkAnalyzer {
	return &LinkAnalyzer{
		githubRegex:        regexp.MustCompile(`^https?://(?:www\.)?github\.com/([^/]+)/([^/]+)(?:/.*)?$`),
		stackoverflowRegex: regexp.MustCompile(`^https?://(?:www\.)?stackoverflow\.com/questions/(\d+)(?:/.*)?$`),
	}
}

func (a *LinkAnalyzer) AnalyzeLink(url string) models.LinkType {
	if a.githubRegex.MatchString(url) {
		return models.GitHub
	}

	if a.stackoverflowRegex.MatchString(url) {
		return models.StackOverflow
	}

	return models.Unknown
}

func ParseGitHubURL(url string) (owner, repo string, err error) {
	re := regexp.MustCompile(`^https?://(?:www\.)?github\.com/([^/]+)/([^/]+)(?:/.*)?$`)

	matches := re.FindStringSubmatch(url)
	if len(matches) < 3 {
		return "", "", &errors.ErrInvalidURL{URL: url}
	}

	return matches[1], matches[2], nil
}

func ParseStackOverflowURL(url string) (questionID int64, err error) {
	re := regexp.MustCompile(`^https?://(?:www\.)?stackoverflow\.com/questions/(\d+)(?:/.*)?$`)

	matches := re.FindStringSubmatch(url)
	if len(matches) < 2 {
		return 0, &errors.ErrInvalidURL{URL: url}
	}

	questionID, err = strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0, &errors.ErrInvalidURL{URL: url}
	}

	return questionID, nil
}
