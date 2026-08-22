# golang-benchmark — hotpaths
Date: 2026-08-21 · commit: e2946cfc52e620126cb6a497c44e44890e367654
## Benchmarks added
- internal/storage/dao_bench_test.go: BenchmarkProtocolToPart, BenchmarkUpsertPart, BenchmarkPartToProtocol — the per-delta persist path pinned by perf-1/2/3: text parts at 1KB/64KB/128KB (plus the end+synthetic finalization shape) and a tool part with 64KB streamed State.Input; UpsertPart at 1KB/64KB/256KB state_json re-writing one row (the streaming update shape) plus a 1KB insert over a 1024-id ring, on a temp-file WAL DB exactly like storage_test.go; PartToProtocol decodes rows produced by the real encoder (round-trip contract). Prose fixture carries quotes/unicode/newlines so JSON-escape cost is representative.
- internal/session/engine_bench_test.go: BenchmarkCallKeyHash, BenchmarkMessagesFor — doom-window identity hash (flat 1KB + nested ~32KB args, unmarshal + sorted-key re-marshal + sha256, the deferred 207–240µs evidence shape) and the perf-7 P2 per-round history mapping over 100 messages / 300 parts (50 user × 3 text, 50 assistant × text + 2 tool parts, first 20 carrying 32KB-input/64KB-output tools), temp DB + static offline provider registry; one untimed warm call primes the git-repo detection cache.
- internal/llm/openai_bench_test.go: BenchmarkOAReadSSE — 1k/10k SSE frames over bytes.Reader (content deltas + two tool calls announced once with 48 argument fragments each + finish + usage + [DONE], channel drained), the perf-5 P3-fixed byte-based decode pipeline.
- internal/llm/anthropic_bench_test.go: BenchmarkAnReadSSE — 1k/10k deltas (text_delta + one tool_use block of input_json fragments) inside the real event:/data: message_start…message_stop envelope.
- internal/tui/session_bench_test.go: BenchmarkRenderMessages — inventory hot-path #5 (never measured by wave 12, out of chunk scope): 50/200 message transcripts × 5 parts (2 text, reasoning, completed tool, error tool), width 80, collapsed and expanded (I/O + reasoning bodies shown).
## Baseline numbers (1x runs, note: relative only)
Ryzen 7 5800X3D, Go 1.26.7, N=1 first-iteration includes cold-cache effects; absolute values inflated, treat as relative:
- storage::BenchmarkProtocolToPart — 1KB 21.6µs/15, 64KB 157.9µs/28, 128KB 493.6µs/31, 128KB_final 339.8µs/32, tool/input64KB 149.4µs/30 (ns/op, allocs/op)
- storage::BenchmarkUpsertPart — update/1KB 113.4µs/20, update/64KB 301.1µs/14, update/256KB 963.2µs/14, insert/1KB 62.0µs/19
- storage::BenchmarkPartToProtocol — text/64KB 319.1µs/8, text/128KB 863.6µs/8, tool/input64KB 328.7µs/18
- session::BenchmarkCallKeyHash — flat/1KB 26.2µs/31, nested/32KB 775µs/9043
- session::BenchmarkMessagesFor — 26.0ms/12296 (101 DB queries + part decode per round)
- llm::BenchmarkOAReadSSE — frames/1000 1.61ms/16841, frames/10000 15.65ms/151841 (≈1.57µs/frame)
- llm::BenchmarkAnReadSSE — deltas/1000 1.86ms/15111, deltas/10000 17.20ms/150119 (≈1.72µs/frame)
- tui::BenchmarkRenderMessages — collapsed/50 607.8µs/1508, collapsed/200 2.26ms/6013, expanded/200 3.36ms/8812
## Deferred
- Candidate 9 `(*Store).Apply` — wave 12 reviewed the fold and verified appendDelta zero-copy (builder aliasing, no finding); no hot-path evidence to pin a baseline against.
- Candidate 10 `Truncate` — no performance-wave evidence bounding it (deviation 11 pinned its semantics); single-pass tail cut, not on a measured hot path.
## Stats
P0:0 P1:0 P2:0 P3:8
COVERAGE: full — skipped: candidates 9 (store.Apply), 10 (Truncate), both deferred with reason above
