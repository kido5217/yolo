# golang-testing — server+session tests
Date: 2026-08-21 · chunk: server+session tests · source files: 9

## Findings

- [testing-1] P2 internal/session/engine_perm_test.go:299 — `time.Sleep(50ms)` is the only wait for the permission ask to park before `Abort`; the skill's core flakiness rule (no sleep-derived ordering) applies, and the harness already has a deterministic seam (`h.waitForEvent` on `permission.asked`, engine_test.go:297, used correctly elsewhere in this file). If parking takes >50ms on a loaded CI, Abort lands *before* the park: the test still passes (pre-park and mid-park abort both yield `error: aborted`), but it silently stops testing the path its name claims ("AbortDuringAsk"), weakening the regression pin for the parked-ask abort branch. — fix: replace the sleep with `h.waitForEvent(t, func(e) bool { return e.Type == protocol.EventTypePermissionAsked })` before `h.eng.Abort(ses)`. — contract-risk: none
- [testing-2] P2 internal/server/server_test.go:235 — `TestSendMessage409AndEvents` connects SSE and immediately sends; it asserts presence of all four event types in the collected frames but never confirms the subscription is live first. A publish that lands in the subscribe/handshake window (exactly the gap `WaitSubscribe` documents, testutil.go:115-128) drops the early frames and fails the test spuriously. `TestSSEOrdering` handles this correctly (contract_test.go:306, `s.WaitSubscribe(t, 1)` before publish). — fix: insert `s.WaitSubscribe(t, 1)` after `SSEConnect`, before the first `POST /message`. — contract-risk: none
- [testing-3] P2 internal/server/server_test.go:87 (routes: POST /session :87, POST /message :165; handlers_misc_test.go:88 PATCH /config) — the in-scope server tests never send a malformed-JSON body (truncated/invalid JSON) to any POST/PATCH route; the 400 + error-envelope behavior for that class is unasserted by any test in this chunk (bad dir → 400 and bad command values are covered; bad JSON is the missing basic error path the skill calls out). — fix: add malformed-body table cases (e.g. `{"text":` for create/message, `{"model":` for PATCH /config) asserting 400 + non-empty error envelope; if the port returns anything else that exposes a production bug — escalate to P1 before asserting. — contract-risk: none
- [testing-4] P2 internal/session/engine_test.go:60 — no goroutine-leak detection anywhere in the session or server test packages despite heavy goroutine lifecycle (turn goroutines, SSE readers, bus subscribers, bash shells, the harness's own event collector/replyWatcher). The harness cleans up carefully, but the skill asks for leak verification in packages with goroutines; a future turn-loop or tool-shell leak would be invisible to the suite (no goleak; no TestMain census). — fix: dependency-free first step — a `TestMain` that snapshots `runtime.NumGoroutine()` before/after (baseline captured at TestMain start) and fails on growth; `goleak.VerifyTestMain` is the stronger option but is a new dev-only dep and needs an explicit user call under the pinned-deps rule. — contract-risk: none
- [testing-5] P3 internal/server/server_test.go:263 · internal/session/lifecycle_test.go:294 — the "second send during an active turn → 409/ErrSessionBusy" assertions rest on `time.Sleep(50ms)` after the 202 response; a status-driven wait (poll for `busy` exactly as `waitIdle` polls for `idle`, contract_test.go:171 / engine_test.go:332) is deterministic and removes the only timing assumption. — fix: replace the sleeps with a short busy-poll before the busy-send; keep the 200ms/500ms fake delays as the busy window. — contract-risk: none
- [testing-6] P3 internal/session/engine_test.go:681 — `TestShutdownAbortsActiveAndWaits` proves "abort, not wait" by wall clock: fail if `elapsed > 400ms` while the hold is 500ms — a 100ms margin that a slow/loaded CI can breach, turning a correct implementation into a flake. — fix: have `slowDriver` record per-session ctx cancellation (e.g. set a flag in the `ctx.Done()` branch) and assert both sessions observed cancellation + both idle after Shutdown; drop or relax the wall-clock bound. — contract-risk: none
- [testing-7] P3 internal/server/contract_test.go:171 · internal/server/server_test.go:170-183, 416 — harness logic duplicated: the HTTP `waitIdle` poll exists twice (named helper in contract_test.go, inlined in `TestMessagesEndpoint`), and `TestSendLogsFailedTurn` hand-rolls the entire stack (storage+bus+provider+permission+engine+server+httptest, server_test.go:416-462) because `testutil.boot` exposes no `Log` seam. — fix: move `waitIdle` onto `*TestServer` (testutil), and add an optional log dir to `Boot` (e.g. `BootWithLog` or a `TestServer.LogDir` field) so the log test reuses the harness. — contract-risk: none
- [testing-8] P3 internal/session/prompt_test.go:87 — `TestFamilySelection` runs 11 table cases in a flat loop without named `t.Run` subtests (skill rule 1: "table-driven tests MUST use named subtests"); failures only localize via the `t.Fatalf` message text, and per-case pass/fail is not reported. — fix: wrap the loop body in `t.Run(c.api+"/"+c.prov, ...)`. — contract-risk: none
- [testing-9] P3 internal/server/contract_test.go:435 — `TestFakeEnvE2E` is fully offline (env-gated fake driver, no network) but the `E2E` suffix collides with the on-demand, user-run, network-touching `scripts/e2e-live.sh`; a future reader triaging "e2e" failures (or adding build tags) will misclassify it. — fix: rename to `TestFakeEnvConversation` (or similar) to keep the name honest about what it exercises. — contract-risk: none
- [testing-10] P3 internal/server/handlers_misc_test.go:169 — `TestAuthPutDelete`'s post-delete check loops `for _, p := range ps { if p.ID == "opencode" && ... }`: if `opencode` is missing from the list the check passes vacuously (masking a catalog regression). — fix: first assert `opencode` is present in the list (as `TestProviderListAndAuth` does via `byID`), then assert status. — contract-risk: none
- [testing-11] P3 internal/server/handlers_misc_test.go:213 — the LOCKED "validate body first → 400" case only asserts `resp.StatusCode != 404`; a 5xx or 404-shape regression on the bad-response path would slip through. — fix: assert `== 400` per the LOCKED comment one line above. — contract-risk: none
- [testing-12] P3 internal/session/engine_test.go:622 — reads the read-tool part via an IIFE `h.db.ListParts(func() string { ... t.Fatalf ...; return msgs[1].ID }())` — a closure that fails the test while also producing the argument; obscures the simple "get msgs[1].ID, then list its parts" flow. — fix: hoist to a plain variable (fetch messages, `t.Fatal` on error, then `h.db.ListParts(msgs[1].ID)`). — contract-risk: none
- [testing-13] P3 internal/server/server_test.go:33 — no test in either package calls `t.Parallel()` (skill rule 4), although each test boots a fully isolated stack (own temp dirs, DB, bus, server); the suites run in ~5s serially and the golden/contract subtests (14 independent boots) are the natural candidates. — fix: add `t.Parallel()` to the independent full-stack tests and `TestGoldenResponses` subtests; keep `TestFakeEnvE2E` serial (`t.Setenv` in `newSrvFakeEnv` is incompatible with parallel). — contract-risk: none

## Deferred / Noted (no fix in 0.1.2)

- `goleak` (testing-4's stronger fix) requires a new dev-only dep beyond the pinned set — needs an explicit user call; the dependency-free TestMain census is the 0.1.2-safe form.
- references/integration-testing.md assessed N/A: no Docker/SQL integration fixtures in scope (tests are in-process; live path is the user-run `scripts/e2e-live.sh`, env-gated per CONTEXT).
- teatest v2 harness quirks (drained WaitFor, NoTTY, Tick signature) do not touch this chunk — no TUI files in scope.
- `TestSSEOrdering`'s frame-sequence assertions (contract_test.go:330-394) pin the port's faithful SSE ordering (CONTEXT pin, deviation 41) — verified present, not a finding.

## Stats
P0:0 P1:0 P2:4 P3:9
COVERAGE: full — skipped: none

## Status (wave 10 fix pass, 2026-08-21)
- testing-1 → FIXED 490da6b
- testing-2 → FIXED 490da6b
- testing-3 → FIXED 12909d0
- testing-4 → FIXED d8e1a2e
- testing-5 → FIXED 490da6b
- testing-6 → FIXED 649d729
- testing-7 → FIXED 33cd3bc
- testing-8 → FIXED 5a69e8b
- testing-9 → FIXED 5a69e8b
- testing-10 → FIXED 12909d0
- testing-11 → FIXED 12909d0
- testing-12 → FIXED 5a69e8b
- testing-13 → FIXED c685c35
