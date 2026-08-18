// Package log_test pins yolo's file logger: the sink location, the line
// format, no-op safety, and the 5MiB single-generation rotation.
package log_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kido5217/yolo/internal/log"
)

const (
	miB     = 1024 * 1024
	lineLen = 1024 // Infof line: 20 (RFC3339Z) + 1 + 4 ("info") + 1 + 997 + 1
	bodyLen = 997
)

// TestWritesToDataDirLog pins the sink location (<dir>/log/yolo.log) and the
// "RFC3339-UTC <level> <text>" line format for both levels.
func TestWritesToDataDirLog(t *testing.T) {
	dir := t.TempDir()
	l := log.New(dir)
	defer l.Close()
	l.Infof("hello %d", 1)
	l.Errorf("boom %s", "x")

	b, err := os.ReadFile(filepath.Join(dir, "log", "yolo.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2: %q", len(lines), b)
	}
	for i, wantSfx := range []string{" info hello 1", " error boom x"} {
		ln := lines[i]
		if !strings.HasSuffix(ln, wantSfx) {
			t.Fatalf("line %d = %q, want suffix %q", i+1, ln, wantSfx)
		}
		ts := ln[:strings.IndexByte(ln, ' ')]
		if len(ts) != 20 || ts[4] != '-' || ts[7] != '-' || ts[10] != 'T' ||
			ts[13] != ':' || ts[16] != ':' || ts[19] != 'Z' {
			t.Fatalf("line %d timestamp %q, want RFC3339 UTC", i+1, ts)
		}
	}
}

// TestRotationTriggersOnSize pins the rotation contract: when a write would
// push the active file past 5MiB it moves to yolo.log.1 (single generation,
// overwriting the previous one) and a fresh active file starts; the tail of
// the stream lands in the active file.
func TestRotationTriggersOnSize(t *testing.T) {
	dir := t.TempDir()
	l := log.New(dir)
	defer l.Close()
	active := filepath.Join(dir, "log", "yolo.log")
	rot := active + ".1"

	msg := strings.Repeat("x", bodyLen)
	uniq := strings.Repeat("x", bodyLen-1) + "a" // only in generation 1

	// Two full generations of 5120 lines + 760 more.
	const total = 11000
	for i := range total {
		line := msg
		if i == 0 {
			line = uniq
		}
		l.Infof("%s", line)
	}

	st, err := os.Stat(rot)
	if err != nil {
		t.Fatalf("yolo.log.1 missing: %v", err)
	}
	if st.Size() != 5120*lineLen {
		t.Fatalf("yolo.log.1 size = %d, want %d", st.Size(), 5120*lineLen)
	}
	b1, err := os.ReadFile(rot)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b1), "a") {
		t.Fatal("yolo.log.1 still holds the first generation; rotation must overwrite")
	}

	st, err = os.Stat(active)
	if err != nil {
		t.Fatal(err)
	}
	if st.Size() != 760*lineLen {
		t.Fatalf("active size = %d, want %d", st.Size(), 760*lineLen)
	}
	b, err := os.ReadFile(active)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), msg) {
		t.Fatal("active file is missing the tail lines")
	}
}

// A nil or not-yet-opened logger must be a safe no-op so wiring can hold a
// zero value without guards.
func TestNilLoggerNoOp(t *testing.T) {
	var l *log.Logger
	l.Infof("x")
	l.Errorf("y %d", 1)
}
