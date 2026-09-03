# AGENTS.md — internal/protocol (wire contract)

## Purpose

Single source of truth for yolo's wire contract: the legacy REST
paths/JSON shapes and legacy SSE event set, mirrored from opencode v1.18.18
so the port stays verifiable against its OpenAPI contract.

## Ownership

`internal/protocol/*.go` — agent, command, config, errors, event, id,
message, part, provider, session DTOs + `protocol_test.go`.

## Local Contracts

- Wire shapes are defined only here: `server` emits them, `internal/tui`
  consumes them; no other package defines or remarshal wire structs.
- The mirror is verbatim except the standing baseline deviation (root
  principle 2): scoping header is `x-yolo-directory` (upstream:
  `x-opencode-directory`). Further upstream deviations are possible only on
  explicit user instruction (root principle 2), each logged in
  `docs/superpowers/DEVIATIONS.md`.
- Telemetry surfaces skipped, not deferred (root principle 1): no
  `experimental.openTelemetry` or telemetry-identity fields in the config
  DTOs.

## Work Guidance

- Golang skills (full table in root AGENTS.md "Golang skills") — new DTOs
  follow `golang-naming` + `golang-code-style` (always paired).
- Keep the package wire-level: DTOs, ids, event shapes — no business logic,
  no I/O.
- Wire changes land here first; `server` (emitting) and `tui` (consuming)
  follow in the same change.

## Verification

- Root gate: `go vet ./... && go test ./...`. Cross-package mirror check
  lives in the `internal/server` wire-contract suites (`golden_test.go`,
  `sse_ordering_test.go`, `fake_env_e2e_test.go`, `scope_test.go`).

## Child DOX Index

- None.
