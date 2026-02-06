package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
)

type contextKey struct{}

// Init initializes the default slog logger with a JSON handler.
// If logFile is non-empty, logs are written to that file; otherwise stdout.
// level should be "debug", "info", "warn", or "error" (defaults to "info").
func Init(logFile string, level string) error {
	return InitWithBroadcaster(logFile, level, nil)
}

// InitWithBroadcaster is like Init but also broadcasts each log record to b.
// If b is nil, behaves identically to Init.
func InitWithBroadcaster(logFile string, level string, b Broadcaster) error {
	var w io.Writer = os.Stdout

	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		w = f
	}

	var lvl slog.Level
	if level != "" {
		if err := lvl.UnmarshalText([]byte(level)); err != nil {
			lvl = slog.LevelInfo
		}
	}

	primary := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: lvl})

	var handler slog.Handler = primary
	if b != nil {
		handler = NewTeeHandler(primary, b)
	}

	slog.SetDefault(slog.New(handler))
	return nil
}

// NewContext returns a new context carrying the given logger.
func NewContext(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, l)
}

// FromContext retrieves a logger from context, falling back to the default.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(contextKey{}).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}
