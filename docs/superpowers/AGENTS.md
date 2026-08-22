# AGENTS.md — docs/superpowers (process memory)

## Purpose

The process tree: what the current session is doing, which plan + spec
define the work, and where review findings land. `PROGRESS.md` is the
single resume rail — read first by every new session (root "Key files").

## Ownership

`PROGRESS.md`, `plans/`, `specs/`, `reviews/`,
`deviations-archive-v0.1.0.md`.

## Local Contracts

- `PROGRESS.md` is a rolling checkpoint, never a diary: active task (full
  detail), one-line last-completed, open items, key verified facts,
  append-only deviation log. Keep it small — `git log --oneline` and the
  plan files are the archive. Roll it before moving on after any
  checkpoint, plan/spec change, or deviation (root principle 6).
- Deviation log: append-only, numbered, continuous across the archive
  (items 1–66 frozen in `deviations-archive-v0.1.0.md`); plan
  contradictions resolve per "tests define the contract" (root principle
  5) and the resolution is logged with severity.
- Layout: `plans/<date>-<topic>.md` = dated implementation plans (one
  active, named in PROGRESS.md); `specs/<date>-<topic>-design.md` =
  approved designs; `reviews/<version>/` = per-wave review findings
  (e.g. `v0.1.2/`).
- Subagents strictly one at a time (root principle 7).

## Work Guidance

- Read the active plan's Resume Protocol, then ONLY the active task slice.
- Strict 5-step TDD per plan task: failing test → confirm FAIL → minimal
  implementation → gate green → commit with the plan's pinned message.

## Verification

## Child DOX Index

- None.
