---
type: concept
title: Session Engine (Agent Loop)
description: "The internal/session engine that drives one user message through model/tool rounds: turn lifecycle, retry and overflow semantics, part bookkeeping, permission-gated tool execution, and Abort/Close/Shutdown invariants."
tags: [session, agent-loop, turn, retry, tool-execution, permissions]
verified:
  - by: openwiki/0.4.0
    at: 2026-08-26T18:04:14.871Z
sources:
  - id: openwiki-source-bb4569d49179af7f3184b471
    resource: repo://internal/session/engine.go
  - id: openwiki-source-b931ae975a64e5737c96adfe
    resource: repo://internal/session/policy.go
  - id: openwiki-source-79c24acd17d6312b4a07b6a5
    resource: repo://internal/session/prompt.go
  - id: openwiki-source-304d095693b256ae8b642b59
    resource: repo://internal/session/round.go
  - id: openwiki-source-d6bdfd1760b3c0aca7efa469
    resource: repo://internal/session/title.go
  - id: openwiki-source-c8be83aef956b9049cf244dd
    resource: repo://internal/session/tool_exec.go
generated: {by: "opencode", at: "2026-08-26T18:04:14.871Z"}
---

# Session Engine (Agent Loop)

`internal/session` owns the agent loop. `Engine` is constructed once per process (see
`cmd/yolo/deps.go`) and runs every conversation turn as a goroutine, persisting
through `internal/storage` and publishing wire events through `internal/bus`.
Construction is fail-fast: `DB`, `Bus`, `Prov`, `Perm`, and `Tools` are required,
so a miswired dependency is a construction error rather than a nil panic inside an
un-recovered turn goroutine (internal/session/engine.go:91-106).

## Turn lifecycle

`Send(ctx, sessionID, text, onDone)` (internal/session/engine.go:174-228):

1. Loads the session row and resolves the session's `provider/model` ref through
   the provider registry.
2. Registers the session in the busy map under the engine lock; a second `Send`
   for the same session returns `ErrSessionBusy`.
3. Persists the user message and its text part, publishing
   `message.updated` + `message.part.updated` for both.
4. Spawns `runTurn` in a goroutine and returns `SendResult{MessageID, PartID}`.

`runTurn` (internal/session/engine.go:490-572) publishes
`session.status busy`, loads config + permission rules + the auth key, builds the
system prompt and one-shot history snapshot, then loops model rounds up to
`maxToolRounds = 50` (internal/session/engine.go:35-43). On exit — success,
failure, abort, or retry exhaustion — it publishes `session.status idle`, removes
the busy entry, and fires `onDone` exactly once. A panic anywhere in a turn
(tool, driver, or DB) is recovered and reported as a failed turn through the same
exit path: the single binary never crashes on turn panics
(internal/session/engine.go:492-500).

Per-turn budgets reset on every `Send`:

- `maxToolRounds = 50` — model round-trips per turn.
- `maxToolSteps = 50` — tool calls per turn.
- `maxRetryAttempts = 4` — stream attempts per round (initial request included).

## Round lifecycle: retry and overflow

Each round creates the assistant message row *before* the first stream attempt, so
a failed round still finalizes a (possibly empty) assistant message
(internal/session/round.go:132-147). `streamWithRetry`
(internal/session/round.go:271-330) applies the LOCKED pre-stream policy:

- **Transient failures** (429/5xx/network) retry up to `maxRetryAttempts` with
  exponential backoff — `1s × 2^(attempt-1)` scaled by uniform jitter in
  `[0.8, 1.2]` (internal/session/engine.go:150-156) — while nothing of the round
  is persisted, emitting `session.status retry` (with attempt, message, and next
  delay) before each sleep.
- **Overflow** (below) ends the round with a synthetic note and `errRoundEnded`;
  the turn ends idle.
- **Any other pre-stream failure** saves a synthetic error note, finalizes the
  round, and fails the turn.

Mid-stream failures are never retried: partial text is kept, a synthetic error
note is attached, and the turn fails (internal/session/round.go:166-204).

Context overflow is detected two ways (internal/session/round.go:250-257,
476-533):

- **By usage** — the round's `usage.Input` exceeds `model.Context`.
- **By API error** — a byte-faithful port of opencode's curated
  classifier: 27 case-insensitive overflow patterns (e.g. `prompt is too long`,
  `model_context_window_exceeded`) with rate-limit exclusions (AND-NOT), plus
  status rules: 413 always, 400/413 with empty body, or body
  `error.code == "context_length_exceeded"`.

Overflow stops the turn with a fixed synthetic note ("context overflow: model
context N exceeded by input M tokens… v1 has no compaction"); v1 has no
compaction path.

## Part bookkeeping

`partState` (internal/session/round.go:28-108) drives text/reasoning parts:

- **Start** on the first delta: mints a part id, publishes one
  `message.part.updated` (created) plus the first `message.part.delta`.
- **Delta**: wire-only — text accumulates in memory and *no per-delta DB write
  happens* (it would be O(n²) for long responses). A crash mid-turn loses the
  in-flight text; this is an accepted trade.
- **Finalize**: the sole DB upsert, publishing the terminal
  `message.part.updated` with an end time. Finalization uses
  `context.WithoutCancel` so an abort cannot strand the part "running" in the
  store (same pattern for tool parts and `finishRound`,
  internal/session/round.go:351-362).

Tool parts are created "running" before execution and finalized "completed" or
"error" afterwards. The tool part id **is** the model call id — call ids are not
persisted elsewhere, and history replay needs them to pair assistant `ToolCalls`
with `RoleTool` results (internal/session/round.go:110-117). A tool round that
continues the text stream after a tool call starts a *new* text block (fresh part
id, upstream parity) instead of re-using the finalized part's id
(internal/session/round.go:224-231).

`finishRound` (internal/session/round.go:336-377) completes the assistant row
with cost (derived from usage × model price, per-million) and tokens, re-publishes
`message.updated` with the final state, and appends the completed round to the
turn's in-memory history snapshot.

## History and system prompt

At turn start `loadHistory` (internal/session/engine.go:596-643) builds the system
text and snapshots the full message history **once**; each round appends its
completed assistant message to the snapshot, so `messagesFor` maps memory instead
of re-querying the store every round. Synthetic parts (error notes, overflow
notes, flagged `IsSynthetic`) are excluded from replay: the model must never see
engine-generated notes (internal/session/round.go:400-422,
internal/session/engine.go:628-632).

System texts are ordered `[family prompt, env block, config instructions]`
(internal/session/prompt.go:171-196, internal/session/engine.go:596-611):

- The **family prompt** is one of 14 sha256-pinned embedded files under
  `internal/session/prompt/`, selected by pinned first-match-wins rules on the
  api/provider id (`muse` → meta, `gpt-4`/`o1`/`o3` → beast, `gpt` → codex/gpt,
  `gemini-` → gemini, `claude` → anthropic, `trinity`, `kimi`, else default).
- The **env block** carries model id, working directory, git-repo status
  (cached, 60 s TTL, 2 s exec timeout), platform, and date.
- The **nearest AGENTS.md** (walk-up from the project dir, max 32 hops, nearest
  wins) is appended when present (internal/session/prompt.go:196-212).
- Config `instructions` files are appended relative to the project dir
  (internal/session/engine.go:601-611).

## Tool execution and permission gates

`executeTool` (internal/session/tool_exec.go:34-172) runs each model-issued call
through the LOCKED gate sequence, then the tool itself:

1. **Doom check** — a sliding window on the turn's call history
   (sha256 of canonical sorted-key args): the third identical call of the turn
   fires a `doom_loop` ask *before* the part goes "running"; a "once" reply does
   not extend the exemption (internal/session/tool_exec.go:174-222).
2. **Hidden guard** — a tool denied by a wildcard rule is not offered to the
   model; if called anyway, the part errors "tool not available"
   (internal/session/tool_exec.go:79-83).
3. **External-directory gate** — one ask per outside directory pattern
   (`<dir>/*`) for tool paths outside the session dir; the part is "running"
   first so the TUI shows the pending state
   (internal/session/tool_exec.go:229-256).
4. **Core ask** — with `Resources`/`Always` from the tool's `Patterns`
   (internal/session/tool_exec.go:261-281).

The session ruleset is assembled in LOCKED order — agent builtins (unknown agents
fall back to the build matrix) + config permission rules + the session's
persisted "always" rules — and is **re-read per round and per tool call**, so an
"always" reply applies from the very next evaluation (internal/session/policy.go:
11-50). Tool visibility uses the same ruleset: a wildcard deny on "edit" hides
both `edit` and `write` (internal/session/policy.go:52-79).

Every gate path finalizes the part: deny → "permission rejected"; context cancel
while parked (Abort) → "aborted"; invalid args or unknown tool → hard error. The
model continues either way. The bash shell is lazily spawned per session
(`shellFor`) and handed to tools via `tool.Env`; a call for a *closed* session
gets a "shell is not initialized" tool error instead of re-spawning a leaked
shell (internal/session/engine.go:431-447).

When the per-turn step budget is exhausted, the remaining tool calls of the
final stream are **dropped** (not persisted, not executed) and the turn ends
idle (internal/session/round.go:233-240).

## Abort, Close, Shutdown

- **Abort** (internal/session/engine.go:244-255) cancels the active turn and its
  title side-call *under the busy-map lock*: a turn starting in the window gets
  its own cancel, never the previous turn's (TOCTOU). Abort is user-initiated,
  not a failure: partial text is kept and `context.Canceled` is logged at the
  send boundary.
- **Close** (internal/session/engine.go:263-290) aborts the in-flight turn,
  marks the session deleted (further events for it are suppressed —
  `eventSuppressed` covers the engine's five prop shapes), and closes the bash
  shell only after the turn settles (bounded 2 s wait, then hard close). A
  post-DELETE tool call must not re-spawn a leaked shell.
- **Shutdown** (internal/session/engine.go:295-349) cancels every active turn
  and title goroutine, waits for them to release (at most 5 s, or ctx deadline),
  then releases all session shells.

## Title generation

`maybeScheduleTitle` fires the one-shot title generation for a session's first
user message while the title is still the default ("New session"): a 30 s
timeout side-call with the pinned `prompt/title.txt`; best-effort — errors are
dropped, the title is truncated to 50 runes, and the title goroutine is tracked
(`titleCtx` + `titleWait`) so Abort and Shutdown can cancel/wait on it
(internal/session/title.go:21-96).

## Representative tests

- `TestSingleTextTurnEndToEnd`, `TestRunTurnRecoversPanic`,
  `TestTextDeltasEmitSSEAndPersistAtFinalize`, `TestHistorySnapshotAccumulatesAcrossRounds`,
  `TestCloseWhileBusyAbortsAndSuppresses`, `TestShutdownAbortsActiveAndWaits`
  (internal/session/engine_test.go).
- Lifecycle: `internal/session/lifecycle_test.go` / `lifecycle_internal_test.go`.
- Permissions: `internal/session/engine_perm_test.go`; overflow:
  `overflow_internal_test.go`; prompts: `prompt_test.go` (sha256 pins).
- Benchmarks: `engine_bench_test.go`.
