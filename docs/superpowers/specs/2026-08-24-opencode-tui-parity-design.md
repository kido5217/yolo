# TUI Parity — Full Copy of opencode's TUI (style, design, colors)

- **Date:** 2026-08-24
- **Status:** Approved design (user, 2026-08-24, in chat — all sections LGTM)
- **Upstream reference:** [anomalyco/opencode](https://github.com/anomalyco/opencode) tag
  **`v1.18.18`** (clone at `/tmp/opencode-upstream`). Upstream TUI =
  `packages/tui/src` — solid-js on **`@opentui/core` 0.4.5** (TypeScript,
  ~27k LOC / 152 files). Yolo TUI today: Go bubbletea, ~3.4k LOC / 40 files.
- **Supersedes:** the TUI v1.x non-goals list of
  `2026-08-17-yolo-go-port-design.md` §5 (themes/keymap engine, command
  palette, stashes, timelines, workspaces UI, plugin slots): those surfaces
  are now in scope **where the wire contract backs them** (Section 1).
- **Policy basis:** v0.4.0 "reference, not contract" — this task is an
  explicit user instruction to copy; every forced deviation is logged in
  `DEVIATIONS.md` with severity (append-only discipline unchanged).

---

## 1. Scope & fidelity bar

**Scope: TUI-only, contract-backed** (user decision, 2026-08-24). No core
server changes, no new wire endpoints. Surfaces whose data is not on yolo's
wire contract are deferred, not stubbed.

**Fidelity bar: strict copy** (user decision, 2026-08-24):

- Theme colors **exact by construction** (ported JSON data).
- Dark/light mode selection follows the upstream algorithm verbatim.
- Spacing / glyphs / pinned text follow the upstream source as faithfully as
  Go + terminal allow.
- Any forced deviation (a token lipgloss can't express, a structural default
  of a reused library) is logged in `DEVIATIONS.md` with severity. Verified
  per Section 7.

**In scope:**

| Area | Surfaces |
|---|---|
| Theme engine | All **33 theme JSONs** ported verbatim (embed); per-token `dark`/`light`; auto mode via terminal-background luminance (`0.299r+0.587g+0.114b > 0.5` → light); **system theme** generated from the terminal's own 16-color palette; config `theme` string (honored — today "accepted, ignored"); theme-list dialog |
| Chrome | logo, startup-loading spinner, borders, busy spinner, toasts, link styling |
| Home route | session list (styled rows, relative time, last 50), **tips** (rotating hints), home footer (key hints from keymap registry), session-destination |
| Session route | transcript with **full markdown + syntax-highlighted code blocks** (theme `markdown*`/`syntax*` tokens), styled reasoning blocks, tool-part rows (glyphs, expandable I/O), permission dialog, **todo sidebar** (driven by `todowrite` parts — already on the wire), session footer, dialog-message |
| Dialog system | modal stack; huh fields (alert, confirm, input); ported select (workhorse); contract-backed set: **model, provider, agent, session-list, session-rename, session-delete-failed, status, theme-list, help, retry-action** |
| Command palette | upstream palette UX over yolo's `GET /command` set (`/help`, `/new`, `/model`, `/agents`, `/quit` + `/exit` alias) |
| Prompt | styled prompt box + cwd line; **history** (↑/↓, persisted); **frecency**-ranked recall; **@-autocomplete** (fuzzy over local files + slash commands — TUI-local, no core change); `\`+enter newline preserved |
| Keymap | upstream default bindings; **configurable keybinds** (ported keybind config schema); **which-key** overlay |
| Attention | terminal bell on turn completion / error (local only) |

**Deferred (no contract backing — listed, not stubbed):** MCP dialog +
sidebar, LSP sidebar, workspace dialogs/label/prompt, skill dialog, stash
(dialog + prompt), subagent footer + dialog, question dialog, timeline +
fork-from-timeline, diff-viewer + file tree, plugins dialog, console-org,
files sidebar, context sidebar, tag/variant/move-session dialogs (settled
per-dialog in the slice plan unless their data exists on the wire), image/
PDF attachments, math (katex — renders as raw code), mouse selection.

## 2. Dependencies (all live-verified 2026-08-24)

New runtime deps, each filed as a **dep-proposal bead** (module + exact
version + live evidence) as the first task of the slice that first uses it;
`go get` only after approval (repo dependency policy):

| Module | Version | Used by | Evidence (verified live) |
|---|---|---|---|
| `charm.land/glamour/v2` | v2.0.1 | S1 transcript | Charm **v2 charm.land line**, requires `lipgloss/v2` (stack-compatible). Markdown element renderers + **per-token custom chroma style** (~32 token fields) + GFM (tables/task lists/strikethrough) + word wrap. MIT, active (tag v2.0.1, pushed 2026-08). Genuinely-new transitive ≈ 8 modules (goldmark, goldmark-emoji, chroma/v2, bluemonday, gorilla/css, douceur, x/exp/{golden,slice}, x/net); rest already in the graph. |
| `charm.land/huh/v2` | v2.0.3 | S2 dialogs | Charm v2 line; direct deps = the three allowlisted charm modules + tiny helpers (`catppuccin/go`, `x/exp/{ordered,strings}`, `hashstructure/v2`). MIT, active (v2.0.3, pushed 2026-08). User-selected (pragmatic call; structural deviations vs upstream look are logged). |
| `github.com/sahilm/fuzzy` | v0.1.3 | S2 select filter, S4 palette | Go de-facto subsequence fuzzer (upstream uses `fuzzysort`). **Zero runtime deps** (test-only godebug/x-tools), MIT, 1.4k stars, pushed 2026-06. v0 line, stable 2-call API (`Match`/`Score`). |

**Ecosystem survey (charmbracelet org, live):** `ultraviolet` (renderer
under bubbletea v2), `colorprofile`, `x/ansi`, `x/term`, `x/windows` are
**already in the transitive graph** — usable, not new (e.g. `x/ansi` for
OSC response parsing). `bubbles v2` sub-packages remain available under the
existing allowlist. Evaluated and **rejected**: `harmonica` (physics
animation — no animation in a strict copy), `sequin` (CLI tool; drags
cobra/fang), `a2tea` (out of scope), `colorprofile` as a direct dep (we
port upstream's OSC algorithm). Dev-time verification aids (manual, not
deps): `vhs` (terminal→GIF), `freeze` (terminal→PNG).

## 3. Theme engine (S0 — the one horizontal slice)

Foundational infrastructure with no user-visible slice until consumed —
the accepted exception to vertical slicing.

**Package `internal/tui/theme/`:**

- `assets/` — the 33 upstream JSONs embedded verbatim (`//go:embed`,
  unchanged file names; host embed quirk workaround per
  `internal/tool/read.go` pattern).
- `theme.go` — `ThemeJson` model (`defs` map; semantic tokens each with
  `dark`/`light` values, each a hex or a ref into `defs`) + ported
  `resolveTheme` → Go `Theme` struct exposing **lipgloss style accessors**
  (components never see hex: `th.MarkdownHeading()`, `th.Border()`,
  `th.Syntax(tokenType)`…).
- `system.go` — ported `generateSystem` (16-color terminal palette →
  generated theme) + terminal queries: **OSC 11** (background → luminance
  mode), **OSC 10/4** (palette for the system theme), ~100 ms timeout;
  no response → no system theme → active falls back to `"opencode"`
  (upstream's own catch path).
- `syntax.go` — ported `generateSyntax` / `generateSubtleSyntax`
  (theme → syntax token styles consumed by S1).
- `discover.go` — custom themes from `~/.config/yolo/themes/*.json` +
  `.yolo/themes/*.json` walking up from CWD; `isTheme` validation;
  **SIGUSR2 refresh** kept.

**Selection semantics (ported verbatim):** active theme = `config.theme`
(live) > TUI-local KV `theme` > default **`"opencode"`**. Mode = KV
`theme_mode_lock` > terminal luminance auto-detect > fallback; one-shot
`theme_mode` override cleared after use. KV = tiny JSON file at
`~/.local/share/yolo/tui/kv.json` (TUI-local, no core involvement).

**S0 deliverable:** engine + the **existing app shell restyled** (logo,
borders, footer, home list, session chrome) in the resolved theme.

**Verification:** pinned golden matrix (33 themes × dark/light) generated
**once** by running the upstream pure resolution functions in a standalone
node script (node available; functions are pure) → checked in under
`testdata/`; table-driven resolve tests; unit tests for mode/lock/selection
chains; teatest goldens for the restyled shell.

## 4. Transcript rendering (S1)

- **Renderer:** glamour `TermRenderer` built from the resolved theme:
  `ansi.StyleConfig` element styles ← theme `markdown*` tokens;
  `CodeBlock.Chroma` (~32 token fields) ← theme `syntax*` tokens;
  `WithWordWrap(width)` (v0.4.2 wrap behavior preserved).
- **Streaming:** re-render the text part on each batched SSE delta (existing
  batching); benchmark gate: 100 KB part re-render < 50 ms target
  (number finalized in the plan; extends `session_bench_test.go`).
- **GFM on:** tables, task lists, strikethrough. Math (katex upstream):
  **deferred** — renders as raw code.
- **Reasoning / tool / error:** restyle existing behavior to upstream
  tokens (dimmed indented collapsible reasoning; tool rows with per-tool
  glyphs + `alt+e` expand; errors in `th.Error()`).
- **Fidelity risk control:** S1 runs the pty-capture diff (Section 7) on a
  transcript fixture *before* the slice closes; visible structural gaps get
  per-element `StyleConfig` overrides, worst case a custom element renderer
  — logged either way.

## 5. Dialogs, command palette, which-key, keymap (S2–S4)

**Modal stack (ours — ported from `dialog.tsx`):** centered overlay, focus
capture, escape-to-close, stackable, promise-style `show()` in the
Go/tea idiom (dialog model + result callback).

**Field primitives → huh (themed, deviations logged):** `huh.Alert`,
`huh.Confirm`, `huh.Input` for alert / confirm / rename-and-prompt
dialogs; huh `StyleConfig` mapped from resolved theme tokens. Residual
structural differences vs upstream's borderless-pill look are logged per
deviation (user-accepted pragmatic call); any field dialog that comes out
unrecognizable is replaced by the hand-rolled version (logged).

**Select (workhorse) — ported, not huh:** the 791-line upstream
`dialog-select` (fuzzy filter input, categorized groups, per-option
details/actions/footer views, footer hints, scroll acceleration) ported 1:1;
its filter input uses `bubbles/textinput` (allowlisted); fuzzy scoring via
`sahilm/fuzzy`.

**Command palette — ported** (`command-palette.tsx`): overlay over the
`GET /command` set, fuzzy filter, arrow nav, enter runs, esc closes.

**Which-key — ported** (`feature-plugins/system/which-key.tsx`): shows the
pending binding group after a prefix key; driven entirely by the keymap
registry.

**Keymap + keybind config (ported):** binding registry (per-context groups,
upstream defaults) + the keybind config schema (binding value = string |
keystroke object | array | `false`/`"none"`; exact config field name
verified against upstream when porting) under the ported schema. Single
source of truth: which-key, `/help`, and footer hints all render from it.
Remaps take effect immediately; unknown commands ignored.

## 6. Routes & prompt (S5–S7)

**Home route:** ported logo; startup-loading spinner while hydrating;
session list rows (title · model · relative time via ported `util/locale`,
updated-desc, last 50); session-destination view; **rotating tips**
(ported `feature-plugins/home/tips*`; tips text becomes new sha256-pinned
content per principle 3); home footer = key hints from the keymap registry.

**Session route:** transcript (Section 4) in the viewport; reasoning/tool/
error restyle; permission dialog restyle; **todo sidebar** (ported
`sidebar/todo.tsx`: latest `todowrite` part → status-glyph list,
keymap-toggled); session footer restyle (model/agent/tokens/cost/busy
spinner/connection); **dialog-message** (full-message view) ported.
Subagent footer, question, timeline: deferred (Section 1).

**Prompt:** styled prompt box + cwd line (ported `component/prompt`);
**history** (↑/↓ recall, persisted in the KV file); **frecency** ranking
(ported scoring, persisted); **@-autocomplete** — `@` → fuzzy picker over
local files, `/` → slash commands (sahilm/fuzzy); purely TUI-local.
`\`+enter newline preserved.

**Attention:** terminal bell on turn completion / error (ported
`notifications.ts` conditions).

**Implementation reference:** TUI slices follow the project
**`charm-stack` skill** (Model-Update-View, component composition,
lipgloss layout, huh integration). The skill's v1 import paths are
illustrative; the allowlist's `charm.land/*` v2 line wins.

## 7. Verification strategy

**CI gate (unchanged):** `go vet ./... && go test ./...` + gofmt clean.
Tests never hit the network.

1. **Theme matrix golden (unit).** Expected resolved values for 33 themes ×
   {dark, light} generated once from the upstream pure resolution functions
   (standalone node script) → pinned under
   `internal/tui/theme/testdata/`; table-driven tests; mode/lock/selection
   chain tests.
2. **Per-surface teatest goldens (integration).** teatest v2 scripted runs,
   rendered-output goldens per surface: home, transcript (markdown fixture
   covering code blocks, lists, tables, quotes, links, CJK width), every
   dialog, palette, which-key, prompt states, sidebar. New suites hardened
   per the deviation-118 precedent (strip-SGR, independent tokens).
3. **Upstream pty-capture reference (dev-time, not CI — the strict-copy
   ground truth).** Deterministic capture: mock OpenAI-compatible server
   returning a canned SSE stream → `opencode serve` (npm
   `opencode-ai@1.18.18`) → its TUI driven in a pty with a scripted key
   sequence → terminal output captured (volatile bits stripped) → pinned
   fixtures. Diffed against yolo's teatest renders; **every visible
   mismatch that can't be closed becomes a `DEVIATIONS.md` entry with
   severity** — the diff drives the log. `vhs`/`freeze` remain available
   for manual side-by-side spot checks.

**Performance gate:** transcript re-render on batched delta benchmarked
(extends `session_bench_test.go`).

**Pins (principle 3):** new sha256 pins for tips text and any other
ported pinned text discovered per-slice; re-baselined in-commit when
intentionally changed; never left dangling.

## 8. Slicing plan (beads)

Three levels: **epic → slice beads → task beads**, strictly sequential
(core principle 7). Task-bead criteria (binding): one bead ≈ 1–3 files,
hours not days; gate green after every bead; **one commit per bead with the
plan-pinned message**; a slice bead closes only when all its children close
and the slice-level gate (incl. teatest goldens) is green. The
implementation plan enumerates **every** task bead with its pinned commit
message.

| Slice | Content | Dep gate |
|---|---|---|
| **S0** | theme engine (Section 3) + app-shell restyle | none |
| **S1** | transcript: glamour renderer from theme, reasoning/tool/error restyle, re-render benchmark | glamour v2.0.1 |
| **S2** | dialog system: modal stack + huh fields + select port + permission/model/agent restyle | huh v2.0.3, sahilm/fuzzy v0.1.3 |
| **S3** | remaining dialogs: session-list, session-rename, session-delete-failed, status, help, retry-action, theme-list (engine + KV wiring) | — |
| **S4** | keymap registry + keybinds config schema + command palette + which-key | — |
| **S5** | prompt completion: history, frecency, @-autocomplete + terminal bell | — |
| **S6** | home completion: tips (+sha256 pin), footer hints, session-destination, startup loading | — |
| **S7** | session completion: todo sidebar, dialog-message, footer detail restyle | — |
| **S8** | parity audit: full pty-capture diff sweep → close gaps or log deviations → re-baseline goldens → PROGRESS.md fact (tag only on user go-ahead) | — |

**S0 task beads (binding):** 1) embed 33 JSONs + `ThemeJson` model + parse
tests · 2) golden matrix generation + `resolveTheme` port + 33×2 golden
tests · 3) `Theme` struct + lipgloss accessors + tests · 4)
`generateSystem` port + tests · 5) OSC 11/10/4 queries + luminance mode +
100 ms fallback + tests · 6) custom theme discovery (config dir + `.yolo`
walk, SIGUSR2) + tests · 7) selection chain (config > KV > default) + mode
lock + KV file + tests · 8) shell restyle: logo + borders (+ goldens) ·
9) shell restyle: home list + footer (+ goldens) · 10) shell restyle:
session chrome (+ goldens).

**S1 task beads (binding):** 1) dep proposal: glamour v2.0.1 (approval
gate, then `go get`) · 2) `TermRenderer` from resolved theme
(`StyleConfig` + chroma map) + fixture unit tests · 3) wire renderer into
text parts (replaces plain wrap) + teatest goldens · 4) syntax-highlighted
code blocks (per-language) + tests · 5) GFM: tables, task lists,
strikethrough + tests · 6) reasoning block restyle + tests · 7) tool-row
restyle (glyphs, expand) + tests · 8) error parts + toast restyle + tests ·
9) re-render benchmark + budget gate.

**S2 task beads (binding):** 1) dep proposal: huh v2.0.3 + sahilm/fuzzy
v0.1.3 · 2) modal stack (port `dialog.tsx`: overlay, focus, stack, esc) +
tests · 3) huh fields: alert + confirm (themed) + tests · 4) huh field:
input (rename/prompt, themed) + tests · 5) select core: options,
navigation, fuzzy filter + tests · 6) select: categories/groups +
per-option details + tests · 7) select: actions + footer hints + scroll
acceleration + tests · 8) permission dialog restyle + tests · 9) model
dialog restyle (on select) + tests · 10) agent dialog restyle + tests.

**S3–S8 grain:** S3 ≈ 7–8 beads (one per dialog; theme-list = 2) · S4 ≈ 6
(registry ×2, keybinds config, palette ×2, which-key ×2) · S5 ≈ 6–7
(history ×2, frecency, autocomplete ×2, bell) · S6 ≈ 5 (logo/startup,
tips ×2 + pin, destination, footer) · S7 ≈ 4 (sidebar ×2, dialog-message,
footer detail) · S8 ≈ 5 (mock-SSE server, pty-capture script, diff sweep,
deviation-log + re-baseline, close-out). **Total ≈ 55–60 task beads +
9 slice beads + 1 epic.**

## 9. Risks & open items

**Risks → mitigations:**

1. glamour's structural fidelity vs opentui's `<markdown>` parser → S1
   runs the pty diff on the transcript fixture before the slice closes;
   per-element `StyleConfig` overrides; worst case a custom element
   renderer (logged).
2. huh's look vs upstream's borderless pills → user-accepted; S2 logs
   per-dialog deviations; unrecognizable dialogs replaced by hand-rolled
   versions (logged).
3. OSC palette queries flaky across terminals → hard 100 ms fallback to
   no-system-theme / `"opencode"` active (upstream's own catch path); KV
   mode lock covers user override.
4. Streaming re-render cost → S1 benchmark gate.
5. Fuzzy ranking (sahilm vs fuzzysort) → ordering check in the S4 diff;
   small scoring tweak if visibly off.
6. teatest flake class → deviation-118 hardening from the first new suite.

**Open items (plan-phase, not design blockers):** exact per-dialog
contract details (retry-action semantics, status content) read from
upstream per-slice; mock-SSE-server details for pty capture
(OpenAI-compatible — yolo already implements the client side); benchmark
budget number; whether @-autocomplete file walk honors `.gitignore`
(check upstream).

## 10. Explicitly out of scope

- Core server / wire contract — unchanged by this epic (TUI-only scope
  call); no new endpoints.
- The deferred surface list (Section 1) — listed, not stubbed; each gets
  its own future scope decision when (if) the wire contract grows.
- Zero telemetry — unchanged, non-negotiable (no console-org, no OTEL,
  `OTEL_*` inert).
- The mirrored legacy REST/SSE baseline — unchanged.
- Tags — only with explicit user go-ahead (semantic versioning).
