# AGENTS.md — internal/tui (pure-client TUI)

## Purpose

The bubbletea v2 TUI. A pure client: it talks to the in-process core HTTP
server over the wire contract only — it never reaches into core internals.

## Ownership

Everything under `internal/tui/`: the app and its concern files (app,
attention, commands, deletefailed, destination, dialog, footer, frecency, home, huhdlg,
hydrate, keymap, keys, locale, logo, mention, permission, prompt, providerdlg,
rename, retrydlg, select, session, sessionsdlg, startup, statusdlg, style,
themecmds, themedlg, tips, toast, view, whichkey, wrap), `client/` (HTTP + SSE client, backoff),
`store/` (display state), `theme/` (theme engine — 33 embedded upstream
themes, resolution, system-theme generation, OSC palette detection, custom
discovery, selection chain over the TUI-local KV file; TUI-local by root
principle 4, all filesystem paths injected by cmd/yolo), `imports_test.go`,
and the teatest suites.

## Local Contracts

- Import purity (root principle 4, enforced by `TestImportsDirection` in
  `imports_test.go`): non-test files import only `internal/protocol` and
  `internal/tui/*` from within the module. `_test.go` may additionally
  import `internal/server/testutil` (real-stack blackbox suites) and
  `internal/llm` / `internal/llm/fake` (scripted fake turns).
- V1 behavior pins (PROGRESS.md "Key verified facts"): keymap is pgup/pgdn
  scroll + `\`+enter newline (noted in /help).
- Transcript word-wrap (yolo-0ca): `renderMessages` wraps every PLAIN
  transcript line at the viewport width via `wrapLine` (wrap.go) — word
  boundaries, over-long tokens hard-split, CJK/emoji count 2 columns, tab =
  word separator. Assistant TEXT parts render through the glamour renderer
  (S1.3) — the renderer's `WithWordWrap(w-3)` is their wrap strategy, every
  rendered line carries the upstream 3-column indent, and the styled output
  never reaches `wrapLine`. The rest of the transcript renders through
  theme-derived surfaces (S1.6–S1.8): reasoning parts through the
  `NewReasoningRenderer` (dimmed, collapsible, subtle syntax), tool rows
  through `toolRow` (per-tool glyph + `alt+e` expand), errors through the
  left-only-border `messageErrorBoxStyle` box, and toasts in their own
  left-only Error-colored box (the same visual language) — all styled from
  the resolved theme's `StyleConfig`, with one transcript/reasoning renderer
  constructed per `renderMessages` call. Styled lines wrap BEFORE styling
  (`toolRow` returns the style + plain text; `writeStyled` re-renders
  each wrapped line). The viewport's hard clip remains the backstop, never
  the content strategy (no horizontal scroll is bound — clipped text would
  be unreadable). `WindowSizeMsg` sets `sess.isDirty` so a resize re-wraps.
- Below-viewport surface wrap (yolo-ukc): every non-transcript text surface
  wraps at the terminal width (`App.termWidth()`, fallback 80) with the same
  `wrapLine` — toasts, permission overlay, slash menu, model/agent dialogs
  (rows AND hint lines via `dimWrapped`), home session rows, the `!` error
  line; each renderer takes a `w` param from `App.view()`. Home session rows
  wrap as tagged segments (`rowLines` re-derives the title/` · meta` split
  per visual line, S0.9); the selected row's background paints every
  rendered line's content only — no background on the plain indent or the
  empty tail beyond the content. The home logo
  (S0.8) is the one exception: a fixed 39-column glyph block that never
  wraps or shrinks (the upstream look) — terminals under 39 columns clip
  it in the alt-screen frame. The session
  route counts the wrapped help line's real line count in the viewport
  height budget. The model dialog cell hangs at the left-pane column
  (`modelRow`); when the left pane alone ≥ width, cell lines go full width.
   Footer, divider and the locked quit dialog stay single-line (the help
   dialog is modal since S3.6 — the modal frame word-wraps its over-wide
   body line at the panel width).
  Prompt line: bubbles v2 textinput `View` = prompt(2) + `SetWidth` +
  cursor(1) — `WindowSizeMsg` sets `SetWidth(w-3)` so the line fits.
- SSE drop contract (v0.1.3): `client.Events` returns `(events, resync)` —
  every dropped `/event` connection pings resync (the bus has no replay, so
  gap events are unrecoverable); the app re-hydrates the current route on
   `resyncMsg` (hydrate.go) and re-arms `resyncPump`. Never make reconnects
  silent again (the pre-v0.1.3 silent reconnect left the footer stuck on
  `busy` and the transcript stale).
- Completed `bash` tool parts render an inline output preview (10-line head,
  `…` overflow hint, `headPreview` in session.go) without alt+e — upstream
  parity; other tools stay row-only until expanded.
- App-shell themed (S0.8–S0.10): every shell surface reads the theme (the
  `th` render-arg or `a.theme`) — the only remaining statics are `title` +
  `divider` (the yolo-specific session title/divider + the dialog titles —
  S2–S3/S1 retheme them) and the `cursorStyle(th)` helper (bold + text fg).
  teatest SGR goldens pin TTY_FORCE=1 + TERM=xterm-256color (ANSI256
  `38;5;N` via x/ansi v0.11.8 `Convert256` — deviation 125 at the S0 slice
  gate).

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
    (dependency policy: allowlist only, root "Project")
- lipgloss v2 `Render()` appends a trailing SGR reset AFTER the styled
  input: `TrimRight` padded plain strings BEFORE styling (a post-style trim
  silently misses), and count display widths in runes
  (`utf8.RuneCountInString`) — `·` is 2 bytes/1 col, `○` 3 bytes/1 col.
- Golang skills (full table in root AGENTS.md "Golang skills") — invoke the
  relevant one(s) per task. Always pair `golang-naming` + `golang-code-style`
  for new code. The TUI-relevant set: `golang-testing` (teatest suites,
  table-driven), `golang-concurrency` (the event loop, SSE resync pump,
  goroutine hygiene), `golang-safety` (store state, slice/append aliasing),
  `golang-data-structures` (wrap/transcript buffers). TUI patterns follow the
  project `charm-stack` skill (Bubbletea/Bubbles/Lipgloss; its v1 import
  paths are illustrative — the allowlist's `charm.land/*` v2 line wins);
  suites follow `tui_suite_test.go`.

## Verification

- Root gate at module root: `go vet ./... && go test ./...` (includes
  `TestImportsDirection` and the teatest suites).

## Child DOX Index

- None.
