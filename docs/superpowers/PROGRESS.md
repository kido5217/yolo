# Yolo — Progress & Status (session checkpoint)

**Updated:** 2026-08-22 (**v0.1.3 output-fix branch complete**: 4 commits on
`v0.1.3_output_fix`, gate green, PR pending user merge)

Rolling checkpoint: active task + last-completed + verified facts + v0.1.2-era deviation log +
open items. Keep it small — `git log --oneline` and the plan files are the archive (no
per-task history, no plan-slice copies). Pre-v0.1.2 deviations (items 1–66, frozen):
`docs/superpowers/deviations-archive-v0.1.0.md`.

## Where we are

**v0.1.0**, **v0.1.1**, **v0.1.2** complete — merged, tagged, released. **v0.1.3**
(user-reported output/hang bugfix) is implemented on branch `v0.1.3_output_fix`
(4 commits, base `main` = `6046ed1`); awaiting **user PR merge**, then tag `v0.1.3`
(semantic patch: fixes only). 0.2.0 still needs a design (brainstorm) + plan after.

## Resume instructions

1. Repo: `/home/kido/network/projects/yolo`. If resuming the v0.1.3 line: check out
   `v0.1.3_output_fix` (up to date with origin after push). No active plan — the
   v0.1.3 work was a user-reported bugfix (systematic-debugging + TDD per commit),
   not a plan task. 0.2.0 needs a new design (`brainstorming`), then a plan
   (`writing-plans`) sourced from the 0.2.0 seed below — before any implementation.
2. Per task of any future plan: Step 1 failing test → Step 2 confirm FAIL → Step 3
   minimal impl → Step 4 `go vet ./... && go test ./...` (+ gofmt + golangci-lint) PASS
   → Step 5 commit with the plan's pinned message; then roll this file (active →
   one-line "Last completed", next task → "Active").

## Active

**v0.1.3 RELEASED** (2026-08-22): PR #7 merged to `main` (`1d3eca6`) + tag
`v0.1.3` + GitHub release cut. Five root-cause fixes behind the "run CI gate and print
full output" failure (apparent hang, then an infinite tool loop that did NOT occur on the
same prompt+model in upstream opencode). Commits: `85a227e` fix(tool) marker decode + cwd
newline; `16a2e13` fix(tui) SSE-drop re-hydrate + SSE write-error return; `968b9ba`
test(tui) resync-pump flake; `da25275` feat(tui) inline bash output preview; `18ea0b6`
fix(tool) save full truncated bash output + verbatim marker (loop in
`ses_EuCqnuD7PTQQxVu5xmFX` — the model re-ran `go test -v` ~14× because the 1036-of-1209-line
tail arrival was silent; upstream shell.ts:579 pins the `Full output saved to:` marker the
v1 plan omitted — Task 11 pinned only `tail()`, so this is a port gap, deviation 76);
`9d88357` fix(session) drop user-message re-append on tool rounds — history replay is
1:1 with upstream (loop in `ses_Mt8jhDCdseSyZjcqVhED`: same prompt does NOT loop in
upstream opencode, so the re-append was the diff; deviation 77).

**Next (0.2.0):** the v1 port is feature-complete and released. 0.2.0 seeds live in
`docs/superpowers/reviews/v0.1.2/DEFERRED.md` + `08-refactoring-backlog.md` (+ the
version-wiring open item below). No active 0.2.0 plan yet — start via `writing-plans`
after user picks scope.

Root causes (archive): (1) `16d0483` (v0.1.2 datastruct-2)
re-wrote the shared end-marker regex without re-teaching `decodeMarker` — from the
2nd bash command on the reported exit code was the marker counter and the cwd was never
decoded (a latent extra bug: `pwd`'s trailing newline in the base64 made the respawn
`os.Stat` fail → always respawned in the root dir); (2) the TUI never noticed its SSE
stream dropping (silent reconnect, no re-hydrate) — a lost `session.status` left the
footer stuck on `busy` forever ("hang") with a stale transcript ("nothing printed");
(3) bash output was row-only until alt+e (upstream shows it inline);
(4) truncated bash output reached the model silently — `tail()` was ported without
upstream's full-output save + `Full output saved to:` marker (plan Task 11 pinned only
`tail()`), so a 1036-of-1209-line CI-gate run arrived mid-stream and the model
re-ran the gate ~14×;
(5) plan Task 16's LOCKED mapping RE-APPENDED the newest user message at the tail of
every tool-call round, so the model re-saw its own instruction each round and re-ran
tools in a loop even with (4) fixed — upstream replays history 1:1 (round ends with
the tool result). Decisive diff: the same prompt+model does NOT loop in upstream
opencode.

## Last completed

v0.1.3 RELEASED (2026-08-22, tag `v0.1.3`, PR #7 → `main` `1d3eca6`): the reported
loop + all five contributing causes fixed with failing tests first (shell marker
exit/cwd; SSE-drop resync; inline bash preview + expanded-empty-output parts-loop
escape; truncated-bash full-output save + marker; history replay 1:1 — dropped the
plan's user-message re-append), full gate green (vet+test+gofmt+golangci-lint).
Evidence:
hung session `ses_wNbfyVPnHLrEyJXM8nrr` (12 ok CI-gate runs with incrementing phantom
`exit:5..16` metadata.

## Open items

- Version wiring (0.2.0, user decision 2026-08-21): `yolo version` prints hardcoded
  `0.0.0-dev` (cmd/yolo/main.go:58, plan-derived placeholder; no ldflags/build-info
  mechanism). Wire build-time version (e.g. ldflags `-X` from `git describe --tags`) in
  0.2.0+ — not in v0.1.x.
- [x] All 15 waves full-coverage (wave-1 skipped chunks backfilled — deviation 67; no
  residual `COVERAGE: skipped` notes)

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
- Shell end-marker wire form (v0.1.3, verified against live markers):
`__YOLO_END_{n}_{exit}_{b64(pwd incl. trailing \n)}` matched by the shared regex
`^__YOLO_END_(\d+)_([^\s]*)$` with `m[1]==n`; `decodeMarker` splits `m[2]` at the first
`_` (std base64 has no `_`) and trims `pwd`'s newline. The emitted marker has never
carried a colon (one stale comment claimed `_{n}_:`, a doc typo since the protocol's
first commit `ae0ff27`); the pre-v0.1.3 `decodeMarker` mis-parsed the GROUP positions,
not a separator. The first command (n=0) masked the group bug because counter==0.
- SSE drop contract (v0.1.3): `client.Events` returns `(events, resync)`; a ping per drop
(buffered, non-blocking); app re-hydrates the current route on `resyncMsg` (the bus has
no replay — gap events are unrecoverable, recovery is REST hydration from storage).

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
72. Roll-up wave-13 Summary row (process, minor, 2026-08-22): the Task 16 roll-up
subagent's wave-13 row reads `8 findings / 8 fixed / 2 deferred`, which does not sum
(10 > 8). Cause: wave 13 (golang-benchmark) is assessment-only — its 8 Stats items are
delivered benchmarks (all done), and its 2 "deferred" are benchmark *candidates* that
were never findings. Per core principle 5 the orchestrator kept the row as the most
faithful representation (8 delivered + 2 candidate-deferred = seed info, not an error),
footnoted it directly under the DEFERRED.md Summary table, and recorded it here. The
14 defect waves (1–12, 14, 15) all sum exactly: 268 findings = 188 fixed + 88 deferred
+ 0 false + 0 wontfix. No counts were altered.
73. v0.1.3 marker-decode regression (bugfix, P0, 2026-08-22): the v0.1.2 review fix
datastruct-2 (commit `16d0483`, "compile shell end-marker regex once per package")
switched the per-n literal regex to the shared counter-carrying regex and updated the
counter check, but left `decodeMarker` parsing the OLD group layout (group 1 = exit
code). From the 2nd bash command on, the reported exit code was the incrementing marker
counter and the cwd was never decoded (base64 of `"{exit}_{b64}"` fails). Proven in a
user's production session (12 ok CI-gate runs, metadata `exit:5..16`). Also surfaced by
the new test: a latent pre-existing bug — the marker encodes `$(pwd | base64 -w0)`, i.e.
`pwd`'s trailing newline; the stored cwd therefore carried `\n`, the respawn `os.Stat`
failed, and the shell always respawned in the root dir (no pre-v0.1.3 test ever killed
the shell, so the documented "respawn from last cwd" never worked). Fixed in `85a227e`
(split `m[2]` at first `_`, trim newline); `TestBashNonZeroExitIsSuccessWithMeta` kept
masking the marker path because `exit 3` kills the shell (process-death path) —
`TestBashMarkerExitCode` uses `(exit 4)` (shell survives) instead. The stale colon-marker
comment in shell.go was corrected in the same commit.
74. v0.1.3 SSE-drop re-hydrate (bugfix, P0, 2026-08-22): the TUI had no way to notice
its `/event` stream dropping: the client reconnected silently and the app never
re-hydrated, so events published in the gap were lost — a missed `session.status` kept
the footer spinner on `busy` indefinitely (the user-visible "hang") and the transcript
stayed stale (the user-visible "nothing printed"). Design gap rather than plan
contradiction; fixed in `16a2e13`: `client.Events` now returns a resync ping channel,
the app re-hydrates the current route on `resyncMsg` and re-arms `resyncPump`, and
`server/sse.go` ends the response on a failed write (a dead stream) instead of holding
the bus subscription until ctx cancel. Contract pinned in `internal/tui/AGENTS.md`.
75. v0.1.3 inline bash output (bugfix/parity, P1, 2026-08-22): yolo rendered completed
bash parts as row-only (output behind alt+e) while upstream opencode shows the output
inline (10-line collapse + expand); a correct CI-gate run therefore looked like nothing
was printed. Implemented `headPreview` (10-line head, `…` hint; upstream
`collapseToolOutput` parity) in `da25275`. Same commit fixes a latent render bug found
while adding the case: the expanded-tool empty-output branch used `break`, escaping the
PARTS loop and silently dropping every part after an expanded empty-output tool; it now
`continue`s. The two permission teatest suites needed a taller terminal (80x10 → 80x16)
because the inline preview grows the end transcript to 9 lines beyond the 7-row
viewport — test-only, documented in `permHarness`.
76. Truncated-bash output port gap (bugfix, P0, 2026-08-22): the v1 plan (Task 11,
line 3178) pinned only the `tail()` helper — upstream shell.ts's accompanying
behavior (store the FULL output at `Global.Path.data/tool-output/tool_<id>` and
prepend `...output truncated...\n\nFull output saved to: ${file}\n\n` +
metadata.outputPath, 7-day retention) was never in scope, so truncated arrivals
reached the model silently (tail only, no marker): in session
`ses_EuCqnuD7PTQQxVu5xmFX` the model re-ran the CI gate ~14× ("truncated at the
beginning", cat/tee workarounds). Fixed in `18ea0b6`: `tool.WriteFullOutput` +
`tool.CleanOutputDir` (dataDir/tool-output; startup sweep, 7-day retention),
bash.go verbatim marker + meta.outputPath, engine wires `Env.OutputDir`.
Pinned by `TestBashTruncatedOutputTellsModelWhereFullOutputIs`, which asserts
the marker in the SECOND MODEL ROUND's tool message (the model-visible
contract), not just the stored part.
77. History re-append made the model re-see its user instruction EVERY tool
round (bugfix, P0, 2026-08-22): plan Task 16 LOCKED a "request ends with the
newest user message, re-appended on tool-call rounds" mapping. Upstream
(`message-v2.toModelMessagesEffect`) maps persisted history 1:1 — in a tool
round the request ends with the TOOL result and the user message appears
ONCE. With the re-append, each round re-issued the user prompt verbatim at
the tail, which Qwen3.8-27B read as "the user is asking again" and it re-ran
the CI gate ~14× without ever emitting a final text answer
(session `ses_Mt8jhDCdseSyZjcqVhED`, turn aborted 17:53:32). The same prompt
on the same model in upstream opencode does NOT loop — the decisive diff.
Fix: drop the re-append (`engine.messagesFor`); the request mirrors history
1:1. `TestHistoryReplayIncludesToolResults` now pins tail [user, assistant,
tool] + exactly ONE user message. Residual minor divergences noted, not
changed: yolo sends system entries as separate RoleSystem messages (upstream
joins to one string) and omits reasoning parts on replay (upstream replays
them) — neither is tool-round-specific; revisit if a loop recurs.
