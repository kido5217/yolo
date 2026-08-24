package tui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

// toast is one line of the transient error-flash block (LOCKED: red, above
// the footer, cap 3 with the oldest evicted; newest line on top).
type toast struct {
	id  int
	msg string
}

// maxToasts caps the visible toast queue (LOCKED: ≤3).
const maxToasts = 3

// toastTTL is the per-toast auto-clear window (LOCKED: 4s, via a tea tick
// cmd). It is a var so tests can shorten it.
var toastTTL = 4 * time.Second

// toastExpireMsg clears the toast it names when the armed tick fires; it is
// a no-op if the toast was evicted by a newer one first.
type toastExpireMsg struct{ id int }

// toast records a transient message, evicting past the cap and arming one
// tick cmd per toast; the cmds are drained by Update and run next frame.
func (a *App) toast(msg string) {
	a.toastSeq++
	id := a.toastSeq
	a.toasts = append(a.toasts, toast{id: id, msg: msg})
	if len(a.toasts) > maxToasts {
		// Fresh slice: eviction drops the old backing array's head so an
		// in-flight View of pre-eviction toasts is not mutated out from it.
		kept := make([]toast, maxToasts)
		copy(kept, a.toasts[len(a.toasts)-maxToasts:])
		a.toasts = kept
	}
	a.toastCmds = append(a.toastCmds, tea.Tick(toastTTL, func(_ time.Time) tea.Msg {
		return toastExpireMsg{id: id}
	}))
}

// drainToastCmds hands over the ticks armed during the current update (nil
// when none).
func (a *App) drainToastCmds() tea.Cmd {
	if len(a.toastCmds) == 0 {
		return nil
	}
	cmds := a.toastCmds
	a.toastCmds = nil
	if len(cmds) == 1 {
		return cmds[0]
	}
	return tea.Batch(cmds...)
}

// removeToast drops one toast by id (the cap can have evicted it already).
func (a *App) removeToast(id int) {
	for i, t := range a.toasts {
		if t.id == id {
			a.toasts = append(a.toasts[:i], a.toasts[i+1:]...)
			return
		}
	}
}

// toastsView renders the block above the footer, newest line on top. Each
// message word-wraps at the terminal width (a toast can carry a long error
// string; the frame budget counts the wrapped lines dynamically).
func (a *App) toastsView(w int) string {
	if len(a.toasts) == 0 {
		return ""
	}
	var b strings.Builder
	for i := len(a.toasts) - 1; i >= 0; i-- {
		if i != len(a.toasts)-1 {
			b.WriteByte('\n')
		}
		for j, l := range strings.Split(wrapLine("\u2022 "+a.toasts[i].msg, w), "\n") {
			if j > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(errRed.Render(l))
		}
	}
	return b.String()
}
