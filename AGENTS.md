# AGENTS.md — agent instructions for the yolo repo

This file is loaded by any agent (opencode, yolo itself, …) working in this repo.
Read it before acting. For session state, read `docs/superpowers/PROGRESS.md` second.

## Project

**yolo** is a faithful Go port of [opencode](https://github.com/anomalyco/opencode) **v1.18.18** — TUI + core server only (web/desktop/slack/console dropped). Purpose: test the capabilities of **Qwen3.8-27B** on a single RTX 5090 behind `https://ai.kido.ws/v1`.

- Module `github.com/kido5217/yolo`, binary `yolo`, Go ≥ 1.25 (installed 1.26.5).
- Single binary: starts the core HTTP server (REST + SSE) **in-process**, then runs the bubbletea v2 TUI which talks to it **only** via the wire contract.
- Core layering: `protocol` (wire DTOs, single source of truth) → `server` → `session` (agent loop) → `llm` / `provider` / `tool` / `permission` / `config` / `auth` / `storage` / `bus`.
- Pinned deps, exact versions, **nothing else**: `charm.land/bubbletea/v2` v2.0.8, `charm.land/lipgloss/v2` v2.0.6, `charm.land/bubbles/v2` v2.1.1, `modernc.org/sqlite` v1.56.0 (pure Go, no cgo), `tidwall/jsonc` v0.3.3; dev-only `github.com/charmbracelet/x/exp/teatest/v2` v2.0.0-20260816001655-68d539dca504.

## Key files (read order)

| File | What it is |
|---|---|
| `docs/superpowers/PROGRESS.md` | **Read first on resume.** Active task, resume instructions, plan deviations, key verified facts (do not re-litigate) |
| `docs/superpowers/plans/` | Dated implementation plans; one active at a time (named in `PROGRESS.md` → Resume instructions). **Read ONLY the active task slice** |
| `docs/superpowers/specs/` | Dated approved designs; the active one is named in `PROGRESS.md`. Zero-telemetry statement lives in the v1 design's §1 |
| `/tmp/opencode-upstream` | Upstream reference clone at tag `v1.18.18`. If missing: `git clone --depth 1 --branch v1.18.18 https://github.com/anomalyco/opencode /tmp/opencode-upstream`. **Never touch `/tmp/opencode`** — pre-existing user data |
| `.agents/skills/` + `skills-lock.json` | 15 hash-locked golang skills (samber/cc-skills-golang) — see Skills below |

## Core principles (non-negotiable)

1. **Zero telemetry.** yolo runs on the end user's machine and must contain zero telemetry: no usage data ever sent to any remote server, no opt-in telemetry. Upstream telemetry surfaces are **skipped, not deferred**: OTEL/OTLP exporter (`packages/core/src/observability/otlp.ts`), OpenTelemetry spans on LLM calls (`experimental.openTelemetry` / `experimental_telemetry` in `packages/opencode/src/session/llm.ts`), telemetry-identity field. `OTEL_*` env vars are inert; the ported config schema omits `experimental.openTelemetry`. Full statement: spec §1.
2. **Faithful port, one deliberate wire deviation.** Legacy REST paths/JSON shapes and the legacy SSE event set are mirrored so the port is verifiable against opencode's OpenAPI contract. The single deviation: scoping header is **`x-yolo-directory`** (upstream: `x-opencode-directory`).
3. **Verbatim pins are tests, not decoration.** 14 session prompt files and all tool `desc/*.txt` files are ported **byte-verbatim** from upstream and guarded by sha256 pins in tests. Do not "improve", rewrap, or reword pinned text.
4. **TUI is a pure client.** Non-test files under `internal/tui/` import only `internal/protocol` + `internal/tui/*` (+ stdlib/charm deps). `_test.go` may use `internal/server/testutil` (escape hatch). Enforced by Task 29.
5. **Tests define the contract.** When the plan contradicts itself (or its own test code is buggy), resolve per the last-stated call, fix the test, and **log the deviation in `PROGRESS.md` → "Plan deviations logged"** with severity.
6. **`PROGRESS.md` is never stale.** After any edit, plan, spec, design decision, deviation, or checkpoint — anything that changes what the next session must know — roll `docs/superpowers/PROGRESS.md` before moving on. It is the single thing a future session reads to resume; a stale one is a broken resume. What it holds and how to keep it small: "Commit & branch discipline."
7. **Subagents one at a time.** Never dispatch more than one subagent concurrently (via the `task` tool): dispatch one, wait for it to fully return, then dispatch the next. This supersedes any plan/spec wording permitting parallel subagents (the v0.1.2 plan/spec "≤3 parallel" text was revised 2026-08-20; see PROGRESS.md deviation log).

## Commands & verification

- **The CI gate (run at module root):** `go vet ./... && go test ./...` — **every task ends with both green and a commit.**
- **Formatting (run at module root after every completed step, before the commit):** gofmt from the **Go 1.26** toolchain on all code — `gofmt -l .` must print nothing; if it prints files, run `gofmt -w` on them and re-run the gate.
- Unit/integration tests **never hit the network.** Live paths are env-gated: `YOLO_LLM=fake` (+ `YOLO_FAKE_SCRIPT`) selects the scripted fake driver; the e2e smoke vs `ai.kido.ws` (`scripts/e2e-live.sh`, created in Task 30) is on-demand, user-run, never CI.
- Host toolchain quirk (both installed toolchains): plain `import "embed"` + scalar `//go:embed` fails typecheck with `embed imported and not used` — the workaround in use is `import _ "embed"` (see `internal/tool/read.go`). Keep the pattern.
- Zen catalog CDN (`models.opencode.ai`) blocks python-urllib (403) — fetch with **curl + browser UA**.

## Commit & branch discipline

- Work happens inline on the current task branch (named with the active plan — see `PROGRESS.md` → Resume instructions), **one task at a time**.
- Conventional commits (`feat:`/`fix:`/`docs:`/`test:`/…, imperative, ≤ 72-char subject). Task commits use **the commit message pinned in the plan**. Between tasks: `docs: checkpoint — Task N (...) done, next is Task N+1`.
- Update `PROGRESS.md` at each checkpoint. It is a **rolling checkpoint** — keep it small: active task (full detail), one-line last-completed, key verified facts, append-only deviation log (current era inline; 2026-08-20 archived pre-v0.1.2 items 1–66 to `docs/superpowers/deviations-archive-v0.1.0.md`), open items. No per-task history, no plan-slice copies: `git log --oneline` and the plan file are the archive.
- **Tag `v0.1.0` (Task 30) ONLY with explicit user go-ahead.** Versioning: **semantic versioning** — `MAJOR.MINOR.PATCH`, MAJOR = breaking changes, MINOR = new features, PATCH = fixes. 0.1.0 = current scope; out-of-scope features land in 0.2.0+.

## Superpowers workflow (required)

Entries live under `superpowers:` skills; invoke a skill **before** acting when it applies — not as a formality.

| Situation | Skill |
|---|---|
| Any new conversation | `using-superpowers` — establishes skill usage; check for skills before ANY response or action |
| New design requirement, behavior change, or "build X" | `brainstorming` — classify spike / bounded / architectural; **approval gate before any implementation** |
| Executing the plan, task-by-task | `executing-plans` or `subagent-driven-development` (recommended) — **strict 5-step TDD per plan task**: (1) write failing test → (2) confirm FAIL → (3) minimal implementation → (4) `go vet ./... && go test ./...` green → (5) commit with the plan's message |
| Writing any implementation code | `test-driven-development` — failing test first, always |
| Bug, crash, deadlock, unexpected behavior | `systematic-debugging` — root cause before proposing a fix |
| Before claiming "done / fixed / passing", before committing | `verification-before-completion` — run the gate, read the output, then claim |
| Committing changes | `git-commit` — conventional message from the actual diff |
| Milestone finished / before merge | `requesting-code-review` — and `receiving-code-review` when feedback arrives |
| A new spec needs an implementation plan | `writing-plans` — specs → `docs/superpowers/specs/`, plans → `docs/superpowers/plans/`, progress → `docs/superpowers/PROGRESS.md` |
| 2+ independent tasks with no shared state | `dispatching-parallel-agents` — apply sequentially: one subagent at a time (core principle 7) |
| Feature work needing workspace isolation | `using-git-worktrees` |
| Implementation complete, deciding integration | `finishing-a-development-branch` |

## Golang skills (15, in `.agents/skills/`, hash-locked in `skills-lock.json`)

Invoke the relevant one(s) **per task**. Do not edit skills; they are pinned by hash.

| Work | Skill |
|---|---|
| New code (always pair the two) | `golang-naming` + `golang-code-style` |
| Error flow, wrapping, logging | `golang-error-handling` |
| Tests (table-driven, fake driver, goleak) | `golang-testing` |
| Goroutines, channels, engine turn loop | `golang-concurrency` |
| Defensive review (nil-safety, append aliasing) | `golang-safety` |
| Hot paths | `golang-data-structures` / `golang-performance` / `golang-benchmark` |
| Storage package (SQLite DAOs) | `golang-database` |
| Secrets, injection, config, auth | `golang-security` |
| Bugs, crashes, deadlocks | `golang-troubleshooting` |
| Interfaces, DI, lifecycle | `golang-design-patterns` |
| `cmd/yolo` CLI | `golang-cli` |
| Large restructure | `golang-refactoring` |

# DOX framework

- DOX is highly performant AGENTS.md hierarchy installed here
- Agent must follow DOX instructions across any edits

## Core Contract

- AGENTS.md files are binding work contracts for their subtrees
- Work products, source materials, instructions, records, assets, and durable docs must stay understandable from the nearest applicable AGENTS.md plus every parent AGENTS.md above it

## Read Before Editing

1. Read the root AGENTS.md
2. Identify every file or folder you expect to touch
3. Walk from the repository root to each target path
4. Read every AGENTS.md found along each route
5. If a parent AGENTS.md lists a child AGENTS.md whose scope contains the path, read that child and continue from there
6. Use the nearest AGENTS.md as the local contract and parent docs for repo-wide rules
7. If docs conflict, the closer doc controls local work details, but no child doc may weaken DOX

Do not rely on memory. Re-read the applicable DOX chain in the current session before editing.

## Update After Editing

Every meaningful change requires a DOX pass before the task is done.

Update the closest owning AGENTS.md when a change affects:

- purpose, scope, ownership, or responsibilities
- durable structure, contracts, workflows, or operating rules
- required inputs, outputs, permissions, constraints, side effects, or artifacts
- user preferences about behavior, communication, process, organization, or quality
- AGENTS.md creation, deletion, move, rename, or index contents

Update parent docs when parent-level structure, ownership, workflow, or child index changes. Update child docs when parent changes alter local rules. Remove stale or contradictory text immediately. Small edits that do not change behavior or contracts may leave docs unchanged, but the DOX pass still must happen.

## Hierarchy

- Root AGENTS.md is the DOX rail: project-wide instructions, global preferences, durable workflow rules, and the top-level Child DOX Index
- Child AGENTS.md files own domain-specific instructions and their own Child DOX Index
- Each parent explains what its direct children cover and what stays owned by the parent
- The closer a doc is to the work, the more specific and practical it must be

## Child Doc Shape

- Create a child AGENTS.md when a folder becomes a durable boundary with its own purpose, rules, responsibilities, workflow, materials, or quality standards
- Work Guidance must reflect the current standards of the project or user instructions; if there are no specific standards or instructions yet, leave it empty
- Verification must reflect an existing check; if no verification framework exists yet, leave it empty and update it when one exists

Default section order:
- Purpose
- Ownership
- Local Contracts
- Work Guidance
- Verification
- Child DOX Index

## Style

- Keep docs concise, current, and operational
- Document stable contracts, not diary entries
- Put broad rules in parent docs and concrete details in child docs
- Prefer direct bullets with explicit names
- Do not duplicate rules across many files unless each scope needs a local version
- Delete stale notes instead of explaining history
- Trim obvious statements, repeated rules, misplaced detail, and warnings for risks that no longer exist

## Closeout

1. Re-check changed paths against the DOX chain
2. Update nearest owning docs and any affected parents or children
3. Refresh every affected Child DOX Index
4. Remove stale or contradictory text
5. Run existing verification when relevant
6. Report any docs intentionally left unchanged and why

## User Preferences

When the user requests a durable behavior change, record it here or in the relevant child AGENTS.md

## Child DOX Index

- `internal/AGENTS.md` — core packages: package map, layering, pinned text, in-code zero telemetry (children: `internal/protocol`, `internal/tui`)
- `docs/superpowers/AGENTS.md` — process memory: PROGRESS.md checkpoint discipline, plans/specs/reviews layout, deviation log
- Root-owned files: `README.md`, `LICENSE`, `go.mod`, `.golangci.yml`, `.gitignore`, `skills-lock.json`, `cmd/`, `scripts/`, `.agents/skills/` (hash-locked — do not edit), and root-level project documentation.
