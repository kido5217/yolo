# S1 — transcript rendering (slice bead `yolo-oae.2`)

Render the session transcript with glamour — GFM markdown + per-language
chroma syntax highlighting driven by the resolved S0 theme — and restyle
the reasoning, tool-row, and error surfaces to the upstream tokens.

**State: detail pass pending** — execution of this slice is GATED on the
detail pass below (Slice Detail Protocol rule 2): no task may start until
this file holds full 5-step TDD detail for all of its tasks.

## Binding task table

Pointer only (FROZEN — Slice Detail Protocol rule 1): `plan.md` →
`## Task inventory` → `### S1 — transcript rendering (slice bead yolo-oae.2)`.
Bead titles, scope, and pinned commit messages live there and may not be
changed by a detail pass; any required change = STOP + explicit user
approval + re-record in plan.md.

## Dep gate

`charm.land/glamour/v2` v2.0.1 — dep-proposal bead first (root AGENTS.md
dependency policy: evidence from live web search — maintenance, license,
pure Go, transitive surface; approval gate = STOP before `go get`; lands as
task S1.1).

## Exact upstream sources (the detail pass reads these AT DETAIL TIME)

`/tmp/opencode-upstream` @ v1.18.18:

- `packages/tui/src/routes/session/index.tsx` — the assistant text-part
  markdown render 1591–1720 (`createSyntaxStyleMemo(generateSubtleSyntax(theme))`
  1607, `<markdown fg={theme.markdownText} syntaxStyle={syntax()}>`
  1700–1707), further markdown usages 2114–2129, the assistant error box
  1534–1548, `InlineTool`/`InlineToolRow` 1850–2000, `BlockTool` 2040–2050
  (S0.10 restyled yolo's tool-row CHROME; S1.7 ports the upstream tool-row
  rendering semantics — read the S0.10 notes in `s0-theme-engine.md` for the
  handoff), the reasoning block (grep `reasoning` in this file).
- `packages/tui/src/theme/index.ts` — `generateSubtleSyntax` + the
  markdown/syntax token map (the port source for the glamour `StyleConfig`
  + chroma token map in S1.2).
- `packages/tui/src/theme/assets/*.json` — the `markdown*` and `syntax*`
  keys (flat: `markdownText`, `markdownCodeBlock`, `syntaxComment`,
  `syntaxKeyword`, … — the S0.1 embedded `ThemeJson` model may need
  extending; the detail pass decides and says so).
- the opentui `<markdown>` element (GFM feature set + chroma token names):
  the bundled core at `/tmp/opencode/.opentui-core/` (grep `markdown` for
  the chunk) or `node_modules/@opentui/core` in the upstream tree — map its
  options + token list onto glamour v2.0.1's `StyleConfig`.

## yolo anchors

- `internal/tui/session.go` — the transcript render path; S1.3 replaces the
  plain wrap for text parts.
- `internal/tui/theme/` — renderer home; `syntax.go` (ported
  `generateSyntax`/`generateSubtleSyntax`) lands here with Task S1.2 per the
  S0 package-layout note.
- `internal/tui/session_bench_test.go` — S1.9 extends it (re-render benchmark
  + budget gate; spec §4: 100 KB part re-render).
- `internal/tui/AGENTS.md` — the V1 wrap/scroll pins must not break.

## Detail pass (protocol)

One writing-plans pass, one subagent, `thinking=high`, dispatched by the
root session strictly sequentially (root principle 7) — per the Slice
Detail Protocol in plan.md: it fills this file (after this section) with the
full 5-step TDD detail for each task in the binding table (failing test
code, implementation code, gate, pinned commit), reading the named upstream
files at that moment. It commits as
`docs: TUI parity plan — detail S1 tasks` on its own bead
(`bd create "detail S1 plan tasks" --parent=yolo-oae.2 --json`).

## S1 slice gate (slice bead `yolo-oae.2`)

NOT a task bead; runs after all child beads close. Mirror the S0 slice gate
shape: (1) module gate `go vet ./... && go test ./...` + `gofmt -l .` empty
(incl. `TestImportsDirection` + the S1 teatest goldens); (2) user-run smoke
(NOT CI): in a real TTY, render a transcript with a fenced code block, a
table, a task list, and reasoning — theme-colored markdown, syntax-highlighted
code, and the reasoning/tool/error surfaces restyled; (3) append any forced
DEVIATIONS.md entries this slice named (with severity, same-commit rule —
root principle 2; spec §4: the transcript-fixture pty diff runs before the
slice closes — gaps become per-element `StyleConfig` overrides or a logged
custom renderer); (4) PROGRESS.md one-line status pointer; (5) commit
`docs: checkpoint — S1 done, next is S2 detail pass`; (6)
`bd close yolo-oae.2 --reason "all 9 child beads closed, gate green" --json`.
