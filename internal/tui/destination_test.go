package tui

import (
	"strings"
	"testing"
)

// TestAbbrevHome pins the ported abbreviateHome (upstream runtime.tsx:3-10).
func TestAbbrevHome(t *testing.T) {
	cases := []struct{ dir, home, want string }{
		{"/home/u/proj", "/home/u", "~/proj"},
		{"/home/u", "/home/u", "~"},
		{"/etc", "/home/u", "/etc"},
		{"/home/u2/x", "/home/u", "/home/u2/x"},
		{"/home/u/../etc", "/home/u", "/home/u/../etc"},
		{"", "/home/u", ""},
		{"/home/u", "", "/home/u"},
		{"/tmp/xyz/001", "/home/u", "/tmp/xyz/001"},
	}
	for _, c := range cases {
		if got := abbrevHome(c.dir, c.home); got != c.want {
			t.Fatalf("abbrevHome(%q, %q) = %q, want %q", c.dir, c.home, got, c.want)
		}
	}
}

// TestSessionDestination pins the scope-dir resolution + the ""-Dir
// omission (testApp) + the homeDir seam.
func TestSessionDestination(t *testing.T) {
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

// TestHomeFooterLine pins the S6.4 line shape (destination only — the
// hint joins at S6.5): the dimmed single line, omitted when empty, and
// its render slot (after the help line + tips line, before the footer).
func TestHomeFooterLine(t *testing.T) {
	a := testApp()
	if a.homeFooterLine(80) != "" {
		t.Fatal("no destination (Dir == \"\") must omit the line")
	}
	a.Service.Dir = "/home/u/proj"
	a.homeDirFunc = func() string { return "/home/u" }
	if got := stripANSI(a.homeFooterLine(80)); got != "~/proj" {
		t.Fatalf("footer line = %q, want %q", got, "~/proj")
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
