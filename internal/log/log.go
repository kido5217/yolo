// Package log is yolo's best-effort file logger: one append-only file at
// <dataDir>/log/yolo.log, rotated to yolo.log.1 (single generation,
// overwritten) once a write would push it past 5 MiB. Open and write
// failures never propagate: an unusable logger no-ops until a later write
// retries the open.
package log

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// maxSize is the active-file rotation threshold: 5 MiB.
const maxSize = 5 * 1024 * 1024

// Logger appends "<RFC3339 UTC> <level> <text>" lines to
// <dir>/log/yolo.log. A nil *Logger is a no-op.
type Logger struct {
	mu     sync.Mutex
	path   string
	backup string
	f      *os.File
	size   int64
}

// New opens (creating <dir>/log when needed) the logger for the data root.
// Open errors are swallowed: the returned logger no-ops and retries on the
// next write.
func New(dir string) *Logger {
	l := &Logger{path: filepath.Join(dir, "log", "yolo.log")}
	l.backup = l.path + ".1"
	_ = l.open()
	return l
}

// open (re)creates the active file and refreshes its size; best-effort.
func (l *Logger) open() error {
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	l.f = f
	l.size = st.Size()
	return nil
}

func (l *Logger) write(level, text string) {
	if l == nil {
		return
	}
	line := time.Now().UTC().Format("2006-01-02T15:04:05Z") + " " + level + " " + text + "\n"
	b := []byte(line)
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		_ = l.open()
		if l.f == nil {
			return
		}
	}
	if l.size+int64(len(b)) > maxSize {
		l.rotate()
		if l.f == nil {
			return
		}
	}
	if _, err := l.f.Write(b); err != nil {
		// Lost the file underneath us: drop it so the next write retries.
		_ = l.f.Close()
		l.f = nil
		return
	}
	l.size += int64(len(b))
}

// rotate moves the active file onto the single-generation backup and starts
// a fresh active file. Callers hold l.mu.
func (l *Logger) rotate() {
	_ = l.f.Close()
	l.f = nil
	_ = os.Rename(l.path, l.backup)
	_ = l.open()
}

// Infof writes an info line.
func (l *Logger) Infof(format string, args ...any) {
	l.write("info", fmt.Sprintf(format, args...))
}

// Errorf writes an error line.
func (l *Logger) Errorf(format string, args ...any) {
	l.write("error", fmt.Sprintf(format, args...))
}

// Close syncs and closes the active file (best-effort).
func (l *Logger) Close() {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return
	}
	_ = l.f.Sync()
	_ = l.f.Close()
	l.f = nil
}
