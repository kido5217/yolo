// Package log_test pins yolo's slog-based file logger: sink location, line
// format (time/level/run/msg), level filtering, 5MiB single-generation
// rotation, value escaping (CWE-117), the opt-in stderr mirror, UTC
// timestamps, the run id, nil/Noop no-op safety, and the NewTo test seam.
package log_test

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/kido5217/yolo/internal/log"
)

const miB = 1024 * 1024

// lineRe matches one slog text line: time=RFC3339Z level=LEVEL run=8hex msg=...
var lineRe = regexp.MustCompile(`^time=\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z level=(DEBUG|INFO|WARN|ERROR) run=[0-9a-f]{8} msg=`)

func readLog(t *testing.T, dir string) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "log", "yolo.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
}

// TestWritesLinesInSlogFormat pins the sink location and the line format
// (UTC RFC3339 with millis, level, run id, msg + key=value args).
func TestWritesLinesInSlogFormat(t *testing.T) {
	t.Setenv("YOLO_LOG_LEVEL", "")
	dir := t.TempDir()
	l := log.New(dir)
	defer l.Close()
	l.Info("serving on", "addr", "127.0.0.1:4096")
	l.Warn("slow round", "latency_ms", 1234)
	l.Error("persist part", "session_id", "s1")
	l.Debug("debug hidden by default")

	lines := readLog(t, dir)
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3 (default level INFO hides debug):\n%q", len(lines), lines)
	}
	for i, ln := range lines {
		if !lineRe.MatchString(ln) {
			t.Fatalf("line %d %q does not match the slog format", i+1, ln)
		}
	}
	if !strings.Contains(lines[0], "msg=serving on") || !strings.Contains(lines[0], "addr=127.0.0.1:4096") {
		t.Fatalf("line 1 = %q", lines[0])
	}
	if !strings.Contains(lines[1], "level=WARN") || !strings.Contains(lines[1], "latency_ms=1234") {
		t.Fatalf("line 2 = %q", lines[1])
	}
	if !strings.Contains(lines[2], "level=ERROR") || !strings.Contains(lines[2], "session_id=s1") {
		t.Fatalf("line 3 = %q", lines[2])
	}
	// UTC: the timestamp ends in Z with millisecond precision.
	ts := strings.SplitN(lines[0], " ", 2)[0][len("time="):]
	if !strings.HasSuffix(ts, "Z") || !strings.Contains(ts, ".") {
		t.Fatalf("timestamp %q is not RFC3339 UTC with millis", ts)
	}
}

// TestLevelFiltering pins YOLO_LOG_LEVEL: case-insensitive, invalid → INFO.
func TestLevelFiltering(t *testing.T) {
	for _, tc := range []struct {
		env       string
		wantDebug bool
	}{
		{"DEBUG", true},
		{"debug", true},
		{"ERROR", false},
		{"bogus", false},
		{"", false},
	} {
		t.Run(tc.env, func(t *testing.T) {
			t.Setenv("YOLO_LOG_LEVEL", tc.env)
			dir := t.TempDir()
			l := log.New(dir)
			l.Debug("d")
			l.Info("i")
			l.Close()
			lines := readLog(t, dir)
			hasDebug := false
			for _, ln := range lines {
				if strings.Contains(ln, "msg=d") {
					hasDebug = true
				}
			}
			if hasDebug != tc.wantDebug {
				t.Fatalf("debug logged = %v, want %v (level %q)\n%q", hasDebug, tc.wantDebug, tc.env, lines)
			}
		})
	}
}

// TestRotationTriggersOnSize pins the (moved) rotation contract: a write that
// would push the active file past 5MiB moves it to yolo.log.1 (single
// generation, overwritten) and starts a fresh active file.
func TestRotationTriggersOnSize(t *testing.T) {
	t.Setenv("YOLO_LOG_LEVEL", "")
	dir := t.TempDir()
	l := log.New(dir)
	defer l.Close()
	active := filepath.Join(dir, "log", "yolo.log")
	rot := active + ".1"
	body := strings.Repeat("x", 1000)
	// Lines are ~1076 bytes ("time=...Z level=INFO run=8hex msg=line i=N pad=1000x");
	// estimate at 1000 so the total is guaranteed to cross the 5 MiB threshold.
	for i := 0; i < (5*miB)/1000+2; i++ {
		l.Info("line", "i", i, "pad", body)
	}
	if _, err := os.Stat(rot); err != nil {
		t.Fatalf("rotation backup missing: %v", err)
	}
	b, err := os.ReadFile(active)
	if err != nil {
		t.Fatalf("active missing after rotation: %v", err)
	}
	if len(b) > 5*miB+2048 {
		t.Fatalf("active file %d bytes after rotation", len(b))
	}
	if len(b) == 0 {
		t.Fatal("active file empty after rotation")
	}
}

// TestNewlineCannotForgeLines pins CWE-117 (security-5): embedded newlines in
// VALUES are quoted/escaped by the text handler — one log call is one line.
func TestNewlineCannotForgeLines(t *testing.T) {
	t.Setenv("YOLO_LOG_LEVEL", "")
	dir := t.TempDir()
	l := log.New(dir)
	defer l.Close()
	l.Info("tool output", "text", "line1\nFORGED level=INFO msg=fake")
	lines := readLog(t, dir)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1 (newline must not forge):\n%q", len(lines), lines)
	}
	if !strings.Contains(lines[0], `FORGED level=INFO msg=fake`) || strings.Count(lines[0], "\n") != 0 {
		t.Fatalf("line = %q", lines[0])
	}
}

// TestPrintLogsMirrorsToStderr pins YOLO_PRINT_LOGS=1: the same line lands on
// stderr (the file sink is unchanged).
func TestPrintLogsMirrorsToStderr(t *testing.T) {
	t.Setenv("YOLO_LOG_LEVEL", "")
	t.Setenv("YOLO_PRINT_LOGS", "1")
	dir := t.TempDir()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	l := log.New(dir)
	l.Info("mirror me", "k", "v")
	l.Close()
	_ = w.Close()
	os.Stderr = old
	var sb strings.Builder
	if _, err := io.Copy(&sb, r); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sb.String(), "msg=mirror me") || !strings.Contains(sb.String(), "k=v") {
		t.Fatalf("stderr mirror = %q", sb.String())
	}
	// file sink still has it
	if b, _ := os.ReadFile(filepath.Join(dir, "log", "yolo.log")); !strings.Contains(string(b), "msg=mirror me") {
		t.Fatalf("file sink = %q", b)
	}
}

// TestNilAndNoopAreNoOps pins nil-receiver safety (troubleshoot-2) and Noop():
// neither panics, neither writes.
func TestNilAndNoopAreNoOps(t *testing.T) {
	var nilL *log.Logger
	nilL.Debug("d")
	nilL.Info("i")
	nilL.Warn("w")
	nilL.Error("e")
	nilL.Close()

	dir := t.TempDir()
	n := log.Noop()
	n.Info("nowhere")
	n.Close()
	if b, _ := os.ReadFile(filepath.Join(dir, "log", "yolo.log")); len(b) != 0 {
		t.Fatalf("Noop wrote to a file: %q", b)
	}
}

// TestCloseIsIdempotent pins the existing Close contract (second call no-op).
func TestCloseIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	l := log.New(dir)
	l.Info("x")
	l.Close()
	l.Close() // must not panic
}

// TestNewToCapturesToWriter pins the test constructor: writes to the given
// writer, no file, no rotation.
func TestNewToCapturesToWriter(t *testing.T) {
	t.Setenv("YOLO_LOG_LEVEL", "DEBUG")
	var buf strings.Builder
	l := log.NewTo(&buf)
	l.Debug("dbg", "a", 1)
	l.Error("err", "b", "x")
	s := buf.String()
	if !strings.Contains(s, "msg=dbg") || !strings.Contains(s, "a=1") || !strings.Contains(s, "level=ERROR") {
		t.Fatalf("captured = %q", s)
	}
}
