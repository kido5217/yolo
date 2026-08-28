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
124. lipgloss v2 re-baseline of the S0.3 style-accessor tests (test/low, 2026-08-25): the S0.3 brief's styles_test.go was written for the lipgloss v1 API (lipgloss.Color as `type Color string`, GetForeground/GetBackground returning it, unset == ""), but the allowlist pins lipgloss v2.0.6 where Color is the constructor `func Color(s string) color.Color` (no Color type), the getters return image/color.Color (unset = lipgloss.NoColor{}; a 6-digit hex parses to an opaque color.RGBA) — the pinned test cannot compile against the pinned dependency. Per principle 5 the test was re-baselined to the v2 types: got color.Color field, hex comparison via the interface RGBA() method, unset detected by a lipgloss.NoColor type assertion. All hex expectations and alpha semantics are unchanged; the brief's implementation block was already v2-compatible and is unchanged.
125. S0.4 oracle fidelity fixes + system-golden regeneration (test/low, 2026-08-25): three bugs in the S0.2 oracle (scripts/tui-theme-golden.mjs) surfaced in S0.4: (a) terminalMode compared 0-255 luminance (int channels) against 0.5, so every non-black background resolved to "light"; the upstream terminalMode (packages/tui/src/theme/index.ts:353-358) compares 0-1 luminance against 0.5 (RGBA fields are 0-1: @opentui/core@0.4.5 get r() { return (buffer & 0xff) / 255 }) — caught at the #7f7f7f boundary (127/255 = 0.498 → upstream "dark", oracle "light"); (b) the system fixtures passed hex strings where generateSystem expects int 0-255 arrays, corrupting every bg-derived token in the checked-in system-golden.json (NaN → 0 → #000000ff); (c) grays/tint kept NaN alive in arrays where upstream RGBA.fromInts collapses NaN→0 at construction (toByte: Number.isFinite(v) ? v : 0) — visible only at black-bg + light-mode, where 0/0 = NaN makes upstream-faithful diffAdded/RemovedLineNumberBg #001200ff/#120000ff (tint of the collapsed-black gray with the fixture's palette[2] #008000 at 0.14), not #000000ff. The oracle was fixed (0-1 terminalMode comparison, hexToRgb-wrapped fixtures, NaN collapse at grays/tint construction preserving floor semantics) and terminal-mode-golden.json (one entry: #7f7f7f light→dark) plus system-golden.json (fully regenerated) were rewritten; theme-golden.json is bit-identical. The brief's Go port was already upstream-faithful in all three places (controller-verified the NaN chain on x86_64: math.Max(NaN,0)=NaN → uint8(NaN)=0 → gray #000000, then tint with #008000 at 0.14 → #001200/#120000, matching the upstream node chain) and is unchanged.
126. S0.4 test + implementation re-baseline (test/low, 2026-08-25): the S0.4 brief's test line 74 used a bare composite literal in an if condition (c != Rgba{0, 0, 0, 0}), a Go syntax error — composite literals need parentheses in expression context; parenthesized. The brief's Step-3 implementation resolved empty-palette bg/fg via FromHex("") → the magenta invalid-hex sentinel, but the brief's own Interfaces note ("missing palette entries fall back to the ANSI table, missing default bg/fg to palette[0]/palette[7]") and its own TestGenerateSystemPaletteFallbacks pin the ANSI-table fallback (text = AnsiToRgba(7), background = transparent black); upstream is undefined for this input (@opentui/core@0.4.5 hexToRgb runs hex.replace and throws TypeError on undefined), so the spec text + test settle the port decision in favor of the ANSI fallback — per principle 5 the implementation was fixed to a col(0)/col(7) fallback, keeping the upstream `default ?? fallback` shape.
127. S0.5 test re-baseline (test/low, 2026-08-25): the S0.5 brief's TestDetectPaletteFullResponse asserted DefaultBackground/DefaultForeground == "#c0c0c0" with the comment "hex16[11%16] = #c0c0c0", but the test's own fixture has hex16[10] = #00ff00 (OSC 10 → DefaultForeground) and hex16[11] = #ffff00 (OSC 11 → DefaultBackground) — #c0c0c0 is hex16[7]; the assertions contradicted the test's own response stream. The two assertions were corrected to #00ff00/#ffff00; the implementation is unchanged (OSC 10/11 semantics verbatim from upstream).
128. OSC palette-detection timeouts 100ms/100ms/100ms vs upstream 300ms/300ms/5s (behavior/low, 2026-08-25): approved spec 2026-08-24-opencode-tui-parity-design.md §3 pins "~100 ms timeout; no response → no system theme" and risk 3 pins "hard 100 ms fallback"; upstream `@opentui/core` 0.4.5 `terminal-palette.ts` uses 300 ms probe / 300 ms idle / 5 s hard. The port keeps the upstream MECHANICS verbatim (probe OSC 4;0, 16+9 queries, tmux wrapping, rgb: scaling, 8192/4096 buffer cap) with the spec-pinned constants; worst-case startup block ~200 ms; partial palettes remain usable (only palette[0] gates the system theme, upstream theme.tsx:159). `theme.PaletteOptions` keeps all three timeouts overridable — restoring the upstream constants is a one-line change.
129. S0.5 DetectStd restructure: /dev/tty-only probe + probe-answer semantics note (behavior/low, 2026-08-25): the S0.5 brief's DetectStd char-device branch (stdin+stdout ttys) read os.Stdin through a pump goroutine without ever putting stdin in raw mode — two coupled defects on the normal startup path: (a) in canonical (cooked) mode Read(os.Stdin) blocks until a newline and the BEL-terminated OSC probe answer contains none, so the probe could never complete within the 100 ms timeout and DetectStd always returned false on a plain interactive terminal (system theme silently dead on the path it exists for); (b) the pump goroutine lingered in Read(os.Stdin) after return and stole the user's first input line. DetectStd now opens /dev/tty (O_RDWR) as the sole probe path — owned fd, raw mode via x/term with restore + close on exit (no lingering readers, no fd-reuse risk) — and returns (TerminalColors{}, false) when /dev/tty is unavailable (no controlling terminal → no system theme, spec §3 fallback; matches the brief's own Interfaces note, 'raw-mode /dev/tty wrapper'). Related semantics note: the single-buffer demux (brief-mandated architecture) stores the probe's OSC 4;0 answer as Palette[0] first-wins, whereas upstream runs the probe in a separate buffer and discards its answer — test-pinned by TestDetectPaletteRGBScaling, indistinguishable on real terminals (one answer per index, identical values); only palette[0] PRESENCE gates system-theme eligibility (S0.7). Also added: idle-timer, non-legacy no-wrap, and 16-bit rgb: scaling tests (coverage holes in the brief's test file), and a DetectPalette doc line requiring in to eventually EOF/error.
130. config.theme wire field type change (wire/low, 2026-08-25): spec §3's selection chain (config > KV > default, upstream theme.tsx:121-122) requires the config `theme` to be a theme NAME string; the ported wire had `protocol.Config.Theme map[string]any` — a verbatim mirror of the upstream opencode config, whose `theme` field is a legacy object toggle (e.g. `{"dark": true}`), never read by the upstream TUI's selection chain. Change: `Theme string` (`json:"theme,omitempty"`), the ONE sanctioned wire deviation for S0 (plan 2026-08-24-opencode-tui-parity, Task S0.7). Re-baselined in the same commit (root principle 3): `internal/config/config_test.go` (fixture `"theme":"opencode"` + string assertion). Blast radius (grep-verified before the change): no server handler reads `.Theme`; no `internal/server/testdata/` golden encodes the old map shape; `internal/config` code is a generic JSON round-trip and needs no change. Spec §10 "no new endpoints" unaffected (field type only). Root principles 2+3: explicit user-sanctioned deviation, logged here.
131. Theme palette probed exactly once at startup (behavior/low, 2026-08-25): upstream re-probes the terminal palette on refresh (theme.tsx:181-200 clears the palette cache and re-issues the OSC queries through the opentui renderer's input pipeline); yolo's bubbletea program owns the tty while running, so a mid-session raw-mode re-probe is not possible without pausing the program. The engine (`theme.Engine`) probes once in `Resolve` and re-resolves on the CACHED palette + fresh customs discovery for all later events (Apply/Free/Reapply/RefreshCustoms/SIGUSR2). Upstream parity check (S8) may revisit.
132. S0.7 plan deviation-entry numbering is stale (plan-accuracy/low, 2026-08-26): the plan's Task S0.7 prescribes appending "entries 123 and 124 to DEVIATIONS.md (after entry 122)", but at execution time entries 123-129 already existed (landed in S0.2-S0.5) with different content. Per the append-only, continuous-numbering rule the plan's two entries were appended as 130/131 with their content verbatim, and the cross-reference in the re-baselined `internal/config/config_test.go` assertion points to entry 130 (the plan's "deviation 123" reference).
133. S0.7 plan test bug: int 0 vs float64(0) in the KV falsy-preservation assertion (test-accuracy/low, 2026-08-26): the pinned TestKVGetSetAndNilDelete does `kv.Set("zero", 0)` (Go int) then asserts `kv.Get("zero", "dflt") != float64(0)` — but the KV's in-memory store is the source of truth and holds the Go value as set (upstream kv.tsx `setStore` likewise holds the JS value verbatim; the JSON round-trip happens only on flush/reload, where numbers decode as float64). The same-instance Get returns int 0, so the pinned assertion could never pass. Per principle 5 the test was fixed minimally: expect `0` (int, the value as set). The `??` falsy-preservation semantics are unchanged and remain pinned (false / "" / 0 all return from Get instead of the default).
134. S0.8 plan test bug: missing space in the zero-theme logo expectation (test-accuracy/low, 2026-08-26): the pinned TestRenderLogoZeroThemeIsPlain `want` line 2 right block was "█   █  █ █  █ █▀▀▀" (3 spaces after the leading █), but the verbatim port of upstream logo.ts right line 2 ("█___ █__█ █__█ █^^^" = █ + 3 underscores + 1 separator space) with the logo.tsx mark translation ('_' → " ") produces 4 spaces after the leading █ ("█    █  █ █  █ █▀▀▀"). Strict-copy bar (upstream source is the reference; glyphs follow it) — the port's output wins; the test expectation was corrected minimally (3 → 4 spaces). The 8-line sha256 pin is unaffected: it pins the raw source lines, not the translated glyphs, and matches the verbatim upstream port (3132b810…9e2ee — no re-baseline needed).
135. S0.9 plan rowLines fast path vs the pinned join-split test (test-accuracy/low, 2026-08-26): the plan's rowLines code block returns a FITTING row (runeWidth(prefix+title+meta) <= w) verbatim as {cur: body, title, meta}, but the plan's own pinned TestHomeRowLines "join split" case (rowLines("  ", "T1", " · kido/q · 2m", 80) — a fitting row) expects title="T1 ", meta="· kido/q · 2m", i.e. the title/meta join space attributed to the PRECEDING run, exactly the joinRowLine contract ("a join space belongs to the PRECEDING word's run") and the test doc comment ("a mid-line boundary leaves the trailing join space on the title run"). Per principle 5 the test defines the contract: the runeWidth(plain) <= w fast-path condition was dropped — the tagged-word re-derivation (joinRowLine) applies to EVERY visual line, including the single line of a fitting row. The only behavior delta vs the plan code is run-boundary attribution (and the wrapLine-consistent single-space rejoin) on fitting rows; no pinned golden changes (TestHomeRenderLockedLayout and the SGR goldens stay green).
136. S0.9 plan implementation block does not compile as written (plan-accuracy/low, 2026-08-26): four non-compiling details in the plan's Step-3 code, each fixed minimally with no behavior change: (a) wTag is declared inside rowLines but referenced by the package-level joinRowLine (and by the pinned Interfaces signature joinRowLine([]wTag) rowLine) — hoisted to a package-level type; (b) writeRowLine's parameter cursor bool shadows the package-level cursor static that its own zero-Theme fallback references (cursor.Render(l.cur) — type bool has no field Render) — parameter renamed selected; (c) writeRowLine declares bg in the if bg, ok := th.Color("primary"); !ok { … } scope and uses it after the scope (bg.Hex() for the background style) — declaration hoisted above the if; (d) the file list's "footer.go — the import block gains the theme import" is unneeded — a.theme.TextMuted() is field access, not package qualification, so the import would be unused; omitted.
137. S0.10 plan test bug: `string(st.GetForeground())` does not compile (plan-accuracy/low, 2026-08-26): the plan's TestToolRowLineTheme asserts `string(st.GetForeground()) == tt.fgWant`, but the allowlist's lipgloss v2.0.6 `Style.GetForeground()` returns `image/color.Color` (the interface, get.go:65), not a string — the 24-bit hex stored by `Foreground(lipgloss.Color(hex))` lands as an opaque `color.RGBA` (color.go parseHex). Per principle 5 the test was fixed minimally: the foreground is compared BY VALUE against a `color.RGBA` parsed from the same `#rrggbb` hex (a `hexColor` helper in session_theme_test.go); the pinned hex values (#808080/#eeeeee/#e06c75) and the state→token chain they pin are unchanged.
138. S0.10 plan test bug: `\u` escapes in the SGR regexes panic at init (test-accuracy/low, 2026-08-26): the plan's completedRowRe/errorRowRe/liveDotRe are Go RAW strings containing `\u2713`/`\u2717`/`\u25CF` — Go's regexp has no `\u` escape (the `\x1b` hex escape IS supported), so `regexp.MustCompile` panics at package init (`error parsing regexp: invalid escape sequence: \u`). The three regexes were fixed minimally by inlining the literal UTF-8 runes (✓/✗/●) — pattern semantics unchanged.
139. S0.10 plan test bug: zero-theme `want` uses the plain divider (test-accuracy/low, 2026-08-26): the pinned TestSessionChromeZeroThemeIsPlain `want` splices the plain `dividerLine()`, but `renderMessages` draws the divider through the surviving STATIC `divider` style (style.go — the plan's own "style.go final shape" keeps `title` + `divider` after S0.10), so the divider run always carries its ANSI-240 SGR even under a zero Theme and the byte-exact comparison could never match. Per principle 5 the test was fixed minimally: `want` inlines `divider.Render(dividerLine())`. The byte-exact `want` still pins the absence of every theme-token SGR (any other SGR fails the equality).
140. S0.10 plan re-baseline scope: once/always subtests pin whole-row adjacency (test-accuracy/low, 2026-08-26): the plan's Task S0.10 re-baseline list names only the `TestTUIPermissionFlow/reject` subtest, but the zero-engine `once`/`always` subtests pinned `hasLines("\u2713 bash", "all done")` — the cell-diff renderer re-emits only the changed icon cell (▶→✓) for an unstyled (zero-engine) row, so the adjacency "✓ bash" never appears in any drain (only the bare "✓" rune does) and the subtests timed out. Per principle 5 both subtests were fixed minimally to pin `hasLines("\u2713", "all done")` (icon rune + final text, no adjacency). The reject subtest needed no such fix: its row re-emits near-whole because the error text replaces the title, so "✗ bash" and "permission rejected" both appear.
141. S0.10 plan test bug: the four chrome tokens span two drains (test-accuracy/low, 2026-08-26): the plan's terminal settled-state `WaitFor` asserted all four chrome SGR tokens (244/246/114/255) in one drain, but each `WaitFor` drains the shared stream and the cell-diff renderer emits a styled run only in the frame its cells change — the completed read row (244), the running bash row (255) and the `● live` conn dot (114) settle BEFORE the permission reply, while the rejected bash row (246) settles only AFTER it; no single drain can carry all four. The token set was split into `chromeTokensSettled` {244,114,255} (dialog drain — merged with the perm echo + row/dot regexes) and `chromeTokensRejected` {246} (reject drain — merged with the ✗-row + final text); each condition stays ONE merged condition per the teatest convention.
142. S0.10 plan test bug: the prompt cursor is pinned in a drain where it never re-emits (test-accuracy/low, 2026-08-26): the plan asserted `cursorRe` (the prompt cursor's merged CSI `fg(text)+reverse`) in the dialog drain on the premise that the cursor "re-emits on every keystroke", but the cell-diff renderer re-emits a line only when its cells change — the prompt line is re-emitted by the home→session route frame (the row moves) and is then unchanged across the typing/submit diff (the renderer coalesces frames: the last emitted frame already carries `> ` + cursor, so the typed text and its clearing never appear in any emitted diff — verified by dumping the full byte stream of the scenario), so the cursor CSI appears in no later drain. The cursor assertion was moved to the session-route drain, where it is deterministically present (merged with the help-line marker in ONE condition); the dialog drain keeps the row/dot regexes + settled tokens.
143. S0.10 plan re-baseline scope: the toast test pins the removed static red (test-accuracy/low, 2026-08-26): the plan's Task S0.10 Step-3 rewrites `toastsView` to `a.theme.Error().Render(l)` (removing the static `errRed`), but its file list never re-baselines `TestToastRendersAboveFooterInRed`, which pins the raw static SGR `\x1b[38;5;196m` on a zero-engine (`testApp()`) view — after the change the block renders plain (no SGR from a missing token, the S0.7 rule) and the assertion could never match. Per principle 5 the test was re-baselined minimally: the toast LINE is pinned PLAIN (the zero-engine `testApp()` view is the home frame, whose surviving statics — the bold selection row and the prompt statics — carry SGR unrelated to the toast; pinning the whole view plain was impossible) and the above-footer placement pin is unchanged; the test was renamed `TestToastRendersAboveFooter` (the "InRed" name described the removed static). The error token's SGR under the real engine is the same `th.Error()` the S0.10 golden pins as `38;5;246` (chromeTokensRejected).
144. S0.10 plan re-baseline scope: the two permission-dialog teatest tests pin whole-row adjacency (test-accuracy/low, 2026-08-26): the plan's re-baseline list names only the `TestTUIPermissionFlow` subtests, but `TestPermissionDialogKeyReply` and `TestPermissionDialogHTTPReply` also pin `hasLine("\u2713 bash")` on zero-engine (`newRecApp`) runs — the cell-diff renderer re-emits only the changed icon cell (▶→✓) for an unstyled row, so the adjacency "✓ bash" never appears in any drain and both WaitFors timed out (same root cause as deviation 140). Per principle 5 both were fixed minimally to `hasLines("\u2713", "done")` (icon rune + the scenario's final text, no adjacency).
145. Bead yolo-oae.1.12 pump-wait mechanism: select(2) instead of syscall.Poll (behavior/low, 2026-08-26): the bead's fix design (a) prescribes a `syscall.Poll`-based pump loop (10–50 ms timeout, done-flag check per wakeup), but Go 1.26.7 (the project toolchain) removed `syscall.Poll`/`syscall.PollFd` and the `POLLIN` constant from the stdlib (also `FdSet.Set`/`IsSet` — the bit is now set manually on `FdSet.Bits`). The remaining stdlib bounded-readability wait is `syscall.Select` (select(2)); `waitForReadable` (internal/tui/theme/palette.go) uses it with identical semantics — 20 ms bounded wait, EINTR restarts the wait, the done flag is checked after every timeout, and a read happens only after readiness (n=0/EAGAIN handled by looping). No behavior contract changes: the bead's test-pinned guarantee (the tty reader goroutine is joined and provably dead before `DetectStd`/`detectFd` return — close() alone cannot wake a read blocked in the kernel) is implemented and pinned by `TestDetectFdNoLingeringReader`.
146. lipgloss color-profile quantization of the SGR encoding (render/low, 2026-08-26): yolo renders through lipgloss v2 + the bubbletea v2 renderer, which quantize hex tokens per the TERMINAL color profile — the teatest goldens pin TERM=xterm-256color, so the SGR codes are ANSI256 `38;5;N` derived by `x/ansi` v0.11.8 `Convert256` (to6Cube levels 0x00/0x5f/0x87/0xaf/0xd7/0xff with v<48→0, v<115→1, else (v-35)/40; exact-cube early return; else the grey index (avg-3)/10 with avg>238→23, grey=8+10*idx, avg=(r+g+b)/3; DistanceHSLuv cube-vs-grey tie-break, the cube winning ties); opentui always emits 24-bit SGR. The TOKEN hex chain is exact by construction (S0.2 golden matrix, bit-identical port) — only the SGR ENCODING follows the terminal profile: a truecolor terminal gets 24-bit SGR directly. The S8 pty-capture diff must run under a truecolor TERM to compare against the upstream captures (whose SGR is always 24-bit).
147. S0 close-out plan numbering staleness (plan-accuracy/low, 2026-08-26): the S0 slice-gate section of the plan references DEVIATIONS entries "123", "124" and "125" for the wire change, the palette-probe-once scoping, and the lipgloss profile note; by close-out time entries 123–129 had already landed (S0.2–S0.5) with different content, so the close-out items landed as 130/131 (S0.7) and 146 (this close-out) per the continuous-numbering rule. The plan's cross-references are stale; the audit log numbers are authoritative.
148. S1.1 dep landing: `go mod tidy` prunes the not-yet-imported glamour requirement (plan-accuracy/low, 2026-08-26): the plan's S1.1 Step 2 runs `go get charm.land/glamour/v2@v2.0.1` then `go mod tidy`, expecting `grep glamour go.mod` to show a direct require — but `go mod tidy` removes any module no package in the main module (incl. tests) imports yet, and yolo's first glamour import lands in S1.2 (`internal/tui/theme/syntax.go`). Ran the sequence live: tidy left go.mod with only the go-line bump (1.25.0 → 1.25.8, as the plan predicted) and zero glamour entries, graph 53 → 53. Resolution: land with `go get` alone (glamour v2.0.1 + transitive closure in go.mod/go.sum, marked `// indirect` until the first import); the first `go mod tidy` runs at S1.2 when the import exists (it then flips to a direct require). Live module-count delta 53 → 69 = 16 NEW modules (the plan's ≈10 estimate was superseded by the live count, per the plan's own "live count is authoritative" clause). Pure-Go re-verified post-landing: `CGO_ENABLED=0 go build ./...` green; the cgo grep hits in the new tree are all `//go:build ignore` (x/net generated defs) or `//go:build icu` (x/text optional) — not in the build.
149. S1.2 plan implementation bug: the zero-Theme renderer emitted attribute SGR (plan-accuracy/low, 2026-08-26): the plan's `NewTranscriptRenderer` passed `WithStyles(th.StyleConfig(...))` unconditionally, but `StyleConfig` always sets the attribute pointers (Heading/Strong Bold, Emph Italic, Link Underline) even for a zero Theme whose token colors are all nil — glamour's renderText builds a non-empty x/ansi Style from the attributes alone, so the plan's own pinned `TestTranscriptRendererRenders` zero-Theme assertion ("degrades to plain output (no SGR)") could never pass (observed: `hello \x1b[1mworld\x1b[m`). Per principle 5 the test defines the contract: `NewTranscriptRenderer` now gates `WithStyles` on `!th.Zero()` (a zero Theme leaves `ansi.Options.Styles` all-nil → glamour renders plain — exactly the S0.7 nil-engine contract the plan's own Interfaces note attaches to `Theme.Zero()`). Themed renderers are unchanged: a named Theme always gets the full StyleConfig (attributes + colors).
150. [render/low] S1.3: the upstream TextPart passes bg=theme.background to the <markdown> element (index.tsx:1705) — a terminal background hint with no glamour equivalent (no SGR block background without per-line backgrounds; the yolo frame is transparent by S0 design). yolo omits the Document BackgroundColor; the S1 pty diff arbitrates (spec §4).
151. S1.3 plan test bug: the merged terminal condition pins the help line that never reaches its own drain (test-accuracy/low, 2026-08-26): the plan's `TestMarkdownTextPartSGR` merges `esc abort/back` (the session help line) into the THIRD drain's condition alongside the themed text lines, but the help line settles in the session-route frame and is consumed by the second `WaitFor` — the cell-diff renderer re-emits a line only when its cells change, so the unchanged help line appears in no later drain (same root cause as deviations 141/142, the S0.10 drain semantics the plan's own convention notes cite). Per principle 5 the fix is minimal: the third condition pins only the feature tokens (3-column indent on both rendered lines, markers stripped, `38;5;255` + `38;5;215`), and the help line stays pinned in the session-route drain — exactly the `TestSessionChromeThemeSGR` structure.
152. S1.4 plan newRenderer dropped the zero-Theme gate (plan-accuracy/low, 2026-08-27): the brief's newRenderer passed `glamour.WithStyles(cfg)` unconditionally and set `cfg.CodeBlock.Chroma = &ch` for every theme, regressing the S1.2 zero-Theme plain contract (deviation 149) that `TestTranscriptRendererRenders` pins — `StyleConfig` always sets the attribute pointers (Bold/Italic/Underline) even with nil colors, so a zero-Theme renderer would emit attribute-only SGR, and the near-empty chroma map would squat the global "charm" slot (glamour registers it under the fixed `chromaStyleTheme = "charm"` name whenever `CodeBlock.Chroma` is non-nil — `ansi/codeblock.go`). Per principle 5 the implementation keeps the `!th.Zero()` gate around WithStyles AND the chroma attach (nil chroma pointer → Render skips the `delete(styles.Registry, "charm")` slot delete), so a zero-Theme renderer renders plain, code blocks included (Chroma nil + empty CodeBlock.Theme → glamour's plain fallback render). `WithWordWrap` stays unconditional — it is not styling.
153. S1.4 plan test bug: TestSubtleChroma check-closure parameter type (test-accuracy/low, 2026-08-27): the brief's TestSubtleChroma declares its check closure as `func(name string, got, want string)`, but the body dereferences `got.Color` and every call site passes an `ansi.StylePrimitive` value (`sub.Comment`, …) — as written the test cannot compile. Per principle 5 the parameter was fixed minimally to `got ansi.StylePrimitive` (the sibling shape of the TestChromaMapping closure); all color/attribute expectations are unchanged.

154. S1.5 plan test bug: TestGFMRender contiguity pin on the raw ANSI output (test-accuracy/low, 2026-08-27): the brief's TestGFMRender asserted the contiguous substring "• done" in the raw ANSI output, but glamour v2.0.1 emits the TaskElement tick prefix and the item text as separate SGR runs with a reset between them (ansi/task.go + baseelement.go; observed: `\x1b[38;2;238;238;238m• \x1b[m\x1b[38;2;238;238;238mdone`), so the contiguity was structurally unsatisfiable; per principle 5 the check now runs on an SGR-stripped copy (regex `\x1b\[[0-9;]*m` → "") while every raw-SGR assertion (base color 38;2;238;238;238, SGR-9 form, table grid) stays on the raw output — the intended pin (bullet immediately precedes its item's text) is unchanged.
155. S1.6 plan reasoningSummary left the title's blank line in the body (plan-accuracy/low, 2026-08-27): the brief's helper returned `strings.TrimRight(rest, " \t\r\n")` where rest still carried the leading blank line (e.g. body `"\n\nThe body here."`), but the brief's own TestReasoningSummary table — and the parity note it cites (thinking.ts:12 `/^\*\*([^*\n]+)\*\*(?:\r?\n\r?\n|$)/`: the blank line is consumed BY the title block) — expect the body AFTER the blank line ("The body here."). Per principle 5 the test defines the contract: the implementation consumes the blank line ((\r?\n) twice, mixed endings allowed) before the body and TrimRights the remainder; the no-match and title-only rows are unchanged.
156. S1.6 plan openMark double space (plan-accuracy/low, 2026-08-27): the brief's openMark returns a trailing-space mark ("+ "/"- ") and its reasoning rows append a leading-space " Thought: …" string, building `+  Thought: <title>` (double space) — contradicting the brief's own binding parity note (`"+ Thought: <title> · <duration>"`, single space) and its own TestReasoningPartSGR regex `[+-] Thought: Planning · …` (single space). Per principle 5 openMark was fixed to return the bare mark ("-"/"+"); the single separating space lives in the row strings.
157. S1.6 plan blended() snippet does not compile (plan-accuracy/low, 2026-08-27): the brief's `lipgloss.NewStyle().Foreground(hex6(out))` passes a plain string, but lipgloss v2 `Foreground` takes `color.Color` (a string does not implement it; the package's own `fg` casts through `lipgloss.Color(hex)`). Fixed to `Foreground(lipgloss.Color(hex6(out)))` — same blend math, compile fix only.
158. S1.6 plan test re-baseline: TestReasoningPartSGR duration and body base (test-accuracy/low, 2026-08-27): the S1.6 brief's TestReasoningPartSGR (a) required `· <duration>` in the collapsed Thought row, but the engine stamps PartTime at ms resolution (internal/session/round.go) and the fake turn finalizes in <1ms (End−Start=0), so the brief's own title-only row form is the actual one — the regex now makes the duration optional; (b) pinned the expanded body base text as pre-blended textMuted (#515151 → 38;5;239), but upstream v1.18.18 renders the reasoning body with RAW theme.textMuted fg + subtle pre-blended CHROMA (routes/session/index.tsx ReasoningPart `<code fg={theme.textMuted} syntaxStyle={generateSubtleSyntax(theme)}>`), the S1.4 contract the tree implements — so the pin re-baselines to raw textMuted 38;5;244 and additionally pins the deterministic open-header warning-subtle 38;5;94; (c) the expanded-frame assertion ALSO required the duration via the text check "- Thought: Planning · " — the same root cause as (a), since session.go builds ONE row for collapsed/expanded (open only flips the mark) — so that check was relaxed to "- Thought: Planning" (title-only, duration optional). Clause (c) is a third minimal fix beyond the two the controller ruling enumerated, resolved per principle 5 from the ruling's own rationale ("the title-only row form is the actual row") and its Step-2 requirement that the test pass (empirically no drain carries `· `: the real drain shows `[38;5;94m- Thought: Planning[m` + `[38;5;244mvar x = 1[m`).
159. S1.7 brief re-baseline scope (plan-scope/low, 2026-08-27): the S1.7 brief's re-baseline scope (Step 1/Step 5: session_theme_test.go only) missed old-form pins in session_test.go (TestRenderMessages want blocks + TestRenderMessagesTitleFallbacks), tui_suite_test.go (4 sites), permission_test.go (2 sites), and app_test.go (1 site) that its own binding Step 4 full gate (go vet ./... && go test ./...) requires — the file list was extended per the controller ruling (3 → 10 files, incl. the one-word toolRowLine → toolRow renames in internal/tui/AGENTS.md + docs/superpowers/PROGRESS.md); TestRenderMessagesTitleFallbacks was re-pointed to completed/error states because the S1.7 running row renders only pending text (the fallback would lose all coverage); TestTUIPermissionFlow/reject gains an alt+e expansion step so the deviation-56 error-text pin ("permission rejected") stays observable on the new row form (the error text moves off the row into the S1.7 expansion); the stale icon-cell drain-comment rationales (deviations 140/144) were rewritten for the whole-row transition (running→completed now changes the WHOLE row, e.g. `~ Writing command...` → `$ echo hi`, so the full line lands in the drain); and the brief's re-anchored errorRowRe `m$ echo hi` was escaped to `m\$ echo hi` — a bare mid-pattern `$` is an anchor in RE2 (empirically verified: it never matches), so the brief's verbatim regex could not pass the brief's own Step 4.
160. S1.7 swap table rejected-bash row pin (test-accuracy/low, 2026-08-27): the S1.7 swap table pinned the rejected bash row as "$ echo hi", but the server reject path persists no title — failToolPart (internal/session/tool_exec.go:296) saves ToolState with empty Input and Error "permission rejected" and no Title — so the row carries the CallID fallback; the SGR re-anchor (m$ call_2 + drain "$ call_2") and TestTUIPermissionFlow/reject ("$ call_1") re-pin to the actual form.
161. S1.7 ruling reject pin un-waitable as one merged condition (test-accuracy/low, 2026-08-27): the ruling's TestTUIPermissionFlow/reject construction (alt+e immediately after '3', then ONE merged WaitFor hasLines("$ call_1", "permission rejected", "all done")) is mechanically unsatisfiable: while store.Pending is non-empty handleKey (internal/tui/keys.go:18) routes EVERY key to handlePermKey, and the reject reply clears Pending only async (permReplyMsg from the HTTP round-trip / the permission.replied SSE event) — an alt+e queued right after '3' is deterministically eaten by the perm handler (the event loop processes it before any reply can return), the expansion never happens, and the coalesced post-dialog frame carries `$ call_1` + `all done` WITHOUT the error text (empirically verified: the drain shows exactly those lines, expanded map empty). Because permission.replied (resolve, internal/permission/service.go:260) precedes the part update on the bus, the rejected row's render is the sync point where Pending is provably cleared; the subtest now waits hasLines("$ call_1", "all done") (the same coalesced frame the once/always subtests pin), sends alt+e, and then pins the expansion line with hasLine("permission rejected") — all three ruling tokens stay asserted, split across the two drains the cell-diff renderer actually emits (the expansion frame re-emits only the added line, never the unchanged `$ call_1`/`all done` rows).
162. S1.8 brief error-box snippet enabled the RIGHT border (plan-bug/low, 2026-08-27): the S1.8 brief's error-box snippet calls `Border(leftOnlyBorder(), false, true, false, false)`, but lipgloss v2.0.6's 4-arg `Style.Border` order is (top, right, bottom, left) (set.go:490, whichSidesBool:871) — so the call enabled the RIGHT side, where `leftOnlyBorder` leaves an empty char (it sets only Left), rendering an invisible right edge; this contradicts the brief's own left-only test (`TestMessageErrorBoxStyle` pins left-only), the upstream `border=["left"]` (index.tsx:1534-1548), and the helper comment. The brief's consistent toast call `(false, true, false, true)` = left+right proves the box line a transposition — fixed to `Border(leftOnlyBorder(), false, false, false, true)`. Also logged: the brief's Step 5 `git add` omitted `session_theme_test.go` (named in its own Files section) and a twin `"! something broke"` want at `session_test.go:112` (zero-theme render → re-baselined to the bare message per the brief's own zero-theme contract; the stale case name renamed to `message error renders bare line after parts`); the brief's test import block was trimmed 9→4 (9 unused imports — Go rejects them; the brief notes "No teatest this task"); the wrapLine string-spread drop and the four `lipgloss.Color(rgbaHex(c))` casts are compile-level adaptations with no value change (the cast pattern predates this task at home.go:304 / logo.go:71).
163. S1.9 brief hundredKBPart built 210,681 B, not the spec's 100 KB, and the spec §4 50 ms gate is unmeetable on glamour (plan-bug/medium, 2026-08-27): the S1.9 brief's `hundredKBPart` fixture produced 210,681 bytes, not the spec's 100 KB (the 1800-line code loop alone is 126 KB; the `len < 100*1024` pad branch never fired and its divisor 44 did not match the 38-char pad string) — the fixture is fixed to true spec shape (104,981 B: ~20 KB fenced code + ~85 KB prose, divisor 38); and the spec §4 50 ms budget is unmeetable on glamour v2.0.1 — the cost is intrinsic markdown→ANSI rendering (~0.9–1.0 ms/KB; isolation probes: plain zero-theme path 1.78 ms on the same state, renderer construct 61 µs one-per-renderMessages) so a true 100 KB spec-shape part measures min-of-5 98.7–101.6 ms (2026-08-27, Ryzen 7 5800X3D, opencode dark) — the spec-time ~22 ms derivation predates the glamour dep and does not reproduce — the gate value re-baselines 50 ms → 150 ms (1.5× headroom; the gate still fails on a regression ≥1.5× the floor) per the brief's own knob ("the gate value (not the code) is the knob"); the full measurement table is in .superpowers/sdd/s1-transcript/task-S1.9-report.md.
164. S1 slice gate: the transcript-fixture pty diff defers to the S8 diff sweep (plan-bug/low, 2026-08-27): the S1 slice gate item 3 (s1-transcript.md) and spec §4 "Fidelity risk control" / §9 risk 1 state that "the transcript-fixture pty diff runs before the slice closes", but the spec's pty-capture infrastructure — the mock OpenAI-compatible SSE server, the pty-driven opencode TUI, and the capture/diff tooling — is explicitly allocated to S8 (spec §8 bead grain: "S8 ≈ 5 (mock-SSE server, pty-capture script, diff sweep, deviation-log + re-baseline, close-out)"), which executes after S1; the S1 gate mirrors the S0 gate, which resolved the same parenthetical by deferring the truecolor pty comparison to S8 in deviation 125. Per principle 5 the last-stated call (the §8 S8 tooling allocation) wins: the interim S1 fidelity anchors are the teatest SGR goldens — the S1.3 transcript fixture (fenced code, lists, tables, quotes, links, CJK width) plus the S1.4–S1.8 per-surface restyle goldens — and the glamour-vs-opentui structural gap check runs in S8's diff sweep, where visible gaps become per-element StyleConfig overrides or a logged custom renderer (logged either way, as the gate prescribes).
165. S2 detail pass: task-bead id shift (breadcrumb/info, 2026-08-27): the frozen S2 table (plan.md) names the task beads `yolo-oae.3.1`–`3.10`, but the detail pass consumed bead numbers `yolo-oae.3.1` (the detail bead, claimed) and `yolo-oae.3.2`/`3.3` (duplicate detail beads, closed) before the task beads could be created — the S1 precedent puts the detail bead LAST (yolo-oae.2.10), which was impossible for S2 because the detail pass precedes slice start. The 10 task beads therefore land in table order at `yolo-oae.3.4`–`yolo-oae.3.13` (S2.1→.4, S2.2→.5, S2.3→.6, S2.4→.7, S2.5→.8, S2.6→.9, S2.7→.10, S2.8→.11, S2.9→.12, S2.10→.13); the frozen titles and pinned commit messages are unchanged. No code or wire impact.
166. S2.2 modal backdrop: plain terminal-background lines (render/low, 2026-08-27): the upstream `dialog.tsx` overlay backdrop is `RGBA.fromInts(0, 0, 0, 150)` (rgba(0,0,0,0.15)) dimming the live frame beneath the panel, but a plain terminal has no SGR-alpha dim — yolo's `viewModal` renders the backdrop as plain (blank) terminal-background lines. The upstream `onMouseUp` dismiss has no yolo analog (no mouse support) — a capability note, not a deviation.
167. S2.2 plan test bug: clearModals expectation drops the pending replacement's onClose (test-accuracy/low, 2026-08-27): the brief's TestModalStackOps expects `closed == "second,first,old,c2"` after pushing "c2" and `clearModals`, but the stack then holds [new, c2] — the "new" modal pushed by the immediately preceding `replaceModal` is still a modal item — so the brief's own `clearModals` (and the upstream `clear()` it ports, dialog.tsx:140-148, which fires every item's onClose) also fires "new"'s callback. The expected string was fixed to "second,first,old,c2,new"; the top-down order, the depth-0 pin, and every other assertion are unchanged.
168. S2.2 plan test bug: frame prefix checks run on raw lines that carry the static bold title SGR (test-accuracy/low, 2026-08-27): the brief's TestModalFrameLayout / TestModalFrameSessionClamp assert `strings.HasPrefix(lines[N], 10sp+"Model")` on the RAW view line, but `modelDlg.view` renders its title through the surviving STATIC `title` style (style.go:24, `Bold(true)` — the S0.8+ static the teatest goldens pin), which emits `\x1b[1mModel\x1b[m`, so the raw line can structurally never have the plain prefix (the tests' own Fatalf messages already print `stripANSI`). Per principle 5 both prefix checks were fixed to run on `stripANSI(lines[N])`; the pinned content and placement (10-column lead, panel-top row) are unchanged.
169. S2.2 clearModals fires onClose callbacks top-down (newest-first) where upstream dialog.tsx clear() walks bottom-up (render/low, 2026-08-27): yolo's `App.clearModals` pops the modal stack top-down (matching `closeTopModal`'s single-close semantics), firing each `onClose` in pop order; upstream's `clear()` iterates the stack bottom-up. The S2.2 test pins the top-down order (`"c2,new"` in `TestModalStackOps`); no current or planned callback relies on bottom-up ordering (each modal's result callback is self-contained), logged per principle 2 so the S8 parity audit has it on record.
170. S2.3 huh form look and keymap differ from the upstream opentui pills (render/low, 2026-08-27): the upstream dialog-alert/dialog-confirm are hand-rolled opentui components (dialog-confirm.tsx — return/left/right bindings, custom pill render), but yolo ports them as huh v2.0.3 one-group forms (the S2.1 approved dep) — `buildAlertForm` = one Confirm field `Affirmative("ok")`/`Negative("")` (lone button, huh disables the reject keys), `buildConfirmForm` = `Affirmative("Confirm")`/`Negative("Cancel")` starting on confirm — so the field look is huh's themed field (deviation: borderless as close as huh allows via `themeDialog`, tokens from the resolved Theme: text/title, textMuted/description+blurred-button, primary/selected-foreground/focused-button) and the keymap is huh's (left/right toggle, y/n set, enter submits) instead of the upstream three bindings (return/left/right). Behavior is equivalent: enter confirms the active pill, left/right toggle, esc/ctrl+c cancel via the S2.2 modal stack (firing onClose). Logged per principle 2.
171. S2.3 plan test bugs: huh v2.0.3 forms render blank unsized and submit is cmd-cascade-driven (test-accuracy/low, 2026-08-27): the S2.3 brief's TestAlertFormSingleButton calls `f.View()` on a never-sized form — huh v2's confirm field computes `maxWidth := width - frameSize` and `lipgloss.Wrap` drops the title/description/buttons at limit ≤ 0, so the view is blank padded lines (probe: same form after a WindowSizeMsg renders normally); the test now sizes the form through `openFormModal` (a.size 80x24) and reads the pushed form's View. The brief's TestConfirmFormSubmit/EscCancels/KeysToggle call `a.handleKey(enter)` and expect immediate completion — but huh v2's Confirm.Update returns the NextField CMD (unexported `nextFieldMsg`), the group's nextField emits the `nextGroup` cmd, and only the form's `nextGroupMsg` handler reaches `StateCompleted`; the App dropped huh's unexported internal msgs, so onConfirm never fired. Fix: `App.updateMsg`'s default case forwards unhandled msgs to the open form modal (`dialogStack.form()` + `huhFormDlg.forwardMsg`, which re-feeds form-progress msgs and ignores `tea.Cmd` values); the tests replay the cascade through `App.Update` with the `driveCmds` helper (BatchMsg fan-out + returned-cmd capture, mirroring the production bubbletea event loop). The pinned outcomes (submit fires onConfirm + closes, esc fires onClose only, left toggles the pill to cancel) are unchanged.
172. S2.4 plan test bugs — unfocused input drops typed runes, the placeholder renders only when sized + focused, and the focused input line is SGR-interleaved (test-accuracy/low, 2026-08-27): the S2.4 brief's TestInputFormTypedSubmit typed before driving the form's Init (focus) cmds — an unfocused bubbles v2 textinput drops typed runes (`Update` returns immediately when `!m.focus`), so `got` would have been the initial value; the test now sizes the form (a.size 80x24) and drives the queued Init cmds through `driveCmds` before typing. The brief's TestInputFormPlaceholder asserted `View()` on a never-sized, never-focused form — huh v2.0.3 renders the title/description only once sized and the placeholder only once the textinput is focused (same root cause as deviation 171's first clause). Additionally, the focused input's line renders the placeholder SGR-interleaved (the cursor char reversed, the placeholder dim — observed `\x1b[7ms\x1b[m\x1b[90mession name\x1b[m` for placeholder "session name"), so the raw `strings.Contains` could never match; the check now runs on the `stripANSI` copy (the deviation 168 precedent). The pinned outcomes (oldhi typed submit + close, esc cancels only, placeholder visible) are unchanged; no production-code deviation. Probe note: the focused textinput's cursor schedules 530 ms blink `tea.Tick` cmds that `driveCmds` blocks on (TestInputFormTypedSubmit runs ~9.5 s) — library-intrinsic, not a test bug.

173. S2.5 plan test bug: TestSelectFuzzyFilter pins a single hit for needle "g" (test-accuracy/low, 2026-08-27): the brief's test asserts `len(filtered()) == 1` for needle "g", but `filtered()` searches titles + categories (upstream keys: ["title", "category"]) and sahilm/fuzzy v0.1.3 matches case-insensitively — with selTestOptions()'s categories "Group A"/"Group B", needle "g" matches all three (Gamma via title ×2 + category, Alpha/Beta via category) → filtered = [Gamma, Alpha, Beta], len 3, unreachable (upstream parity itself returns all three). Per principle 5 (test defines the contract) the assertion fixes to `len != 3 || filtered()[0].title != "Gamma"`; the enter-picks-Gamma assertion already holds (filtered()[0] = Gamma).
174. S2.5 plan test bugs + one implementation gap: the brief's select tests could not compile or pass as written (test-accuracy/low, 2026-08-27): (a) pushSelectModal used `select: m` — `select` is a reserved Go keyword and the brief's own Step-3 struct names the field `sel`, so the file could never parse — fixed to `sel: m`; (b) its param `a *App` cannot take `testApp()`'s `*recApp` — fixed to `a *recApp`; (c) TestSelectFuzzyFilter types 'g' into the filter but the brief's selectNew never focuses the bubbles textinput, which drops runes unfocused (the deviation-172 class) — `filtered()` returned the original order; per upstream auto-focus (dialog-select.tsx:590) and the brief's own syncFilter contract, selectNew now calls `_ = m.input.Focus()` (implementation addition; the discarded blink cmd leaves a static cursor until the S2.7 cmd plumbing); (d) TestSelectViewLayout's filter row check ran raw strings.Contains on the SGR-interleaved placeholder (the deviation-168/172 class) — now checks `stripANSI(out[1])`; (e) its empty-state check set `m.filter = "zzz"` directly but view()'s syncFilter derives filter from the empty input and clobbers it — fixed to `m.input.SetValue("zzz")`. Pinned outcomes unchanged.
175. S2.6 plan test bugs + header-bold contradiction (test-accuracy/low, 2026-08-28): the S2.6 brief's three new tests could not pass as pinned against the brief's own pinned implementation: (a) TestSelectScrollWindowCountsRows walks `m.move(1)` 20 times over 20 options — the 20th move wraps the selection back to 0 (move's wraparound), the row-window anchor resets to row 0 and `m.top` deterministically ends at 0, so the brief's own final assertion `m.top < 20` was unreachable; the test's own comment pins "sel 19 → row 38-39 → top = 38-14+1 = 25", so the loop bound fixes to 19 moves (top = 25, assertion green); (b) TestSelectCategoriesRender sets `m.filter = "a"` directly, but the pinned view runs syncFilter first and the filter INPUT is the source of truth (the S2.5 contract — deviation 174(e) fixed the same pattern to `m.input.SetValue`), so the direct set was clobbered to "" and the headers rendered; fixed to `m.input.SetValue("a")` (the full input→syncFilter→flat pipeline runs, the pin unchanged); (c) the S2.5 TestSelectViewLayout uses the shared selTestOptions() fixture, which carries categories — the new S2.6 header rows shift the row lines the S2.5 pin indexes (out[2..4]), while the brief's Step 4 expects "the no-category options render identically: no headers, no details, no footer" (no re-baseline of the S2.5 pin named); the view test now builds a local no-category fixture with the same titles/descriptions, so the S2.5 pins stay byte-identical; the other S2.5 tests (navigation/enter/filter/weighting) never render the view and are untouched. Additionally, the brief's parity note and the pinned buildLines doc comment say the category header is BOLD (upstream dialog-select.tsx: `attributes={TextAttributes.BOLD}`), but the pinned code renders `th.Accent().Render("   " + o.category)` without bold, and the pinned test requires the EXACT plain header line (`l == "   Group A"` on a zero Theme, where a bold attribute alone emits SGR — the deviation-149 class); per principle 5 the last-stated call (pinned code + pinned test) wins: the header renders without bold, and the pinned doc comment is kept verbatim per the task's comment rule. Pinned outcomes (headers + blank separator + flat-on-filter, details truncateMiddle'd to the row width, footer tail at the right edge, row-count scroll window) unchanged.
176. S2.7: the select's scroll acceleration is pinned to ±10 list rows (behavior/info, 2026-08-28): upstream dialog-select.tsx page-scrolls through `getScrollAcceleration` (env-machined acceleration, `CustomSpeedScroll(3)`) — the env machinery is not ported; yolo pins pgup/pgdn = exactly ±10 rendered rows (the WINDOW moves, the selection stays; the delta is consumed by view() on the next render, clamped to the list bounds). The plan's pre-assigned number "deviation 170" was stale (the audit log tail was 175; the plan numbers are stale per the 132/147 precedent — audit-log numbering is authoritative), so the entry lands as 176.
177. S2.7 plan defect: the brief's view() pseudocode introduces `selectModel.lastSel` without specifying its initial value (plan-accuracy/low, 2026-08-28): the pseudocode re-anchors the window "only when the selection row changed; else consume pageDelta", but a literal `-1` init makes the brief's own pinned TestSelectScrollAcceleration unreachable — first view call: selRow 0 != lastSel -1 → the re-anchor branch runs, `top` stays 0, while the test wants top 10 after one pgdn + view (and every S2.5/S2.6 test would re-anchor on its first view). Per the controller ruling the initial window state is initialized as already anchored at selection row 0 — `lastSel` takes the Go zero value 0 with a field comment stating the initial anchor — so the pinned test passes unchanged and the S2.5/S2.6 tests keep their exact first-view behavior (no anchor, top stays 0).
178. S2.7 plan test bug: the pinned test msgs reference constants that do not exist in bubbletea v2.0.9 (test-accuracy/low, 2026-08-28): the brief's `selShiftTabMsg = tea.KeyPressMsg{Code: tea.KeyBackTab}` and `selPgDnMsg = tea.KeyPressMsg{Code: tea.KeyPgDn}` name `tea.KeyBackTab` and `tea.KeyPgDn`, which the pinned bubbletea v2.0.9 (key.go) does not define — shift+tab is decoded as `{Code: KeyTab, Mod: ModShift}` (uv decoder, which is also how the msg's `String()` yields "shift+tab"), and the page-down constant is `KeyPgDown`. Per principle 5 the msgs are fixed minimally to the correct representations (`KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}` / `KeyPressMsg{Code: tea.KeyPgDown}`); the msg contract the brief names (tab, shift+tab, pgup, pgdown) is preserved exactly. Derived consequence on the implementation side: the pgdn binding's key string is "pgdown", not the brief's snippet "pgdn" — bubbles v2.2.1 `key.Matches` compares `msg.String()` verbatim against the binding keys, and `KeyPressMsg{Code: KeyPgDown}.String()` is "pgdown" (uv keyTypeString). Pinned outcomes (action keys run, tab/shift+tab focus cycle with wrap, enter on focus, footer hints, pgup/pgdn ±10) unchanged.
179. S2.8 info() data source: the port reads the ask body from the tool-part input, not request Meta (behavior/low, 2026-08-28): the S2.8 brief ports the upstream info() (permission.tsx:195-380) to render the ask's icon/title/body — bash → `$ <command>` body, edit → `filePath` title, read/list → `filePath`/`path` body, glob/grep → quoted `pattern` body, webfetch/websearch → `url`/`query` body, task → `subagent_type`/`description` body — but the wire's PermissionAskedProps carries no request Meta (no upstream Meta/MetaDiff), so the data source is the hydrated tool part's State.Input map (partInput matches p.Tool.CallID + optional MessageID across store.Messages) and the upstream EditBody diff view is dropped (there is no request Meta to diff against). The input keys follow the tool schemas (camelCase: filePath, pattern, command, url, query, path, subagent_type, description). The brief's pre-assigned number "169" is stale (the audit-log tail was 178; per the 132/147/176 precedent the audit-log numbering is authoritative), so the entry lands as 179.
180. S2.8 plan test bug: the edit-ask subtest fixture could not resolve the part input (test-accuracy/low, 2026-08-28): the S2.8 brief's TestPermissionRender "edit ask formats the path title" subtest pinned an edit ask whose Tool ref did not match the part — partInput keys off p.Tool.CallID (+ optional MessageID) and returns nil on a mismatch, so permInfo's edit branch read an empty filePath and the pinned title `→ Edit /tmp/x.go` was unreachable; the fixture also carried the bash ask's Patterns, which would have violated the subtest's own "empty lines omitted" assertion. Per the controller ruling the fixture is fixed minimally to Patterns = nil, Always = nil, Tool = &protocol.PermissionToolRef{CallID: "c1"} with a part of CallID "c1" carrying Input {"filePath": "/tmp/x.go"} — the edit title renders `→ Edit /tmp/x.go` and the empty patterns/Always lines are omitted. Pinned outcomes unchanged.
181. S2.8 plan test bugs: the SGR golden's harness, drain, and pin shape could not pass as written (test-accuracy/low, 2026-08-28): the S2.8 brief's TestPermissionDialogSGR had three structural defects. (a) It drove the shared permFlowHarness, which runs a nil theme engine (the zero Theme) — the zero theme paints no SGR at all, so no `38;5;215`/`48;5;215` token can appear; the test now builds a real theme.Engine (theme.New + Resolve, active "opencode" dark) and passes it to NewApp (the TestSessionChromeThemeSGR pattern). (b) It waited for the echo tokens and then did a SECOND WaitFor for the SGR — but each WaitFor drains the shared stream and the cell-diff renderer emits the panel once and never re-emits the unchanged lines, so a second drain would contain only the footer spinner's bytes (deviation 141's merged-condition convention); the test now merges the SGR tokens + the plain tokens into one WaitFor. (c) It pinned a bare "38;5;215m" substring — the cell-diff renderer merges the changed SGR params into ONE CSI (the inner param order is not pinned) and fragments the plain text with erase/CUP sequences, so a standalone bare-`38;5;215m` is structurally unmatchable (the deviation 141/142 anchored-regex convention); the test now pins position-anchored regexes permWarnHeaderRe (a CSI carrying `38;5;215` that opens the `△ ` header run) and permPillBgRe (a CSI carrying `48;5;215` that opens the `Allow` pill run). Additionally, under the real theme the cell-diff splits the panel lines at pen changes (the header lands as "△ Permission" + a separate "required" run), so the plain tokens are pinned per RUN, not the contiguous hasPermDialogEcho form — the contiguous form is only matchable in the zero-theme flow drains (which is why the flow tests' hasPermDialogEcho was re-baselined to per-run tokens in the same commit). Pinned outcomes (warning header fg 215, selected pill bg 215) unchanged.
182. S2.8: the selected permission pill is pinned to the warning token, not the upstream accent (render/low, 2026-08-28): the S2.8 brief states the selected pill's look "rides on deviation 167" — but 167 is the S2.3 huh-form look (a different surface) and the upstream permission pills paint the selected one in the accent token; the brief's "rides on 167" is stale. yolo deliberately pins the selected pill to the warning token (bg #f5a742 → ANSI256 215, fg = SelectedForeground-on-warning, bold), the unselected pills muted — logged per principle 2. The SGR golden pins the bg 215 (deviation 181).
