# TUI Parity Implementation Plan (full copy of opencode's TUI — style, design, colors)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Copy opencode v1.18.18's TUI style/design/colors into yolo's Go TUI across
nine slices — slice S0 (theme engine + app-shell restyle) is fully detailed in this
plan and executes next; slices S1–S8 (transcript, dialogs, keymap/palette,
prompt/home/session completion, parity audit) are bound by their task tables and
detailed per the Slice Detail Protocol before they start.

**Architecture:** New package `internal/tui/theme/` is the horizontal foundation
(spec §3): 33 embedded upstream theme JSONs, a pure `resolveTheme` port,
`generateSystem` (terminal palette → theme), OSC 11/10/4 terminal palette
detection, custom-theme discovery (`~/.config/yolo/themes/*.json` +
`.yolo/themes` walk up from CWD, SIGUSR2 refresh — spec §3, flat under the
global config root like upstream; paths injected by cmd/yolo, TUI purity),
and the selection chain
(config `theme` string > TUI KV > default `"opencode"`) over a tiny KV file at
`<dataDir>/tui/kv.json`. The app shell consumes a resolved `theme.Theme` through
lipgloss style accessors (components never see hex). Later slices stack on it:
glamour transcript rendering (S1), huh/select dialog system (S2–S3), keymap
registry + command palette + which-key (S4), prompt completion + bell (S5),
home completion (S6), session completion (S7), and the pty-capture parity audit
(S8). Each task bead ends gate-green with one commit carrying the plan-pinned
message.

**Tech Stack:** Go 1.26 (module requires ≥ 1.25); allowlist runtime deps
unchanged in S0 (`charm.land/bubbletea/v2` v2.0.9, `lipgloss/v2` v2.0.6,
`bubbles/v2` v2.2.1); one **promotion** in S0: `github.com/charmbracelet/x/term`
v0.2.2 (already indirect via bubbletea — zero new modules — raw-mode tty for OSC
queries; dep-proposal bead + user approval before `go mod tidy`, spec §2
ecosystem survey). Later slices add (each behind its own dep-proposal bead +
approval): `charm.land/glamour/v2` v2.0.1 (S1), `charm.land/huh/v2` v2.0.3 +
`github.com/sahilm/fuzzy` v0.1.3 (S2). Dev-only: node (v26, golden-matrix
generation script), teatest/v2 (already dev-pinned).

**Spec:** `docs/superpowers/specs/2026-08-24-opencode-tui-parity-design.md`
(approved 2026-08-24). The plan argues from the spec; read both. Epic: `yolo-oae`;
slice beads `yolo-oae.1`–`yolo-oae.9` exist; plan bead: `yolo-oae.10`.

## Global Constraints

- **Strict-copy bar** (spec §1, user decision): theme colors **exact by
  construction** (ported JSON data); dark/light selection follows the upstream
  algorithm verbatim (`0.299r+0.587g+0.114b > 0.5` → light); spacing/glyphs/pinned
  text follow upstream source as faithfully as Go + terminal allow. **Any forced
  deviation is logged in `docs/superpowers/DEVIATIONS.md` with severity in the
  same commit that lands the deviation** (root principle 2).
- **TUI-only, contract-backed** (spec §1): no core server changes, no new wire
  endpoints. The ONE sanctioned wire change: `protocol.Config.Theme` from
  `map[string]any` (v1 "accepted, ignored", port spec §6.1) to `string` —
  spec §1 "config `theme` string (honored)" — logged as a deviation (wire/low) in
  S0 Task 7.
- **Zero telemetry** (root principle 1) unchanged: OSC queries are local terminal
  I/O only; nothing leaves the machine.
- **Dependency policy** (root "Project"): allowlist + agent-proposable. New
  modules (x/term promotion in S0; glamour/huh/fuzzy in S1/S2) land ONLY after a
  dep-proposal bead (module + exact version + live-verified evidence) and
  explicit user approval; then `go get`/`go mod tidy`. Nothing else is added.
- **TUI import purity** (root principle 4, enforced by `TestImportsDirection`):
  non-test files under `internal/tui/` import only `internal/protocol` +
  `internal/tui/*` (+ stdlib / charm-ecosystem deps). Consequence:
  `internal/tui/theme` does NOT import `internal/config` — all filesystem paths
  (KV file, discovery dirs, data dir) are INJECTED by `cmd/yolo`.
- **Gate at module root after every task:** `go vet ./... && go test ./...` green
  and `gofmt -l .` prints nothing.
- **Tests never hit the network.** Palette detection is tested against scripted
  in-memory I/O with an injected clock; theme resolution against the golden
  matrix; discovery against `t.TempDir()` fixtures; SIGUSR2 debounce against an
  injected clock.
- **teatest hardening (deviation-118 precedent, from the first new suite):**
  merge multi-token terminal states into ONE `WaitFor` condition; probe `Read`s
  consume bytes; strip-ANSI + independent tokens for frame assertions; pin
  `teatest.WithProgramOptions(tea.WithEnvironment([]string{"TTY_FORCE=1",
  "TERM=xterm-256color"}))` whenever a test asserts on SGR codes.
- **embed:** `import "embed"` + `var assets embed.FS` works (verified 2026-08-24
  on the 1.26.7 toolchain); the `import _ "embed"` + scalar-var workaround
  (see `internal/tool/read.go`) is only needed for scalar `//go:embed` vars.
- **Pins** (root principle 3): newly ported pinned text (the logo block in S0
  Task 8; tips in S6) gets a sha256 pin in the same commit it lands; an
  intentional change re-baselines the pin in the same commit; never leave a pin
  dangling.
- **Branch discipline:** never commit to `main`. Work happens inline on the
  current task branch, one task at a time. Task commits use the EXACT pinned
  message of their task (conventional, imperative, ≤ 72-char subject).
- **Subagents:** one at a time (root principle 7); a YOLO root spawns only YOLO
  subagents (root principle 8); dispatch with `thinking=medium`.
- **Upstream reference:** `/tmp/opencode-upstream` at tag `v1.18.18`. If missing:
  `git clone --depth 1 --branch v1.18.18 https://github.com/anomalyco/opencode
  /tmp/opencode-upstream`. **OSC port reference:** the extracted
  `@opentui/core` 0.4.5 npm package at `/tmp/opencode/.opentui-core/package/`
  (`chunk-node-q0cwyvm9.js` lines 10050–10330 = `src/lib/terminal-palette.ts`).
  If missing: `cd /tmp/opencode && mkdir -p .opentui-core && cd .opentui-core &&
  npm pack @opentui/core@0.4.5 && tar xzf opentui-core-0.4.5.tgz`.

## Branch & execution setup

Work branch: **`new_tui`** (the current branch; carries the approved spec +
epic commits `2f5afff`/`2016303`/`65cabad`). It has no upstream; do NOT push
without explicit user go-ahead (branch → commit → push → PR → user merge).

**Per-task stop cadence** (v0.2.0/v0.3.0 pattern): complete a task's full
5-step TDD ending in its Step-5 pinned-message commit (plus a `DEVIATIONS.md`
entry where the task names one), then **STOP**: report the gate result, the
commit, and `git status`; wait for go-ahead. Beads: claim the task's bead at
task start (`bd update <id> --claim --json`), close it at the stop point
(`bd close <id> --reason "..." --json`), then commit the `.beads/` export diff
(`issues.jsonl`, `interactions.jsonl`) as a separate
`chore: beads export (<id> closed)` commit — the pinned task message stays
untouched. The Dolt DB itself is never git-committed.

## Bead model (spec §8 — binding)

- Epic `yolo-oae`; slice beads `yolo-oae.1`–`yolo-oae.9` already exist
  (S0 = `yolo-oae.1`, …, S8 = `yolo-oae.9`).
- **Task beads are created as children of the active slice bead when the slice
  starts** (`bd create "<task title>" --parent=yolo-oae.N -p 2 --json`), in the
  order of the slice's table below. IDs follow `yolo-oae.N.M`. Title, scope, and
  pinned commit message come verbatim from the table.
- A **slice bead closes only when all its task beads are closed** and the
  slice-level gate (full `go vet ./... && go test ./...` + `gofmt`, plus the
  slice's teatest suites) is green.
- The tables below enumerate **every** task bead with its pinned commit message
  (spec §8 binding). Total: **65 task beads** (10+9+10+9+7+6+5+4+5) + 9 slice
  beads + 1 epic.

## Slice Detail Protocol (how S1–S8 get their step-level detail)

This plan is a directory: `plan.md` (this file — global constraints, bead
model, the binding 65-bead inventory) + one file per slice. S0 is fully
detailed in `s0-theme-engine.md` — it is the active slice. For S1–S8, each
slice file carries: a pointer to its binding task table (the inventory in
this file — the single source of truth, not duplicated), its dep gate, and
the **exact upstream sources** the detail pass reads. Before a slice starts, a **writing-plans pass** (one subagent,
`thinking=high`) fills that slice's file with the full 5-step TDD detail
(failing test code, implementation code, gate, pinned commit) for each of its
tasks — reading the named upstream files at that moment so the port code is
fresh and accurate. Rules:

1. The binding tables (bead titles, scope, pinned commit messages, dep gates)
   are FROZEN by this plan; a detail pass may not change them. If detail reveals
   a table row must change, STOP and get explicit user approval (spec §8 grain
   is binding) and re-record the change here.
2. Execution of a slice does NOT start until its section holds step-level detail
   for all of its tasks (no "fill in later" — the detail pass is a task gate).
3. The detail pass commits as `docs: TUI parity plan — detail <slice> tasks`
   (its own bead: `bd create "detail <slice> plan tasks" --parent=<slice bead>`).
4. After each slice closes, the next slice's detail pass runs — strictly
   sequential (root principle 7).

## Task inventory (all 9 slices — binding)

### S0 — theme engine + app-shell restyle (slice bead `yolo-oae.1`) — fully detailed in `s0-theme-engine.md`

| Bead | Task | Pinned commit message |
|---|---|---|
| `yolo-oae.1.1` | Embed 33 upstream theme JSONs + `ThemeJson` model + parse tests | `feat: embed 33 upstream theme JSONs + ThemeJson model` |
| `yolo-oae.1.2` | Golden matrix generation + `resolveTheme` port + 33×2 golden tests | `feat: port resolveTheme + 33x2 golden matrix` |
| `yolo-oae.1.3` | `Theme` struct + lipgloss style accessors + tests | `feat: Theme struct + lipgloss style accessors` |
| `yolo-oae.1.4` | `generateSystem` port + tests | `feat: port generateSystem (terminal palette to theme)` |
| `yolo-oae.1.5` | OSC 11/10/4 palette detection + luminance mode + fast-fail + tests (incl. x/term promotion dep proposal) | `feat: OSC 11/10/4 terminal palette detection` |
| `yolo-oae.1.6` | Custom theme discovery (config dir + `.yolo` walk, SIGUSR2 refresh) + tests | `feat: custom theme discovery (.yolo walk) + SIGUSR2 refresh` |
| `yolo-oae.1.7` | Selection chain (config > KV > default) + mode lock + KV file + `config.theme` wire change + app wiring | `feat: theme selection chain (config > KV > default) + TUI KV` |
| `yolo-oae.1.8` | Shell restyle: logo + borders (+ teatest SGR goldens) | `feat: shell restyle - upstream logo + border tokens` |
| `yolo-oae.1.9` | Shell restyle: home list + footer (+ teatest SGR goldens) | `feat: shell restyle - home list + footer theme tokens` |
| `yolo-oae.1.10` | Shell restyle: session chrome (+ teatest SGR goldens) | `feat: shell restyle - session chrome theme tokens` |

### S1 — transcript rendering (slice bead `yolo-oae.2`)

Dep gate: **`charm.land/glamour/v2` v2.0.1** (dep-proposal bead first; spec §2
evidence row 1).

| Bead | Task | Pinned commit message |
|---|---|---|
| `yolo-oae.2.1` | Dep proposal glamour v2.0.1 (approval gate) → `go get` + smoke render | `deps: add glamour v2.0.1 (transcript rendering)` |
| `yolo-oae.2.2` | `TermRenderer` from resolved theme (`StyleConfig` + chroma token map) + fixture unit tests | `feat: glamour TermRenderer from resolved theme tokens` |
| `yolo-oae.2.3` | Wire renderer into text parts (replaces plain wrap) + teatest goldens | `feat: render text parts through the glamour renderer` |
| `yolo-oae.2.4` | Syntax-highlighted code blocks (per-language chroma styles) + tests | `feat: syntax-highlighted code blocks (theme syntax tokens)` |
| `yolo-oae.2.5` | GFM: tables, task lists, strikethrough + tests | `feat: GFM in transcript - tables, task lists, strikethrough` |
| `yolo-oae.2.6` | Reasoning block restyle (dimmed, indented, collapsible) + tests | `feat: reasoning block restyle (dimmed collapsible)` |
| `yolo-oae.2.7` | Tool-row restyle (per-tool glyphs, alt+e expand) + tests | `feat: tool-row restyle (glyphs + expand)` |
| `yolo-oae.2.8` | Error parts + toast restyle (theme tokens) + tests | `feat: error parts + toast restyle (theme tokens)` |
| `yolo-oae.2.9` | Re-render benchmark on batched delta + budget gate (extends `session_bench_test.go`) | `perf: transcript re-render benchmark + budget gate` |

### S2 — dialog system (slice bead `yolo-oae.3`)

Dep gate: **`charm.land/huh/v2` v2.0.3 + `github.com/sahilm/fuzzy` v0.1.3**
(dep-proposal bead first; spec §2 evidence rows 2–3).

| Bead | Task | Pinned commit message |
|---|---|---|
| `yolo-oae.3.1` | Dep proposal huh v2.0.3 + sahilm/fuzzy v0.1.3 (approval) → `go get` + smoke | `deps: add huh v2.0.3 + sahilm/fuzzy v0.1.3 (dialogs)` |
| `yolo-oae.3.2` | Modal dialog stack (port `dialog.tsx`: centered overlay, focus capture, esc, stackable, result callback) + tests | `feat: modal dialog stack (overlay, focus, esc, stack)` |
| `yolo-oae.3.3` | huh fields: alert + confirm (themed via `StyleConfig` ← theme tokens) + tests | `feat: huh field dialogs - alert + confirm (themed)` |
| `yolo-oae.3.4` | huh field: input (rename/prompt, themed) + tests | `feat: huh field - themed input (rename/prompt)` |
| `yolo-oae.3.5` | Select core (ported from upstream `dialog-select`): options, navigation, fuzzy filter + tests | `feat: select core - options, navigation, fuzzy filter` |
| `yolo-oae.3.6` | Select: categories/groups + per-option details + tests | `feat: select - categories + per-option details` |
| `yolo-oae.3.7` | Select: actions + footer hints + scroll acceleration + tests | `feat: select - actions, footer hints, scroll acceleration` |
| `yolo-oae.3.8` | Permission dialog restyle (on the select stack) + tests | `feat: permission dialog restyle (on select)` |
| `yolo-oae.3.9` | Model dialog restyle (on select) + tests | `feat: model dialog restyle (on select)` |
| `yolo-oae.3.10` | Agent dialog restyle (on select) + tests | `feat: agent dialog restyle (on select)` |

### S3 — remaining contract-backed dialogs (slice bead `yolo-oae.4`)

| Bead | Task | Pinned commit message |
|---|---|---|
| `yolo-oae.4.1` | Session-list dialog (on select) + tests | `feat: session-list dialog (on select)` |
| `yolo-oae.4.2` | Session-rename dialog (themed huh input) + tests | `feat: session-rename dialog (huh input)` |
| `yolo-oae.4.3` | Session-delete-failed dialog + tests | `feat: session-delete-failed dialog` |
| `yolo-oae.4.4` | Provider dialog restyle + tests | `feat: provider dialog restyle` |
| `yolo-oae.4.5` | Status dialog + tests | `feat: status dialog` |
| `yolo-oae.4.6` | Help dialog restyle (keymap-registry-driven) + tests | `feat: help dialog restyle (keymap-driven)` |
| `yolo-oae.4.7` | Retry-action dialog + tests | `feat: retry-action dialog` |
| `yolo-oae.4.8` | Theme-list dialog (select over `theme.All()`) + tests | `feat: theme-list dialog (select over themes)` |
| `yolo-oae.4.9` | Theme-list: KV wiring + mode switch/lock keybinds + tests | `feat: theme commands - KV wiring + mode switch/lock` |

### S4 — keymap + command palette + which-key (slice bead `yolo-oae.5`)

| Bead | Task | Pinned commit message |
|---|---|---|
| `yolo-oae.5.1` | Keymap registry: upstream default bindings (ported `config/keybind.ts`) + tests | `feat: keymap registry - upstream default bindings` |
| `yolo-oae.5.2` | Keymap registry: per-context groups + runtime remap (takes effect immediately) + tests | `feat: keymap registry - context groups + runtime remap` |
| `yolo-oae.5.3` | Keybinds config schema under `yolo.jsonc` (binding value = string \| keystroke object \| array \| `false`/`"none"`) + tests | `feat: keybinds config schema (yolo.jsonc keybinds field)` |
| `yolo-oae.5.4` | Command palette: overlay over `GET /command` + fuzzy filter + tests | `feat: command palette - overlay + fuzzy filter` |
| `yolo-oae.5.5` | Command palette: arrow nav + enter runs + esc closes + tests | `feat: command palette - nav, run, esc` |
| `yolo-oae.5.6` | Which-key: pending prefix-group overlay (registry-driven) + tests | `feat: which-key - prefix group overlay` |
| `yolo-oae.5.7` | Which-key: registry integration — /help + footer hints render from it + tests | `feat: which-key - /help + footer hints from registry` |

### S5 — prompt completion + attention (slice bead `yolo-oae.6`)

| Bead | Task | Pinned commit message |
|---|---|---|
| `yolo-oae.6.1` | Prompt history: ↑/↓ recall + tests | `feat: prompt history - up/down recall` |
| `yolo-oae.6.2` | Prompt history: persistence in KV (dedupe, cap) + tests | `feat: prompt history - KV persistence` |
| `yolo-oae.6.3` | Frecency-ranked recall (ported scoring, persisted) + tests | `feat: prompt frecency ranking` |
| `yolo-oae.6.4` | @-autocomplete: fuzzy file picker (sahilm/fuzzy) + tests | `feat: @-autocomplete - fuzzy file picker` |
| `yolo-oae.6.5` | /-autocomplete: slash-command picker (GET /command) + tests | `feat: /-autocomplete - slash command picker` |
| `yolo-oae.6.6` | Terminal bell on turn completion / error (ported `notifications.ts` conditions) + tests | `feat: terminal bell on turn completion/error` |

### S6 — home completion (slice bead `yolo-oae.7`)

| Bead | Task | Pinned commit message |
|---|---|---|
| `yolo-oae.7.1` | Home: startup-loading spinner while hydrating + tests | `feat: home - startup loading spinner` |
| `yolo-oae.7.2` | Home: rotating tips — ported tips data + sha256 pin + tests | `feat: home - rotating tips (ported, sha256-pinned)` |
| `yolo-oae.7.3` | Home: tips rotation cadence + rendering + tests | `feat: home - tips rotation + rendering` |
| `yolo-oae.7.4` | Home: session-destination view + tests | `feat: home - session destination view` |
| `yolo-oae.7.5` | Home: footer key hints from the keymap registry + tests | `feat: home - footer key hints from registry` |

### S7 — session completion (slice bead `yolo-oae.8`)

| Bead | Task | Pinned commit message |
|---|---|---|
| `yolo-oae.8.1` | Todo sidebar: latest `todowrite` part → status-glyph list + tests | `feat: todo sidebar - todowrite part to status list` |
| `yolo-oae.8.2` | Todo sidebar: keymap toggle + layout + tests | `feat: todo sidebar - keymap toggle + layout` |
| `yolo-oae.8.3` | Dialog-message (full-message view) + tests | `feat: dialog-message (full message view)` |
| `yolo-oae.8.4` | Session footer detail restyle (model/agent/tokens/cost/spinner/connection) + tests | `feat: session footer detail restyle` |

### S8 — parity audit + close-out (slice bead `yolo-oae.9`)

| Bead | Task | Pinned commit message |
|---|---|---|
| `yolo-oae.9.1` | Mock OpenAI-compatible SSE server (canned stream) for deterministic capture | `test: mock OpenAI-compatible SSE server for parity capture` |
| `yolo-oae.9.2` | Upstream pty-capture script (npm `opencode-ai@1.18.18`, scripted keys, volatile bits stripped, pinned fixtures) | `test: upstream pty-capture script + fixtures` |
| `yolo-oae.9.3` | Parity diff sweep: yolo teatest renders vs upstream captures, per-surface | `test: parity diff sweep - yolo vs upstream captures` |
| `yolo-oae.9.4` | Close every visible gap or log it (DEVIATIONS.md with severity) + re-baseline goldens | `docs: parity deviations logged + goldens re-baselined` |
| `yolo-oae.9.5` | Close-out: PROGRESS.md verified fact; epic close; tag ONLY on explicit user go-ahead | `docs: TUI parity close-out (PROGRESS fact)` |

## Slice files

| File | Contents | State |
|---|---|---|
| `plan.md` | this file — goal, global constraints, branch setup, bead model, Slice Detail Protocol, the binding 65-bead inventory | done |
| `s0-theme-engine.md` | S0.1–S0.10 full 5-step TDD detail + S0 slice gate | **fully detailed — the active slice** (execution pending) |
| `s1-transcript.md` | S1 slice brief (dep gate, upstream sources, gate) | brief done; detail pass before S1 starts |
| `s2-dialogs.md` | S2 slice brief | brief done; detail pass before S2 starts |
| `s3-dialogs-2.md` | S3 slice brief | brief done; detail pass before S3 starts |
| `s4-keymap.md` | S4 slice brief | brief done; detail pass before S4 starts |
| `s5-prompt-completion.md` | S5 slice brief | brief done; detail pass before S5 starts |
| `s6-home-completion.md` | S6 slice brief | brief done; detail pass before S6 starts |
| `s7-session-completion.md` | S7 slice brief | brief done; detail pass before S7 starts |
| `s8-parity-audit.md` | S8 slice brief | brief done; detail pass before S8 starts |

Execution reads ONLY the active slice's file (plus this file's Global
Constraints) — never the whole directory.

---

