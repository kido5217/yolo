# S6 — home completion (slice bead `yolo-oae.7`)

Complete the home route: the startup spinner, the rotating sha256-pinned
tips, the session-destination view, and the footer key hints rendered from
the S4 keymap registry.

**State: detail pass pending** — execution of this slice is GATED on the
detail pass below (Slice Detail Protocol rule 2): no task may start until
this file holds full 5-step TDD detail for all of its tasks.

## Binding task table

Pointer only (FROZEN — Slice Detail Protocol rule 1): `plan.md` →
`## Task inventory` → `### S6 — home completion (slice bead yolo-oae.7)`.
Bead titles, scope, and pinned commit messages live there and may not be
changed by a detail pass; any required change = STOP + explicit user
approval + re-record in plan.md.

## Dep gate

None.

## Exact upstream sources (the detail pass reads these AT DETAIL TIME)

`/tmp/opencode-upstream` @ v1.18.18:

- `packages/tui/src/routes/home.tsx` — the home route: startup state +
  session list (S6.1/S6.4 context).
- `packages/tui/src/routes/home/session-destination.tsx` — S6.4.
- `packages/tui/src/feature-plugins/home/tips.tsx` — the tips DATA (S6.2 —
  sha256-pinned port, root principle 3).
- `packages/tui/src/feature-plugins/home/tips-view.tsx` — rotation cadence
  + rendering (S6.3).
- `packages/tui/src/component/startup-loading.tsx` — the spinner (S6.1).
- `packages/tui/src/feature-plugins/home/footer.tsx` — the footer key
  hints (S6.5 — S0.9 already ported the token map; S6.5 renders hints FROM
  the S4 keymap registry — cross-slice dependency: S4.1–S4.2 must be
  closed first; the strict slice order (S6 runs only after S4 closes)
  satisfies it).

## yolo anchors

- `internal/tui/home.go` — the home route surface.
- `internal/tui/footer.go` — the footer key-hints surface (S6.5).
- the S4 keymap registry — cross-slice: S6.5 renders the hints from it.
- `internal/protocol/` — the sessions DTO for the destination view.

## Detail pass (protocol)

One writing-plans pass, one subagent, `thinking=high`, dispatched by the
root session strictly sequentially (root principle 7) — per the Slice
Detail Protocol in plan.md: it fills this file (after this section) with the
full 5-step TDD detail for each task in the binding table (failing test
code, implementation code, gate, pinned commit), reading the named upstream
files at that moment. It commits as
`docs: TUI parity plan — detail S6 tasks` on its own bead
(`bd create "detail S6 plan tasks" --parent=yolo-oae.7 --json`).

## S6 slice gate (slice bead `yolo-oae.7`)

NOT a task bead; runs after all child beads close. Mirror the S0 slice gate
shape: (1) module gate `go vet ./... && go test ./...` + `gofmt -l .` empty
(incl. `TestImportsDirection` + the S6 teatest goldens; the S6.2 tips sha256
pin must not be dangling — root principle 3); (2) user-run smoke (NOT CI):
on home in a real TTY — the startup spinner during hydration, the tips
rotating on cadence, the footer key hints, and the session-destination
view; (3) append any forced DEVIATIONS.md entries this slice named (with
severity, same-commit rule — root principle 2); (4) PROGRESS.md one-line
status pointer; (5) commit
`docs: checkpoint — S6 done, next is S7 detail pass`; (6)
`bd close yolo-oae.7 --reason "all 5 child beads closed, gate green" --json`.
