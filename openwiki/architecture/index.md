# Files

- [Session Engine (Agent Loop)](agent-loop.md) - The internal/session engine that drives one user message through model/tool rounds: turn lifecycle, retry and overflow semantics, part bookkeeping, permission-gated tool execution, and Abort/Close/Shutdown invariants.
- [Event Bus and Delivery](event-flow.md) - End-to-end event flow in yolo: in-process pub/sub (internal/bus), SSE delivery by the server (internal/server/sse.go), the TUI's SSE client and event pump, re-hydration on drop, and the local-only rotating file logger (internal/log).
- [Architecture Overview](overview.md) - yolo's single-binary design: the core HTTP server (REST + SSE) runs in-process, and the bubbletea TUI is a pure client over the wire contract. Covers package layering, the opencode-v1.18.18 reference-not-contract stance, and the dependency allowlist.
- [Wire Contract (protocol)](wire-contract.md) - internal/protocol is the single source of truth for yolo's wire contract: the legacy REST paths, JSON shapes, and SSE event set mirrored from opencode v1.18.18, plus the id contract and the test suites that verify the mirror.
