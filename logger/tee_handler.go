package logger

import (
	"context"
	"encoding/json"
	"log/slog"
)

// Broadcaster receives serialized log records for distribution.
type Broadcaster interface {
	Broadcast(msg []byte)
}

// TeeHandler is an slog.Handler that writes to a primary handler and
// broadcasts a JSON copy of each record to a Broadcaster (e.g. WebSocket hub).
type TeeHandler struct {
	primary     slog.Handler
	broadcaster Broadcaster
	attrs       []slog.Attr
	group       string
}

// NewTeeHandler wraps primary so that every log record is also broadcast.
// If broadcaster is nil the handler passes through to primary only.
func NewTeeHandler(primary slog.Handler, broadcaster Broadcaster) *TeeHandler {
	return &TeeHandler{primary: primary, broadcaster: broadcaster}
}

func (h *TeeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.primary.Enabled(ctx, level)
}

func (h *TeeHandler) Handle(ctx context.Context, r slog.Record) error {
	// Always write to the primary handler first.
	if err := h.primary.Handle(ctx, r); err != nil {
		return err
	}

	if h.broadcaster == nil {
		return nil
	}

	// Build a JSON map for the broadcast message.
	m := map[string]any{
		"time":  r.Time.Format("2006-01-02T15:04:05.000Z07:00"),
		"level": r.Level.String(),
		"msg":   r.Message,
	}

	prefix := h.group
	// Merge handler-level attrs.
	for _, a := range h.attrs {
		key := a.Key
		if prefix != "" {
			key = prefix + "." + key
		}
		m[key] = a.Value.Any()
	}
	// Merge record-level attrs.
	r.Attrs(func(a slog.Attr) bool {
		key := a.Key
		if prefix != "" {
			key = prefix + "." + key
		}
		m[key] = a.Value.Any()
		return true
	})

	data, err := json.Marshal(m)
	if err != nil {
		return nil // don't fail the log call if broadcast serialization fails
	}
	h.broadcaster.Broadcast(data)
	return nil
}

func (h *TeeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TeeHandler{
		primary:     h.primary.WithAttrs(attrs),
		broadcaster: h.broadcaster,
		attrs:       append(append([]slog.Attr{}, h.attrs...), attrs...),
		group:       h.group,
	}
}

func (h *TeeHandler) WithGroup(name string) slog.Handler {
	g := name
	if h.group != "" {
		g = h.group + "." + name
	}
	return &TeeHandler{
		primary:     h.primary.WithGroup(name),
		broadcaster: h.broadcaster,
		attrs:       append([]slog.Attr{}, h.attrs...),
		group:       g,
	}
}
