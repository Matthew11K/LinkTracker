// Code generated by mockery v2.53.2. DO NOT EDIT.

package mocks

import (
	context "context"

	domain "github.com/central-university-dev/go-Matthew11K/internal/bot/domain"
	mock "github.com/stretchr/testify/mock"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// TelegramClientAPI is an autogenerated mock type for the TelegramClientAPI type
type TelegramClientAPI struct {
	mock.Mock
}

// GetBot provides a mock function with no fields
func (_m *TelegramClientAPI) GetBot() *tgbotapi.BotAPI {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetBot")
	}

	var r0 *tgbotapi.BotAPI
	if rf, ok := ret.Get(0).(func() *tgbotapi.BotAPI); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*tgbotapi.BotAPI)
		}
	}

	return r0
}

// GetUpdates provides a mock function with given fields: ctx, offset
func (_m *TelegramClientAPI) GetUpdates(ctx context.Context, offset int) ([]domain.Update, error) {
	ret := _m.Called(ctx, offset)

	if len(ret) == 0 {
		panic("no return value specified for GetUpdates")
	}

	var r0 []domain.Update
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, int) ([]domain.Update, error)); ok {
		return rf(ctx, offset)
	}
	if rf, ok := ret.Get(0).(func(context.Context, int) []domain.Update); ok {
		r0 = rf(ctx, offset)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]domain.Update)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, int) error); ok {
		r1 = rf(ctx, offset)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// SendMessage provides a mock function with given fields: ctx, chatID, text
func (_m *TelegramClientAPI) SendMessage(ctx context.Context, chatID int64, text string) error {
	ret := _m.Called(ctx, chatID, text)

	if len(ret) == 0 {
		panic("no return value specified for SendMessage")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, int64, string) error); ok {
		r0 = rf(ctx, chatID, text)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SendUpdate provides a mock function with given fields: ctx, update
func (_m *TelegramClientAPI) SendUpdate(ctx context.Context, update interface{}) error {
	ret := _m.Called(ctx, update)

	if len(ret) == 0 {
		panic("no return value specified for SendUpdate")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, interface{}) error); ok {
		r0 = rf(ctx, update)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SetMyCommands provides a mock function with given fields: ctx, commands
func (_m *TelegramClientAPI) SetMyCommands(ctx context.Context, commands []domain.BotCommand) error {
	ret := _m.Called(ctx, commands)

	if len(ret) == 0 {
		panic("no return value specified for SetMyCommands")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, []domain.BotCommand) error); ok {
		r0 = rf(ctx, commands)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewTelegramClientAPI creates a new instance of TelegramClientAPI. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewTelegramClientAPI(t interface {
	mock.TestingT
	Cleanup(func())
}) *TelegramClientAPI {
	mock := &TelegramClientAPI{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
