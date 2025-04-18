// Code generated by mockery v2.53.2. DO NOT EDIT.

package mocks

import (
	context "context"
	time "time"

	models "github.com/central-university-dev/go-Matthew11K/internal/domain/models"
	mock "github.com/stretchr/testify/mock"
)

// LinkUpdater is an autogenerated mock type for the LinkUpdater type
type LinkUpdater struct {
	mock.Mock
}

type LinkUpdater_Expecter struct {
	mock *mock.Mock
}

func (_m *LinkUpdater) EXPECT() *LinkUpdater_Expecter {
	return &LinkUpdater_Expecter{mock: &_m.Mock}
}

// GetLastUpdate provides a mock function with given fields: ctx, url
func (_m *LinkUpdater) GetLastUpdate(ctx context.Context, url string) (time.Time, error) {
	ret := _m.Called(ctx, url)

	if len(ret) == 0 {
		panic("no return value specified for GetLastUpdate")
	}

	var r0 time.Time
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (time.Time, error)); ok {
		return rf(ctx, url)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) time.Time); ok {
		r0 = rf(ctx, url)
	} else {
		r0 = ret.Get(0).(time.Time)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, url)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// LinkUpdater_GetLastUpdate_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetLastUpdate'
type LinkUpdater_GetLastUpdate_Call struct {
	*mock.Call
}

// GetLastUpdate is a helper method to define mock.On call
//   - ctx context.Context
//   - url string
func (_e *LinkUpdater_Expecter) GetLastUpdate(ctx interface{}, url interface{}) *LinkUpdater_GetLastUpdate_Call {
	return &LinkUpdater_GetLastUpdate_Call{Call: _e.mock.On("GetLastUpdate", ctx, url)}
}

func (_c *LinkUpdater_GetLastUpdate_Call) Run(run func(ctx context.Context, url string)) *LinkUpdater_GetLastUpdate_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *LinkUpdater_GetLastUpdate_Call) Return(_a0 time.Time, _a1 error) *LinkUpdater_GetLastUpdate_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *LinkUpdater_GetLastUpdate_Call) RunAndReturn(run func(context.Context, string) (time.Time, error)) *LinkUpdater_GetLastUpdate_Call {
	_c.Call.Return(run)
	return _c
}

// GetUpdateDetails provides a mock function with given fields: ctx, url
func (_m *LinkUpdater) GetUpdateDetails(ctx context.Context, url string) (*models.UpdateInfo, error) {
	ret := _m.Called(ctx, url)

	if len(ret) == 0 {
		panic("no return value specified for GetUpdateDetails")
	}

	var r0 *models.UpdateInfo
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*models.UpdateInfo, error)); ok {
		return rf(ctx, url)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *models.UpdateInfo); ok {
		r0 = rf(ctx, url)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*models.UpdateInfo)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, url)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// LinkUpdater_GetUpdateDetails_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetUpdateDetails'
type LinkUpdater_GetUpdateDetails_Call struct {
	*mock.Call
}

// GetUpdateDetails is a helper method to define mock.On call
//   - ctx context.Context
//   - url string
func (_e *LinkUpdater_Expecter) GetUpdateDetails(ctx interface{}, url interface{}) *LinkUpdater_GetUpdateDetails_Call {
	return &LinkUpdater_GetUpdateDetails_Call{Call: _e.mock.On("GetUpdateDetails", ctx, url)}
}

func (_c *LinkUpdater_GetUpdateDetails_Call) Run(run func(ctx context.Context, url string)) *LinkUpdater_GetUpdateDetails_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *LinkUpdater_GetUpdateDetails_Call) Return(_a0 *models.UpdateInfo, _a1 error) *LinkUpdater_GetUpdateDetails_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *LinkUpdater_GetUpdateDetails_Call) RunAndReturn(run func(context.Context, string) (*models.UpdateInfo, error)) *LinkUpdater_GetUpdateDetails_Call {
	_c.Call.Return(run)
	return _c
}

// NewLinkUpdater creates a new instance of LinkUpdater. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewLinkUpdater(t interface {
	mock.TestingT
	Cleanup(func())
}) *LinkUpdater {
	mock := &LinkUpdater{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
