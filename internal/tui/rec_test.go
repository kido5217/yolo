package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
)

// recApp is App with the emitted-cmd capture sink installed (the production
// App deliberately has no test hook).
type recApp struct {
	*App
	Cmds []tea.Cmd
}

func newRecApp(c *client.Service, s store.State, startSessionID string) *recApp {
	ra := &recApp{App: NewApp(c, s, startSessionID, nil)}
	ra.emitSink = func(cmds ...tea.Cmd) { ra.Cmds = append(ra.Cmds, cmds...) }
	return ra
}

// TestZeroDialogIsNotQuit pins the dlgNone sentinel (naming-5): the
// zero-initialized dialog{} must not be the quit dialog — 'y' on it must not
// arm the quit path (emit quitCmd), and the kind is popped defensively.
func TestZeroDialogIsNotQuit(t *testing.T) {
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)

	var zero dialog
	if zero.kind == dlgQuit {
		t.Fatal("zero dialog kind must not equal dlgQuit (the dlgNone sentinel)")
	}
	a.dlg.push(zero)
	if cmds := a.handleKey(press('y')); len(cmds) != 0 {
		t.Fatalf("y on the zero dialog emitted %d cmds (the quit path), want 0", len(cmds))
	}
	if !a.dlg.empty() {
		t.Fatal("the zero dialog must be popped defensively")
	}
}
