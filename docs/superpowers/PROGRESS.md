# Yolo — Progress & Status (session checkpoint)

**Updated:** 2026-08-17 (M0–M2 done, executing M3)

## Where we are

Plan approved by user ("LGTM"). Executing inline on branch `plan`, one task at a time, strict 5-step TDD per plan, commit per task. **M0, M1, M2 COMPLETE.** Currently starting **M3 (Task 10: internal/glob + internal/permission)**.

## Resume instructions (next session)

1. Repo: `/home/kido/network/projects/yolo` (branch `plan`). Plan: `docs/superpowers/plans/2026-08-17-yolo-go-port.md` (6020 lines; read the task's slice before executing it). Spec: `docs/superpowers/specs/2026-08-17-yolo-go-port-design.md`.
2. Continue at the Active task below. Per task: Step 1 failing test → Step 2 confirm FAIL → Step 3 minimal impl → Step 4 `go vet ./... && go test ./...` PASS → Step 5 commit with the plan's message.
3. LSP diagnostics for `cmd/yolo/main.go` re `loadStore`/`store` are STALE — the file builds and all tests pass.
4. Zen catalog CDN blocks python-urllib (403); fetch with curl + browser UA.

## Completed work this session

| Item | Where |
|---|---|
| Requirements + 7 design sections approved by user | spec (§1–§8), commits `f54991c`, `cec9a8f` |
| Spec written, source-verified, committed | `docs/superpowers/specs/2026-08-17-yolo-go-port-design.md` |
| Implementation plan: 30 tasks over M0–M8 | `docs/superpowers/plans/2026-08-17-yolo-go-port.md` |
| M0 Task 1: module bootstrap, cmd/yolo dispatch, server /global/health | `e2d2565` |
| M1 Task 2: protocol DTOs (id/session/message/part/event/provider/agent/config) | `742c652` |
| M1 Task 3: config discovery, JSONC, deep merge, env substitution | `c1a0b5c` |
| M1 Task 4: auth.json store, key resolution, `yolo auth` CLI | `6562ed2` |
| M1 Task 5: storage (SQLite schema v1, DAOs, aggregates, ProtocolToPart) | `ec6ea45` |
| M1 Task 6: in-process event bus | `de97df8` |
| M2 Task 7: llm Driver interface + OpenAI chat-completions SSE driver | `03fdb36` |
| M2 Task 8: Anthropic Messages SSE driver (+ shared test helpers) | `765293c` |
| M2 Task 9: provider registry — kido live/fallback, zen catalog filter+cache (frozen fixture; 91/64/57/42+15/7 gate verified) | `62edde5` |

## Plan resolutions & flags to raise at handoff (severity)

1. **important** — teatest: spec's `charm.land/x/exp/teatest` is the v1 module and cannot build a v2 TUI; plan pins `charm.land/x/exp/teatest/v2` v2.0.0-20260816001655-68d539dca504 (dev-only).
2. **important** — spec DDL lacked per-message cost/tokens; plan adds `message.cost REAL` + `message.tokens TEXT` (Task 5) and session-level aggregates at read time (Task 19).
3. **important** — spec DDL lacked todo persistence; plan adds migration v2 `todo` table + DAOs + `protocol.Todo` (Task 14).
4. minor — `title.txt` actually lives in `agent/prompt/` (spec said `session/prompt/`); Task 15 embeds 14 prompt files (13 session + title).
5. minor — config `agents` map vs spec's `agent` string ambiguity resolved to `agents` (Task 3/20); custom agents v1 = permission merge + description stub.
6. minor — SSE frames include `id` beyond the spec envelope example (type+properties).
7. minor — JSONC comments are not preserved when config PATCH rewrites the file.
8. minor — v1 keymap: pgup/pgdn scroll viewport, `\`+enter newline (spec's ↑/↓ viewport scroll replaced; noted in /help text).

## Key verified facts (so they don't get re-litigated)

- Permission engine = port of `packages/opencode/src/permission/index.ts` + matrices in `agent/agent.ts` (build/plan/yolo verbatim in Task 10).
- Doom loop = sliding 3-identical window; wildcard-deny hides tool iff last matching rule is `*` deny; write+edit both map to permission `edit`.
- Pinned deps: `charm.land/bubbletea/v2` v2.0.8, `charm.land/lipgloss/v2` v2.0.6, `charm.land/bubbles/v2` v2.1.1, `modernc.org/sqlite` v1.56.0, `tidwall/jsonc` v0.3.3; dev-only `teatest/v2`.
- Module `github.com/kido5217/yolo`, Go ≥ 1.25 (installed 1.26.5).
- Single deliberate wire deviation: `x-yolo-directory` header.
- Test gating: unit tests never hit network; `YOLO_LLM=fake` + `YOLO_FAKE_SCRIPT` selects the scripted fake driver (wired in Task 19, e2e in Task 21); zen fixture gate = 57 models (42 openai + 15 anthropic, 7 google excluded).
- TUI import rule: non-test files under `internal/tui/` import only `internal/protocol` + `internal/tui/*`; `_test.go` may use `internal/server/testutil` (escape hatch). Enforced by Task 29.

## Active

**Task 10 (M3): `internal/glob` + `internal/permission`** — matcher, evaluation, matrices, ask/reply service. Plan slice starts ~line 2752. Key plan notes inside the slice: `internal/glob` is a new tiny package (imports only strings/regexp); permission matrix order is significant; `yolo = {"*","*","allow"}` only; PLAN FIX: the third plan `edit` rule (worktree-relative) is added by the **engine** at session start, not in `LoadBuiltins`. DDL gap #2 (todo table, migration v2) is OWNED by Task 14, not this one.

## Plan deviations logged so far (established pattern: tests define contract)

1. T2: plan test bugs (SessionWireShape model; MessageRoles key; id alphabet) — fixed in tests.
2. T3: `LoadAt` 4-arg → `LoadAt(globalDir, startDir)`; M5 global config file `global.jsonc` → `yolo.jsonc`.
3. T5: `sess.Model` is `*ModelRef` (compare `.ProviderID`/`.ID`); CallID not persisted in schema v1 (noted).
4. T6: `TestCancelStopsDelivery` needed `ok` check on closed-channel receive.
5. T7: `Part` gained `Args json.RawMessage`; test uses local `stream()` helper instead of plan's `.must`/`drainFinal` placeholders; midstream test drains from the same PartStream.
6. T8: plan's `common_test.go` was missing the `encoding/json` import — added.
7. T9: plan's fixture generator wrote the bare `opencode` entry, but both the test and parser expect the `{"opencode": ...}` wrapper — fixture regenerated with the wrapper (matches what the real fetch caches). `TestKidoParsesLlamacpp` passes `srv.URL+"/v1"` to keep its `/v1/models` path assertion meaningful. Zen auth test isolates via `XDG_DATA_HOME` + `OPENCODE_API_KEY=""`.

## Open items

- [ ] Execute Tasks 10–30 inline (per task commit messages in the plan)
- [ ] Task 30 tag `v1.0.0` ONLY with explicit user go-ahead
- [ ] On-demand live e2e vs `ai.kido.ws` (scripts/e2e-live.sh) — user-run, never CI
- [ ] Flags for user at handoff: plan-matrix third `edit` rule moved to engine (T10 note); CallID not persisted (T5)
