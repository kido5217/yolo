package tui

import (
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// commandFrecencyCommands returns two same-length commands sharing the "ze"
// prefix. For the query "ze" both get an equal fuzzy score and the prefix x2
// boost, so the command-name frecency is the only thing that can order them.
// The frequent command (/zebra1) is placed LAST in the merged list so the
// natural fuzzy-only (stable) order ranks it second — the frecency fold is
// what lifts it to first.
func commandFrecencyCommands() []protocol.Command {
	return []protocol.Command{
		{Name: "/zebra2", Description: "rare"},
		{Name: "/zebra1", Description: "frequent"},
	}
}

// menuNames extracts the command names from a menuItems() result.
func menuNames(cs []protocol.Command) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.Name)
	}
	return out
}

// seedCommandFrecency persists entries under the command_frecency KV key with
// a controlled now (the test's current wall clock as each entry's lastOpen,
// so the age seen by menuItems is ~0 and the score is the frequency), then
// reloads them into the app's in-memory cmdFreq.
func seedCommandFrecency(t *testing.T, ra *recApp, e *theme.Engine, entries []frecencyEntry) {
	t.Helper()
	now := nowMillis()
	for i := range entries {
		entries[i].LastOpen = now
	}
	e.KV().Set(kvCommandFrecencyKey, entries)
	ra.loadCommandFrecency()
}

// newCommandFrecencyApp wires a recApp over a fresh engine + KV file (the
// command-name frecency tests need the persistence surface).
func newCommandFrecencyApp(t *testing.T) (*recApp, *theme.Engine) {
	t.Helper()
	dir := t.TempDir()
	return historyEngine(t, filepath.Join(dir, "kv.json"))
}

// TestCommandFrecencyRanking pins the S4 ranking: at equal fuzzy score (the
// same query against two commands of equal match quality) the frequently used
// command ranks above the rarely used one (spec §3.3).
func TestCommandFrecencyRanking(t *testing.T) {
	ra, e := newCommandFrecencyApp(t)
	ra.store.Commands = commandFrecencyCommands()
	seedCommandFrecency(t, ra, e, []frecencyEntry{
		{Path: "/zebra1", Frequency: 10},
		{Path: "/zebra2", Frequency: 1},
	})
	ra.prompt.input.SetValue("/ze")
	got := menuNames(ra.menuItems())
	if len(got) != 2 || got[0] != "/zebra1" || got[1] != "/zebra2" {
		t.Fatalf("rank = %v, want [/zebra1 /zebra2] (the frequent above the rare)", got)
	}
}

// TestCommandFrecencyEmptyQuery pins the S4 empty-query behavior: the full
// command list is frecency-ranked (deviation 226's empty-query ordering
// extended per spec §3.3 — the order becomes the frecency rank). The
// frequent, then the rare, then the zero-frecency locals in their merged
// order.
func TestCommandFrecencyEmptyQuery(t *testing.T) {
	ra, e := newCommandFrecencyApp(t)
	ra.store.Commands = commandFrecencyCommands()
	seedCommandFrecency(t, ra, e, []frecencyEntry{
		{Path: "/zebra1", Frequency: 10},
		{Path: "/zebra2", Frequency: 1},
	})
	ra.prompt.input.SetValue("/")
	want := []string{"/zebra1", "/zebra2", "/sessions", "/connect", "/status", "/themes"}
	got := menuNames(ra.menuItems())
	if len(got) != len(want) {
		t.Fatalf("empty query = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("empty query = %v, want %v", got, want)
		}
	}
}

// TestCommandFrecencyTouch pins the S4 touch seam: executing a command via the
// slash menu (the enter path) bumps its frequency + sets lastOpen + persists
// to the command_frecency KV key; the which-key palette path runs the same
// command WITHOUT touching the store (the touch seam is the slash-menu
// execution only, spec §3.3).
func TestCommandFrecencyTouch(t *testing.T) {
	t.Run("enter via the slash menu bumps, lastOpen, and persists", func(t *testing.T) {
		ra, e := newCommandFrecencyApp(t)
		ra.store.Commands = testCommands()
		typeStr(ra, "/quit")
		ra.handleKey(press(tea.KeyEnter))

		if len(ra.cmdFreq) != 1 {
			t.Fatalf("cmdFreq = %v, want one /quit entry", ra.cmdFreq)
		}
		he := ra.cmdFreq[0]
		if he.Path != "/quit" || he.Frequency != 1 || he.LastOpen == 0 {
			t.Fatalf("cmdFreq[0] = %+v, want /quit freq=1 lastOpen!=0", he)
		}
		raw := e.KV().Get(kvCommandFrecencyKey, nil)
		entries, ok := raw.([]frecencyEntry)
		if !ok || len(entries) != 1 || entries[0].Path != "/quit" || entries[0].Frequency != 1 {
			t.Fatalf("KV[%s] = %#v, want one /quit entry freq=1", kvCommandFrecencyKey, raw)
		}
	})

	t.Run("the which-key palette path does not touch the store", func(t *testing.T) {
		ra, e := newCommandFrecencyApp(t)
		ra.store.Commands = testCommands()
		ra.paletteSelectPick(selectOption{value: "/quit"})

		if len(ra.cmdFreq) != 0 {
			t.Fatalf("palette path must not write the in-memory store, got %v", ra.cmdFreq)
		}
		if v := e.KV().Get(kvCommandFrecencyKey, nil); v != nil {
			t.Fatalf("palette path must not persist the key, got %#v", v)
		}
	})
}
