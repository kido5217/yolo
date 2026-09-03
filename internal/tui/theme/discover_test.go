package theme

import (
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func writeThemeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	full := filepath.Join(dir, "themes", name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func primaryOf(v any) string {
	obj, _ := v.(map[string]any)
	th, _ := obj["theme"].(map[string]any)
	p, _ := th["primary"].(string)
	return p
}

// TestThemeDirs: the exact upstream order (theme.tsx:38-44) — the injected
// global dir first, then <dir>/.yolo for cwd walking up to and including
// the filesystem root (the /.yolo entry); no dedupe. ThemeDirs is a pure
// string walk (no FS access), so a fixed absolute path is fully hermetic.
func TestThemeDirs(t *testing.T) {
	t.Parallel()
	got := ThemeDirs("/home/u/.config/yolo", "/home/u/proj/pkg")
	want := []string{
		"/home/u/.config/yolo",
		"/home/u/proj/pkg/.yolo",
		"/home/u/proj/.yolo",
		"/home/u/.yolo",
		"/home/.yolo",
		"/.yolo",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ThemeDirs = %v, want %v", got, want)
	}
	if got := ThemeDirs("/g", "/"); !reflect.DeepEqual(got, []string{"/g", "/.yolo"}) {
		t.Fatalf("ThemeDirs at the filesystem root = %v, want [/g /.yolo]", got)
	}
}

// TestDiscover: names = stems; later dirs override earlier (upstream
// object-assignment order — the outer project dir beats the inner one,
// which beats the global dir); dotfile names included (upstream dot:true);
// symlinked theme files followed (upstream symlink:true); non-theme JSON
// returned raw (the IsTheme filter is the caller's job — theme.tsx:137-140);
// non-.json files ignored; missing themes dirs skipped without error.
func TestDiscover(t *testing.T) {
	t.Parallel()
	global := t.TempDir()
	base := t.TempDir()
	mid := filepath.Join(base, "mid")
	cwd := filepath.Join(mid, "src")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	writeThemeFile(t, global, "shared.json", `{"theme":{"primary":"#111111"}}`)
	writeThemeFile(t, global, "custom.json", `{"theme":{"primary":"#ffffff"}}`)
	writeThemeFile(t, filepath.Join(base, ".yolo"), "shared.json", `{"theme":{"primary":"#222222"}}`)
	writeThemeFile(t, filepath.Join(mid, ".yolo"), "shared.json", `{"theme":{"primary":"#333333"}}`)
	writeThemeFile(t, filepath.Join(mid, ".yolo"), ".hidden.json", `{"theme":{"primary":"#444444"}}`)
	writeThemeFile(t, filepath.Join(mid, ".yolo"), "notatoken.json", `{"defs":{}}`)
	writeThemeFile(t, filepath.Join(mid, ".yolo"), "notes.txt", "not json")
	link := filepath.Join(mid, ".yolo", "themes", "linked.json")
	if err := os.Symlink(filepath.Join(global, "themes", "custom.json"), link); err != nil {
		t.Fatal(err)
	}

	got, err := Discover(ThemeDirs(global, cwd))
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("entries = %v, want 5 (shared, custom, .hidden, linked, notatoken)", got)
	}
	if p := primaryOf(got["shared"]); p != "#222222" {
		t.Errorf("shared = %q, want #222222 (outer project dir overrides inner + global)", p)
	}
	if p := primaryOf(got["custom"]); p != "#ffffff" {
		t.Errorf("custom = %q, want #ffffff (global dir)", p)
	}
	if p := primaryOf(got[".hidden"]); p != "#444444" {
		t.Errorf(".hidden = %q, want #444444 (dotfile name included)", p)
	}
	if p := primaryOf(got["linked"]); p != "#ffffff" {
		t.Errorf("linked = %q, want #ffffff (symlinked theme file followed)", p)
	}
	if IsTheme(got["notatoken"]) {
		t.Error(`notatoken (no "theme" key) must be returned raw — IsTheme filtering is caller-side`)
	}
	if _, ok := got["notes"]; ok {
		t.Error("non-.json entry must not be scanned")
	}
}

// TestDiscoverCorruptFileIsHardError: an unparseable .json fails the whole
// discover (upstream JSON.parse throws; the caller's catch sets active to
// "yolo" — S0.7). Never a per-file skip.
func TestDiscoverCorruptFileIsHardError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeThemeFile(t, dir, "good.json", `{"theme":{"primary":"#ffffff"}}`)
	writeThemeFile(t, dir, "bad.json", `{not json`)
	if _, err := Discover([]string{dir}); err == nil {
		t.Fatal("corrupt .json must be a hard error")
	}
}

// TestDiscoverNoThemesDirs: a missing <dir>/themes is skipped (upstream
// Glob.scan yields nothing) — the common case on a clean machine.
func TestDiscoverNoThemesDirs(t *testing.T) {
	t.Parallel()
	got, err := Discover([]string{t.TempDir(), t.TempDir()})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("entries = %v, want none", got)
	}
}

// TestWatchThemeSignals: SIGUSR2 forwards to refresh; after stop() no
// further forwarding. stop() restores the default SIGUSR2 disposition
// (signal.Reset — which would terminate the process), so the post-stop kill
// is guarded with signal.Ignore.
func TestWatchThemeSignals(t *testing.T) {
	var n atomic.Int32
	stop := WatchThemeSignals(func() { n.Add(1) })

	if err := syscall.Kill(os.Getpid(), syscall.SIGUSR2); err != nil {
		t.Fatalf("kill: %v", err)
	}
	for i := 0; i < 200 && n.Load() == 0; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if got := n.Load(); got != 1 {
		t.Fatalf("refresh count = %d, want 1 (SIGUSR2 must forward to refresh)", got)
	}
	stop()
	signal.Ignore(syscall.SIGUSR2)
	if err := syscall.Kill(os.Getpid(), syscall.SIGUSR2); err != nil {
		t.Fatalf("kill: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if got := n.Load(); got != 1 {
		t.Errorf("refresh count after stop = %d, want 1 (no forwarding after stop)", got)
	}
}
