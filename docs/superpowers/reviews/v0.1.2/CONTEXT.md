# v0.1.2 Review Context — invariants (do NOT flag these)

Repo: /home/kido/network/projects/yolo · module github.com/kido5217/yolo · Go ≥ 1.25 (installed 1.26.5).
Upstream: faithful port of anomalyco/opencode v1.18.18 (read-only reference clone: /tmp/opencode-upstream).
Gate (must pass before every commit): `go vet ./... && go test ./...` · `gofmt -l .` prints nothing (Go 1.26) · `golangci-lint run ./...` clean.

## Hard invariants (flagging any of these is a misfire)
1. Zero telemetry is a permanent project principle. OTEL/OTLP exporter, OTel spans on LLM
   calls, and the telemetry-identity field are intentionally ABSENT; `OTEL_*` env vars are
   inert; the ported config schema deliberately omits `experimental.openTelemetry`.
2. Single deliberate wire deviation: scoping header `x-yolo-directory` (upstream:
   `x-opencode-directory`). All other REST paths / JSON shapes and the legacy SSE event
   set mirror upstream verbatim.
3. Verbatim pins, sha256-guarded by tests: the 14 session prompt files
   (`internal/session/prompt/*.txt`) and every tool `desc/*.txt` file. Never reword.
   Golden fixtures (`internal/server/testdata/golden/`) regenerate only via the contract
   suite's `-update` flag.
4. TUI import rule: non-test files under `internal/tui/` import only `internal/protocol`
   + `internal/tui/*` (+ stdlib + charm deps). `_test.go` may import
   `internal/server/testutil` (deliberate escape hatch). Guarded by
   `internal/tui/imports_test.go`.
5. Host toolchain quirk: plain `import "embed"` + a scalar `//go:embed` fails typecheck
   on both installed toolchains; the `import _ "embed"` workaround (internal/tool/
   read.go, write.go, edit.go) is intentional.
6. Tests never hit the network. `YOLO_LLM=fake` (+ `YOLO_FAKE_SCRIPT`) selects the
   scripted fake driver (`internal/llm/fake`). `scripts/e2e-live.sh` is user-run only,
   never CI.
7. Provider catalog is built ONCE at server startup from the startup-dir config; per-turn
   config does not change BaseURL. `KIDO_BASE_URL` is honored by the e2e script (it
   writes config), not by yolo.

## Faithful-behavior pins (upstream-mirrored; not findings)
- SSE ordering: user message + part events are published BEFORE the busy
  `session.status` (upstream prompt.ts parity; deviation 41).
- Doom loop: sliding window of 3 identical calls; wildcard `*`-deny hides a tool iff the
  last matching rule is a `*` deny; `write`+`edit` both map to permission `edit`.
- `/quit` is the canonical exit slash command; `/exit` is an accepted alias on every
  surface (deviation 66).
- TUI keymap: pgup/pgdn scroll, `\`+enter newline. JSONC comments are NOT preserved on
  config PATCH rewrite (v1 behavior pin).
- `POST /session/{id}/abort` blocks until engine idle (≤2s, 10ms poll) before 200
  (deviation 33).
- Tool Part ID == model call ID; CallID is not persisted elsewhere in schema v1
  (deviations 20/23).
- Engine continues a round when finish == "tool_calls" OR the round emitted any tool
  part (robustness rule, deviation 21e); max 50 rounds.

## Meta-rule
Flag the port's INTERNAL correctness and idiomaticity: races, leaks, nil-safety, error
flow, resource lifecycle, test quality, naming, style. Do not re-derive upstream v1.18.18
design choices. If a behavior diff from upstream looks suspect, compare
/tmp/opencode-upstream (read-only) before flagging, or mark the finding
"needs-upstream-check".
