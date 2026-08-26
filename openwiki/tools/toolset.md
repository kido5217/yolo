---
type: reference
title: Toolset
description: "internal/tool built-in tools (read, write, edit, glob, grep, bash, todowrite): the Tool interface and registry, permission-based visibility, the per-session persistent bash shell, output truncation with the full output persisted to <data>/tool-output (7-day retention, startup sweep), and the sha256-pinned desc/*.txt descriptions."
tags: [tools, toolset, bash, shell, truncation, schema, desc, embed]
verified:
  - by: openwiki/0.4.0
    at: 2026-08-26T18:04:14.871Z
sources:
  - id: openwiki-source-41612f8ed7b59c998588fda2
    resource: repo://cmd/yolo/deps.go
  - id: openwiki-source-f703acf2f5c1f3e3c606ce01
    resource: repo://internal/tool/bash.go
  - id: openwiki-source-76ce0b34c8925895c52f0673
    resource: repo://internal/tool/read_test.go
  - id: openwiki-source-7303ae6a35e98d58c0ef9ffb
    resource: repo://internal/tool/read.go
  - id: openwiki-source-0f8c52f609e3e0c574870155
    resource: repo://internal/tool/shell.go
  - id: openwiki-source-b33956af69d605588792144c
    resource: repo://internal/tool/tool.go
  - id: openwiki-source-b247c4fd7703027a8a373425
    resource: repo://internal/tool/truncate.go
generated: {by: "opencode", at: "2026-08-26T18:04:14.871Z"}
---

# Toolset

`internal/tool` implements the v1 tool framework: the **`Tool` contract**, the
**registry**, **permission-based visibility**, **LLM schema rendering**,
**output truncation**, and the built-in tools
**`read`, `write`, `edit`, `glob`, `grep`, `bash`, `todowrite`**
(`tool.go`, and one file each under `internal/tool/`).

## The Tool contract

```go
type Tool interface {
    ID() string
    Permission() string
    Patterns(raw json.RawMessage) (resources []string, always []string, err error)
    External(raw json.RawMessage) ([]string, error)
    Schema() map[string]any
    Desc() string
    Run(ctx context.Context, raw json.RawMessage, env *Env) (Output, error)
}
```

- **`Patterns`/`External` receive raw args only (no `Env`)** — they emit paths
  as given; the engine resolves them against `Env.Dir` before checks.
- **`Run`** receives `*Env` (the session project dir — the permission anchor
  and base for relative paths — the session shell, output limits, the output
  dir, session storage, session id, and a logger) and returns **`Output`**
  (`Title` for the TUI, `Text` for the model, `Meta` for TUI display metadata).
- **`Registry()`** returns the built-ins keyed by ID; **`Visible`** filters out
  tools the ruleset hides (`permission.Hidden`); **`SchemaFor`** renders the
  LLM tool-call schema (`type:"function"` wrapping `name`/`description`/
  `parameters`).

Output limits default to **2000 lines / 50 KiB** (`DefaultMaxLines` /
`DefaultMaxBytes`, mirroring upstream `truncate.ts`).

## The per-session persistent bash shell

`Shell` (`shell.go`) keeps **one `bash --norc --noprofile` process alive
across `Exec` calls** so `cd`, environment assignments, and other shell state
carry over. It is spawned lazily on the first `Exec`, rooted at the session dir.

Each command is written as its lines followed by a **marker line**
`echo __YOLO_END_{n}_$?_$(pwd | base64 -w0)`; the child's stdout **and** stderr
both hit a single output pipe, and output is read until a line matches the
shared `endMarkerRe` (`^__YOLO_END_(\d+)_([^\s]*)$`) **with the matching
counter `n`** (a marker with another counter — e.g. echoed by the command itself
— falls through as normal output). The exit code and the new cwd come from the
marker. Key behaviors:

- A **10 MB in-memory cap** (`shellOutputCap`) guards each invocation (display
  truncation is applied separately by the bash tool).
- On **timeout** or **abort** the **process group is SIGKILL'd** (`Setpgid`
  makes the shell the group leader, so children share the group) and the shell
  is marked dead; the next `Exec` respawns from the last known cwd (the dir if
  it no longer exists).
- Signal-terminated processes report exit `128+signum` (matching the marker
  path, e.g. 137/143).

`bashTool.Run` runs the command through the shell and rewrites the sentinel
errors to the **pinned upstream v1.18.18 messages**: a timeout becomes
`"shell tool terminated command after exceeding timeout %d ms. ... retry with a
larger timeout value in milliseconds"` and an abort becomes
`"command aborted"`. A **non-zero exit is NOT a tool error** — the exit code is
surfaced only via `meta["exit"]` when present.

## Output truncation contract

`Truncate(text, Limits)` keeps the **TAIL** of the text: the last up-to
`MaxLines` lines within `MaxBytes` UTF-8 bytes (a port of upstream `shell.ts
tail()`, including the UTF-8-boundary cut of a single over-long line); it
returns `cut` when anything was removed.

The bash tool's truncation contract (upstream `shell.ts`): **a truncated run
stores the FULL output and tells the model the path** — without the marker the
model sees a silent mid-stream start and re-runs the command in a loop.
Concretely:

- `WriteFullOutput(dir, text)` writes the untruncated output to
  **`dir/tool_<id>`** (the data dir's `tool-output/`) and returns the path; an
  empty dir returns `("", nil)` so the caller skips the marker.
- When `cut`, the model-visible text is prefixed with
  `"...output truncated...\n\nFull output saved to: <path>\n\n"` and
  `meta["outputPath"]` is set.
- **`CleanOutputDir(dir)`** removes `tool_*` outputs older than
  **`OutputDirRetention` (7 days)**; a missing dir is a no-op. Upstream runs
  this sweep hourly; **v1 runs it once at startup** (`cmd/yolo/deps.go`,
  best-effort so the hygiene pass never blocks boot).

## The individual tools

- **`bash`** — `Patterns` pins a v1 simplification: the permission resource and
  always-rule are the **first whitespace token plus `" *"`** when the command
  has more tokens (e.g. `git *`), else the token alone (e.g. `ls`);
  `External` is empty (no external-directory pre-scan for bash in v1).
- **`read`** — upstream v1.18.18 format verbatim; a 2000-char per-line cap and
  an 8000-byte NUL-sniff window for binary detection.
- **`write`, `edit`, `glob`, `grep`** — file/path tools whose `External` paths
  feed the engine's external-directory gate.
- **`todowrite`** — persists the session's todo list (the only tool that uses
  `Env.Storage`).

## Pinned descriptions + the embed quirk

Every tool's description is an **embedded, sha256-pinned text file**: each tool
file carries `//go:embed desc/<tool>.txt` (e.g. `desc/bash.txt`,
`desc/read.txt`, …), loaded via the scalar `//go:embed` var. Because the
installed toolchains fail a plain `import "embed"` + scalar embed with
`embed imported and not used`, the in-repo workaround is
**`import _ "embed"`** (seen in `read.go` and every other tool file) — keep
this pattern. The pins record current intended content, not an upstream lock;
an intentional change re-baselines the sha256 pin in the same commit (see the
`TestDescPinned` suite).
