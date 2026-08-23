# AGENTS.md — internal/ (core packages)

## Purpose

Core layer of the single binary: everything the TUI consumes, faithful port of
opencode v1.18.18 core.

## Ownership

The 14 packages under `internal/`: `protocol`, `server`, `session`, `llm`,
`provider`, `tool`, `permission`, `config`, `auth`, `storage`, `bus`, `glob`,
`log`, `tui`.

Cross-cutting principles (zero telemetry, verbatim pins, TUI purity) stay
owned by the root AGENTS.md; this doc owns the package map and per-boundary
contracts.

## Local Contracts

- Layering, dependency direction bottom-up: `protocol` (wire DTOs, single
  source of truth — see `protocol/AGENTS.md`) → `server` (REST + SSE) →
  `session` (agent loop) → `llm` / `provider` / `tool` / `permission` /
  `config` / `auth` / `storage` / `bus`. Supporting: `glob`, `log`.
- Package map:
  - `protocol` — wire DTOs only (agent, command, config, errors, event, id,
    message, part, provider, session)
  - `server` — REST handlers, SSE stream, `contract_test.go` (mirror check),
    `testdata/`, `testutil/` (blackbox harness = the TUI test escape hatch)
  - `session` — `engine.go` agent loop, lifecycle, `policy.go`, `prompt/`
    (14 byte-verbatim pinned prompt files)
  - `llm` — openai + anthropic drivers, `fake/` scripted driver (`YOLO_LLM=fake`),
    `testdata/`
  - `provider` — provider catalog: `zen.go`, `kido.go`, `seams.go`
  - `tool` — bash/edit/glob/grep/read/write/todowrite; `desc/*.txt`
    byte-verbatim pinned descriptions. Truncated bash runs store the FULL
    output at `<data>/tool-output/tool_<id>` and the model-visible text
    carries the upstream marker (bash.go `WriteFullOutput`); startup
    sweeps it (`CleanOutputDir`, 7-day retention)
  - `permission` — engine (upstream port) + `builtins.go` + `service.go`
  - `config` — `yolo.jsonc` load/PATCH (JSONC)
  - `auth` — key resolution: env → auth.json → config
  - `storage` — SQLite DAOs + migrations (`modernc.org/sqlite`, pure Go, no cgo)
  - `bus` — event bus
  - `glob`, `log` — path glob, slog-based leveled file logger (rotating `<dataDir>/log/yolo.log`, `YOLO_LOG_LEVEL`, opt-in `YOLO_PRINT_LOGS=1` stderr mirror; local-only, zero telemetry)
- Zero telemetry (root principle 1): no OTEL/OTLP code, no telemetry-identity
  field anywhere in these packages; the config schema omits
  `experimental.openTelemetry` by design.
- Pinned text (root principle 3): `session/prompt/*.txt` + `tool/desc/*.txt`
  are sha256-pinned by tests — never rewrap or reword.

## Work Guidance

- New code: invoke `golang-naming` + `golang-code-style` (always as a pair);
  llm paths test against the fake driver; storage uses parameterized queries
  (golang-database); errors wrapped with `%w` (golang-error-handling).
- embed quirk (root "Commands & verification"): `import _ "embed"` pattern —
  see `internal/tool/read.go`.
- Pinned dep set (root "Project"): no new module without an explicit user
  call.

## Verification

- Root CI gate at module root: `go vet ./... && go test ./...` plus clean
  `gofmt -l .`; the v0.1.2 commit gate adds `golangci-lint run ./...`
  (root "Commands & verification").
- Unit tests never hit the network (live paths env-gated; e2e user-run only).

## Child DOX Index

- `protocol/AGENTS.md` — wire contract ownership + the one intentional deviation
- `tui/AGENTS.md` — pure-client TUI: import purity (enforced), teatest/lipgloss mechanics
