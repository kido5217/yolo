# Yolo — Progress & Status (session checkpoint)

**Updated:** 2026-08-17 (plan complete)

## Where we are

Superpowers **writing-plans** is COMPLETE. The implementation plan is written (30 tasks, M0–M8), grounded against upstream v1.18.18, self-reviewed, and committed. We are at the **"user reviews written plan"** gate. Per the superpowers flow, execution starts ONLY after user approval. Execution preference (user): inline in this repo, tasks executed one-by-one, no subagent delegation.

## Resume instructions (next session)

1. Repo: `/home/kido/network/projects/yolo` (branch `plan`). Plan: `docs/superpowers/plans/2026-08-17-yolo-go-port.md` (~6000 lines). Spec: `docs/superpowers/specs/2026-08-17-yolo-go-port-design.md`.
2. Ask the user to review the plan. If approved → execute: start at Task 1 (go.mod + skeleton) and follow each task's 5-step TDD loop exactly; commit per task as specified in the plan.
3. Upstream reference clone (`/tmp/opencode-upstream`, tag `v1.18.18`) is likely gone after restart. Recreate if needed:
   `git clone --depth 1 --branch v1.18.18 https://github.com/anomalyco/opencode /tmp/opencode-upstream`
   (`/tmp/opencode` is pre-existing user data — never touch it.)
4. GitHub MCP is NOT yet authorized if needed for repo/PR work (OAuth URL was surfaced earlier; user action required). Not needed for plan writing.

## Completed work this session

| Item | Where |
|---|---|
| Requirements gathered + 7 design sections approved by user | spec (§1–§8), commits `f54991c`, `cec9a8f` |
| Spec written, source-verified, self-reviewed, committed | `docs/superpowers/specs/2026-08-17-yolo-go-port-design.md` |
| Implementation plan: 30 tasks over M0–M8, each with Files/Interfaces/5-step TDD/commit; grounded verbatim against v1.18.18 (permission matrices, tool output formats, prompt txt files + sha256 pins, agent descriptions, SSE event shapes) | `docs/superpowers/plans/2026-08-17-yolo-go-port.md` |
| Plan self-review: spec coverage M0–M8 check, placeholder scan clean, cross-task type consistency (protocol DTOs, testutil seams, fake-LLM wiring, import-direction guard) | plan "Self-Review Notes" (end of file) |

## Plan resolutions & flags to raise at handoff (severity)

1. **important** — teatest: spec's `charm.land/x/exp/teatest` is the v1 module and cannot build a v2 TUI; plan pins `charm.land/x/exp/teatest/v2` v2.0.0-20260816001655-68d539dca504 (dev-only).
2. **important** — spec DDL lacked per-message cost/tokens; plan adds `message.cost REAL` + `message.tokens TEXT` (Task 5) and session-level aggregates at read time (Task 19).
3. **important** — spec DDL lacked todo persistence; plan adds migration v2 `todo` table + DAOs + `protocol.Todo` (Task 14).
4. minor — `title.txt` actually lives in `agent/prompt/` (spec said `session/prompt/`); Task 15 embeds 14 prompt files (13 session + title).
5. minor — config `agents` map vs spec's `agent` string ambiguity resolved to `agents` (Task 3/20); custom agents v1 = permission merge + description stub.
6. minor — SSE frames include `id` beyond the spec envelope example (type+properties).
7. minor — JSONC comments are not preserved when config PATCH rewrites the file.
8. minor — v1 keymap: pgup/pgdn scroll viewport, `\`+enter newline (spec's ↑/↓ viewport scroll replaced; noted in /help text).

## Key verified facts (so they don't get re-litigated)

- Permission engine = port of `packages/opencode/src/permission/index.ts` + matrices in `agent/agent.ts` (build/plan/yolo verbatim in Task 10).
- Doom loop = sliding 3-identical window; wildcard-deny hides tool iff last matching rule is `*` deny; write+edit both map to permission `edit`.
- Pinned deps: `charm.land/bubbletea/v2` v2.0.8, `charm.land/lipgloss/v2` v2.0.6, `charm.land/bubbles/v2` v2.1.1, `modernc.org/sqlite` v1.56.0, `tidwall/jsonc` v0.3.3; dev-only `teatest/v2`.
- Module `github.com/kido5217/yolo`, Go ≥ 1.25 (installed 1.26.5).
- Single deliberate wire deviation: `x-yolo-directory` header.
- Test gating: unit tests never hit network; `YOLO_LLM=fake` + `YOLO_FAKE_SCRIPT` selects the scripted fake driver (wired in Task 19, e2e in Task 21); zen fixture gate = 57 models (42 openai + 15 anthropic, 7 google excluded).
- TUI import rule: non-test files under `internal/tui/` import only `internal/protocol` + `internal/tui/*`; `_test.go` may use `internal/server/testutil` (escape hatch). Enforced by Task 29.

## Open items

- [ ] User review of the PLAN (next gate)
- [ ] Execute Tasks 1–30 inline (per task commit messages in the plan)
- [ ] Task 30 tag `v1.0.0` ONLY with explicit user go-ahead
- [ ] On-demand live e2e vs `ai.kido.ws` (scripts/e2e-live.sh) — user-run, never CI
