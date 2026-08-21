# golang-testing — tool+llm+provider+storage tests
Date: 2026-08-21 · chunk: tool+llm+provider+storage tests · source files: 13
Scope note: the orchestrator's list named 12 files with stale line counts; `internal/tool/read_test.go`
exists in the package and was reviewed too. `internal/llm/fake/` has no test file at all (see testing-3).

## Findings
- [testing-1] P2 internal/tool/bash_test.go:49 — three of the five shell-owning tests (TestBashNonZeroExitIsSuccessWithMeta :49-63, TestBashStderrMerged :65-73, TestBashTimeoutKillsAndReports :75-93) build `NewShell(...)` and never Close it, while the other two shell tests in the same file (:21-25, :98-102) both `t.Cleanup(Shell.Close())` — leaked shells (OS processes + any reader goroutines) accumulate for the whole run — fix: add the same `t.Cleanup(func() { env.Shell.Close() })` to the three — contract-risk: none
- [testing-2] P2 internal/tool/tool_test.go:75 — TestReadDirListing claims to verify localeCompare order (comment :90) but line 91's compound `!Contains("A.txt\nb.txt\nsub/\n") && !Contains("A.txt")` is effectively vacuous (second disjunct), and the fragment loop :94-98 asserts membership only — a listing-order or dir-suffix-format regression would still pass — fix: assert the exact ordered sequence of the three entries in out.Text (positions, not just presence) — contract-risk: none
- [testing-3] P2 internal/llm/fake/fake.go:166 — the fake package has zero test files (`go test` prints "no test files") for a load-bearing double: `FromScript` (production wiring in server/deps.go for `YOLO_LLM=fake`) is untested (file-missing / malformed-JSON error paths, turn parsing, delay_ms), as are title-marker routing (titleMarker matched against prompt/title.txt is implicit — it matches today, verified), AutoText synthesis, Err turns, Delay+ctx-cancel, and concurrent Stream+Requests() — fix: add internal/llm/fake/fake_test.go covering those seams incl. a -race concurrent-stream case (ReqLog is only race-safe via the Requests() copy) — contract-risk: none
- [testing-4] P2 internal/tool/write_test.go:39 — TestWriteMissingDirCreated asserts only `err == nil` and never reads the file back — a regression that creates the parent dirs but writes nothing (or empty content) would pass, contradicting the test's stated purpose — fix: `os.ReadFile` the target and compare against "x" — contract-risk: none
- [testing-5] P2 internal/tool/edit_test.go:31 — TestEditErrorsPinned chains 7 pinned-error scenarios plus a replaceAll success in one linear function — an early `t.Fatal` aborts the remaining cases and failures are not attributable to a named case (skill: every test case needs a named subtest); same pattern in TestGlobTool :11, TestGrepTool :57, TestKidoSkipsInvalidEntries (provider_test.go:56), TestTodoWritePersistsAndTitles (todowrite_test.go:14) — fix: table-drive the error cases (name/args/want, file recreated per case) with t.Run; same for the tests listed — contract-risk: none
- [testing-6] P3 internal/storage/storage_test.go:116 — TestCascadeDelete creates part prt_1 but after DeleteSession only ListMessages is asserted; the part-level cascade is unverified — fix: also assert `db.GetPart("prt_1")` returns storage.ErrNotFound after the delete — contract-risk: none
- [testing-7] P3 internal/storage/storage_test.go:82 — TestSessionCRUDAndListOrder verifies only got[0]; the relative order of ses_aaa/ses_bbb — the bulk of "newest-first" — is unasserted — fix: assert the full ID order [ses_ccc, ses_bbb, ses_aaa] — contract-risk: none
- [testing-8] P3 internal/storage/storage_test.go:203 — `raw, _ := json.Marshal(prow.StateJSON); _ = raw` is dead code (marshal + discard, no assertion) — fix: delete; if the intent was to guard StateJSON validity, assert the unmarshal succeeds — contract-risk: none
- [testing-9] P3 internal/tool/bash_test.go:130 — TestBashPermissionPatterns indexes res[0]/always[0] and res2[0] without length checks — a regression where Patterns returns empty slices panics (index out of range) instead of a readable failure — fix: check len first and t.Fatalf with the slice in the message — contract-risk: none
- [testing-10] P3 internal/llm/llm_test.go:160 — TestOpenAIRequestShape asserts only stream=true and tools[0].function.name; it does not verify max_tokens (100 is sent) or the messages array (system+user roles/content) — its anthropic counterpart (anthropic_test.go:124-143) asserts all three, so the openai request shape is under-constrained — fix: assert got["max_tokens"]==100.0 and the messages roles/content — contract-risk: none
- [testing-11] P3 internal/llm/common_test.go:64 — collect() (and the midstream-error drain loops llm_test.go:131, anthropic_test.go:91) passes `ctx0(t)` a fresh 10 s context to every Next call, so a stream that yields parts forever without Finish never times out — the test hangs until go test's 10 m cap instead of failing fast with a location (skill helpers.md timeout pattern) — fix: bound the whole drain by one parent timeout (or the panic-with-caller-location helper) — contract-risk: none
- [testing-12] P3 internal/tool/read_test.go:1 — test-file organization deviates from one-test-file-per-source-file: read-tool tests are split across tool_test.go (TestRead* / TestTruncate* / TestReadSchema / TestDescPinned + runRead helper) and read_test.go (two overflow cases), and glob+grep are merged into globgrep_test.go — fix: move the read tests and runRead into read_test.go so each source file maps to one test file (runTool in write_test.go is fine as shared) — contract-risk: none
- [testing-13] P3 internal/provider/provider_test.go:28 — no test in the four packages calls t.Parallel() although they are hermetic (per-test TempDir DBs, per-test httptest servers); the suite can't run faster and hidden shared-state usage would never surface — fix: add t.Parallel() to independent tests/subtests (keep TestRegistryListAndResolve serial — it uses t.Setenv) — contract-risk: none
- [testing-14] P3 internal/tool/tool_test.go:201 — sha256Ok(t, label, want) takes a `label` but always hashes the package variable `readDesc`; extending the pin suite to a second desc file would silently re-hash readDesc (confusing failure, or a wrong pass if hashes coincidentally match) — fix: pass the desc content/bytes as a parameter instead of the hard-coded variable (pin values unchanged) — contract-risk: none

## Deferred / Noted (no fix in 0.1.2)
- provider_test.go:133 handler-incremented `hits` counter is read by the test goroutine without synchronization; verified clean under `go test -race -count=30` on Go 1.26.5 (a minimal repro is also clean — the HTTP round trip orders the accesses on this toolchain); make it atomic if the handler ever does post-response work.
- TestBashTimeoutKillsAndReports (bash_test.go:75) costs ≥300 ms wall time; inherent to a real subprocess timeout (no sleep-polling flakiness); lower the `timeout` arg to ~100 ms if suite time ever matters.
- fake.go:21 titleMarker matches the first line of pinned prompt/title.txt today (verified); the coupling is implicit until fake-package tests exist (testing-3).
- common_test.go:36 sseServerSplit writes the fixture byte-by-byte but flushes once at the end — the mid-frame coverage is parser-level, not socket-delivery-level; noted, not a defect.

## Stats
P0:0 P1:0 P2:5 P3:9
COVERAGE: full — skipped: none (internal/tool/read_test.go was absent from the orchestrator list, located in-package, and reviewed)

## Status (wave 10 fix pass, 2026-08-21)
- testing-1 → FIXED 02a8a0d
- testing-2 → FIXED 05b4ff1
- testing-3 → FIXED ba99613
- testing-4 → FIXED 02a8a0d
- testing-5 → FIXED ce4669b
- testing-6 → FIXED b1952f2
- testing-7 → FIXED b1952f2
- testing-8 → FIXED b1952f2
- testing-9 → FIXED 02a8a0d
- testing-10 → FIXED e0e590e
- testing-11 → FIXED e0e590e
- testing-12 → FIXED 0b54534
- testing-13 → FIXED bc9665c
- testing-14 → FIXED 05b4ff1
