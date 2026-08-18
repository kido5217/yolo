package tui

import (
	"strings"
	"testing"
	"time"
)

// T28 toast contract: red flash block above the footer, newest line on top,
// each toast auto-clears via a tick cmd toastTTL (4s) after it is raised,
// and the queue holds at most 3 (oldest evicted). Deviation 56 pins the
// interpretation of the plan's "or on next toast" clause.

// setToastTTL shortens toastTTL for a test; returns a restore func.
func setToastTTL(d time.Duration) func() {
	old := toastTTL
	toastTTL = d
	return func() { toastTTL = old }
}

func TestToastQueueCapAndOrder(t *testing.T) {
	a := testApp()
	for _, m := range []string{"first", "second", "third", "fourth"} {
		a.toast(m)
	}
	if got := len(a.toasts); got != 3 {
		t.Fatalf("toasts = %d, want 3 (queue cap)", got)
	}
	lines := strings.Split(strings.TrimSpace(stripANSI(a.toastsView())), "\n")
	if len(lines) != 3 {
		t.Fatalf("toast lines = %d, want 3: %q", len(lines), lines)
	}
	want := []string{"fourth", "third", "second"} // newest on top
	for i, w := range want {
		if !strings.Contains(lines[i], w) {
			t.Errorf("line %d = %q, want line containing %q (newest on top)", i, lines[i], w)
		}
	}
}

func TestToastAutoClearTick(t *testing.T) {
	t.Cleanup(setToastTTL(20 * time.Millisecond))
	a := testApp()
	a.toast("boom")
	if got := len(a.toastCmds); got != 1 {
		t.Fatalf("pending toast cmds = %d, want 1 (tick armed per toast)", got)
	}
	cmd := a.drainToastCmds()
	if cmd == nil {
		t.Fatal("drainToastCmds = nil, want the armed tick")
	}
	if c := a.drainToastCmds(); c != nil {
		t.Fatalf("drain returned %v twice; the tick must arm once per toast", c)
	}
	m, ok := cmd().(toastExpireMsg)
	if !ok {
		t.Fatalf("tick msg = %T, want toastExpireMsg", cmd)
	}
	if _, c := a.Update(m); c != nil {
		t.Fatalf("expire tick returned cmd %v, want nil", c)
	}
	if len(a.toasts) != 0 {
		t.Fatalf("toasts = %d after the expire tick, want 0", len(a.toasts))
	}
}

func TestToastRendersAboveFooterInRed(t *testing.T) {
	a := testApp()
	a.toast("boom")
	raw := a.view()
	if !strings.Contains(raw, "\x1b[38;5;196m") {
		t.Fatal("toast block is not rendered in the red SGR")
	}
	lines := strings.Split(stripANSI(raw), "\n")
	for i, l := range lines {
		if strings.Contains(l, "boom") {
			if i == len(lines)-1 {
				t.Fatal("toast rendered on the last line; it must stay above the footer")
			}
			return
		}
	}
	t.Fatal("toast text missing from the view")
}
