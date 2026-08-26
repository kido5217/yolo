---
type: concept
title: Testing Strategy
description: "How the suite verifies behavior with no network: the YOLO_LLM=fake + YOLO_FAKE_SCRIPT scripted driver, the server wire-contract suites (golden, sse_ordering, fake_env_e2e, scope), the testutil blackbox harness (the TUI test escape hatch), teatest goldens, TestImportsDirection, sha256 prompt/description pins, and the user-run scripts/e2e-live.sh."
tags: [testing, fake-driver, wire-contract, golden, teatest, env-gating, e2e]
verified:
  - by: openwiki/0.4.0
    at: 2026-08-26T18:04:14.871Z
sources:
  - id: openwiki-source-41612f8ed7b59c998588fda2
    resource: repo://cmd/yolo/deps.go
  - id: openwiki-source-23f7f075092612c9b403937d
    resource: repo://internal/AGENTS.md
  - id: openwiki-source-15eeab880b360d20063c007a
    resource: repo://internal/llm/fake/fake.go
  - id: openwiki-source-91aefa64656a21284db3a0ae
    resource: repo://internal/server/deps.go
  - id: openwiki-source-5b7822b784c9af4720615a4a
    resource: repo://internal/server/fake_env_e2e_test.go
  - id: openwiki-source-91ae46f8620ffc477b418d90
    resource: repo://internal/server/golden_test.go
  - id: openwiki-source-e9ac54be1f5cc9b916932944
    resource: repo://internal/server/scope_test.go
  - id: openwiki-source-01d95c64c836dd21c23ad332
    resource: repo://internal/server/sse_ordering_test.go
  - id: openwiki-source-a624d5cb35926b1e5fffd4b4
    resource: repo://internal/server/testutil/testutil.go
  - id: openwiki-source-de9dc637c2398a58d49722b9
    resource: repo://internal/session/prompt_test.go
  - id: openwiki-source-d0397239fcad93c406c7a9be
    resource: repo://internal/tool/glob_test.go
  - id: openwiki-source-76ce0b34c8925895c52f0673
    resource: repo://internal/tool/read_test.go
  - id: openwiki-source-c2cb02284a8a0a4cb045cdde
    resource: repo://internal/tui/app_test.go
  - id: openwiki-source-99b37a6823820f4cb0c51a48
    resource: repo://internal/tui/imports_test.go
  - id: openwiki-source-533c7135c399c4d7221ac8d7
    resource: repo://scripts/e2e-live.sh
  - id: openwiki-source-8dd676e2fbca5d25bbd4a0e5
    resource: repo://scripts/tui-theme-golden.mjs
generated: {by: "opencode", at: "2026-08-26T18:04:14.871Z"}
---

# Testing Strategy

The whole suite runs **offline**. Unit and integration tests **never hit the
network** — live model paths are **env-gated**, and the one real-network check
is a **user-run** script, never CI. The core idea: a **scripted fake LLM
driver** lets the full stack (server → session → tools → storage) be exercised
deterministically.

## The scripted fake driver

`internal/llm/fake` is a scripted `llm.Driver`. A `Turn` is one scripted
`Stream` reply: parts emitted in order, the last of which **must** carry
`Finish`; for `Kind:"tool"` parts the tool arguments JSON lives in `Text`
(locked convention — `Part.Args` is the canonical carrier) (`fake.go`).

`Driver` serves turns in order and records every request in `ReqLog`
(`Requests()` returns a copy). Title-generation requests — whose first system
message starts with the `titleMarker` (`prompt/title.txt` first line) — draw
from `TitleTurns`; everything else draws from `Turns`. When no turn remains:
with `AutoText()` a synthesized `"ok-<n>"` text part is emitted (the
harness-doesn't-care placeholder for server tests / in-process smoke); without
it the stream is empty (immediate EOF). `Turn.Err` makes `Stream` return an
error, and `Turn.Delay` holds the reply open (slow-turn tests).

`FromScript(path)` loads a driver from a **JSON script file** (M5 format):
`[{"parts":[{"kind":"text","text":"hi","finish":"stop",...}],"delay_ms":0}]`
(`fake.go`).

## Env-gating: `YOLO_LLM` / `YOLO_FAKE_SCRIPT`

The production path selects drivers by environment. `FakeFromEnv` in
`internal/server/deps.go` (and the wiring in `cmd/yolo/deps.go`) implements the
gate:

- **unset** → `(nil, nil)`: production drivers (the `Loader.Env` convention).
- **`YOLO_LLM=fake` + `YOLO_FAKE_SCRIPT`** → the scripted driver loaded from
  that JSON script.
- **`YOLO_LLM=fake` without a script, or any other value** → error (500 at
  boot).

So `yolo serve` with `YOLO_LLM=fake` runs the exact same path the offline
e2e uses — the suite never needs a live endpoint.

## Server wire-contract suites

`internal/server` carries the upstream-mirror check — the four suites that pin
the legacy REST/SSE contract against opencode v1.18.18:

- **`golden_test.go`** — `golden` performs one canonical request, **normalizes**
  the JSON body (generated IDs → `<PREFIX><n>` deduped per concrete id, the
  test project dir + basename → `<DIR>/<DIRNAME>`, epoch-millisecond ints →
  `<T>`, maps re-emitted key-sorted), and compares — or with `-update`,
  regenerates — against `testdata/golden/<name>.json`.
- **`sse_ordering_test.go`** — `TestSSEOrdering` asserts the **exact** faithful
  frame order for one text turn: the user message/part are published
  synchronously in `Send` **before** the turn goroutine emits `busy`, matching
  upstream v1.18.18 and deviating from the plan's pinned "busy first" order
  (see `PROGRESS.md`).
- **`fake_env_e2e_test.go`** — `newSrvFakeEnv` boots the full stack but wires
  the kido driver from `YOLO_LLM`/`YOLO_FAKE_SCRIPT` (the M5 gate), so it runs
  the same path as `yolo serve` with `YOLO_LLM=fake`. `TestFakeEnvConversation`
  drives a two-send conversation over HTTP, verifying messages/parts persist
  and a later model request replays history including a tool result. The name
  deliberately avoids the "e2e" of `scripts/e2e-live.sh` (which is network,
  user-run).
- **`scope_test.go`** — `TestScopeMatrix` verifies directory scoping: an absent
  scoping header falls back to the server work dir, and every id-scoped route
  404s for a session id belonging to a different directory.

## The blackbox harness (TUI escape hatch)

`internal/server/testutil` exports the **M5 server harness**: `TestServer`
boots the **full core server stack on a scripted fake provider (no network)**
— server + engine + bus + storage + temp data/home dirs — for black-box tests,
including TUI tests that drive the wire contract end to end. This is the
**TUI test escape hatch**: TUI `_test.go` files may import `internal/server/testutil`
(and `internal/llm{,/fake}`) even though the TUI is otherwise a pure client.

## TUI: teatest goldens + import purity

- **teatest**: `internal/tui` tests build a `teatest.NewTestModel` (initial
  80×24), drive the app, and assert on the **captured output stream**
  (`teatest.WaitFor` / `WaitFinished`) rather than on a live terminal — the
  v2 teatest output-stream model (`app_test.go`, `tui_suite_test.go`).
- **`TestImportsDirection`** (`internal/tui/imports_test.go`) **guards TUI
  purity** (core principle 4) by parsing every `internal/tui/*.go` file:
  non-test files may import only `internal/protocol` + `internal/tui/*` from
  within the module; `_test.go` files may additionally use the escape hatches.

## Pinned text (sha256)

The 14 `session/prompt/*.txt` files and the tool `desc/*.txt` files are
**sha256-pinned** by tests — the pins record current intended content, not an
upstream lock; an intentional change re-baselines the pin in the same commit:

- `TestPromptFilesPinned` (`internal/session/prompt_test.go`) pins all 14 prompt
  files.
- `TestDescPinned` (`internal/tool/read_test.go`) pins each tool description
  file; tool JSON-schema bytes are also pinned (e.g. the glob schema in
  `internal/tool/glob_test.go`).

## Live end-to-end (user-run only)

`scripts/e2e-live.sh` is the **ON-DEMAND** live end-to-end check against a real
kido endpoint — **NEVER run in CI, user-run only** (`KIDO_API_KEY=...`). It
boots `yolo serve` in a scratch project dir and drives the wire contract:
health → create session → send "list files in /tmp" (expect a completed
read/glob tool call + a non-empty assistant reply) → abort test → SIGTERM
(graceful shutdown, exit 0). Exits 0 on PASS, 1 on FAIL.

A separate golden-matrix oracle, `scripts/tui-theme-golden.mjs`, ports the
upstream **pure** theme-resolution functions (RGBA/ANSI) to Node so the Go
port is verified **bit-for-bit** against upstream, writing
`internal/tui/theme/testdata/theme-golden.json` (checked in).
