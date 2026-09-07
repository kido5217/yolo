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

// TestAbbrevHome pins the ported abbreviateHome (upstream runtime.tsx:3-10).
func TestAbbrevHome(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		dir  string
		home string
		want string
	}{
		{"under home", "/home/u/proj", "/home/u", "~/proj"},
		{"home itself", "/home/u", "/home/u", "~"},
		{"outside home", "/etc", "/home/u", "/etc"},
		{"sibling prefix", "/home/u2/x", "/home/u", "/home/u2/x"},
		{"dotdot unresolved", "/home/u/../etc", "/home/u", "/home/u/../etc"},
		{"empty dir", "", "/home/u", ""},
		{"empty home", "/home/u", "", "/home/u"},
		{"outside tmp", "/tmp/xyz/001", "/home/u", "/tmp/xyz/001"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := abbrevHome(c.dir, c.home); got != c.want {
				t.Fatalf("abbrevHome(%q, %q) = %q, want %q", c.dir, c.home, got, c.want)
			}
		})
	}
}

// TestSessionDestination pins the scope-dir resolution + the ""-Dir
// omission (testApp) + the homeDir seam.
func TestSessionDestination(t *testing.T) {
	t.Parallel()
	a := testApp() // Dir "" → the server work dir is unknown → omitted
	if a.sessionDestination() != "" {
		t.Fatal("an empty Dir must omit the destination")
	}
	a.Service.Dir = "/home/u/proj"
	a.homeDirFunc = func() string { return "/home/u" }
	if got := a.sessionDestination(); got != "~/proj" {
		t.Fatalf("destination = %q, want %q", got, "~/proj")
	}
	// outside the home dir → the raw path
	a.Service.Dir = "/tmp/xyz/001"
	if got := a.sessionDestination(); got != "/tmp/xyz/001" {
		t.Fatalf("outside destination = %q", got)
	}
}

// TestHomeFooterContentRowFrame pins the 0.8.0 frame footer content row
// (App.homeFooterContentRow) on the real boot: the destination prefix is the
// unpredictable TempDir path (the unit leg in homeview_test.go pins it); the
// teatest leg pins the ":branch" suffix + the dir-abbreviated prefix.
func TestHomeFooterContentRowFrame(t *testing.T) {
	ts := testutil.Boot(t)
	runGit(t, ts.Dir, "init", "-q", "-b", "yolo-wire-branch")
	c := client.New(ts.URL, ts.Dir)
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		full := stripANSI(string(b))
		return hasLine(homeLogoLine)(b) && strings.Contains(full, ":yolo-wire-branch")
	}, teatest.WithDuration(5*time.Second))
	tm.Send(ctrlCKey)
	tm.Send(press('y'))
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
