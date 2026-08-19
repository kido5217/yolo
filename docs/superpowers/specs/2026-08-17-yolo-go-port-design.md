# Yolo — Go Port of opencode (TUI + Core)

- **Date:** 2026-08-17
- **Status:** Approved design (all sections confirmed by user); reconciled in place against as-built **v0.1.0** on 2026-08-19 (deviations in `PROGRESS.md` log)
- **Upstream reference:** [anomalyco/opencode](https://github.com/anomalyco/opencode) tag **`v1.18.18`** (local clone at `/tmp/opencode-upstream`)
- **Upstream contract reference:** `packages/sdk/openapi.json` (162 paths), event schemas in `packages/schema/src/*.ts`, TUI at `packages/tui/src/`
- **Purpose (per README):** a Go rewrite of opencode to test the capabilities of **Qwen3.8-27B** running on a single RTX 5090 behind `https://ai.kido.ws/v1`.

---

## 1. Scope & key decisions

Yolo ports the **TUI + core server** of opencode v1.18.18 in Go. Confirmed decisions:

| # | Decision |
|---|---|
| 1 | TUI built with **bubbletea** (Charm **v2** line: `charm.land/bubbletea/v2`, `lipgloss/v2`, `bubbles/v2`) |
| 2 | **TUI only** — opencode's web UI, desktop, slack, console, stats, etc. are dropped |
| 3 | Built-in `opencode` (OpenCode Zen) provider kept with **paid models only** (zero-cost `-free` variants dropped) |
| 4 | New provider **`kido`** (`ai.kido.ws`) is the **default provider**; default model **`Qwen3.8-27B`**; base URL `https://ai.kido.ws/v1`; model list from `GET /v1/models` (llamacpp format) |
| 5 | Upstream reference is tag `v1.18.18` (note: tag is `v`-prefixed) |
| 6 | Project name **Yolo**, binary `yolo`, module `github.com/kido5217/yolo` |
| 7 | Architecture: **faithful client-server** — core HTTP server (REST + SSE) + TUI as a pure wire-protocol client (user chose option B) |
| 8 | Porting strategy: **contract-faithful, layered port** (user chose approach 1) — opencode's legacy REST paths/JSON shapes and legacy SSE event set are mirrored so the port is verifiable against the published OpenAPI spec |
| 9 | LLM wire protocols: **OpenAI-compatible chat-completions** + **Anthropic Messages** (user chose option B) — covers `kido` + 57 of 64 paid zen models; the 7 Google-adapter zen models are excluded from the model list until v1.x |
| 10 | Storage: **SQLite**, pure-Go driver `modernc.org/sqlite` (no cgo); Yolo defines its own clean v1 schema (no replay of opencode's 38 historical migrations) |
| 11 | Config: opencode's config **schema**, but file names **`yolo.json` / `yolo.jsonc`** (project) and `~/.config/yolo/` (global) |
| 12 | Auth: opencode-compatible **`auth.json`** + env vars + config `apiKey`, plus a `yolo auth` CLI subcommand (user chose option A) |
| 13 | Extra built-in agent: **`yolo`** — permits everything unconditionally (no permission prompts for any tool/resource). Alongside opencode's built-ins `build` (default) and `plan` (read-only) |
| 14 | Directory-scoping header renamed: **`x-yolo-directory`** (opencode uses `x-opencode-directory`; Yolo TUI is the only client, so the rename carries no interop cost). This is the single deliberate deviation from the upstream wire contract |

**Core principle — zero telemetry:** Yolo runs entirely on the end user's machine. It must contain **zero telemetry**: no usage or telemetry data is ever sent to any remote server, and there is no opt-in telemetry either. This is a permanent project principle, not a v1 scope decision. Upstream telemetry surfaces are **skipped, not deferred**:

- OTEL/OTLP exporter (`packages/core/src/observability/otlp.ts`, env-gated `OTEL_EXPORTER_OTLP_ENDPOINT`)
- OpenTelemetry spans on LLM calls (`experimental.openTelemetry` config / `experimental_telemetry` flag in `packages/opencode/src/session/llm.ts`)
- Telemetry-identity username field in upstream config
- The `experimental.openTelemetry` key is dropped from Yolo's ported config schema

`OTEL_*` environment variables are inert in yolo.

**Feature scope for v1 = "core agent loop" (scope A):** session view with streaming, prompt input, tool execution, permission prompts, model + provider + agent selection, config, auth, session persistence. Explicitly **out** of v1 (deferred to v1.x): MCP, skills, subagents (`task`), LSP, snapshots/revert, workspaces, compaction/summarize, themes engine & keymap config, command palette, @file fuzzy mentions, image/PDF attachments, OAuth flows, `/api/*` v2 routes.

---

## 2. Architecture & process model

Single Go binary `yolo` (Go ≥ 1.25; module `github.com/kido5217/yolo`):

- `yolo` (default) — starts the core server **in-process** on a local port, then launches the bubbletea TUI, which connects to it over HTTP + SSE. The TUI is a *pure client*: it never imports core packages.
- `yolo serve [--addr ADDR]` — headless server only (default `127.0.0.1:4096`; debugging / remote TUI).
- `yolo auth [list|add <provider> [key]|remove <provider>]` — credential management (stdlib `flag` + manual subcommand dispatch; no Cobra).
- `yolo <sessionID>` — resume a session (TUI opens session route directly).

**Key invariant:** TUI ↔ server communicates *only* via the wire protocol (Section 3). The TUI is a drop-in substitute for opencode's TUI.

**Built-in agents (three):**

| Agent | Mode | Permissions (resolved ruleset) |
|---|---|---|
| `build` (default) | primary | opencode v1.18.18 semantics: blanket allow; `read` on `*.env`/`*.env.*` → **ask** (`*.env.example` allow); `question` allow; `plan_enter` allow; `plan_exit` deny; `doom_loop` ask; external directory → ask (whitelist exceptions) — full table in §4.5 |
| `plan` | primary | build base, but `edit`/`write` → **deny** except plan-note files (`plans/*.md` under the data dir, plus a worktree-relative allow rule for it at session start); `plan_exit` allow |
| `yolo` (new) | primary | `{action: "*", resource: "*", effect: "allow"}` — everything permitted, unconditionally: no prompt for bash, edits, `.env` reads, external dirs, anything |

Agent selection: config `agent` field (default) + TUI agent dialog / `/agents` command. Custom agents from config: v1 merges their `permission` rules into the built-in matrix; full custom system-prompt support is v1.x.

**Package layout** (domain-named, one level deep under `internal/`):

```
cmd/yolo/            # entrypoint + subcommands (tui default, serve, auth)
internal/protocol/   # DTOs + SSE event types — single source of truth for the wire contract
internal/server/     # HTTP mux, handlers, SSE fanout, x-yolo-directory scoping
internal/bus/        # in-process pub/sub event bus
internal/session/    # agent loop: turn execution, system prompt, title gen, retry, overflow
internal/llm/        # Driver interface + openai-chat + anthropic drivers (hand-rolled SSE)
internal/provider/   # kido + opencode(zen) + config-defined providers, model catalog, auth state
internal/tool/       # registry + read, write, edit, glob, grep, bash, todowrite
internal/glob/       # pattern matcher (opencode glob semantics, used by permission)
internal/permission/ # rulesets, evaluation, built-in agent rules
internal/config/     # discovery, merge, JSONC, env substitution
internal/auth/       # auth.json, env vars, key resolution
internal/storage/    # SQLite open/migrations/DAOs
internal/log/        # rotating file logger (<data>/log/yolo.log)
internal/tui/        # bubbletea v2 app + client/ + store/ + home/, session/, prompt/, dialog components
```

**Runtime dependencies (pinned):**

| Module | Version | Why |
|---|---|---|
| `charm.land/bubbletea/v2` | v2.0.8 | TUI framework (stable v2 line; requires Go ≥ 1.25) |
| `charm.land/lipgloss/v2` | v2.0.6 | styling/layout |
| `charm.land/bubbles/v2` | v2.1.1 | key, textinput, viewport (spinner + pickers hand-rolled) |
| `modernc.org/sqlite` | v1.56.0 | pure-Go SQLite (no cgo) |
| `tidwall/jsonc` | v0.3.3 | JSONC comment stripping → stdlib `encoding/json` |

Everything else is **stdlib**: `net/http` (Go 1.22+ `ServeMux` method+pattern routing — no router framework), SSE via `http.Flusher`, hand-rolled LLM HTTP/SSE clients (no LLM SDK), `flag` for the CLI. Dev-only: `github.com/charmbracelet/x/exp/teatest/v2` for TUI e2e tests (same pinned pseudo-version; served from github.com, not charm.land — see deviation 46).

---

## 3. Wire contract (REST + SSE)

REST = opencode's **legacy endpoint paths and JSON shapes** (the set the v1.18.18 TUI consumes; shapes derived from `packages/sdk/openapi.json`):

| Endpoint | Purpose |
|---|---|
| `GET /global/health` | health probe |
| `GET /path` | working-directory info |
| `GET /project/current` | project identity (id, name, directory) |
| `GET /agent` | agents `build`, `plan`, `yolo` (+ config-defined) |
| `GET /command` | slash-command definitions (minimal: `/help`, `/new`, `/model`, `/agents`, `/exit`) |
| `GET /event` | SSE stream (below) |
| `GET /session/status` | active session busy/idle snapshot for the footer |
| `GET,POST /session` | list (scoped by directory header) / create session |
| `GET,PATCH,DELETE /session/{id}` | get / update (title, model, agent, time) / delete |
| `GET,POST /session/{id}/message` | list messages / send user prompt |
| `POST /session/{id}/abort` | cancel in-flight turn |
| `POST /session/{id}/command` | server-side slash-command execution (minimal set) |
| `GET /provider` | providers + models + auth state (drives the model dialog) |
| `GET /provider/auth` | auth method metadata |
| `GET,PATCH /config` | project `yolo.jsonc` |
| `GET,PATCH /global/config` | global config (`~/.config/yolo/`) |
| `PUT,DELETE /auth/{providerID}` | store / remove credentials |
| `GET /permission` | pending permission requests |
| `POST /permission/{requestID}/reply` | reply: `once` \| `always` \| `reject` |

Skipped endpoint families (server returns 404; the TUI never calls them in v1): `/api/*` v2 routes, `/tui/*` push-control, `/mcp/*`, `/workspace*`, `/experimental/*`, `/sync/*`, `/question/*`, `/revert*`, `/share*`, `/skill*`, `/lsp*`, `/vcs*`, `/file*`, `/find*`, `/formatter*`, `/project/git/*`, `/global/upgrade`, `POST /log`.

**SSE** on `GET /event`: frames `data: {JSON}\n\n`, envelope `{"type": "...", "properties": {...}}` — opencode's **legacy event set**, subset:

- `message.updated`, `message.part.updated`, `message.part.delta`, `message.removed`, `message.part.removed`
  (part kinds: `text`, `reasoning`, `tool` with state `running` | `completed` | `error`)
- `session.updated`, `session.deleted`, `session.status`
- `permission.asked`, `permission.replied`

  (as-built v0.1.0: `message.removed` / `message.part.removed` are defined and consumed by the TUI store but never emitted; user message/part events are published **before** the busy `session.status` — deviation 41)

**Directory scoping:** header **`x-yolo-directory`** carrying the URL-encoded absolute project directory (opencode uses `x-opencode-directory` — the one deliberate deviation; TUI sends it at connect time, server defaults to the process CWD when absent).

**Error envelope:** `{"error": {"message": "...", "data"?: ...}}` with proper statuses: 400 schema/bad input, 404 unknown resource, 409 session busy, 500 internal.

`internal/protocol` holds all DTO/event types once; server marshals, TUI unmarshals.

---

## 4. Core

### 4.1 LLM layer (`internal/llm`)

Two hand-rolled protocol drivers over stdlib HTTP, one interface:

```go
type Driver interface {
    // normalized stream: text delta | reasoning delta | tool call | finish | usage
    Stream(ctx context.Context, req MessageRequest) (PartStream, error)
}
```

1. **OpenAI chat-completions** — `POST {base}/chat/completions` with `stream: true`; SSE `data:` frames; `choices[].delta.{content, reasoning_content, tool_calls[]}`. Drives `kido` and the opencode-zen models with openai/openai-compatible adapters (42 zen models). `reasoning_content`/`reasoning` → reasoning part.
2. **Anthropic Messages** — `POST {base}/messages` with `stream: true`; SSE event frames (`content_block_start/delta/stop`, `message_delta`); `text_delta`/`thinking_delta`/`tool_use` blocks. Drives the 15 anthropic-adapter zen models.

Per-turn usage (input/output/cache tokens) is captured for cost display.

### 4.2 Providers (`internal/provider`)

- **`kido`** (default provider). Base URL `https://ai.kido.ws/v1`. Key **optional** (`KIDO_API_KEY` env → `auth.json` → config). Model list fetched from `GET https://ai.kido.ws/v1/models` (llamacpp shape: `data[].meta.n_ctx` → context limit, etc.); static metadata fallback for `Qwen3.8-27B` (context 262144, `tool_call` + `reasoning` enabled) so startup never blocks on the network. Default model: `kido/Qwen3.8-27B`.
- **`opencode`** (OpenCode Zen). Base URL `https://opencode.ai/zen/v1`. Key **required** (`OPENCODE_API_KEY` env → `auth.json` → config). Catalog fetched from `https://models.opencode.ai/api.json`, cached at `~/.cache/yolo/models.json` (consult cache file; refetch + atomic rewrite when mtime older than 5 min — opencode's `models-dev` cache behavior). **Filters:** paid models only (`cost.input > 0`; drops ~40 `-free` variants) and Google-adapter models excluded (7) ⇒ **57 models** with per-model adapter → driver mapping.
- Config may override built-ins or add providers (`provider.<id>`: `baseURL`, `apiKey`, `options`, custom `models` map) with opencode's schema.
- `GET /provider` exposes per-provider auth state (loaded / key-required) for the TUI dialog.

### 4.3 Session engine (`internal/session`) — the agent loop

Triggered by `POST /session/{id}/message`:

```
1. persist user message (text parts)
2. build request: system prompt + history (SQLite: messages + tool results) + tool schemas
3. loop:
   - stream LLM → append text/reasoning parts live (SSE events + SQLite)
   - on tool_calls → for each: permission check → execute → append tool part (state: running→completed|error)
   - repeat until the turn produces no tool_calls (or abort / overflow / max-turns)
4. persist final message; fire session.status idle
```

- **System prompt** = agent system text (per agent: opencode's `agent.ts` texts) + **model-family prompt file** (opencode's `session/prompt/*.txt` ported verbatim as Go embeds — `default.txt` for Qwen3.8-27B; `anthropic.txt`, `gpt.txt`, `gemini.txt`, `codex.txt`, `kimi.txt`, `meta.txt`, `copilot-gpt-5.txt`, `beast.txt`, `trinity.txt` selected by model family; `plan.txt`/plan-mode additions for the plan agent) + environment block (model name/id, working directory, worktree, git yes/no, platform, date) + instruction files (project `AGENTS.md` walking up from CWD + config `instructions[]`).
- **Title generation**: after the first user message, an async side-call (opencode's `title.txt` prompt) using the session's model; result updates `session.title` via `session.updated`.
- **Abort**: `POST .../abort` cancels context → in-flight HTTP stream and tool subprocess killed; partial parts persisted with error flag.
- **Retry**: transient LLM errors (429/5xx/network) → exponential backoff + jitter, bounded attempts, surfaced as events.
- **Overflow**: if tokens exceed the model context limit → hard-stop the turn with a clear error (auto-compaction is v1.x).
- **Max-turns guard** (50 tool steps) prevents runaway loops.
- **Concurrency**: one active turn per session (concurrent send → 409); multiple sessions run concurrently (goroutines; SQLite WAL).

### 4.4 Tools (`internal/tool`) — 7 tools (model-visible names)

Model-facing descriptions + JSON schemas ported verbatim from opencode's `tool/*` definitions (prompt fidelity matters for the model).

| Tool | Permission action | Behavior (ported from opencode) |
|---|---|---|
| `read` | `read` | line-numbered content, offset/limit, directory listing, "did you mean" on miss |
| `write` | `edit` | create/overwrite file |
| `edit` | `edit` | exact string replace; `oldString`/`newString`/`replaceAll`; unique-match error semantics |
| `glob` | `glob` | pattern → files sorted by mtime desc |
| `grep` | `grep` | ripgrep when available, Go-regex fallback; include-pattern filtering |
| `bash` | `bash` | persistent shell per session, per-command timeout, cwd = worktree; stdout+stderr capture, truncation policy |
| `todowrite` | `todowrite` | JSON todos `{content, status, priority}` |

Tool output truncation matches opencode (size cap + truncation marker).

### 4.5 Permissions (`internal/permission`)

Faithful port of opencode's engine (`packages/opencode/src/permission/index.ts` + built-in agent rules in `agent/agent.ts`), verified against v1.18.18 source:

- Rule entries `{action, pattern (resource glob), effect}`. Evaluation = flatten the effective ruleset, **last matching rule wins** (`findLast`); **no rule matches → `ask`**.
- Effective order (last wins): `[…agent base rules, …user config `permission` rules, …interactive "always" approvals]` — user config overrides agent base; "always" approvals override everything. Within an agent base, broad rules come first, narrow rules later (e.g. `read *` before `read *.env`).
- A request may carry several patterns: any pattern `deny` → whole call denied; else any `ask` → ask; else allow.
- **`build` base = opencode v1.18.18 defaults:** `*` allow; `doom_loop` ask; `external_directory` ask (whitelisted dirs allow); `question` allow; `plan_enter` allow; `plan_exit` deny; `read`: `*` allow, `*.env` + `*.env.*` ask, `*.env.example` allow. **`plan`** = build base + `plan_exit` allow, `edit` deny except plan-note files, plans dir whitelisted under `external_directory`. **`yolo`** = solely `{* → allow}` (no prompts for anything).
- **Doom loop:** before executing a tool call, if the assistant message's last 3 tool parts are the same tool with deep-equal inputs, a `doom_loop` ask fires first (pattern = tool name); outcome flows through the normal ask path.
- `ask` → engine pauses the turn, stores the pending request, emits `permission.asked` (`{id, sessionID, permission (action), patterns[], metadata{tool, input, …}, always[] (tool-suggested pattern suggestions), tool}`). TUI dialog: **1 = once · 2 = always · 3 = reject** → `POST /permission/{id}/reply`:
  - `once` → the call proceeds.
  - `always` → proceeds; each suggested `always` pattern is added as `{action, pattern, effect: allow}` (persisted with the session, Section 6.3); other pending requests in the same session whose patterns are then fully covered are auto-answered `always`.
  - `reject` → the call fails (tool part `error`: "permission rejected", user feedback if given is fed to the model); **all other pending requests in the same session are cascade-rejected** (opencode behavior).
- In every case the agent loop continues — the model sees the error and adapts.
- Tools whose permission action carries a wildcard-deny rule (`pattern "*"`) are **hidden from the model** entirely (excluded from its tool list).

---

## 5. TUI (bubbletea v2)

Pure HTTP/SSE client (imports only `internal/protocol` + its own client). Stack: `charm.land/bubbletea/v2` + `lipgloss/v2` + `bubbles/v2`. Architecture: **tree of models** — root app model routes messages; home, session, prompt, pickers, permission dialog, footer are child components/models.

**Root model state:**

- `client` — small HTTP client for the Section-3 endpoints (`internal/tui/client`)
- `store` — REST state snapshot (providers, config, sessions, current session + messages/parts, permissions) hydrated on route entry, then updated **only** via SSE (reader goroutine → `tea.Msg`, batched)
- `route` — `home` | `session` (mirrors opencode's two routes)
- `dialogs` — modal stack: model picker, agent picker, permission, help, quit-confirm
- SSE reconnect: exponential backoff 1 s → 30 s (opencode's TUI policy); on reconnect, state re-hydrated via REST

**Home route:** session list (title, model, relative time; updated-desc; last 50) + "new session" entry; keyboard-driven.

**Session route:**

- **Message viewport** (`bubbles/viewport`): user messages; assistant text streaming (appended on `message.part.delta`); reasoning parts as dimmed indented collapsible blocks (toggle); tool parts inline rows (`✓ read src/main.go (123 lines)`, `▶ bash: ls -la …`) with expandable full I/O; error tool parts in red.
- **Prompt input** (`bubbles/textinput`): multiline; `/` opens slash-command menu (`/new`, `/model`, `/agents`, `/help`, `/exit`). @-mentions are plain text (no fuzzy picker in v1).
- **Permission dialog:** inline overlay above the prompt on `permission.asked`; keys 1/2/3 → allow once / always / reject.
- **Model dialog** (two-pane picker, hand-rolled — key bindings + lipgloss; no `bubbles/list` in v1): providers (with auth state) → models (default marker, context + cost); "use for this session" vs "set default" (PATCH `/config`).
- **Agent dialog / `/agents`:** pick `build` / `plan` / `yolo` (PATCH session + config).
- **Footer:** model `provider/id`, active agent, tokens in/out, cost, connection indicator, busy spinner (driven by `session.status`).
- **Toasts:** error flash area (minimal; opencode's MCL/LSP/status-banner lines dropped).

**Keymap (v1; surfaced in `/help`):**

| Key | Action |
|---|---|
| enter | send prompt |
| esc | abort turn (busy) / close dialog |
| ctrl+c | quit (confirm) |
| ctrl+p | model dialog |
| ctrl+a | agent dialog |
| `/` | command menu |
| pgup/pgdn | viewport scroll |
| 1/2/3 | permission reply |
| `alt+e` / `alt+t` | expand tool part / toggle reasoning |

`\` + enter inserts a newline in the prompt (rendered in `/help` as the footer note "pgup/pgdn scroll · \+enter newline", not as a table row).

**Non-goals (TUI v1.x):** themes/keymap engine, command palette, mouse selection/scrolling, stashes, timelines, workspaces UI, plugin slots.

**TUI testing:** `teatest` scripted runs against a server with a fake LLM driver (recorded/fixture streams).

---

## 6. Config, auth, storage

### 6.1 Config (`internal/config`)

opencode's schema, Yolo file names. Discovery + merge order (mirrors opencode):

1. **Global** `~/.config/yolo/`: merge all present of `config.json` → `yolo.json` → `yolo.jsonc`
2. **Project**: `yolo.json` / `yolo.jsonc` found **walking up from CWD** to filesystem root
3. **Project dir** `<project>/.yolo/`: `yolo.json` / `yolo.jsonc` + instruction files (`AGENTS.md`)

Merge semantics: opencode's deep merge (objects merge; `instructions[]` concatenates + dedupes); later layer wins. Env substitution in strings (`"{env:NAME}"` / bare env names, e.g. API keys). JSONC via `tidwall/jsonc` then stdlib `encoding/json`. Unknown fields ignored (forward compat).

**Schema subset (fields with v1 behavior):**

```jsonc
{
  "model": "kido/Qwen3.8-27B",   // default model (overridable per session)
  "agent": "build",              // default agent: build | plan | yolo | custom id
  "provider": {                  // override built-ins or define new providers
    "kido":     { "baseURL": "...", "apiKey": "...", "options": {}, "models": {} },
    "opencode": { "apiKey": "..." }
  },
  "permission": { "bash": "ask", "edit": "allow", "read": { "*.env": "ask" } },
  "instructions": ["docs/STYLE.md"],   // extra instruction files (files only in v1)
  "theme": {}                       // accepted, ignored in v1
}
```

Custom `agent.<id>` entries: `permission` rules merge into the built-in matrix in v1; custom system prompts deferred to v1.x.

### 6.2 Auth (`internal/auth`)

- File `~/.local/share/yolo/auth.json` (0600), opencode format: `{ "<providerID>": { "type": "api", "key": "...", "metadata": {} } }` (v1: `api` type only).
- Key resolution per provider: **env** (`OPENCODE_API_KEY`, `KIDO_API_KEY`) → **auth.json** → **config** `provider.<id>.apiKey` / `options.apiKey` (post env-substitution). `kido` needs no key (optional).
- CLI: `yolo auth list` · `yolo auth add <provider> [key]` (prompts when key omitted) · `yolo auth remove <provider>`.
- `GET /provider` carries per-provider auth state for the dialog.

### 6.3 Storage (`internal/storage`)

**SQLite** (`modernc.org/sqlite`), single file `~/.local/share/yolo/storage/yolo.db`, WAL mode, `busy_timeout=5000`. One DB for all directories; sessions carry `project_dir`. Yolo's own v1 schema (no upstream migration replay):

```sql
CREATE TABLE session (
  id TEXT PRIMARY KEY,            -- opencode-style "ses_..." ids
  project_dir TEXT NOT NULL,      -- x-yolo-directory value
  title TEXT NOT NULL DEFAULT '',
  model TEXT NOT NULL,            -- "provider/model"
  agent TEXT NOT NULL DEFAULT 'build',
  time_created INTEGER NOT NULL,
  time_updated INTEGER NOT NULL
);
CREATE TABLE message (
  id TEXT PRIMARY KEY,            -- "msg_..."
  session_id TEXT NOT NULL REFERENCES session(id) ON DELETE CASCADE,
  role TEXT NOT NULL,             -- user | assistant
  time_created INTEGER NOT NULL,
  time_completed INTEGER
);
CREATE TABLE part (
  id TEXT PRIMARY KEY,            -- "prt_..."
  message_id TEXT NOT NULL REFERENCES message(id) ON DELETE CASCADE,
  session_id TEXT NOT NULL,
  type TEXT NOT NULL,             -- text | reasoning | tool
  tool TEXT,                      -- for type=tool
  state_json TEXT NOT NULL,       -- kind-specific payload; tool state/status
  time_created INTEGER NOT NULL
);
CREATE TABLE permission (
  request_id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  action TEXT NOT NULL,
  resource TEXT NOT NULL,         -- requested pattern
  response TEXT,                  -- once | always | reject
  always_json TEXT,               -- JSON array: patterns approved when response='always'
  time_created INTEGER NOT NULL
);
CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);  -- schema_version, ...
```

Migrations: versioned SQL list tracked in `meta.schema_version`, applied at boot. IDs follow opencode's formats (`ses_*`, `msg_*`, `prt_*`). Concurrency: single process, per-session turn mutex, WAL for the rest. `DELETE /session/{id}` cascades. On session resume, `permission` rows with `response='always'` are re-registered as `{action, pattern, allow}` rules (opencode keeps these in instance memory; Yolo persists them so they survive restarts).

**On-disk layout:**

```
~/.config/yolo/             global config (config.json | yolo.json | yolo.jsonc)
~/.local/share/yolo/        auth.json · storage/yolo.db · plans/*.md · log/yolo.log
~/.cache/yolo/              models.json (zen catalog cache)
<project>/yolo.jsonc        project config
<project>/.yolo/            project-dir config + instructions
```

---

## 7. Error handling & testing

**Error handling by layer:**

1. **HTTP** — opencode's envelope; 400/404/409/500 as specified in Section 3; handler-level recover → log + 500.
2. **LLM** — transient (429/5xx/network) → bounded exponential backoff + jitter; non-transient (401/400/model-not-found) → fail fast. After a 200 stream has started, failures ride as an error part + `session.status` idle; TUI shows toast + red part. 401 on `opencode` provider → actionable message (`yolo auth add opencode` / `OPENCODE_API_KEY`).
3. **Tools** — failure → part `state=error` with message; agent loop continues (model adapts). Bash: non-zero exit keeps stdout+stderr in output; timeout/abort → error. `plan` agent edits → permission-denied error.
4. **SSE** — server: one goroutine per client, 1024-slot buffer; overflow closes the client; TUI reconnects (1 s→30 s backoff) and re-hydrates via REST — no state loss.
5. **Process** — SIGINT/SIGTERM → abort active turns → drain SSE → close DB → exit 0. Logging to `~/.local/share/yolo/log/yolo.log` (5 MiB size rotation → `yolo.log.1`, single generation, overwrite).

**Testing (stdlib `testing` + `httptest`; goldens checked in):**

| Layer | Coverage |
|---|---|
| unit: config | merge order, env substitution, JSONC, unknown fields, array concat |
| unit: permission | glob matching, precedence, build/plan/yolo matrix |
| unit: llm drivers | recorded SSE fixtures (both protocols): deltas split across frames, reasoning, tool calls, usage, mid-stream error/abort |
| unit: provider | catalog filters (free dropped; 7 google dropped; 57 remain; adapter→driver map), llamacpp `/v1/models` → model meta |
| unit: tools | temp-dir fixtures per tool; `replaceAll`/unique-match miss; bash timeout; truncation |
| unit: storage | CRUD, cascades, migration bump |
| integration: engine | fake LLM driver: multi-tool turn, permission ask→reply, abort mid-tool, 429 retry, max-turn guard, overflow → assert event sequence **and** persisted rows |
| contract: server | every Section-3 endpoint vs opencode-OpenAPI-derived JSON goldens; SSE frame ordering; `x-yolo-directory` scoping |
| TUI: teatest | scripted: home → new session → streamed text + tool + permission keypresses → assert; model dialog; abort |
| e2e smoke (on-demand, not CI-gated) | real binary vs live `ai.kido.ws`: one prompt, a completed tool call (read/glob/grep/bash) + non-empty assistant text (`scripts/e2e-live.sh`) |

CI gates: `go vet` + `golangci-lint` (errcheck, staticcheck, govet) + `go test ./...`.

---

## 8. Milestones & acceptance

| # | Deliverable | Done when |
|---|---|---|
| M0 | Skeleton: go.mod, layout, `serve` stub, health | `go build` + `GET /global/health` |
| M1 | protocol + config + auth + storage | unit suites green |
| M2 | llm drivers + providers | SSE fixture tests green (both protocols) + live `kido` model fetch |
| M3 | tools + permissions | tool fixtures + permission matrix green |
| M4 | session engine (fake LLM) | integration: multi-tool turn, abort, retry, events |
| M5 | server contract + SSE bus | contract tests + SSE ordering green |
| M6 | TUI core: home, session view, streaming, permission dialog, prompt | teatest: full scripted turn |
| M7 | TUI model/agent dialogs, footer, toasts, `/help` | teatest suites green |
| M8 | Polish: real-LLM e2e vs `ai.kido.ws`, README, tag | live dogfood: real coding task on Qwen3.8-27B |

**v1 success criteria:** `yolo` boots to home, starts a session on `kido/Qwen3.8-27B` by default, streams, and runs all 7 tools with correct per-agent permission behavior (`yolo` agent = zero prompts); sessions persist across restarts and appear in the home list; zen paid models work through both wire protocols; CI green.

**Non-goals (v1.x):** MCP, skills, subagents/`task`, LSP, snapshots/revert, workspaces, compaction, themes/keymaps engine, @file fuzzy mentions, image/PDF attachments, OAuth flows, `/api/*` v2 routes, web/desktop UIs. (Telemetry is not a non-goal — it is a permanent exclusion, see the zero-telemetry core principle in Section 1.)
