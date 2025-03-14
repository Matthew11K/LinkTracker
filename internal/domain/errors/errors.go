package errors

import (
	"fmt"
)

type ErrLinkAlreadyExists struct {
	URL string
}

func (e *ErrLinkAlreadyExists) Error() string {
	return "ссылка уже отслеживается: " + e.URL
}

type ErrLinkNotFound struct {
	URL string
}

func (e *ErrLinkNotFound) Error() string {
	return "ссылка не найдена: " + e.URL
}

type ErrChatNotFound struct {
	ChatID int64
}

func (e *ErrChatNotFound) Error() string {
	return "чат не найден: " + string(rune(e.ChatID))
}

type ErrInvalidURL struct {
	URL string
}

func (e *ErrInvalidURL) Error() string {
	return "неверный формат URL: " + e.URL
}

type ErrUnknownCommand struct {
	Command string
}

func (e *ErrUnknownCommand) Error() string {
	return "неизвестная команда: " + e.Command
}

type ErrUnsupportedLinkType struct {
	URL string
}

func (e *ErrUnsupportedLinkType) Error() string {
	return "неподдерживаемый тип ссылки: " + e.URL
}

type ErrInternalServer struct {
	Message string
}

func (e *ErrInternalServer) Error() string {
	return "внутренняя ошибка сервера: " + e.Message
}

type ErrInvalidArgument struct {
	Message string
}

func (e *ErrInvalidArgument) Error() string {
	return fmt.Sprintf("некорректный аргумент: %s", e.Message)
}

type ErrMissingRequiredField struct {
	FieldName string
}

func (e *ErrMissingRequiredField) Error() string {
	return fmt.Sprintf("отсутствует обязательное поле: %s", e.FieldName)
}
