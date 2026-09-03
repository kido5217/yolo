// parity_test.go — the S8.3 yolo-side render dump (the D6 design):
// TestParityDump renders each of the 17 parity surfaces (the frozen D2
// list) through the real stack (testutil + the fake driver scripted from
// the shared canned constants + the theme engine under TTY_FORCE) and
// writes the FULL raw teatest stream to $YOLO_PARITY_DUMP/yolo/<name>.raw
// for the sweep normalizer. Gated on the env var (t.Skip when unset) so
// the CI gate never renders the set — the sweep is user-run, never CI
// (the root e2e-live.sh pattern).
package tui

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/kido5217/yolo/internal/llm"
	"github.com/kido5217/yolo/internal/llm/fake"
	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// paritySurface is one D2 surface (the name = the upstream fixture file
// name under internal/tui/testdata/parity/upstream/).
type paritySurface struct {
	name   string
	width  int
	height int
	turn   string // "text" | "tool" | "todo" — the canned book key
	// steps drive the key script (the home settle precedes them).
	steps []parityStep
}

type parityStep struct {
	keys []tea.KeyPressMsg
	text string // tm.Type (plain prompt text)
	cond func([]byte) bool
	d    time.Duration
}

func pressCtrlD() tea.KeyPressMsg { return tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl} }
func pressCtrlR() tea.KeyPressMsg { return tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl} }
func pressCtrlC() tea.KeyPressMsg { return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl} }

func leaderCont(k rune) []tea.KeyPressMsg {
	return []tea.KeyPressMsg{pressLeader(), press(k)}
}

// paritySurfaces is the frozen D2 table (the yolo-side scripts; the
// upstream key equivalents live in scripts/parity/capture.py SURFACES).
func paritySurfaces() []paritySurface {
	// turn drives the canned prompt + enter + settle on the final
	// transcript line (the yolo side needs `n` for the new session first
	// — the upstream side's prompt is ready on home; the scripts are
	// per-side, the normalizer compares the final screens).
	turn := func(p promptTurn) []parityStep {
		return []parityStep{
			{keys: []tea.KeyPressMsg{press('n')}, cond: hasLine("esc abort/back"), d: 10 * time.Second},
			{text: p.prompt, cond: hasLine(p.prompt), d: 10 * time.Second},
			{keys: []tea.KeyPressMsg{press(tea.KeyEnter)}, cond: p.settle, d: 15 * time.Second},
		}
	}
	textTurn := promptTurn{prompt: parityPromptText, settle: hasLine("Done.")}
	toolTurn := promptTurn{prompt: parityPromptTool, settle: hasLine("parity-ok")}
	todoTurn := promptTurn{prompt: parityPromptTodo, settle: hasLine("first item")}
	return []paritySurface{
		{name: "home", width: 80, height: 24, turn: "text"},
		{name: "session-text", width: 80, height: 24, turn: "text", steps: turn(textTurn)},
		{name: "session-tool", width: 80, height: 24, turn: "tool", steps: turn(toolTurn)},
		{name: "palette", width: 80, height: 24, turn: "text", steps: append(
			turn(textTurn), parityStep{keys: []tea.KeyPressMsg{pressCtrlP()}, cond: hasLine("Commands"), d: 10 * time.Second})},
		{name: "help", width: 80, height: 24, turn: "text", steps: append(
			turn(textTurn),
			parityStep{text: "/help", cond: hasLine("/help"), d: 10 * time.Second},
			parityStep{keys: []tea.KeyPressMsg{press(tea.KeyEnter)}, cond: hasLine("Help"), d: 10 * time.Second})},
		{name: "model", width: 80, height: 24, turn: "text", steps: append(
			turn(textTurn), parityStep{keys: leaderCont('m'), cond: hasLines("Model", "Qwen"), d: 15 * time.Second})},
		{name: "agent", width: 80, height: 24, turn: "text", steps: append(
			turn(textTurn), parityStep{keys: leaderCont('a'), cond: hasLines("Agents", "build"), d: 10 * time.Second})},
		{name: "theme", width: 80, height: 24, turn: "text", steps: append(
			turn(textTurn), parityStep{keys: leaderCont('t'), cond: hasLine("Themes"), d: 10 * time.Second})},
		{name: "session-list", width: 80, height: 24, turn: "text", steps: append(
			turn(textTurn), parityStep{keys: leaderCont('l'), cond: hasLine("Sessions"), d: 10 * time.Second})},
		{name: "session-rename", width: 80, height: 24, turn: "text", steps: append(
			turn(textTurn), parityStep{keys: []tea.KeyPressMsg{pressCtrlR()}, cond: hasLine("Rename Session"), d: 10 * time.Second})},
		{name: "session-delete", width: 80, height: 24, turn: "text", steps: append(
			turn(textTurn),
			parityStep{keys: leaderCont('l'), cond: hasLine("Sessions"), d: 10 * time.Second},
			parityStep{keys: []tea.KeyPressMsg{pressCtrlD()}, cond: hasLine("Press ctrl+d again to confirm"), d: 10 * time.Second})},
		{name: "status", width: 80, height: 24, turn: "text", steps: append(
			turn(textTurn), parityStep{keys: leaderCont('s'), cond: hasLine("Status"), d: 10 * time.Second})},
		{name: "which-key", width: 80, height: 24, turn: "text", steps: []parityStep{
			{keys: []tea.KeyPressMsg{pressLeader()}, cond: hasLines("Model", "Session"), d: 10 * time.Second},
		}},
		{name: "sidebar", width: 140, height: 30, turn: "todo", steps: turn(todoTurn)},
		{name: "prompt-slash", width: 80, height: 24, turn: "text", steps: append(
			turn(textTurn), parityStep{text: "/", cond: hasLine("/help"), d: 10 * time.Second})},
		{name: "prompt-mention", width: 80, height: 24, turn: "text", steps: append(
			turn(textTurn), parityStep{text: "@par", cond: hasLine("parity-a.txt"), d: 10 * time.Second})},
		// The ctrl+c is fire-and-forget: the alt-screen exit byte only
		// reaches the teatest stream during the harness teardown (after
		// tm.Quit/WaitFinished), so it cannot be a step terminal condition
		// — the settle is the shared 2 s appendPump + the post-Quit drain.
		{name: "epilogue", width: 80, height: 24, turn: "text", steps: append(
			turn(textTurn), parityStep{keys: []tea.KeyPressMsg{pressCtrlC()}})},
	}
}

type promptTurn struct {
	prompt string
	settle func([]byte) bool
}

func TestParityDump(t *testing.T) {
	dir := os.Getenv("YOLO_PARITY_DUMP")
	if dir == "" {
		t.Skip("YOLO_PARITY_DUMP unset — the parity sweep is user-run, never CI")
	}
	if err := os.MkdirAll(filepath.Join(dir, "yolo"), 0o755); err != nil {
		t.Fatalf("parity dump: mkdir: %v", err)
	}
	for _, s := range paritySurfaces() {
		s := s
		t.Run(s.name, func(t *testing.T) { dumpSurface(t, dir, s) })
	}
}

func parityDriver(turn string) *fake.Driver {
	switch turn {
	case "tool":
		return fake.New(
			fake.Turn{Parts: []llm.Part{{Kind: "tool", Name: "bash", CallID: "call_canned1",
				Args: json.RawMessage(parityArgsTool), Text: parityArgsTool,
				Usage: &llm.Usage{Input: 12, Output: 40}, Finish: "tool_calls"}}},
			fake.Turn{Parts: []llm.Part{{Kind: "text", Text: parityReplyTool,
				Usage: &llm.Usage{Input: 12, Output: 40}, Finish: "stop"}}},
		)
	case "todo":
		return fake.New(
			fake.Turn{Parts: []llm.Part{{Kind: "tool", Name: "todowrite", CallID: "call_canned2",
				Args: json.RawMessage(parityArgsTodo), Text: parityArgsTodo,
				Usage: &llm.Usage{Input: 12, Output: 40}, Finish: "tool_calls"}}},
			fake.Turn{Parts: []llm.Part{{Kind: "text", Text: parityReplyTodo,
				Usage: &llm.Usage{Input: 12, Output: 40}, Finish: "stop"}}},
		)
	default:
		return fake.New(
			fake.Turn{Parts: []llm.Part{{Kind: "text", Text: parityReplyText,
				Usage: &llm.Usage{Input: 12, Output: 40}, Finish: "stop"}}},
		)
	}
}

// parityConfig is the yolo-side config (the scope config): it defines the
// providers the model dialog lists — "openai" seeded from the pinned
// catalog (the same 47 models the upstream catalog pin carries) + the
// "mockllm" custom provider (the yolo referent of the upstream
// opencode.json provider entry). The LLM itself is the in-process fake
// driver (the config never reaches the network).
func parityConfig(t *testing.T) *protocol.Config {
	t.Helper()
	catalog, err := os.ReadFile(filepath.Join("testdata", "parity", "catalog-pin.json"))
	if err != nil {
		t.Fatalf("parity config: catalog pin: %v", err)
	}
	var c struct {
		OpenAI struct {
			Models map[string]struct {
				Name string `json:"name"`
			} `json:"models"`
		} `json:"openai"`
	}
	if err := json.Unmarshal(catalog, &c); err != nil {
		t.Fatalf("parity config: catalog decode: %v", err)
	}
	models := map[string]any{}
	for id, m := range c.OpenAI.Models {
		models[id] = map[string]any{"name": m.Name}
	}
	return &protocol.Config{
		// the tool/todo surfaces auto-approve (the upstream side runs the
		// equivalent --auto — both sides reach the second turn without a
		// permission overlay).
		Permission: map[string]any{"bash": "allow", "todowrite": "allow"},
		Provider: map[string]protocol.ProviderConfig{
			"openai": {BaseURL: "https://api.openai.com/v1", Models: models},
			"mockllm": {BaseURL: "http://127.0.0.1:0/v1",
				Models: map[string]any{"canned": map[string]any{"name": "Canned"}}},
		},
	}
}

// parityApp boots the theme engine + the App (the session_markdown_test.go
// idiom).
func parityApp(t *testing.T, ts *testutil.TestServer) *App {
	t.Helper()
	dir := t.TempDir()
	e, err := theme.New(theme.EngineOptions{
		KVPath:        filepath.Join(dir, "kv.json"),
		GlobalYoloDir: dir,
		CWD:           dir,
		Palette:       func(context.Context) (theme.TerminalColors, bool) { return theme.TerminalColors{}, false },
	})
	if err != nil {
		t.Fatalf("theme.New: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })
	if err := e.Resolve(context.Background()); err != nil {
		t.Fatalf("theme.Resolve: %v", err)
	}
	c := client.New(ts.URL, ts.Dir)
	a := NewApp(c, store.State{}, "", e)
	t.Cleanup(a.Close)
	return a
}

// pumpUntil drains the teatest output buffer until cond matches the
// accumulated raw (or the deadline). io.ReadAll on the buffer is
// non-blocking (it consumes what is present, EOF on empty) — the loop
// cannot deadlock (the detail-pass finding: the teatest v2 Output
// semantics).
func pumpUntil(t *testing.T, tm *teatest.TestModel, raw []byte, cond func([]byte) bool, d time.Duration) []byte {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if b, _ := io.ReadAll(tm.Output()); len(b) > 0 {
			raw = append(raw, b...)
		}
		if cond(raw) {
			return raw
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("parity dump: the terminal condition was not met within %s (stream %d bytes)", d, len(raw))
	return nil
}

func appendPump(t *testing.T, tm *teatest.TestModel, raw []byte, d time.Duration) []byte {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if b, _ := io.ReadAll(tm.Output()); len(b) > 0 {
			raw = append(raw, b...)
		}
		time.Sleep(25 * time.Millisecond)
	}
	return raw
}

func drain(tm *teatest.TestModel) []byte {
	b, _ := io.ReadAll(tm.Output())
	return b
}

// dumpSurface boots ONE surface, drives its key script, and writes the
// full raw stream to <outDir>/yolo/<name>.raw.
func dumpSurface(t *testing.T, outDir string, s paritySurface) {
	t.Helper()
	ts := testutil.BootWithDriverConfig(t, parityDriver(s.turn), parityConfig(t))
	if s.name == "prompt-mention" {
		// the mention menu lists the session-dir files (the pinned
		// scratch pair — the upstream side has the same pair).
		for _, f := range []string{"parity-a.txt", "parity-b.txt"} {
			if err := os.WriteFile(filepath.Join(ts.Dir, f), []byte("x"), 0o644); err != nil {
				t.Fatalf("parity dump: %v", err)
			}
		}
	}
	a := parityApp(t, ts)
	tm := teatest.NewTestModel(t, a,
		teatest.WithInitialTermSize(s.width, s.height),
		teatest.WithProgramOptions(tea.WithEnvironment([]string{
			"TTY_FORCE=1", "TERM=xterm-256color",
		})),
	)
	var raw []byte
	raw = pumpUntil(t, tm, raw, hasLine("New session"), 15*time.Second) // the home settle
	for _, st := range s.steps {
		if st.text != "" {
			tm.Type(st.text)
		}
		for _, k := range st.keys {
			tm.Send(k)
		}
		if st.cond != nil && st.d > 0 {
			raw = pumpUntil(t, tm, raw, st.cond, st.d)
		}
	}
	raw = appendPump(t, tm, raw, 2*time.Second) // the final settle
	raw = append(raw, drain(tm)...)
	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(10*time.Second))
	raw = append(raw, drain(tm)...)
	if err := os.WriteFile(filepath.Join(outDir, "yolo", s.name+".raw"), raw, 0o644); err != nil {
		t.Fatalf("parity dump: write %s: %v", s.name, err)
	}
}
