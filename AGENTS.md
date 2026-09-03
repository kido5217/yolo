# AGENTS.md — agent instructions for the yolo repo

Resume rail: beads epic (`bd ready`) → active plan slice →
`docs/superpowers/PROGRESS.md` (facts) → `docs/superpowers/DEVIATIONS.md` (audit).

## Project

**yolo** is a Go TUI + core-server harness in the lineage of [opencode](https://github.com/anomalyco/opencode) **v1.18.18** (TUI + core server only; web/desktop/slack/console dropped). It began as a faithful port whose purpose was to test the capabilities of **Qwen3.8-27B** on a single RTX 5090 behind `https://ai.kido.ws/v1` — that goal is complete (local Qwen 3.8 tested, stable, optimized; the v0.3.0 tree is the stable baseline). From v0.4.0 the project's purpose is to **test various LLM harnesses and frameworks** (yolo drives and evaluates other harnesses). opencode remains a **reference** for how things should be done, not a binding contract (core principle 2; spec `docs/superpowers/specs/2026-08-24-v0.4.0-design.md`).

- Module `github.com/kido5217/yolo`, binary `yolo`, Go ≥ 1.25.
- Single binary: starts the core HTTP server (REST + SSE) **in-process**, then runs the bubbletea v2 TUI which talks to it **only** via the wire contract.
- Core layering: `protocol` (wire DTOs, single source of truth) → `server` → `session` (agent loop) → `llm` / `provider` / `tool` / `permission` / `config` / `auth` / `storage` / `bus`.
- **Dependency policy** — allowlist + agent-proposable; exact versions and the proposal procedure are in "Dependency policy" below.

## Dependency policy

Runtime deps are pinned at exact versions (`go.mod` is the current state). The
allowlist below is pre-approved; anything outside it requires an agent
**dep proposal** (in the task's spec/plan or beads issue) BEFORE any
`go get`/`go mod tidy`.

**Allowlist** (module version — purpose):

- `charm.land/bubbletea/v2` v2.0.9 — TUI engine
- `charm.land/lipgloss/v2` v2.0.6 — styling
- `charm.land/bubbles/v2` v2.2.1 — TUI components
- `modernc.org/sqlite` v1.57.0 — pure-Go SQLite (no cgo)
- `tidwall/jsonc` v0.3.3 — JSON-with-comments parsing
- `github.com/aymanbagabas/go-udiff` v0.4.1 — Myers line diff
- `github.com/charmbracelet/x/term` v0.2.2 — raw-mode tty for OSC palette detection (already in the graph via bubbletea v2)
- `charm.land/glamour/v2` v2.0.1 — GFM markdown + chroma syntax highlighting for the transcript (also imports `glamour/ansi` + `chroma/v2/styles` for the global "charm" slot workaround)
- `charm.land/huh/v2` v2.0.3 — field dialogs: alert/confirm/input
- `github.com/sahilm/fuzzy` v0.1.3 — subsequence fuzzy filter for the select/palette
- `github.com/spf13/cobra` v1.10.2 — the v0.6.0 CLI command/flag tree (Apache-2.0 accepted as permissive, so the checklist below reads MIT/BSD/Apache-2.0; transitive indirects `pflag` + `mousetrap`)
- dev-only: `github.com/charmbracelet/x/exp/teatest/v2` v2.0.0-20260823001701-96af6d2cb5f6 — TUI golden testing; `go.uber.org/goleak` v1.3.0 — goroutine-leak detection via `VerifyTestMain` in the internal/session, internal/server and internal/bus test suites (zero new `go.mod` modules; its test-deps land in `go.sum` only)

**Proposal procedure.** Each proposal names module + exact version, with evidence
from **extensive web search** — the agent MUST treat its own memory as outdated:
maintenance status, last activity, license, and available versions are verified
live (e.g. GitHub API, `go list -m`), never recalled. Checklist: actively
maintained, pure Go / no cgo, permissive license (MIT/BSD/Apache-2.0), transitive
surface (how many NEW modules it adds to the build); why stdlib or hand-rolling is
inadequate. Landing requires explicit user approval; approved deps join the
allowlist above + a `docs/superpowers/PROGRESS.md` fact.

## Key documents (topic → source of truth)

| Topic | Source of truth |
|---|---|
| Task state (active, next, blocked) | **beads epic (`bd ready`) — read first on resume.** No file |
| Verified facts (do not re-litigate) | `docs/superpowers/PROGRESS.md` |
| Deviation audit (append-only, principle 5) | `docs/superpowers/DEVIATIONS.md` |
| 0.3.0 work list (deferred findings) | `docs/superpowers/DEFERRED.md` |
| Implementation plan | `docs/superpowers/plans/` — dated, one active at a time (named in the beads epic). **Read ONLY the active task slice** |
| Approved design | `docs/superpowers/specs/` — dated, the active one is named in the beads epic. Zero-telemetry statement: `2026-08-17-yolo-go-port-design.md` §1 |
| Upstream reference | `/tmp/opencode-upstream` — clone at tag `v1.18.18`. If missing: `git clone --depth 1 --branch v1.18.18 https://github.com/anomalyco/opencode /tmp/opencode-upstream`. **Never touch `/tmp/opencode`** — pre-existing user data |
| Golang skills | `.agents/skills/` + `skills-lock.json` — 15 hash-locked (samber/cc-skills-golang) — see Skills below |

## Core principles (non-negotiable)

1. **Zero telemetry.** yolo runs on the end user's machine and must contain zero telemetry: no usage data ever sent to any remote server, no opt-in telemetry. Upstream telemetry surfaces are **skipped, not deferred**: OTEL/OTLP exporter (`packages/core/src/observability/otlp.ts`), OpenTelemetry spans on LLM calls (`experimental.openTelemetry` / `experimental_telemetry` in `packages/opencode/src/session/llm.ts`), telemetry-identity field. `OTEL_*` env vars are inert; the ported config schema omits `experimental.openTelemetry`. Full statement: spec §1.
2. **Reference, not contract.** opencode v1.18.18 is a *reference* for how things should be done, not a binding contract. The legacy REST paths/JSON shapes and the legacy SSE event set remain mirrored so the baseline stays verifiable against opencode's OpenAPI contract, but yolo may deviate from upstream (wire shapes, behavior, pinned text) on explicit user instruction — every such deviation is logged in `docs/superpowers/DEVIATIONS.md` with severity. Standing baseline deviation: the scoping header is **`x-yolo-directory`** (upstream: `x-opencode-directory`).
3. **Pins are change gates, not upstream locks.** The 14 session prompt files and all tool `desc/*.txt` files are guarded by sha256 pins in tests. The pins record current intended content, not an upstream lock: pinned text may change in normal work, and an intentional change re-baselines the sha256 pin in the same commit. Never leave a pin dangling.
4. **TUI is a pure client.** Non-test files under `internal/tui/` import only `internal/protocol` + `internal/tui/*` (+ stdlib/charm deps). `_test.go` may use `internal/server/testutil` (escape hatch). Enforced by `TestImportsDirection` (`internal/tui/imports_test.go`).
5. **Tests define the contract.** When the plan contradicts itself (or its own test code is buggy), resolve per the last-stated call, fix the test, and **log the deviation in `docs/superpowers/DEVIATIONS.md`** with severity.
6. **Task state in beads; proven knowledge in docs.** Track task state in beads — never duplicate it in files. A stale audit log is a broken resume. File shapes: "Commit & branch discipline."
7. **Subagents, at most one at a time.** Dispatch at most ONE subagent (via the `task` tool) at a time; wait for its full return before dispatching the next. This supersedes any plan/spec wording permitting parallel subagents.
8. **YOLO spawns only YOLO.** If the root agent is `YOLO`, any subagent it spawns MUST also be `YOLO` — never dispatch a subagent of a different agent type.
9. **Subagents for every task.** Every requested work item — change, review, audit, research, status report, or explanation — is executed by a subagent (via the `task` tool) whenever it needs repo legwork (file reads, searches, commands, history digging); never inline in the root agent. The test is cost, not form: the deliverable's shape (report, answer, diff) is irrelevant — what is minimized is the root's context, so only summaries and conclusions return. The root's inline surface is closed and limited to: bead lifecycle (claim/close), dispatch, reviewing returned summaries, gate verification, git/PR operations, and pure conversation requiring no repo legwork. Subagent type follows the work (`beads-task-agent` for beads work, `general`/`explore` for implementation/research, `YOLO` per principle 8). This pairs with principle 7 (one at a time) and supersedes any plan/spec wording implying inline root execution.

## Commands & verification

- **The CI gate (run at module root):** `go vet ./... && go test ./...` — **every task ends with both green and a commit.**
- **Formatting (run at module root after every completed step, before the commit):** gofmt from the **Go 1.26** toolchain on all code — `gofmt -l .` must print nothing; if it prints files, run `gofmt -w` on them and re-run the gate.
- Unit/integration tests **never hit the network.** Live paths are env-gated: `YOLO_LLM=fake` (+ `YOLO_FAKE_SCRIPT`) selects the scripted fake driver; the e2e smoke vs `ai.kido.ws` (`scripts/e2e-live.sh`) is on-demand, user-run, never CI.
- Host toolchain quirk: plain `import "embed"` + scalar `//go:embed` fails typecheck with `embed imported and not used` — the workaround in use is `import _ "embed"` (see `internal/tool/read.go`). Keep the pattern.
- Zen catalog CDN (`models.opencode.ai`) blocks python-urllib (403) — fetch with **curl + browser UA**.

## Commit & branch discipline

- **NEVER commit directly onto `main`** (user directive, 2026-08-22). Every change —
  code, docs, checkpoints, release bookkeeping — is committed on a task branch and
  reaches `main` only via a merged PR (branch → commit → push → PR → merge). The
  agent merges its own task PRs itself once the CI gate is green (user go-ahead
  2026-09-03); no further per-PR approval needed.
- Work happens inline on the current task branch (named with the active plan), **one task at a time**.
- Conventional commits (`feat:`/`fix:`/`docs:`/`test:`/…, imperative, ≤ 72-char subject). Task commits use **the commit message pinned in the plan**. Between tasks: `docs: checkpoint — Task N (...) done, next is Task N+1`.
- When a fact is proven or a deviation is logged, update `docs/superpowers/PROGRESS.md` (facts) / `docs/superpowers/DEVIATIONS.md` (audit) before moving on. **No per-task history in files, no plan-slice copies:** `git log --oneline` and the plan file are the archive. Deviations are append-only in `docs/superpowers/DEVIATIONS.md` (pre-v0.1.2 items 1–66 frozen in `docs/superpowers/deviations-archive-v0.1.0.md`).
- **Tags ONLY with explicit user go-ahead.** Versioning: **semantic versioning** — `MAJOR.MINOR.PATCH`, MAJOR = breaking changes, MINOR = new features, PATCH = fixes.

## Agent skills

### Issue tracker

Issues live in beads (`bd` CLI / beads MCP) — local Dolt-backed store. See `docs/agents/issue-tracker.md`.

### Triage labels

Five canonical roles, label strings equal to role names. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: `CONTEXT.md` + `docs/adr/` at the repo root, created lazily. See `docs/agents/domain.md`.

## Golang skills (15, in `.agents/skills/`, hash-locked in `skills-lock.json`)

Invoke the relevant `golang-*` skill(s) **per task** — the harness skill list carries each skill's trigger. New code always pairs `golang-naming` + `golang-code-style`. Do not edit skills; they are pinned by hash.

## User Preferences

- Harnesses: **opencode only** (yolo itself in the future). All agent logic
  lives in `AGENTS.md`; the only managed block is `bd setup opencode` (do not
  hand-edit its markers). The project beads skill at `.agents/skills/beads/`
  is bd-managed and stays.
- TUI work: follow the project `charm-stack` skill
  (`.agents/skills/charm-stack/`) for Bubbletea/Bubbles/Lipgloss/Huh
  patterns; its v1 import paths are illustrative — the allowlist's
  `charm.land/*` v2 line wins.

# DOX framework

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

## Child DOX Index

- `internal/AGENTS.md` — core packages: package map, layering, pinned text, in-code zero telemetry (children: `internal/protocol`, `internal/tui`)
- `docs/superpowers/AGENTS.md` — process memory: verified facts (PROGRESS.md), deviation audit (DEVIATIONS.md), 0.3.0 work list (DEFERRED.md), plans/specs/reviews layout
- Root-owned files: `README.md`, `LICENSE`, `go.mod`, `justfile`, `.golangci.yml`, `.gitignore`, `skills-lock.json`, `cmd/`, `scripts/`, `.agents/skills/` (hash-locked golang skills; `.agents/skills/beads/` is bd-managed), `docs/agents/` (engineering-skill config), and root-level project documentation.


<!-- BEGIN BEADS INTEGRATION v:1 profile:full hash:19cc25d9 -->
## Issue Tracking with bd (beads)

**IMPORTANT**: This project uses **bd (beads)** for ALL issue tracking. Do NOT use markdown TODOs, task lists, or other tracking methods.

### Why bd?

- Dependency-aware: Track blockers and relationships between issues
- Git-friendly: Dolt-powered version control with native sync
- Agent-optimized: JSON output, ready work detection, discovered-from links
- Prevents duplicate tracking systems and confusion

### Quick Start

**Check for ready work:**

```bash
bd ready --json
```

**Create new issues:**

```bash
bd create "Issue title" --description="Detailed context" -t bug|feature|task -p 0-4 --json
bd create "Issue title" --description="What this issue is about" -p 1 --deps discovered-from:bd-123 --json
```

**Claim and update:**

```bash
bd update <id> --claim --json
bd update bd-42 --priority 1 --json
```

**Complete work:**

```bash
bd close bd-42 --reason "Completed" --json
```

### Issue Types

- `bug` - Something broken
- `feature` - New functionality
- `task` - Work item (tests, docs, refactoring)
- `epic` - Large feature with subtasks
- `chore` - Maintenance (dependencies, tooling)

### Priorities

- `0` - Critical (security, data loss, broken builds)
- `1` - High (major features, important bugs)
- `2` - Medium (default, nice-to-have)
- `3` - Low (polish, optimization)
- `4` - Backlog (future ideas)

### Workflow for AI Agents

1. **Check ready work**: `bd ready` shows unblocked issues
2. **Claim your task atomically**: `bd update <id> --claim`
3. **Work on it**: Implement, test, document
4. **Discover new work?** Create linked issue:
   - `bd create "Found bug" --description="Details about what was found" -p 1 --deps discovered-from:<parent-id>`
5. **Complete**: `bd close <id> --reason "Done"`

### Quality
- Use `--acceptance` and `--design` fields when creating issues
- Use `--validate` to check description completeness

### Lifecycle
- `bd defer <id>` / `bd supersede <id>` for issue management
- `bd stale` / `bd orphans` / `bd lint` for hygiene
- `bd human <id>` to flag for human decisions
- `bd formula list` / `bd mol pour <name>` for structured workflows

### Sync

bd stores issue history in Dolt:

- Each write auto-commits to Dolt history
- Use `bd dolt push`/`bd dolt pull` for remote sync
- Do not treat `.beads/issues.jsonl` as the sync protocol

**Architecture in one line:** issues live in a local Dolt DB; sync uses `refs/dolt/data` on your git remote; `.beads/issues.jsonl` is a passive export. See https://github.com/gastownhall/beads/blob/main/docs/SYNC_CONCEPTS.md for details and anti-patterns.

### Important Rules

- ✅ Use bd for ALL task tracking
- ✅ Always use `--json` flag for programmatic use
- ✅ Link discovered work with `discovered-from` dependencies
- ✅ Check `bd ready` before asking "what should I work on?"
- ❌ Do NOT create markdown TODO lists
- ❌ Do NOT use external issue trackers
- ❌ Do NOT duplicate tracking systems

For more details, see README.md and docs/QUICKSTART.md.

## Agent Context Profiles

The managed Beads block is task-tracking guidance, not permission to override repository, user, or orchestrator instructions.

- **Conservative (default)**: Use `bd` for task tracking. Do not run git commits, git pushes, or Dolt remote sync unless explicitly asked. At handoff, report changed files, validation, and suggested next commands.
- **Minimal**: Keep tool instruction files as pointers to `bd prime`; use the same conservative git policy unless active instructions say otherwise.
- **Team-maintainer**: Only when the repository explicitly opts in, agents may close beads, run quality gates, commit, and push as part of session close. A current "do not commit" or "do not push" instruction still wins.

## Session Completion

This protocol applies when ending a Beads implementation workflow. It is subordinate to explicit user, repository, and orchestrator instructions.

1. **File issues for remaining work** - Create beads for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **Handle git/sync by active profile**:
   ```bash
   # Conservative/minimal/default: report status and proposed commands; wait for approval.
   git status

   # Team-maintainer opt-in only, unless current instructions forbid it:
   git pull --rebase
   bd dolt push
   git push
   git status
   ```
5. **Hand off** - Summarize changes, validation, issue status, and any blocked sync/commit/push step

**Critical rules:**
- Explicit user or orchestrator instructions override this Beads block.
- Do not commit or push without clear authority from the active profile or the current user request.
- If a required sync or push is blocked, stop and report the exact command and error.

<!-- END BEADS INTEGRATION -->

