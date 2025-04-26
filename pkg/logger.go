package pkg

import (
	"io"
	"log/slog"
)

func NewLogger(w io.Writer) *slog.Logger {
	handler := slog.NewJSONHandler(w, nil)
	return slog.New(handler)
}
