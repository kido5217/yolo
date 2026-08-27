package tui

import (
	"testing"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// TestToolRowGlyphs pins the S1.7 per-tool icon + pending/complete/error
// row forms (the opencode dark tokens; the SGR quantization is the
// teatest layer's job).
func TestToolRowGlyphs(t *testing.T) {
	all, err := theme.AllThemes()
	if err != nil {
		t.Fatalf("AllThemes: %v", err)
	}
	r, err := theme.ResolveTheme(all["opencode"], "dark")
	if err != nil {
		t.Fatalf("ResolveTheme: %v", err)
	}
	th := theme.Theme{R: r, Name: "opencode", Mode: "dark"}
	part := func(tool string, status, title, errMsg string) protocol.Part {
		return protocol.Part{ID: "t", Type: "tool", Tool: tool, CallID: "c",
			State: &protocol.ToolState{Status: status, Title: title, Error: errMsg}}
	}
	tests := []struct {
		name   string
		p      protocol.Part
		want   string
		fgWant string
	}{
		{"bash completed", part("bash", "completed", "ls -la", ""), "$ ls -la", "#808080"},
		{"bash running", part("bash", "running", "", ""), "~ Writing command...", "#eeeeee"},
		{"read running (spinner)", part("read", "running", "", ""), "⠋ Reading file...", "#eeeeee"},
		{"write completed", part("write", "completed", "f.go", ""), "← f.go", "#808080"},
		{"edit completed", part("edit", "completed", "f.go", ""), "← f.go", "#808080"},
		{"glob running", part("glob", "running", "", ""), "~ Finding files...", "#eeeeee"},
		{"grep error", part("grep", "error", "grep", "no match"), "✱ grep", "#e06c75"},
		{"todowrite error (failure text)", part("todowrite", "error", "todos", "boom"), "⚙ Todo update failed", "#e06c75"},
		{"unknown tool completed", part("webfetch", "completed", "url", ""), "⚙ url", "#808080"},
		{"read error", part("read", "error", "f.go", "not found"), "→ f.go", "#e06c75"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sty, row, ok := toolRow(tc.p, th, "⠋")
			if !ok || row != tc.want {
				t.Fatalf("toolRow = (%q, %v), want (%q, true)", row, ok, tc.want)
			}
			// the S0.10 mechanism (session_theme_test.go:241): lipgloss
			// GetForeground returns the 24-bit hex as an opaque RGBA.
			if got, want := sty.GetForeground(), hexColor(tc.fgWant); got != want {
				t.Errorf("fg = %v, want %v", got, want)
			}
		})
	}
}
