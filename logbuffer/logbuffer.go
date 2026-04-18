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
	buf   *Buffer
	level string
}

func (b *Buffer) Writer(level string) *writer {
	return &writer{buf: b, level: level}
}

func (w *writer) Write(p []byte) (n int, err error) {
	msg := strings.TrimRight(string(p), "\n")
	if msg == "" {
		return len(p), nil
	}
	w.buf.Add(Entry{
		Time:    time.Now(),
		Level:   w.level,
		Message: msg,
	})
	return len(p), nil
}
