# S7 — session completion (slice bead `yolo-oae.8`)

Complete the session route: the todo sidebar (latest `todowrite` part →
status-glyph list, keymap toggle), the full-message dialog, and the
session footer detail restyle.

**State: detail pass pending** — execution of this slice is GATED on the
detail pass below (Slice Detail Protocol rule 2): no task may start until
this file holds full 5-step TDD detail for all of its tasks.

## Binding task table

Pointer only (FROZEN — Slice Detail Protocol rule 1): `plan.md` →
`## Task inventory` → `### S7 — session completion (slice bead yolo-oae.8)`.
Bead titles, scope, and pinned commit messages live there and may not be
changed by a detail pass; any required change = STOP + explicit user
approval + re-record in plan.md.

## Dep gate

None.

## Exact upstream sources (the detail pass reads these AT DETAIL TIME)

`/tmp/opencode-upstream` @ v1.18.18:

- `packages/tui/src/feature-plugins/sidebar/todo.tsx` +
  `packages/tui/src/component/todo-item.tsx` — the todo sidebar: latest
  `todowrite` part → status-glyph list (S7.1) + keymap toggle + layout
  (S7.2).
- `packages/tui/src/routes/session/sidebar.tsx` — the session sidebar
  layout.
- `packages/tui/src/routes/session/dialog-message.tsx` — the
  full-message view (S7.3).
- `packages/tui/src/routes/session/footer.tsx` — the session footer detail:
  model/agent/tokens/cost/spinner/connection, status dots 70–84 (S7.4 —
  S0.10 already themed the conn segment tokens; S7.4 refines the whole
  session footer).
- `packages/tui/src/routes/session/subagent-footer.tsx` — read for context
  (no yolo subagent surface; note it as out of scope unless the detail
  pass finds a contract-backed analog).

## yolo anchors

- `internal/tui/session.go` — the session route surface (sidebar + message
  view).
- `internal/tui/footer.go` — the session footer (S7.4).
- `internal/tui/dialog.go` — the dialog surface for the full-message view
  (S7.3).
- the S4 keymap registry — the S7.2 sidebar toggle binding.
- `internal/protocol/` — the `todowrite` part DTO — verify the wire shape
  carries enough for the status list (if not: spec §10 — no wire changes in
  this epic; the detail pass logs a deviation instead).

## Detail pass (protocol)

One writing-plans pass, one subagent, `thinking=high`, dispatched by the
root session strictly sequentially (root principle 7) — per the Slice
Detail Protocol in plan.md: it fills this file (after this section) with the
full 5-step TDD detail for each task in the binding table (failing test
code, implementation code, gate, pinned commit), reading the named upstream
files at that moment. It commits as
`docs: TUI parity plan — detail S7 tasks` on its own bead
(`bd create "detail S7 plan tasks" --parent=yolo-oae.8 --json`).

## S7 slice gate (slice bead `yolo-oae.8`)

NOT a task bead; runs after all child beads close. Mirror the S0 slice gate
shape: (1) module gate `go vet ./... && go test ./...` + `gofmt -l .` empty
(incl. `TestImportsDirection` + the S7 teatest goldens); (2) user-run smoke
(NOT CI): in a real TTY — the todo sidebar (toggle + the status-glyph list
for a session with a `todowrite` part), the full-message dialog, and the
session footer segments (model/agent/tokens/cost/spinner/connection);
(3) append any forced DEVIATIONS.md entries this slice named (with
severity, same-commit rule — root principle 2); (4) PROGRESS.md one-line
status pointer; (5) commit
`docs: checkpoint — S7 done, next is S8 detail pass`; (6)
`bd close yolo-oae.8 --reason "all 4 child beads closed, gate green" --json`
+ the beads-export commit `chore: beads export (yolo-oae.8 closed)`.
