package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/storage"
)

// TestProfileCompletion pins yolo-k49: --profile completion candidates are
// the profile ids and (when different) names from config.List, with the
// NoFileComp directive and exit 0. The completion request is read-only: a
// fresh root yields no candidates and never creates the default profile.
func TestProfileCompletion(t *testing.T) {
	t.Run("candidates: ids + names, deduped", func(t *testing.T) {
		ch := withConfigHome(t)
		seedProfiles(t, ch,
			testProfile{id: "aaaa1111", name: "work", isActive: true},
			testProfile{id: "bbbb2222", name: "personal"},
			testProfile{id: "cccc3333", name: "cccc3333"},
		)
		code, out, errOut := runOutput(t, "__complete", "--profile", "")
		if code != 0 {
			t.Fatalf("exit = %d stderr = %q", code, errOut)
		}
		for _, want := range []string{"aaaa1111", "work", "bbbb2222", "personal", "cccc3333", ":4"} {
			if !strings.Contains(out, want) {
				t.Fatalf("completion output missing %q:\n%s", want, out)
			}
		}
		if n := strings.Count(out, "cccc3333"); n != 1 {
			t.Fatalf("name == id listed %d times, want 1:\n%s", n, out)
		}
	})
	t.Run("fresh root: no candidates, no default created", func(t *testing.T) {
		ch := withConfigHome(t)
		code, out, errOut := runOutput(t, "__complete", "--profile", "")
		if code != 0 {
			t.Fatalf("exit = %d stderr = %q", code, errOut)
		}
		if !strings.Contains(out, ":4") {
			t.Fatalf("missing the NoFileComp directive:\n%s", out)
		}
		if strings.Contains(out, "default") {
			t.Fatalf("fresh root should yield no candidates:\n%s", out)
		}
		if _, err := os.Stat(filepath.Join(ch, "yolo", "default")); !os.IsNotExist(err) {
			t.Fatalf("completion request created the default profile (must be read-only): %v", err)
		}
	})
}

// TestSessionIDCompletion pins yolo-k49: the root positional (yolo
// [sessionID]) completion candidates are the --dir store's session ids
// (most-recently updated first), and a bad --dir stays quiet (no
// candidates, exit 0, no stderr message).
func TestSessionIDCompletion(t *testing.T) {
	root := withXDG(t)
	wd, other := t.TempDir(), t.TempDir()
	db, err := openDB(filepath.Join(root, "data", "yolo", "storage", "yolo.db"))
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	now := time.Now().UnixMilli()
	ids := make([]string, 0, 3)
	for i, dir := range []string{wd, other, wd} {
		id := protocol.NewID("ses")
		if err := db.CreateSession(context.Background(), storage.SessionRow{
			ID: id, ProjectDir: dir, Title: "t", Model: "kido/q",
			TimeCreated: now, TimeUpdated: now + int64(i),
		}); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
		ids = append(ids, id)
	}

	t.Run("candidates: the --dir store's sessions only, recent first", func(t *testing.T) {
		code, out, errOut := runOutput(t, "__complete", "--dir", wd, "")
		if code != 0 {
			t.Fatalf("exit = %d stderr = %q", code, errOut)
		}
		if !strings.Contains(out, ids[0]) || !strings.Contains(out, ids[2]) {
			t.Fatalf("completion output missing the --dir store's sessions:\n%s", out)
		}
		if strings.Contains(out, ids[1]) {
			t.Fatalf("completion output includes another dir's session:\n%s", out)
		}
		if i, j := strings.Index(out, ids[2]), strings.Index(out, ids[0]); i < 0 || j < 0 || i > j {
			t.Fatalf("most-recent-first order violated (%s before %s):\n%s", ids[2], ids[0], out)
		}
		if !strings.Contains(out, ":4") {
			t.Fatalf("missing the NoFileComp directive:\n%s", out)
		}
	})
	t.Run("bad --dir: no candidates, exit 0, quiet", func(t *testing.T) {
		code, out, errOut := runOutput(t, "__complete", "--dir", "/nonexistent-yolo-test-dir", "")
		if code != 0 {
			t.Fatalf("exit = %d stderr = %q", code, errOut)
		}
		for _, id := range ids {
			if strings.Contains(out, id) {
				t.Fatalf("bad --dir yielded candidates:\n%s", out)
			}
		}
		if strings.Contains(errOut, "error") || strings.Contains(errOut, "no such") {
			t.Fatalf("bad --dir leaked an error to stderr:\n%s", errOut)
		}
	})
}
