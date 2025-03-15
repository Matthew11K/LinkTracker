package common_test

import (
	"testing"

	"github.com/central-university-dev/go-Matthew11K/internal/common"

	"github.com/central-university-dev/go-Matthew11K/internal/domain/errors"
	"github.com/central-university-dev/go-Matthew11K/internal/domain/models"
	"github.com/stretchr/testify/assert"
)

func TestLinkAnalyzer_AnalyzeLink(t *testing.T) {
	analyzer := common.NewLinkAnalyzer()

	tests := []struct {
		name     string
		url      string
		expected models.LinkType
	}{
		{
			name:     "GitHub URL",
			url:      "https://github.com/owner/repo",
			expected: models.GitHub,
		},
		{
			name:     "GitHub URL with www",
			url:      "https://www.github.com/owner/repo",
			expected: models.GitHub,
		},
		{
			name:     "GitHub URL with path",
			url:      "https://github.com/owner/repo/tree/main",
			expected: models.GitHub,
		},
		{
			name:     "GitHub URL with HTTP",
			url:      "http://github.com/owner/repo",
			expected: models.GitHub,
		},
		{
			name:     "StackOverflow URL",
			url:      "https://stackoverflow.com/questions/12345",
			expected: models.StackOverflow,
		},
		{
			name:     "StackOverflow URL with www",
			url:      "https://www.stackoverflow.com/questions/12345",
			expected: models.StackOverflow,
		},
		{
			name:     "StackOverflow URL with path",
			url:      "https://stackoverflow.com/questions/12345/some-question",
			expected: models.StackOverflow,
		},
		{
			name:     "StackOverflow URL with HTTP",
			url:      "http://stackoverflow.com/questions/12345",
			expected: models.StackOverflow,
		},
		{
			name:     "Invalid URL",
			url:      "https://example.com",
			expected: models.Unknown,
		},
		{
			name:     "Empty URL",
			url:      "",
			expected: models.Unknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.AnalyzeLink(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseGitHubURL(t *testing.T) {
	tests := []struct {
		name          string
		url           string
		expectedOwner string
		expectedRepo  string
		expectedErr   error
	}{
		{
			name:          "Valid GitHub URL",
			url:           "https://github.com/owner/repo",
			expectedOwner: "owner",
			expectedRepo:  "repo",
			expectedErr:   nil,
		},
		{
			name:          "GitHub URL with path",
			url:           "https://github.com/owner/repo/tree/main",
			expectedOwner: "owner",
			expectedRepo:  "repo",
			expectedErr:   nil,
		},
		{
			name:          "GitHub URL with www",
			url:           "https://www.github.com/owner/repo",
			expectedOwner: "owner",
			expectedRepo:  "repo",
			expectedErr:   nil,
		},
		{
			name:          "Invalid GitHub URL format",
			url:           "https://github.com/owner",
			expectedOwner: "",
			expectedRepo:  "",
			expectedErr:   &errors.ErrInvalidURL{URL: "https://github.com/owner"},
		},
		{
			name:          "Non-GitHub URL",
			url:           "https://example.com",
			expectedOwner: "",
			expectedRepo:  "",
			expectedErr:   &errors.ErrInvalidURL{URL: "https://example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := common.ParseGitHubURL(tt.url)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedErr.Error(), err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedOwner, owner)
				assert.Equal(t, tt.expectedRepo, repo)
			}
		})
	}
}

func TestParseStackOverflowURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expectedID  int64
		expectedErr error
	}{
		{
			name:        "Valid StackOverflow URL",
			url:         "https://stackoverflow.com/questions/12345",
			expectedID:  12345,
			expectedErr: nil,
		},
		{
			name:        "StackOverflow URL with title",
			url:         "https://stackoverflow.com/questions/12345/some-question-title",
			expectedID:  12345,
			expectedErr: nil,
		},
		{
			name:        "StackOverflow URL with www",
			url:         "https://www.stackoverflow.com/questions/12345",
			expectedID:  12345,
			expectedErr: nil,
		},
		{
			name:        "Invalid StackOverflow URL format",
			url:         "https://stackoverflow.com/questions/abc",
			expectedID:  0,
			expectedErr: &errors.ErrInvalidURL{URL: "https://stackoverflow.com/questions/abc"},
		},
		{
			name:        "Non-StackOverflow URL",
			url:         "https://example.com",
			expectedID:  0,
			expectedErr: &errors.ErrInvalidURL{URL: "https://example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := common.ParseStackOverflowURL(tt.url)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedErr.Error(), err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedID, id)
			}
		})
	}
}
