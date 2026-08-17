# Yolo — Progress & Status (session checkpoint)

**Updated:** 2026-08-17 (T10–T14 done, executing M3 — active: Task 15)

## Where we are

Plan approved by user ("LGTM"). Executing inline on branch `plan`, one task at a time, strict 5-step TDD per plan, commit per task. **M0, M1, M2 COMPLETE.** M3 in progress: **Tasks 10–14 DONE** (glob+permission, tool framework + truncation + read, write + edit, glob + grep tools, bash persistent shell + todowrite); active is **Task 15: internal/session — prompt builder (embeds, family, env, instructions)**.

## Resume instructions (next session)

1. Repo: `/home/kido/network/projects/yolo` (branch `plan`). Plan: `docs/superpowers/plans/2026-08-17-yolo-go-port.md` (6020 lines; read the task's slice before executing it). Spec: `docs/superpowers/specs/2026-08-17-yolo-go-port-design.md`.
2. Continue at the Active task below. Per task: Step 1 failing test → Step 2 confirm FAIL → Step 3 minimal impl → Step 4 `go vet ./... && go test ./...` PASS → Step 5 commit with the plan's message.
3. LSP diagnostics for `cmd/yolo/main.go` re `loadStore`/`store` are STALE — the file builds and all tests pass.
4. Zen catalog CDN blocks python-urllib (403); fetch with curl + browser UA.
5. Golang skills: 15 project skills in `.agents/skills/` (samber/cc-skills-golang, hashes in `skills-lock.json`) — invoke the relevant one(s) per task: `golang-naming`/`golang-code-style` for new code, `golang-error-handling` for error flow, `golang-testing` for tests, `golang-concurrency` for concurrent code, `golang-safety` for defensive review, `golang-data-structures`/`golang-performance`/`golang-benchmark` for hot paths, `golang-database` (storage pkg), `golang-security`, `golang-troubleshooting` (bugs), `golang-design-patterns` (interfaces/DI), `golang-cli` (cmd/yolo), `golang-refactoring`.

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
| M3 Task 10: internal/glob matcher + internal/permission (Evaluate findLast, Hidden, DoomLoopDue, build/plan/yolo matrices, ask/reply Service with park/persist/cascade) | `e4f23cd` |
| M3 Task 11: internal/tool — framework (Limits/Env/Output/Tool/Registry/Visible/SchemaFor), Truncate (upstream tail() port, UTF-8-boundary cut), read tool (file/dir/binary/miss-suggest, desc sha256-pinned) | `7130344` |
| M3 Task 12: internal/tool — write (MkdirAll, meta {added,removed} via LCS line diff in write.go) + edit (exact-match replacer, per-file sync.Map mutex per upstream Semaphore, upstream error strings verbatim, empty-oldString→create path kept); desc/write.txt + desc/edit.txt byte-verbatim (hash-verified) | `3ad74dc` |
 | M3 Task 13: internal/tool — glob (WalkDir, hidden entries skipped, sorted, limit 100, missing/`File` path → error `glob path must be a directory: {search}`) + grep (RE2, hidden-skipped walk per Step-3 deviation note, >10MB + NUL-binary skips, include via `glob.Match` on rel, limit 100 with pinned "Found N matches (more matches available)" + truncation note, file root → its dir, missing root → "No files found"); desc/glob.txt + desc/grep.txt byte-verbatim (hash-verified). In-plan contradictions resolved per last-stated call: grep skips dotfiles (Step-3 deviation note overrides "dotfiles searched"); glob missing dir → explicit error (spec parenthetical overrides "empty result") | `cc29f28` |
 | M3 Task 14: internal/tool — persistent-session `bash` (shell.go: one `bash --norc --noprofile` per session, Setpgid, marker `echo __YOLO_END_{n}_$?_$(pwd | base64 -w0)` + reader `^__YOLO_END_{n}_(\d+)_(\S*)$`, exit code + cwd from marker, single output pipe with stdout+stderr on its write end, SIGKILL process group on timeout/abort + respawn, 10MB in-memory guard) + todowrite (replace-whole-list persist, `N todos` title = non-completed count, 2-space-indented JSON text, `Meta{todos}`, status/priority validation); storage migration v2 `todo` table + `SaveTodos`/`GetTodos` + `protocol.Todo`; `Env` gains `Storage *storage.DB` + `SessionID string`; `Registry()` = 7. desc/bash.txt (rendered from upstream v1 template, workdir section dropped, /tmp/yolo pinned) + desc/todowrite.txt byte-verbatim (hash-verified). See deviations 15–16 (marker colon; stderr-on-read-end) | `ae0ff27` |

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

**Task 15 (M4): `internal/session` — prompt builder (embeds, family, env, instructions).** Create `internal/session/prompt.go`, `prompt_test.go`, and `internal/session/prompt/*.txt` (14 files — the 13 from upstream `session/prompt/` minus `plan-reminder-anthropic.txt`, **plus** `title.txt` copied from `agent/prompt/`). Copy step (byte-verbatim, never hand-edit — sha256-pin tests guard it): for each of the 13 `cp /tmp/opencode-upstream/packages/opencode/src/session/prompt/<f> internal/session/prompt/<f>`; for title `cp /tmp/opencode-upstream/packages/opencode/src/agent/prompt/title.txt internal/session/prompt/title.txt`; then `sha256sum` each and fill the 14 `promptPins` map constants in the test (the `…`/`/*compute*/` placeholders in the plan test must be replaced — the pin test `t.Skip`s on unfilled pins). **NOTE:** this embeds `//go:embed prompt/*.txt` (an FS target, which works on this host — the scalar `import _ "embed"` workaround from deviation 12 is NOT needed here). Produces `BuildSystemPrompt(dir, model, apiID, providerID) []string` (order: [familyPrompt, envBlock, instructions...]), `FamilyPrompt(apiID, providerID)` (pins the 8-way family selection table), `EnvBlock(dir, apiID, providerID)`, `PlanReminders(history, currentAgent)`. Consumes `provider.Model`. Family selection + env-block format + 14-file list + plan-reminder behavior are pinned VERBATIM in the M4 header (plan lines 3946–3976) — read that slice before implementing. Also per PLAN FIX (line 3976): `message` table gains `agent TEXT NOT NULL DEFAULT 'build'` + `MessageRow.Agent` (Task 5 DDL amended, still migration v1) — fold into this task's storage touch if the plan places it here. Upstream refs re-clone pinned v1.18.18 into `/tmp/opencode-upstream` if wiped.

## Plan deviations logged so far (established pattern: tests define contract)

1. T2: plan test bugs (SessionWireShape model; MessageRoles key; id alphabet) — fixed in tests.
2. T3: `LoadAt` 4-arg → `LoadAt(globalDir, startDir)`; M5 global config file `global.jsonc` → `yolo.jsonc`.
3. T5: `sess.Model` is `*ModelRef` (compare `.ProviderID`/`.ID`); CallID not persisted in schema v1 (noted).
4. T6: `TestCancelStopsDelivery` needed `ok` check on closed-channel receive.
5. T7: `Part` gained `Args json.RawMessage`; test uses local `stream()` helper instead of plan's `.must`/`drainFinal` placeholders; midstream test drains from the same PartStream.
6. T8: plan's `common_test.go` was missing the `encoding/json` import — added.
7. T9: plan's fixture generator wrote the bare `opencode` entry, but both the test and parser expect the `{"opencode": ...}` wrapper — fixture regenerated with the wrapper (matches what the real fetch caches). `TestKidoParsesLlamacpp` passes `srv.URL+"/v1"` to keep its `/v1/models` path assertion meaningful. Zen auth test isolates via `XDG_DATA_HOME` + `OPENCODE_API_KEY=""`.
8. T10: two rule vocabularies — `protocol.Rule.Action` uses config/wire "allow"|"deny"|"ask" (new `RuleAllow/RuleDeny/RuleAsk` consts), while the `Decision` constants are "allow"|"denied"|"ask" (`Allow/Deny/AskAction`). Evaluations compare against the Rule consts. Auto-answered pendings (via `always` coverage) are stored `"once"` so only explicit `always` replies mint always rules. Plan's `service_test` had a dead `done <- Allow` goroutine before the real `Reply`; cleaned to a single `Reply` call.
9. T10: `Service` gained `SetConfigRules`/`SetDataDir` so `DecisionFor` can evaluate builtins + config rules + DB always rules without changing `New(db,bus)`; `DecisionPre==""||AskAction` triggers a fresh `decisionFor`; the catch-all `*`-permission rule only applies to known core actions on the decision path (unknown actions default to ask, matching upstream no-rule behavior).
10. T11: two plan test bugs — `TestReadByteCap` used 3000×9-byte lines (27000B total) which can never trip the 50KB cap under any accounting (changed to 3000×40-byte lines = 120000B so it cuts ~line 1305, before the 2000-line limit, as the test promises); `TestReadMissingFileSuggests` used siblings `app.go`/missing `ap.go` which never substring-match either direction under the pinned algorithm (changed to sibling `myapp.go`/missing `app.go`).
11. T11: test I authored (plan left it to the implementer) `TestTruncateSingleLineUTF8Cut` — verified against upstream `shell.ts tail()`: a single over-long line keeps its **last** MaxBytes bytes advanced to a UTF-8 boundary (40000×U+00E9 → keep 51200B = 25600 runes; 17100×U+65E5 mid-rune cut → 51198B = 17066 runes). Initial expectation of 14400 was the *removed* rune count, not kept.
12. T11 (environmental): on this host, a **plain** `import "embed"` + a scalar (`string`/`[]byte`) `//go:embed` fails typecheck with `embed imported and not used` under **both** installed toolchains (go1.26.5 via mise, and go1.25.10 via GOTOOLCHAIN) — an upstream-unused-import exemption is missing from the bundled types2 for scalar embed targets here (`embed.FS` targets work). Workaround used in `read.go`: `import _ "embed"` (still embeds content verbatim; runtime-verified on both toolchains). Flag: revisit if a future toolchain restores the plain-import path; the pinned sha256 pin in `TestDescPinned` guards against silent content drift regardless. `write.go`/`edit.go` reuse the same blank-import workaround.
13. T12: plan test bug — `TestEditPatternsAndExternal` expected `Patterns(raw)` to return the env.Dir-relative path (`sub/f.txt`), but the committed Task 11 interface (tool.go:72-73) takes raw args only: paths are emitted **as given**; the engine (Task 17) resolves/relativizes against Env.Dir (read does the same). Test fixed to assert as-given. Corollary design: edit `Patterns`/`External` parse only `filePath` (via `editFilePath`), matching the plan test's `{"filePath": f}`-only args and upstream (patterns built from the path alone); full old/new validation happens in `Run`.
14. T13: plan test bugs — (a) `TestGlobTool` expected 3 lines for `**/*.go`, but only 2 files can match (`.git/skip.go` is excluded by the hidden-skip rule the same test asserts; `a/z.md` is not `.go`) → fixed to 2. (b) `TestGrepTool` carried a dead `lines` variable (Go compile error: declared and not used) + a stray leading space in `joined :=` → dropped the dead loop, kept `joined`. Implementation note: `Patterns`/`External` for glob/grep emit the as-given `path` (or `["*"]` when omitted) per the raw-args-only interface; the engine resolves in Task 17.
15. T14 (implementation bug, found at Step 4): the plan pin (line 2739) is internally inconsistent — the marker `echo __YOLO_END_{n}_:$?_$(pwd | base64 -w0)` emits a line with a **colon** before `$?` (e.g. `__YOLO_END_0_:0_<b64>`), but the reader regex `^__YOLO_END_{n}_(\d+)_(\S*)$` has **no** colon, so it never matches → `Exec` hung until the 120s timeout (reader *did* receive the line; diagnosed via temp stderr logging + child-fd inspection). Resolved by keeping the plan's regex as authoritative and dropping the stray colon in the emitted marker → `echo __YOLO_END_{n}_$?_$(pwd | base64 -w0)`, so the emitted `__YOLO_END_0_0_<b64>` matches exactly. (Upstream v1.18.18 does NOT use this persistent-shell+marker design — it runs one-shot `shell -c` with `stdin:"ignore"`; the marker protocol is a plan-pinned v1 design, so we reconciled the plan's own two pins rather than upstream.)
16. T14 (implementation bug, found at Step 4): first stderr-merge used `cmd.Stderr = stdoutPipe.(*os.File)` where that file was the pipe's **read** end (O_RDONLY) — the child's writes to stderr failed silently (EBADF) and vanished, so `echo err >&2` produced no output (only the stdout marker arrived). Fixed by creating one output pipe via `os.Pipe()` and assigning the **write** end to both `cmd.Stdout` and `cmd.Stderr` (child dups it to fd 1 and 2), parent closes its write-end copy after `Start` so the reader sees EOF on child death; the read end is stored on `shellProc.stdout` and closed in `reapProc`.

## Open items

- [ ] Execute Tasks 14–30 inline (per task commit messages in the plan)
- [ ] Task 30 tag `v0.1.0` ONLY with explicit user go-ahead (versioning: 0.1.0 = current scope; out-of-scope features → 0.2.0, …)
- [ ] On-demand live e2e vs `ai.kido.ws` (scripts/e2e-live.sh) — user-run, never CI
- [ ] Flags for user at handoff: plan-matrix third `edit` rule moved to engine (T10 note); CallID not persisted (T5); host toolchain scalar `//go:embed` typecheck gap → `import _ "embed"` workaround (T11 note 12)
