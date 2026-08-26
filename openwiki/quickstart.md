---
type: concept
title: Quickstart
description: "How to build, run, test, and drive yolo: the single Go binary and its subcommands (default TUI, serve, auth, profile, version), the key environment variables (YOLO_LLM=fake, YOLO_PROFILE, YOLO_LOG_LEVEL, KIDO_API_KEY), and the CI gate (go vet && go test, gofmt)."
tags: [quickstart, cli, build, run, test, env, justfile]
verified:
  - by: openwiki/0.4.0
    at: 2026-08-26T18:04:14.871Z
sources:
  - id: openwiki-source-d418c74eeb4c988387e6dc32
    resource: repo://cmd/yolo/main.go
  - id: openwiki-source-e9096526cb7c8e8729cb3c9b
    resource: repo://internal/config/profile.go
  - id: openwiki-source-b3b67094b286e55952c7bbfa
    resource: repo://internal/log/log.go
  - id: openwiki-source-c59fe4336a371ea1052a01dd
    resource: repo://justfile
  - id: openwiki-source-23775c3de52f3ab95a13cb8b
    resource: repo://README.md
generated: {by: "opencode", at: "2026-08-26T18:04:14.871Z"}
---

# Quickstart

**yolo** is a single Go binary: it starts the **core HTTP server (REST + SSE)
in-process** and, by default, runs the **bubbletea TUI**, which talks to it
**only** through the wire contract (`internal/protocol` via
`internal/tui/client`). `yolo serve` runs the server alone; `yolo auth` manages
credentials. It contains **zero telemetry**. Prerequisite: **Go ≥ 1.25**
(pure-Go build, no cgo — `modernc.org/sqlite` is embedded).

## Build

```sh
just build                    # version-stamped binary (git describe, -ldflags -X main.version)
go build -o yolo ./cmd/yolo   # plain build (no version stamp)
```

## Run

```sh
yolo                 # TUI (in-process server on an ephemeral localhost port)
yolo <sessionID>     # TUI, resume an existing session (not found -> exit 2)
yolo --dir DIR       # start in a specific project directory
yolo --profile NAME  # run under a profile (id or name)
yolo serve           # core server only (default http://127.0.0.1:4096)
yolo serve --addr 127.0.0.1:0
yolo auth list       # list stored credentials
yolo auth add <provider> [key]   # add (key omitted = prompt on stdin)
yolo auth remove <provider>
yolo profile list    # list profiles (* = active)
yolo version         # print version (same as: yolo -v / --version)
yolo help
```

The subcommands, as dispatched in `cmd/yolo/main.go`:

| command | action |
|---|---|
| *(none)* | start the TUI (in-process server on an ephemeral port + the bubbletea program) |
| `serve` | `serveCmd` — core server only (`--addr`, default `127.0.0.1:4096`) |
| `auth` | `authCmd` — `list` \| `add <provider> [key]` \| `remove <provider>` |
| `profile` | `profileCmd` — `list` \| `add [name] [-d DESC]` \| `use ID` \| `edit ID [-n NAME] [-d DESC]` \| `remove ID` \| `copy SRC NAME [-d DESC]` |
| `-v` / `--version` / `version` | print the version block |
| `help` / `-h` / `--help` | usage |

In **TUI mode** the server binds `127.0.0.1:0` (ephemeral port); a resume
session id is validated up front (a missing session prints
`session not found: <id>` and exits 2). In **serve mode** SIGINT/SIGTERM
triggers a graceful drain — in-flight turns are cancelled, the listener shuts
down within 5 s (a second signal force-kills) — and the process exits 0.

## Configuration & profiles

Config is **JSONC**, merged deterministically (innermost wins; `instructions`
concatenates), with `{env:NAME}` substitution:

- **Global**: `~/.config/yolo/<profile_id>/yolo.jsonc` (file precedence in the
  profile dir: `config.json` → `yolo.json` → `yolo.jsonc`).
- **Project**: `yolo.jsonc` (or `yolo.json`) in the working directory and each
  ancestor up to `/`, plus `<workDir>/.yolo/yolo.jsonc` as the innermost
  override.

Global config is partitioned into **profiles** (one dir per profile under
`~/.config/yolo/`, the active one in `~/.config/yolo/active`; a first run
creates `default`). Selection precedence: **`--profile` flag > `YOLO_PROFILE`
env > active profile > `default`**.

## Key environment variables

| variable | effect |
|---|---|
| `YOLO_LLM=fake` (+ `YOLO_FAKE_SCRIPT=path.json`) | dev mode: swap the LLM drivers for a scripted fake (one scripted turn per model request; `delay_ms` per turn) — offline, never a live endpoint |
| `YOLO_PROFILE` | select the config profile (beaten by the `--profile` flag; beats the active marker) |
| `YOLO_LOG_LEVEL` | logger level `DEBUG`/`INFO`/`WARN`/`ERROR` (invalid → INFO) |
| `YOLO_PRINT_LOGS=1` | mirror the log to stderr |
| `KIDO_API_KEY` | credential for the builtin `kido` provider (also read at request time, highest precedence) |
| `KIDO_BASE_URL` / `E2E_TIMEOUT` | live-e2e overrides (default `https://ai.kido.ws/v1` / 180 s) |

## Tests & verification

```sh
go vet ./... && go test ./...     # the CI gate — never hits the network
```

plus a clean `gofmt -l .`. Unit/integration tests never touch the network
(live paths are env-gated via `YOLO_LLM=fake`); the live e2e is **user-run,
never CI**.

**Live e2e (manual)**: `just e2e-live` (`scripts/e2e-live.sh`) builds the
binary, boots `yolo serve` from a scratch project pinned to the real `kido`
endpoint, and drives the wire contract: health check → create a `yolo`-agent
session → send "list files in /tmp" (asserts a completed tool call + a
non-empty text reply) → abort tests (idle → `aborted:false`, busy →
`aborted:true`) → SIGTERM → graceful exit 0. Requires `KIDO_API_KEY`.

## Data directory

```
~/.local/share/yolo/
  auth.json          # yolo auth credentials
  storage/yolo.db    # sessions, messages, parts, permissions, todos (SQLite)
  plans/             # plan-agent files (writable by the engine without asking)
  log/yolo.log       # logger (info/error), rotated at 5 MiB to yolo.log.1
~/.cache/yolo/
  models.json        # cached Zen model catalog (opencode provider)
```

Global config lives in `~/.config/yolo/<profile_id>/yolo.jsonc` (active profile
in `~/.config/yolo/active`). XDG: `XDG_CONFIG_HOME` is honored;
`XDG_DATA_HOME` / `XDG_CACHE_HOME` shift the data and cache roots.
