# AGENTS.md — internal/ (core packages)

## Purpose

Core layer of the single binary: everything the TUI consumes. Built as a
faithful port of opencode v1.18.18 core; since v0.4.0 opencode is a reference,
not a contract (root principle 2; spec
`docs/superpowers/specs/2026-08-24-v0.4.0-design.md`).

## Ownership

The 14 packages under `internal/`: `protocol`, `server`, `session`, `llm`,
`provider`, `tool`, `permission`, `config`, `auth`, `storage`, `bus`, `glob`,
`log`, `tui`.

Cross-cutting principles (zero telemetry, pinned text, TUI purity) stay
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
  - `server` — REST handlers, SSE stream, wire-contract suites
    (`golden_test.go` / `sse_ordering_test.go` / `fake_env_e2e_test.go` /
    `scope_test.go` — the upstream mirror check), `testdata/`, `testutil/`
    (blackbox harness = the TUI test escape hatch)
  - `session` — `engine.go` agent loop, lifecycle, `policy.go`, `prompt/`
    (14 sha256-pinned prompt files)
  - `llm` — openai + anthropic drivers, `fake/` scripted driver (`YOLO_LLM=fake`),
    `testdata/`
  - `provider` — provider catalog: `zen.go`, `kido.go`, `seams.go`
  - `tool` — bash/edit/glob/grep/read/write/todowrite; `desc/*.txt`
    sha256-pinned descriptions. Truncated bash runs store the FULL
    output at `<data>/tool-output/tool_<id>` and the model-visible text
    carries the upstream marker (bash.go `WriteFullOutput`); startup
    sweeps it (`CleanOutputDir`, 7-day retention)
  - `permission` — engine (upstream port) + `builtins.go` + `service.go`
  - `config` — profile-aware global config (`~/.config/yolo/<profile_id>/`
    load/PATCH, JSONC) + profile lifecycle/selection (`profile.go`: id gen,
    active marker, `list`/`add`/`use`/`edit`/`remove`/`copy`, `ProcessProfile`
    precedence flag > `YOLO_PROFILE` > marker; a corrupt profile config
    never fails `List`/name matching — id fallback, blank metadata) —
    deviation 121
  - `auth` — key resolution: env → auth.json → config
  - `storage` — SQLite DAOs + migrations (`modernc.org/sqlite`, pure Go, no cgo)
  - `bus` — event bus
  - `glob`, `log` — path glob, slog-based leveled file logger (rotating `<dataDir>/log/yolo.log`, `YOLO_LOG_LEVEL`, opt-in `YOLO_PRINT_LOGS=1` stderr mirror; local-only, zero telemetry)
- Zero telemetry (root principle 1): no OTEL/OTLP code, no telemetry-identity
  field anywhere in these packages; the config schema omits
  `experimental.openTelemetry` by design.
- Pinned text (root principle 3): `session/prompt/*.txt` + `tool/desc/*.txt`
  are sha256-pinned by tests — the pins record current intended content, not
  an upstream lock; an intentional change re-baselines the pin in the same
  commit.

## Work Guidance

- Golang skills (full table in root AGENTS.md "Golang skills") — invoke the
  relevant one(s) per task: `golang-naming` + `golang-code-style` (always
  paired) for new code; `golang-error-handling` (wrapping `%w`, logging);
  `golang-testing` (table-driven, fake driver, goleak); `golang-concurrency`
  (engine turn loop, bus, SSE pump); `golang-database` (storage DAOs —
  parameterized queries only); `golang-security` (auth, config, injection);
  `golang-troubleshooting` (bugs/crashes/deadlocks — root cause first);
  `golang-safety` (defensive review); `golang-design-patterns` (interfaces,
  DI, lifecycle); `golang-data-structures` / `golang-performance` /
  `golang-benchmark` (hot paths); `golang-refactoring` (large restructures).
  llm paths always test against the fake driver. `golang-cli` covers `cmd/`
  (root-owned subtree).
- embed quirk (root "Commands & verification"): `import _ "embed"` pattern —
  see `internal/tool/read.go`.
- Dependency policy (root "Project"): allowlist + agent-proposable — a new
  module needs a dep proposal (extensive-web-search-backed evidence) and
  explicit user approval before it lands.

## Verification

- Root CI gate at module root: `go vet ./... && go test ./...` plus clean
  `gofmt -l .`; the v0.1.2 commit gate adds `golangci-lint run ./...`
  (root "Commands & verification").
- Unit tests never hit the network (live paths env-gated; e2e user-run only).

## Child DOX Index

- `protocol/AGENTS.md` — wire contract ownership + the standing baseline deviation
- `tui/AGENTS.md` — pure-client TUI: import purity (enforced), teatest/lipgloss mechanics
