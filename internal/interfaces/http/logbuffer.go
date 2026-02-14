package http

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// LogEntry represents a single log entry.
type LogEntry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
	Attrs   map[string]any `json:"attrs,omitempty"`
}

// LogBuffer is a thread-safe ring buffer that captures slog log entries.
type LogBuffer struct {
	mu      sync.RWMutex
	entries []LogEntry
	size    int
	pos     int
	count   int
}

// NewLogBuffer creates a new log buffer with the given capacity.
func NewLogBuffer(size int) *LogBuffer {
	return &LogBuffer{
		entries: make([]LogEntry, size),
		size:    size,
	}
}

// Add adds a log entry to the buffer.
func (b *LogBuffer) Add(entry LogEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.entries[b.pos] = entry
	b.pos = (b.pos + 1) % b.size
	if b.count < b.size {
		b.count++
	}
}

// Entries returns all buffered log entries in chronological order.
func (b *LogBuffer) Entries() []LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.count == 0 {
		return []LogEntry{}
	}

	result := make([]LogEntry, b.count)
	if b.count < b.size {
		copy(result, b.entries[:b.count])
	} else {
		// Ring buffer is full, read from pos to end, then start to pos
		n := copy(result, b.entries[b.pos:])
		copy(result[n:], b.entries[:b.pos])
	}
	return result
}

// LogBufferHandler is an slog.Handler that captures logs into a LogBuffer.
type LogBufferHandler struct {
	buffer *LogBuffer
	inner  slog.Handler
	attrs  []slog.Attr
	groups []string
}

// NewLogBufferHandler creates an slog handler that writes to both the inner handler and the buffer.
func NewLogBufferHandler(inner slog.Handler, buffer *LogBuffer) *LogBufferHandler {
	return &LogBufferHandler{
		buffer: buffer,
		inner:  inner,
	}
}

// Enabled reports whether the handler handles records at the given level.
func (h *LogBufferHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle handles the Record by writing to both the inner handler and the buffer.
func (h *LogBufferHandler) Handle(ctx context.Context, r slog.Record) error {
	// Collect attributes
	attrs := make(map[string]any)
	for _, a := range h.attrs {
		attrs[a.Key] = a.Value.Any()
	}
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})

	entry := LogEntry{
		Time:    r.Time,
		Level:   r.Level.String(),
		Message: r.Message,
	}
	if len(attrs) > 0 {
		entry.Attrs = attrs
	}

	h.buffer.Add(entry)

	return h.inner.Handle(ctx, r)
}

// WithAttrs returns a new handler with the given attributes.
func (h *LogBufferHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &LogBufferHandler{
		buffer: h.buffer,
		inner:  h.inner.WithAttrs(attrs),
		attrs:  append(h.attrs, attrs...),
		groups: h.groups,
	}
}

// WithGroup returns a new handler with the given group name.
func (h *LogBufferHandler) WithGroup(name string) slog.Handler {
	return &LogBufferHandler{
		buffer: h.buffer,
		inner:  h.inner.WithGroup(name),
		attrs:  h.attrs,
		groups: append(h.groups, name),
	}
}
