package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
)

// TestStartupLoadingStateMachine pins the ported state machine (upstream
// startup-loading.tsx: the 500 ms arming, the min-3 s hold once shown, the
// ready text swap, the no-op ready-twice).
func TestStartupLoadingStateMachine(t *testing.T) {
	a := testApp()
	if a.loadShown || a.loadReady {
		t.Fatal("fresh app must start unshown + unready")
	}
	a.Update(loadShowMsg{})
	if !a.loadShown {
		t.Fatal("loadShowMsg must arm the shown state")
	}
	if a.loadReady {
		t.Fatal("loadShowMsg must not mark ready")
	}
	if got := a.loadingText(); got != startupTextLoading {
		t.Fatalf("loading text = %q, want %q", got, startupTextLoading)
	}
	if got := a.loadDone(); got == nil {
		t.Fatal("ready while shown must return the hold tick")
	}
	if !a.loadShown {
		t.Fatal("the hold must keep the line shown")
	}
	if got := a.loadingText(); got != startupTextReady {
		t.Fatalf("ready text = %q, want %q", got, startupTextReady)
	}
	a.Update(loadDoneMsg{})
	if a.loadShown {
		t.Fatal("loadDoneMsg must hide the line")
	}
	if got := a.loadDone(); got != nil {
		t.Fatal("a second ready must be a no-op (nil)")
	}
}

// TestStartupLoadingReadyBeforeShow pins the fast-hydrate path: ready
// before the 500 ms tick fired ⇒ the line never shows; a late tick after
// ready is a no-op; an expired hold hides immediately.
func TestStartupLoadingReadyBeforeShow(t *testing.T) {
	a := testApp()
	if got := a.loadDone(); got != nil {
		t.Fatal("ready before show must return nil (no hold)")
	}
	if a.loadShown {
		t.Fatal("ready before show must leave the line unshown")
	}
	a.Update(loadShowMsg{})
	if a.loadShown {
		t.Fatal("loadShowMsg after ready must be ignored")
	}

	b := testApp()
	b.Update(loadShowMsg{})
	b.loadStamp = time.Now().Add(-startupMinHold - time.Second)
	if got := b.loadDone(); got != nil {
		t.Fatal("an expired hold must hide immediately (nil)")
	}
	if b.loadShown {
		t.Fatal("the expired hold must leave the line hidden")
	}
}

// TestStartupLoadingRender pins the home-only bottom line and its slot
// (after the help line content, before the footer line).
func TestStartupLoadingRender(t *testing.T) {
	a := testApp()
	a.Update(loadShowMsg{})
	if got := stripANSI(a.loadingView(80)); !strings.Contains(got, startupTextLoading) {
		t.Fatalf("loading line = %q, want %q", got, startupTextLoading)
	}
	got := stripANSI(a.view())
	if !strings.Contains(got, startupTextLoading) {
		t.Fatalf("home view missing the loading line:\n%s", got)
	}
	i := strings.Index(got, startupTextLoading)
	if i < 0 || i < strings.Index(got, helpText) {
		t.Fatalf("the loading line must render after the help line (i=%d):\n%s", i, got)
	}
	a.route = routeSession
	if a.loadingView(80) != "" {
		t.Fatal("the session route must not show the loading line")
	}
}

// TestStartupLoadingBootTest is the teatest smoke: the real-boot home
// renders (the real boot hydrates faster than the 500 ms arm, so the
// spinner is not asserted here — absence assertions are forbidden by the
// buffer-drain rule; the state machine is unit-pinned above).
func TestStartupLoadingBootTest(t *testing.T) {
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))
	teatest.WaitFor(t, tm.Output(), hasLine("New session"), teatest.WithDuration(5*time.Second))
	tm.Send(ctrlCKey)
	tm.Send(press('y'))
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
