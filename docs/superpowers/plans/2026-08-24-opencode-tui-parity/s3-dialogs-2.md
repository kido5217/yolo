# S3 — remaining contract-backed dialogs (slice bead `yolo-oae.4`)

Land the remaining contract-backed dialogs — session-list/rename/
delete-failed, provider, status, help, retry-action, theme-list — plus the
theme mode switch/lock keybinds wired to the S0 KV.

**State: detail pass pending** — execution of this slice is GATED on the
detail pass below (Slice Detail Protocol rule 2): no task may start until
this file holds full 5-step TDD detail for all of its tasks.

## Binding task table

Pointer only (FROZEN — Slice Detail Protocol rule 1): `plan.md` →
`## Task inventory` → `### S3 — remaining contract-backed dialogs (slice bead yolo-oae.4)`.
Bead titles, scope, and pinned commit messages live there and may not be
changed by a detail pass; any required change = STOP + explicit user
approval + re-record in plan.md.

## Dep gate

None — `huh` + `sahilm/fuzzy` already landed via the S2 gate; the S3
dialogs reuse the S2 select + themed huh input primitives.

## Exact upstream sources (the detail pass reads these AT DETAIL TIME)

`/tmp/opencode-upstream` @ v1.18.18:

- `packages/tui/src/component/dialog-session-list.tsx` — S3.1, session-list
  dialog.
- `packages/tui/src/component/dialog-session-rename.tsx` — S3.2, rename
  (uses the themed huh input from S2.4).
- `packages/tui/src/component/dialog-session-delete-failed.tsx` — S3.3,
  delete-failed dialog.
- `packages/tui/src/component/dialog-provider.tsx` — S3.4, provider
  restyle.
- `packages/tui/src/component/dialog-status.tsx` — S3.5, status dialog
  (content contract read from upstream at detail time — spec §9 open
  items).
- `packages/tui/src/component/dialog-retry-action.tsx` — S3.7, retry-action
  (semantics read from upstream at detail time — spec §9 open items).
- `packages/tui/src/component/dialog-theme-list.tsx` — S3.8, theme-list
  (select over `theme.All()`; S3.9 wires KV + mode keybinds).
- `packages/tui/src/ui/dialog-help.tsx` — S3.6, help dialog; note it is
  keymap-registry-driven per the S4 handoff: S4.7 completes the
  registry-driven rendering of /help (frozen S4 table row); the S3.6
  detail pass defines the exact consumption contract at detail time.
- `packages/tui/src/context/kv.tsx` — the upstream KV store (the port
  reference for S3.9 mode switch/lock semantics on top of yolo's S0.7 KV
  file + Engine).
- `packages/tui/src/keymap.tsx` — the keybinds for theme mode switch/lock
  (S3.9).

## yolo anchors

- `internal/tui/dialog.go` — the existing dialog surface extended here.
- `internal/tui/commands.go` + `internal/tui/view.go` — the existing
  provider/model/status surfaces (verified 2026-08-25: there is no
  `menu.go` in the tree; the surfaces live here).
- `internal/tui/theme/` — `theme.All()`, Engine Set/Pin/Free, and the KV
  file (S0.7) for the theme-list + mode wiring.
- `internal/protocol/` — command DTOs, if needed.
- `internal/config/` — the config-side `theme` string + profile surface.

## Detail pass (protocol)

One writing-plans pass, one subagent, `thinking=high`, dispatched by the
root session strictly sequentially (root principle 7) — per the Slice
Detail Protocol in plan.md: it fills this file (after this section) with the
full 5-step TDD detail for each task in the binding table (failing test
code, implementation code, gate, pinned commit), reading the named upstream
files at that moment. It commits as
`docs: TUI parity plan — detail S3 tasks` on its own bead
(`bd create "detail S3 plan tasks" --parent=yolo-oae.4 --json`).

## S3 slice gate (slice bead `yolo-oae.4`)

NOT a task bead; runs after all child beads close. Mirror the S0 slice gate
shape: (1) module gate `go vet ./... && go test ./...` + `gofmt -l .` empty
(incl. `TestImportsDirection` + the S3 teatest goldens); (2) user-run smoke
(NOT CI): in a real TTY, open session-list, rename a session, status, help,
retry-action, and theme-list (select a theme; switch/lock the mode with the
new keybinds); (3) append any forced DEVIATIONS.md entries this slice named
(with severity, same-commit rule — root principle 2); (4) PROGRESS.md
one-line status pointer; (5) commit
`docs: checkpoint — S3 done, next is S4 detail pass`; (6)
`bd close yolo-oae.4 --reason "all 9 child beads closed, gate green" --json`.
