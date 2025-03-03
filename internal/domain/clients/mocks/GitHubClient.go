// Code generated by mockery v2.53.0. DO NOT EDIT.

package mocks

import (
	context "context"
	time "time"

	mock "github.com/stretchr/testify/mock"
)

// GitHubClient is an autogenerated mock type for the GitHubClient type
type GitHubClient struct {
	mock.Mock
}

// GetRepositoryLastUpdate provides a mock function with given fields: ctx, owner, repo
func (_m *GitHubClient) GetRepositoryLastUpdate(ctx context.Context, owner string, repo string) (time.Time, error) {
	ret := _m.Called(ctx, owner, repo)

	if len(ret) == 0 {
		panic("no return value specified for GetRepositoryLastUpdate")
	}

	var r0 time.Time
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (time.Time, error)); ok {
		return rf(ctx, owner, repo)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) time.Time); ok {
		r0 = rf(ctx, owner, repo)
	} else {
		r0 = ret.Get(0).(time.Time)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, owner, repo)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewGitHubClient creates a new instance of GitHubClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewGitHubClient(t interface {
	mock.TestingT
	Cleanup(func())
}) *GitHubClient {
	mock := &GitHubClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
