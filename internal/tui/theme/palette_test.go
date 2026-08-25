package theme

import (
	"bytes"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"
)

func fastOpts() PaletteOptions {
	return PaletteOptions{ProbeTimeout: 50 * time.Millisecond, IdleTimeout: 50 * time.Millisecond, HardTimeout: 200 * time.Millisecond}
}

func resp4(i int, hex string) string { return "\x1b]4;" + strconv.Itoa(i) + ";" + hex + "\x07" }
func respN(i int, hex string) string { return "\x1b]" + strconv.Itoa(i) + ";" + hex + "\x07" }

// fullResponses returns a probe response + all 16 palette + all 9 special
// responses (the complete terminal answer).
func fullResponses() string {
	var b strings.Builder
	b.WriteString(resp4(0, "#111111")) // probe answer (re-consumed by the probe phase)
	for i := 0; i < 16; i++ {
		b.WriteString(resp4(i, hex16[i]))
	}
	for _, i := range []int{10, 11, 12, 13, 14, 15, 16, 17, 19} {
		b.WriteString(respN(i, hex16[i%16]))
	}
	return b.String()
}

var hex16 = []string{"#000000", "#800000", "#008000", "#808000", "#000080", "#800080", "#008080", "#c0c0c0", "#808080", "#ff0000", "#00ff00", "#ffff00", "#0000ff", "#ff00ff", "#00ffff", "#ffffff"}

func TestDetectPaletteFullResponse(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader(fullResponses())
	got, ok := DetectPalette(in, &out, fastOpts())
	if !ok {
		t.Fatal("expected OSC support")
	}
	if got.Palette[1] != "#800000" || got.Palette[15] != "#ffffff" {
		t.Errorf("palette = %v", got.Palette)
	}
	if got.DefaultBackground != "#ffff00" { // respN(11, hex16[11%16] = "#ffff00")
		t.Errorf("DefaultBackground = %q", got.DefaultBackground)
	}
	if got.DefaultForeground != "#00ff00" { // respN(10, hex16[10%16] = "#00ff00")
		t.Errorf("DefaultForeground = %q", got.DefaultForeground)
	}
	// the queries must have been written, tmux-unwrapped by default
	q := out.String()
	if !strings.Contains(q, "\x1b]4;0;?\x07") || !strings.Contains(q, "\x1b]4;15;?\x07") {
		t.Errorf("palette queries missing: %q", q)
	}
	if !strings.Contains(q, "\x1b]11;?\x07") || !strings.Contains(q, "\x1b]19;?\x07") {
		t.Errorf("special queries missing: %q", q)
	}
}

func TestDetectPaletteRGBScaling(t *testing.T) {
	// the terminal answers the probe with a full 16-bit rgb: value:
	// rgb:1f/3c/5a → each component scaled: round(31/255*255)=31 etc.
	// (maxIn = 1 << (4*len) - 1 = 255 for 2-digit hex — identity at 8-bit)
	probe := resp4(0, "rgb:1f/3c/5a")
	full := probe
	for i := 0; i < 16; i++ {
		full += resp4(i, hex16[i])
	}
	for _, i := range []int{10, 11, 12, 13, 14, 15, 16, 17, 19} {
		full += respN(i, hex16[i%16])
	}
	got, ok := DetectPalette(strings.NewReader(full), &bytes.Buffer{}, fastOpts())
	if !ok {
		t.Fatal("expected OSC support")
	}
	if got.Palette[0] != "#1f3c5a" { // rgb:1f/3c/5a → #1f3c5a (8-bit identity)
		t.Errorf("scaled palette[0] = %q, want #1f3c5a", got.Palette[0])
	}
}

func TestDetectPaletteNoResponseUnsupported(t *testing.T) {
	// no probe answer within ProbeTimeout → unsupported (spec §3: no system
	// theme, active falls back to "opencode").
	in := func() io.Reader {
		pr, pw := io.Pipe()
		go func() { time.Sleep(120 * time.Millisecond); pw.Close() }()
		return pr
	}()
	got, ok := DetectPalette(in, &bytes.Buffer{}, fastOpts())
	if ok {
		t.Fatal("expected unsupported")
	}
	if got != (TerminalColors{}) {
		t.Errorf("colors = %+v, want zero", got)
	}
}

func TestDetectPalettePartialResponseIdleFallsBack(t *testing.T) {
	// the terminal answers only palette indices 0-2, then goes silent: the
	// idle timer ends the query and the partial result is returned (the
	// caller treats palette[0] present as usable; S0.7 decides system-theme
	// eligibility upstream-exactly: palette[0] must be present).
	probe := resp4(0, "#111111")
	partial := probe + resp4(0, "#111111") + resp4(1, "#222222") + resp4(2, "#333333")
	got, ok := DetectPalette(strings.NewReader(partial), &bytes.Buffer{}, fastOpts())
	if !ok {
		t.Fatal("expected OSC support (probe answered)")
	}
	if got.Palette[0] != "#111111" || got.Palette[1] != "#222222" || got.Palette[2] != "#333333" {
		t.Errorf("partial palette = %v", got.Palette)
	}
	if got.Palette[3] != "" {
		t.Errorf("unanswered index must stay empty, got %q", got.Palette[3])
	}
}

func TestDetectPaletteLegacyTmuxWrapping(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader(fullResponses())
	opts := fastOpts()
	opts.LegacyTmux = true
	if _, ok := DetectPalette(in, &out, opts); !ok {
		t.Fatal("expected OSC support")
	}
	q := out.String()
	// legacy tmux: ESC ESC doubled, palette query wrapped in Ptmux container;
	// special queries NOT wrapped (upstream writeOsc second arg).
	if !strings.Contains(q, "\x1bPtmux;") {
		t.Errorf("legacy tmux wrapping missing: %q", q)
	}
	if strings.Contains(q, "\x1bPtmux;\x1b\x1b]11;?") {
		t.Errorf("special queries must not be tmux-wrapped: %q", q)
	}
}

func TestDetectPaletteNonLegacyNotWrapped(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader(fullResponses())
	if _, ok := DetectPalette(in, &out, fastOpts()); !ok {
		t.Fatal("expected OSC support")
	}
	q := out.String()
	if !strings.Contains(q, "\x1b]11;?\x07") {
		t.Errorf("bare special query missing: %q", q)
	}
	if strings.Contains(q, "\x1bPtmux;") {
		t.Errorf("non-legacy output must not be tmux-wrapped: %q", q)
	}
}

func TestDetectPaletteIdleTimerFires(t *testing.T) {
	// the terminal answers the probe + palette 0-2, then goes silent (the
	// writer blocks without closing): the per-group idle timer (50ms) must
	// end the query long before the hard timer (200ms) or any EOF.
	pr, pw := io.Pipe()
	go func() {
		_, _ = pw.Write([]byte(resp4(0, "#111111") + resp4(1, "#222222") + resp4(2, "#333333")))
		time.Sleep(300 * time.Millisecond)
		_ = pw.Close()
	}()
	start := time.Now()
	got, ok := DetectPalette(pr, &bytes.Buffer{}, fastOpts())
	elapsed := time.Since(start)
	if !ok {
		t.Fatal("expected OSC support (probe answered)")
	}
	if got.Palette[0] != "#111111" || got.Palette[1] != "#222222" || got.Palette[2] != "#333333" {
		t.Errorf("partial palette = %v", got.Palette)
	}
	if got.Palette[3] != "" {
		t.Errorf("unanswered index must stay empty, got %q", got.Palette[3])
	}
	if elapsed >= 200*time.Millisecond {
		t.Errorf("query ended in %v: the idle timer (50ms), not the hard timer (200ms) or EOF, must have fired", elapsed)
	}
}

func TestDetectPaletteRGBScaling16Bit(t *testing.T) {
	// 4-digit rgb: components pin the non-identity scaling path:
	// maxIn = 1<<(4*4)-1 = 65535 → round(65535/65535*255)=255,
	// round(32768/65535*255)=128.
	full := resp4(0, "rgb:ffff/0000/8000")
	for i := 0; i < 16; i++ {
		full += resp4(i, hex16[i])
	}
	for _, i := range []int{10, 11, 12, 13, 14, 15, 16, 17, 19} {
		full += respN(i, hex16[i%16])
	}
	got, ok := DetectPalette(strings.NewReader(full), &bytes.Buffer{}, fastOpts())
	if !ok {
		t.Fatal("expected OSC support")
	}
	if got.Palette[0] != "#ff0080" {
		t.Errorf("16-bit scaled palette[0] = %q, want #ff0080", got.Palette[0])
	}
}
