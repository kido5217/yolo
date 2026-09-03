package tool

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

//go:embed desc/bash.txt
var bashDesc string

const defaultBashTimeoutMS = 120000

const maxBashTimeoutMS = 1<<31 - 1 // ~24.8 days; keeps the int64 ns Duration from wrapping

type bashTool struct{}

var _ Tool = bashTool{}

func (bashTool) ID() string         { return "bash" }
func (bashTool) Permission() string { return "bash" }
func (bashTool) Desc() string       { return bashDesc }

func (bashTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The command to execute",
			},
			"timeout": map[string]any{
				"type":        "number",
				"description": "Optional timeout in milliseconds",
			},
		},
		"required": []string{"command"},
	}
}

// Patterns pins the plan's v1 simplification: the permission resource and
// always-rule are the first whitespace token plus " *" when the command has
// more tokens (e.g. "git *"), else the token alone (e.g. "ls"). Upstream's
// tree-sitter scan and external-directory pre-scan are out of scope (v1).
func (bashTool) Patterns(raw json.RawMessage) ([]string, []string, error) {
	command, _, err := bashArgs(raw)
	if err != nil {
		return nil, nil, err
	}
	p := bashPrefix(command)
	return []string{p}, []string{p}, nil
}

// External is empty: v1 does no external-directory pre-scan for bash.
func (bashTool) External(raw json.RawMessage) ([]string, error) {
	return nil, nil
}

func bashPrefix(command string) string {
	tokens := strings.Fields(command)
	if len(tokens) <= 1 {
		return tokens[0]
	}
	return tokens[0] + " *"
}

func bashArgs(raw json.RawMessage) (command string, timeoutMS int, err error) {
	m, err := argsMap(raw)
	if err != nil {
		return
	}
	v, _ := m["command"].(string)
	if strings.TrimSpace(v) == "" {
		err = errors.New("command is required")
		return
	}
	command = v
	timeoutMS = defaultBashTimeoutMS
	if v, ok := m["timeout"]; ok && v != nil {
		n, ok2 := argInt(v)
		if !ok2 {
			err = errors.New("timeout must be a positive integer")
			return
		}
		if n <= 0 {
			err = errors.New("timeout must be a positive integer (milliseconds)")
			return
		}
		if n > maxBashTimeoutMS {
			n = maxBashTimeoutMS
		}
		timeoutMS = n
	}
	return
}

func (bashTool) Run(ctx context.Context, raw json.RawMessage, env *Env) (Output, error) {
	command, timeoutMS, err := bashArgs(raw)
	if err != nil {
		return Output{}, err
	}
	if env == nil {
		env = &Env{}
	}
	if env.Shell == nil {
		return Output{}, errors.New("shell is not initialized")
	}
	if env.Log != nil {
		env.Log.Info("bash command", "command", shortRunes(command, 200))
	}
	code, out, err := env.Shell.Exec(ctx, command, timeoutMS, nil)
	switch {
	case errors.Is(err, errShellTimeout):
		// Pinned upstream v1.18.18 message (shell.ts).
		return Output{}, fmt.Errorf(
			"shell tool terminated command after exceeding timeout %d ms. "+
				"If this command is expected to take longer and is not waiting for "+
				"interactive input, retry with a larger timeout value in milliseconds",
			timeoutMS)
	case errors.Is(err, errShellAborted):
		return Output{}, errors.New("command aborted")
	case err != nil:
		return Output{}, err
	}
	text, cut := Truncate(out, env.Limits.withDefaults())
	if text == "" {
		text = "(no output)"
	}
	meta := map[string]any{"truncated": cut}
	if cut {
		// Upstream shell.ts: a truncated run stores the FULL output and
		// tells the model the path — without the marker the model sees a
		// silent mid-stream start and re-runs the command in a loop. A
		// store failure degrades to the bare marker and is logged (the
		// model still sees the truncated tail).
		file, werr := WriteFullOutput(env.OutputDir, out)
		switch {
		case werr != nil:
			if env.Log != nil {
				env.Log.Warn("bash: saving full output failed", "error", werr)
			}
		case file != "":
			text = "...output truncated...\n\nFull output saved to: " + file + "\n\n" + text
			meta["outputPath"] = file
		}
	}
	if code != 0 {
		// Non-zero exit is NOT a tool error; the model sees the exit code
		// only when present.
		meta["exit"] = code
	}
	if env.Log != nil {
		env.Log.Info("bash exit", "exit", code, "truncated", cut)
	}
	return Output{Title: command, Text: text, Meta: meta}, nil
}
