package tui

import (
	"strings"
	"testing"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// TestMessageErrorBox pins the S1.8 box SHAPE at the render level (pure
// render, no renderer): the non-aborted message error is the left-border
// box (the "│" border line is structural — it survives the non-TTY
// strip, so its presence is assertable); the aborted error is the muted
// "~ <message>" line with no border. Colors are NOT asserted here —
// without TTY_FORCE lipgloss strips SGR in a unit test — they are pinned
// through the style constructor in TestMessageErrorBoxStyle.
func TestMessageErrorBox(t *testing.T) {
	all, err := theme.AllThemes()
	if err != nil {
		t.Fatalf("AllThemes: %v", err)
	}
	r, err := theme.ResolveTheme(all["yolo"], "dark")
	if err != nil {
		t.Fatalf("ResolveTheme: %v", err)
	}
	th := theme.Theme{R: r, Name: "yolo", Mode: "dark"}

	// non-aborted: the box (border + message).
	out := renderMessageError(protocol.MessageError{Type: "unknown", Message: "boom"}, th, 77)
	stripped := stripANSI(out)
	if !strings.Contains(stripped, "boom") {
		t.Errorf("box missing the message: %q", stripped)
	}
	if !strings.Contains(stripped, "│") {
		t.Errorf("box missing the left border: %q", stripped)
	}
	// aborted: the muted line, no border.
	out = renderMessageError(protocol.MessageError{Type: "aborted", Message: "user interrupted"}, th, 77)
	stripped = stripANSI(out)
	if strings.Contains(stripped, "│") {
		t.Errorf("aborted must not box: %q", stripped)
	}
	if !strings.Contains(stripped, "~ user interrupted") {
		t.Errorf("aborted line = %q, want '~ user interrupted'", stripped)
	}
	// zero Theme: the bare plain message (S0.7 degradation), no border.
	out = renderMessageError(protocol.MessageError{Type: "unknown", Message: "boom"}, theme.Theme{}, 77)
	if got := stripANSI(out); got != "boom" {
		t.Errorf("zero-theme = %q, want bare \"boom\"", got)
	}
}

// TestMessageErrorBoxStyle pins the box CHROME via the style constructor's
// accessors (the S0.10 `fgWant` mechanism): left border only, in the error
// token; the panel background; the 1/2 padding; textMuted text.
func TestMessageErrorBoxStyle(t *testing.T) {
	all, err := theme.AllThemes()
	if err != nil {
		t.Fatalf("AllThemes: %v", err)
	}
	r, err := theme.ResolveTheme(all["yolo"], "dark")
	if err != nil {
		t.Fatalf("ResolveTheme: %v", err)
	}
	th := theme.Theme{R: r, Name: "yolo", Mode: "dark"}
	st := messageErrorBoxStyle(th)
	_, top, right, bottom, left := st.GetBorder()
	if !left || top || right || bottom {
		t.Errorf("border = (top %v, right %v, bottom %v, left %v), want left only", top, right, bottom, left)
	}
	if got, want := st.GetBorderLeftForeground(), hexColor("#e06c75"); got != want {
		t.Errorf("border fg = %v, want error %v", got, want)
	}
	if got, want := st.GetBackground(), hexColor("#141414"); got != want {
		t.Errorf("bg = %v, want backgroundPanel %v", got, want)
	}
	if pt, pr, pb, pl := st.GetPadding(); pt != 1 || pr != 0 || pb != 1 || pl != 2 {
		t.Errorf("padding = (%d,%d,%d,%d), want (1,0,1,2)", pt, pr, pb, pl)
	}
}
