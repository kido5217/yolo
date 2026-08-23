# yolo

A faithful Go port of [opencode](https://github.com/anomalyco/opencode) **v1.18.18** — TUI + core server only (web/desktop/slack/console dropped). Single binary: it starts the core HTTP server (REST + SSE) in-process, then runs a bubbletea TUI which talks to it **only** through the wire contract (`internal/protocol`), so the REST/SSE surface stays verifiable against opencode's OpenAPI contract. One deliberate wire deviation: the scoping header is `x-yolo-directory` (upstream: `x-opencode-directory`).

yolo contains **zero telemetry**: it runs on your machine, sends nothing anywhere, and the ported config schema has no telemetry surface.

## Prerequisites

- Go ≥ 1.25 (pure-Go build, no cgo; `modernc.org/sqlite` is embedded)

## Build

```sh
just build                    # version-stamped binary (git describe)
go build -o yolo ./cmd/yolo   # plain build (no version stamp)
```

## Run

```sh
yolo                 # TUI (in-process server on an ephemeral localhost port)
yolo <sessionID>     # TUI, resume an existing session
yolo --dir DIR       # start in a specific project directory
yolo serve           # core server only (default http://127.0.0.1:4096)
yolo serve --addr 127.0.0.1:0
yolo auth list       # list stored credentials
yolo auth add <provider> [key]   # add (key omitted = prompt on stdin)
yolo auth remove <provider>
yolo version
yolo help
```

SIGINT/SIGTERM (TUI or `serve`) triggers a graceful drain — in-flight turns are cancelled, the listener shuts down within 5 s — and the process exits 0.

## Configuration

JSONC, merged deterministically (innermost wins; `instructions` concatenates). `{env:NAME}` and whole-string env names are substituted.

- **Global**: `~/.config/yolo/yolo.jsonc` (file precedence in `~/.config/yolo/`: `config.json` → `yolo.json` → `yolo.jsonc`)
- **Project**: `yolo.jsonc` (or `yolo.json`) in the working directory and each ancestor up to `/`, innermost last, plus `<workDir>/.yolo/yolo.jsonc` as the innermost override

### Fields (minimal)

| Key | Type | Meaning |
|---|---|---|
| `model` | string | `"providerID/modelID"`, e.g. `"kido/Qwen3.8-27B"` (default) |
| `agent` | string | default agent for new sessions (`build`) |
| `provider` | map | per provider: `baseURL`, `apiKey` (or `"{env:NAME}"`), `options`, `models` |
| `permission` | map | per permission (`bash`, `edit`, `read`, `glob`, `grep`, `todowrite`): `"allow" \| "ask" \| "deny"` or a pattern→action map |
| `instructions` | string[] | extra system instructions (concatenated across layers) |
| `theme` | map | reserved (no theme engine in v1) |
| `tool_output` | object | `max_lines` / `max_bytes` truncation of tool output |
| `agents` | map | custom agents: `description` + `permission` merge (v1) |

Example project `yolo.jsonc`:

```jsonc
{
  "model": "kido/Qwen3.8-27B",
  "permission": { "bash": "ask", "read": "allow" },
  "provider": { "kido": { "baseURL": "https://ai.kido.ws/v1", "apiKey": "{env:KIDO_API_KEY}" } }
}
```

## Auth

v1 credential sources, in precedence order used at request time:

1. environment variable `<PROVIDER>_API_KEY` — `kido` reads `KIDO_API_KEY`, `opencode` reads `OPENCODE_API_KEY`
2. `yolo auth add <provider> [key]` (stored in `~/.local/share/yolo/auth.json`)
3. `provider.<id>.apiKey` in config (literal or `{env:NAME}`, or `provider.<id>.options.apiKey`)

The in-process server also exposes the opencode `/auth` API (`GET /provider/auth`, `PUT|DELETE /auth/{providerID}`) for parity; yolo has no other credential UI.

The builtin `kido` provider (default) points at `https://ai.kido.ws/v1` with model `Qwen3.8-27B`; override with `provider.kido.baseURL`.

## TUI keymap

| Key | Action |
|---|---|
| enter | send prompt |
| esc | abort turn (busy) / close dialog |
| ctrl+c | quit (confirm) |
| ctrl+p | model dialog |
| ctrl+a | agent dialog |
| / | command menu |
| ↑/↓ / pgup/pgdn | viewport scroll |
| 1/2/3 | permission reply (once / always / reject) |
| e / t | expand tool part / toggle reasoning |

`pgup/pgdn` scroll · `\`+enter newline.

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

Global config lives in `~/.config/yolo/yolo.jsonc` (XDG: honor `XDG_CONFIG_HOME`; `XDG_DATA_HOME` / `XDG_CACHE_HOME` shift the data and cache roots).

## Tests

```sh
go vet ./... && go test ./...     # the CI gate — never hits the network
```

Dev mode: `YOLO_LLM=fake` (+ optional `YOLO_FAKE_SCRIPT=path.json`) swaps the LLM drivers for a scripted fake (one scripted turn per model request; `"delay_ms"` per turn for slow-turn tests). The e2e suite exercises the full TUI against this.

**Live e2e (manual, never in CI)**: `just e2e-live` (`scripts/e2e-live.sh`) builds the binary, boots `yolo serve` from a scratch project pinned to the real `kido` endpoint, and drives the wire contract: health check → create a `yolo`-agent session → send "list files in /tmp" (asserts a completed `read`/`glob`/`grep`/`bash` tool call plus a non-empty text reply) → abort tests (idle → `aborted:false`, busy → `aborted:true`) → SIGTERM → graceful exit 0. Requires `KIDO_API_KEY`; `KIDO_BASE_URL` (default `https://ai.kido.ws/v1`) and `E2E_TIMEOUT` (default 180 s) are optional.

## v1 non-goals

Out of scope for 0.1.0 (landing in 0.2.0+): web/desktop/slack/console frontends, custom themes, MCP integration, snapshot/revert, multi-provider routing beyond kido + opencode (Zen), share links, and any telemetry.

## License

[DO WHAT THE FUCK YOU WANT TO PUBLIC LICENSE (WTFPL), Version 2](LICENSE).
