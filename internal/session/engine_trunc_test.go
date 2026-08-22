package session_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kido5217/yolo/internal/llm"
	fakellm "github.com/kido5217/yolo/internal/llm/fake"
)

// TestBashTruncatedOutputTellsModelWhereFullOutputIs is the model-visible
// contract for truncated bash output (upstream shell.ts): the tool result in
// the next model round must start with the truncation marker pointing at the
// saved full-output file, so the model can page it with read instead of
// re-running the command.
func TestBashTruncatedOutputTellsModelWhereFullOutputIs(t *testing.T) {
	h := newHarness(t)
	h.build(t)
	h.drv.Turns = []fakellm.Turn{
		{Parts: []llm.Part{
			{Kind: "tool", Name: "bash", CallID: "call_seq", Text: `{"command":"seq 1 3000"}`},
		}},
		{Parts: []llm.Part{
			{Kind: "text", Text: "done", Finish: "stop", Usage: &llm.Usage{Input: 1, Output: 1}},
		}},
	}
	ses := h.startSession(t, t.TempDir())
	waitIdle(t, h, ses, func() {
		if _, err := h.eng.Send(t.Context(), ses, "run seq", nil); err != nil {
			t.Fatalf("Send: %v", err)
		}
	})
	reqs := nonTitle(h.drv.Requests())
	if len(reqs) < 2 {
		t.Fatalf("model rounds = %d, want 2", len(reqs))
	}
	var toolContent string
	for _, m := range reqs[1].Messages {
		if m.Role == llm.RoleTool {
			toolContent = m.Content
		}
	}
	const marker = "...output truncated...\n\nFull output saved to: "
	if !strings.HasPrefix(toolContent, marker) {
		t.Fatalf("tool result does not start with the truncation marker:\n%q", toolContent[:min(120, len(toolContent))])
	}
	rest := strings.TrimPrefix(toolContent, marker)
	path, _, _ := strings.Cut(rest, "\n")
	wantDir := filepath.Join(h.dataDir, "tool-output")
	if !strings.HasPrefix(path, wantDir+string(filepath.Separator)+"tool_") {
		t.Fatalf("full output path %q not under %s/tool_", path, wantDir)
	}
	full, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("full output file: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(string(full), "\n"), "\n")
	if len(lines) != 3000 || lines[0] != "1" || lines[2999] != "3000" {
		t.Fatalf("file has %d lines (first=%q last=%q), want 1..3000", len(lines), lines[0], lines[len(lines)-1])
	}
}
