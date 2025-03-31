// Code generated by mockery v2.53.2. DO NOT EDIT.

package mocks

import (
	context "context"
	time "time"

	models "github.com/central-university-dev/go-Matthew11K/internal/domain/models"
	mock "github.com/stretchr/testify/mock"
)

// GitHubClient is an autogenerated mock type for the GitHubClient type
type GitHubClient struct {
	mock.Mock
}

type GitHubClient_Expecter struct {
	mock *mock.Mock
}

func (_m *GitHubClient) EXPECT() *GitHubClient_Expecter {
	return &GitHubClient_Expecter{mock: &_m.Mock}
}

// GetRepositoryDetails provides a mock function with given fields: ctx, owner, repo
func (_m *GitHubClient) GetRepositoryDetails(ctx context.Context, owner string, repo string) (*models.GitHubDetails, error) {
	ret := _m.Called(ctx, owner, repo)

	if len(ret) == 0 {
		panic("no return value specified for GetRepositoryDetails")
	}

	var r0 *models.GitHubDetails
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (*models.GitHubDetails, error)); ok {
		return rf(ctx, owner, repo)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) *models.GitHubDetails); ok {
		r0 = rf(ctx, owner, repo)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*models.GitHubDetails)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, owner, repo)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GitHubClient_GetRepositoryDetails_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetRepositoryDetails'
type GitHubClient_GetRepositoryDetails_Call struct {
	*mock.Call
}

// GetRepositoryDetails is a helper method to define mock.On call
//   - ctx context.Context
//   - owner string
//   - repo string
func (_e *GitHubClient_Expecter) GetRepositoryDetails(ctx interface{}, owner interface{}, repo interface{}) *GitHubClient_GetRepositoryDetails_Call {
	return &GitHubClient_GetRepositoryDetails_Call{Call: _e.mock.On("GetRepositoryDetails", ctx, owner, repo)}
}

func (_c *GitHubClient_GetRepositoryDetails_Call) Run(run func(ctx context.Context, owner string, repo string)) *GitHubClient_GetRepositoryDetails_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(string))
	})
	return _c
}

func (_c *GitHubClient_GetRepositoryDetails_Call) Return(_a0 *models.GitHubDetails, _a1 error) *GitHubClient_GetRepositoryDetails_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *GitHubClient_GetRepositoryDetails_Call) RunAndReturn(run func(context.Context, string, string) (*models.GitHubDetails, error)) *GitHubClient_GetRepositoryDetails_Call {
	_c.Call.Return(run)
	return _c
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

// GitHubClient_GetRepositoryLastUpdate_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetRepositoryLastUpdate'
type GitHubClient_GetRepositoryLastUpdate_Call struct {
	*mock.Call
}

// GetRepositoryLastUpdate is a helper method to define mock.On call
//   - ctx context.Context
//   - owner string
//   - repo string
func (_e *GitHubClient_Expecter) GetRepositoryLastUpdate(ctx interface{}, owner interface{}, repo interface{}) *GitHubClient_GetRepositoryLastUpdate_Call {
	return &GitHubClient_GetRepositoryLastUpdate_Call{Call: _e.mock.On("GetRepositoryLastUpdate", ctx, owner, repo)}
}

func (_c *GitHubClient_GetRepositoryLastUpdate_Call) Run(run func(ctx context.Context, owner string, repo string)) *GitHubClient_GetRepositoryLastUpdate_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(string))
	})
	return _c
}

func (_c *GitHubClient_GetRepositoryLastUpdate_Call) Return(_a0 time.Time, _a1 error) *GitHubClient_GetRepositoryLastUpdate_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *GitHubClient_GetRepositoryLastUpdate_Call) RunAndReturn(run func(context.Context, string, string) (time.Time, error)) *GitHubClient_GetRepositoryLastUpdate_Call {
	_c.Call.Return(run)
	return _c
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
