// Package log is yolo's leveled file logger: a small slog text handler on a
// rotating append-only file at <dataDir>/log/yolo.log (single generation,
// 5 MiB threshold → .1), with an opt-in stderr mirror (YOLO_PRINT_LOGS=1).
// Open/write failures never propagate: an unusable file no-ops and each
// write retries the open (best-effort logger). A nil *Logger is a no-op;
// Noop() is an explicit discard logger. Lines are
// "time=<RFC3339 UTC ms> level=<LEVEL> run=<8hex> msg=<msg> k=v ..."
// (spec §3 order; the stdlib TextHandler emits msg before handler attrs, so
// the pinned order is owned by pinnedHandler). Values that could forge a
// line or break key=value parsing are quoted/escaped (CWE-117). Nothing is
// ever sent anywhere (zero telemetry).
package log

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// maxSize is the active-file rotation threshold: 5 MiB.
const maxSize = 5 * 1024 * 1024

// timeMilli is the line timestamp format: RFC3339 UTC with milliseconds
// (the trailing Z is literal: records are forced to UTC before formatting).
const timeMilli = "2006-01-02T15:04:05.000Z"

// Logger is a leveled, rotating file logger. A nil *Logger is a no-op.
type Logger struct {
	mu     sync.Mutex
	w      io.Writer    // rotating file (+ optional stderr) or a test writer
	slog   *slog.Logger // pinnedHandler, level-filtered, run-id stamped
	run    string       // 8-hex process id (upstream run= parity)
	closed bool
}

// New opens (creating <dir>/log when needed) the logger for the data root.
// YOLO_LOG_LEVEL (DEBUG/INFO/WARN/ERROR, case-insensitive, invalid → INFO)
// is read once; YOLO_PRINT_LOGS=1 mirrors to stderr. Open errors are
// swallowed: the logger no-ops and retries on the next write.
func New(dir string) *Logger {
	rw := &rotatingWriter{
		path:   filepath.Join(dir, "log", "yolo.log"),
		backup: filepath.Join(dir, "log", "yolo.log") + ".1",
	}
	_ = rw.open() // eager, best-effort: the file exists even before the first write
	var out io.Writer = rw
	if os.Getenv("YOLO_PRINT_LOGS") == "1" {
		out = io.MultiWriter(rw, os.Stderr)
	}
	run := newRunID()
	return &Logger{w: out, slog: newSlog(out, parseLevel(os.Getenv("YOLO_LOG_LEVEL")), run), run: run}
}

// NewTo builds a logger writing to w (test seam: no file, no rotation).
func NewTo(w io.Writer) *Logger {
	run := newRunID()
	return &Logger{w: w, slog: newSlog(w, parseLevel(os.Getenv("YOLO_LOG_LEVEL")), run), run: run}
}

// Noop returns a discard logger (the engine's nil-Deps.Log replacement).
func Noop() *Logger {
	return &Logger{w: io.Discard, slog: slog.New(&pinnedHandler{w: io.Discard, run: newRunID()})}
}

func parseLevel(s string) slog.Level {
	switch strings.ToUpper(s) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func newRunID() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%08x", uint32(b[0])<<24|uint32(b[1])<<16|uint32(b[2])<<8|uint32(b[3]))
}

func newSlog(w io.Writer, level slog.Level, run string) *slog.Logger {
	return slog.New(&pinnedHandler{w: w, level: level, run: run})
}

// pinnedHandler renders each record as one line in the spec §3 order:
// time level run msg k=v ... Concurrent-safe; values are quoted/escaped
// when they contain characters that could forge a line (CWE-117).
type pinnedHandler struct {
	w     io.Writer
	mu    sync.Mutex
	level slog.Level
	run   string
	attrs []slog.Attr
	group string
}

func (h *pinnedHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

// WithAttrs/WithGroup copy field-wise (never *h): the handler carries a
// sync.Mutex, which vet copylocks rejects.
func (h *pinnedHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &pinnedHandler{
		w: h.w, level: h.level, run: h.run, group: h.group,
		attrs: append(append([]slog.Attr{}, h.attrs...), attrs...),
	}
}

func (h *pinnedHandler) WithGroup(name string) slog.Handler {
	group := h.group
	if group == "" {
		group = name
	} else {
		group = group + "." + name
	}
	return &pinnedHandler{
		w: h.w, level: h.level, run: h.run, group: group,
		attrs: append([]slog.Attr{}, h.attrs...),
	}
}

func (h *pinnedHandler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.Grow(128)
	b.WriteString("time=")
	b.WriteString(r.Time.UTC().Format(timeMilli))
	b.WriteString(" level=")
	b.WriteString(r.Level.String())
	b.WriteString(" run=")
	b.WriteString(h.run)
	b.WriteString(" msg=")
	writeMsg(&b, r.Message)
	var recordAttrs []slog.Attr
	r.Attrs(func(a slog.Attr) bool {
		recordAttrs = append(recordAttrs, a)
		return true
	})
	writeAttrs(&b, h.group, recordAttrs, h.attrs)
	b.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write([]byte(b.String()))
	return err
}

// writeAttrs renders k=v pairs: call-site (record) attrs first, then handler
// (With) attrs; groups nest into dotted keys.
func writeAttrs(b *strings.Builder, group string, record, handler []slog.Attr) {
	for _, list := range [][]slog.Attr{record, handler} {
		for _, a := range list {
			writeAttr(b, group, a)
		}
	}
}

func writeAttr(b *strings.Builder, group string, a slog.Attr) {
	key := a.Key
	if group != "" {
		key = group + "." + key
	}
	v := a.Value.Resolve()
	if v.Kind() == slog.KindGroup {
		for _, sub := range v.Group() {
			writeAttr(b, key, sub)
		}
		return
	}
	b.WriteByte(' ')
	b.WriteString(key)
	b.WriteByte('=')
	writeValue(b, v)
}

// writeMsg writes the message unquoted (spec §3: "msg=serving on"); embedded
// control characters are escaped so a message cannot forge a line.
func writeMsg(b *strings.Builder, s string) {
	if strings.ContainsAny(s, "\n\r\t") {
		b.WriteString(escape(s))
		return
	}
	b.WriteString(s)
}

// writeValue renders a resolved value; strings that could break the
// key=value shape (empty, spaces, quotes, control chars) are quoted with
// backslash escapes (CWE-117: embedded newlines become a literal \n).
func writeValue(b *strings.Builder, v slog.Value) {
	switch v.Kind() {
	case slog.KindString:
		writeString(b, v.String())
	case slog.KindInt64:
		b.WriteString(strconv.FormatInt(v.Int64(), 10))
	case slog.KindUint64:
		b.WriteString(strconv.FormatUint(v.Uint64(), 10))
	case slog.KindFloat64:
		b.WriteString(strconv.FormatFloat(v.Float64(), 'g', -1, 64))
	case slog.KindBool:
		b.WriteString(strconv.FormatBool(v.Bool()))
	case slog.KindDuration:
		b.WriteString(v.Duration().String())
	case slog.KindTime:
		b.WriteByte('"')
		b.WriteString(v.Time().UTC().Format(time.RFC3339Nano))
		b.WriteByte('"')
	default:
		writeString(b, fmt.Sprint(v.Any()))
	}
}

func writeString(b *strings.Builder, s string) {
	if needsQuote(s) {
		b.WriteByte('"')
		b.WriteString(escape(s))
		b.WriteByte('"')
		return
	}
	b.WriteString(s)
}

func needsQuote(s string) bool {
	if s == "" {
		return true
	}
	return strings.ContainsAny(s, " \t\n\r\"\\")
}

// escapeReplacer is built once: escape runs on every quoted value (the
// handler is on the log hot path) and NewReplacer allocates.
var escapeReplacer = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`, "\t", `\t`)

func escape(s string) string {
	return escapeReplacer.Replace(s)
}

// Debug logs at DEBUG level.
func (l *Logger) Debug(msg string, args ...any) { l.log(slog.LevelDebug, msg, args...) }

// Info logs at INFO level.
func (l *Logger) Info(msg string, args ...any) { l.log(slog.LevelInfo, msg, args...) }

// Warn logs at WARN level.
func (l *Logger) Warn(msg string, args ...any) { l.log(slog.LevelWarn, msg, args...) }

// Error logs at ERROR level.
func (l *Logger) Error(msg string, args ...any) { l.log(slog.LevelError, msg, args...) }

// Close syncs and closes the file (idempotent: a second call is a no-op).
func (l *Logger) Close() {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return
	}
	l.closed = true
	if rw, ok := l.w.(*rotatingWriter); ok {
		rw.close()
	}
}

func (l *Logger) log(level slog.Level, msg string, args ...any) {
	if l == nil {
		return
	}
	l.slog.Log(context.Background(), level, msg, args...)
}

// rotatingWriter appends to l.path, rotating to l.backup (single
// generation, overwritten) when a write would push it past maxSize.
// Unopenable → writes are dropped; every write retries the open.
type rotatingWriter struct {
	mu     sync.Mutex
	path   string
	backup string
	f      *os.File
	size   int64
}

func (rw *rotatingWriter) Write(p []byte) (int, error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	if rw.f == nil {
		_ = rw.open()
		if rw.f == nil {
			return len(p), nil
		}
	}
	if rw.size+int64(len(p)) > maxSize {
		rw.rotate()
		if rw.f == nil {
			return len(p), nil
		}
	}
	if _, err := rw.f.Write(p); err != nil {
		// Lost the file underneath us: drop it so the next write retries.
		_ = rw.f.Close()
		rw.f = nil
		return 0, err
	}
	rw.size += int64(len(p))
	return len(p), nil
}

// open (re)creates the active file and refreshes its size; best-effort.
func (rw *rotatingWriter) open() error {
	if err := os.MkdirAll(filepath.Dir(rw.path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(rw.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	rw.f = f
	rw.size = st.Size()
	return nil
}

// rotate moves the active file onto the single-generation backup and starts
// a fresh active file. Callers hold rw.mu.
func (rw *rotatingWriter) rotate() {
	_ = rw.f.Close()
	rw.f = nil
	_ = os.Rename(rw.path, rw.backup)
	_ = rw.open()
}

// close syncs and closes the active file (best-effort).
func (rw *rotatingWriter) close() {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	if rw.f == nil {
		return
	}
	_ = rw.f.Sync()
	_ = rw.f.Close()
	rw.f = nil
}
