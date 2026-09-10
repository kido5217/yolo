package tui

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/kido5217/yolo/internal/tui/theme"
)

// wantLogoBlockSHA256 pins the 8 logo lines in logo.go (root principle 3:
// the pin records the current intended content; an intentional change
// re-baselines the pin in the same commit). Canonical form: logoLeft[0..7]
// then logoRight[0..7], each line followed by "\n".
const wantLogoBlockSHA256 = "58a2c7c1c21ba8fe4b01076431bba324f969dc726932d6fa9b6dbb80444d6d42"

func logoBlockText() string {
	var b strings.Builder
	for _, l := range logoLeft {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	for _, l := range logoRight {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	return b.String()
}

func TestLogoBlockPinned(t *testing.T) {
	t.Parallel()
	sum := sha256.Sum256([]byte(logoBlockText()))
	if got := hex.EncodeToString(sum[:]); got != wantLogoBlockSHA256 {
		t.Fatalf("logo block sha256 = %s, want %s — re-baseline the pin in the same commit", got, wantLogoBlockSHA256)
	}
}

// logoPlainLines are the 8 combined (left + gap + right) plain lines —
// the zero-Theme render; the homeView geometry tests compose the layout
// over them.
func logoPlainLines() []string {
	var zero theme.Theme
	return strings.Split(renderLogo(zero), "\n")
}

// TestRenderLogoZeroThemeIsPlain pins the mark translation with no theme
// (nil-engine runs, S0.7): the plain translated glyphs (the hollow '_'
// counters render as spaces), no SGR, never a panic.
func TestRenderLogoZeroThemeIsPlain(t *testing.T) {
	t.Parallel()
	var zero theme.Theme
	got := renderLogo(zero)
	want := strings.Join([]string{
		"██     ██ █████████" + " " + "██      █████████",
		"██     ██ █████████" + " " + "██      █████████",
		"██     ██ ██     ██" + " " + "██      ██     ██",
		"██     ██ ██     ██" + " " + "██      ██     ██",
		"█████████ ██     ██" + " " + "██      ██     ██",
		"█████████ ██     ██" + " " + "██      ██     ██",
		"    ██    █████████" + " " + "███████ █████████",
		"    ██    █████████" + " " + "███████ █████████",
	}, "\n")
	if got != want {
		t.Fatalf("zero-theme logo = %q, want the plain translated glyphs:\n%q", got, want)
	}
}
