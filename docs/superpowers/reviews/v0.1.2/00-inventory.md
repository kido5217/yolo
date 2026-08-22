# v0.1.2 Inventory

## 1. Package metrics

wc -l over `*.go`; recursive per package (subpackages listed in their parent block).
Large files = every file >500 lines.

- internal/auth — 224 total; large files: none
  121 auth.go · 103 auth_test.go
- internal/bus — 151 total; large files: none
  70 bus.go · 81 bus_test.go
- internal/config — 420 total; large files: none
  304 config.go · 116 config_test.go
- internal/glob — 187 total; large files: none
  158 glob.go · 29 glob_test.go
- internal/llm — 1310 total (incl. fake/fake.go 192); large files: none
  309 openai.go · 281 anthropic.go · 192 fake/fake.go · 185 llm_test.go ·
  144 anthropic_test.go · 121 llm.go · 78 common_test.go
- internal/log — 230 total; large files: none
  119 log.go · 111 log_test.go
- internal/permission — 797 total; large files: none
  373 service.go · 145 service_test.go · 115 permission.go · 94 permission_test.go · 70 builtins.go
- internal/protocol — 575 total; large files: none
  156 protocol_test.go · 101 event.go · 71 config.go · 58 session.go · 42 provider.go ·
  40 message.go · 37 part.go · 30 agent.go · 26 id.go · 7 command.go · 7 errors.go
- internal/provider — 814 total; large files: none
  295 provider.go · 210 provider_test.go · 171 zen.go · 75 seams.go · 63 kido.go
- internal/server — 2605 total (incl. testutil/testutil.go 285); large files: contract_test.go (594)
  397 server_test.go · 382 handlers_misc.go · 362 handlers_session.go · 285 testutil/testutil.go ·
  267 handlers_misc_test.go · 215 server.go · 41 sse.go · 36 errors.go · 26 deps.go
- internal/session — 2861 total; large files: engine.go (1145), engine_test.go (612)
  1145 engine.go · 612 engine_test.go · 334 engine_perm_test.go · 246 lifecycle_test.go ·
  218 prompt.go · 195 prompt_test.go · 89 policy.go · 22 lifecycle_internal_test.go
- internal/storage — 889 total; large files: dao.go (502)
  502 dao.go · 172 storage_test.go · 117 migrate.go · 98 db.go
- internal/tool — 2512 total; large files: none
  374 read.go · 320 shell.go · 236 grep.go · 210 tool_test.go · 200 edit.go · 167 glob.go ·
  128 todowrite.go · 126 bash.go · 124 tool.go · 118 globgrep_test.go · 108 bash_test.go ·
  100 edit_test.go · 144 write.go · 66 todowrite_test.go · 48 write_test.go · 43 truncate.go
- internal/tui — 5892 total (incl. client/ 368, store/ 414); large files: app.go (826)
  826 app.go · 432 model_test.go · 418 prompt_test.go · 320 session.go · 318 permission_test.go ·
  316 model.go · 305 agent_test.go · 287 client/client.go · 273 app_test.go · 265 tui_suite_test.go ·
  260 session_test.go · 233 store/store.go · 192 home_test.go · 160 home.go · 147 agent.go ·
  181 store/store_test.go · 110 prompt.go · 106 footer_test.go · 97 permission.go · 91 footer.go ·
  88 help_test.go · 84 toast_test.go · 81 client/event.go · 79 toast.go · 75 imports_test.go ·
  69 client/client_test.go · 57 client/event_test.go · 22 style.go
- cmd/yolo — 659 total; large files: none
  394 main.go · 265 main_test.go

Repo total (all .go above): 20126 lines; non-TUI: 14234 (matches plan R15a).
>500-line files, all of them: internal/session/engine.go (1145), internal/tui/app.go (826),
internal/session/engine_test.go (612), internal/server/contract_test.go (594), internal/storage/dao.go (502).

## 2. Hot-path candidates

1. `internal/session/engine.go:runRound` (560) — per-token stream loop: every delta appends to a
   strings.Builder, re-serializes the accumulating part, SQLite UpsertPart, 1–2 bus Publishes; the
   hottest function in the server (runs per model chunk, up to 50 rounds).
2. `internal/server/sse.go:handleEvent` (10) — the real SSE emit path: json.Marshal + Fprintf +
   Flush per event per subscriber for the whole session lifetime.
3. `internal/bus/bus.go:Publish` (56) — every event goes through this: mutex + non-blocking
   channel fan-out to all subscribers; overflow drops (closes) the subscriber.
4. `internal/tui/store/store.go:Apply` (29) — per SSE event: json.Unmarshal of props + linear
   O(M×P) scan in upsertPart/applyDelta; feeds the render path on every streamed delta.
5. `internal/tui/app.go:view` (777) → viewSession → `internal/tui/session.go:renderMessages` (84)
   — the per-frame TUI render path: every bubbletea tick (default 60fps) rebuilds the entire frame
   (transcript re-rendered through lipgloss styling + all overlays).
6. `internal/llm/openai.go:oaReadSSE` (167) / `internal/llm/anthropic.go:anReadSSE` (161) — the
   network stream decode path: ReadString per SSE line, json.Unmarshal per chunk,
   map[string]any type-assert walks over the token stream.
7. `internal/storage/dao.go:UpsertPart` (229) — the per-delta SQLite DAO query: ON CONFLICT
   full-row upsert whose state_json grows with accumulated part text.
8. `internal/session/engine.go:messagesFor` (415) + `internal/session/prompt.go:BuildSystemPrompt`
   (158) — the prompt-build path, run per model round: full history re-read (ListMessages +
   ListParts N+1 queries, JSON decode of every part) plus system-prompt rebuild; cost grows with
   conversation length.

## 3. Incident areas (deviation log)

1. Deviation 11 — Truncate keeps the tail up to MaxBytes advanced to a UTF-8 boundary (upstream
   tail() semantics); initial test expectation counted removed runes, not kept.
   Files: internal/tool/truncate.go, internal/tool/read.go, internal/tool/read tests.
2. Deviation 12 — plain `import "embed"` + scalar `//go:embed` fails typecheck on both installed
   toolchains; `import _ "embed"` workaround is load-bearing.
   Files: internal/tool/read.go, write.go, edit.go.
3. Deviation 15 — emitted shell marker had a stray colon the reader regex (kept authoritative)
   never matched → Exec hung to the 120s timeout; reconciled the plan's own two pins.
   Files: internal/tool/shell.go.
4. Deviation 16 — first stderr-merge assigned the pipe's read end (O_RDONLY) to cmd.Stderr →
   child EBADF silently swallowed stderr (only the stdout marker arrived); now one os.Pipe(),
   write end on both fds 1+2, read end stored on shellProc and closed in reapProc.
   Files: internal/tool/shell.go.
5. Deviation 21 — engine adaptations bundle: racy title side-call vs turn assertions (ReqLog),
   fake-driver convention (args JSON in Part.Text), round-continuation robustness rule
   (finish=="tool_calls" OR any tool part, max 50), exactly one final part.updated, onDone
   exactly-once from the turn goroutine's defer.
   Files: internal/session/engine.go, internal/llm/fake/fake.go, engine_test.go, engine_perm_test.go.
6. Deviation 22 — two of T17's four pinned permission tests were already green under T16 code
   (fresh always-rule re-eval in decisionFor; wildcard hiding in buildRequest) — kept as
   regression guards; only deny/doom/abort differentiators needed T17 changes.
   Files: internal/session/engine.go, internal/permission/service.go, engine_perm_test.go.
7. Deviation 29 — `synthetic` flag round-trip added to part storage JSON so engine notes survive
   DB re-reads while staying excluded from history replay.
   Files: internal/storage/dao.go, internal/session/engine.go.
8. Deviation 30 — pinned server `Deps` forced an M0 reconcile: legacy New/Scope/Opt API dropped,
   CommandResponse DTO added, ProjectID/auth/config seams exported, fake grew auto-turn seams.
   Files: internal/server/{server,deps,handlers_*,sse}.go, internal/protocol/command.go,
   internal/storage/db.go, internal/auth/auth.go, internal/config/config.go,
   internal/provider/seams.go, internal/llm/fake/fake.go, cmd/yolo/main.go.
9. Deviation 33 — `POST /session/{id}/abort` blocks until Engine.Status=="idle" (≤2s, 10ms poll)
   before 200; without the settle-poll the pinned immediate status GET races the turn's defer.
   Files: internal/server/handlers_session.go, internal/session/engine.go.
10. Deviation 41 — plan pinned SSE first frame = busy status, but the faithful engine (upstream
    prompt.ts parity) publishes the user message+part BEFORE busy; test asserts actual order,
    engine unchanged.
    Files: internal/session/engine.go, internal/server/contract_test.go (golden).
11. Deviation 47 — plan TUI sketch assumed bubbletea v1 (View() string, tea.Quit as cmd, v1
    teatest run loop); implemented v2: tea.View struct + quitCmd, NewTestModel/Send/WaitFor with
    a stream that WaitFor consumes.
    Files: internal/tui/app.go, teatest suites.
12. Deviation 50 — v2 renderer stores the LATEST view per Update and flushes on the program
    ticker: intermediate frames coalesce away, the plan's 80x5 geometry is structurally impossible
    (assistant msg created before turn-2 streaming auto-follows the transcript), and two sequential
    WaitFor cannot both see one coalesced flush → single WaitFor, 80x10.
    Files: internal/tui/app.go, tui_suite_test.go and dependent blackbox tests.
13. Deviation 60 — alt-screen frame is fixed to terminal size → any overflow is silently
    truncated (5 teatest tests red, visible UI genuinely broken); fix: App.overlayLines() reserves
    overlay rows so the viewport shrinks (min 1); cell-diff also means an unchanged marker-line
    tail (` think`) is never re-emitted.
    Files: internal/tui/app.go, tui blackbox tests.
14. Deviation 64 — e2e asserts a completed tool call from read/glob/grep/bash (any), not the
    plan's "one read/glob tool call" — the real model chose `bash ls /tmp`, a legitimate choice.
    Files: scripts/e2e-live.sh.
15. Deviation 65 — e2e boots `yolo serve` from a scratch project dir with `provider.kido.baseURL`
    pinned in $PROJ/yolo.jsonc: the provider registry is built ONCE at startup from the
    startup-dir config, per-turn config never changes BaseURL, `KIDO_BASE_URL` is honored by the
    script (which writes the config), not by yolo.
    Files: scripts/e2e-live.sh, cmd/yolo/main.go, internal/provider/provider.go.

## 4. Benchmark candidates

1. `ProtocolToPart` — internal/storage/dao.go:297 — per-delta JSON encode on the emit path —
   Part with 128KB accumulated text plus a tool part with 64KB State.Input.
2. `(*DB).UpsertPart` — internal/storage/dao.go:229 — per-delta SQL upsert — in-memory/temp-file
   DB as storage_test.go does, 64KB state_json rows, N upserts.
3. `PartToProtocol` — internal/storage/dao.go:325 — inverse decode on the history-replay path —
   rows round-tripped through ProtocolToPart with large text/tool states.
4. `callKeyHash` — internal/session/engine.go:1118 — unmarshal + canonical marshal + sha256 per
   tool call (doom window) — ~32KB args JSON blob.
5. `(*Engine).messagesFor` — internal/session/engine.go:415 — per-round full history mapping
   (N+1 queries, per-part JSON decode, system-prompt build) — temp DB seeded with ~100 messages /
   300 parts, Deps with a fake cfg func.
6. `(*OpenAI).oaReadSSE` — internal/llm/openai.go:167 — SSE frame decode per token — ~10k
   pre-built `data:` chunks (text + tool_calls + usage frames) over bytes.Reader, channel drained.
7. `(*Anthropic).anReadSSE` — internal/llm/anthropic.go:161 — Anthropic event-frame decode per
   token — pre-built message_start / content_block_* / message_stop frames over bytes.Reader.
8. `renderMessages` — internal/tui/session.go:84 — per-frame transcript composition through
   lipgloss styling — store with ~200 messages / 1000 parts, width 80.
9. `(*Store).Apply` — internal/tui/store/store.go:29 — per-event fold (Unmarshal + scan +
   delta append) — message.part.delta event JSON against a pre-grown store.
10. `Truncate` — internal/tool/truncate.go:9 — tail + UTF-8-boundary cut on every read/bash
    output — 1MB multi-byte strings, with and without single-line overflow.

## 5. Chunk-list cross-check

Method: union of every file path appearing in the plan's chunk Scope blocks (waves 1–15,
dedup, 109 unique files: internal/*, cmd/yolo/*, scripts/e2e-live.sh) was existence-checked
with a shell loop (`[ -f ]` per path). The wave-13 "(e.g. …)" future benchmark file names
(internal/storage/bench_test.go, internal/llm/openai_bench_test.go, internal/session/
prompt_bench_test.go, internal/tool/read_bench_test.go, internal/tui/store/store_bench_test.go)
are files wave 13 creates, not chunk scopes — excluded.

- none
