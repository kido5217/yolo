# S4 — keymap + command palette + which-key (slice bead `yolo-oae.5`)

Land the keymap registry (upstream defaults, per-context groups, runtime
remap), the `yolo.jsonc` keybinds schema, the command palette, and the
which-key overlay — the single source for every TUI binding.

**State: detail pass pending** — execution of this slice is GATED on the
detail pass below (Slice Detail Protocol rule 2): no task may start until
this file holds full 5-step TDD detail for all of its tasks.

## Binding task table

Pointer only (FROZEN — Slice Detail Protocol rule 1): `plan.md` →
`## Task inventory` → `### S4 — keymap + command palette + which-key (slice bead yolo-oae.5)`.
Bead titles, scope, and pinned commit messages live there and may not be
changed by a detail pass; any required change = STOP + explicit user
approval + re-record in plan.md.

## Dep gate

None — the palette's fuzzy filter reuses `sahilm/fuzzy` from the S2 gate.

## Exact upstream sources (the detail pass reads these AT DETAIL TIME)

`/tmp/opencode-upstream` @ v1.18.18:

- `packages/tui/src/config/keybind.ts` — the upstream default bindings
  (S4.1 registry data; binding value shapes: string | keystroke object |
  array | `false`/`"none"` per S4.3).
- `packages/tui/src/keymap.tsx` — per-context groups + runtime remap
  (S4.2).
- `packages/tui/src/component/command-palette.tsx` — the palette overlay +
  fuzzy + run (S4.4/S4.5).
- `packages/tui/src/feature-plugins/system/which-key.tsx` — the pending
  prefix-group overlay (S4.6/S4.7).

## yolo anchors

- `internal/tui/app.go` — `Update`: the key-handling hub; the S4 registry
  becomes the single source for all bindings.
- `internal/tui/keys.go` — the existing key-constant surface the registry
  replaces.
- `internal/tui/AGENTS.md` — the V1 keymap pins (pgup/pgdn, `\`+enter) must
  survive.
- `internal/protocol/` — the GET /command DTO the palette lists over.
- `internal/config/` — the `yolo.jsonc` keybinds schema (S4.3).

## Detail pass (protocol)

One writing-plans pass, one subagent, `thinking=high`, dispatched by the
root session strictly sequentially (root principle 7) — per the Slice
Detail Protocol in plan.md: it fills this file (after this section) with the
full 5-step TDD detail for each task in the binding table (failing test
code, implementation code, gate, pinned commit), reading the named upstream
files at that moment. It commits as
`docs: TUI parity plan — detail S4 tasks` on its own bead
(`bd create "detail S4 plan tasks" --parent=yolo-oae.5 --json`).

## S4 slice gate (slice bead `yolo-oae.5`)

NOT a task bead; runs after all child beads close. Mirror the S0 slice gate
shape: (1) module gate `go vet ./... && go test ./...` + `gofmt -l .` empty
(incl. `TestImportsDirection` + the S4 teatest goldens; the V1 keymap pins
green); (2) user-run smoke (NOT CI): in a real TTY — remap a binding via
`yolo.jsonc` and via the runtime remap, open the command palette (filter +
run), and hold a key prefix to see the which-key overlay; (3) append any
forced DEVIATIONS.md entries this slice named (with severity, same-commit
rule — root principle 2; spec §9 risk 5: the fuzzy ranking order is checked
in the S8 diff, small scoring tweak if visibly off); (4) PROGRESS.md
one-line status pointer; (5) commit
`docs: checkpoint — S4 done, next is S5 detail pass`; (6)
`bd close yolo-oae.5 --reason "all 7 child beads closed, gate green" --json`
+ the beads-export commit `chore: beads export (yolo-oae.5 closed)`.
