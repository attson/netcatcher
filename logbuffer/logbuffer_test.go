package logbuffer

import (
	"fmt"
	"testing"
	"time"
)

func TestWriteAndRecent(t *testing.T) {
	buf := New(5, nil)
	for i := 0; i < 3; i++ {
		buf.Add(Entry{
			Time:    time.Now(),
			Level:   "info",
			Message: fmt.Sprintf("msg %d", i),
		})
	}
	entries := buf.Recent(10)
	if len(entries) != 3 {
		t.Fatalf("expected 3, got %d", len(entries))
	}
	if entries[0].Message != "msg 0" {
		t.Fatalf("expected msg 0, got %s", entries[0].Message)
	}
}

func TestOverflow(t *testing.T) {
	buf := New(3, nil)
	for i := 0; i < 5; i++ {
		buf.Add(Entry{
			Time:    time.Now(),
			Level:   "info",
			Message: fmt.Sprintf("msg %d", i),
		})
	}
	entries := buf.Recent(10)
	if len(entries) != 3 {
		t.Fatalf("expected 3, got %d", len(entries))
	}
	if entries[0].Message != "msg 2" {
		t.Fatalf("expected msg 2 (oldest kept), got %s", entries[0].Message)
	}
	if entries[2].Message != "msg 4" {
		t.Fatalf("expected msg 4 (newest), got %s", entries[2].Message)
	}
}

func TestRecentLimit(t *testing.T) {
	buf := New(10, nil)
	for i := 0; i < 8; i++ {
		buf.Add(Entry{
			Time:    time.Now(),
			Level:   "info",
			Message: fmt.Sprintf("msg %d", i),
		})
	}
	entries := buf.Recent(3)
	if len(entries) != 3 {
		t.Fatalf("expected 3, got %d", len(entries))
	}
	if entries[0].Message != "msg 5" {
		t.Fatalf("expected msg 5, got %s", entries[0].Message)
	}
}

func TestCallback(t *testing.T) {
	var received []Entry
	cb := func(e Entry) {
		received = append(received, e)
	}
	buf := New(5, cb)
	buf.Add(Entry{Time: time.Now(), Level: "warn", Message: "test"})
	if len(received) != 1 {
		t.Fatalf("expected callback called once, got %d", len(received))
	}
	if received[0].Level != "warn" {
		t.Fatalf("expected warn, got %s", received[0].Level)
	}
}

func TestWriter(t *testing.T) {
	buf := New(10, nil)
	w := buf.Writer("info")
	_, err := w.Write([]byte("hello from writer\n"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	entries := buf.Recent(10)
	if len(entries) != 1 {
		t.Fatalf("expected 1, got %d", len(entries))
	}
	if entries[0].Message != "hello from writer" {
		t.Fatalf("expected 'hello from writer', got '%s'", entries[0].Message)
	}
}
