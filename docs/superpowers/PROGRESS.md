# Yolo — Progress & Status (session checkpoint)

**Updated:** 2026-08-22 (v0.1.2 in progress: wave 14/16 done, next wave 15 — code-style)

Rolling checkpoint: active task + last-completed + verified facts + v0.1.2-era deviation log +
open items. Keep it small — `git log --oneline` and the plan files are the archive (no
per-task history, no plan-slice copies). Pre-v0.1.2 deviations (items 1–66, frozen):
`docs/superpowers/deviations-archive-v0.1.0.md`.

## Where we are

**v0.1.0** (Tasks 1–30, M0–M8) and **v0.1.1** (post-release follow-ups) are complete —
merged to `main`, tagged, released. Current focus: the **v0.1.2 skill-driven review**
(16 waves; details in "Active"). Out-of-scope features → 0.2.0+.

## Resume instructions

1. Repo: `/home/kido/network/projects/yolo`, branch `v0.1.2_skills_review` (off the v0.1.1
main merge `1784ac0`). Active plan:
`docs/superpowers/plans/2026-08-19-v0.1.2-skill-review.md` — read its Resume Protocol +
Task 0 first, then ONLY the active task slice. Active spec:
`docs/superpowers/specs/2026-08-19-v0.1.2-skill-review-design.md`. (The original port
plan/spec `2026-08-17-yolo-go-port*.md` are closed.)
2. Per task: Step 1 failing test → Step 2 confirm FAIL → Step 3 minimal impl → Step 4
`go vet ./... && go test ./...` PASS → Step 5 commit with the plan's pinned message; then
roll this file (active → one-line "Last completed", next task → "Active").

## Active

v0.1.2 skill-driven review — wave 15 (golang-code-style) next. Read plan Task 15
slice (chunks R15a non-tui, R15b tui; class `script-driven` — run the scan command
block from the plan line FIRST, read only hit files, max 12 per subagent, remainder →
Deferred/Noted as style-scan-overflow). Steps: dispatch R15a → R15b one at a time
(Template R, `<SKILL>`=`golang-code-style`, IDs `[style-<n>]`); verify COVERAGE (one
split re-dispatch per partial; residual → `COVERAGE: skipped` note + open item);
commit findings `docs(review): v0.1.2 — wave 15 (code-style): <N> findings (…)`, then
Steps 2–6 same shape as Task 1: dispatch fix subagent (Template F,
`<SKILL>`=`golang-code-style`) iff any `contract-risk: none` — this batch must NOT
change any rendered byte (teatest suites are the tripwire; any teatest red → roll
back that fix); full gate; PROGRESS roll. Fix subagent: one at a time, YOLO
(core principle 8 — deviation 69). Commit gate:
go vet ./... && go test ./... + gofmt -l . + golangci-lint run ./...

## Last completed

Wave 14 (naming): 50 findings (P0:0 P1:0 P2:1 P3:49, commit 7ec2196) — 42 FIXED
(aa53daa…9fd780b, 9 commits; status 5da587d), 8 DEFERRED (6 contract auto-defers:
wire JSON tags, behavior>5 files ×3, render error text; 1 0.2.0 additive enum
sentinel; 1 test-name collision), 0 FALSE / 0 WONTFIX; stop-the-line n/a; gate green.
Wave 13 (benchmark): 8 hermetic benchmarks in 5 new bench files, 2 candidates deferred
with reason; gate green (commits e2946cf, 5edac05 — detail in those commits +
13-benchmark-hotpaths.md).

## Open items

- Version wiring (0.2.0, user decision 2026-08-21): `yolo version` prints hardcoded
  `0.0.0-dev` (cmd/yolo/main.go:58, plan-derived placeholder; no ldflags/build-info
  mechanism). Wire build-time version (e.g. ldflags `-X` from `git describe --tags`) in
  0.2.0+ — not in v0.1.x.
- none from v0.1.2 waves (wave-1 skipped chunks backfilled to `COVERAGE: full` — deviation 67)

## Key verified facts (so they don't get re-litigated)

- Permission engine = port of `packages/opencode/src/permission/index.ts` + matrices in
`agent/agent.ts` (build/plan/yolo verbatim, Task 10).
- Doom loop = sliding 3-identical window; wildcard-deny hides tool iff last matching rule
is `*` deny; `write`+`edit` both map to permission `edit`.
- Pinned deps: `charm.land/bubbletea/v2` v2.0.8, `charm.land/lipgloss/v2` v2.0.6,
`charm.land/bubbles/v2` v2.1.1, `modernc.org/sqlite` v1.56.0 (pure Go, no cgo),
`tidwall/jsonc` v0.3.3; dev-only `teatest/v2` v2.0.0-20260816001655-68d539dca504.
- Module `github.com/kido5217/yolo`, Go ≥ 1.25 (installed 1.26.5).
- Single deliberate wire deviation: `x-yolo-directory` header.
- Test gating: unit tests never hit network; `YOLO_LLM=fake` (+ `YOLO_FAKE_SCRIPT`) selects
the scripted fake driver; zen fixture gate = 57 models (42 openai + 15 anthropic,
7 google excluded).
- TUI import rule: non-test files under `internal/tui/` import only `internal/protocol` +
`internal/tui/*`; `_test.go` may use `internal/server/testutil` (escape hatch).
- v1 behavior pins: keymap is pgup/pgdn scroll + `\`+enter newline (noted in /help; spec's
↑/↓ viewport scroll replaced); JSONC comments are NOT preserved when a config PATCH
rewrites `yolo.jsonc`.
- lipgloss v2 `Render()` appends a trailing SGR reset AFTER the styled input: trim padded
plain strings (`TrimRight`) BEFORE styling (a styled string's last bytes are `\x1b[m`, so a
post-style trim silently misses), and count display widths in runes
(`utf8.RuneCountInString`) — `·` is 2 bytes, `○` 3 (both 1 column). Both bit T27's two-pane
column math.
- e2e/endpoint facts (Task 30): `scripts/e2e-live.sh` validated PASS (exit 0) 2026-08-18
BOTH against a mock OpenAI SSE endpoint and the REAL `https://ai.kido.ws/v1` — the spec v1
dogfood success criterion is met (completed `bash ls /tmp` tool call + text reply; abort
idle → `aborted:false`, busy → `aborted:true`; SIGTERM → exit 0). `ai.kido.ws` accepts ANY
bearer token (private endpoint; no auth.json on this host — key order env → auth.json →
config). `GET /global/health` → `{"status":"ok"}`; `/session/{id}/message` rows =
`{"info":{role,error:{type},...},"parts":[...]}` (jq: `.info.role`). Script mechanics:
`req()` must set globals (never run inside `$(…)` — subshell loses `HTTP_STATUS`); boot
from the scratch project dir (deviation 65).
- teatest v2 output mechanics (bit T28's suites): (a) each `WaitFor` drains the SHARED
output buffer — consecutive `WaitFor`s observe DISJOINT slices, so a multi-token terminal
state must be ONE merged condition, never two sequential waits (an idle app emits no new
frames for the second); probe `Read`s consume bytes later assertions need; (b) the fake
terminal is not a TTY → lipgloss strips EVERY style; pin `teatest.WithProgramOptions(tea.WithEnvironment([]string{"TTY_FORCE=1","TERM=xterm-256color"}))` for deterministic
ANSI256 SGR (derived from TERM alone, no terminfo; `charmbracelet/colorprofile` stays
indirect — never import directly, "no other deps" rule); (c) v2 `tea.Tick(d, f)` callback
is `func(time.Time) tea.Msg` (v1: `func() tea.Msg`); v2 programs handle `tea.QuitMsg`
internally.

## Plan deviations logged (append-only; items 1–66 in `docs/superpowers/deviations-archive-v0.1.0.md` — pattern: tests define contract)

67. Wave-1 backfill (user-approved split re-dispatch, process, minor): the prescribed single
re-dispatch of the two failed wave-1 chunks (session+permission+bus, server+llm+provider)
returned empty both times; per plan they were marked `COVERAGE: skipped`, then the user
approved a split backfill — 4 subagents (session / permission+bus / server / llm+provider,
≤3 parallel per dispatch message) all returned `COVERAGE: full`. Placeholders superseded:
01-concurrency-session.md rewritten in place; 01-concurrency-server-llm.md deleted,
replaced by 01-concurrency-server.md + 01-concurrency-llm-provider.md. Fix subagent then
worked the 8 `contract-risk: none` findings: 8 FIXED, 0 FALSE, 0 WONTFIX; gate + `-race`
green.
68. User-directive (process, minor, 2026-08-20): subagent dispatch is strictly one at a
time — never more than one subagent in flight; wait for a full return before dispatching
the next. Supersedes the v0.1.2 plan/spec "≤3 parallel" wording (plan Global Constraints,
Architecture line, per-wave dispatch lines; spec §1 decision 4, §3 step 1, §8). AGENTS.md
core principle 7 + PROGRESS.md Active updated the same day. Wave 1 already ran under the
old rule
(deviation 67) — its historical records are unrewritten.
69. Subagent dispatch type (process, minor, 2026-08-21): plan text says the fix
subagent is dispatched as one `task` call, `general` (Task 1 Step 4, carried into all
later waves via "same shape as Task 1"). Core principle 8 (AGENTS.md, added same day)
supersedes plan text: root agent is YOLO → subagents must be YOLO. Wave-5 fix subagent
dispatched as YOLO (first run 3 fixes + status commit 9d463d4; resumed once for
deferred-status backfill ea67e84). Plan text left unedited — principle 8 governs all
remaining dispatches; historical (waves 1–4) dispatch records unrewritten.
70. Wave-12 count correction (process, minor, 2026-08-21): R12a returned a Stats line
`P0:0 P1:0 P2:2 P3:4` (6) but its `## Findings` section holds 7 lines (P2:2 P3:5,
perf-1..7). Per core principle 5 the finding lines are the contract: orchestrator
corrected the Stats line to `P3:5` and amended the still-local, unreferenced findings
commit to `docs(review): v0.1.2 — wave 12 (performance): 7 findings (P0:0 P1:0 P2:2
P3:5)` (9e2a28b) before any other commit referenced it.
71. Wave-14 R14a redo (user-directive, process, minor, 2026-08-22): an earlier partial
run had left `14-naming-core.md` as a placeholder (`COVERAGE: partial — skipped: all`).
User directive: "do not skip R14a, redo full step" — orchestrator removed the placeholder
and re-dispatched R14a fresh (full Template R, no resume of the placeholder); it returned
`COVERAGE: full` (10 findings). This supersedes the plan fallback ("re-dispatch once,
then mark skipped") for this chunk only, by explicit user override.
