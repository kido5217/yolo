# AGENTS.md — internal/tui (pure-client TUI)

## Purpose

The bubbletea v2 TUI. A pure client: it talks to the in-process core HTTP
server over the wire contract only — it never reaches into core internals.

## Ownership

Everything under `internal/tui/`: app/model/panes, `client/` (HTTP + SSE
client, backoff), `store/` (display state), `imports_test.go`, and the
teatest suites.

## Local Contracts

- Import purity (root principle 4, enforced by `TestImportsDirection` in
  `imports_test.go`): non-test files import only `internal/protocol` and
  `internal/tui/*` from within the module. `_test.go` may additionally
  import `internal/server/testutil` (real-stack blackbox suites) and
  `internal/llm` / `internal/llm/fake` (scripted fake turns).
- V1 behavior pins (PROGRESS.md "Key verified facts"): keymap is pgup/pgdn
  scroll + `\`+enter newline (noted in /help).

## Work Guidance

- teatest v2 mechanics (bit T28; full detail in PROGRESS.md "Key verified
  facts"):
  - each `WaitFor` drains the SHARED output buffer — consecutive `WaitFor`s
    observe disjoint slices, so a multi-token terminal state must be ONE
    merged condition, never two sequential waits; probe `Read`s consume
    bytes later assertions need
  - the fake terminal is not a TTY → lipgloss strips EVERY style; pin
    `teatest.WithProgramOptions(tea.WithEnvironment([]string{"TTY_FORCE=1",
    "TERM=xterm-256color"}))` for deterministic ANSI256 SGR
  - v2 `tea.Tick(d, f)` callback is `func(time.Time) tea.Msg`; v2 programs
    handle `tea.QuitMsg` internally
  - `charmbracelet/colorprofile` stays indirect — never import it directly
    ("nothing else" dep rule)
- lipgloss v2 `Render()` appends a trailing SGR reset AFTER the styled
  input: `TrimRight` padded plain strings BEFORE styling (a post-style trim
  silently misses), and count display widths in runes
  (`utf8.RuneCountInString`) — `·` is 2 bytes/1 col, `○` 3 bytes/1 col.
- New code: `golang-naming` + `golang-code-style`; suites follow
  `tui_suite_test.go`.

## Verification

- Root gate at module root: `go vet ./... && go test ./...` (includes
  `TestImportsDirection` and the teatest suites).

## Child DOX Index

- None.
