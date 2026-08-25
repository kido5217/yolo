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
90. Plan Task A test code had two bugs; fixed, assertions kept (low,
2026-08-23): (a) `panicDriver` used a value receiver, so `p.fired.Store(true)`
mutated a copy of the driver held in the `llm.Driver` interface — the
`pd.fired.Load()` "probe was never called" guard could never pass. Fix:
pointer receiver + `h.overrideDriver = &pd`. (b) The final
`eventCount(idle)` ran immediately after `onDone`, but the idle
`session.status` is the LAST publish in the turn's defer (same closure that
fires `onDone`), so it raced the harness's collector goroutine (count read 0
before the collector folded the event). Fix: `h.waitForEvent` on the idle
status before the count. Both are test-only; the pinned assertions (failed
turn, `StatusIdle`, exactly one idle event) are unchanged and now
deterministic. Resolves per principle 5 (tests define the contract; plan's
own test code buggy — fix the test, log the deviation).
91. Send early-fail SSE sequence: the lone `session.status` idle frame on the
Send CreateMessage/UpsertPart error path is no longer published (wire/low,
2026-08-23): finding [concurrency-5] row 3 — the early-fail path published an
idle with no preceding busy, a transition no client observed a start to. Spec
§3.1 B picks "skip both": no status event on early fail; the error reaches the
TUI via the Send return value (the existing failure channel). The wire-visible
change is confined to the Send error path (success paths unchanged). Resolves
per principle 2.
92. Plan Task C test driver had the deviation-90 value-receiver bug class
(low, 2026-08-23): `holdTitleDriver` used a value receiver, so
`d.cancelled.Store(true)` inside `Stream` mutated the copy of the driver
held in the `llm.Driver` interface — the `hd.cancelled.Load()` guards in
TestAbortCancelsTitleGoroutine and TestShutdownCancelsAndWaitsTitle could
never pass. Fix: pointer receiver + `h.overrideDriver = &hd`. Test-only;
the pinned assertions are unchanged. Resolves per principle 5.
93. dropTitleCtx conditional drop (low, 2026-08-23): the plan-pinned
unconditional `delete(e.titleCtx, sessionID)` in the title goroutine's
defer could remove a NEWER title's cancel — a retry that schedules a
second title while the first is still in flight (turn ended without an
assistant message; title call bounded at 30 s) makes the first exit
drop the second's cancel, leaving it uncancellable by Abort/Shutdown
(spec §3.1 C "cancelled by Abort"). Fix: `dropTitleCtx(sessionID,
cancel)` deletes only when `e.titleCtx[sessionID] == cancel`.
Resolves per principle 5 (review finding on plan-pinned shape; spec is
the binding authority). Implementation note: Go forbids `==` between
non-nil func values, so the tracked cancel is stored as a
`*titleCancel` pointer wrapper and the comparison is on the pointer
(identity semantics identical to the stated form); the Abort/Shutdown
call sites invoke `tc.cancel()` / append `tc.cancel` under the same lock
discipline.
94. Delete-while-busy aborts the in-flight turn (behavior/med, 2026-08-23):
findings [troubleshoot-5] + [concurrency-2] Close half — `DELETE
/session/{id}` previously only closed the bash shell; the in-flight turn kept
publishing events for the deleted session and a post-Close tool call
re-spawned a leaked shell. Upstream `Session.remove` (session.ts:606-629)
cancels subagent jobs, removes child sessions and durable event rows, but
does NOT interrupt the in-flight main turn (it runs in the instance-scoped
Runner; `SessionRunState.cancel` is the separate abort path) — upstream parity
is structurally unachievable (yolo has no subagent jobs, no durable event
rows, a per-session engine), so yolo aborts the turn on delete: Close =
Abort + suppress the session's events + settle-then-close the shell (bounded
2 s, then hard close). The user-initiated delete is treated as an abort
intent; partial assistant content persists as today (no synthetic
"aborted" note). Resolves per principle 2 + the implement-everything scope
policy (spec §2.4a).
95. Plan Task D test code had two bugs; fixed, assertions kept (low,
2026-08-23): (a) TestCloseWhileBusyAbortsAndSuppresses called
`waitBusy(t, h, ses, fn)` — a 4-arg form that does not exist (the harness
`waitBusy` takes 3 args and all existing callers use that form), so the
pinned test could not compile. Fix: `Send` before `waitBusy(t, h, ses)` —
the same start-then-act-within-the-busy-window intent. (b) The flag-based
`waitBusy` does not guarantee the busy `session.status` EVENT is published
before `Close` (the busy flag is set synchronously in Send, but the event is
published from the turn goroutine), so the "busy-only statuses survive"
count raced the publish and read 0. Fix: `h.waitForEvent` on the busy status
event for the session before `Close` — matching the plan's own Step 2
expectation that the busy event is delivered pre-Delete. Both are test-only;
the pinned assertions (abort applied, `slowCancel` observed, exactly one
busy `session.status` survives) are unchanged and now deterministic.
Resolves per principle 5 (tests define the contract; plan's own test code
buggy — fix the test, log the deviation).
96. Plan Task E test used the deviation-95(a) 4-arg waitBusy form (low,
2026-08-23): the harness `waitBusy` takes 3 args, so the pinned
TestAbortThenNewTurnCompletes could not compile. Fix: Send before
`waitBusy(t, h, ses)`. Test-only; the pinned assertions are unchanged.
Resolves per principle 5.
97. Tool-round text part ids: post-tool text now starts a fresh part id
(wire/med, 2026-08-23): finding [troubleshoot-3] — the pre-tool
finalizePart did not reset the round's textState, so post-tool deltas
appended to the already-finalized part id and the round-exit finalize
re-persisted + re-published it (≥2 final part.updated frames per text part;
the transcript showed one merged part). Upstream mints a fresh part id per
text block; yolo now resets the text/reasoning state (id + buffer) at the
tool boundary, so post-tool text is a new part and each part finalizes
exactly once. Wire-visible on tool rounds that continue with text after a
tool call: part-frame sequence and transcript structure change (new part id
+ its own start/final frames instead of a re-finalization frame).
Resolves per principle 2 (parity fix; spec §3.1 F).
98. Plan Task F test code inspected the wrong assistant message (low,
2026-08-23): runRound mints a new assistant message per round and a tool
round (sawToolPart) always continues to a round 2, where the auto fake
synthesizes an ok-N text part — the pinned test's "last assistant message"
selection read round 2's message, never the tool round's parts. Fix:
collect text parts across all assistant messages into a text→id map and
assert "before"/"after" present with distinct ids; the session-wide
per-id ≤2 part.updated frame check is unchanged. Test-only; the pinned
contract (fresh part id per text block, no re-finalization) is unchanged.
Resolves per principle 5.
99. git-repo cache now expires (behavior/low, 2026-08-23): finding
[design-7] — the package-global git-dir cache was unbounded and non-expiring:
 a dir that became a git repo mid-process stayed "no" forever in every env
 block. Now bounded (1024 entries; cap breach drops the cache) with a 60 s
 TTL. Upstream check: `Is directory a git repo` renders from
 `ctx.project.vcs` (system.ts:79) — a persisted project property detected at
 project scan, static per instance, never re-checked; yolo's per-process cache
 is the faithful equivalent, so the expiry is a deliberate deviation from the
 upstream static answer. Model-visible only in the env-block line for a dir
 whose git status changes mid-process (previously permanently stale "no", now
 correct within 60 s). The pinned env-block text is unchanged. Resolves per
 principle 2 + the implement-everything scope policy (spec §3.1 G).
 100. Plan Task G test set the short TTL after the first gitRepo call
 (low, 2026-08-23): the pre-init "not a repo" check inserted a "no" entry
 whose expiry was computed with the default 60 s TTL before the test
 shortened `gitCacheTTL` to 30 ms, so the 2 s expiry poll would time out
 post-fix. Fix: shorten the TTL (with cleanup restore) before the first
 `gitRepo` call. Test-only; the pinned behavior (a cached "no" expires once
 the TTL passes) is unchanged. Resolves per principle 5.
101. Plan Task I test code referenced a nonexistent message row (low,
2026-08-23): the pinned test mints parts with `MessageID: "msg_t"`
(`const ses, msg = "ses_t", "msg_t"`) but never creates a `msg_t` message
row; with `_foreign_keys=1` in the DSN the `UpsertPart` insert fails
(FK `part.message_id REFERENCES message(id)`), so the test could never
pass pre- or post-fix. Fix: point the parts at the real `msg_a` row
(one token, `const ses, msg = "ses_t", "msg_a"`). Test-only; the pinned
contract (same-time_created rows come back in insertion/rowid order) is
unchanged. Resolves per principle 5.
102. Plan Task K brief put the live turn ctx at the finalization sites
(low, 2026-08-23): the brief's ctx-source map said the turn ctx at every
site inside `finishRound`/`finalizePart`/`saveToolPart`; implemented
literally, the pre-existing abort tests
(TestPermissionAbortDuringAskAbortsTool, TestAbortMidTurn) failed —
the finalization DB writes ran on the already-cancelled turn ctx, the
driver failed them with `context.Canceled`, and the tool part stayed
`running` (End:0) in the store, while the abort contract pins the
finalized `error: aborted` part. Fix: `context.WithoutCancel(ctx)` in
exactly those three finalization functions (internal/session/engine.go) —
values preserved, cancellation dropped; `saveSynthetic` keeps the plain
ctx (its call sites check `ctx.Err()` first and return before it), and
all cancellable work (stream Next, perm Ask, tool Run, round-start
loads) still uses the live turn ctx. Impact: round-finalization writes
are no longer interruptible by turn cancel — deliberate: they record the
terminal state of an exiting round. No wire change. Resolves per
principle 5.
103. Plan Task M test code + brief were inconsistent (medium, 2026-08-23):
(a) the brief's TestReapProcWaitFailureIsNegative set `stdin` to
`io.NopCloser(io.Discard)` — an `io.ReadCloser`, but `shellProc.stdin` is
an `io.WriteCloser`; the test would not compile. Fixed with a no-op
WriteCloser (test-only). (b) The brief expected both pin tests to PASS
pre-fix, but TestShellSelfKillReportsExitCode failed pre-fix with code =
-1: Go's `(*exec.ExitError).ExitCode()` returns -1 for a signal-terminated
process, so the old reapProc already conflated "killed by signal" with
"Wait failed", and the brief's Step 3 snippet (code < 0 → error in the
markerless-EOF branch) would have turned real signal-kills (137/143) into
errors — contradicting the brief's own interface note ("real exit codes —
including 137/143 from a signal — still surface as meta[\"exit\"]") and the
test (err == nil, code 137). Fix: reapProc now maps a signal-terminated
process to 128+signum (syscall.WaitStatus), matching the marker path where
bash's `$?` reports the same value; -1 is reserved for a Wait failure with
no exit status, and only that becomes the tool error in the markerless-EOF
branch. Resolves per principle 5 (test + interface note = the last-stated
call).
104. write/edit added/removed counts: DP-LCS replaced by the go-udiff Myers
line diff (behavior/low, parity fix, 2026-08-23; the plan assigned this
entry the number 94, already in use — next free is 104): finding
[security-3] — the O(len·len) DP on every write/edit blocked the engine
~tens of seconds on a one-line edit of a ~60k-line file (~3.6e9 cells).
`diffCounts` now uses `udiff.Lines` (go-udiff v0.4.1 — dep proposal #1,
user-approved 2026-08-23; BSD-3, zero deps, pure Go) and derives
added/removed from the edit list. The counts are the optimal
newline-terminated-line edit — verified 1:1 against the upstream jsdiff
`diffLines` line model on all 10 `TestDiffCountsPins` cases. Three former
pins changed because the OLD DP (bare-line `strings.Split`) deviated from
upstream: `{"x",""}` (1,1)→(0,1), `{"a\n","a"}` (0,1)→(1,1),
`{"p\nq\nr","q\nr\ns"}` (1,1)→(2,2). Model-visible in the write/edit tool
meta (added/removed) only for those edge shapes. Resolves per principle 2
(parity fix; spec §3.3 N).
105. Plan Task O test/helper would not compile (test-only/low, 2026-08-23):
both the one-off hash helper and TestGlobSchemaEmittedBytes sliced the
return of `sha256.Sum256(b)` directly (`sha256.Sum256(b)[:]`) — Go forbids
slicing an unaddressable array value. Fixed by binding first
(`sum := sha256.Sum256(b)` then `sum[:]`). Emitted schema bytes, the pin
value, and the style fix are all as the brief specifies. Resolves per
principle 5.
106. Plan Task S guard test used the wrong route (test-only/low,
2026-08-23): the brief built the app with `newRecApp(client.New(...),
store.Store{}, "")` — a HOME-route app — but the home route's Enter is
consumed by `homeEnter()` (home.go) before it reaches `promptEnter`, so the
soft-enter loop never accumulated the draft (red run: draft length 0, not
the intended timing failure). Fixed by driving a SESSION-route app via
`testSessionApp(sessionFixture())`, matching the existing soft-enter
subtest in the same file. The amortization, the two switched draft
assertions, and the 5 s guard are as the brief specifies. Resolves per
principle 5.
107. Plan Task T implementation snippet would not compile (impl/low,
2026-08-23): the brief's `case tea.InterruptMsg: return a.handleKey(…)`
returned the `[]tea.Cmd` from `handleKey` where `Update` must return a
`tea.Cmd`. Fixed by mirroring the adjacent `tea.KeyPressMsg` case:
`cmds := a.handleKey(…); if len(cmds) == 0 { return nil }; return
tea.Batch(cmds...)`. The synthetic key is the locked `ctrlCKey` shape
`{Code:'c', Mod: tea.ModCtrl}` (home_test.go:44) and the precedence
behavior (pending perm ask / open dialog still owns the keys) is exactly
as the brief specifies. Resolves per principle 5.
108. wire-DTO bool JSON tags renamed to predicate form (wire/medium,
2026-08-23; the plan assigned this entry the number 95, already in use —
next free is 108): finding [naming-9] — the protocol bool fields lacked
is/has predicate form. Renamed: ProviderAuth.KeyRequired→RequiresKey
(`keyRequired`→`requiresKey`), Model.ToolCall→SupportsToolCall
(`toolcall`→`supportsToolCall`), Model.Reasoning→SupportsReasoning
(`reasoning`→`supportsReasoning`), Model.Attachment→SupportsAttachment
(`attachment`→`supportsAttachment`), Agent.Hidden→IsHidden
(`hidden`→`isHidden`), Part.Synthetic→IsSynthetic
(`synthetic`→`isSynthetic`), Part.Ignored→IsIgnored (`ignored`→`isIgnored`),
PermissionRepliedProps.Auto→IsAuto (`auto`→`isAuto`). Server and TUI are
one module: every encoder and decoder moved in the same commit, so the
in-process wire is consistent; the deviation is against the
upstream-mirrored JSON shapes (principle 2), accepted by spec §3.5 V / §5.
Pinned by protocol.TestWireBoolTags; server golden provider.json
regenerated (four key renames only, values unchanged). Resolves per
principle 5 (spec-directed wire change).
109. Plan Task V1 tag-pin test would not compile (test-only/low,
2026-08-23): the brief's case rows were parenthesized tuples
(`(protocol.Part{}, "IsSynthetic")`) — not a composite value in Go — and
`f := reflect.TypeOf(c.typ).FieldByName(c.name)` ignored the method's
second return value. Fixed to positional composite rows
(`{protocol.Part{}, "IsSynthetic"}`) and
`f, found := …; if !found { … }`. The eight predicate renames, JSON tags,
and the lowerCamel prefix assertion are as the brief specifies. Resolves
per principle 5.
110. client sentinel error text gains the "client: " package prefix
(render/low, 2026-08-23; the plan assigned this entry the number 96,
already in use — next free is 110): finding [naming-3] — the sentinel
messages ("not found", "session busy", "bad request") carried no package
origin, lost when wrapped, and their text is what the user sees in the
status line. client.ErrNotFound/ErrBusy/ErrBadRequest now read
"client: not found" / "client: session busy" / "client: bad request";
errors.Is matching is value-based and unchanged. The visible
status-line/toast text changes (e.g. "client: session busy: <server
message>"). Pinned by client.TestSentinelPrefixes. Resolves per principle
5 (spec-directed render change).
111. Plan Task V7 test pinned in the external test package (test-only/low,
2026-08-23): the plan slice pinned TestZeroDialogIsNotQuit in
internal/tui/app_test.go, which is `package tui_test` (external, since
creation) — the white-box test cannot compile there (undefined: newRecApp,
dialog, dlgQuit, press; all unexported internals). The test moved
unchanged to internal/tui/rec_test.go (`package tui`, home of newRecApp);
its `store.Store{}` is Task V5 drift, corrected to `store.State{}`.
Everything else (dlgNone zero value, explicit handleDialogKey guard,
DEFERRED.md disposition, pinned commit message) lands as planned. Resolves
per principle 5.
112. Plan Task W step-1 test literal was malformed (test-only/low,
2026-08-24): the script JSON literal was missing the closing `]` of the
outer array (`...}}]}` instead of `...}}]}]`), so the fake driver's
eager script parse (`fake.FromScript` at buildDeps) failed at startup
and the child exited 1 with `yolo: fake: parse script: unexpected end of
JSON input` before `tea.Run` was ever reached — the plan's predicted
step-2 failure ("stderr empty") did not occur. Fixed by appending the
one missing character; the test pins W (row 12) as designed. Resolves per
principle 5.
113. Plan Task X test code had two compile errors (test-only/low,
2026-08-24): (a) the `Drivers` map literal omitted its key —
`map[string]llm.Driver{hangDriver{gate: gate}}` cannot compile; the
intended key is the provider id `"kido"` (the session model is
`kido/q`; the same seam key `buildDeps` uses) — without it the hang
driver would be unreachable and the test would not pin X; (b)
`db.CreateSession(...)` was missing the ctx argument (real API
`CreateSession(ctx, r)`, internal/storage/dao.go — identical
semantics). Both fixed in `TestServeDrainForceKill`; everything else
(`drainCtx`/`armForceKill`, the `serveCmd` wiring, the 2 s force-kill
bound, DEFERRED.md disposition, pinned commit message) lands as
planned. Resolves per principle 5.
114. Plan Task AC benchmark code needed two fixes (test-only/low,
2026-08-24): (a) the plan's `st := &store.Store{}` does not compile —
the type is `store.State` (store.go:15; the same Task V5 drift noted
in 111), corrected in `BenchmarkStoreApply`; (b) the plan's
`message.updated` prop map omitted the top-level `sessionID` that
`Apply` gates on (store.go:50 `isCurrent`), which would have silently
dropped every `message.updated` and left `upsertMessage` unmeasured —
the plan's own verification step directs adjusting the maps to the
wire names the store decodes, so `"sessionID": "ses_bench"` was added
per that instruction. Everything else (part/delta maps, event-type
constants, hermetic no-baseline-claim scope, pinned commit message)
lands as planned. Resolves per principle 5.
115. Plan 1 close-out -race gate vs a pre-existing wall-clock
threshold (test-only/low, 2026-08-24): the close-out's full gate
(`go test -race ./...`) failed in `TestDraftSoftEnterAmortized` —
40k soft-enters take 9–10.5 s under the race detector against the
pinned `< 5s` bound (1.3 s without `-race`; ~8x slowdown on this
string-heavy path). The test and its code path (draft growth,
datastruct-9) predate Plan 1 and were untouched by its 39 tasks.
Fix: the bound is now `draftAmortizedLimit` — 5 s via a
`//go:build !race` file, 40 s via a `//go:build race` file (Go sets
the `race` build tag when `-race` is passed; verified on go1.26.7).
The iteration count is unchanged, so a quadratic draft re-scan
(minutes under race) still fails the relaxed bound; the bound only
absorbs the instrumentation slowdown. Resolves per principle 5.
116. Plan 2 R3 (refactor-2) runRound line target vs the named
extracts (test-only/low, 2026-08-24): the plan's Step 4 gate required
`runRound` ≤ 75 lines (the spec row's "~60-line round driver"), but
both the plan's Step 3 and the spec row keep the round exit paths
(err/p.Err switch, tool budget drop, final-overflow block — ~105
lines) inside the driver; the two named extracts (`partState`,
`streamWithRetry` = the `openStream` rename) landed verbatim, leaving
`runRound` at 139 lines. Resolution: the named extracts are the
contract, the line count a stale estimate (consistent with the plan's
global constraint "locate by name, never by line number"); no
behavior change, pins green UNMODIFIED (engine_test.go +
engine_perm_test.go). The same gate's `grep -rn openStream` → 0 check
caught a stale doc ref in engine.go:30 left behind by the R2 split;
fixed in the same commit as this entry. Resolves per principle 5.
117. Plan 2 R16 (refactor-15) payload ownership vs the
"tests UNMODIFIED" pin (test-only/low, 2026-08-24): the R16
spec notes pin app_test.go/model_test.go/agent_test.go as
green UNMODIFIED, but those suites reference the App-owned
modelDlg/agentDlg fields directly (41 field reads/asserts,
e.g. `a.modelDlg.selProv`, `a.agentDlg.hasSubChoice`), so the
field removal required by the plan's own Steps 3/4 (App loses
the two fields; `grep a.modelDlg|a.agentDlg` over
`internal/tui/*.go` → 0 matches, tests included) could not
compile without rewriting those references. Resolution:
mechanical rewrite to item access — field reads become the
new `dialogStack.model()`/`agent()` accessors in the tests
and `top().model`/`top().agent` where the top item is already
in hand; every assertion, key sequence and expectation stays
semantically identical, and the teatest scenarios
(`TestTUIModelDialog`/`TestTUIAgentDialog`) are byte-for-byte
untouched. No behavior/wire/render change. Resolves per
principle 5.
118. Plan 2 close-out -race gate vs brittle contiguous-substring
WaitFors in the TUI prompt suites (test-only/low, 2026-08-24): the
close-out's `go test -race ./...` is timing-sensitive in
`TestPromptSendAndSlashMenu` — its render-sync gate at app_test.go:172
waited for the literal contiguous bytes `"User: hello"`, but the TUI
renders the transcript incrementally: under the race detector's
slowdown the footer (`● live`) or the fake driver's reply (`ok-1`)
updates in the same frame and its CSI cursor-jump sequences
(`\x1b[24;36H`, `\x1b[7G`) land between `User:` and `hello`, so the
contiguous substring never appears in a single WaitFor slice. The
interleaving is load-dependent: non-`-race` and isolated `-race` runs
pass, but it failed once during the full-module `-race` run and passed
5/5 in isolation. The test's real contract (the user message landing on
the server, asserted via `testutil.LastMessages` at app_test.go:175-188)
is untouched. Fix: follow the file's own established idiom
(`TestSessionStreamingViewport` app_test.go:123-126,
`permission_test.go:232`) — strip SGR via `stripANSITest`, then match
the label and text as INDEPENDENT tokens
(`strings.Contains(s,"User:") && strings.Contains(s,"hello"|"first")`),
which is robust to the CSI interleaving (each token is contiguous and
present). The same hardening (strip SGR, keep the single-span phrase) is
applied to the sibling brittle WaitFors in the same file: `"User:
first"` (app_test.go:231, identical transcript mechanism), `"Hello ·
kido/q"` (:56) and the busy toast `"abort or wait (esc aborts)"` (:237).
No behavior/wire/render change; every key sequence, scenario and
server-side assertion stays identical. Verified: gofmt + `go vet` +
`go test ./...` green, full-module `go test -race ./...` green,
`internal/tui` `-race -count=5` green. Resolves per principle 5.
119. v0.4.0 plan Task 3 edit list vs the Task 4 byte-verbatim grep
(docs-only/low, 2026-08-24): the v0.4.0 plan contradicted itself —
Task 3's edit list (four exact edits to the internal DOX chain) omitted
two `byte-verbatim` package-map lines in `internal/AGENTS.md` (:34,
:39), while the plan's own Task 4 closeout grep mandated zero
`byte-verbatim` matches across `internal/` — impossible without changing
those lines. Resolved per the last-stated call (controller ruling
recorded in the SDD ledger): the implementer's minimal reword to
"sha256-pinned" (the restated principle-3 term) was adopted and folded
into Task 3's commit (`docs: restate internal DOX chain to the v0.4.0
policy`) via a local-only amend. No test or code impact.
120. v0.4.0 spec §2.3 pinned-file count vs the actual pre-fix pin surface
(spec-accuracy/low, 2026-08-24): spec §2.3
(`docs/superpowers/specs/2026-08-24-v0.4.0-design.md`) asserted "21
sha256-pinned files" (14 prompt + 7 tool desc, "per-tool
`TestDescPinned`"); the actual pre-fix pin surface was 15 — a single
`TestDescPinned` (`internal/tool/read_test.go`) pinned only
`desc/read.txt`, the other six desc files (embedded package string
vars, model-visible) were unpinned; principle 3's inherited claim that
all tool `desc/*.txt` are sha256-guarded was inaccurate for 6 of 7.
User decision (2026-08-24, final whole-branch review's Important
finding): add the six missing desc pins on this branch rather than
reword the docs. `TestDescPinned` is now table-driven with one subtest
per file; the "21" count and "per-tool `TestDescPinned`" are accurate
as written.
121. Profile support: global config under `~/.config/yolo/<profile_id>/` with an active marker + `yolo profile` CLI (hard deviation/high, 2026-08-24): user-requested feature with no upstream opencode counterpart (closest prior art, consulted during design: the external `kdcokenny/ocx` profile manager — profile subdirs under the app config root, implicit `default`, per-run override chain; naming model per the Kubernetes identifiers design: user-facing name + system-generated short random id). Design approved in-session 2026-08-24 (beads `yolo-3pe`, branch `feat/profiles`): a profile is a dir named by an auto-generated id (8 lowercase hex, `crypto/rand`, collision-retried; the first-run profile is the literal `default`); display metadata lives in an optional `"profile": {name, description}` element of the profile's own config file (effective name falls back to the id); selection precedence is `--profile` flag (on `yolo` and `yolo serve`) > `YOLO_PROFILE` env > `~/.config/yolo/active` marker > `default` recovery (first run creates root + `default` + marker); CLI `yolo profile list | add [name] [-d DESC] | use <id_or_name> | remove <id_or_name> | copy <src> <name> [-d DESC]`; names are unique (rejected at write time), references resolve id-first then name (a duplicate effective name is an ambiguous error); removing the active profile falls back to the first remaining by name or a recreated `default`; legacy flat files directly in `~/.config/yolo/` are NOT migrated — ignored (user decision); the data dir (auth.json, SQLite, logs, tool output) stays shared across profiles (user decision). Implementation: `internal/config/profile.go` (state/lifecycle), `protocol.Config.Profile` (omitempty — no wire-shape change), profile-aware `Loader.Load` / `Server.globalDir()` / `buildDeps`; `TestGlobalConfig` re-pinned to the profile-dir path. No new dependencies; cobra explicitly deferred (stdlib `flag` retained).
122. backgroundMenu fallback when backgroundElement is absent (behavior/low, 2026-08-25): upstream resolveTheme leaves backgroundMenu undefined when neither backgroundMenu nor backgroundElement exists; yolo's S0.2 golden edge test pins the fallback chain backgroundMenu → backgroundElement → background (the test defines the contract, root principle 5). All 33 embedded assets define backgroundElement, so the behavior is observable only for synthetic/custom themes.
123. S0.2 brief vs upstream in the ANSI edge test and the oracle (test-accuracy/low, 2026-08-25): the S0.2 brief's TestResolveEdgeCases/ansi-cube-and-ramp pinned AnsiToRgba(195)=#d75f5f, AnsiToRgba(231)=#fffffe, AnsiToRgba(255)=#ffffff; all three contradict the brief's own oracle and the true upstream (opencode v1.18.18 packages/tui/src/theme/index.ts:301-340; @opentui/core 0.4.5 ansi256IndexToRgb: cube levels [0,95,135,175,215,255], grayscale 8+(i-232)*10): 195=#d7ffff (the brief's #d75f5f is ANSI 167), 231=#ffffff, 255=#eeeeee. Oracle/upstream values win (strict-copy bar); the three expectations corrected per principle 5. Separately, the brief's oracle omitted the upstream `if (c instanceof RGBA) return c` guard (theme/index.ts:244) and crashed after writing theme-golden.json; the guard (`if (Array.isArray(c)) return c`) was added — theme-golden.json is byte-identical with and without it. 33x2 matrix unaffected either way (no asset uses ANSI ints).
