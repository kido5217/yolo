// Package tool implements the v1 tool framework: the Tool contract, the
// registry, permission-based visibility, LLM schema rendering, output
// truncation, and the read tool (upstream v1.18.18 format verbatim).
package tool

import (
	"context"
	"encoding/json"

	"github.com/kido5217/yolo/internal/log"
	"github.com/kido5217/yolo/internal/permission"
	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/storage"
)

// Defaults mirror upstream truncate.ts (MAX_LINES / MAX_BYTES); config
// tool_output.* overrides arrive with the engine (Task 17+).
const (
	DefaultMaxLines = 2000
	DefaultMaxBytes = 50 * 1024
)

// Limits bounds tool output.
type Limits struct{ MaxLines, MaxBytes int }

// withDefaults fills zero fields with the upstream defaults.
func (l Limits) withDefaults() Limits {
	if l.MaxLines <= 0 {
		l.MaxLines = DefaultMaxLines
	}
	if l.MaxBytes <= 0 {
		l.MaxBytes = DefaultMaxBytes
	}
	return l
}

// Shell is the per-session persistent shell (shell.go: NewShell/Exec/Cwd/
// Close). Tools that don't run commands must tolerate a nil Env.Shell.

// Env is what Run receives: the session project dir (permission anchor and
// base for relative paths), the session shell, output limits, and — for
// tools that persist (todowrite) — the session storage, plus OutputDir (the
// data dir's tool-output/), where bash stores the FULL output of a truncated
// run (upstream shell.ts). Dir/Shell/Limits are the engine's concern to
// populate; OutputDir/Storage/SessionID may be empty/nil for tools that
// ignore them.
type Env struct {
	Dir       string
	Shell     *Shell
	Limits    Limits
	OutputDir string
	Storage   *storage.DB
	SessionID string
	// Log receives tool-level log points (nil = no-op; wired by the engine).
	Log *log.Logger
}

// shortRunes truncates s to n runes (command logging; "..." marker).
func shortRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}

// argsMap normalizes empty raw args to "{}" and unmarshals them into a map
// (every tool's arg parser shares the same normalization).
func argsMap(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// Output is a tool result: Title for the TUI, Text for the model, Meta for
// TUI display metadata (nil ok).
type Output struct {
	Title string
	Text  string
	Meta  map[string]any
}

// Tool is the v1 tool contract.
//
// Engine usage (implemented in internal/session, Task 17):
//
//	res, always, err := t.Patterns(args)
//	for _, p := range t.External(args) {
//	    abs := resolve(p, env.Dir) // relative args resolve against Env.Dir
//	    if abs is outside env.Dir -> svc.Ask("external_directory", dir(abs)+"/*")
//	}
//	d := svc.Ask(ctx, Request{Permission: t.Permission(), Resources: res, Always: always, Tool: t.ID()})
//	Deny -> part error "permission rejected" ; Allow -> out, err := t.Run(...)
//
// Patterns/External receive raw args only (no Env), so they emit paths as
// given; the engine resolves them against Env.Dir before checks.
type Tool interface {
	ID() string
	Permission() string
	Patterns(raw json.RawMessage) (resources []string, always []string, err error)
	External(raw json.RawMessage) ([]string, error)
	Schema() map[string]any
	Desc() string
	Run(ctx context.Context, raw json.RawMessage, env *Env) (Output, error)
}

// Registry returns the built-in tools keyed by ID.
func Registry() map[string]Tool {
	return map[string]Tool{
		"read":      readTool{},
		"write":     writeTool{},
		"edit":      editTool{},
		"glob":      globTool{},
		"grep":      grepTool{},
		"bash":      bashTool{},
		"todowrite": todoWriteTool{},
	}
}

// Visible filters out tools the ruleset hides (permission.Hidden).
func Visible(rules []protocol.Rule, all map[string]Tool) map[string]Tool {
	ids := make([]string, 0, len(all))
	for id := range all {
		ids = append(ids, id)
	}
	hidden := permission.Hidden(rules, ids)
	out := make(map[string]Tool, len(all))
	for id, t := range all {
		if hidden[id] {
			continue
		}
		out[id] = t
	}
	return out
}

// SchemaFor renders the LLM tool-call schema for t.
func SchemaFor(t Tool) map[string]any {
	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        t.ID(),
			"description": t.Desc(),
			"parameters":  t.Schema(),
		},
	}
}
