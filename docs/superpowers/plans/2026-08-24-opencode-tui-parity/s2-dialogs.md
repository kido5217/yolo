# S2 — dialog system (slice bead `yolo-oae.3`)

Port opencode's dialog system — the modal stack, the huh field dialogs
(alert/confirm/input), and the select component — then restyle the
permission, model, and agent dialogs on top of it.

**State: detail pass pending** — execution of this slice is GATED on the
detail pass below (Slice Detail Protocol rule 2): no task may start until
this file holds full 5-step TDD detail for all of its tasks.

## Binding task table

Pointer only (FROZEN — Slice Detail Protocol rule 1): `plan.md` →
`## Task inventory` → `### S2 — dialog system (slice bead yolo-oae.3)`.
Bead titles, scope, and pinned commit messages live there and may not be
changed by a detail pass; any required change = STOP + explicit user
approval + re-record in plan.md.

## Dep gate

`charm.land/huh/v2` v2.0.3 + `github.com/sahilm/fuzzy` v0.1.3 (tasks S2.1) —
dep-proposal bead first (root AGENTS.md dependency policy: evidence from
live web search — maintenance, license, pure Go, transitive surface;
approval gate = STOP before `go get`; both modules land as task S2.1).

## Exact upstream sources (the detail pass reads these AT DETAIL TIME)

`/tmp/opencode-upstream` @ v1.18.18:

- `packages/tui/src/ui/dialog.tsx` — the modal stack: centered overlay,
  focus capture, esc, stackable, result callback.
- `packages/tui/src/ui/dialog-select.tsx` — the select core: `Option`
  732–791, the active-row box 667–678, the fuzzy filter, categories,
  actions, footer hints, scroll acceleration (S0.9 already ported the home
  LIST row tokens — S2 ports the component itself).
- `packages/tui/src/ui/dialog-alert.tsx`,
  `packages/tui/src/ui/dialog-confirm.tsx`,
  `packages/tui/src/ui/dialog-prompt.tsx` — the field dialogs
  (huh-equivalents).
- `packages/tui/src/component/dialog-model.tsx` (S2.9),
  `packages/tui/src/component/dialog-agent.tsx` (S2.10) — the restyle
  sources.
- `packages/tui/src/routes/session/permission.tsx` — the S2.8 permission
  dialog on the select stack.

## yolo anchors

- `internal/tui/dialog.go` — the existing dialog surface; its
  `title`/`divider` statics in `internal/tui/style.go` get consumed here
  (statics yield to theme tokens).
- `internal/tui/app.go` — overlay/layer composition for the modal stack.
- `internal/tui/theme/` — theming via `StyleConfig` from the resolved theme
  tokens.
- bubbles v2 + the S2 dep gate (`huh`/`sahilm/fuzzy`) — the new interactive
  primitives.

## Detail pass (protocol)

One writing-plans pass, one subagent, `thinking=high`, dispatched by the
root session strictly sequentially (root principle 7) — per the Slice
Detail Protocol in plan.md: it fills this file (after this section) with the
full 5-step TDD detail for each task in the binding table (failing test
code, implementation code, gate, pinned commit), reading the named upstream
files at that moment. It commits as
`docs: TUI parity plan — detail S2 tasks` on its own bead
(`bd create "detail S2 plan tasks" --parent=yolo-oae.3 --json`).

## S2 slice gate (slice bead `yolo-oae.3`)

NOT a task bead; runs after all child beads close. Mirror the S0 slice gate
shape: (1) module gate `go vet ./... && go test ./...` + `gofmt -l .` empty
(incl. `TestImportsDirection` + the S2 teatest goldens); (2) user-run smoke
(NOT CI): in a real TTY, cycle the permission/model/agent dialogs and one
select (filter, categories, footer hints, scroll acceleration); (3) append
any forced DEVIATIONS.md entries this slice named (with severity,
same-commit rule — root principle 2; spec §9 risk 2: huh's look deviates
from the upstream borderless pills, so per-dialog deviations are expected
and unrecognizable dialogs get hand-rolled versions, logged); (4)
PROGRESS.md one-line status pointer; (5) commit
`docs: checkpoint — S2 done, next is S3 detail pass`; (6)
`bd close yolo-oae.3 --reason "all 10 child beads closed, gate green" --json`.
