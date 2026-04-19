package logbuffer

import (
	"strings"
	"sync"
	"time"
)

type Entry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

type OnNewEntry func(Entry)

type Buffer struct {
	mu       sync.Mutex
	entries  []Entry
	capacity int
	head     int
	count    int
	onNew    OnNewEntry
}

func New(capacity int, onNew OnNewEntry) *Buffer {
	return &Buffer{
		entries:  make([]Entry, capacity),
		capacity: capacity,
		onNew:    onNew,
	}
}

func (b *Buffer) Add(e Entry) {
	b.mu.Lock()
	b.entries[b.head] = e
	b.head = (b.head + 1) % b.capacity
	if b.count < b.capacity {
		b.count++
	}
	b.mu.Unlock()

	if b.onNew != nil {
		b.onNew(e)
	}
}

func (b *Buffer) Recent(n int) []Entry {
	b.mu.Lock()
	defer b.mu.Unlock()

	if n > b.count {
		n = b.count
	}
	result := make([]Entry, n)
	start := (b.head - n + b.capacity) % b.capacity
	for i := 0; i < n; i++ {
		result[i] = b.entries[(start+i)%b.capacity]
	}
	return result
}

type writer struct {
	buf          *Buffer
	defaultLevel string
}

// Writer returns an io.Writer compatible with log.SetOutput. Messages that
// begin with "[<level>] " (one of debug/info/warn/error) are classified with
// that level and the prefix is stripped; otherwise the default level is used.
// Pass "" to fall back to "info".
func (b *Buffer) Writer(defaultLevel string) *writer {
	if defaultLevel == "" {
		defaultLevel = "info"
	}
	return &writer{buf: b, defaultLevel: defaultLevel}
}

func (w *writer) Write(p []byte) (n int, err error) {
	msg := strings.TrimRight(string(p), "\n")
	if msg == "" {
		return len(p), nil
	}
	level, rest := parseLevel(msg, w.defaultLevel)
	w.buf.Add(Entry{
		Time:    time.Now(),
		Level:   level,
		Message: rest,
	})
	return len(p), nil
}

// parseLevel extracts a `[level]` tag from the message and returns the
// classified level plus the remaining message. Unrecognized tags or missing
// prefixes fall back to def and leave the message untouched.
func parseLevel(s, def string) (level, rest string) {
	if len(s) < 3 || s[0] != '[' {
		return def, s
	}
	end := strings.IndexByte(s, ']')
	if end < 0 {
		return def, s
	}
	tag := s[1:end]
	switch tag {
	case "debug", "info", "warn", "error":
	default:
		return def, s
	}
	// Strip the "[level] " prefix (including the single space separator if present).
	rest = s[end+1:]
	if len(rest) > 0 && rest[0] == ' ' {
		rest = rest[1:]
	}
	return tag, rest
}
