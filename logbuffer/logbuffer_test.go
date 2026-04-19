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
	if entries[0].Level != "info" {
		t.Fatalf("expected info, got %s", entries[0].Level)
	}
}

func TestWriterParsesLevelPrefix(t *testing.T) {
	buf := New(10, nil)
	w := buf.Writer("info")
	inputs := []struct {
		raw       string
		wantLvl   string
		wantMsg   string
	}{
		{"[warn] dns: forward failed\n", "warn", "dns: forward failed"},
		{"[error] config: load failed: EOF\n", "error", "config: load failed: EOF"},
		{"[debug] netcatcher/ppp0: add route 1.2.3.4\n", "debug", "netcatcher/ppp0: add route 1.2.3.4"},
		{"[info] app: ready\n", "info", "app: ready"},
		{"no prefix here\n", "info", "no prefix here"},
		{"[unknown] something\n", "info", "[unknown] something"},
	}
	for _, tc := range inputs {
		if _, err := w.Write([]byte(tc.raw)); err != nil {
			t.Fatalf("write %q: %v", tc.raw, err)
		}
	}
	entries := buf.Recent(10)
	if len(entries) != len(inputs) {
		t.Fatalf("expected %d entries, got %d", len(inputs), len(entries))
	}
	for i, tc := range inputs {
		if entries[i].Level != tc.wantLvl {
			t.Errorf("[%d] level = %q, want %q", i, entries[i].Level, tc.wantLvl)
		}
		if entries[i].Message != tc.wantMsg {
			t.Errorf("[%d] message = %q, want %q", i, entries[i].Message, tc.wantMsg)
		}
	}
}
