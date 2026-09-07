package tui

import "testing"

// TestPlainSemver pins the git-describe → plain-semver mapping for the home
// footer (Q10): strip the leading "v" and cut at the first "-" (the
// -N-g<sha> distance suffix and any -dirty suffix).
func TestPlainSemver(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{"0.0.0-dev", "0.0.0"},
		{"v0.8.0", "0.8.0"},
		{"v0.8.0-4-gabcdef", "0.8.0"},
		{"v0.8.0-dirty", "0.8.0"},
		{"", ""},
	}
	for _, tt := range cases {
		if got := plainSemver(tt.in); got != tt.want {
			t.Fatalf("plainSemver(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestSetVersionSmoke pins the post-construction setter (the SetKeybinds
// pattern): a zero App value + SetVersion stores the raw build string (the
// home footer applies plainSemver at render time).
func TestSetVersionSmoke(t *testing.T) {
	t.Parallel()
	a := &App{}
	a.SetVersion("v0.8.0-4-gabcdef")
	if a.version != "v0.8.0-4-gabcdef" {
		t.Fatalf("a.version = %q, want %q", a.version, "v0.8.0-4-gabcdef")
	}
}
