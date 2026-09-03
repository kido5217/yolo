package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/kido5217/yolo/internal/llm/fake"
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

// TestHomeFooterLine pins the seam body shape (the S6.5 parts join: the
// destination part + the hint part, " · "-joined; the dimmed single line,
// omitted only when both parts are empty, and its render slot (after the
// help line + tips line, before the footer)).
func TestHomeFooterLine(t *testing.T) {
	t.Parallel()
	a := testApp()
	// destination omitted (Dir "") → the hint-only line
	if got := stripANSI(a.homeFooterLine(80)); got != "Show keyboard shortcuts with ctrl+x" {
		t.Fatalf("hint-only line = %q", got)
	}
	a.Service.Dir = "/home/u/proj"
	a.homeDirFunc = func() string { return "/home/u" }
	if got := stripANSI(a.homeFooterLine(80)); got != "~/proj · Show keyboard shortcuts with ctrl+x" {
		t.Fatalf("footer line = %q", got)
	}
	// render slot: home view carries the line after the help line
	out := stripANSI(a.view())
	h := strings.Index(out, helpText)
	d := strings.Index(out, "~/proj")
	if h < 0 || d < 0 || d < h {
		t.Fatalf("destination line must follow the help line (h=%d d=%d):\n%s", h, d, out)
	}
	// a wrapped destination line still renders (the dimWrapped at w)
	a.Service.Dir = "/home/u/this/is/a/rather/long/destination/path/for/wrapping"
	if got := a.homeFooterLine(20); got == "" {
		t.Fatal("a long destination must render (wrapped)")
	}
}

// TestHomeShortcutsHint pins the registry-rendered hint: the default
// leader (ctrl+x), the remap-sensitivity, and the leader-disabled
// omission.
func TestHomeShortcutsHint(t *testing.T) {
	t.Parallel()
	a := testApp()
	if got := a.homeShortcutsHint(); got != "Show keyboard shortcuts with ctrl+x" {
		t.Fatalf("hint = %q, want the default-leader form", got)
	}
	if err := a.keymap.Set("leader", "ctrl+j"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := a.homeShortcutsHint(); got != "Show keyboard shortcuts with ctrl+j" {
		t.Fatalf("remapped hint = %q, want the ctrl+j form", got)
	}
	if err := a.keymap.Set("leader", "none"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if a.homeShortcutsHint() != "" {
		t.Fatal("a disabled leader must omit the hint")
	}
}

// TestHomeFooterLineWithHint pins the S6.5 parts join: destination +
// hint " · "-joined, each part omittable, the dimmed single line.
func TestHomeFooterLineWithHint(t *testing.T) {
	t.Parallel()
	a := testApp()
	a.Service.Dir = "/home/u/proj"
	a.homeDirFunc = func() string { return "/home/u" }
	if got := stripANSI(a.homeFooterLine(80)); got != "~/proj · Show keyboard shortcuts with ctrl+x" {
		t.Fatalf("joined line = %q", got)
	}
	// destination omitted (Dir "") → the hint-only line
	b := testApp()
	if got := stripANSI(b.homeFooterLine(80)); got != "Show keyboard shortcuts with ctrl+x" {
		t.Fatalf("hint-only line = %q", got)
	}
	// both omitted (leader none) → ""
	if err := b.keymap.Set("leader", "none"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if b.homeFooterLine(80) != "" {
		t.Fatal("no parts must omit the line")
	}
	// the render slot: the line is the LAST home-bottom line (after the
	// tips line on the testApp)
	c := testApp()
	out := stripANSI(c.home.render(&c.store, 80, c.theme))
	if !strings.HasSuffix(strings.TrimSpace(out), "Show keyboard shortcuts with ctrl+x") {
		t.Fatalf("the footer hint line must be the last home line:\n%s", out)
	}
}

// TestFooterHintTeatest pins the hint on the real boot (the destination
// prefix is the unpredictable TempDir path — the unit leg pins it; the
// teatest leg pins the registry-rendered hint suffix).
func TestFooterHintTeatest(t *testing.T) {
	drv := fake.New()
	ts := testutil.BootWithDriver(t, drv)
	c := client.New(ts.URL, ts.Dir)
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		full := stripANSI(string(b))
		return hasLine("New session")(b) && strings.Contains(full, "Show keyboard shortcuts with ctrl+x")
	}, teatest.WithDuration(5*time.Second))
	tm.Send(ctrlCKey)
	tm.Send(press('y'))
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
