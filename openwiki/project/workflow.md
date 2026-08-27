---
type: concept
title: Project Workflow
description: "How work is run in this repo: beads (bd) owns all task state, docs/superpowers holds verified facts and the deviation audit, the dependency allowlist is agent-proposable with user approval, and the non-negotiable core principles (zero telemetry, reference-not-contract, TUI purity, tests-define-the-contract) gate every change."
tags: [workflow, beads, superpowers, deviations, dependency-policy, core-principles, git-discipline]
sources:
  - id: openwiki-source-8037e2358a2c4f9b2c722a11
    resource: repo://AGENTS.md
  - id: openwiki-source-428f0670e8090e323a49a77d
    resource: repo://docs/superpowers/AGENTS.md
  - id: openwiki-source-4026c267912ddcc4848674b7
    resource: repo://docs/superpowers/DEVIATIONS.md
  - id: openwiki-source-88e030e7afe0cc3f8d8160b0
    resource: repo://docs/superpowers/PROGRESS.md
  - id: openwiki-source-75043e88a22f1cb5cae08c53
    resource: repo://scripts/wiki-stale.sh
  - id: openwiki-source-453611660ffbf02a66fa4bf3
    resource: repo://skills-lock.json
generated: {by: "opencode", at: "2026-08-27T15:27:54.907Z"}
verified:
  - by: openwiki/0.4.0
    at: 2026-08-27T15:27:54.907Z
---

# Project Workflow

This page describes the operating rules for agents and developers. It points at
the sources of truth — **beads** for task state and **`docs/superpowers/`** for
proven knowledge — and does **not** duplicate live task state (which changes
constantly). For "what's next," run `bd ready`; for proven facts, read
`docs/superpowers/PROGRESS.md`.

## Task state: beads (`bd`)

All task state lives in **beads**, not in markdown TODOs or files (root principle
6; `AGENTS.md` "Issue Tracking with bd"). The resume rail is: **beads (the
release epic; `bd ready`) → the active spec/plan → `PROGRESS.md` (facts) →
`DEVIATIONS.md` (audit)** (root "Key documents";
`docs/superpowers/AGENTS.md:5-8`). The workflow is: check `bd ready` for
unblocked work, claim an issue atomically (`bd update <id> --claim`), work it,
link discovered work with `discovered-from`, and close with a reason
(`AGENTS.md` "Workflow for AI Agents").

`PROGRESS.md` holds **only key verified facts + the root-cause archive + a
one-line status pointer to beads — never task history**. Task state is beads;
`git log --oneline` and the plan files are the archive
(`docs/superpowers/AGENTS.md:17-20`; `PROGRESS.md:1-6`).

## The deviation audit

`DEVIATIONS.md` is an **append-only, numbered, continuous** audit log
(`docs/superpowers/AGENTS.md:21-24`; `DEVIATIONS.md:1-8`). Per root principle 5
(*tests define the contract*): when the plan contradicts itself (or its own test
code is buggy), resolve per the last-stated call, fix the test, and log the
resolution here with severity. Items are never edited in place — supersede with a
new one. Items 1–66 (pre-v0.1.2) are frozen in
`deviations-archive-v0.1.0.md`.

## Plans, specs, and reviews layout

`docs/superpowers/` (`docs/superpowers/AGENTS.md:25-34`):

- `plans/<date>-<topic>.md` — dated implementation plans; **one active at a
  time**, named in the beads epic. Large slice-based plans use a directory
  `plans/<date>-<topic>/` instead: `plan.md` (master: global constraints, binding
  bead inventory, slice protocol) + one file per slice, each gated on its detail
  pass (see `2026-08-24-opencode-tui-parity/` as the reference).
- `specs/<date>-<topic>-design.md` — approved designs.
- `DEFERRED.md` — the living deferred work list (OPEN items only; closed findings
  are frozen in `deferred-archive.md`).
- `reviews/<version>/` — per-wave review findings (e.g. `v0.1.2/`).

## The non-negotiable core principles

`AGENTS.md` "Core principles (non-negotiable)" (lines 52-61):

1. **Zero telemetry** — no usage data ever sent to any remote server, no opt-in.
   Upstream telemetry surfaces (OTEL/OTLP exporter, OTel spans on LLM calls, the
   telemetry-identity field) are **skipped, not deferred**; `OTEL_*` env vars are
   inert and the config schema omits `experimental.openTelemetry`.
2. **Reference, not contract** — opencode v1.18.18 is a *reference*, not a binding
   contract; the legacy REST/SSE shapes stay mirrored for verifiability, but
   deviations are allowed on explicit user instruction and each is logged in
   `DEVIATIONS.md` (standing deviation: the `x-yolo-directory` header).
3. **Pins are change gates, not upstream locks** — the 14 session prompt files and
   tool `desc/*.txt` files are sha256-pinned in tests; an intentional change
   re-baselines the pin in the same commit, and a pin is never left dangling.
4. **TUI is a pure client** — non-test files under `internal/tui/` import only
   `internal/protocol` + `internal/tui/*` (+ stdlib/charm); enforced by
   `TestImportsDirection`.
5. **Tests define the contract** — plan contradictions resolve per the
   last-stated call, the test is fixed, and the resolution is logged with
   severity.
6. **Task state in beads; proven knowledge in docs** — task state is never
   duplicated in files; proven facts and logged deviations are written to
   `PROGRESS.md` / `DEVIATIONS.md` before moving on.
7. **Subagents one at a time** — never dispatch more than one subagent
   concurrently.
8. **YOLO spawns only YOLO** — if the root agent is `YOLO`, any subagent it spawns
   must also be `YOLO`.

## Dependency policy (allowlist + agent-proposable)

Runtime dependencies are **pinned at exact versions** behind an allowlist
(`AGENTS.md` "Project", line 14). Anything outside the allowlist requires an
agent **dependency proposal** *before* any `go get`/`go mod tidy`: the module +
exact version, with **live-verified evidence** (maintenance status, last
activity, license, transitive module surface — never recalled from memory), and a
checklist (actively maintained, pure Go/no cgo, permissive license, why stdlib or
hand-rolling is inadequate). **Landing requires explicit user approval**; an
approved dependency joins the allowlist and a `PROGRESS.md` fact.

## Commit and branch discipline

`AGENTS.md` "Commit & branch discipline" (lines 71-79):

- **NEVER commit directly onto `main`** — every change is committed on a task
  branch and reaches `main` only via a merged PR (branch → commit → push → PR →
  user merge).
- Work happens inline on the current task branch, **one task at a time**.
- Conventional commits (`feat:`/`fix:`/`docs:`/…, imperative, ≤72-char subject);
  task commits use **the commit message pinned in the plan**.
- **Tags only with explicit user go-ahead**; versioning is semantic
  (`MAJOR.MINOR.PATCH`).

## Verification gate

Every task ends with the module-root gate green **and a commit**:
`go vet ./... && go test ./...`, plus a clean `gofmt -l .`
(`AGENTS.md` "Commands & verification", line 65). Unit tests never hit the
network — live paths are env-gated (`YOLO_LLM=fake`), and the e2e smoke is
user-run, never CI.

### OpenWiki wiki gate (pre-merge)

The generated `openwiki/` evidence index must be current before a milestone
merges: `just wiki-stale` (`scripts/wiki-stale.sh`) compares the `gitHead`
recorded in `openwiki/.last-update.json` to `HEAD` and exits 1 when the wiki
trails the source it documents. The refresh is a host-driven update through
the `openwiki` skill (MCP `openwiki_begin` mode=update) — there is **no
scheduled CI workflow** (a GitHub runner cannot reach the local model). The
gate is wired into the root `AGENTS.md` "Superpowers workflow" as the first
step of the pre-merge `requesting-code-review` row; generated pages are never
hand-edited (source changes propagate via the update).

## Skills

The repo carries a **hash-locked skill set** in `.agents/skills/`, pinned by
`skills-lock.json` (`AGENTS.md` "Golang skills", line 111): 15 golang/charm
skills (from `samber/cc-skills-golang` + `yurifrl/cly`) plus the bd-managed
`beads` skill. The relevant skill(s) are invoked per task; the golang skills are
**not edited** (they are pinned by hash), and `.agents/skills/beads/` is
bd-managed.

## Execution discipline

Per the active plan, execution is **strict 5-step TDD per task**: (1) write a
failing test → (2) confirm it FAILs → (3) minimal implementation → (4) the gate
green → (5) commit with the plan's pinned message
(`AGENTS.md` "Superpowers workflow", line 89; `docs/superpowers/AGENTS.md:37-42`).
