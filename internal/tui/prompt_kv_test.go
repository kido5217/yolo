package tui

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// historyEngine wires a fresh engine over a shared KV path (the restart
// round-trip needs two engines on the same file).
func historyEngine(t *testing.T, kvPath string) (*recApp, *theme.Engine) {
	t.Helper()
	dir := filepath.Dir(kvPath)
	e, err := theme.New(theme.EngineOptions{
		KVPath:        kvPath,
		GlobalYoloDir: dir,
		CWD:           dir,
		Palette:       func(context.Context) (theme.TerminalColors, bool) { return theme.TerminalColors{}, false },
	})
	if err != nil {
		t.Fatalf("theme.New: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })
	if err := e.Resolve(context.Background()); err != nil {
		t.Fatalf("theme.Resolve: %v", err)
	}
	ra := &recApp{App: NewApp(client.New("http://127.0.0.1:9", ""), store.State{}, "", e)}
	return ra, e
}

func TestPromptHistoryKVPersistence(t *testing.T) {
	t.Run("append persists and reloads across a restart", func(t *testing.T) {
		dir := t.TempDir()
		kvPath := filepath.Join(dir, "kv.json")
		ra, e := historyEngine(t, kvPath)
		ra.appendHistory("one")
		ra.appendHistory("two")
		_ = e.Close()                      // final flush + the writer stops
		ra2, _ := historyEngine(t, kvPath) // a fresh engine on the SAME file
		if len(ra2.hist) != 2 || ra2.hist[0] != "one" || ra2.hist[1] != "two" {
			t.Fatalf("reload across restart: %v (want [one two])", ra2.hist)
		}
	})

	t.Run("nil engine: the history stays empty in-memory", func(t *testing.T) {
		a := testApp() // nil engine
		a.appendHistory("x")
		if len(a.hist) != 1 || a.hist[0] != "x" {
			t.Fatalf("in-memory append still works: %v", a.hist)
		}
	})
}
