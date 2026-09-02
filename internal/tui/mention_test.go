package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
)

func TestMentionTriggerIndex(t *testing.T) {
	tests := []struct {
		in   string
		want int
		ok   bool
	}{
		{"", -1, false},
		{"hello", -1, false},
		{"@", 0, true},
		{"@f", 0, true},
		{"fix @f", 4, true},
		{"fix@f", -1, false},
		{"fix @f el", -1, false},
	}
	for _, tc := range tests {
		idx, ok := mentionTriggerIndex(tc.in)
		if idx != tc.want || ok != tc.ok {
			t.Fatalf("mentionTriggerIndex(%q) = (%d,%v), want (%d,%v)", tc.in, idx, ok, tc.want, tc.ok)
		}
	}
}

func TestWalkFiles(t *testing.T) {
	dir := t.TempDir()
	mk := func(rel string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte("x"), 0o644)
	}
	mk("alpha.go")
	mk("src/gamma.go")
	mk("node_modules/dep.js")
	mk(".git/config")
	got := walkFiles(dir)
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "alpha.go") || !strings.Contains(joined, "src/gamma.go") {
		t.Fatalf("walk missed fixture files:\n%s", joined)
	}
	if strings.Contains(joined, "node_modules") || strings.Contains(joined, ".git") {
		t.Fatalf("walk must skip the static ignore set:\n%s", joined)
	}
}

func TestMentionOptions(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"alpha.go", "beta.go", "alpha_beta.go"} {
		os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644)
	}
	a := testApp()
	a.Service.Dir = dir
	a.prompt.input.SetValue("@al")
	opts := a.mentionOptions()
	if len(opts) == 0 {
		t.Fatal("no options for @al")
	}
	// the prefix match (alpha.go) ranks first (the x2 prefix boost)
	if opts[0].value.(string) != "alpha.go" {
		t.Fatalf("top option = %v, want alpha.go (the prefix match)", opts[0].value)
	}
}

func TestAcInsert(t *testing.T) {
	a := testApp()
	a.Service.Dir = t.TempDir()
	a.prompt.input.SetValue("see @fil")
	a.acInsert("alpha.go")
	if got := a.prompt.input.Value(); got != "see alpha.go" {
		t.Fatalf("insert = %q, want the path replacing the @-query", got)
	}
	if len(a.freq) != 1 || a.freq[0].Path != "alpha.go" || a.freq[0].Frequency != 1 {
		t.Fatalf("frecency not recorded: %v", a.freq)
	}
}

// TestTUIAtPicker is the teatest leg: a real stack, the @-picker filters the
// walked files and enter inserts the selected path.
func TestTUIAtPicker(t *testing.T) {
	ts := testutil.Boot(t)
	os.WriteFile(filepath.Join(ts.Dir, "alpha.go"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(ts.Dir, "beta.go"), []byte("x"), 0o644)
	c := client.New(ts.URL, ts.Dir) // scope (and the walk) to the server work dir
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))
	teatest.WaitFor(t, tm.Output(), hasLine("New session"), teatest.WithDuration(5*time.Second))
	tm.Send(press('n'))
	teatest.WaitFor(t, tm.Output(), hasLine("esc abort/back"), teatest.WithDuration(5*time.Second))
	suiteType(tm, "see @a")
	teatest.WaitFor(t, tm.Output(), hasLine("alpha.go"), teatest.WithDuration(5*time.Second))
	tm.Send(press(tea.KeyEnter))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return strings.Contains(a.prompt.input.Value(), "alpha.go")
	}, teatest.WithDuration(5*time.Second))
	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
