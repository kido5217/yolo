# S8 — parity audit + close-out (slice bead `yolo-oae.9`)

The parity audit + close-out: the deterministic mock-SSE capture runtime,
the upstream pty-capture fixtures, the per-surface diff sweep, close or
log every visible gap, re-baselined goldens, and the PROGRESS verified
fact.

**State: fully detailed** — the 5-step TDD detail for all 5 tasks is in
the `## S8 detail` section below (Slice Detail Protocol rule 2);
execution may start at task S8.1.

## Binding task table

Pointer only (FROZEN — Slice Detail Protocol rule 1): `plan.md` →
`## Task inventory` → `### S8 — parity audit + close-out (slice bead yolo-oae.9)`.
Bead titles, scope, and pinned commit messages live there and may not be
changed by a detail pass; any required change = STOP + explicit user
approval + re-record in plan.md.

## Dep gate

None (Go). The npm package `opencode-ai@1.18.18` + the node toolchain is a
dev-only capture runtime — NOT a Go dependency, NOT a dep-proposal target:
the parity scripts live under `scripts/` and their fixtures under a testdata
dir (the root dependency policy governs Go modules only).

## Exact upstream sources (the detail pass reads these AT DETAIL TIME)

`/tmp/opencode-upstream` @ v1.18.18 — no upstream source to port; the
parity REFERENCE is:

- the npm package `opencode-ai@1.18.18` — S8.2 pty-capture runs it
  (`opencode serve` against the S8.1 mock SSE server, a scripted key
  sequence, volatile bits stripped, pinned fixtures).
- the upstream tree — behavior reference for the diff-sweep judgment (which
  surface, which expectation).
- the spec's binding methodology: §7 (Verification strategy, item 3 — the
  upstream pty-capture reference: mock SSE → scripted pty → pinned
  fixtures → diff; every visible mismatch that can't be closed becomes a
  DEVIATIONS.md entry with severity) + §9 (Risks & open items — the
  mitigations the sweep must honor, e.g. risk 2's per-dialog huh
  deviations).
- S8.1 mock SSE server: env-gated, local-only, deterministic canned stream
  (root AGENTS.md: unit tests never hit the network — the mock is
  in-process).

## yolo anchors

- `internal/server/testutil` — the test-server scaffolding the mock SSE
  server builds on.
- `scripts/e2e-live.sh` — the existing on-demand live script; the parity
  scripts follow its user-run, never-CI pattern.
- the teatest goldens landed in S1–S7 — re-baselined in S8.4.
- `docs/superpowers/DEVIATIONS.md` — S8.4: close or log every visible gap,
  with severity.
- `docs/superpowers/PROGRESS.md` — S8.5: the verified fact.

## Detail pass (protocol)

One writing-plans pass, one subagent, `thinking=high`, dispatched by the
root session strictly sequentially (root principle 7) — per the Slice
Detail Protocol in plan.md: it fills this file (after this section) with the
full 5-step TDD detail for each task in the binding table (failing test
code, implementation code, gate, pinned commit), reading the named upstream
files at that moment. It commits as
`docs: TUI parity plan — detail S8 tasks` on its own bead
(`bd create "detail S8 plan tasks" --parent=yolo-oae.9 --json`).

## S8 detail

Detail pass 2026-09-03. Deviations tail at detail time = 252; S8 entries
start at 253. Breadcrumb note (DEVIATIONS.md entry 253, severity info):
the frozen S8 table (plan.md) names the task beads `yolo-oae.9.1`–`9.5`,
but the S8 detail bead consumed `yolo-oae.9.1` (created + claimed before
the detail pass; the S1 "detail-bead-last" precedent is impossible because
the detail pass precedes slice start, as in S2/dev 165, S3/dev 188, S4/dev
206, S5/dev 221, S6/dev 233, S7/dev 245). The 5 task beads therefore land
in table order at `yolo-oae.9.2`–`yolo-oae.9.6` (S8.1→.2, S8.2→.3, S8.3→.4,
S8.4→.5, S8.5→.6); the frozen titles and pinned commit messages are
unchanged. No code or wire impact.

### Detail-pass findings (read AT DETAIL TIME, 2026-09-03 — binding)

1. **npm package `opencode-ai@1.18.18` surface** (inspected via `npm pack`
   + a scratch install under `/tmp/opencode-npm` — never inside the repo,
   never `/tmp/opencode`):
   - the package is a wrapper: `postinstall` swaps `bin/opencode.exe`
     (a Node stub) for the platform single-file binary
     (`@opencode-ai/opencode-linux-x64`); the CLI runs the binary directly.
   - CLI surface: `opencode [project]` = the TUI (default), `opencode
     serve` = headless, `opencode attach <url>`; flags `--port/--hostname`
     (external server), `-m provider/model`, `--auto` (auto-approve
     permissions), `--pure` (no plugins), `--print-logs`. The TUI runs the
     core server **in-process in a Bun worker** (the `tui.ts` RPC fetch
     transport) — no HTTP unless `--port/--hostname` is given; the parity
     capture therefore drives the in-process TUI and only the LLM call
     leaves the process (to the S8.1 mock).
   - LLM wiring: an opencode.json `provider.<id>` entry
     (`{name, npm, options:{baseURL,apiKey}, models:{<id>:{name,...}}}`)
     with `npm: "@ai-sdk/openai-compatible"` routes the provider to
     `POST {baseURL}/chat/completions` (the OpenAI-compatible path —
     `BUNDLED_PROVIDERS` statically bundles that ai-sdk adapter, so NO
     network install happens at TUI start). The built-in `openai` provider
     uses the Responses API instead — the capture MUST use a custom
     provider id (`mockllm`) for the mock.
   - determinism knobs (all verified in a pty smoke): hermetic `HOME` +
     `XDG_DATA/CACHE/CONFIG/STATE_HOME`, `OPENCODE_MODELS_PATH=<file>`
     (catalog file — no fetch), `OPENCODE_DISABLE_MODELS_FETCH=1`,
     `OPENCODE_DISABLE_AUTOUPDATE=1`, `OPENCODE_DISABLE_MOUSE=1`, `TERM=
     xterm-256color` + `COLORTERM=truecolor` (24-bit truecolor SGR is
     emitted — `38;2;r;g;b` confirmed in the smoke, deviation 125 holds).
   - catalog: `https://models.opencode.ai/api.json` (212 providers /
     7495 models; fetched with curl + a browser UA — python-urllib gets a
     403). The parity pin is the reduced `{openai}` snapshot (47 models;
     the openai provider only — every other provider is absent from both
     sides). Detail-time snapshot sha256 =
     `3df03cfe32dc1e4df4a5d357ee664cce46e29a6d1a954e063fa10d2641c74ac6`
     (re-baseline + re-capture if a re-fetch differs — the pin is the
     committed file, not the CDN).
   - `--print-logs` writes log lines INTO the pty (verified) — the
     capture MUST NOT use it.
   - the smoke-verified key behavior: a fresh boot lands on the home
     route with the prompt ready (no new-session key needed — the yolo
     side needs `n` first, which is fine: the scripts are per-side and
     the normalizer compares the final screens, not the keystrokes);
     `ctrl+p` opens the command palette (title "Commands", Search field,
     a Suggested category); `ctrl+c` in a session exits **immediately**
     (no quit-confirm) and the app then prints the **session epilogue**
     (upstream `util/presentation.ts` `sessionEpilogue`: the wordmark +
     "Session <title>" + "Continue opencode -s <ses_id>" — the `ses_` id
     is volatile).
2. **Mock SSE frame order** (verified against the upstream ai-sdk
   openai-compatible client in the pty smoke — both turns rendered):
   - text turn: `role` chunk (`delta:{role:"assistant"}`, content
     omitted) → N content chunks (`delta:{content:<run>}`, 6-rune runs in
     the pin) → the finish chunk (`delta:{}`, `finish_reason:"stop"`) →
     the usage chunk (`choices:[]` + `usage:{prompt_tokens,
     completion_tokens,total_tokens}`) → `data: [DONE]`. Every chunk:
     `id`, `object:"chat.completion.chunk"`, `created`, `model`.
   - tool turn: `role` chunk → the tool-call id chunk
     (`tool_calls:[{index:0,id,type:"function",function:{name}}]`) → the
     args chunk (`tool_calls:[{index:0,function:{arguments}}]`) → the
     finish chunk (`finish_reason:"tool_calls"`) → usage → `[DONE]`;
     after the client posts the `role:"tool"` result message the server
     streams the follow-up text turn (the canned `tool_reply`).
   - the request body the client sends: `{stream, messages, tools?, ...}`
     — the mock keys the second call on the presence of a
     `role:"tool"` message OR an assistant message with `tool_calls`.
   - non-stream fallback: `object:"chat.completion"` with
     `choices:[{index:0,message:{role:"assistant",content},
     finish_reason:"stop"}]` + `usage` (kept for testability; the client
     streams).
3. **yolo-side anchors (verified at detail time):**
   - `internal/llm/fake`: `fake.New(turns ...Turn) *fake.Driver`;
     `Turn{Parts []llm.Part}` (the last part MUST carry `Finish`); a
     `Kind:"tool"` part's `Text` mirrors the `Args` JSON (the LOCKED
     convention); `llm.Part{Kind, Name, CallID, Args json.RawMessage,
     Text, Usage *llm.Usage, Finish, Err}`; `llm.Usage{Input, Output,
     Reasoning, CacheRead, CacheWrite int}`.
   - `internal/server/testutil`: `BootWithDriverConfig(t, drv,
     &protocol.Config{})` — the full-stack harness (the TUI test escape
     hatch); `TestServer{URL, Dir, ...}`; the provider registry is
     `provider.NewStaticForTest()` (EMPTY) — so the model-dialog contents
     come ONLY from the config's `provider` map
     (`protocol.ProviderConfig{BaseURL, APIKey, Options,
     Models map[string]any}` — the yolo referent of the upstream
     opencode.json provider entry; the config never reaches the network
     because the LLM is the in-process fake driver).
   - the server seeds the locked catalogs (`internal/server/
     handlers_catalog.go`): agents `build`/`plan`/`yolo` + config-defined
     ids; commands `/help` `/new` `/model` `/agents` `/quit`.
   - the teatest idiom (`session_markdown_test.go`): theme engine
     (`theme.New` + `Resolve`) → `client.New(ts.URL, ts.Dir)` →
     `NewApp(c, store.State{}, "", e)` → `teatest.NewTestModel(t, a,
     teatest.WithInitialTermSize(w, h), teatest.WithProgramOptions(
     tea.WithEnvironment([]string{"TTY_FORCE=1", "TERM=xterm-256color"})))`
     → drive with `press`/`pressCtrlP()`/`pressLeader()`/`tm.Type(s)` +
     `teatest.WaitFor(t, tm.Output(), <merged condition>,
     teatest.WithDuration(...))` → `tm.Quit()` +
     `tm.WaitFinished(t, teatest.WithFinalTimeout(...))`.
   - teatest v2 `Output()` is a drainable buffer and every read of it is
     **non-blocking**: the buffer's own `Read` returns `io.EOF` on empty,
     and one `io.ReadAll` call drains everything currently present.
     CAVEAT (verified at detail time): `io.ReadAll` never reports the EOF —
     its contract converts it to `nil`, so an empty-buffer ReadAll returns
     `(empty, nil)`; a loop that re-calls `io.ReadAll` until `err != nil`
     spins forever (the S8.3 dump test's `drain` is a single `io.ReadAll`
     for exactly this reason). `WaitFor` itself uses
     `io.ReadAll(io.TeeReader(...))` per check. The dump test therefore
     replaces `WaitFor` with a `pumpUntil` that accumulates the FULL stream
     (bounded by a deadline; the deviation-118 merged-condition rule
     applies to the accumulated raw).
   - the 10 TTY_FORCE SGR golden files (the S8.4 re-baseline scope):
     `deletefailed_test.go`, `help_test.go`, `home_theme_test.go`,
     `permission_theme_test.go`, `session_error_test.go`,
     `session_markdown_test.go`, `session_reasoning_test.go`,
     `sessionsdlg_test.go`, `session_theme_test.go`,
     `tui_suite_test.go` (the goldens are inline SGR pins, not testdata
     files; the S0 matrix goldens `internal/tui/theme/testdata/*.json`
     are a separate S0 pin).
   - yolo dialog titles (the yolo-side terminal tokens): palette
     "Commands" (the `command_list` opener, `palette_test.go`), help
     "Help" (the bold header row), model "Model" + "loading…" (the
     `dlgModel`), agents "Agents" (the `dlgAgent`), sessions "Sessions"
     (the `dlgSessions`), themes "Themes" (the `dlgThemes`), status
     "Status" (the `statusHeaderRow`), rename "Rename Session" (the huh
     input form), the two-step session delete INSIDE the sessions dialog:
     `ctrl+d` arms the row (the title becomes "Press ctrl+d again to
     confirm"), a second `ctrl+d` deletes (`sessionsdlg.go` — the yolo
     `session_delete` referent; the registry binding `ctrl+d` in the
     session route is the yolo-surface inert case — the dialog opener is
     the parity referent).
     MODEL LIST TOKEN (verified by render at detail time): the dialog
     lists `st.Providers` in catalog order — the kido local provider
     ("Qwen", the selected ● row) + the built-in zen catalog ("Claude
     Opus 4.7", "GPT-5 Nano", …) render ABOVE THE FOLD; the catalog-pin
     `openai` provider's 47 models (gpt-4o → name "GPT-4o" — title case,
     `provider.go cfgModel` reads the catalog `name` field) are a group
     BELOW THE FOLD — the model surface cond is therefore
     `hasLines("Model", "Qwen")` (NOT "GPT-4o": it never renders in the
     initial 80x24 screen).
   - the which-key overlay (`whichkey.go`): the held-leader overlay lists
     the current context's `<leader>`-prefixed bindings, grouped by the
     binding-name prefix (Model/Agent/Session/Status/Theme/App/…); the
     default `Current()` context is `BaseMode` (home route — no mode
     pushed) whose group carries the leader continuations
     (`model_list` `<leader>m`, `agent_list` `<leader>a`,
     `status_view` `<leader>s`, `theme_list` `<leader>t`,
     `session_new` `<leader>n`, `session_list` `<leader>l`,
     `sidebar_toggle` `<leader>b`, …) — so the which-key surface runs on
     the HOME route (both sides).
   - **the yolo has NO exit epilogue** (verified: `cmd/yolo/main.go
     tuiCmd` prints nothing after `program.Run()`; no epilogue referent
     in `internal/tui`) — the `epilogue` surface is a known gap (S8.4
     closes or logs it; the upstream text is "Continue opencode -s
     <ses_id>", the yolo referent would be "Continue yolo <id>").
   - the `todowrite` tool input shape (both sides):
     `{"todos":[{"content","status","priority"?}]}` (yolo
     `internal/tool/todowrite.go`; upstream `packages/opencode/src/tool/
     todo.ts` `Schema.Struct({todos: array(Todo.Info)})`) — the
     `sidebar` surface's canned turn uses it (the todo sidebar shows for
     an open-item list, both sides: the upstream `todo.tsx` show gate +
     the yolo S7.1 `latestTodos` referent).
   - `internal/tui/keymap.go` `contextGroups` (L815): `BaseMode` = the
     leader-continuation set above; `"session"` =
     `messages_page_up/down`, `session_interrupt`, `session_rename` —
     the yolo session route dispatches `session_rename` (`ctrl+r`) +
     `session_interrupt` (`esc`) in `handleSessionKey` (L578+).
4. **scripts layout** (the user-run, never-CI pattern — the root
   `scripts/e2e-live.sh` precedent: `set -uo pipefail`, `step/ok/bad`,
   exit 0/1, the justfile target runs the script):
   - `scripts/parity/mock/main.go` — the S8.1 mock runner (a `main`
     package under `scripts/` — no import-purity constraint; it builds
     with the module, runs on 127.0.0.1 only).
   - `scripts/parity/capture.sh` + `capture.py` + `normalize.py` (S8.2),
     `scripts/parity/sweep.py` (S8.3).
   - fixtures: `internal/tui/testdata/parity/` — `canned.json` (S8.1, the
     shared canned book the Go mock is pinned to), `catalog-pin.json`
     (S8.2, the reduced `{openai}` catalog snapshot),
     `upstream/<surface>.screen.json` × 17 + `upstream/MANIFEST.json`
     (S8.2, the pinned fixtures).
   - the sweep report: `docs/superpowers/plans/2026-08-24-opencode-tui-
     parity/parity-sweep-report.md` (S8.3, re-written on every sweep).

### Design decisions (binding)

- **D1 — one shared canned book.** `internal/tui/testdata/parity/
  canned.json` is the single source for the scripted turns; the S8.1 Go
  mock is pinned to it by `TestCannedMatchesDefault` (root principle 3),
  the S8.2 capture passes it to the mock via `-canned` (the built-in
  `DefaultBook()` is the default), and the S8.3 yolo dump scripts the
  fake driver from the same constants (pinned by
  `TestParityCannedConsistent`). Three turns: `text` (the pinned markdown
  reply exercising headings/bold/code/list/quote/table/link/js-fence/CJK/
  final-line), `tool` (the bash `echo parity-ok` call + the follow-up),
  `todo` (the `todowrite` call + the follow-up — the sidebar surface).
  Fixed ids: `chatcmpl-canned01`, bash `call_canned1`, todowrite
  `call_canned2`, `created 1700000000`, model `canned`, usage
  `{input:12, output:40}`.
- **D2 — the 17-surface list (frozen at detail time).** Each surface = a
  fresh boot (upstream: a fresh hermetic HOME + pty; yolo: a fresh
  testutil stack + teatest app), one scripted key sequence, one final
  screen. Sizes: 80×24 except `sidebar` (140×30 — the upstream sidebar
  auto-shows at width > 120; the yolo S7.2 port mirrors the gate):
  `home`, `session-text`, `session-tool`, `palette`, `help`, `model`,
  `agent`, `theme`, `session-list`, `session-rename`, `session-delete`,
  `status`, `which-key`, `sidebar`, `prompt-slash`, `prompt-mention`,
  `epilogue`.
- **D3 — the shared normalizer.** `scripts/parity/normalize.py` replays a
  raw terminal byte stream into ONE canonical screen JSON
  (`{"cols","rows","cells":{"<r>:<c>":{"t","fg","bg","b"}}}`) via
  cursor-positioning + SGR replay; the FINAL frame wins (both TUIs
  repaint with absolute positioning, so the replay is the true final
  screen). Volatile bits are masked to fixed tokens BEFORE replay: OSC
  12/0/22 (cursor color / titles / OSC 22), synchronized output
  (`[>4;…m` / `[<4m`), `ses_[A-Za-z0-9]{10,}` → `<SES>`, ISO-8601
  timestamps → `<TS>`, `/tmp/opencode-parity` → `<SCRATCH>`. Truecolor
  and 256-color values are kept VERBATIM — the color-space difference
  (upstream 24-bit vs yolo ANSI256, deviation 125) is a parity finding
  for S8.4, not normalizer noise.
- **D4 — the pinned fixtures are the NORMALIZED screens.**
  `upstream/<surface>.screen.json` = the normalizer output of the upstream
  capture (not the raw pty bytes); `MANIFEST.json` carries per-surface
  `{name, cols, rows, sha256}` + `npm_version` + `catalog_sha256` +
  `canned_sha256`. The pin guard (`TestParityFixturesPinned`) fails on any
  drift. Re-baseline = re-capture + commit the changed fixtures in the
  same commit (root principle 3).
- **D5 — the capture is double-run.** Each surface is captured TWICE
  (fresh pty each run); the normalized screens must be byte-identical or
  the capture FAILs (exit 1) — the determinism gate before the fixture is
  written.
- **D6 — the yolo dump is teatest, not a real pty.** `TestParityDump`
  (gated on `YOLO_PARITY_DUMP`, `t.Skip` when unset — the CI gate never
  renders the set) renders each surface through the real stack
  (testutil + the fake driver scripted per D1 + the theme engine with
  `TTY_FORCE=1` + `TERM=xterm-256color`) and writes the FULL raw teatest
  stream to `$YOLO_PARITY_DUMP/yolo/<surface>.raw`; the sweep normalizes
  it with the SAME `normalize.py`. The yolo key scripts mirror the
  upstream ones (per-side: the yolo side adds `n` for the new session;
  the which-key surface runs on home, both sides). Two script-safety
  notes (verified at detail time): the tool/todo surfaces auto-approve
  (yolo: the dump config seeds `permission: {bash: allow, todowrite:
  allow}` via `ParsePerms` wildcard rules; upstream: the symmetric
  opencode.json `permission` map + `--auto`) so both sides reach the
  second turn without a permission overlay; and the fake driver draws
  the per-session title-generation request from its `TitleTurns` (empty
  → empty stream), never from the scripted canned `Turns` — the canned
  turns are safe from the title leg.
- **D7 — the sweep is mechanical; S8.4 is the judgment.**
  `sweep.py` = run the yolo dump → normalize both sides → per-surface
  cell diff → `parity-sweep-report.md` (MATCH / GAPS(n) table + the
  mismatch detail + the environment: yolo HEAD sha, manifest sha, npm
  version). Exit 0 on a COMPLETED sweep (gaps are informational — S8.4
  consumes the report), exit 1 on a mechanical failure (go test failure,
  missing fixture, crash). S8.4's rule: every GAPS surface is either
  CLOSED (the yolo render change + the re-baselined SGR goldens in the
  same commit) or LOGGED (a DEVIATIONS.md entry with severity + the
  surface noted in the report); re-run the sweep until every surface is
  MATCH or logged.
- **D8 — the expected gaps (read at detail time).** (1) `epilogue`:
  upstream prints the session epilogue on exit; yolo prints nothing
  (finding 3) — CLOSE is in scope (the yolo referent exists: `yolo
  [<sessionID>]` resume — the ported text "Continue yolo <id>"); if the
  close needs a behavior beyond the surface, LOG it (severity medium).
  (2) the color space (upstream truecolor vs yolo ANSI256, deviation 125)
  — every fg/bg mismatch whose ONLY difference is the color encoding is a
  known-surface: LOG once (severity info) if it is the only gap class on
  a surface. (3) which-key overlay contents (the yolo context filter,
  deviation 207(3)) — expect text gaps. (4) anything else the sweep
  finds — S8.4 judges per D7.
- **D9 — beads.** Each task bead is created at task-execution time (NOT
  by the detail pass) with the frozen title: `bd create "<frozen title>"
  --description="S8.x — execute per docs/superpowers/plans/2026-08-24-
  opencode-tui-parity/s8-parity-audit.md (## S8 detail, Task S8.x)" -t
  task -p 2 --parent=yolo-oae.9 --json`, then `bd update <id> --claim
  --json`.

### Task S8.1: Mock OpenAI-compatible SSE server (canned stream) for deterministic capture (bead `yolo-oae.9.1`, expected id `yolo-oae.9.2`)

**Files:** new `internal/llm/mockllm/mockllm.go` (the handler + the canned
types), new `internal/llm/mockllm/mockllm_test.go`, new
`internal/tui/testdata/parity/canned.json` (the shared fixture — created
here because the cross-pin test needs it), new `scripts/parity/mock/
main.go` (the runnable mock: 127.0.0.1 listener + the `MOCK_PORT=`
handshake).

**Interfaces:** `mockllm.Usage{Input, Output int}` (json `input`/
`output`), `mockllm.Tool{Name, Args string}`, `mockllm.Canned{Prompt,
Reply string; ChunkSize int; Tool *Tool; ToolReply string; Usage
Usage}`, `mockllm.Book{Text, Tool, Todo Canned}`,
`mockllm.DefaultBook() Book` (pinned against `canned.json`),
`mockllm.LoadBook(raw []byte) (Book, error)`,
`mockllm.Handler(c Canned) http.Handler` (POST any
`*/chat/completions` = the canned stream for the turn — the tool turn
streams the tool call until a `role:"tool"` result is posted, then the
`ToolReply` text turn; GET any `*/models` = the pinned single-model list;
anything else 404). The request path is suffix-matched (the ai-sdk
client posts `{baseURL}/chat/completions` with a caller-chosen baseURL
prefix).

**Step 1 — failing test** (`internal/llm/mockllm/mockllm_test.go`):

```go
package mockllm

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const cannedReply = "## Heading\n\nSome **bold** and `inline code` text.\n\n- one\n- two\n\n> a quote\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n[link](https://example.com)\n\n```js\nconst x = 1;\n```\n\n你好 world\n\nDone."

const toolCannedArgs = `{"command":"echo parity-ok"}`
const toolCannedReply = "The check printed parity-ok.\n"
const todoCannedArgs = `{"todos":[{"content":"first item","status":"in_progress"},{"content":"second item","status":"pending"}]}`
const todoCannedReply = "Todos updated.\n"

func textCanned() Canned {
	return Canned{Prompt: "say hi to the mock", Reply: cannedReply, ChunkSize: 6, Usage: Usage{Input: 12, Output: 40}}
}

func toolCanned() Canned {
	return Canned{
		Prompt:    "run the parity check",
		Tool:      &Tool{Name: "bash", Args: toolCannedArgs},
		ToolReply: toolCannedReply,
		ChunkSize: 6,
		Usage:     Usage{Input: 12, Output: 40},
	}
}

func todoCanned() Canned {
	return Canned{
		Prompt:    "add two todos",
		Tool:      &Tool{Name: "todowrite", Args: todoCannedArgs},
		ToolReply: todoCannedReply,
		ChunkSize: 6,
		Usage:     Usage{Input: 12, Output: 40},
	}
}

type testFrame struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// postStream runs one chat/completions request and returns the "data:"
// payloads in order (the last is [DONE]).
func postStream(t *testing.T, c Canned, body string) []string {
	t.Helper()
	srv := httptest.NewServer(Handler(c))
	defer srv.Close()
	resp, err := http.Post(srv.URL+"/v1/chat/completions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var out []string
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "data: ") {
			out = append(out, strings.TrimPrefix(line, "data: "))
		}
	}
	if len(out) == 0 || out[len(out)-1] != "[DONE]" {
		t.Fatalf("expected a [DONE] terminator, got %d frames: %q", len(out), out)
	}
	return out
}

func frames(t *testing.T, payloads []string) []testFrame {
	t.Helper()
	var out []testFrame
	for _, p := range payloads {
		if p == "[DONE]" {
			continue
		}
		var f testFrame
		if err := json.Unmarshal([]byte(p), &f); err != nil {
			t.Fatalf("frame %q: %v", p, err)
		}
		out = append(out, f)
	}
	return out
}

func pinMeta(t *testing.T, fs []testFrame) {
	t.Helper()
	for i, f := range fs {
		if f.ID != "chatcmpl-canned01" || f.Object != "chat.completion.chunk" ||
			f.Created != 1700000000 || f.Model != "canned" {
			t.Fatalf("frame %d meta = %+v (want the fixed id/object/created/model)", i, f)
		}
	}
}

// TestCannedStreamFrames pins the text-turn frame order (D1): role chunk →
// ≤6-rune content chunks (the re-join is the canned reply) → the finish
// chunk → the usage chunk → [DONE].
func TestCannedStreamFrames(t *testing.T) {
	p := postStream(t, textCanned(), `{"stream":true,"messages":[{"role":"user","content":"say hi to the mock"}]}`)
	fs := frames(t, p)
	if len(fs) < 4 {
		t.Fatalf("frame count = %d, want >= 4 (role + content + finish + usage)", len(fs))
	}
	pinMeta(t, fs)
	if got := fs[0].Choices[0].Delta; got.Role != "assistant" || got.Content != "" {
		t.Fatalf("first frame delta = %+v, want role=assistant content empty", got)
	}
	var joined string
	for _, f := range fs[1 : len(fs)-2] {
		if len(f.Choices) != 1 {
			t.Fatalf("content frame: %d choices", len(f.Choices))
		}
		d := f.Choices[0].Delta
		if d.Role != "" || len([]rune(d.Content)) == 0 || len([]rune(d.Content)) > 6 {
			t.Fatalf("bad content chunk: %+v", d)
		}
		joined += d.Content
	}
	if joined != cannedReply {
		t.Fatalf("re-joined content != canned reply:\n got %q\nwant %q", joined, cannedReply)
	}
	fin := fs[len(fs)-2]
	if fin.Choices[0].FinishReason == nil || *fin.Choices[0].FinishReason != "stop" {
		t.Fatalf("finish chunk = %+v, want finish_reason=stop", fin.Choices[0])
	}
	ug := fs[len(fs)-1]
	if len(ug.Choices) != 0 || ug.Usage == nil ||
		ug.Usage.PromptTokens != 12 || ug.Usage.CompletionTokens != 40 || ug.Usage.TotalTokens != 52 {
		t.Fatalf("usage chunk = %+v, want empty choices + 12/40/52", ug)
	}
}

// TestToolTurn pins the tool-call frame order + the post-result follow-up
// (D1): role → tc-id → tc-args → finish(tool_calls) → usage → [DONE],
// then the ToolReply text turn after the tool result is posted.
func TestToolTurn(t *testing.T) {
	p := postStream(t, toolCanned(), `{"stream":true,"messages":[{"role":"user","content":"run the parity check"}]}`)
	fs := frames(t, p)
	if len(fs) != 5 {
		t.Fatalf("tool frame count = %d, want 5 (role + tc-id + tc-args + finish + usage)", len(fs))
	}
	pinMeta(t, fs)
	if tc := fs[1].Choices[0].Delta.ToolCalls; len(tc) != 1 || tc[0].ID != "call_canned1" ||
		tc[0].Type != "function" || tc[0].Function.Name != "bash" || tc[0].Function.Arguments != "" {
		t.Fatalf("tc-id frame = %+v", tc)
	}
	if tc := fs[2].Choices[0].Delta.ToolCalls; len(tc) != 1 || tc[0].Function.Arguments != toolCannedArgs {
		t.Fatalf("tc-args frame = %+v", tc)
	}
	if fr := fs[3].Choices[0].FinishReason; fr == nil || *fr != "tool_calls" {
		t.Fatalf("tool finish = %+v, want tool_calls", fs[3].Choices[0])
	}
	reqBody, err := json.Marshal(map[string]any{
		"stream": true,
		"messages": []any{
			map[string]string{"role": "user", "content": "run the parity check"},
			map[string]any{
				"role":    "assistant",
				"content": "",
				"tool_calls": []any{map[string]any{
					"id":       "call_canned1",
					"function": map[string]string{"name": "bash", "arguments": toolCannedArgs},
				}},
			},
			map[string]string{"role": "tool", "tool_call_id": "call_canned1", "content": "parity-ok\n"},
		},
	})
	if err != nil {
		t.Fatalf("marshal follow-up body: %v", err)
	}
	p2 := postStream(t, toolCanned(), string(reqBody))
	fs2 := frames(t, p2)
	var joined string
	for _, f := range fs2[1 : len(fs2)-2] {
		joined += f.Choices[0].Delta.Content
	}
	if joined != toolCannedReply {
		t.Fatalf("follow-up re-join = %q, want %q", joined, toolCannedReply)
	}
}

// TestTodoTurn pins the todowrite call id + args (the D1 third turn).
func TestTodoTurn(t *testing.T) {
	p := postStream(t, todoCanned(), `{"stream":true,"messages":[{"role":"user","content":"add two todos"}]}`)
	fs := frames(t, p)
	if len(fs) != 5 {
		t.Fatalf("todo frame count = %d, want 5", len(fs))
	}
	if tc := fs[1].Choices[0].Delta.ToolCalls; len(tc) != 1 || tc[0].ID != "call_canned2" ||
		tc[0].Function.Name != "todowrite" {
		t.Fatalf("todo tc-id frame = %+v", tc)
	}
	if tc := fs[2].Choices[0].Delta.ToolCalls; len(tc) != 1 || tc[0].Function.Arguments != todoCannedArgs {
		t.Fatalf("todo tc-args frame = %+v", tc)
	}
}

// TestNonStream pins the non-stream completion body.
func TestNonStream(t *testing.T) {
	srv := httptest.NewServer(Handler(textCanned()))
	defer srv.Close()
	resp, err := http.Post(srv.URL+"/v1/chat/completions", "application/json",
		strings.NewReader(`{"stream":false,"messages":[{"role":"user","content":"say hi to the mock"}]}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	var b struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		Model   string `json:"model"`
		Choices []struct {
			Index   int `json:"index"`
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&b); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if b.ID != "chatcmpl-canned01" || b.Object != "chat.completion" || b.Created != 1700000000 ||
		b.Model != "canned" || len(b.Choices) != 1 ||
		b.Choices[0].Message.Content != cannedReply || b.Choices[0].FinishReason != "stop" ||
		b.Usage.PromptTokens != 12 || b.Usage.TotalTokens != 52 {
		t.Fatalf("non-stream body = %+v", b)
	}
}

// TestModelsEndpoint pins the GET /models body (byte-identical).
func TestModelsEndpoint(t *testing.T) {
	srv := httptest.NewServer(Handler(textCanned()))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/v1/models")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(raw, modelsJSON) {
		t.Fatalf("models body = %s, want the pinned body", raw)
	}
}

// TestDeterminism pins byte-identical streams across two handler
// instances (the capture-determinism gate, D5's Go-side referent).
func TestDeterminism(t *testing.T) {
	body := `{"stream":true,"messages":[{"role":"user","content":"say hi to the mock"}]}`
	getBody := func() []byte {
		srv := httptest.NewServer(Handler(textCanned()))
		defer srv.Close()
		resp, err := http.Post(srv.URL+"/v1/chat/completions", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("post: %v", err)
		}
		defer resp.Body.Close()
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		return raw
	}
	if !bytes.Equal(getBody(), getBody()) {
		t.Fatal("two handler instances produced different streams")
	}
}

// TestCannedMatchesDefault pins the shared fixture against DefaultBook
// (D1, root principle 3 — an intentional change re-baselines BOTH in the
// same commit).
func TestCannedMatchesDefault(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "tui", "testdata", "parity", "canned.json"))
	if err != nil {
		t.Fatalf("canned.json: %v", err)
	}
	b, err := LoadBook(raw)
	if err != nil {
		t.Fatalf("LoadBook: %v", err)
	}
	if !reflect.DeepEqual(b, DefaultBook()) {
		t.Fatalf("canned.json drifted from DefaultBook:\n got %+v\nwant %+v", b, DefaultBook())
	}
}
```

**Step 2 — confirm FAIL:** `go test ./internal/llm/mockllm/` — the
package does not exist yet (build failure: `no Go files in
/home/kido/network/projects/yolo/internal/llm/mockllm` / `cannot find
package`). `go test ./internal/tui/ -run TestParityCannedConsistent` is
NOT run yet (that test lands in S8.2 with its fixture pin).

**Step 3 — minimal implementation:**

`internal/llm/mockllm/mockllm.go`:

```go
// Package mockllm is the S8.1 deterministic OpenAI-compatible mock (spec
// §7.3; root AGENTS.md: unit tests never hit the network): it serves a
// pinned canned chat-completions stream with byte-deterministic SSE
// frames on a 127.0.0.1 listener; the S8.2 parity capture runs the
// upstream npm TUI against it. Pure net/http — no state, no egress.
package mockllm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// The fixed wire values (pinned for the byte-deterministic captures, D1).
const (
	completionID = "chatcmpl-canned01"
	bashCallID   = "call_canned1"
	todoCallID   = "call_canned2"
	fixedCreated = 1700000000
	fixedModel   = "canned"
)

// modelsJSON is the pinned GET /models body (byte-identical).
var modelsJSON = []byte(`{"object":"list","data":[{"id":"canned","object":"model","created":1700000000,"owned_by":"parity"}]}`)

var notFoundJSON = []byte(`{"error":{"message":"not found"}}`)

// Usage is the canned usage reported in the completion (the yolo
// llm.Usage int shape — the D1 cross-pin target).
type Usage struct {
	Input  int `json:"input"`
	Output int `json:"output"`
}

// Tool is one canned tool call (name + the JSON args string the
// completion streams).
type Tool struct {
	Name string `json:"name"`
	Args string `json:"args"`
}

// Canned is one scripted turn: the text turn streams Reply as content
// chunks; a tool turn streams Tool as tool_calls frames, then — once the
// client posts the tool result — the ToolReply text turn.
type Canned struct {
	Prompt    string `json:"prompt"`
	Reply     string `json:"reply,omitempty"`
	ChunkSize int    `json:"chunk_size,omitempty"`
	Tool      *Tool  `json:"tool,omitempty"`
	ToolReply string `json:"tool_reply,omitempty"`
	Usage     Usage  `json:"usage"`
}

// Book is the canned set (the pinned fixture
// internal/tui/testdata/parity/canned.json decodes to exactly
// DefaultBook — TestCannedMatchesDefault).
type Book struct {
	Text Canned `json:"text"`
	Tool Canned `json:"tool"`
	Todo Canned `json:"todo"`
}

// DefaultBook is the built-in book (D1).
func DefaultBook() Book {
	return Book{
		Text: Canned{
			Prompt:    "say hi to the mock",
			Reply:     "## Heading\n\nSome **bold** and `inline code` text.\n\n- one\n- two\n\n> a quote\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n[link](https://example.com)\n\n```js\nconst x = 1;\n```\n\n你好 world\n\nDone.",
			ChunkSize: 6,
			Usage:     Usage{Input: 12, Output: 40},
		},
		Tool: Canned{
			Prompt:    "run the parity check",
			Tool:      &Tool{Name: "bash", Args: `{"command":"echo parity-ok"}`},
			ToolReply: "The check printed parity-ok.\n",
			ChunkSize: 6,
			Usage:     Usage{Input: 12, Output: 40},
		},
		Todo: Canned{
			Prompt:    "add two todos",
			Tool:      &Tool{Name: "todowrite", Args: `{"todos":[{"content":"first item","status":"in_progress"},{"content":"second item","status":"pending"}]}`},
			ToolReply: "Todos updated.\n",
			ChunkSize: 6,
			Usage:     Usage{Input: 12, Output: 40},
		},
	}
}

// LoadBook decodes a canned book (the S8.2 capture passes the shared
// fixture to the mock via -canned so both sides share one source).
func LoadBook(raw []byte) (Book, error) {
	var b Book
	if err := json.Unmarshal(raw, &b); err != nil {
		return Book{}, err
	}
	return b, nil
}

// Handler returns the mock's http.Handler for one canned turn (D1): POST
// any */chat/completions serves the canned stream (a tool turn streams the
// tool call until the request carries a tool result, then ToolReply); GET
// any */models serves the pinned model list.
func Handler(c Canned) http.Handler {
	if c.ChunkSize <= 0 {
		c.ChunkSize = 6
	}
	toolTurn := c.Tool != nil
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/models") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(modelsJSON)
			return
		}
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/chat/completions") {
			var req chatRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			reply := c.Reply
			if toolTurn && req.hasToolResult() {
				reply = c.ToolReply
			}
			if !toolTurn || req.hasToolResult() {
				if req.Stream {
					writeTextStream(w, c, reply)
				} else {
					writeTextJSON(w, c, reply)
				}
				return
			}
			writeToolStream(w, c)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write(notFoundJSON)
	})
}

type chatRequest struct {
	Stream   bool `json:"stream"`
	Messages []struct {
		Role      string `json:"role"`
		ToolCalls []any  `json:"tool_calls"`
	} `json:"messages"`
}

func (q chatRequest) hasToolResult() bool {
	for _, m := range q.Messages {
		if m.Role == "tool" || (m.Role == "assistant" && len(m.ToolCalls) > 0) {
			return true
		}
	}
	return false
}

// The wire shapes (field order = byte order — the determinism pin).
type wireUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (u Usage) wire() wireUsage {
	return wireUsage{PromptTokens: u.Input, CompletionTokens: u.Output, TotalTokens: u.Input + u.Output}
}

type wireFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type wireToolCall struct {
	Index    int          `json:"index"`
	ID       string       `json:"id,omitempty"`
	Type     string       `json:"type,omitempty"`
	Function wireFunction `json:"function"`
}

type wireDelta struct {
	Role      string         `json:"role,omitempty"`
	Content   string         `json:"content,omitempty"`
	ToolCalls []wireToolCall `json:"tool_calls,omitempty"`
}

type wireChoice struct {
	Index        int       `json:"index"`
	Delta        wireDelta `json:"delta"`
	FinishReason *string   `json:"finish_reason,omitempty"`
}

type wireFrame struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []wireChoice `json:"choices"`
	Usage   *wireUsage   `json:"usage,omitempty"`
}

type wireNSMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type wireNSChoice struct {
	Index        int           `json:"index"`
	Message      wireNSMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type wireNSBody struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []wireNSChoice `json:"choices"`
	Usage   wireUsage      `json:"usage"`
}

func sse(w http.ResponseWriter, v any) {
	b, _ := json.Marshal(v)
	fmt.Fprintf(w, "data: %s\n\n", b)
}

func baseFrame() wireFrame {
	return wireFrame{ID: completionID, Object: "chat.completion.chunk", Created: fixedCreated, Model: fixedModel}
}

// callIDFor is the fixed tool-call id (one per tool name — D1).
func callIDFor(name string) string {
	if name == "todowrite" {
		return todoCallID
	}
	return bashCallID
}

// chunks splits s into rune runs of at most n runes (the D1 6-rune pin).
func chunks(s string, n int) []string {
	if n < 1 {
		n = 1
	}
	r := []rune(s)
	out := make([]string, 0, (len(r)+n-1)/n)
	for i := 0; i < len(r); i += n {
		end := i + n
		if end > len(r) {
			end = len(r)
		}
		out = append(out, string(r[i:end]))
	}
	return out
}

func writeTextStream(w http.ResponseWriter, c Canned, reply string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	f := baseFrame()
	f.Choices = []wireChoice{{Index: 0, Delta: wireDelta{Role: "assistant"}}}
	sse(w, f)
	for _, p := range chunks(reply, c.ChunkSize) {
		f := baseFrame()
		f.Choices = []wireChoice{{Index: 0, Delta: wireDelta{Content: p}}}
		sse(w, f)
	}
	f = baseFrame()
	stop := "stop"
	f.Choices = []wireChoice{{Index: 0, FinishReason: &stop}}
	sse(w, f)
	f = baseFrame()
	f.Choices = []wireChoice{}
	u := c.Usage.wire()
	f.Usage = &u
	sse(w, f)
	fmt.Fprint(w, "data: [DONE]\n\n")
}

func writeToolStream(w http.ResponseWriter, c Canned) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	id := callIDFor(c.Tool.Name)
	f := baseFrame()
	f.Choices = []wireChoice{{Index: 0, Delta: wireDelta{Role: "assistant"}}}
	sse(w, f)
	f = baseFrame()
	f.Choices = []wireChoice{{Index: 0, Delta: wireDelta{ToolCalls: []wireToolCall{{Index: 0, ID: id, Type: "function", Function: wireFunction{Name: c.Tool.Name}}}}}}
	sse(w, f)
	f = baseFrame()
	f.Choices = []wireChoice{{Index: 0, Delta: wireDelta{ToolCalls: []wireToolCall{{Index: 0, Function: wireFunction{Arguments: c.Tool.Args}}}}}}
	sse(w, f)
	f = baseFrame()
	toolCalls := "tool_calls"
	f.Choices = []wireChoice{{Index: 0, FinishReason: &toolCalls}}
	sse(w, f)
	f = baseFrame()
	f.Choices = []wireChoice{}
	u := c.Usage.wire()
	f.Usage = &u
	sse(w, f)
	fmt.Fprint(w, "data: [DONE]\n\n")
}

func writeTextJSON(w http.ResponseWriter, c Canned, reply string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	body := wireNSBody{
		ID:      completionID,
		Object:  "chat.completion",
		Created: fixedCreated,
		Model:   fixedModel,
		Choices: []wireNSChoice{{Index: 0, Message: wireNSMessage{Role: "assistant", Content: reply}, FinishReason: "stop"}},
		Usage:   c.Usage.wire(),
	}
	_ = json.NewEncoder(w).Encode(body)
}
```

`internal/tui/testdata/parity/canned.json` (the shared fixture — exactly
`DefaultBook` under the JSON tags; an intentional change re-baselines the
file + `DefaultBook` + the Go constants in the same commit):

```json
{
  "text": {
    "prompt": "say hi to the mock",
    "reply": "## Heading\n\nSome **bold** and `inline code` text.\n\n- one\n- two\n\n> a quote\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n[link](https://example.com)\n\n```js\nconst x = 1;\n```\n\n你好 world\n\nDone.",
    "chunk_size": 6,
    "usage": { "input": 12, "output": 40 }
  },
  "tool": {
    "prompt": "run the parity check",
    "tool": { "name": "bash", "args": "{\"command\":\"echo parity-ok\"}" },
    "tool_reply": "The check printed parity-ok.\n",
    "chunk_size": 6,
    "usage": { "input": 12, "output": 40 }
  },
  "todo": {
    "prompt": "add two todos",
    "tool": { "name": "todowrite", "args": "{\"todos\":[{\"content\":\"first item\",\"status\":\"in_progress\"},{\"content\":\"second item\",\"status\":\"pending\"}]}" },
    "tool_reply": "Todos updated.\n",
    "chunk_size": 6,
    "usage": { "input": 12, "output": 40 }
  }
}
```

`scripts/parity/mock/main.go` (the runnable mock — user-run, never CI):

```go
// Command mock is the S8.1 parity mock runner: it binds 127.0.0.1:0,
// prints MOCK_PORT=<port> (the handshake line the S8.2 capture reads),
// and serves the canned book for -turn (text|tool|todo). The capture
// starts it per surface and kills it when the pty closes.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/kido5217/yolo/internal/llm/mockllm"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:0", "listen address (0 = ephemeral port)")
	turn := flag.String("turn", "text", "canned turn: text | tool | todo")
	canned := flag.String("canned", "", "canned book JSON path (default: the built-in book)")
	flag.Parse()

	book := mockllm.DefaultBook()
	if *canned != "" {
		raw, err := os.ReadFile(*canned)
		if err != nil {
			log.Fatalf("canned: %v", err)
		}
		if book, err = mockllm.LoadBook(raw); err != nil {
			log.Fatalf("canned: %v", err)
		}
	}
	var c mockllm.Canned
	switch *turn {
	case "text":
		c = book.Text
	case "tool":
		c = book.Tool
	case "todo":
		c = book.Todo
	default:
		log.Fatalf("unknown turn %q (text|tool|todo)", *turn)
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	srv := &http.Server{Handler: mockllm.Handler(c)}
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Fatalf("serve: %v", err)
		}
	}()
	fmt.Printf("MOCK_PORT=%d\n", ln.Addr().(*net.TCPAddr).Port)
	select {} // the capture kills the process
}
```

**Step 4 — gate:** `go vet ./... && go test ./...` green (incl.
`TestCannedMatchesDefault` + `TestImportsDirection` — `mockllm` imports
only stdlib; `scripts/parity/mock` imports the module + stdlib) +
`gofmt -l .` empty.

**Step 5 — commit** the pinned message `test: mock OpenAI-compatible SSE
server for parity capture` (staging `internal/llm/mockllm/`,
`internal/tui/testdata/parity/canned.json`, `scripts/parity/mock/`), then
`bd close yolo-oae.9.2 --reason "S8.1 done: the mockllm package (the
byte-deterministic canned SSE stream — text/tool/todo turns, the fixed
ids/created/model/usage, the tool-result branch, the /models endpoint) +
the runnable scripts/parity/mock (MOCK_PORT handshake) + the shared
canned.json fixture pinned to DefaultBook" --json`.

### Task S8.2: Upstream pty-capture script (npm `opencode-ai@1.18.18`, scripted keys, volatile bits stripped, pinned fixtures) (bead `yolo-oae.9.2`, expected id `yolo-oae.9.3`)

**Files:** new `scripts/parity/capture.sh` (the entry: the npm runtime +
the mock build + the catalog pin + the python driver), new
`scripts/parity/capture.py` (the pty driver + the double-run determinism +
the fixture/manifest writer), new `scripts/parity/normalize.py` (the D3
shared normalizer), new `internal/tui/testdata/parity/catalog-pin.json`
(the reduced `{openai}` catalog snapshot — created by the capture's
first run; the detail-time sha256
`3df03cfe32dc1e4df4a5d357ee664cce46e29a6d1a954e063fa10d2641c74ac6`),
new `internal/tui/testdata/parity/upstream/` (the 17
`<surface>.screen.json` fixtures + `MANIFEST.json`, written by the
capture), new `internal/tui/parity_fixture_test.go` (the pin guard + the
canned-consistency test), modify `justfile` (the `parity-capture`
target).

**Interfaces:** `normalize.py` — `mask(data: bytes) -> bytes`,
`screen(data: bytes, cols: int, rows: int) -> dict` (the D3 canonical
screen JSON); `capture.py` — the SURFACES table (the D2 17-surface list,
frozen), `run_pty(cols, rows, steps) -> bytes`, `capture_surface(name,
size, turn, steps) -> screen` (the double-run gate, D5); `capture.sh` —
the exit-code contract (0 = PASS + the re-baselined fixture list printed,
1 = FAIL). `MANIFEST.json` shape (D4):
`{"npm_version","catalog_sha256","canned_sha256","surfaces":[{name,cols,
rows,sha256} x17]}`.

**Step 1 — failing test** (`internal/tui/parity_fixture_test.go`):

```go
// parity_fixture_test.go — the S8.2 pin guards (root principle 3): the
// upstream pty-capture fixtures + the shared parity constants are the
// S8.3 sweep's contract — every surface has a fixture whose sha256
// matches MANIFEST.json, the catalog/canned pins match their manifest
// entries, and the yolo-side Go canned constants (parity_test.go, S8.3)
// agree with the shared canned.json the S8.1 mock is pinned to.
package tui

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const paritySurfaceCount = 17

// paritySurfaceNames is the frozen D2 list (it must equal the capture's
// SURFACES names and the MANIFEST surface names).
var paritySurfaceNames = []string{
	"home", "session-text", "session-tool", "palette", "help", "model",
	"agent", "theme", "session-list", "session-rename", "session-delete",
	"status", "which-key", "sidebar", "prompt-slash", "prompt-mention",
	"epilogue",
}

func sha256hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestParityFixturesPinned(t *testing.T) {
	man, err := os.ReadFile(filepath.Join("testdata", "parity", "upstream", "MANIFEST.json"))
	if err != nil {
		t.Fatalf("read MANIFEST.json: %v (run the S8.2 capture first: just parity-capture)", err)
	}
	var m struct {
		NPMVersion    string `json:"npm_version"`
		CatalogSHA256 string `json:"catalog_sha256"`
		CannedSHA256  string `json:"canned_sha256"`
		Surfaces      []struct {
			Name   string `json:"name"`
			Cols   int    `json:"cols"`
			Rows   int    `json:"rows"`
			SHA256 string `json:"sha256"`
		} `json:"surfaces"`
	}
	if err := json.Unmarshal(man, &m); err != nil {
		t.Fatalf("MANIFEST.json: %v", err)
	}
	if m.NPMVersion != "1.18.18" {
		t.Fatalf("npm_version = %q, want 1.18.18", m.NPMVersion)
	}
	if len(m.Surfaces) != paritySurfaceCount {
		t.Fatalf("surface count = %d, want %d", len(m.Surfaces), paritySurfaceCount)
	}
	seen := map[string]bool{}
	for _, s := range m.Surfaces {
		seen[s.Name] = true
		raw, err := os.ReadFile(filepath.Join("testdata", "parity", "upstream", s.Name+".screen.json"))
		if err != nil {
			t.Fatalf("fixture %s: %v", s.Name, err)
		}
		if got := sha256hex(raw); got != s.SHA256 {
			t.Fatalf("fixture %s sha256 = %s, manifest says %s (re-baseline in the same commit as the change)", s.Name, got, s.SHA256)
		}
		var scr map[string]any
		if err := json.Unmarshal(raw, &scr); err != nil {
			t.Fatalf("fixture %s: not screen JSON: %v", s.Name, err)
		}
	}
	for _, name := range paritySurfaceNames {
		if !seen[name] {
			t.Fatalf("the manifest is missing the surface %q", name)
		}
	}
	catalog, err := os.ReadFile(filepath.Join("testdata", "parity", "catalog-pin.json"))
	if err != nil {
		t.Fatalf("catalog-pin.json: %v (the capture creates it)", err)
	}
	if got := sha256hex(catalog); got != m.CatalogSHA256 {
		t.Fatalf("catalog pin sha256 = %s, manifest says %s", got, m.CatalogSHA256)
	}
	canned, err := os.ReadFile(filepath.Join("testdata", "parity", "canned.json"))
	if err != nil {
		t.Fatalf("canned.json: %v", err)
	}
	if got := sha256hex(canned); got != m.CannedSHA256 {
		t.Fatalf("canned pin sha256 = %s, manifest says %s", got, m.CannedSHA256)
	}
}

// TestParityCannedConsistent pins the yolo-side Go canned constants
// (parity_test.go, S8.3) against the shared canned.json (the S8.1
// mock's source) — a drift would surface as a false parity gap (D1).
func TestParityCannedConsistent(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "parity", "canned.json"))
	if err != nil {
		t.Fatalf("canned.json: %v", err)
	}
	type ctool struct {
		Name string `json:"name"`
		Args string `json:"args"`
	}
	type ccanned struct {
		Prompt    string `json:"prompt"`
		Reply     string `json:"reply"`
		Tool      *ctool `json:"tool"`
		ToolReply string `json:"tool_reply"`
		Usage     struct {
			Input  int `json:"input"`
			Output int `json:"output"`
		} `json:"usage"`
	}
	var b struct {
		Text ccanned `json:"text"`
		Tool ccanned `json:"tool"`
		Todo ccanned `json:"todo"`
	}
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if b.Text.Prompt != parityPromptText || b.Text.Reply != parityReplyText ||
		b.Text.Usage.Input != 12 || b.Text.Usage.Output != 40 {
		t.Fatalf("text turn drifted: %+v", b.Text)
	}
	if b.Tool.Prompt != parityPromptTool || b.Tool.Tool == nil ||
		b.Tool.Tool.Name != "bash" || b.Tool.Tool.Args != parityArgsTool ||
		b.Tool.ToolReply != parityReplyTool {
		t.Fatalf("tool turn drifted: %+v", b.Tool)
	}
	if b.Todo.Prompt != parityPromptTodo || b.Todo.Tool == nil ||
		b.Todo.Tool.Name != "todowrite" || b.Todo.Tool.Args != parityArgsTodo ||
		b.Todo.ToolReply != parityReplyTodo {
		t.Fatalf("todo turn drifted: %+v", b.Todo)
	}
}

// The yolo-side canned constants (D1 — pinned against canned.json by
// TestParityCannedConsistent; the S8.3 fake driver scripts from them).
const (
	parityPromptText = "say hi to the mock"
	parityReplyText  = "## Heading\n\nSome **bold** and `inline code` text.\n\n- one\n- two\n\n> a quote\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n[link](https://example.com)\n\n```js\nconst x = 1;\n```\n\n你好 world\n\nDone."

	parityPromptTool = "run the parity check"
	parityArgsTool   = `{"command":"echo parity-ok"}`
	parityReplyTool  = "The check printed parity-ok.\n"

	parityPromptTodo = "add two todos"
	parityArgsTodo   = `{"todos":[{"content":"first item","status":"in_progress"},{"content":"second item","status":"pending"}]}`
	parityReplyTodo  = "Todos updated.\n"
)
```

**Step 2 — confirm FAIL:** `go test ./internal/tui/ -run
'TestParityFixturesPinned'` → FAIL: `read MANIFEST.json: no such file or
directory (run the S8.2 capture first: just parity-capture)`.
(`TestParityCannedConsistent` is GREEN from S8.2 onward: the `parity_*`
constants ship in this same file, and `canned.json` already exists from
S8.1 — the S8.3 dump test consumes the constants, it does not declare
them.)

**Step 3 — minimal implementation:**

The constant block — appended to `parity_fixture_test.go` (the yolo-side
canned constants; S8.3's `parity_test.go` uses them):

```go
// The yolo-side canned constants (D1 — pinned against canned.json by
// TestParityCannedConsistent; the S8.3 fake driver scripts from them).
const (
	parityPromptText = "say hi to the mock"
	parityReplyText  = "## Heading\n\nSome **bold** and `inline code` text.\n\n- one\n- two\n\n> a quote\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n[link](https://example.com)\n\n```js\nconst x = 1;\n```\n\n你好 world\n\nDone."

	parityPromptTool = "run the parity check"
	parityArgsTool   = `{"command":"echo parity-ok"}`
	parityReplyTool  = "The check printed parity-ok.\n"

	parityPromptTodo = "add two todos"
	parityArgsTodo   = `{"todos":[{"content":"first item","status":"in_progress"},{"content":"second item","status":"pending"}]}`
	parityReplyTodo  = "Todos updated.\n"
)
```

`scripts/parity/normalize.py` (the D3 shared normalizer):

```python
#!/usr/bin/env python3
"""normalize.py — the S8.2/S8.3 shared normalizer (spec §7.3, D3).

Replays a raw terminal byte stream (the upstream pty capture or the yolo
teatest dump) into ONE canonical screen JSON:

    {"cols": W, "rows": H,
     "cells": {"<row>:<col>": {"t": char, "fg": color, "bg": color,
                                "b": true}}}

Replay model: cursor positioning (CUP H/F), SGR (0, 1, 30-37/90-97,
40-47/100-107, 38;2;r;g;b / 48;2;r;g;b, 38;5;n / 48;5;n), printable
characters (UTF-8, wide runes take 2 columns, autowrap at the right
edge), CR/LF/VT/FF, TAB. The stream is replayed IN ORDER so the last
repaint of a cell wins — both TUIs (opentui, bubbletea) repaint with
absolute cursor positioning, so the result is the true final screen.

Volatile bits are masked to fixed tokens BEFORE replay (D5): OSC 12/0/22
(cursor color / window+icon titles / OSC 22), synchronized output
([>4;...m / [<4m), ses_ session ids, ISO-8601 timestamps, and the fixed
scratch path prefix /tmp/opencode-parity. Truecolor and 256-color values
are kept VERBATIM — the color-space difference (upstream 24-bit vs the
yolo ANSI256 teatest pin, deviation 125) is a parity finding for S8.4,
not normalizer noise.
"""
import re
import unicodedata

_MASKS = [
    (re.compile(rb"\x1b\]12;[^\x07\x1b]*"), b""),
    (re.compile(rb"\x1b\]0;[^\x07\x1b]*"), b""),
    (re.compile(rb"\x1b\]22;[^\x07\x1b]*"), b""),
    (re.compile(rb"\x1b\[>4;[0-9]*m"), b""),
    (re.compile(rb"\x1b\[<4m"), b""),
    (re.compile(rb"ses_[A-Za-z0-9]{10,}"), b"<SES>"),
    (re.compile(rb"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z"), b"<TS>"),
    (re.compile(rb"/tmp/opencode-parity"), b"<SCRATCH>"),
]

_CSI = re.compile(rb"\x1b\[([0-9;?<]*)([A-Za-z])")

_FG_BASIC = {c: "ansi:%d" % (30 + c) for c in range(8)}
_BG_BASIC = {c: "ansi:%d" % (40 + c) for c in range(8)}
_FG_BRIGHT = {c: "ansi:%d" % (90 + c) for c in range(8)}
_BG_BRIGHT = {c: "ansi:%d" % (100 + c) for c in range(8)}


def mask(data: bytes) -> bytes:
    """Apply the volatile-bit masks (D3)."""
    for rx, sub in _MASKS:
        data = rx.sub(sub, data)
    return data


def screen(data: bytes, cols: int, rows: int) -> dict:
    """Replay the raw stream into the final cols x rows cell grid."""
    data = mask(data)
    grid = [[None] * cols for _ in range(rows)]
    r = c = 0
    fg = bg = None
    bold = False
    i, n = 0, len(data)
    while i < n:
        b = data[i]
        if b == 0x1B:
            m = _CSI.match(data, i)
            if m:
                params, fin = m.group(1), m.group(2)
                i = m.end()
                if fin in (b"H", b"F"):
                    ps = [p for p in params.split(b";") if p != b""]
                    if len(ps) >= 2:
                        r = max(0, min(rows - 1, int(ps[0]) - 1))
                        c = max(0, min(cols - 1, int(ps[1]) - 1))
                elif fin == b"m":
                    ps = [p for p in params.split(b";") if p != b""]
                    if not ps:
                        ps = [b"0"]
                    j = 0
                    while j < len(ps):
                        code = int(ps[j])
                        if code == 0:
                            fg = bg = None
                            bold = False
                            j += 1
                        elif code == 1:
                            bold = True
                            j += 1
                        elif code in (38, 48) and j + 1 < len(ps):
                            kind, adv = None, 1
                            if ps[j + 1] == b"2" and j + 4 < len(ps):
                                kind = "rgb:#%02x%02x%02x" % (
                                    int(ps[j + 2]), int(ps[j + 3]), int(ps[j + 4]))
                                adv = 5
                            elif ps[j + 1] == b"5" and j + 2 < len(ps):
                                kind = "256:" + ps[j + 2].decode("ascii")
                                adv = 3
                            if kind is not None:
                                if code == 38:
                                    fg = kind
                                else:
                                    bg = kind
                            j += adv
                        elif 30 <= code <= 37:
                            fg = _FG_BASIC[code - 30]
                            j += 1
                        elif 90 <= code <= 97:
                            fg = _FG_BRIGHT[code - 90]
                            j += 1
                        elif 40 <= code <= 47:
                            bg = _BG_BASIC[code - 40]
                            j += 1
                        elif 100 <= code <= 107:
                            bg = _BG_BRIGHT[code - 100]
                            j += 1
                        else:
                            j += 1
                continue
            if data[i:i + 2] == b"\x1b]":
                # an OSC the masks did not claim: skip to the terminator.
                j = i + 2
                while j < n and data[j] not in (0x07, 0x1B):
                    j += 1
                i = (j + 1) if j < n else i + 1
                continue
            i += 1
            continue
        if b == 0x0D:
            c = 0
        elif b in (0x0A, 0x0B, 0x0C):
            r = min(rows - 1, r + 1)
        elif b == 0x09:
            c = min(cols - 1, (c // 8 + 1) * 8)
        elif 0x20 <= b <= 0x7E:
            grid[r][c] = {"t": bytes([b]).decode("ascii"), "fg": fg, "bg": bg, "bold": bold}
            c += 1
            if c >= cols:
                c = 0
                r = min(rows - 1, r + 1)
        elif b >= 0x80:
            ln = 4 if b >= 0xF0 else 3 if b >= 0xE0 else 2 if b >= 0xC0 else 1
            seq = data[i:i + ln]
            if len(seq) < ln:
                i += 1
                continue
            try:
                s = seq.decode("utf-8")
            except UnicodeDecodeError:
                i += 1
                continue
            grid[r][c] = {"t": s, "fg": fg, "bg": bg, "bold": bold}
            c += 2 if unicodedata.east_asian_width(s) in ("W", "F") else 1
            if c >= cols:
                c = 0
                r = min(rows - 1, r + 1)
            i += ln
            continue
        else:
            pass  # the other control bytes: ignored
        i += 1
    cells = {}
    for rr in range(rows):
        for cc in range(cols):
            cell = grid[rr][cc]
            if cell is None:
                continue
            if cell["t"] == " " and cell["fg"] is None and cell["bg"] is None and not cell["bold"]:
                continue
            out = {"t": cell["t"]}
            if cell["fg"] is not None:
                out["fg"] = cell["fg"]
            if cell["bg"] is not None:
                out["bg"] = cell["bg"]
            if cell["bold"]:
                out["b"] = True
            cells["%d:%d" % (rr, cc)] = out
    return {"cols": cols, "rows": rows, "cells": cells}
```

`scripts/parity/capture.py` (the D2/D5 driver):

```python
#!/usr/bin/env python3
"""capture.py — the S8.2 upstream pty-capture driver (spec §7.3).

ON-DEMAND, user-run, NEVER CI (the root e2e-live.sh pattern — the entry
is capture.sh / `just parity-capture`). Drives the npm
opencode-ai@1.18.18 TUI (the Bun single-file binary; the core server runs
in-process in a worker) in a pty against the S8.1 mock OpenAI-compatible
SSE server, per surface. Determinism (D5): a FRESH hermetic HOME +
project scratch per surface (fixed prefix /tmp/opencode-parity — masked
by normalize.py), the pinned catalog (OPENCODE_MODELS_PATH),
OPENCODE_DISABLE_MODELS_FETCH/AUTOUPDATE/MOUSE=1, --pure --auto, NO
--print-logs (it writes into the pty — verified at detail time), fixed
terminal sizes, the volatile bits masked, and a DOUBLE capture per
surface (the two normalized screens must be byte-identical or the run
fails). Writes the pinned fixtures + MANIFEST.json (D4) to
internal/tui/testdata/parity/upstream/ and prints the re-baselined
fixture list (the executor commits them in the same commit — root
principle 3).
"""
import fcntl
import hashlib
import json
import os
import pty
import re
import select
import signal
import struct
import subprocess
import sys
import termios
import time

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(os.path.dirname(HERE))
sys.path.insert(0, HERE)
import normalize  # noqa: E402

TESTDATA = os.path.join(REPO, "internal", "tui", "testdata", "parity")
UPSTREAM = os.path.join(TESTDATA, "upstream")
CATALOG = os.path.join(TESTDATA, "catalog-pin.json")
CANNED = os.path.join(TESTDATA, "canned.json")
SCRATCH = "/tmp/opencode-parity"
BIN = os.path.join(SCRATCH, "node", "node_modules", "opencode-ai", "bin", "opencode.exe")
MOCK = os.path.join(SCRATCH, "mock")
NPM_VERSION = "1.18.18"

# The D2 17-surface table (frozen at detail time): (name, (cols, rows),
# canned turn, key script). A step is ("wait", secs) | ("keys", bytes,
# label). The turn step expands to: type the canned prompt, enter, wait.
TURN = [
    ("keys", b"__PROMPT__", "prompt"),
    ("wait", 0.5),
    ("keys", b"\r", "enter"),
    ("wait", 8.0),
]


def _turn():
    return [list(s) for s in TURN]


SURFACES = [
    ("home", (80, 24), "text", [("wait", 8.0)]),
    ("session-text", (80, 24), "text", _turn()),
    ("session-tool", (80, 24), "tool", _turn()),
    ("palette", (80, 24), "text", _turn() + [("keys", b"\x10", "ctrl+p"), ("wait", 4.0)]),
    ("help", (80, 24), "text", _turn() + [
        ("keys", b"/help", "slash"), ("wait", 0.5), ("keys", b"\r", "enter"), ("wait", 4.0)]),
    ("model", (80, 24), "text", _turn() + [
        ("keys", b"\x18", "leader"), ("wait", 0.5), ("keys", b"m", "model_list"), ("wait", 4.0)]),
    ("agent", (80, 24), "text", _turn() + [
        ("keys", b"\x18", "leader"), ("wait", 0.5), ("keys", b"a", "agent_list"), ("wait", 4.0)]),
    ("theme", (80, 24), "text", _turn() + [
        ("keys", b"\x18", "leader"), ("wait", 0.5), ("keys", b"t", "theme_list"), ("wait", 4.0)]),
    ("session-list", (80, 24), "text", _turn() + [
        ("keys", b"\x18", "leader"), ("wait", 0.5), ("keys", b"l", "session_list"), ("wait", 4.0)]),
    ("session-rename", (80, 24), "text", _turn() + [
        ("keys", b"\x12", "ctrl+r"), ("wait", 4.0)]),
    ("session-delete", (80, 24), "text", _turn() + [
        ("keys", b"\x18", "leader"), ("wait", 0.5), ("keys", b"l", "session_list"),
        ("wait", 3.0), ("keys", b"\x04", "ctrl+d arm"), ("wait", 3.0)]),
    ("status", (80, 24), "text", _turn() + [
        ("keys", b"\x18", "leader"), ("wait", 0.5), ("keys", b"s", "status_view"), ("wait", 4.0)]),
    ("which-key", (80, 24), "text", [("wait", 8.0), ("keys", b"\x18", "leader held"), ("wait", 3.0)]),
    ("sidebar", (140, 30), "todo", _turn()),
    ("prompt-slash", (80, 24), "text", _turn() + [
        ("keys", b"/", "slash menu"), ("wait", 3.0)]),
    ("prompt-mention", (80, 24), "text", _turn() + [
        ("keys", b"@par", "mention"), ("wait", 3.0)]),
    ("epilogue", (80, 24), "text", _turn() + [
        ("keys", b"\x03", "ctrl+c exit"), ("wait", 4.0)]),
]

CANNED_PROMPTS = {}  # filled from canned.json at main()


def sha256(path):
    with open(path, "rb") as fh:
        return hashlib.sha256(fh.read()).hexdigest()


def env_for(rundir):
    home = os.path.join(rundir, "home")
    os.makedirs(home, exist_ok=True)
    e = dict(os.environ)
    e.update({
        "HOME": home,
        "XDG_DATA_HOME": os.path.join(home, ".local", "share"),
        "XDG_CACHE_HOME": os.path.join(home, ".cache"),
        "XDG_CONFIG_HOME": os.path.join(home, ".config"),
        "XDG_STATE_HOME": os.path.join(home, ".local", "state"),
        "OPENCODE_MODELS_PATH": CATALOG,
        "OPENCODE_DISABLE_MODELS_FETCH": "1",
        "OPENCODE_DISABLE_AUTOUPDATE": "1",
        "OPENCODE_DISABLE_MOUSE": "1",
        "TERM": "xterm-256color",
        "COLORTERM": "truecolor",
    })
    return e


def write_project(rundir, port):
    proj = os.path.join(rundir, "proj")
    os.makedirs(proj, exist_ok=True)
    for f in ("parity-a.txt", "parity-b.txt"):  # the mention-surface files
        with open(os.path.join(proj, f), "w") as fh:
            fh.write("x")
    cfg = {
        "$schema": "https://opencode.ai/config.json",
        "model": "mockllm/canned",
        # the tool/todo surfaces auto-approve (the yolo side seeds the
        # equivalent config permission rules — both sides reach the
        # second turn without a permission overlay).
        "permission": {"bash": "allow", "todowrite": "allow"},
        "provider": {
            "mockllm": {
                "name": "Mock LLM",
                "npm": "@ai-sdk/openai-compatible",
                "options": {"baseURL": "http://127.0.0.1:%d/v1" % port, "apiKey": "parity"},
                "models": {"canned": {"name": "Canned"}},
            },
        },
    }
    with open(os.path.join(proj, "opencode.json"), "w") as fh:
        json.dump(cfg, fh)
    return proj


def start_mock(turn):
    p = subprocess.Popen([MOCK, "-addr", "127.0.0.1:0", "-turn", turn, "-canned", CANNED],
                         stdout=subprocess.PIPE)
    line = p.stdout.readline()
    m = re.match(rb"MOCK_PORT=(\d+)", line.strip())
    if not m:
        p.kill()
        raise SystemExit("mock handshake failed: %r" % line)
    return p, int(m.group(1))


def set_winsize(fd, rows, cols):
    fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", rows, cols, 0, 0))


def pump(fd, secs, raw):
    end = time.time() + secs
    while time.time() < end:
        r, _, _ = select.select([fd], [], [], 0.05)
        if fd in r:
            try:
                data = os.read(fd, 65536)
            except OSError:
                break
            if not data:
                break
            raw.extend(data)


def reap(pid, fd):
    try:
        os.kill(pid, signal.SIGKILL)
    except ProcessLookupError:
        pass
    try:
        os.waitpid(pid, 0)
    except ChildProcessError:
        pass
    try:
        os.close(fd)
    except OSError:
        pass


def run_pty(cols, rows, steps, rundir):
    proj = os.path.join(rundir, "proj")
    pid, fd = pty.fork()
    if pid == 0:
        try:
            os.chdir(proj)
            os.execve(BIN, ["opencode", "--pure", "--auto"], env_for(rundir))
        except Exception:
            os._exit(127)
    set_winsize(fd, rows, cols)
    raw = bytearray()
    for step in steps:
        if step[0] == "wait":
            pump(fd, step[1], raw)
        elif step[0] == "keys":
            os.write(fd, step[1])
    pump(fd, 2.0, raw)  # the final settle
    reap(pid, fd)
    return bytes(raw)


def run_once(cols, rows, steps, turn):
    rundir = os.path.join(SCRATCH, "run")
    os.makedirs(rundir, exist_ok=True)
    mock, port = start_mock(turn)
    try:
        write_project(rundir, port)
        raw = run_pty(cols, rows, steps, rundir)
    finally:
        mock.kill()
        mock.wait()
    return raw


def expand(steps, prompt):
    """Substitute the canned prompt into the __PROMPT__ placeholder."""
    return [s if s[0] != "keys" or s[1] != b"__PROMPT__" else ["keys", prompt.encode(), "prompt"]
            for s in steps]


def capture_surface(name, size, steps, turn, prompt):
    """The D5 double-run: two fresh pty runs must normalize identically."""
    st = expand(steps, prompt)
    a = normalize.screen(run_once(size[0], size[1], st, turn), size[0], size[1])
    b = normalize.screen(run_once(size[0], size[1], st, turn), size[0], size[1])
    if a != b:
        ka = set(a["cells"])
        kb = set(b["cells"])
        diffs = [k for k in sorted(ka & kb) if a["cells"][k] != b["cells"][k]]
        raise SystemExit("FAIL: %s — the double-capture screens differ (first: %s)"
                         % (name, diffs[:5] or (ka ^ kb)))
    return a


def main():
    for req in (CATALOG, CANNED, MOCK, BIN):
        if not os.path.exists(req):
            raise SystemExit("FAIL: missing %s (capture.sh prepares the runtime)" % req)
    book = json.load(open(CANNED))
    os.makedirs(UPSTREAM, exist_ok=True)
    man_path = os.path.join(UPSTREAM, "MANIFEST.json")
    if os.path.exists(man_path):
        man = json.load(open(man_path))
    else:
        man = {"npm_version": NPM_VERSION, "surfaces": []}
    changed = []
    for name, size, turn, steps in SURFACES:
        prompt = book[turn]["prompt"]
        screen = capture_surface(name, size, steps, turn, prompt)
        blob = json.dumps(screen, sort_keys=True, separators=(",", ":")).encode()
        path = os.path.join(UPSTREAM, "%s.screen.json" % name)
        old = open(path, "rb").read() if os.path.exists(path) else None
        if old != blob:
            with open(path, "wb") as fh:
                fh.write(blob)
            changed.append(name)
            print("   re-baselined: %s" % name)
        else:
            print("   stable:       %s" % name)
        man["surfaces"] = [s for s in man.get("surfaces", []) if s["name"] != name]
        man["surfaces"].append({"name": name, "cols": size[0], "rows": size[1],
                                "sha256": hashlib.sha256(blob).hexdigest()})
    man["catalog_sha256"] = sha256(CATALOG)
    man["canned_sha256"] = sha256(CANNED)
    man["npm_version"] = NPM_VERSION
    with open(man_path, "w") as fh:
        json.dump(man, fh, sort_keys=True, indent=1)
        fh.write("\n")
    if changed:
        print("PASS: %d fixtures re-baselined: %s" % (len(changed), ", ".join(changed)))
    else:
        print("PASS: all 17 fixtures stable")


if __name__ == "__main__":
    main()
```

`scripts/parity/capture.sh` (the entry — the e2e-live.sh pattern):

```bash
#!/usr/bin/env bash
# capture.sh — the S8.2 upstream pty-capture (spec §7.3). ON-DEMAND,
# user-run, NEVER CI (the root e2e-live.sh pattern).
#
# Usage: just parity-capture   (or: bash scripts/parity/capture.sh)
#
# Prereqs: node + npm (the opencode-ai@1.18.18 capture runtime), python3
# (stdlib only), go (the S8.1 mock build). Installs the npm package into a
# scratch dir (first run), builds the mock, creates the catalog pin on
# first run (curl + browser UA — the CDN 403s python-urllib), then
# re-captures all 17 surfaces. The re-baselined fixtures are printed —
# commit them in the same commit (root principle 3). Exits 0 with PASS,
# 1 with FAIL.

set -uo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SCRATCH="/tmp/opencode-parity"
CATALOG_URL="https://models.opencode.ai/api.json"
UA="Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
FAIL=0

step() { printf '\n== %s\n' "$1"; }
ok()   { printf '   ok: %s\n' "$1"; }
bad()  { printf '   FAIL: %s\n' "$1"; FAIL=1; }

command -v python3 >/dev/null || { bad "python3 not found"; exit 1; }
command -v node >/dev/null || { bad "node not found"; exit 1; }
command -v npm >/dev/null || { bad "npm not found"; exit 1; }
command -v go >/dev/null || { bad "go not found"; exit 1; }

step "scratch + npm runtime (opencode-ai@1.18.18)"
rm -rf "$SCRATCH/node"
mkdir -p "$SCRATCH/node"
(cd "$SCRATCH/node" && npm init -y >/dev/null && npm install opencode-ai@1.18.18 --loglevel=error) \
  && ok "opencode-ai@1.18.18 installed" || bad "npm install failed"
[ "$FAIL" -eq 0 ] || exit 1

step "mock binary"
(cd "$ROOT" && go build -o "$SCRATCH/mock" ./scripts/parity/mock) \
  && ok "mock built" || { bad "go build failed"; exit 1; }

step "catalog pin"
CATPIN="$ROOT/internal/tui/testdata/parity/catalog-pin.json"
mkdir -p "$(dirname "$CATPIN")"
if [ ! -f "$CATPIN" ]; then
  curl -fsSL -A "$UA" "$CATALOG_URL" -o "$SCRATCH/api.json" \
    && node -e "const fs=require('fs');const c=JSON.parse(fs.readFileSync(process.argv[1],'utf8'));fs.writeFileSync(process.argv[2],JSON.stringify({openai:c.openai}))" "$SCRATCH/api.json" "$CATPIN" \
    && ok "catalog pin created (reduced {openai} snapshot)" || { bad "catalog fetch/trim failed"; exit 1; }
else
  ok "catalog pin present (re-fetch manually to re-baseline it)"
fi

step "capture (17 surfaces, double-run determinism)"
(cd "$ROOT" && python3 scripts/parity/capture.py)
rc=$?
[ $rc -eq 0 ] && ok "fixtures + MANIFEST.json written" || bad "capture.py exit $rc"

exit $FAIL
```

`justfile` (appended — the existing targets keep their position):

```
# Parity capture (S8.2) — on-demand, user-run, NEVER CI: re-captures the
# 17 upstream pty fixtures + MANIFEST.json. See scripts/parity/capture.sh.
parity-capture:
    bash scripts/parity/capture.sh
```

**Step 4 — gate:** `go vet ./... && go test ./...` green (incl.
`TestParityFixturesPinned` — the capture has run, the fixtures are
pinned — and `TestParityCannedConsistent`) + `gofmt -l .` empty. The
capture itself is the user-run step: `just parity-capture` exits 0 with
PASS (any re-baselined fixture is committed in Step 5's commit).

**Step 5 — commit** the pinned message `test: upstream pty-capture script
+ fixtures` (staging `scripts/parity/capture.sh`,
`scripts/parity/capture.py`, `scripts/parity/normalize.py`,
`internal/tui/testdata/parity/catalog-pin.json`,
`internal/tui/testdata/parity/upstream/` (the 17 fixtures +
`MANIFEST.json`), `internal/tui/parity_fixture_test.go`, `justfile`),
then `bd close yolo-oae.9.3 --reason "S8.2 done: the capture runtime
(npm opencode-ai@1.18.18 scratch install, the hermetic per-surface pty
driver, the 17-surface key scripts, the D3 shared normalizer, the D5
double-run determinism gate) + the pinned fixtures (17 normalized
screens + MANIFEST.json + the catalog pin) + the pin guards
(TestParityFixturesPinned / TestParityCannedConsistent)" --json`.

### Task S8.3: Parity diff sweep: yolo teatest renders vs upstream captures, per-surface (bead `yolo-oae.9.3`, expected id `yolo-oae.9.4`)

**Files:** new `scripts/parity/sweep.py` (the D7 sweep), new
`internal/tui/parity_test.go` (the D6 yolo dump test), modify `justfile`
(the `parity-sweep` target), new
`docs/superpowers/plans/2026-08-24-opencode-tui-parity/
parity-sweep-report.md` (written by the sweep — committed with the task).

**Interfaces:** `TestParityDump` (gated on `YOLO_PARITY_DUMP` —
`t.Skip` when unset, so the CI gate never renders the set), the
`paritySurface{name, width, height, turn, drive}` table (the D2 17
surfaces — the names must equal `paritySurfaceNames`), the raw dumps at
`$YOLO_PARITY_DUMP/yolo/<name>.raw`; `sweep.py` — exit 0 on a COMPLETED
sweep (the GAPS lines are informational, D7), exit 1 on a mechanical
failure.

**Step 1 — failing test** (`internal/tui/parity_test.go` — the dump
test; its RED state is the sweep's "0/17 surfaces rendered" — the test
itself is green-by-construction once it compiles, so the RED confirmation
is the SWEEP's before-state: `just parity-sweep` FAILs (exit 1) with
"the parity dump produced no surfaces" because `TestParityDump` does not
exist yet):

```go
// parity_test.go — the S8.3 yolo-side render dump (the D6 design):
// TestParityDump renders each of the 17 parity surfaces (the frozen D2
// list) through the real stack (testutil + the fake driver scripted from
// the shared canned constants + the theme engine under TTY_FORCE) and
// writes the FULL raw teatest stream to $YOLO_PARITY_DUMP/yolo/<name>.raw
// for the sweep normalizer. Gated on the env var (t.Skip when unset) so
// the CI gate never renders the set — the sweep is user-run, never CI
// (the root e2e-live.sh pattern).
package tui

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/kido5217/yolo/internal/llm"
	"github.com/kido5217/yolo/internal/llm/fake"
	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// paritySurface is one D2 surface (the name = the upstream fixture file
// name under internal/tui/testdata/parity/upstream/).
type paritySurface struct {
	name   string
	width  int
	height int
	turn   string // "text" | "tool" | "todo" — the canned book key
	home   bool   // run on the home route (no session turn)
	// steps drive the key script (the home settle precedes them).
	steps []parityStep
}

type parityStep struct {
	keys []tea.KeyPressMsg
	text string // tm.Type (plain prompt text)
	cond func([]byte) bool
	d    time.Duration
}

func pressCtrlD() tea.KeyPressMsg { return tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl} }
func pressCtrlR() tea.KeyPressMsg { return tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl} }
func pressCtrlC() tea.KeyPressMsg { return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl} }

func leaderCont(k rune) []tea.KeyPressMsg {
	return []tea.KeyPressMsg{pressLeader(), press(k)}
}

// paritySurfaces is the frozen D2 table (the yolo-side scripts; the
// upstream key equivalents live in scripts/parity/capture.py SURFACES).
func paritySurfaces() []paritySurface {
	// turn drives the canned prompt + enter + settle on the final
	// transcript line (the yolo side needs `n` for the new session first
	// — the upstream side's prompt is ready on home; the scripts are
	// per-side, the normalizer compares the final screens).
	turn := func(p promptTurn) []parityStep {
		return []parityStep{
			{keys: []tea.KeyPressMsg{press('n')}, cond: hasLine("esc abort/back"), d: 10 * time.Second},
			{text: p.prompt, cond: hasLine(p.prompt), d: 10 * time.Second},
			{keys: []tea.KeyPressMsg{press(tea.KeyEnter)}, cond: p.settle, d: 15 * time.Second},
		}
	}
	textTurn := promptTurn{prompt: parityPromptText, settle: hasLine("Done.")}
	toolTurn := promptTurn{prompt: parityPromptTool, settle: hasLine("parity-ok")}
	todoTurn := promptTurn{prompt: parityPromptTodo, settle: hasLine("first item")}
	return []paritySurface{
		{name: "home", width: 80, height: 24, turn: "text", home: true},
		{name: "session-text", width: 80, height: 24, turn: "text", steps: turn(textTurn)},
		{name: "session-tool", width: 80, height: 24, turn: "tool", steps: turn(toolTurn)},
		{name: "palette", width: 80, height: 24, turn: "text", steps: append(
			turn(textTurn), parityStep{keys: []tea.KeyPressMsg{pressCtrlP()}, cond: hasLine("Commands"), d: 10 * time.Second})},
		{name: "help", width: 80, height: 24, turn: "text", steps: append(
			turn(textTurn),
			parityStep{text: "/help", cond: hasLine("/help"), d: 10 * time.Second},
			parityStep{keys: []tea.KeyPressMsg{press(tea.KeyEnter)}, cond: hasLine("Help"), d: 10 * time.Second})},
		{name: "model", width: 80, height: 24, turn: "text", steps: append(
			turn(textTurn), parityStep{keys: leaderCont('m'), cond: hasLines("Model", "Qwen"), d: 15 * time.Second})},
		{name: "agent", width: 80, height: 24, turn: "text", steps: append(
			turn(textTurn), parityStep{keys: leaderCont('a'), cond: hasLines("Agents", "build"), d: 10 * time.Second})},
		{name: "theme", width: 80, height: 24, turn: "text", steps: append(
			turn(textTurn), parityStep{keys: leaderCont('t'), cond: hasLine("Themes"), d: 10 * time.Second})},
		{name: "session-list", width: 80, height: 24, turn: "text", steps: append(
			turn(textTurn), parityStep{keys: leaderCont('l'), cond: hasLine("Sessions"), d: 10 * time.Second})},
		{name: "session-rename", width: 80, height: 24, turn: "text", steps: append(
			turn(textTurn), parityStep{keys: []tea.KeyPressMsg{pressCtrlR()}, cond: hasLine("Rename Session"), d: 10 * time.Second})},
		{name: "session-delete", width: 80, height: 24, turn: "text", steps: append(
			turn(textTurn),
			parityStep{keys: leaderCont('l'), cond: hasLine("Sessions"), d: 10 * time.Second},
			parityStep{keys: []tea.KeyPressMsg{pressCtrlD()}, cond: hasLine("Press ctrl+d again to confirm"), d: 10 * time.Second})},
		{name: "status", width: 80, height: 24, turn: "text", steps: append(
			turn(textTurn), parityStep{keys: leaderCont('s'), cond: hasLine("Status"), d: 10 * time.Second})},
		{name: "which-key", width: 80, height: 24, turn: "text", home: true, steps: []parityStep{
			{keys: []tea.KeyPressMsg{pressLeader()}, cond: hasLines("Model", "Session"), d: 10 * time.Second},
		}},
		{name: "sidebar", width: 140, height: 30, turn: "todo", steps: turn(todoTurn)},
		{name: "prompt-slash", width: 80, height: 24, turn: "text", steps: append(
			turn(textTurn), parityStep{text: "/", cond: hasLine("/help"), d: 10 * time.Second})},
		{name: "prompt-mention", width: 80, height: 24, turn: "text", steps: append(
			turn(textTurn), parityStep{text: "@par", cond: hasLine("parity-a.txt"), d: 10 * time.Second})},
		{name: "epilogue", width: 80, height: 24, turn: "text", steps: append(
			turn(textTurn), parityStep{keys: []tea.KeyPressMsg{pressCtrlC()}, cond: func([]byte) bool { return true }, d: 5 * time.Second})},
	}
}

type promptTurn struct {
	prompt string
	settle func([]byte) bool
}

func TestParityDump(t *testing.T) {
	dir := os.Getenv("YOLO_PARITY_DUMP")
	if dir == "" {
		t.Skip("YOLO_PARITY_DUMP unset — the parity sweep is user-run, never CI")
	}
	if err := os.MkdirAll(filepath.Join(dir, "yolo"), 0o755); err != nil {
		t.Fatalf("parity dump: mkdir: %v", err)
	}
	for _, s := range paritySurfaces() {
		s := s
		t.Run(s.name, func(t *testing.T) { dumpSurface(t, dir, s) })
	}
}

func parityDriver(turn string) *fake.Driver {
	switch turn {
	case "tool":
		return fake.New(
			fake.Turn{Parts: []llm.Part{{Kind: "tool", Name: "bash", CallID: "call_canned1",
				Args: json.RawMessage(parityArgsTool), Text: parityArgsTool,
				Usage: &llm.Usage{Input: 12, Output: 40}, Finish: "tool_calls"}}},
			fake.Turn{Parts: []llm.Part{{Kind: "text", Text: parityReplyTool,
				Usage: &llm.Usage{Input: 12, Output: 40}, Finish: "stop"}}},
		)
	case "todo":
		return fake.New(
			fake.Turn{Parts: []llm.Part{{Kind: "tool", Name: "todowrite", CallID: "call_canned2",
				Args: json.RawMessage(parityArgsTodo), Text: parityArgsTodo,
				Usage: &llm.Usage{Input: 12, Output: 40}, Finish: "tool_calls"}}},
			fake.Turn{Parts: []llm.Part{{Kind: "text", Text: parityReplyTodo,
				Usage: &llm.Usage{Input: 12, Output: 40}, Finish: "stop"}}},
		)
	default:
		return fake.New(
			fake.Turn{Parts: []llm.Part{{Kind: "text", Text: parityReplyText,
				Usage: &llm.Usage{Input: 12, Output: 40}, Finish: "stop"}}},
		)
	}
}

// parityConfig is the yolo-side config (the scope config): it defines the
// providers the model dialog lists — "openai" seeded from the pinned
// catalog (the same 47 models the upstream catalog pin carries) + the
// "mockllm" custom provider (the yolo referent of the upstream
// opencode.json provider entry). The LLM itself is the in-process fake
// driver (the config never reaches the network).
func parityConfig(t *testing.T) *protocol.Config {
	t.Helper()
	catalog, err := os.ReadFile(filepath.Join("testdata", "parity", "catalog-pin.json"))
	if err != nil {
		t.Fatalf("parity config: catalog pin: %v", err)
	}
	var c struct {
		OpenAI struct {
			Models map[string]struct {
				Name string `json:"name"`
			} `json:"models"`
		} `json:"openai"`
	}
	if err := json.Unmarshal(catalog, &c); err != nil {
		t.Fatalf("parity config: catalog decode: %v", err)
	}
	models := map[string]any{}
	for id, m := range c.OpenAI.Models {
		models[id] = map[string]any{"name": m.Name}
	}
	return &protocol.Config{
		// the tool/todo surfaces auto-approve (the upstream side runs the
		// equivalent --auto — both sides reach the second turn without a
		// permission overlay).
		Permission: map[string]any{"bash": "allow", "todowrite": "allow"},
		Provider: map[string]protocol.ProviderConfig{
			"openai": {BaseURL: "https://api.openai.com/v1", Models: models},
			"mockllm": {BaseURL: "http://127.0.0.1:0/v1",
				Models: map[string]any{"canned": map[string]any{"name": "Canned"}}},
		},
	}
}

// parityApp boots the theme engine + the App (the session_markdown_test.go
// idiom).
func parityApp(t *testing.T, ts *testutil.TestServer) *App {
	t.Helper()
	dir := t.TempDir()
	e, err := theme.New(theme.EngineOptions{
		KVPath:        filepath.Join(dir, "kv.json"),
		GlobalYoloDir: dir,
		CWD:           dir,
		Palette:       func(context.Context) (theme.TerminalColors, bool) { return theme.TerminalColors{}, false },
	})
	if err != nil {
		t.Fatalf("theme.New: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })
	if err := e.Resolve(context.Background()); err != nil {
		t.Fatalf("theme.Resolve: %v", err)
	}
	c := client.New(ts.URL, ts.Dir)
	a := NewApp(c, store.State{}, "", e)
	t.Cleanup(a.Close)
	return a
}

// pumpUntil drains the teatest output buffer until cond matches the
// accumulated raw (or the deadline). io.ReadAll on the buffer is
// non-blocking (it consumes what is present, EOF on empty) — the loop
// cannot deadlock (the detail-pass finding: the teatest v2 Output
// semantics).
func pumpUntil(t *testing.T, tm *teatest.TestModel, raw []byte, cond func([]byte) bool, d time.Duration) []byte {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if b, _ := io.ReadAll(tm.Output()); len(b) > 0 {
			raw = append(raw, b...)
		}
		if cond(raw) {
			return raw
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("parity dump: the terminal condition was not met within %s (stream %d bytes)", d, len(raw))
	return nil
}

func appendPump(t *testing.T, tm *teatest.TestModel, raw []byte, d time.Duration) []byte {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if b, _ := io.ReadAll(tm.Output()); len(b) > 0 {
			raw = append(raw, b...)
		}
		time.Sleep(25 * time.Millisecond)
	}
	return raw
}

func drain(tm *teatest.TestModel) []byte {
	b, _ := io.ReadAll(tm.Output())
	return b
}

// dumpSurface boots ONE surface, drives its key script, and writes the
// full raw stream to <outDir>/yolo/<name>.raw.
func dumpSurface(t *testing.T, outDir string, s paritySurface) {
	t.Helper()
	ts := testutil.BootWithDriverConfig(t, parityDriver(s.turn), parityConfig(t))
	if s.name == "prompt-mention" {
		// the mention menu lists the session-dir files (the pinned
		// scratch pair — the upstream side has the same pair).
		for _, f := range []string{"parity-a.txt", "parity-b.txt"} {
			if err := os.WriteFile(filepath.Join(ts.Dir, f), []byte("x"), 0o644); err != nil {
				t.Fatalf("parity dump: %v", err)
			}
		}
	}
	a := parityApp(t, ts)
	tm := teatest.NewTestModel(t, a,
		teatest.WithInitialTermSize(s.width, s.height),
		teatest.WithProgramOptions(tea.WithEnvironment([]string{
			"TTY_FORCE=1", "TERM=xterm-256color",
		})),
	)
	var raw []byte
	raw = pumpUntil(t, tm, raw, hasLine("New session"), 15*time.Second) // the home settle
	for _, st := range s.steps {
		if st.text != "" {
			tm.Type(st.text)
		}
		for _, k := range st.keys {
			tm.Send(k)
		}
		if st.cond != nil && st.d > 0 {
			raw = pumpUntil(t, tm, raw, st.cond, st.d)
		}
	}
	raw = appendPump(t, tm, raw, 2*time.Second) // the final settle
	raw = append(raw, drain(tm)...)
	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(10*time.Second))
	raw = append(raw, drain(tm)...)
	if err := os.WriteFile(filepath.Join(outDir, "yolo", s.name+".raw"), raw, 0o644); err != nil {
		t.Fatalf("parity dump: write %s: %v", s.name, err)
	}
}
```

**Step 2 — confirm FAIL:** `just parity-sweep` → FAIL (exit 1):
`scripts/parity/sweep.py` does not exist yet (`python3: can't open file`)
— and the yolo half of the sweep (the `TestParityDump` dump test) is the
missing half: without it the sweep would render 0/17 surfaces. The module
gate itself stays green throughout (the dump test is env-gated — it
`t.Skip`s without `YOLO_PARITY_DUMP`).

**Step 3 — minimal implementation:**

`scripts/parity/sweep.py` (the D7 sweep):

```python
#!/usr/bin/env python3
"""sweep.py — the S8.3 parity diff sweep (spec §7.3, D7).

ON-DEMAND, user-run, NEVER CI (the root e2e-live.sh pattern — the entry
is `just parity-sweep`):
  1. renders the yolo side: YOLO_PARITY_DUMP=<tmp> go test -count=1
     -run ^TestParityDump$ ./internal/tui/ (the D6 dump test writes the
     17 raw streams to <tmp>/yolo/<name>.raw),
  2. normalizes BOTH sides with the shared normalize.py (the yolo raw
     streams; the upstream side is the pinned normalized fixture
     upstream/<name>.screen.json — D4),
  3. per-surface cell diff (t/fg/bg/b on the union of cell keys),
  4. writes docs/superpowers/plans/2026-08-24-opencode-tui-parity/
     parity-sweep-report.md (the MATCH / GAPS(n) table + the mismatch
     detail + the environment: the yolo HEAD sha, the manifest sha256,
     the npm version) and prints the summary.

Exit 0 on a COMPLETED sweep — the GAPS lines are INFORMATIONAL (S8.4
consumes the report and closes or logs every gap, D7); exit 1 on a
mechanical failure (the go test failure, a missing fixture, a crash).
"""
import hashlib
import json
import os
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(os.path.dirname(HERE))
sys.path.insert(0, HERE)
import normalize  # noqa: E402

TESTDATA = os.path.join(REPO, "internal", "tui", "testdata", "parity")
UPSTREAM = os.path.join(TESTDATA, "upstream")
MANIFEST = os.path.join(UPSTREAM, "MANIFEST.json")
REPORT = os.path.join(REPO, "docs", "superpowers", "plans",
                      "2026-08-24-opencode-tui-parity", "parity-sweep-report.md")


def fail(msg):
    print("FAIL: %s" % msg)
    sys.exit(1)


def main():
    if not os.path.exists(MANIFEST):
        fail("the upstream MANIFEST.json is missing (run `just parity-capture` first)")
    man = json.load(open(MANIFEST))
    dump = tempfile.mkdtemp(prefix="yolo-parity-")
    env = dict(os.environ, YOLO_PARITY_DUMP=dump)
    r = subprocess.run(["go", "test", "-count=1", "-run", "^TestParityDump$", "./internal/tui/"],
                       cwd=REPO, env=env, capture_output=True, text=True)
    if r.returncode != 0:
        print(r.stdout)
        print(r.stderr)
        fail("the yolo dump test failed (exit %d)" % r.returncode)
    head = subprocess.run(["git", "rev-parse", "HEAD"], cwd=REPO,
                          capture_output=True, text=True).stdout.strip()
    man_sha = hashlib.sha256(open(MANIFEST, "rb").read()).hexdigest()
    rows = []
    for s in man["surfaces"]:
        name = s["name"]
        yolo_path = os.path.join(dump, "yolo", name + ".raw")
        up_path = os.path.join(UPSTREAM, name + ".screen.json")
        if not os.path.exists(yolo_path):
            fail("the yolo dump is missing %s (TestParityDump did not render it)" % name)
        yolo = normalize.screen(open(yolo_path, "rb").read(), s["cols"], s["rows"])
        upstream = json.load(open(up_path))
        rows.append((name, s["cols"], s["rows"], diff_screens(upstream, yolo)))
    # the report
    lines = [
        "# Parity sweep report (S8.3)",
        "",
        "- yolo HEAD: `%s`" % head,
        "- fixture manifest sha256: `%s`" % man_sha,
        "- npm opencode-ai: %s" % man["npm_version"],
        "",
        "| surface | size | result |",
        "|---|---|---|",
    ]
    for name, cols, rows_, gaps in rows:
        lines.append("| %s | %dx%d | %s |" % (
            name, cols, rows_, "MATCH" if not gaps else "GAPS(%d)" % len(gaps)))
    lines.append("")
    for name, cols, rows_, gaps in rows:
        if not gaps:
            continue
        lines.append("## %s — %d gaps" % (name, len(gaps)))
        lines.append("")
        for g in gaps[:20]:
            lines.append("- cell %s %s: upstream=%r yolo=%r" % (
                g["cell"], g["field"], g["upstream"], g["yolo"]))
        if len(gaps) > 20:
            lines.append("- … %d more" % (len(gaps) - 20))
        lines.append("")
    os.makedirs(os.path.dirname(REPORT), exist_ok=True)
    with open(REPORT, "w") as fh:
        fh.write("\n".join(lines) + "\n")
    n_match = sum(1 for _, _, _, g in rows if not g)
    print("PASS: sweep complete — %d/%d surfaces MATCH, report at %s"
          % (n_match, len(rows), os.path.relpath(REPORT, REPO)))


def cellkey(k):
    r, c = k.split(":")
    return (int(r), int(c))


def diff_screens(a, b):
    """The per-cell diff (D7): t/fg/bg/b on the union of cell keys."""
    out = []
    keys = set(a.get("cells", {})) | set(b.get("cells", {}))
    for k in sorted(keys, key=cellkey):
        ca = a.get("cells", {}).get(k, {})
        cb = b.get("cells", {}).get(k, {})
        for f in ("t", "fg", "bg", "b"):
            if ca.get(f) != cb.get(f):
                out.append({"cell": k, "field": f,
                            "upstream": ca.get(f), "yolo": cb.get(f)})
    return out


if __name__ == "__main__":
    main()
```

`justfile` (appended):

```
# Parity sweep (S8.3) — on-demand, user-run, NEVER CI: renders the yolo
# side + diffs all 17 surfaces against the pinned upstream fixtures.
# See scripts/parity/sweep.py.
parity-sweep:
    python3 scripts/parity/sweep.py
```

**Step 4 — gate:** `go vet ./... && go test ./...` green (the dump test
skips without the env var; `TestImportsDirection` — `parity_test.go` is a
test file: the extra imports are `server/testutil` + `llm`/`llm/fake` +
`theme`/`client`/`store` = the sanctioned TUI-test set) + `gofmt -l .`
empty + `just parity-sweep` exits 0 ("sweep complete — N/17 surfaces
MATCH" — N is whatever the render shows; the GAPS surfaces are the
expected D8 input for S8.4, D7). The report is committed.

**Step 5 — commit** the pinned message `test: parity diff sweep - yolo
vs upstream captures` (staging `scripts/parity/sweep.py`,
`internal/tui/parity_test.go`, `justfile`, and the sweep report
`docs/superpowers/plans/2026-08-24-opencode-tui-parity/
parity-sweep-report.md`), then `bd close yolo-oae.9.4 --reason "S8.3
done: the yolo render dump (TestParityDump — the 17 D2 surfaces through
the real stack + the fake driver scripted from the shared canned
constants, the YOLO_PARITY_DUMP gate, the full-stream raw dumps) + the
sweep (normalize both sides with the shared normalizer, the per-surface
cell diff, the MATCH/GAPS report) + the first sweep report committed as
S8.4's input" --json`.

### Task S8.4: Close every visible gap or log it (DEVIATIONS.md with severity) + re-baseline goldens (bead `yolo-oae.9.4`, expected id `yolo-oae.9.5`)

**Files:** the gap set = the S8.3 report's GAPS surfaces (this task
reads `docs/superpowers/plans/2026-08-24-opencode-tui-parity/
parity-sweep-report.md` — the mechanical input); the per-surface
resolution is either (a) CLOSE: a yolo render change (the usual
`internal/tui/*` files + the re-baselined SGR goldens in the SAME commit,
root principle 3) or (b) LOG: a `docs/superpowers/DEVIATIONS.md` entry
(numbered from 254 — the 253 detail-pass breadcrumb precedes it) with
severity + the surface + the judgment reason; the re-run of the sweep
re-writes `parity-sweep-report.md` (the logged surfaces keep their GAPS
rows — the report is the sweep's output, the deviation log is the
judgment).

**Interfaces:** the D7 judgment rule — every GAPS surface is CLOSED or
LOGGED; no third state. The expected gaps (D8, read at detail time)
bound the work: (1) `epilogue` — the yolo prints nothing on exit vs the
upstream session epilogue (the yolo referent exists: `yolo [<sessionID>]`
resume — the ported text "Continue yolo <id>"; CLOSE is in scope, LOG
severity medium is the fallback); (2) the color-space class (upstream
truecolor vs yolo ANSI256 — deviation 125): a surface whose ONLY gap
cells are fg/bg encodings of the same visual color is LOGGED once as a
class (severity info) — the normalizer keeps the color values verbatim
by design, so this class is EXPECTED and is not a per-cell log; (3) the
which-key overlay contents (the yolo context filter, deviation 207(3));
(4) anything else the sweep found.

**Step 1 — failing test:** `just parity-sweep` — the report from S8.3
lists the GAPS surfaces (the RED state = the existing unjudged gaps; if
the S8.3 report was all-MATCH this step is a no-op and Steps 3–5 shrink
to the verification + commit only).

**Step 2 — confirm FAIL:** read the report: every GAPS(n) row is the
unjudged work (the expected D8 gaps are named in the report). No test
command — the red state is the report itself (D7: the sweep is the
test of the parity contract).

**Step 3 — the per-surface resolution:** for each GAPS surface, in
report order: judge CLOSE vs LOG (the D7 rule + the D8 expectations —
the judgment reason is one line in the commit body / the deviation
entry). CLOSE (a yolo render change): implement the smallest render fix
that makes the surface MATCH (the usual 5-step TDD: the failing
teatest token → the fix → the gate), AND re-baseline any SGR golden the
fix touches in the SAME commit (root principle 3 — never leave a pin
dangling). LOG: append the DEVIATIONS.md entry (number, severity,
surface, the observed gap, the reason it stays — e.g. the color-space
class, an upstream-only surface with no yolo referent, a wire-freeze
boundary) with the standing form. Re-run `just parity-sweep` until every
surface is MATCH or logged (the report is re-written by the sweep).

**Step 4 — gate:** `go vet ./... && go test ./...` green (incl. the
re-baselined goldens) + `gofmt -l .` empty + the sweep report shows
every GAPS surface carried by a DEVIATIONS.md entry (the commit body
lists the surface → entry number mapping).

**Step 5 — commit** the pinned message `docs: parity deviations logged
+ goldens re-baselined` (staging the render fixes + the re-baselined
goldens + `DEVIATIONS.md` + the re-written sweep report), then
`bd close yolo-oae.9.5 --reason "S8.4 done: <N> surfaces MATCH, <M>
logged as deviations <range> (the color-space class info, the
<list>) — the sweep report is the mechanical record, the deviation log
the judgment" --json`.

> Note (2026-09-03, post-S8.4): the "goldens re-baselined" clause applies
> only on the CLOSE path (Step 3 re-baselines a TTY_FORCE SGR golden only
> when a surface is CLOSED with a render fix). The all-LOG S8.4 run closed
> 0 surfaces, so 0 SGR goldens were re-baselined (no non-parity test file
> changed) — the 14/17 upstream parity-fixture re-baseline is the S8.2/S8.4
> capture pin, not the SGR goldens. The committed subject therefore
> overstates the golden step. The frozen message is retained as-is per the
> freeze rule (line 16); this note records the conditional intent for
> future frozen messages that conditionally apply.

### Task S8.5: Close-out: PROGRESS.md verified fact; epic close; tag ONLY on explicit user go-ahead (bead `yolo-oae.9.5`, expected id `yolo-oae.9.6`)

**Files:** `docs/superpowers/PROGRESS.md` (the verified fact + the
status pointer — the only docs change), the beads state (the epic close
+ any tag — the USER-GATED branch).

**Interfaces:** the verified-fact shape (the PROGRESS.md "Key verified
facts" convention): one line — the sweep outcome (N MATCH / M logged +
the deviation numbers), the fixture pin (17 normalized screens +
MANIFEST + the catalog pin + the canned pin, npm 1.18.18), the
re-baselined golden set. The user gate: the epic close (`bd close
yolo-oae`) AND any tag are executed ONLY on explicit user go-ahead (root:
tags only with explicit user go-ahead; semantic versioning). If the
go-ahead is absent at this task's execution, the bead still closes and
the PROGRESS status pointer records "epic close pending user go-ahead".

**Step 1 — failing test:** the verification artifacts are the input: the
module gate green, `gofmt -l .` empty, the sweep report complete
(every surface MATCH or logged), the slice gate (2) user-run smoke
performed (the capture + the sweep under a TRUECOLOR terminal — the
slice gate owns the smoke; this task records its outcome).

**Step 2 — confirm FAIL:** the PROGRESS.md status pointer still says the
pre-S8 state (the slice is not yet recorded as done).

**Step 3 — the close-out:** write the PROGRESS.md verified fact (the
Interfaces shape) + the one-line status pointer ("S8 done — epic close
pending user go-ahead" or, on go-ahead, the closed state). On explicit
user go-ahead ONLY: `bd close yolo-oae --reason "the TUI parity epic:
S0–S8 done, the 17-surface sweep logged" --json` and the semantic tag
(the user names it; the version is the user's call — the tag is the
user's action recorded here, never the agent's).

**Step 4 — gate:** `go vet ./... && go test ./...` green + `gofmt -l .`
empty (no code change expected — the gate confirms the no-drift state
before the fact is recorded).

**Step 5 — commit** the pinned message `docs: TUI parity close-out
(PROGRESS fact)` (staging `docs/superpowers/PROGRESS.md`), then
`bd close yolo-oae.9.6 --reason "S8.5 done: the PROGRESS verified fact
recorded; the epic close + tag <performed on user go-ahead | pending
user go-ahead>" --json`.

## S8 slice gate (slice bead `yolo-oae.9`)

NOT a task bead; runs after all child beads close. Mirror the S0 slice gate
shape: (1) module gate `go vet ./... && go test ./...` + `gofmt -l .` empty
(incl. `TestImportsDirection` + the re-baselined teatest goldens);
(2) user-run smoke (NOT CI): run the parity capture + diff sweep under a
TRUECOLOR terminal (deviation 125: the upstream SGR is always 24-bit, so
the capture TERM must be truecolor for a comparable diff); (3) append any
forced DEVIATIONS.md entries this slice named (with severity, same-commit
rule — root principle 2); (4) PROGRESS.md one-line status pointer;
(5) commit `docs: checkpoint — S8 done, epic close pending user go-ahead`;
(6) `bd close yolo-oae.9 --reason "all 5 child beads closed, gate green,
parity sweep logged" --json``. The epic close (`bd close
yolo-oae`) and any tag are S8.5 scope — ONLY on explicit user go-ahead
(root: tags only with explicit user go-ahead; semantic versioning).
