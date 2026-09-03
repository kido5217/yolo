package tool

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/kido5217/yolo/internal/protocol"
)

// OutputDirRetention is the upstream 7-day retention for full tool outputs.
const OutputDirRetention = 7 * 24 * time.Hour

// WriteFullOutput stores one full (untruncated) tool output under dir and
// returns the file path (upstream Truncate.Service.write:
// dir/tool_<id> — the data dir's tool-output/). Empty dir returns ("",
// nil): the caller then skips the truncation marker.
func WriteFullOutput(dir, text string) (string, error) {
	if dir == "" {
		return "", nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, protocol.NewID("tool"))
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// CleanOutputDir removes tool_* outputs older than OutputDirRetention
// (upstream runs the sweep hourly; v1 runs it once at startup). A missing
// dir is a no-op (first run before any truncation). Removal failures are
// joined and returned (a stale file is not an error, but a failed remove is
// reported to the caller rather than dropped).
func CleanOutputDir(dir string) error {
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	cutoff := time.Now().Add(-OutputDirRetention)
	var errs []error
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "tool_") {
			continue
		}
		info, ierr := e.Info()
		if ierr != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			if rerr := os.Remove(filepath.Join(dir, e.Name())); rerr != nil {
				errs = append(errs, rerr)
			}
		}
	}
	return errors.Join(errs...)
}

// Truncate keeps the TAIL of text: the last up-to Limits.MaxLines lines
// within Limits.MaxBytes UTF-8 bytes. cut is true when anything was removed.
// Port of upstream shell.ts tail() (v1.18.18), including the UTF-8-boundary
// cut of a single over-long line.
func Truncate(text string, l Limits) (string, bool) {
	l = l.withDefaults()
	lines := strings.Split(text, "\n")
	if len(lines) <= l.MaxLines && len(text) <= l.MaxBytes {
		return text, false
	}
	out := make([]string, 0, l.MaxLines)
	bytes := 0
	for i := len(lines) - 1; i >= 0 && len(out) < l.MaxLines; i-- {
		size := len(lines[i])
		if len(out) > 0 {
			size++ // joining newline
		}
		if bytes+size > l.MaxBytes {
			if len(out) == 0 {
				b := []byte(lines[i])
				start := len(b) - l.MaxBytes
				if start < 0 {
					start = 0
				}
				for start < len(b) && b[start]&0xc0 == 0x80 {
					start++
				}
				out = append(out, string(b[start:]))
			}
			break
		}
		out = append(out, lines[i])
		bytes += size
	}
	slices.Reverse(out)
	return strings.Join(out, "\n"), true
}
