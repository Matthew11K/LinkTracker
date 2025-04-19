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

func (e *ErrLinkAlreadyExists) Is(target error) bool {
	_, ok := target.(*ErrLinkAlreadyExists)
	return ok
}

type ErrLinkNotFound struct {
	URL string
}

func (e *ErrLinkNotFound) Error() string {
	return "ссылка не найдена: " + e.URL
}

func (e *ErrLinkNotFound) Is(target error) bool {
	_, ok := target.(*ErrLinkNotFound)
	return ok
}

type ErrChatNotFound struct {
	ChatID int64
}

func (e *ErrChatNotFound) Error() string {
	return fmt.Sprintf("чат не найден: %d", e.ChatID)
}

type ErrChatAlreadyExists struct {
	ChatID int64
}

func (e *ErrChatAlreadyExists) Error() string {
	return fmt.Sprintf("чат с ID %d уже существует", e.ChatID)
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

type ErrDetailsNotFound struct {
	LinkID int64
}

func (e *ErrDetailsNotFound) Error() string {
	return fmt.Sprintf("детали для ссылки с ID %d не найдены", e.LinkID)
}

type ErrUnknownDBAccessType struct {
	AccessType string
}

func (e *ErrUnknownDBAccessType) Error() string {
	return fmt.Sprintf("неизвестный тип доступа к базе данных: %s", e.AccessType)
}

type ErrBeginTransaction struct {
	Cause error
}

func (e *ErrBeginTransaction) Error() string {
	return fmt.Sprintf("ошибка при начале транзакции: %v", e.Cause)
}

func (e *ErrBeginTransaction) Unwrap() error {
	return e.Cause
}

type ErrBuildSQLQuery struct {
	Operation string
	Cause     error
}

func (e *ErrBuildSQLQuery) Error() string {
	return fmt.Sprintf("ошибка при построении SQL запроса для %s: %v", e.Operation, e.Cause)
}

func (e *ErrBuildSQLQuery) Unwrap() error {
	return e.Cause
}

type ErrSQLExecution struct {
	Operation string
	Cause     error
}

func (e *ErrSQLExecution) Error() string {
	return fmt.Sprintf("ошибка при выполнении SQL запроса для %s: %v", e.Operation, e.Cause)
}

func (e *ErrSQLExecution) Unwrap() error {
	return e.Cause
}

type ErrSQLScan struct {
	Entity string
	Cause  error
}

func (e *ErrSQLScan) Error() string {
	return fmt.Sprintf("ошибка при сканировании %s: %v", e.Entity, e.Cause)
}

func (e *ErrSQLScan) Unwrap() error {
	return e.Cause
}

type ErrCommitTransaction struct {
	Cause error
}

func (e *ErrCommitTransaction) Error() string {
	return fmt.Sprintf("ошибка при фиксации транзакции: %v", e.Cause)
}

func (e *ErrCommitTransaction) Unwrap() error {
	return e.Cause
}

type ErrTagAlreadyExists struct {
	Tag string
	URL string
}

func (e *ErrTagAlreadyExists) Error() string {
	return fmt.Sprintf("тег '%s' уже добавлен к ссылке '%s'", e.Tag, e.URL)
}

type ErrTagNotFound struct {
	Tag string
	URL string
}

func (e *ErrTagNotFound) Error() string {
	return fmt.Sprintf("тег '%s' не найден для ссылки '%s'", e.Tag, e.URL)
}

type ErrLinkNotInChat struct {
	ChatID int64
	LinkID int64
}

func (e *ErrLinkNotInChat) Error() string {
	return fmt.Sprintf("ссылка c ID %d не найдена в чате c ID %d", e.LinkID, e.ChatID)
}

type ErrChatStateNotFound struct {
	ChatID int64
}

func (e *ErrChatStateNotFound) Error() string {
	return fmt.Sprintf("состояние чата не найдено: %d", e.ChatID)
}

func (e *ErrChatStateNotFound) Is(target error) bool {
	_, ok := target.(*ErrChatStateNotFound)
	return ok
}

const (
	OpGetChatState       = "get_chat_state"
	OpSetChatState       = "set_chat_state"
	OpGetChatStateData   = "get_chat_state_data"
	OpSetChatStateData   = "set_chat_state_data"
	OpClearChatStateData = "clear_chat_state_data"
)
