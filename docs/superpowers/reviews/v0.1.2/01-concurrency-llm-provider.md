# golang-concurrency — llm+provider
Date: 2026-08-20 · chunk: llm+provider (backfill) · source files: 12
## Findings
- [concurrency-1] P1 internal/llm/openai.go:32 (same at internal/llm/anthropic.go:38) — non-2xx path calls resp.Body.Close() BEFORE oaUpstreamError drains the body, so the {"error":{"message":...}} envelope is never decoded; every 4xx/5xx surfaces as "upstream error (HTTP NNN): <status line>" instead of the API message (helper's own "body ≤ 4KB drained" contract is dead code; verified: ReadAll after Close returns 0 bytes vs 33 in control order) — fix: call oaUpstreamError before resp.Body.Close() at both sites (or move Close into the helper after ReadAll), and assert the decoded message in TestOpenAIUpstream429IsTransient which currently pins only IsTransient — contract-risk: behavior — status: DEFERRED contract (tag: behavior)
## Deferred / Noted (no fix in 0.1.2)
- zen.go:166 writeAtomic uses a fixed "<cache>.tmp" name: two concurrent yolo processes could truncate/rename each other's tmp (corrupt catalog until next refresh); in-process safe, multi-instance startup not an assumed use
- no goleak tracking in llm/provider tests (goleak not in the pinned dep set); reader-goroutine exit proven manually: body read is ctx-bound via NewRequestWithContext, send() selects ctx.Done(), close(ch) deferred; -race green on both packages
- fake driver's once-per-stream time.After (fake.go:130) and len(parts)+1 buffer are not defected (not a hot loop; bounded); sticky-flag Auto design follows documented deviation 35, not flagged
## Stats
P0:0 P1:1 P2:0 P3:0
COVERAGE: full — skipped: none
