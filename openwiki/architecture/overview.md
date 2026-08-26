---
type: concept
title: Architecture Overview
description: "yolo's single-binary design: the core HTTP server (REST + SSE) runs in-process, and the bubbletea TUI is a pure client over the wire contract. Covers package layering, the opencode-v1.18.18 reference-not-contract stance, and the dependency allowlist."
tags: [architecture, single-binary, layering, wire-contract, dependency-policy]
verified:
  - by: openwiki/0.4.0
    at: 2026-08-26T18:04:14.871Z
sources:
  - id: openwiki-source-41612f8ed7b59c998588fda2
    resource: repo://cmd/yolo/deps.go
  - id: openwiki-source-d418c74eeb4c988387e6dc32
    resource: repo://cmd/yolo/main.go
  - id: openwiki-source-7bd911fdd3026b7b031a01e3
    resource: repo://go.mod
  - id: openwiki-source-23f7f075092612c9b403937d
    resource: repo://internal/AGENTS.md
  - id: openwiki-source-35b6e28a1e5217ac97361a86
    resource: repo://internal/protocol/AGENTS.md
  - id: openwiki-source-7d4ac4eaf17b216fdc5fd692
    resource: repo://internal/server/server.go
  - id: openwiki-source-99b37a6823820f4cb0c51a48
    resource: repo://internal/tui/imports_test.go
generated: {by: "opencode", at: "2026-08-26T18:04:14.871Z"}
---

# Architecture Overview

yolo is a single Go binary that starts the **core HTTP server in-process** and,
by default, runs the **bubbletea v2 TUI**, which talks to that server *only*
through the wire contract (`internal/protocol` via `internal/tui/client`). There
is no separate server process and no shared in-memory object graph between the
TUI and the core: the boundary is HTTP + SSE.

## The two runtime modes

`cmd/yolo` (cmd/yolo/main.go:60-88) dispatches:

- **default / `yolo [<sessionID>] [--dir DIR]`** — `tuiCmd`
  (cmd/yolo/main.go:129-256): builds the full dependency stack
  (`buildDeps`), starts the core server on an **ephemeral localhost port**
  (`127.0.0.1:0`), optionally validates a resume `sessionID` over REST, builds
  the theme engine, and runs the bubbletea program. On exit it drains the
  engine (stops active turns), shuts down the listener, and closes the logger
  within a 5 s budget; a second signal force-kills instead of waiting out the
  budget (cmd/yolo/main.go:268-293).
- **`yolo serve [--addr ADDR]`** — `serveCmd`: runs the server alone, no TUI.

Supporting subcommands: `yolo auth` (credential management) and `yolo profile`
(list/add/use/edit/remove/copy config profiles).

## Dependency assembly

`buildDeps` (cmd/yolo/deps.go:28-136) assembles the core stack for a working
directory and profile selection:

1. Resolves XDG dirs (home, data, cache) — a broken home is a startup error, not
   a per-request failure.
2. Opens the logger and runs the startup sweep for truncated-bash full outputs
   (`tool.CleanOutputDir`, best-effort, must not block boot).
3. Opens the SQLite DB at `<data>/storage/yolo.db`.
4. Creates the event bus, the permission service, and the profile-pinned config
   loader (profile selection precedence: flag > `YOLO_PROFILE` env > active
   marker; the loader is pinned to the *resolved* id so it re-resolves
   deterministically).
5. Selects the provider registry: `YOLO_LLM=fake` (+ `YOLO_FAKE_SCRIPT`) installs
   a static catalog + the scripted fake driver so the suite never hits the
   network; any other env builds the live registry.
6. Builds the session engine (required deps: DB, bus, provider, permission,
   tools) and returns the `server.Deps` plus a `closeDB` cleanup func.

## Package layering

Dependency direction is **bottom-up** (internal/AGENTS.md:22-25):

```
protocol  (wire DTOs — single source of truth for the wire contract)
   ^
server    (REST handlers + SSE stream; emits the protocol shapes)
   ^
session   (the agent loop; the engine)
   ^
llm / provider / tool / permission / config / auth / storage / bus
   (+ glob, log as supporting packages)
```

- `internal/protocol` defines the wire shapes **only**: `server` emits them,
  `internal/tui` consumes them, and no other package defines or remarshal wire
  structs. Wire changes land in `protocol` first, with `server` (emitting) and
  `tui` (consuming) following in the same change
  (internal/protocol/AGENTS.md:16-34).
- The TUI is a **pure client** (internal/tui/AGENTS.md). Non-test files under
  `internal/tui/` import only `internal/protocol` and `internal/tui/*` from
  within the module (plus stdlib/charm). This import direction is **enforced** by
  `TestImportsDirection` (internal/tui/imports_test.go:20-), a parser-based test
  that walks every `.go` file. `_test.go` files may additionally use the escape
  hatches (`internal/server/testutil` for real-stack blackbox suites, and
  `internal/llm{,/fake}` for scripted fake turns).

## Reference, not contract

yolo began as a faithful port of **opencode v1.18.18** (core + TUI only;
web/desktop/slack/console dropped). Its purpose has since shifted to testing
various LLM harnesses and frameworks. opencode v1.18.18 remains a **reference
for how things should be done, not a binding contract** (spec
`docs/superpowers/specs/2026-08-24-v0.4.0-design.md`). Consequences:

- The legacy REST paths/JSON shapes and legacy SSE event set remain **mirrored**
  so the baseline stays verifiable against opencode's OpenAPI contract (checked
  by the `internal/server` wire-contract suites), but yolo may deviate on
  explicit user instruction.
- The standing baseline deviation: the scoping header is **`x-yolo-directory`**
  (upstream: `x-opencode-directory`), resolved by `Server.scope`
  (internal/server/server.go:228-242).
- Every deviation is logged in `docs/superpowers/DEVIATIONS.md` with severity.

## Zero telemetry

The project's first core principle: **zero telemetry** (root AGENTS.md, principle
1). yolo runs on the end user's machine and sends no usage data to any remote
server. Upstream telemetry surfaces are **skipped, not deferred**: no OTEL/OTLP
exporter, no OpenTelemetry spans on LLM calls, no telemetry-identity field, and
the config schema omits `experimental.openTelemetry` by design
(internal/AGENTS.md:54-56; internal/protocol/AGENTS.md:23-25). The logger is
local-only (see the event-flow page).

## Dependency policy

Runtime dependencies are **pinned at exact versions** behind an allowlist
(go.mod:5-15). The direct runtime set is the charm stack
(`bubbletea/v2`, `lipgloss/v2`, `bubbles/v2`, `glamour/v2`),
`modernc.org/sqlite` (pure Go, no cgo), `tidwall/jsonc`,
`github.com/aymanbagabas/go-udiff`, and `charmbracelet/x/term`. Anything outside
the allowlist requires an agent **dependency proposal** (with live-verified
evidence) and explicit user approval before it lands (root AGENTS.md, "Project").

## CI gate

Every task ends with the module-root gate green: `go vet ./... && go test
./...`, plus a clean `gofmt -l .`. Unit tests never hit the network (live paths
are env-gated; the e2e smoke is user-run, never CI).
