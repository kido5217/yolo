# OpenWiki instructions for yolo

## What this project is

yolo is a Go TUI + core-server harness in the lineage of opencode v1.18.18
(module `github.com/kido5217/yolo`, binary `yolo`, Go ≥ 1.25, pure Go, no
cgo). Since v0.4.0 its purpose is to test LLM harnesses and frameworks:
opencode is a reference for how things should be done, not a binding contract.
The runtime dependency set is a strict allowlist, and the codebase contains
**zero telemetry**.

## Priorities

Emphasize durable, agent-useful knowledge an engineer would otherwise
re-derive from source:

- Single-binary architecture: the core HTTP server (REST + SSE) runs
  in-process, and the TUI is a pure client over the wire contract only
  (`internal/protocol`).
- Package layering and dependency direction (see `internal/AGENTS.md`):
  `protocol` → `server` → `session` → `llm` / `provider` / `tool` /
  `permission` / `config` / `auth` / `storage` / `bus`, with `glob` / `log`
  supporting.
- The wire contract, SSE event flow, and the TUI/client boundary.
- Build/run/test surface: subcommands, `YOLO_LLM=fake` (+ `YOLO_FAKE_SCRIPT`),
  config profiles, env vars, and the CI gate (`go vet ./... && go test ./...`,
  clean `gofmt -l .`).
- Storage (SQLite via `modernc.org/sqlite`), auth precedence (env →
  auth.json → config), config (JSONC, profiles, `{env:NAME}` substitution).
- TUI internals: bubbletea v2 structure, theme tokens, transcript rendering,
  dialogs (huh), fuzzy filtering.

## What to skip

- Task state and process memory: beads (`bd`) and `docs/superpowers/`
  (`PROGRESS.md`, `DEVIATIONS.md`, `DEFERRED.md`, plans, specs) are the
  process archive, not wiki content.
- Pinned prompt/tool-description text and their sha256 pins — tests own them
  (root AGENTS.md principle 3).
- Transient implementation history, commit-by-commit changes, or roadmap.
- README boilerplate; prefer grounded, code-derived detail.

## Sources of truth

- Source code and tests are authoritative — never `docs/` prose or the wiki
  itself.
- `AGENTS.md` (root, `internal/AGENTS.md`, and package-level) for contracts,
  layering, and principles.
- Upstream opencode v1.18.18 (`/tmp/opencode-upstream`) as reference, not
  contract; yolo's deliberate deviations live in
  `docs/superpowers/DEVIATIONS.md`.

## Conventions

- Match the project's vocabulary: wire contract, TUI purity, zero telemetry,
  reference-not-contract, allowlist.
- Ground every material statement in versioned evidence (Claims with
  `repo://` resources) so updates can detect drift.
- Keep pages current with source; when source moves, update the affected
  pages rather than adding caveats.
