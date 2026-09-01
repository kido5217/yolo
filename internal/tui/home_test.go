package tui

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
)

// sgrRe matches SGR escape sequences emitted by the locked styles so the
// whitebox layout test can compare plain text.
var sgrRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string { return sgrRe.ReplaceAllString(s, "") }

const testNow int64 = 1_000_000_000_000

func refModel(p, m string) *protocol.ModelRef {
	r := protocol.ModelRef{ProviderID: p, ID: m}
	return &r
}

func testApp(sessions ...protocol.Session) *recApp {
	a := newRecApp(client.New("http://127.0.0.1:9", ""), store.State{}, "")
	a.store.Sessions = sessions
	a.home.now = func() int64 { return testNow }
	return a
}

func press(r rune) tea.KeyPressMsg {
	switch r {
	case tea.KeyUp, tea.KeyDown, tea.KeyEnter, tea.KeyEscape, tea.KeyLeft, tea.KeyRight:
		return tea.KeyPressMsg{Code: r}
	}
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

var ctrlCKey = tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}

var (
	ctrlDKey = tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl}
	ctrlRKey = tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl}
)

func TestRelTime(t *testing.T) {
	tests := []struct {
		name string
		d    int64 // ms before testNow
		want string
	}{
		{"now", 0, "0s"},
		{"12s", 12_000, "12s"},
		{"59s", 59_000, "59s"},
		{"1m", 60_000, "1m"},
		{"5m", 300_000, "5m"},
		{"59m", 3_540_000, "59m"},
		{"1h", 3_600_000, "1h"},
		{"3h", 10_800_000, "3h"},
		{"23h", 82_800_000, "23h"},
		{"1d", 86_400_000, "1d"},
		{"4d", 345_600_000, "4d"},
		{"future", -5_000, "0s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := relTime(testNow-tt.d, testNow); got != tt.want {
				t.Errorf("relTime = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHomeRenderLockedLayout(t *testing.T) {
	a := testApp(
		protocol.Session{
			ID:    "ses_0",
			Title: "T1",
			Model: refModel("kido", "q"),
			Time:  protocol.SessionTime{Updated: testNow - 120_000},
		},
		protocol.Session{
			ID:    "ses_1",
			Title: "T2",
			Model: refModel("opencode", "gpt-5-nano"),
			Time:  protocol.SessionTime{Updated: testNow - 10_800_000},
		},
		protocol.Session{
			ID:    "ses_2",
			Title: "old",
			Model: refModel("kido", "q"),
			Time:  protocol.SessionTime{Updated: testNow - 345_600_000},
		},
	)
	div := strings.Repeat("─", 28)
	want := strings.Join(append(logoPlainLines(),
		"  ▸ New session",
		"  T1 · kido/q · 2m",
		"  T2 · opencode/gpt-5-nano · 3h",
		"  old · kido/q · 4d",
		div,
		"↑/↓ move · enter open · n new · /help",
	), "\n")
	got := stripANSI(a.home.render(&a.store, 80, a.theme))
	if got != want {
		t.Errorf("render mismatch:\ngot:\n%q\nwant:\n%q", got, want)
	}
}

func TestAppHandleKeyHome(t *testing.T) {
	three := func() []protocol.Session {
		return []protocol.Session{
			{ID: "ses_0", Title: "T1", Time: protocol.SessionTime{Updated: testNow}},
			{ID: "ses_1", Title: "T2", Time: protocol.SessionTime{Updated: testNow}},
			{ID: "ses_2", Title: "T3", Time: protocol.SessionTime{Updated: testNow}},
		}
	}

	t.Run("cursor wraps down and up", func(t *testing.T) {
		a := testApp(three()...)
		a.handleKey(press(tea.KeyDown))
		if a.home.cursor != 1 {
			t.Fatalf("cursor = %d after down, want 1", a.home.cursor)
		}
		a.handleKey(press(tea.KeyDown))
		a.handleKey(press(tea.KeyDown))
		if a.home.cursor != 3 {
			t.Fatalf("cursor = %d, want 3", a.home.cursor)
		}
		a.handleKey(press(tea.KeyDown)) // wraps
		if a.home.cursor != 0 {
			t.Fatalf("cursor = %d after wrap, want 0", a.home.cursor)
		}
		a.handleKey(press(tea.KeyUp)) // wraps
		if a.home.cursor != 3 {
			t.Fatalf("cursor = %d after wrap, want 3", a.home.cursor)
		}
	})

	t.Run("enter on session opens it and hydrates", func(t *testing.T) {
		a := testApp(three()...)
		a.home.cursor = 2 // T2
		a.handleKey(press(tea.KeyEnter))
		if a.route != routeSession || a.curSessionID != "ses_1" {
			t.Fatalf("route=%v cur=%s, want routeSession/ses_1", a.route, a.curSessionID)
		}
		if len(a.Cmds) != 1 {
			t.Fatalf("recorded %d cmds, want 1 hydrate cmd", len(a.Cmds))
		}
	})

	t.Run("enter on new session row creates without opening", func(t *testing.T) {
		a := testApp(three()...)
		a.handleKey(press(tea.KeyEnter)) // cursor 0
		if a.route != routeHome {
			t.Fatalf("route = %v, want routeHome (open happens on created msg)", a.route)
		}
		if len(a.Cmds) != 1 {
			t.Fatalf("recorded %d cmds, want 1 create cmd", len(a.Cmds))
		}
	})

	t.Run("n issues create cmd", func(t *testing.T) {
		a := testApp()
		a.handleKey(press('n'))
		if len(a.Cmds) != 1 {
			t.Fatalf("recorded %d cmds, want 1 create cmd", len(a.Cmds))
		}
		if a.route != routeHome {
			t.Fatalf("route = %v, want routeHome", a.route)
		}
	})

	t.Run("ctrl+c opens quit dialog, y confirms, esc cancels", func(t *testing.T) {
		a := testApp()
		a.handleKey(ctrlCKey)
		if a.dlg.empty() {
			t.Fatal("quit dialog not opened")
		}
		a.handleKey(press('y'))
		if len(a.Cmds) != 1 {
			t.Fatalf("recorded %d cmds, want 1 quit cmd", len(a.Cmds))
		}

		b := testApp()
		b.handleKey(ctrlCKey)
		b.handleKey(press(tea.KeyEscape))
		if !b.dlg.empty() {
			t.Fatal("dialog should be closed after esc")
		}
	})

	// T25 (deviation 52): the T23 auto-open command buffer is replaced by the
	// slash menu — typing "/help" opens the menu, enter executes it.
	t.Run("typing /help + enter opens help dialog", func(t *testing.T) {
		a := testApp()
		a.store.Commands = testCommands()
		for _, r := range "/help" {
			a.handleKey(press(r))
		}
		a.handleKey(press(tea.KeyEnter))
		d, ok := a.dlg.top()
		if !ok || d.kind != dlgHelp {
			t.Fatalf("dialog = %v (ok=%v), want dlgHelp", d.kind, ok)
		}
	})
}

// TestInterruptMsgOpensQuitDialog pins SIGINT handling (cli-2): a
// tea.InterruptMsg delivered during Run is treated exactly like the ctrl+c
// keystroke — it opens the quit-confirm dialog.
func TestInterruptMsgOpensQuitDialog(t *testing.T) {
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	a.Update(tea.InterruptMsg{})
	d, ok := a.dlg.top()
	if !ok || d.kind != dlgQuit {
		t.Fatalf("after InterruptMsg dialog = %+v (ok=%v), want dlgQuit on top", d, ok)
	}
}
