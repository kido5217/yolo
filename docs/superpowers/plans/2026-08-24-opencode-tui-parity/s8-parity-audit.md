# S8 — parity audit + close-out (slice bead `yolo-oae.9`)

The parity audit + close-out: the deterministic mock-SSE capture runtime,
the upstream pty-capture fixtures, the per-surface diff sweep, close or
log every visible gap, re-baselined goldens, and the PROGRESS verified
fact.

**State: detail pass pending** — execution of this slice is GATED on the
detail pass below (Slice Detail Protocol rule 2): no task may start until
this file holds full 5-step TDD detail for all of its tasks.

## Binding task table

Pointer only (FROZEN — Slice Detail Protocol rule 1): `plan.md` →
`## Task inventory` → `### S8 — parity audit + close-out (slice bead yolo-oae.9)`.
Bead titles, scope, and pinned commit messages live there and may not be
changed by a detail pass; any required change = STOP + explicit user
approval + re-record in plan.md.

## Dep gate

None (Go). The npm package `opencode-ai@1.18.18` + the node toolchain is a
dev-only capture runtime — NOT a Go dependency, NOT a dep-proposal target:
the parity scripts live under `scripts/` and their fixtures under a testdata
dir (the root dependency policy governs Go modules only).

## Exact upstream sources (the detail pass reads these AT DETAIL TIME)

`/tmp/opencode-upstream` @ v1.18.18 — no upstream source to port; the
parity REFERENCE is:

- the npm package `opencode-ai@1.18.18` — S8.2 pty-capture runs it
  (`opencode serve` against the S8.1 mock SSE server, a scripted key
  sequence, volatile bits stripped, pinned fixtures).
- the upstream tree — behavior reference for the diff-sweep judgment (which
  surface, which expectation).
- the spec's binding methodology: §7 (Verification strategy, item 3 — the
  upstream pty-capture reference: mock SSE → scripted pty → pinned
  fixtures → diff; every visible mismatch that can't be closed becomes a
  DEVIATIONS.md entry with severity) + §9 (Risks & open items — the
  mitigations the sweep must honor, e.g. risk 2's per-dialog huh
  deviations).
- S8.1 mock SSE server: env-gated, local-only, deterministic canned stream
  (root AGENTS.md: unit tests never hit the network — the mock is
  in-process).

## yolo anchors

- `internal/server/testutil` — the test-server scaffolding the mock SSE
  server builds on.
- `scripts/e2e-live.sh` — the existing on-demand live script; the parity
  scripts follow its user-run, never-CI pattern.
- the teatest goldens landed in S1–S7 — re-baselined in S8.4.
- `docs/superpowers/DEVIATIONS.md` — S8.4: close or log every visible gap,
  with severity.
- `docs/superpowers/PROGRESS.md` — S8.5: the verified fact.

## Detail pass (protocol)

One writing-plans pass, one subagent, `thinking=high`, dispatched by the
root session strictly sequentially (root principle 7) — per the Slice
Detail Protocol in plan.md: it fills this file (after this section) with the
full 5-step TDD detail for each task in the binding table (failing test
code, implementation code, gate, pinned commit), reading the named upstream
files at that moment. It commits as
`docs: TUI parity plan — detail S8 tasks` on its own bead
(`bd create "detail S8 plan tasks" --parent=yolo-oae.9 --json`).

## S8 slice gate (slice bead `yolo-oae.9`)

NOT a task bead; runs after all child beads close. Mirror the S0 slice gate
shape: (1) module gate `go vet ./... && go test ./...` + `gofmt -l .` empty
(incl. `TestImportsDirection` + the re-baselined teatest goldens);
(2) user-run smoke (NOT CI): run the parity capture + diff sweep under a
TRUECOLOR terminal (deviation 125: the upstream SGR is always 24-bit, so
the capture TERM must be truecolor for a comparable diff); (3) append any
forced DEVIATIONS.md entries this slice named (with severity, same-commit
rule — root principle 2); (4) PROGRESS.md one-line status pointer;
(5) commit `docs: checkpoint — S8 done, epic close pending user go-ahead`;
(6) `bd close yolo-oae.9 --reason "all 5 child beads closed, gate green,
parity sweep logged" --json` + the beads-export commit
`chore: beads export (yolo-oae.9 closed)`. The epic close (`bd close
yolo-oae`) and any tag are S8.5 scope — ONLY on explicit user go-ahead
(root: tags only with explicit user go-ahead; semantic versioning).
