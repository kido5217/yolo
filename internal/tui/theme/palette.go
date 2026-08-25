package theme

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	term "github.com/charmbracelet/x/term"
)

// Response shapes (verbatim from @opentui/core 0.4.5 terminal-palette.ts:
// OSC4_RESPONSE / OSC_SPECIAL_RESPONSE).
var (
	osc4Response       = regexp.MustCompile(`\x1b]4;(\d+);(?:(?:rgb:)([0-9a-fA-F]+)/([0-9a-fA-F]+)/([0-9a-fA-F]+)|#([0-9a-fA-F]{6}))(?:\x07|\x1b\\)`)
	oscSpecialResponse = regexp.MustCompile(`\x1b](\d+);(?:(?:rgb:)([0-9a-fA-F]+)/([0-9a-fA-F]+)/([0-9a-fA-F]+)|#([0-9a-fA-F]{6}))(?:\x07|\x1b\\)`)
)

// toHex8 is the port of toHex (terminal-palette.ts:10065): #hex6 verbatim
// (lowercased), or rgb: components scaled to 8 bits
// (round(val / (16^len-1) * 255)), else #000000.
func toHex8(r, g, b, hex6 string) string {
	if hex6 != "" {
		return "#" + strings.ToLower(hex6)
	}
	if r != "" && g != "" && b != "" {
		return "#" + scaleComponent(r) + scaleComponent(g) + scaleComponent(b)
	}
	return "#000000"
}

// scaleComponent is the port of scaleComponent (terminal-palette.ts:10060):
// a hex component scaled to 8 bits (round(val / maxIn * 255), integer).
func scaleComponent(comp string) string {
	if len(comp) == 0 || len(comp) > 8 {
		return "00"
	}
	val, err := strconv.ParseUint(comp, 16, 32)
	if err != nil {
		return "00"
	}
	maxIn := uint32(1)<<(4*len(comp)) - 1
	return fmt.Sprintf("%02x", (uint32(val)*255+maxIn/2)/maxIn)
}

// wrapForTmux is the port of wrapForTmux (terminal-palette.ts:10072):
// double every ESC and wrap in the Ptmux passthrough container.
func wrapForTmux(osc string) string {
	escaped := strings.ReplaceAll(osc, "\x1b", "\x1b\x1b")
	return "\x1bPtmux;" + escaped + "\x1b\\"
}

type PaletteOptions struct {
	ProbeTimeout time.Duration // default 100ms (spec-pinned; upstream detectOSCSupport is 300ms)
	IdleTimeout  time.Duration // default 100ms (spec-pinned; upstream OTUI_PALETTE_IDLE_TIMEOUT_MS is 300ms)
	HardTimeout  time.Duration // default 100ms (spec-pinned; upstream detect timeout is 5000ms)
	LegacyTmux   bool          // TMUX set && TMUX_PANE unset
}

func (o *PaletteOptions) fill() {
	if o.ProbeTimeout == 0 {
		o.ProbeTimeout = 100 * time.Millisecond
	}
	if o.IdleTimeout == 0 {
		o.IdleTimeout = 100 * time.Millisecond
	}
	if o.HardTimeout == 0 {
		o.HardTimeout = 100 * time.Millisecond
	}
}

// readEvent is one pump event: a data chunk, or the EOF marker.
type readEvent struct {
	data []byte
	eof  bool
}

// readLoop pumps in into ch until EOF/error or quit, then closes ch.
func readLoop(in io.Reader, ch chan readEvent, quit chan struct{}) {
	defer close(ch)
	buf := make([]byte, 512)
	for {
		n, err := in.Read(buf)
		if n > 0 {
			ev := readEvent{data: make([]byte, n)}
			copy(ev.data, buf[:n])
			select {
			case ch <- ev:
			case <-quit:
				return
			}
		}
		if err != nil {
			select {
			case ch <- readEvent{eof: true}:
			case <-quit:
			}
			return
		}
	}
}

// newIdle arms a one-shot AfterFunc signalling the returned channel; the
// channel is buffered so a replaced (abandoned) timer never leaks a
// goroutine.
func newIdle(d time.Duration) chan struct{} {
	ch := make(chan struct{}, 1)
	time.AfterFunc(d, func() {
		ch <- struct{}{}
	})
	return ch
}

// DetectPalette ports TerminalPalette.detect: (1) the OSC 4;0 support probe,
// (2) the 16 palette + 9 special-color queries with per-group idle timers,
// the hard timeout, and the 8192/4096 buffer cap. in/out are injected; the
// TTY preconditions are the caller's job (DetectStd).
func DetectPalette(in io.Reader, out io.Writer, opts PaletteOptions) (TerminalColors, bool) {
	opts.fill()
	ch := make(chan readEvent, 8)
	quit := make(chan struct{})
	defer close(quit)
	go readLoop(in, ch, quit)

	var colors TerminalColors

	// Probe phase: the support probe is OSC 4;0; the terminal's answer is the
	// value of palette index 0, retained as palette[0] (first-wins below).
	probe := "\x1b]4;0;?\x07"
	if opts.LegacyTmux {
		probe = wrapForTmux(probe)
	}
	if _, err := out.Write([]byte(probe)); err != nil {
		return TerminalColors{}, false
	}
	deadline := time.NewTimer(opts.ProbeTimeout)
	defer deadline.Stop()
	buffer := ""
	supported := false
probeLoop:
	for {
		select {
		case ev := <-ch:
			if ev.eof {
				break probeLoop
			}
			buffer += string(ev.data)
			m := osc4Response.FindStringSubmatch(buffer)
			if m == nil {
				continue
			}
			if idx, err := strconv.Atoi(m[1]); err == nil && idx >= 0 && idx < 16 && colors.Palette[idx] == "" {
				colors.Palette[idx] = toHex8(m[2], m[3], m[4], m[5])
			}
			buffer = buffer[len(m[0]):]
			supported = true
			break probeLoop
		case <-deadline.C:
			break probeLoop
		}
	}
	if !supported {
		return TerminalColors{}, false
	}

	// Query phase: the 16 palette queries first (one Ptmux container in
	// legacy tmux), then the special-color queries (10-17,19 — or only 10-12
	// in legacy tmux; never tmux-wrapped, upstream writeOsc second arg).
	specialIdx := []int{10, 11, 12, 13, 14, 15, 16, 17, 19}
	if opts.LegacyTmux {
		specialIdx = []int{10, 11, 12}
	}
	var q strings.Builder
	for i := 0; i < 16; i++ {
		q.WriteString("\x1b]4;" + strconv.Itoa(i) + ";?\x07")
	}
	paletteQueries := q.String()
	if opts.LegacyTmux {
		paletteQueries = wrapForTmux(paletteQueries)
	}
	q.Reset()
	for _, i := range specialIdx {
		q.WriteString("\x1b]" + strconv.Itoa(i) + ";?\x07")
	}
	if _, err := out.Write([]byte(paletteQueries + q.String())); err != nil {
		return colors, true
	}

	specials := make(map[int]string, len(specialIdx))
	specialSet := make([]bool, 20)
	for _, i := range specialIdx {
		specialSet[i] = true
	}

	paletteIdle := newIdle(opts.IdleTimeout)
	specialIdle := newIdle(opts.IdleTimeout)
	hardTimer := time.NewTimer(opts.HardTimeout)
	defer hardTimer.Stop()
	paletteDone, specialDone := false, false
	allPalette := func() bool {
		for i := 0; i < 16; i++ {
			if colors.Palette[i] == "" {
				return false
			}
		}
		return true
	}
	allSpecial := func() bool {
		for _, i := range specialIdx {
			if specials[i] == "" {
				return false
			}
		}
		return true
	}

	// handle demuxes both response regexes over the shared buffer, stores
	// first-wins (a stored slot is never overwritten), drops the consumed
	// prefix, and applies the 8192→keep-last-4096 cap. It reports whether a
	// response for each group was newly stored — the idle timers reset on
	// that only.
	grp := func(m []int, i int) string {
		if m[i] < 0 {
			return ""
		}
		return buffer[m[i]:m[i+1]]
	}
	handle := func() (bool, bool) {
		var pStored, sStored bool
		lastEnd := 0
		for _, m := range osc4Response.FindAllStringSubmatchIndex(buffer, -1) {
			if m[1] > lastEnd {
				lastEnd = m[1]
			}
			idx, err := strconv.Atoi(buffer[m[2]:m[3]])
			if err != nil || idx >= 16 || colors.Palette[idx] != "" {
				continue
			}
			colors.Palette[idx] = toHex8(grp(m, 4), grp(m, 6), grp(m, 8), grp(m, 10))
			pStored = true
		}
		for _, m := range oscSpecialResponse.FindAllStringSubmatchIndex(buffer, -1) {
			if m[1] > lastEnd {
				lastEnd = m[1]
			}
			idx, err := strconv.Atoi(buffer[m[2]:m[3]])
			if err != nil || idx >= len(specialSet) || !specialSet[idx] || specials[idx] != "" {
				continue
			}
			specials[idx] = toHex8(grp(m, 4), grp(m, 6), grp(m, 8), grp(m, 10))
			sStored = true
		}
		buffer = buffer[lastEnd:]
		if len(buffer) > 8192 {
			buffer = buffer[len(buffer)-4096:]
		}
		return pStored, sStored
	}

	first := true
queryLoop:
	for {
		if paletteDone && specialDone {
			break
		}
		if first {
			// the probe read may have buffered the whole answer stream:
			// demux the seed before waiting on the timers.
			first = false
			p, s := handle()
			if p {
				paletteIdle = newIdle(opts.IdleTimeout)
			}
			if s {
				specialIdle = newIdle(opts.IdleTimeout)
			}
			paletteDone = paletteDone || allPalette()
			specialDone = specialDone || allSpecial()
			continue
		}
		select {
		case ev := <-ch:
			if ev.eof {
				break queryLoop
			}
			buffer += string(ev.data)
			p, s := handle()
			if p {
				paletteIdle = newIdle(opts.IdleTimeout)
			}
			if s {
				specialIdle = newIdle(opts.IdleTimeout)
			}
			if !paletteDone && allPalette() {
				paletteDone = true
			}
			if !specialDone && allSpecial() {
				specialDone = true
			}
		case <-paletteIdle:
			paletteDone = true
		case <-specialIdle:
			specialDone = true
		case <-hardTimer.C:
			break queryLoop
		}
	}
	if v := specials[10]; v != "" {
		colors.DefaultForeground = v
	}
	if v := specials[11]; v != "" {
		colors.DefaultBackground = v
	}
	return colors, true
}

// isCharDevice reports whether f is a character device (a real TTY).
func isCharDevice(f *os.File) bool {
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}

// DetectStd is the raw-mode /dev/tty wrapper: stdin+stdout both char devices
// → use os.Stdin as-is; otherwise /dev/tty in raw mode (x/term). Input is
// pumped through a pipe so ctx cancellation closes it early; LegacyTmux is
// detected from the environment (TMUX set && TMUX_PANE unset).
func DetectStd(ctx context.Context) (TerminalColors, bool) {
	opts := PaletteOptions{LegacyTmux: os.Getenv("TMUX") != "" && os.Getenv("TMUX_PANE") == ""}
	var src *os.File
	if isCharDevice(os.Stdin) && isCharDevice(os.Stdout) {
		src = os.Stdin
	} else {
		f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
		if err != nil {
			return TerminalColors{}, false
		}
		defer f.Close()
		state, err := term.MakeRaw(uintptr(f.Fd()))
		if err != nil {
			return TerminalColors{}, false
		}
		defer term.Restore(uintptr(f.Fd()), state)
		src = f
	}
	pr, pw := io.Pipe()
	done := make(chan struct{})
	go func() {
		defer pw.Close()
		buf := make([]byte, 4096)
		for {
			n, err := src.Read(buf)
			if n > 0 {
				if _, werr := pw.Write(buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()
	go func() {
		select {
		case <-ctx.Done():
			pw.Close()
		case <-done:
		}
	}()
	defer close(done)
	defer pw.Close()
	return DetectPalette(pr, os.Stdout, opts)
}
