package models

type CommandType string

const (
	CommandStart   CommandType = "/start"
	CommandHelp    CommandType = "/help"
	CommandTrack   CommandType = "/track"
	CommandUntrack CommandType = "/untrack"
	CommandList    CommandType = "/list"
	CommandMode    CommandType = "/mode"
	CommandTime    CommandType = "/time"
	CommandUnknown CommandType = "unknown"
)

type Command struct {
	Type     CommandType
	ChatID   int64
	UserID   int64
	Text     string
	Username string
}
