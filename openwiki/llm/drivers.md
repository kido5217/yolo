---
type: concept
title: LLM Drivers
description: "The internal/llm provider-agnostic streaming chat interface: the Driver/PartStream contract, the OpenAI- and Anthropic-compatible HTTP/SSE drivers, the shared SSE pump, and the scripted fake driver (YOLO_LLM=fake) that keeps unit tests network-free."
tags: [llm, drivers, sse, streaming, openai, anthropic, fake-driver]
verified:
  - by: openwiki/0.4.0
    at: 2026-08-26T18:04:14.871Z
sources:
  - id: openwiki-source-596f24d929bbc555ab74e86b
    resource: repo://internal/llm/anthropic.go
  - id: openwiki-source-15eeab880b360d20063c007a
    resource: repo://internal/llm/fake/fake.go
  - id: openwiki-source-552b6d83e1d1b259e32d81a4
    resource: repo://internal/llm/llm.go
  - id: openwiki-source-3345efbdfe5cc3508c2d2456
    resource: repo://internal/llm/openai.go
  - id: openwiki-source-ddc0f65decb5783c8ba8d062
    resource: repo://internal/llm/sse.go
  - id: openwiki-source-91aefa64656a21284db3a0ae
    resource: repo://internal/server/deps.go
generated: {by: "opencode", at: "2026-08-26T18:04:14.871Z"}
---

# LLM Drivers

`internal/llm` defines the **provider-agnostic streaming chat interface** and its
implementations: the OpenAI-compatible and Anthropic-compatible HTTP/SSE
drivers, plus the scripted **fake driver** that makes unit tests network-free.
The session engine consumes drivers *only* through this interface (selected via
the provider registry — see the providers page), so the engine never knows which
wire protocol a model speaks.

## The Driver contract

`Driver` is a single method (internal/llm/llm.go:94-97):

```go
type Driver interface {
    Stream(ctx context.Context, req Request) (PartStream, error)
}
```

`Request` carries the model id, API key, base URL, chat messages (roles
`system|user|assistant|tool`, with `ToolCallID` for tool results and `ToolCalls`
for assistant tool invocations), the tool definitions, and optional
temperature/max-tokens (internal/llm/llm.go:32-61).

`PartStream.Next(ctx)` blocks for the next `Part`; it errors only when the
context is done (`ctx.Err()`) or after the final part was delivered (`io.EOF`)
(internal/llm/llm.go:75-92). A `Part` is one emitted stream piece in stream
order with `Kind` `"text"|"reasoning"|"tool"`; tool parts carry `Name`, a
stable `CallID`, and `Args`; the **final part** carries `Usage` (token
accounting) and `Finish` (`"stop"|"tool_calls"|"length"|"error"`), and `Err` is
non-nil only on the final part after a 200 began (internal/llm/llm.go:63-73).

## The error model

- **`APIError`** — a decoded non-2xx upstream response. `Body` is the drained,
  capped (64 KiB) response body; `Message` is the provider-decoded error text
  (`""` when the body carries no message). The text keeps the
  `upstream error (http %d): ...` framing so pins and logs stay comparable
  (internal/llm/llm.go:114-133).
- **`TransientError`** — wraps a retryable upstream failure with its HTTP status
  (internal/llm/llm.go:105-112).
- **`IsTransient(err)`** reports retryability: a 429/5xx `TransientError` **or a
  network error**; context errors are **not** transient
  (internal/llm/llm.go:135-144). This is the predicate the engine's
  pre-stream retry path uses.

`upstreamError` (internal/llm/openai.go:39-74, shared by both drivers) drains the
non-2xx body (capped at `errBodyCap = 64 KiB`) *before* close, decodes the
provider message (the `{"error":{...}}` envelope first, then a plain-string
error, then the trimmed body, then the status line), and returns an `*APIError` —
with 429/5xx wrapped in `*TransientError`.

## The OpenAI driver

`OpenAI.Stream` POSTs `{BaseURL}/chat/completions` with `Authorization: Bearer
<key>`, `stream: true`, and `stream_options.include_usage: true`
(internal/llm/openai.go:13-37, 111-147). The reader (`oaReadSSE`)
(internal/llm/openai.go:189-276) parses each SSE chunk:

- `content` → `text` part; `reasoning_content` (falling back to `reasoning`) →
  `reasoning` part;
- `tool_calls` are **accumulated per index** (id, name, and arguments streamed
  as fragments are joined) and emitted as `tool` parts in first-seen order at
  finish;
- `usage` (a `prompt_tokens`/`completion_tokens` frame with
  `completion_tokens_details.reasoning_tokens` and
  `prompt_tokens_details.cached_tokens`) is captured and delivered on the final
  part;
- the `[DONE]` sentinel (or end of stream) triggers `finish`, which emits the
  tool parts and then the final part with `Finish` (the provider finish reason,
  or `"error"` with a `stream ended without finish reason` error if none).

A typed delta struct means a chunk decodes without per-token map allocation and
interface boxing (internal/llm/openai.go:162-170).

## The Anthropic driver

`Anthropic.Stream` POSTs `{BaseURL}/messages` with `x-api-key` and
`anthropic-version: 2023-06-01` (internal/llm/anthropic.go:19-44). Request
mapping (internal/llm/anthropic.go:63-131): system messages are joined into the
top-level `system` string (`\n\n`-separated); a `tool` role becomes a `user`
message with a `tool_result` block; an assistant message with tool calls becomes
a block list of `text` + `tool_use` blocks (args default to `{}` when empty);
`max_tokens` defaults to 8192. The reader (`anReadSSE`,
internal/llm/anthropic.go:170-251) switches on event type:
`message_start` (input tokens), `content_block_start`/`delta`/`stop` (text,
thinking → reasoning, and `input_json_delta` accumulated per block into tool_use
args), `message_delta` (stop reason + output tokens), `message_stop` (finish),
and `error`. `anFinish` maps the stop reason: `end_turn`→`stop`,
`tool_use`→`tool_calls`, `max_tokens`→`length` (internal/llm/anthropic.go:253-264).

## The shared SSE pump

`sseLoop` (internal/llm/sse.go:10-56) is the shared SSE data-frame pump both
drivers use. It reads byte-based lines, collects each blank-line-delimited
frame's trimmed `data:` values and joins them with `'\n'` (multi-line `data`
fields are valid SSE) before calling `process`. `onErr` receives the first
non-EOF read error. When `done()` reports true (the driver's finish already ran
— `[DONE]` / `message_stop`) the loop stops early so a body that never
terminates does not hold the engine round hostage. `flushTail` controls whether a
partial frame at stream end is processed before `finish()` runs exactly once.

## The fake driver (offline test seam)

`internal/llm/fake` is a **scripted `llm.Driver`** for engine tests and the
`YOLO_LLM=fake` wiring (internal/llm/fake/fake.go:1-16, 89-146). Every `Stream`
call is recorded in `ReqLog` (exposed via `Requests()`). A `Turn` is one scripted
reply: `Parts` emitted in order (the last MUST carry `Finish`), an optional
`Err` (Stream returns the zero stream and that error), `Auto` (the synthesizing
placeholder), and `Delay` (holds the reply open for `d` before any part is
emitted — slow-turn tests).

**Title routing:** requests whose first system message starts with the
title-generation marker (`"You are a title generator"`, the first line of
`prompt/title.txt`) draw from `TitleTurns`; all other requests draw from
`Turns` (internal/llm/fake/fake.go:18-21, 94-105). `AutoText()` marks the
synthesizing placeholder: when no scripted turn remains, a text part
`"ok-<n>"` (where `<n>` is the 1-based request number) is emitted instead of an
empty stream (internal/llm/fake/fake.go:40-42, 106-124). For `Kind:"tool"`
parts, `Text` holds the tool-arguments JSON (a locked convention mirroring
`Args` for script readability).

`FromScript` loads a driver from a JSON script file (M5 format:
`[{"parts":[{"kind":"text","text":"hi","finish":"stop",...}],"delay_ms":0}]`),
with optional per-turn `delay_ms` (internal/llm/fake/fake.go:163-192). The env
gate is `server.FakeFromEnv` (internal/server/deps.go:10-35): unset →
production drivers; `"fake"` + script → the scripted driver; `"fake"` without a
script, or any other value → an error (500 at boot). This is the seam that lets
the whole suite run the same code path as a live run without touching the
network.

## Representative tests

- Driver wire behavior (chunk accumulation, usage, finish, error decoding) is
  unit-tested in `internal/llm` against recorded SSE fixtures.
- Engine behavior is tested against the fake driver
  (`internal/session/*_test.go`); the server's `fake_env_e2e_test.go` boots the
  full stack through the same `YOLO_LLM=fake` env gate (see the testing page).
