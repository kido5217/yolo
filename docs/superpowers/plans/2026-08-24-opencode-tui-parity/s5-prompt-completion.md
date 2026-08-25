# S5 — prompt completion + attention (slice bead `yolo-oae.6`)

Complete the prompt: ↑/↓ history recall with KV persistence, the ported
frecency ranking, the @-file and /-command autocomplete pickers, and the
terminal bell on turn completion/error.

**State: detail pass pending** — execution of this slice is GATED on the
detail pass below (Slice Detail Protocol rule 2): no task may start until
this file holds full 5-step TDD detail for all of its tasks.

## Binding task table

Pointer only (FROZEN — Slice Detail Protocol rule 1): `plan.md` →
`## Task inventory` → `### S5 — prompt completion + attention (slice bead yolo-oae.6)`.
Bead titles, scope, and pinned commit messages live there and may not be
changed by a detail pass; any required change = STOP + explicit user
approval + re-record in plan.md.

## Dep gate

None — the @-picker's fuzzy filter reuses `sahilm/fuzzy` from the S2 gate.

## Exact upstream sources (the detail pass reads these AT DETAIL TIME)

`/tmp/opencode-upstream` @ v1.18.18:

- `packages/tui/src/component/prompt/index.tsx` — the prompt view: history
  wiring, autocomplete trigger, cursor (S0.10 already themed the cursor;
  read for the wiring only).
- `packages/tui/src/component/prompt/history.tsx` +
  `packages/tui/src/prompt/history.tsx` — recall + persistence
  (S5.1/S5.2).
- `packages/tui/src/component/prompt/frecency.tsx` +
  `packages/tui/src/prompt/frecency.tsx` — the scoring algorithm (S5.3 —
  port the scoring verbatim).
- `packages/tui/src/component/prompt/autocomplete.tsx` — the @-file +
  /-command pickers (S5.4/S5.5; the /-picker reads GET /command; spec §9
  open item: check here whether the file walk honors `.gitignore`).
- `packages/tui/src/feature-plugins/system/notifications.ts` — the bell
  conditions (S5.6 — port the CONDITIONS, not the audio).

## yolo anchors

- `internal/tui/prompt.go` — the prompt view (exists; verified 2026-08-25)
  + its tests.
- `internal/tui/theme/` — the S0.7 KV file: the prompt history persistence
  target.
- `internal/protocol/` — GET /command for the /-picker.
- workspace file access for the @-picker — read-only, path-validated (no
  new dep; `internal/tui` stays import-pure — root principle 4).

## Detail pass (protocol)

One writing-plans pass, one subagent, `thinking=high`, dispatched by the
root session strictly sequentially (root principle 7) — per the Slice
Detail Protocol in plan.md: it fills this file (after this section) with the
full 5-step TDD detail for each task in the binding table (failing test
code, implementation code, gate, pinned commit), reading the named upstream
files at that moment. It commits as
`docs: TUI parity plan — detail S5 tasks` on its own bead
(`bd create "detail S5 plan tasks" --parent=yolo-oae.6 --json`).

## S5 slice gate (slice bead `yolo-oae.6`)

NOT a task bead; runs after all child beads close. Mirror the S0 slice gate
shape: (1) module gate `go vet ./... && go test ./...` + `gofmt -l .` empty
(incl. `TestImportsDirection` + the S5 teatest goldens); (2) user-run smoke
(NOT CI): in a real TTY — ↑/↓ recall across a restart (KV persistence), the
@-file picker, the /-command picker, and the bell on turn completion/error;
(3) append any forced DEVIATIONS.md entries this slice named (with
severity, same-commit rule — root principle 2); (4) PROGRESS.md one-line
status pointer; (5) commit
`docs: checkpoint — S5 done, next is S6 detail pass`; (6)
`bd close yolo-oae.6 --reason "all 6 child beads closed, gate green" --json`.
