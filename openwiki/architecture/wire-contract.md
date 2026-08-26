---
type: concept
title: "Wire Contract (protocol)"
description: "internal/protocol is the single source of truth for yolo's wire contract: the legacy REST paths, JSON shapes, and SSE event set mirrored from opencode v1.18.18, plus the id contract and the test suites that verify the mirror."
tags: [wire-contract, protocol, dto, sse, rest, ids]
verified:
  - by: openwiki/0.4.0
    at: 2026-08-26T18:04:14.871Z
sources:
  - id: openwiki-source-35b6e28a1e5217ac97361a86
    resource: repo://internal/protocol/AGENTS.md
  - id: openwiki-source-2858910bb1295e53e369022b
    resource: repo://internal/protocol/config.go
  - id: openwiki-source-122fd78d52d5ba05d7666eda
    resource: repo://internal/protocol/event.go
  - id: openwiki-source-2533779c275a98d5c47a6bde
    resource: repo://internal/protocol/id.go
  - id: openwiki-source-913bee9c78819f3c4699f914
    resource: repo://internal/protocol/part.go
  - id: openwiki-source-186a487bafa29a118f441b7c
    resource: repo://internal/protocol/session.go
  - id: openwiki-source-5b7822b784c9af4720615a4a
    resource: repo://internal/server/fake_env_e2e_test.go
  - id: openwiki-source-91ae46f8620ffc477b418d90
    resource: repo://internal/server/golden_test.go
  - id: openwiki-source-e9ac54be1f5cc9b916932944
    resource: repo://internal/server/scope_test.go
  - id: openwiki-source-7d4ac4eaf17b216fdc5fd692
    resource: repo://internal/server/server.go
  - id: openwiki-source-01d95c64c836dd21c23ad332
    resource: repo://internal/server/sse_ordering_test.go
generated: {by: "opencode", at: "2026-08-26T18:04:14.871Z"}
---

# Wire Contract (protocol)

`internal/protocol` is the **single source of truth for yolo's wire contract**:
the legacy REST paths/JSON shapes and the legacy SSE event set, mirrored from
opencode v1.18.18 so the port stays verifiable against its OpenAPI contract. The
package is wire-level by contract — DTOs, ids, and event shapes only; **no
business logic, no I/O** (internal/protocol/AGENTS.md:30-33). `server` emits the
shapes, `internal/tui` consumes them, and no other package defines or remarshal
wire structs; a wire change lands here first, with both sides following in the
same change.

## The DTO families

The package owns ten DTO files (internal/protocol/AGENTS.md:9-10):

- **message** (`message.go`) — `Message` (id, sessionID, role
  `"user"|"assistant"`, time, agent, model, parentID, …), `MessageTime`,
  `MessageModel`, `MessageError`, and `MessageWithParts` (info + parts, the
  hydration shape).
- **part** (`part.go:3-37`) — `Part` with `type` `"text"|"reasoning"|"tool"`,
  optional `callID`/`tool`/`state`, the `IsSynthetic`/`IsIgnored` flags, and
  `PartTime`; `ToolState` carries `status` `"running"|"completed"|"error"`,
  input, title, output, error, metadata, and time.
- **session** (`session.go`) — `Session` (id, projectID, directory, title,
  agent, model ref, cost, tokens, version, permission rules),
  `SessionStatus` with `type` `"idle"|"busy"|"retry"` plus retry attempt/message/
  next fields, `Tokens`, `CacheTokens`, `ModelRef`, and `Todo`.
- **provider** (`provider.go`) — `Provider`, `Model`, `ModelCost`, `ModelLimit`,
  `ProviderAuth`.
- **agent** (`agent.go`) — `Rule` (permission rule: permission, pattern, action)
  and `Agent`.
- **command** (`command.go`) — `Command` and `CommandResponse`.
- **config** (`config.go`) — `Config` and its nested types (`ToolOutput`,
  `ProviderConfig`, `CustomAgent`, `Profile`), plus `ParsePerms` which decodes a
  config `permission` map into a deterministic, sorted `[]Rule` (wildcard rules
  first, then specific, patterns ordered by length then lexically) — an invalid
  entry is an error the caller degrades to "no config rules"
  (internal/protocol/config.go:52-89).
- **errors** (`errors.go`) — the wire `Error` envelope.
- **event** (`event.go`) — the SSE event set (below).
- **id** (`id.go`) — the id contract.

## The id contract

`NewID(prefix)` mints `prefix + "_" + 20` random characters drawn from a 55-char
alphabet that matches the ID contract regex — digits minus 0/1, `A-Z` minus I/O,
`a-z` minus a/i/o (internal/protocol/id.go:7-26). Event ids use the `evt` prefix
via `NewEventID`.

## The SSE event set

`Event` is the SSE frame payload: `{"id","type","properties"}` where
`properties` is `json.RawMessage`. `MakeEvent` wraps typed props in this legacy
envelope with a fresh event id (internal/protocol/event.go:8-35). The legacy
event set is exactly ten types (internal/protocol/event.go:15-26):

| type | props |
|---|---|
| `message.updated` | `sessionID`, `info` (Message) |
| `message.part.updated` | `sessionID`, `part` (Part), `time` |
| `message.part.delta` | `sessionID`, `messageID`, `partID`, `field` (`"text"\|"reasoning"\|"input"`), `delta` |
| `message.removed` | `sessionID`, `messageID` |
| `message.part.removed` | `sessionID`, `messageID`, `partID` |
| `session.updated` | `sessionID`, `info` (Session) |
| `session.deleted` | `sessionID`, `info` (Session) |
| `session.status` | `sessionID`, `status` (SessionStatus) |
| `permission.asked` | `id`, `sessionID`, `permission`, `patterns`, `metadata`, `always`, `tool?` |
| `permission.replied` | `sessionID`, `requestID`, `reply` (`"once"\|"always"\|"reject"`), `isAuto?` |

## The REST surface

The server mux (internal/server/server.go:89-121) exposes the mirrored routes:
`/global/health`, `/path`, `/project/current`; session CRUD
(`/session`, `/session/{id}` GET/PATCH/DELETE, `/session/status`,
`/session/{id}/message` GET/POST, `/session/{id}/abort`,
`/session/{id}/command`); `/event` (SSE); `/provider`, `/provider/auth`;
`/config` + `/global/config` (GET/PATCH); `/auth/{providerID}` (PUT/DELETE);
`/agent`, `/command`, `/permission` (GET) and
`/permission/{requestID}/reply` (POST); plus a catch-all 404. Requests are
scoped to a project directory by the **`x-yolo-directory`** header (URL-encoded
absolute path), falling back to the server work dir when absent.

## Standing deviation: the scoping header

The mirror is **verbatim except the standing baseline deviation**: the scoping
header is `x-yolo-directory`, not upstream's `x-opencode-directory`
(internal/protocol/AGENTS.md:18-22; root principle 2). Further upstream
deviations are possible only on explicit user instruction and are each logged in
`docs/superpowers/DEVIATIONS.md`. Telemetry surfaces are skipped, not deferred:
no `experimental.openTelemetry` or telemetry-identity fields appear in the
config DTOs (internal/protocol/AGENTS.md:23-25).

## Verifying the mirror

The cross-package mirror check lives in the `internal/server` wire-contract
suites (internal/protocol/AGENTS.md:37-40):

- **`golden_test.go`** — byte-compares server responses against
  `testdata/golden/*.json`. A normalizer rewrites generated ids
  (`ses|msg|prt|prj|evt|perm|cmd|req|mod` + `_` + body) to `<PREFIX><n>`, the
  test project dir to `<DIR>/<DIRNAME>`, and epoch-millisecond integers to
  `<T>`, so a response is stable across runs; `-update` regenerates the goldens
  (internal/server/golden_test.go:15-40).
- **`sse_ordering_test.go`** — asserts the ordering of SSE frames for a scripted
  turn (internal/server/sse_ordering_test.go).
- **`fake_env_e2e_test.go`** — boots the full stack and wires the kido driver
  from the `YOLO_LLM`/`YOLO_FAKE_SCRIPT` environment (the M5 env gate), so the
  e2e runs the same path as `yolo serve` with `YOLO_LLM=fake`
  (internal/server/fake_env_e2e_test.go:25-35).
- **`scope_test.go`** — the scoping matrix: an absent header falls back to the
  server work dir, and every id-scoped route 404s for a session id belonging to
  a different directory (internal/server/scope_test.go:11-30).
