# Yolo — Plan Deviations (append-only audit log)

Per root principle 5 (tests define the contract): when the plan contradicts
itself (or its own test code is buggy), resolve per the last-stated call, fix
the test, and log the resolution here with severity. Rules: numbered and
continuous, append-only — never edit an existing item, supersede with a new
one. Items 1–66 (pre-v0.1.2, frozen): `deviations-archive-v0.1.0.md`.

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
78. Plan Task 1 pins Go <1.26 `runtime/debug` API (info, 2026-08-22): the
pinned `printVersion` reads `bi.Settings["vcs.revision"]` (map) and
`ReadBuildInfo() (…, err)`, but the installed toolchain (1.26.5) returns
`ReadBuildInfo() (*BuildInfo, ok bool)` with `Settings []BuildSetting`
(key/value slice). Adapted: iterate the slice, match `vcs.revision` /
`vcs.time` by key, same output. Behavior identical; no test contract
changed (the pins assert line 1 only).
79. Spec §4 ① is stale (info, 2026-08-22): the spec prescribes "read
s.dataDir under the existing mutex in decisionFor" for the concurrency-1 P0,
but `SetDataDir` was removed in `39a196e` (2026-08-21, before the spec) —
`dataDir` is now a process-constant constructor field, so the race cannot
occur. Verified: no writes to `Service.dataDir` outside `New` (grep), and a
concurrent-sessions `-race` regression test is added instead of the
prescribed mutex fix (plan Task 4). The P0 bead closes as verified-stale.
80. Spec §4 ⑦ says "the 28-entry patterns list" (info, 2026-08-22): the
upstream list in `packages/llm/src/provider-error.ts` has 27 entries. The
plan ports the actual 27 byte-faithfully; the count in the spec is a typo.
81. ④/⑥/⑦ change model-visible error text (low, 2026-08-22): provider
non-2xx errors now surface the DECODED provider message (previously the
generic status text, because the body was closed before read), the
overflow classification narrows to the upstream curated classifier (a 401/403/429
now ends the turn errored with the decoded text instead of a graceful
overflow note when the text matches "context"/"token"-adjacent phrasing),
and bash `timeout: 0` now returns "timeout must be a positive integer"
instead of running unbounded. Per principle 5 the tests define the contract:
the pins in plan Tasks 2/3/11 assert the new behavior.
82. ⑦ patterns also run against the APIError body (low, 2026-08-22): the
plan's implementation snippet matched the curated patterns against
`err.Error()` only, but the plan's own test case "model context window
exceeded code" (body `error.code` with a short message) needs the body text.
Upstream's classifier input (provider/error.ts `message()`) appends the raw
response body when the decoded message is unhelpful — so yolo's
`isOverflowError` tests the patterns against `err.Error()` AND the decoded
`*llm.APIError` body. Test contract kept; snippet adapted.
83. Plan Task 3 e2e test bugs fixed (low, 2026-08-22): (a) both e2e tests
set `h.overrideDriver` AFTER `h.build(t)`, but the harness captures the
driver at build time (engine_test.go `wiredDriver`, seam docs: "set before
build") — moved before build; (b) `TestNonOverflowAPIErrorFailsTurn` read
`turnErr` (written by the async `onDone` callback) without the
done-channel sync the harness convention requires — added the
`close(done)` + select pattern used by the existing retry/abort tests.
84. Pre-stream error paths gained a synthetic note + sentinel (low,
2026-08-22): the plan's test `TestNonOverflowAPIErrorFailsTurn` pins the
decoded provider text on a synthetic note for a PRE-stream 401, but the
plan's implementation only touched `isOverflowError` — the pre-stream
non-transient path returned the error with no note. Added
`e.saveSynthetic(t, r, sErr.Error())` there (mid-stream parity). Also the
overflow path previously returned `PartStream{}, nil`: the round loop
blocked forever on the nil `Parts` channel (the path was dead since ④ made
provider 400s decodable) — it now returns a private `errRoundEnded`
sentinel and the caller ends the turn idle without reading a stream.
85. ⑧ spec's engine-level concurrent e2e satisfied by the concurrent
permission-level unit test (low, 2026-08-22): spec §4 ⑧ calls for
"concurrent sessions with different project rules → no cross-contamination."
A true concurrent engine e2e would flake: the session harness drives all
sessions from one shared `fakellm.Driver` whose `Turns` is a mutex-guarded
FIFO, so two concurrent `Send`s interleave non-deterministically. The
property is pinned instead by concurrent `TestDecisionForUsesRequestCfgRules`
(two rule sets interleaved on the same `Service`, each keeps its own verdict,
`-race` clean); the engine wiring is guarded by the existing engine perm
tests. Resolves per principle 5 (tests define the contract).
86. Go 1.26 stdlib slog TextHandler does not render spec §3's pinned line
format (low, 2026-08-22): (a) `slog.HandlerOptions.TimeFormat` was removed
in Go 1.26; (b) the TextHandler emits `msg` BEFORE handler (With) attrs, so
`With("run", id)` yields `msg=... run=...` — the pinned order is
`run=<8hex> msg=...`; (c) the TextHandler quotes `msg` when it contains
spaces (`msg="serving on"`), but the pinned format is unquoted
(`msg=serving on`). Resolution: `internal/log` implements a small
`pinnedHandler` (slog.Handler) that owns the pinned field order and
quoting — msg unquoted (control chars escaped so a message cannot forge a
line), k=v values quoted/escaped only when they contain chars that could
break the key=value shape (CWE-117 preserved). The plan's
`rotatingWriter` moves verbatim; `New` opens the file eagerly (best-effort)
so the log file exists before the first level-passing write (the pinned
level test reads the file even when everything is filtered).
87. Pinned rotation test's line-count estimate under-shot (low, 2026-08-22):
the plan's `TestRotationTriggersOnSize` writes `(5*miB)/1100 + 2` lines
assuming ~1100-byte lines; actual lines are ~1076 bytes
(`time=...Z level=INFO run=8hex msg=line i=N pad=1000x`), so the total
stays just under the 5 MiB threshold and rotation never triggers. Fixed the
estimate to 1000 bytes/line (`(5*miB)/1000 + 2`), which guarantees the
threshold is crossed. Test bug in the plan's own test; resolved per
principle 5 (tests define the contract — the contract being "a write that
would push the active file past 5MiB rotates").
88. ⑫ glob output order changed from global-sorted to walk-order (low,
2026-08-22): spec §4 ⑫ directs porting upstream's early stop, whose output
is "first 100 in walk order." yolo previously did a full walk +
`sort.Strings` + truncate, so results were globally sorted. The capped walk
(`filepath.SkipAll` at the cap+1th match) returns walk-order (lex
depth-first). This is model-visible (the tool result the model reads). The
existing `TestGlobTool` pins presence only (not order), so no pin breaks.
Also fixed the plan's own test fixture (bug in the plan's test, principle 5):
the pinned `zz_deep/000_first.txt` fixture could not distinguish
full-walk+sort from walk-order (the full path sorts AFTER the `f*.txt`
files, so the old code dropped it too — the pinned test PASSED against the
unfixed code). Replaced with a `zz/` dir (100 files) + root `zz.txt`
fixture, where walk order visits `zz/*` before `zz.txt` (entry "zz" <
"zz.txt") but sort order keeps `zz.txt` first ("." < "/") — the old
implementation fails the corrected test. Resolves per principle 5 (tests
define the contract; the spec's order is authoritative).
89. Quit-confirm dialog text changed `quit? [y/n]` → `quit? [Y/n]` (low,
2026-08-23): user-requested 0.3.0 UX change — the dialog marks the default
choice with a capital letter. The text is yolo-side (upstream's quit
confirm is a button-style Confirm/Cancel dialog, no `quit? [y/n]` literal),
so no verbatim-port conflict; the render text was test-pinned
(help_test.go, tui_suite_test.go) and the pins moved with it. No keymap
change: y/enter/ctrl+c confirmed before this change (dlgYes binding) and
now have an explicit test pin for enter (default-choice semantics).
Resolves per principle 2 (one deliberate deviation per change, logged).
