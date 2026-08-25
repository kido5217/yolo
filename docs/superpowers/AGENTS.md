# AGENTS.md — docs/superpowers (process memory)

## Purpose

Process memory: durable verified facts, the deviation audit, the 0.3.0 work
list, and the plans/specs/reviews layout. Task state lives in beads (release
epic; `bd ready`) — the resume rail is beads → active spec/plan →
`PROGRESS.md` (facts) → `DEVIATIONS.md` (audit) (root "Key documents").

## Ownership

`PROGRESS.md`, `DEVIATIONS.md`, `DEFERRED.md`, `plans/`, `specs/`,
`reviews/`, `deviations-archive-v0.1.0.md`, `deferred-archive.md`.

## Local Contracts

- `PROGRESS.md` holds only key verified facts + the root-cause archive + a
  one-line status pointer to beads — never task history. Task state is
  beads; `git log --oneline` and the plan files are the archive (root
  principle 6).
- Deviation log (`DEVIATIONS.md`): append-only, numbered, continuous across
  the archive (items 1–66 frozen in `deviations-archive-v0.1.0.md`); plan
  contradictions resolve per "tests define the contract" (root principle 5)
  and the resolution is logged with severity.
- Layout: `plans/<date>-<topic>.md` = dated implementation plans (one
  active, named in the beads epic); large slice-based plans use a directory
  `plans/<date>-<topic>/` instead — `plan.md` (master: global constraints,
  binding bead inventory, slice detail protocol) + one file per slice
  (`s0-…md` fully detailed, `s1-…md`… briefs gated on a detail pass; see
  `2026-08-24-opencode-tui-parity/` as the reference); `specs/<date>-<topic>-design.md` =
  approved designs; `DEFERRED.md` = living 0.3.0 work list (OPEN items only;
  closed v0.1.2-review findings frozen in `deferred-archive.md`);
  `reviews/<version>/` = per-wave review findings (e.g. `v0.1.2/`).
- Subagents strictly one at a time (root principle 7).

## Work Guidance

- Read the active plan's Resume Protocol, then ONLY the active task slice.
- Strict 5-step TDD per plan task: failing test → confirm FAIL → minimal
  implementation → gate green → commit with the plan's pinned message.

## Verification

## Child DOX Index

- None.
