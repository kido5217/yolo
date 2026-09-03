# S4 — keymap + command palette + which-key (slice bead `yolo-oae.5`)

Land the keymap registry (upstream defaults, per-context groups, runtime
remap), the `yolo.jsonc` keybinds schema, the command palette, and the
which-key overlay — the single source for every TUI binding.

**State: fully detailed** — the 5-step TDD detail for all 7 tasks is in
the `## S4 detail` section below; execution may start at task S4.1.

## Binding task table

Pointer only (FROZEN — Slice Detail Protocol rule 1): `plan.md` →
`## Task inventory` → `### S4 — keymap + command palette + which-key (slice bead yolo-oae.5)`.
Bead titles, scope, and pinned commit messages live there and may not be
changed by a detail pass; any required change = STOP + explicit user
approval + re-record in plan.md.

## Dep gate

None — the palette's fuzzy filter reuses `sahilm/fuzzy` from the S2 gate.

## Exact upstream sources (the detail pass reads these AT DETAIL TIME)

`/tmp/opencode-upstream` @ v1.18.18:

- `packages/tui/src/config/keybind.ts` — the upstream default bindings
  (S4.1 registry data; binding value shapes: string | keystroke object |
  array | `false`/`"none"` per S4.3).
- `packages/tui/src/keymap.tsx` — per-context groups + runtime remap
  (S4.2).
- `packages/tui/src/component/command-palette.tsx` — the palette overlay +
  fuzzy + run (S4.4/S4.5).
- `packages/tui/src/feature-plugins/system/which-key.tsx` — the pending
  prefix-group overlay (S4.6/S4.7).

## yolo anchors

- `internal/tui/app.go` — `Update`: the key-handling hub; the S4 registry
  becomes the single source for all bindings.
- `internal/tui/keys.go` — the existing key-constant surface the registry
  replaces.
- `internal/tui/AGENTS.md` — the V1 keymap pins (pgup/pgdn, `\`+enter) must
  survive.
- `internal/protocol/` — the GET /command DTO the palette lists over.
- `internal/config/` — the `yolo.jsonc` keybinds schema (S4.3).

## Detail pass (protocol)

One writing-plans pass, one subagent, `thinking=high`, dispatched by the
root session strictly sequentially (root principle 7) — per the Slice
Detail Protocol in plan.md: it fills this file (after this section) with the
full 5-step TDD detail for each task in the binding table (failing test
code, implementation code, gate, pinned commit), reading the named upstream
files at that moment. It commits as
`docs: TUI parity plan — detail S4 tasks` on its own bead
(`bd create "detail S4 plan tasks" --parent=yolo-oae.5 --json`).

## S4 detail

Detail pass 2026-09-02. Deviations tail at detail time = 205; S4 entries
start at 206. Breadcrumb note (DEVIATIONS.md entry 206, severity info):
the frozen table names the task beads `yolo-oae.5.1`–`5.7`, but the S4
detail bead consumed `yolo-oae.5.1` (created + claimed before the detail
pass; the S1 "detail-bead-last" precedent is impossible because the detail
pass precedes slice start, as in S2 / deviation 165 and S3 / deviation 188).
The 7 task beads therefore land in table order at `yolo-oae.5.2`–
`yolo-oae.5.8` (S4.1→.2, …, S4.7→.8); the frozen titles and pinned commit
messages are unchanged. No code or wire impact.

### Detail-pass findings (read at detail time, 2026-09-02 — binding)

1. **Bubbletea v2.0.9 key primitives (verified at detail time):**
   - `tea.KeyPressMsg{Code rune, Mod tea.KeyMod, Text string}`;
     `tea.KeyReleaseMsg` exists (key.go:224) — the upstream `event: "release"`
     binding shape has a referent; the yolo matcher is press-only (deviation
     209).
   - `tea.KeyMod = uv.KeyMod` and `tea` re-exports the mod constants
     `ModShift/ModAlt/ModCtrl/ModMeta/ModHyper/ModSuper` (mod.go) — the
     matcher uses `tea.*` only (ultraviolet is indirect; dependency policy:
     allowlist, no new imports).
   - `KeyPressMsg.Keystroke()` = the key with modifiers in the fixed order
     ctrl, alt, shift, meta, hyper, super (key.go:200-210) — the pressed-side
     matcher parses this string.
   - The existing key-string conventions (bubbles v2.2.1 `key.Matches` +
     `String()`, deviation-178 class): "pgup"/"pgdown" (KeyPgUp/KeyPgDown),
     "esc" (KeyEscape), "enter" (KeyEnter), "backspace" (KeyBackspace),
     "shift+tab", "ctrl+c". Test msg builders: `press(r rune)` (home_test.go:36
     — special-cases Up/Down/Enter/Esc/Left/Right/Backspace; everything else
     carries `Text: string(r)`), `pressTab()`, `ctrlCKey`/`ctrlDKey`/
     `ctrlRKey` (home_test.go:44-49), `pressCtrlP()`/`pressCtrlA()`
     (model_test.go:24-25).
   - `tea.Tick(d, func(time.Time) tea.Msg)` for the leader timeout; v2
     `Update(msg tea.Msg) (tea.Model, tea.Cmd)`.
   - Harness: `testApp(sessions ...protocol.Session) *recApp` (home_test.go:29
     — dummy client, home.now = testNow), `newRecApp(c, s, startSessionID)`
     (rec_test.go:20 — nil engine), `themeApp(t) (*recApp, *theme.Engine)`
     (themedlg_test.go:20), `stripANSI` (home_test.go:20) /
     `stripANSITest` (app_test.go:32), `driveCmds`/`updateKey` (huhdlg_test.go),
     `suiteType` (tui_suite_test.go:27), `hasLine` (permission_test.go:256),
      `hasLines` (tui_suite_test.go:161), `agentFixture()` (model_test.go:61 —
      the `[]protocol.Agent` data fixture), `agentApp()` (agent_test.go:24 —
      the session-route agent-dialog app the S4.2 remap test drives),
      `modelFixture()` (model_test.go:70), `teatest` SGR legs pin
     TTY_FORCE=1 + TERM=xterm-256color.

2. **Upstream `config/keybind.ts` (v1.18.18, read in full at detail time):**
   - `LeaderDefault = "ctrl+x"` (line 41); `LeaderTimeoutDefault = 2000` ms
     (`config/index.tsx:21`).
   - `BindingValueSchema = false | "none" | BindingItem | BindingItem[]`;
     `BindingItem = string | KeyStroke | BindingObject` (lines 8-33);
     `KeyStroke = {name, ctrl?, shift?, meta?, super?, hyper?}`;
     `BindingObject = {key: string|KeyStroke, event?: "press"|"release",
     preventDefault?, fallthrough?}` (a record-with-rest schema).
   - 184 `Definitions` entries (lines 45-240), each
     `{default: BindingValue, description: string}`. The only object-shaped
     default: `input_paste: {key: "ctrl+v", preventDefault: false}` (line
     162). `theme_switch_mode`/`theme_mode_lock` = "none" (lines 79-80 —
     deviation 196's S3.9 note). `session_new: "<leader>n"` (line 89);
     `session_interrupt: "escape"` (line 97); `session_rename: "ctrl+r"`
     (line 93); `messages_page_up: "pageup,ctrl+alt+b"` /
     `messages_page_down: "pagedown,ctrl+alt+f"` (lines 135-136 — the V1
     pgup/pgdn pins); `command_list: "ctrl+p"` (line 57); `model_list:
     "<leader>m"` / `agent_list: "<leader>a"` (lines 121/129); `which_key_*`
     (lines 229-239).
   - `CommandMap` (lines 256-420): binding name → command (e.g.
     `command_list → "command.palette.show"`, `theme_list → "theme.switch"`,
     `session_new → "session.new"`).
   - `parse(overrides)` (lines 449-458): unknown keys →
     `Unrecognized keybind(s): <names>`; each name =
     `decode(overrides[name] ?? default)`. `unknownKeys` (lines 462-464).

3. **Upstream `keymap.tsx` (read in full at detail time):**
   - `LEADER_TOKEN = "leader"`, `OPENCODE_BASE_MODE = "base"`,
     `COMMAND_PALETTE_COMMAND = "command.palette.show"` (lines 20-22).
   - `createOpencodeModeStack` (lines 53-100): `current()` = the top mode or
     "base"; `push(mode)` returns a release fn that splices out THAT entry
     (identity, not mode name); `dispose()`.
   - `KEY_ALIASES = {enter→return, esc→escape, pgdown→pagedown, pgup→pageup}`
     (lines 112-117) — the matching-side normalization (the alias expander
     runs on every registered binding).
   - `registerTimedLeader(keymap, {trigger, name, timeoutMs})` (lines 220-225)
     — the leader trigger arms a timed pending sequence;
     `registerEscapeClearsPendingSequence` + `registerBackspacePopsPendingSequence`
     (lines 227-228); `useLeaderActive()` (lines 246-248) = a pending sequence
     whose first token is the leader.
   - `formatOptions` (lines 190-204): tokenDisplay leader → the resolved
     leader key; keyNameAliases {pageup→pgup, pagedown→pgdn, delete→del};
     modifierAliases {meta→alt}. `useCommandShortcut(command)` (lines 250-258)
     = the FIRST binding sequence formatted.
   - `useCommandSlashes` (lines 260-289): palette-namespace entries
     (visibility "reachable", filter `hidden !== true && name !==
     "command.palette.show"`) → `{display: "/"+slashName, description,
     aliases, onSelect: dispatch}`.
   - `inputCommands` (lines 136-173): the 36 textarea-scoped binding
     commands; `registerManagedTextareaLayer` (lines 229-232) — the input
     bindings own the keys while a textarea has focus (the upstream
     input-layer-wins semantics; yolo's prompt is always focused — deviation
     211).

4. **Upstream `component/command-palette.tsx` (read in full at detail time):**
   DialogSelect "Commands"; options from the palette-namespace reachable
   commands (excludes hidden + the palette command itself); each option
   `{title: command.title ?? name, description, category, footer:
   formatKeyBindings(bindings), value, suggested}`; list = no filter → the
   "Suggested" bucket (`value: suggested:<name>`, category "Suggested") + all
   options; filter → all options (the fuzzy filter narrows).

5. **Upstream `feature-plugins/system/which-key.tsx` (read in full at detail
   time):**
   - The plugin is **disabled by default** (`enabled: false`, line 597); the
     yolo port has no plugin registry — the port is always available (deviation
     207).
   - 11 commands (lines 9-20): toggle / toggleLayout / togglePending /
     groupPrevious / groupNext / scrollUp / scrollDown / pageUp / pageDown /
     home / end. LAYER_PRIORITY 900 (the global layer — the which-key bindings
     consume their keys even when the panel is closed, the upstream layer
     registration semantics).
   - KV `which_key_layout` ("dock"|"overlay", default "dock") +
     `which_key_pending_preview` (default false) (lines 21-22) — the yolo port
     keeps both **in-memory** (the theme KV is theme-owned — the S3.7
     precedent; no new KV surface — deviation 207).
   - Geometry (lines 26-38): COLUMN_GAP 4, TAB_GAP 3, MIN_TAB_GAP 1,
     TAB_CONTENT_GAP 1, MIN_COLUMN_WIDTH 28, MAX_COLUMN_WIDTH 44,
     PANEL_HEIGHT_RATIO 0.3, MIN_PANEL_HEIGHT 8, MAX_PANEL_HEIGHT 16,
     PANEL_TOP_PADDING 1, FOOTER_HEIGHT 1, FOOTER_MARGIN 1. `panelHeight =
     max(8, min(16, floor(h*0.3)))`; `contentWidth = max(1, w-2)`;
     `columns = max(1, min(3, floor((contentWidth+4)/(44+4)) || 1))`;
     `rows = max(1, panelHeight - 1 - header(0|1) - tabGap(0|1) -
     footer(0|2))`; `pageSize = rows*columns`.
   - Skin (lines 95-104): panel=backgroundMenu (#1c1c1c fallback), text
     (#f0f0f0), muted=textMuted (#a5a5a5), subtle=borderSubtle (#6f6f6f),
     key=warning (#ffd75f), accent=primary (#5f87ff), tab=primary,
     tabText=selectedListItemText (#ffffff). **opencode.json carries no
     `backgroundMenu` token** → `th.BackgroundMenu()` is empty; the yolo port
     falls back to the `backgroundPanel` token (opencode dark #141414) —
     deviation 207.
   - Entries (lines 110-152): `{key, label, group, continues}`; the label =
     the command title ?? binding desc ?? command desc ?? "Unknown"; a
     continues entry's label = its pending-token name and group = "System";
     the group = `commandAttrs.category ?? bindingAttrs.group ?? "Unknown"`;
     `grouped()`: per group, entries sorted (continues desc, label
     localeCompare, key localeCompare); groups sorted by label
     localeCompare.
   - Visibility: `pinned || (mode==="overlay" && pendingPreview && pending)`;
     pendingMode = visible && a pending sequence exists — pending mode lists
     ALL groups flattened with group headers (no tabs, no footer).
   - Panel render (lines 452-560): panel box = full width, `panelHeight`
     tall, panel bg, padding left/right/top 1; centered header row (group
     tabs — the selected tab: primary bg, bold, selectedListItemText fg — +
     the "↑ ↓" scroll indicator when scrollable); a 1-row gap when tabs are
     visible; the body rows (each row: up to `columns` centered items, gap 4;
     an entry = label muted (continues → accent) + key text bold,
     space-between at `columnWidth = max(1, min(44, floor((contentWidth -
     (columns-1)*4)/columns)))`; a group header = accent bold centered);
     footer (not pending mode): "toggle <trigger>" left + "<nextMode>
     <modeTrigger>" right.
   - `HomeHint` (lines 165-181): the home-bottom line "Show keyboard
     shortcuts with <trigger>" (muted; the trigger subtle).

6. **yolo anchors (verified at detail time):**
   - keys.go: `handleKey` ladder (permission > dialog > openers
     [dlgModelKey ctrl+p, dlgAgentsKey ctrl+a, homeKeyMap.Quit ctrl+c] > slash
     menu > route > prompt) (lines 22-55); `handleMenuKey` (63-83);
     `handlePromptKey`/`inputUpdate`/`promptEnter` (87-126 — the LOCKED
     soft-enter send semantics).
   - session.go: `sessKeyMap` {PageUp "pgup", PageDown "pgdown", Expand
     "alt+e", Think "alt+t", Rename "ctrl+r"} (lines 29-41);
     `handleSessionKey` (line 571: PageUp/PageDown/Expand/Think/Rename/esc —
     the esc branch aborts while busy, else returns home); `sessionHelp`
     const (line 66) "pgup/pgdn scroll · alt+e expand · alt+t think · esc
     abort/back".
   - home.go: `homeKeyMap` {Up "up", Down "down", Enter "enter", NewSess "n",
     Quit "ctrl+c"} (lines 25-32); `handleHomeKey` (line 328); `helpText`
     const (line 102) "↑/↓ move · enter open · n new · /help".
   - dialog.go: `dialogKind` (13 kinds; S4 adds `dlgPalette`); `dlgSize`
     60/88/116; the `dialog` payload struct (lines 78-93; S4 adds a `palette`
     payload? NO — the palette rides the `sel` field, the S2.9 convention);
     `pushModal/closeTopModal/replaceModal/clearModals` (191-231);
     `modalCanceler` (233); `modalInner` (line 381: the per-kind payload view
     — S4 adds the `dlgPalette` case `d.sel.view(w, h, a.theme)`);
     `handleDialogKey` (line 435: the modal esc/ctrl+c close first —
     `cancelInner` then `closeTopModal` — then the per-kind payload dispatch;
     S4 adds the `dlgPalette` case `d.sel.handleKey(a, k)`);
     `helpDialogView` (line 337: header + "Press {paletteShortcut()} to see
     all available actions and commands in any context." + the LOCKED note
     "pgup/pgdn scroll · \+enter newline" + the ok pill); `paletteShortcut()`
     (line 351: the constant "ctrl+p" — deviation 195's S4.7 rewire seam);
     `dlgModelKey`/`dlgAgentsKey` (lines 629-630: the openers S4.2 removes);
     `openModelDialog` (line 639: pushModal dlgLarge + syncModelSel +
     fetchCatalogCmd), `openAgentDialog` (line 862: pushModal dlgMedium +
     syncAgentSel + fetchCatalogCmd).
   - commands.go: `localCommands()` (4: /sessions /connect /status /themes);
     `mergedCommands()` (local + `store.Commands`); `runCommand(name)` (the
     /help /quit /exit /model /agents /sessions /connect /status /themes
     /new cases); `commandCmd` (POST /session/{id}/command).
   - select.go: `selectOption` {title, description, details, footer,
     category, value any, disabled, bg, gutter} (lines 26-36);
     `selectModel` {…, skipFilter, onFilter} (53-76); `selectNew(title,
     placeholder, options, isCurrent, onSelect, onMove)` (line 79 — the
     input is auto-focused); `WithActions/WithHints`; `filtered()` (line 116:
     the fuzzy title×2 + category×1 port); `handleKey` (line 183: actions
     first; pgup/pgdn ±10; tab/shift+tab; up/down/home/end; enter → focAct or
     submit; default → the input + syncFilter); `submit` (line 285: clamps
     sel, calls onSelect); `view(w, h int, th theme.Theme) string` (line
     303).
   - app.go: `App` struct (line 52 — S4 adds `keymap *Keymap`,
     `pendingLeader bool`, `whichKey whichKeyState`); `NewApp` (line 87 —
     S4.2 builds the default keymap); `Update` (line 144:
     `Update(msg) (tea.Model, tea.Cmd)`, the `case tea.KeyPressMsg` at line
     237 calls `a.handleKey` then `tea.Batch`; S4.2 adds the
     `leaderTimeoutMsg` case); `a.emit` (line 383).
   - view.go: `view()` (line 30: the top modal → `viewModal()`; else the
     route + menu + perm + prompt + toasts + dlg + lastErr + footer — S4.6
     inserts the which-key panel between lastErr and the footer, and
     suppresses the footer in overlay mode); `viewSession` (line 57);
     `modalChromeMin()` (line 95: session = title + viewport + divider +
     help; home = 4+1+1+help — S4.6's home hint line changes the home count
     +1); `sessionChrome` (line 109: the help line via `wrapLine(sessionHelp,
     w)` — S4.7 re-points the const to `a.sessionHelpLine()`).
   - theme/styles.go: `BackgroundMenu()` (line 114) returns an empty style
     when the token is absent (opencode.json: absent); `backgroundPanel`
     (opencode dark #141414) is the fallback token; `SelectedForeground(bg
     ...Rgba) Rgba` (line 77); `Color(name) (Rgba, bool)`.
   - client.go: `ListCommands(ctx) ([]protocol.Command, error)` (line 275,
     GET /command); `store.Commands` is populated by the hydrate (hydrate.go
     71/83/91 — fetch failures degrade to an empty slice).
   - protocol: `Command{Name, Description, Template, Hints}`; `Config`
     (config.go:34-44) — no `keybinds` field yet (S4.3 adds `Keybinds
     map[string]any json:"keybinds,omitempty"`).
   - config: `Loader.LoadAt(globalDir, startDir) (*protocol.Config, error)`
     (config.go:213: the global config.json/yolo.json/yolo.jsonc + the
     project chain + .yolo merge, the env substitution, the JSONC via
     tidwall); test pattern (config_test.go: the `write(t, path, jsonc)`
     helper + `config.Loader{Env: nil}.LoadAt(t.TempDir(), work)`).
   - cmd/yolo/main.go (lines 205-246): `cfg` is loaded via
     `loader.LoadAt` BEFORE `tui.NewApp`; S4.3 inserts `app.SetKeybinds(cfg.Keybinds)`
     between `NewApp` (line 235) and `program.Run()` (line 246).
   - Server: the `GET /config` golden testdata encodes configs without a
     `keybinds` field → `omitempty` leaves every golden byte-identical
     (grep-verified at detail time: no `keybinds` in
     `internal/server/testdata/`).

7. **V1 pins (binding — internal/tui/AGENTS.md):** the keymap is pgup/pgdn
   scroll + `\`+enter newline (noted in /help). The registry carries both:
   `messages_page_up`/`messages_page_down` defaults (first sequences
   "pageup"/"pagedown" → display "pgup"/"pgdn") and the yolo-specific
   `prompt_soft_newline` display entry (deviation 208). The locked /help
   note line + the locked `promptEnter` soft-enter semantics are untouched
   by the S4 rewire.

### Design decisions (binding)

**Registry data model (S4.1, `internal/tui/keymap.go`):**
- `Definitions map[string]keybindDef` — the 184 upstream entries ported
  VERBATIM (name, default, description) + the yolo-specific
  `prompt_soft_newline` display entry (deviation 208) = 185 entries;
   `CommandMap map[string]string` — the ported binding→command map (the 163
   upstream command bindings; the 21 non-command bindings — `leader`, the 13
   `dialog.*` + 5 `prompt.autocomplete.*` navigation bindings,
   `permission.prompt.fullscreen`, `plugins.toggle` — are absent from the
   upstream CommandMap by design).
- `BindingValue` = the raw upstream value shape (`any`: bool | string |
  `[]any` | `map[string]any`); `resolveValue` normalizes it to
   `bindingValue{enabled bool, seqs []string}`: `false`/`"none"`/empty →
   disabled; a string → its comma-separated seqs (the upstream default
   format, e.g. "ctrl+c,ctrl+d,<leader>q" → 3 seqs); a list → each item; a
   map with a `"key"`
  field → a binding object (its key string, or the stringified keystroke);
  a map with a `"name"` field → a keystroke object
  (`stringifyKeyStroke`: name + the ctrl/shift/meta/super/hyper flags in the
  schema field order, joined "+"); anything else → error. The object flags
  (event/preventDefault/fallthrough) are retained as parsed data only —
  the yolo matcher has no opentui runtime to apply them (deviation 209).
- Matching: `keyMatchesSeq(k tea.KeyPressMsg, seq string) bool` — both sides
  alias-normalized via the upstream KEY_ALIASES (enter→return, esc→escape,
  pgdown→pagedown, pgup→pageup); the modifier SETS are compared exactly
  (order-insensitive); the base key is compared case-insensitively. The
  `<leader>` token never matches raw (the pending mechanism owns it).
- Display: `formatKeySequence(seq, leader)` — the `<leader>` token → the
  resolved leader key (the remainder after the token, space-joined:
  "<leader>t" → "ctrl+x t"); the display aliases pageup→pgup, pagedown→pgdn,
  delete→del + the yolo escape→esc (the yolo surface convention — the select
  hint "esc close"; deviation 214); the modifier alias meta→alt.
  `Keymap.Format(name)` = "none" when disabled, else the formatted sequences
  joined by " / " (the upstream keymap-library join is not visible from the
  repo — deviation 214).

**Keymap + context groups + leader (S4.2):**
- `Keymap{bindings map[string]bindingValue, modes []modeEntry, nextID int}`;
  `NewKeymap(overrides map[string]any) (*Keymap, error)` = the ported
  upstream `parse` (unknown keys → the error `unrecognized keybind(s):
  <sorted names>` — the Go lowercase convention; the absent name → the
  default); `Set(name, v) error` = the runtime remap (immediately effective:
  every keypress re-reads the table); `Match(name, k)` (the non-pending
  path: the `<leader>` sequences are skipped); `MatchPending(name, k)` (the
  second key after the leader, matched against the remainder); `Current()`
  / `Push(mode) func()` (the ported mode stack — identity splice).
- `contextGroups map[string][]string` — the yolo context→binding-name groups
  (the upstream context/mode-scoped bindings have no single referent file —
  the groups are the yolo port): `"base"` = the app-level openers (any
  route, no dialog, no pending permission) in match order:
  [which_key_toggle, which_key_layout_toggle, which_key_pending_toggle,
  command_list, app_exit, model_list, agent_list, status_view, theme_list,
  session_new, session_list]; `"session"` = [messages_page_up,
  messages_page_down, session_interrupt, session_rename].
- The dispatch scoping (deviation 211 — behavior/medium): the yolo-specific
  surface keys with NO upstream referent (home up/down/enter/n, the session
  alt+e/alt+t toggles, the prompt's locked soft-enter, the dialog payload
  keys) remain the current `key.Binding` surfaces; the registry is the
  single source for the ported upstream bindings. Concretely:
  - the opener ladder (any route, no dialog): the leader mechanism first
    (the `leader` binding's seqs arm the pending state), then the base
    group in order — each binding's seqs are matched, EXCEPT `app_exit`'s
    `ctrl+d` seq (prompt-owned: the `input_delete` default
    "ctrl+d,delete,shift+delete" — the upstream input-layer-wins semantics;
    the yolo prompt is always focused) and the `<leader>` seqs (the leader
    mechanism owns them). The matched name → `dispatchCommand` (the
    referent-bearing commands; the no-referent ones are no-ops): app_exit →
    the quit dialog (the ctrl+c seq — the current yolo behavior);
    command_list → S4.4's `openPaletteDialog` (the case lands in S4.4 — at
    S4.2 time the match is consumed and does nothing: the model dialog's
    ctrl+p opener is freed); model/agent/status/theme/session_new/
    session_list → the existing openers/`createSessionCmd` (reachable only
    via `<leader>` at S4.2 time — their defaults are all `<leader>*`).
  - the **ctrl+p remap**: ctrl+p now opens the command palette (the upstream
    `command_list`) — the model dialog's opener frees to `<leader>m` +
    `/model`; the **ctrl+a remap**: ctrl+a frees to the prompt input (the
    upstream `input_line_home` default "ctrl+a") — the agent dialog's
    opener frees to `<leader>a` + `/agents`. Both are behavior changes
    (deviation 211) — the V1 pins are untouched.
  - the session route: the registry-backed keys (messages_page_up/down,
    session_interrupt, session_rename — the current handleSessionKey order
    preserved: page up, page down, then the yolo alt+e/alt+t surface
    toggles, then rename, then interrupt); the home route: the surface keys
    unchanged (up/down/enter/n — the 'n' is the yolo home context value,
    deviation 210; the home help line stays byte-identical).
- The leader mechanism (`App.pendingLeader bool` + `leaderTimeoutMsg`):
  armed by the `leader` binding's seqs (default ctrl+x) when no dialog is
  open and no permission is pending — the key is consumed, a
  `tea.Tick(LeaderTimeout=2000ms, leaderTimeoutMsg{})` cmd is returned;
  while pending: esc clears (consumed — the
  registerEscapeClearsPendingSequence port); a second key matching a base
  binding's `<leader>` sequence (group order) dispatches it (consumed); a
  non-matching second key clears AND re-dispatches through the rest of the
  ladder (the key is not lost — the sane-UX choice; the upstream
  keymap-library replay is not verifiable from the repo); the timeout msg
  clears. A leader keypress while pending re-arms (the clear + re-dispatch
  path matches the leader again).

**Command palette (S4.4/S4.5):**
- `openPaletteDialog() []tea.Cmd`: the options = `a.mergedCommands()` (the
  4 local commands FIRST, then the `GET /command` catalog — the slash-menu
  convention; an empty pre-hydrate catalog degrades to the locals); each
  option `{title: name minus the leading "/", description: the Description,
  footer: the registry binding's `Format` (via `commandBindings`, "" when
  "none"), value: the /name}`; the select = `selectNew("Commands", "Filter
  commands", opts, nil, a.paletteSelectPick, nil)` (the S2.5 fuzzy filter
  ACTIVE — no skipFilter); the dialog = `pushModal(dialog{kind: dlgPalette,
  sel: m}, dlgMedium, nil)` (the upstream DialogSelect default size medium —
  the S3 convention). The `commandBindings` map = the yolo command names →
  the registry binding names (the referent subset: /help→help_show,
  /new→session_new, /model→model_list, /agents→agent_list,
  /quit→app_exit, /sessions→session_list, /connect→provider_connect,
  /status→status_view, /themes→theme_list).
- The **Suggested bucket is not ported** (deviation 212): the upstream
  bucket comes from the command's `suggested` flag — the yolo wire catalog
  (`protocol.Command`) has no such field; the list = the merged order with
  the fuzzy filter.
- `paletteSelectPick(a *App, o selectOption)`: `closeTopModal()` then
  `runCommand(o.value.(string))` (the run-on-enter contract — S4.5; the
  select's existing arrow nav + esc close ride the S2.5 select + the S2.2
  modal stack).
- The `dispatchCommand` "command_list" case lands in S4.4 (S4.2's match is
  consumed-but-inert until then).

**Which-key (S4.6/S4.7):**
- `whichKeyState{open, layout ("dock"|"overlay"), preview, group, offset}`
  on the App (in-memory only — deviation 207). Methods: `toggle`,
  `toggleLayout`, `togglePreview`, `moveGroup(d, groups)`, `scroll(d,
  pageSize)`, `page(d, pageSize)`, `jump(offset)` — dispatched via
  `dispatchCommand` (the which_key_* names — the global-layer semantics:
  they consume their keys even when the panel is closed).
- `whichKeyPanelHeight(h) = max(8, min(16, floor(h*0.3)))`;
  `whichKeyColumns(w) = max(1, min(3, floor((max(1,w-2)+4)/48) || 1))`
  (the ported geometry).
- `whichKeyEntries(pending bool)`: pending=false → the base-group bindings
  (enabled only; the FIRST seq per binding — the upstream active-key
  display): `{key: the formatted seq, label: the Definitions description,
  group: whichKeyCategory(name)}`; pending=true → the `<leader>`
  continuations: `{key: the formatted remainder, label: "+" + the key
  display, group: "System", continues: true}` (the upstream continues label
  = the pending tokenName — the keymap-library internals are not verifiable
  from the repo; the yolo port labels with the completing key — deviation
  213). `whichKeyCategory` = the group label from the binding-name prefix
  (the upstream category attributes are per-command registrations not
  present in the ported surface set — deviation 213): which_key_*→System,
  app_*/sidebar_*/scrollbar_*/debug_*→App, command_*→Commands,
  help_*→Help, docs_*→Docs, diff_*→Diff, editor_*→Editor,
  theme_*→Theme, status_*→Status, session_*/tool_*/display_*→Session,
  stash_*→Stash, model_*→Model, mcp_*→MCP, provider_*→Provider,
  console_*→Console, agent_*→Agent, variant_*→Variants,
  messages_*→Messages, prompt_*→Prompt, workspace_*→Workspace,
  input_*→Input, history_*→History, dialog.*→Dialog,
  permission.*→Permission, plugins.*→Plugins, plugin_*→Plugins,
  terminal_*→Terminal, tips_*→Tips, else Unknown. The grouping = the
  upstream `grouped()` (entries per group sorted continues desc / label /
  key; groups sorted by label).
- `whichKeyView(w, h int, th theme.Theme) string` — the ported panel render:
  panel bg = `th.BackgroundMenu()`, falling back to the `backgroundPanel`
  token when the `backgroundMenu` token is absent (opencode.json — the SGR
  leg derives #141414 via the scratch command); the header row (the group
  tabs centered — the selected tab: `Primary()` bg + `SelectedForeground`
  fg + bold; the "↑ ↓" scroll indicator when scrollable); a 1-row gap when
  tabs are visible; the body rows (label `TextMuted()` — continues →
  `Accent()`; key `Text()` bold, space-between at the column width); the
  footer (not pending mode): "toggle <trigger>" (`trigger` = the registry's
  `which_key_toggle` formatted, subtle) + "<nextMode> <modeTrigger>".
  Pending mode: all groups flattened with group headers (`Accent()` bold
  centered), no tabs, no footer.
- The panel visibility: `open || (pendingLeader && preview &&
  layout=="overlay")` (the ported visible memo). The **dock** renders the
  panel between lastErr and the footer in `view()` (the upstream app_bottom
  slot) with the footer below it; the **overlay** renders the panel in place
  of the footer (the upstream absolute bottom overlay has no yolo referent —
  the yolo frame is the fixed terminal height; the overlay covers the footer
  — deviation 207). Both are suppressed while a modal is open (the modal
  owns the frame).
- The home hint: `home.render` appends "Show keyboard shortcuts with
  <trigger>" (muted; the trigger subtle) after the help line (the upstream
  HomeHint port) — `TestHomeRenderLockedLayout` re-baselines with the extra
  line (the teatest home legs are substring pins — untouched);
  `modalChromeMin` home count +1.
- **S4.7 registry integration**: `paletteShortcut()` → `a.keymap.Format("command_list")`
  (deviation 195's rewire seam — the default renders "ctrl+p" byte-identical
  to today); `sessionHelpLine()` renders the session footer line from the
  registry: `<messages_page_up first>/<messages_page_down first> scroll ·
  alt+e expand · alt+t think · <session_interrupt first> abort/back` — with
  the defaults "pgup/pgdn scroll · alt+e expand · alt+t think · esc
  abort/back" BYTE-IDENTICAL to the removed `sessionHelp` const (the V1
  pins); the `helpText` const stays (the home surface keys have no upstream
  referent — the line is byte-identical; deviation 211's scope); the locked
  /help note line ("pgup/pgdn scroll · \+enter newline") is untouched.

### Task S4.1: Keymap registry: upstream default bindings (bead `yolo-oae.5.1`, expected id `yolo-oae.5.2`)

**Files:** new `internal/tui/keymap.go`; new `internal/tui/keymap_test.go`.

**Interfaces:** produces: the `LeaderDefault`/`LeaderToken`/`LeaderTimeout`/`BaseMode` constants; `BindingValue` (the raw value shape); `keybindDef` + the `keybind()` helper; `Definitions` (185 entries); `CommandMap` (163 entries — the upstream command set; the 21 non-command bindings are absent); `bindingValue{enabled, seqs}`; `resolveValue(BindingValue) (bindingValue, error)`; `stringifyKeyStroke(map[string]any) (string, error)`; `keyMatchesSeq(tea.KeyPressMsg, string) bool`; `formatKeySequence(seq, leader string) string`; `leaderSplit(seq string) (has, rest bool)`. No app wiring yet (S4.2).

**Upstream parity notes:** `config/keybind.ts` ported verbatim (findings §2): the 184 entries' names, defaults and descriptions are unchanged; the value-shape decoder is the port of `BindingValueSchema` + `parse`'s per-value decode (the unknown-key check lands in S4.2's `NewKeymap` — the same message shape). The object flags are data-only (deviation 209). The base key compares case-insensitively (a port simplification — the upstream keymap-library case handling is not visible from the repo).

**Step 1 — write the failing tests.** New `internal/tui/keymap_test.go`:

```go
package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestKeymapDefinitionsVerbatim(t *testing.T) {
	if len(Definitions) != 185 {
		t.Fatalf("Definitions = %d entries, want 185 (the 184 upstream + the yolo prompt_soft_newline)", len(Definitions))
	}
	// The upstream defaults (keybind.ts) — spot checks across the value
	// shapes (the verbatim port bar).
	cases := map[string]string{
		"leader":             "ctrl+x",
		"command_list":       "ctrl+p",
		"session_interrupt":  "escape",
		"session_rename":     "ctrl+r",
		"session_new":        "<leader>n",
		"messages_page_up":   "pageup,ctrl+alt+b",
		"messages_page_down": "pagedown,ctrl+alt+f",
		"app_exit":           "ctrl+c,ctrl+d,<leader>q",
		"which_key_toggle":   "ctrl+alt+k",
		"theme_switch_mode":  "none",
		"theme_mode_lock":    "none",
		"model_list":         "<leader>m",
		"agent_list":         "<leader>a",
		"input_newline":      "shift+return,ctrl+return,alt+return,ctrl+j",
	}
	for name, want := range cases {
		if got := Definitions[name].Default; got != want {
			t.Errorf("Definitions[%q].Default = %v, want %q", name, got, want)
		}
	}
	// The only object-shaped default (keybind.ts:162).
	paste, ok := Definitions["input_paste"].Default.(map[string]any)
	if !ok || paste["key"] != "ctrl+v" || paste["preventDefault"] != false {
		t.Errorf("input_paste default = %v, want {key: ctrl+v, preventDefault: false}", Definitions["input_paste"].Default)
	}
	// The yolo-specific display entry (deviation 208): the V1 soft-enter
	// pin has no upstream referent.
	if got := Definitions["prompt_soft_newline"].Default; got != "\\+enter" {
		t.Errorf("prompt_soft_newline = %v, want the V1 soft-enter sentinel", got)
	}
	// Every entry carries a description (the upstream Descriptions,
	// keybind.ts:253-255).
	for name, def := range Definitions {
		if def.Description == "" {
			t.Errorf("Definitions[%q].Description is empty", name)
		}
	}
	// The ported CommandMap is the upstream's 163-entry binding→command map
	// (keybind.ts:256-420) — verbatim. The 21 non-command bindings (leader,
	// the 13 dialog.* + 5 prompt.autocomplete.* navigation bindings,
	// permission.prompt.fullscreen, plugins.toggle) have no command and are
	// absent from the upstream CommandMap by design, so the assertion is
	// scoped to the CommandMap's own set (not the 184 Definitions names);
	// every ported command key must be a ported Definitions name.
	if len(CommandMap) != 163 {
		t.Fatalf("CommandMap = %d entries, want 163 (the upstream set)", len(CommandMap))
	}
	for name := range CommandMap {
		if _, ok := Definitions[name]; !ok {
			t.Errorf("CommandMap[%q] has no Definitions entry", name)
		}
	}
}

func TestKeymapResolveValue(t *testing.T) {
	tests := []struct {
		name string
		in   BindingValue
		want []string
		err  bool
	}{
		{"string", "ctrl+p", []string{"ctrl+p"}, false},
		{"comma list", "ctrl+c,ctrl+d,<leader>q", []string{"ctrl+c", "ctrl+d", "<leader>q"}, false},
		{"comma with none", "a,none,b", []string{"a", "b"}, false},
		{"none string", "none", nil, false},
		{"false", false, nil, false},
		{"nil", nil, nil, false},
		{"list", []any{"a", "b"}, []string{"a", "b"}, false},
		{"keystroke object", map[string]any{"name": "m", "ctrl": true}, []string{"ctrl+m"}, false},
		{"spec object", map[string]any{"key": "ctrl+v", "preventDefault": false}, []string{"ctrl+v"}, false},
		{"spec object keystroke key", map[string]any{"key": map[string]any{"name": "k", "shift": true}}, []string{"shift+k"}, false},
		{"number", 42, nil, true},
		{"empty map", map[string]any{}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveValue(tt.in)
			if (err != nil) != tt.err {
				t.Fatalf("err = %v, want err=%v", err, tt.err)
			}
			if tt.err {
				return
			}
			if len(got.seqs) != len(tt.want) {
				t.Fatalf("seqs = %v, want %v", got.seqs, tt.want)
			}
			for i := range tt.want {
				if got.seqs[i] != tt.want[i] {
					t.Fatalf("seqs = %v, want %v", got.seqs, tt.want)
				}
			}
		})
	}
}

func TestKeymapKeyMatchesSeq(t *testing.T) {
	tests := []struct {
		name string
		k    tea.KeyPressMsg
		seq  string
		want bool
	}{
		{"plain char", press('m'), "m", true},
		{"wrong char", press('m'), "n", false},
		{"ctrl+p", tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl}, "ctrl+p", true},
		{"no ctrl", press('p'), "ctrl+p", false},
		{"two mods", tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl | tea.ModAlt}, "ctrl+alt+b", true},
		{"seq mod order reversed", tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl | tea.ModAlt}, "alt+ctrl+b", true},
		{"extra mod", tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl | tea.ModAlt | tea.ModShift}, "ctrl+alt+b", false},
		{"enter=return", tea.KeyPressMsg{Code: tea.KeyEnter}, "return", true},
		{"return=enter", tea.KeyPressMsg{Code: tea.KeyEnter}, "enter", true},
		{"esc=escape", tea.KeyPressMsg{Code: tea.KeyEscape}, "escape", true},
		{"escape=esc", tea.KeyPressMsg{Code: tea.KeyEscape}, "esc", true},
		{"pgup=pageup", tea.KeyPressMsg{Code: tea.KeyPgUp}, "pageup", true},
		{"pageup=pgup", tea.KeyPressMsg{Code: tea.KeyPgUp}, "pgup", true},
		{"pgdown=pagedown", tea.KeyPressMsg{Code: tea.KeyPgDown}, "pagedown", true},
		{"shift+a", tea.KeyPressMsg{Code: 'a', Mod: tea.ModShift}, "shift+a", true},
		{"uppercase default seq", tea.KeyPressMsg{Code: 'E'}, "E", true},
		{"backspace", press(tea.KeyBackspace), "backspace", true},
		{"leader token does not match raw", tea.KeyPressMsg{Code: 'm'}, "<leader>m", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := keyMatchesSeq(tt.k, tt.seq); got != tt.want {
				t.Fatalf("keyMatchesSeq(%v, %q) = %v, want %v", tt.k, tt.seq, got, tt.want)
			}
		})
	}
}

func TestKeymapFormatKeySequence(t *testing.T) {
	// The display aliases (the upstream keyNameAliases + the yolo
	// escape→esc — deviation 214).
	if got := formatKeySequence("pageup", "ctrl+x"); got != "pgup" {
		t.Errorf("formatKeySequence(pageup) = %q, want pgup", got)
	}
	if got := formatKeySequence("pagedown", "ctrl+x"); got != "pgdn" {
		t.Errorf("formatKeySequence(pagedown) = %q, want pgdn", got)
	}
	if got := formatKeySequence("escape", "ctrl+x"); got != "esc" {
		t.Errorf("formatKeySequence(escape) = %q, want esc", got)
	}
	if got := formatKeySequence("delete", "ctrl+x"); got != "del" {
		t.Errorf("formatKeySequence(delete) = %q, want del", got)
	}
	// The <leader> token expands to the resolved leader key.
	if got := formatKeySequence("<leader>t", "ctrl+x"); got != "ctrl+x t" {
		t.Errorf("formatKeySequence(<leader>t) = %q, want ctrl+x t", got)
	}
	// The modifier alias meta→alt.
	if got := formatKeySequence("meta+k", "ctrl+x"); got != "alt+k" {
		t.Errorf("formatKeySequence(meta+k) = %q, want alt+k", got)
	}
	// A plain seq passes through (the alias table applies to the base +
	// the modifier positions only).
	if got := formatKeySequence("ctrl+p", "ctrl+x"); got != "ctrl+p" {
		t.Errorf("formatKeySequence(ctrl+p) = %q, want ctrl+p", got)
	}
}
```

**Step 2 — confirm FAIL.** `go test ./internal/tui/ -run 'TestKeymap' -count=1` → FAIL (build fails: undefined `Definitions`, `resolveValue`, `keyMatchesSeq`, `formatKeySequence` — the expected red).

**Step 3 — minimal implementation.** New `internal/tui/keymap.go`:

```go
// keymap.go — the keymap registry (S4): the single source for every TUI
// binding. S4.1 ports the upstream default bindings (config/keybind.ts @
// v1.18.18) verbatim + the value-shape decoder + the seq matcher/formatter;
// S4.2 adds the Keymap (context groups, the mode stack, the leader, the
// runtime remap); S4.3 wires the yolo.jsonc keybinds schema.

package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

// The upstream keymap constants (keybind.ts:41, config/index.tsx:21,
// keymap.tsx:20-21).
const (
	LeaderDefault = "ctrl+x"
	LeaderToken   = "leader"
	LeaderTimeout = 2000 * time.Millisecond
	BaseMode      = "base"
)

// BindingValue is the upstream BindingValueSchema (keybind.ts:28-33):
// false | "none" (disabled) | a sequence string | a list of items | a
// keystroke object | a binding object — carried raw (any) until
// resolveValue normalizes it.
type BindingValue any

type keybindDef struct {
	Default     BindingValue
	Description string
}

func keybind(def BindingValue, description string) keybindDef {
	return keybindDef{Default: def, Description: description}
}

// Definitions is the ported upstream default bindings (keybind.ts:45-240) —
// verbatim (names, defaults, descriptions) — plus the yolo-specific
// prompt_soft_newline display entry (deviation 208): the V1-pinned
// trailing-backslash soft-enter has no upstream referent (input_newline is
// the upstream newline binding); the entry carries the gesture for the
// registry-driven /help + which-key rendering (display-only sentinel — the
// gesture is handled by the prompt fallback, not the matcher).
var Definitions = map[string]keybindDef{
	"leader": keybind(LeaderDefault, "Leader key for keybind combinations"),

	"app_exit":                          keybind("ctrl+c,ctrl+d,<leader>q", "Exit the application"),
	"app_debug":                         keybind("none", "Toggle debug panel"),
	"app_console":                       keybind("none", "Toggle console"),
	"app_heap_snapshot":                 keybind("none", "Write heap snapshot"),
	"app_toggle_animations":             keybind("none", "Toggle animations"),
	"app_toggle_file_context":           keybind("none", "Toggle file context"),
	"app_toggle_diffwrap":               keybind("none", "Toggle diff wrapping"),
	"app_toggle_paste_summary":          keybind("none", "Toggle paste summary"),
	"app_toggle_session_directory_filter": keybind("none", "Toggle session directory filtering"),
	"command_list":                      keybind("ctrl+p", "List available commands"),
	"help_show":                         keybind("none", "Open help dialog"),
	"docs_open":                         keybind("none", "Open documentation"),
	"diff_open":                         keybind("none", "Open diff viewer"),
	"diff_close":                        keybind("escape,q", "Close diff viewer"),
	"diff_toggle":                       keybind("enter,space", "Toggle diff viewer item"),
	"diff_expand":                       keybind("right", "Expand diff viewer item"),
	"diff_expand_all":                   keybind("E", "Expand all diff viewer folders"),
	"diff_collapse":                     keybind("left", "Collapse diff viewer item"),
	"diff_switch_focus":                 keybind("tab", "Switch diff viewer focus"),
	"diff_next_hunk":                    keybind("]", "Jump to next diff hunk"),
	"diff_previous_hunk":                keybind("[", "Jump to previous diff hunk"),
	"diff_next_file":                    keybind("n", "Jump to next diff file"),
	"diff_previous_file":                keybind("p", "Jump to previous diff file"),
	"diff_toggle_file_tree":             keybind("b", "Toggle diff viewer file tree"),
	"diff_single_patch":                 keybind("s", "Toggle single patch view"),
	"diff_switch_source":                keybind("d", "Switch diff viewer source"),
	"diff_toggle_view":                  keybind("v", "Toggle diff viewer split or unified view"),
	"diff_help":                         keybind("?", "Show more diff viewer shortcuts"),

	"editor_open":  keybind("<leader>e", "Open external editor"),
	"theme_list":   keybind("<leader>t", "List available themes"),
	"theme_switch_mode": keybind("none", "Switch between light and dark theme mode"),
	"theme_mode_lock":   keybind("none", "Lock or unlock theme mode"),
	"sidebar_toggle":    keybind("<leader>b", "Toggle sidebar"),
	"scrollbar_toggle":  keybind("none", "Toggle session scrollbar"),
	"status_view":       keybind("<leader>s", "View status"),
	"debug_view":        keybind("none", "View debug info"),

	"session_export":                       keybind("<leader>x", "Export session to editor"),
	"session_copy":                         keybind("none", "Copy session transcript"),
	"session_move":                         keybind("none", "Move session"),
	"session_new":                          keybind("<leader>n", "Create a new session"),
	"session_list":                         keybind("<leader>l", "List all sessions"),
	"session_timeline":                     keybind("<leader>g", "Show session timeline"),
	"session_fork":                         keybind("none", "Fork session from message"),
	"session_rename":                       keybind("ctrl+r", "Rename session"),
	"session_delete":                       keybind("ctrl+d", "Delete session"),
	"session_share":                        keybind("none", "Share current session"),
	"session_unshare":                      keybind("none", "Unshare current session"),
	"session_interrupt":                    keybind("escape", "Interrupt current session"),
	"session_background":                   keybind("ctrl+b", "Background synchronous subagents"),
	"session_compact":                      keybind("<leader>c", "Compact the session"),
	"session_toggle_timestamps":            keybind("none", "Toggle message timestamps"),
	"session_toggle_generic_tool_output":   keybind("none", "Toggle generic tool output"),
	"session_queued_prompts":               keybind("<leader>q", "Manage queued prompts"),
	"session_child_first":                  keybind("<leader>down", "Go to first child session"),
	"session_child_cycle":                  keybind("right", "Go to next child session"),
	"session_child_cycle_reverse":          keybind("left", "Go to previous child session"),
	"session_parent":                       keybind("up", "Go to parent session"),
	"session_pin_toggle":                   keybind("ctrl+f", "Pin or unpin session in the session list"),
	"session_quick_switch_1":               keybind("<leader>1", "Switch to session in quick slot 1"),
	"session_quick_switch_2":               keybind("<leader>2", "Switch to session in quick slot 2"),
	"session_quick_switch_3":               keybind("<leader>3", "Switch to session in quick slot 3"),
	"session_quick_switch_4":               keybind("<leader>4", "Switch to session in quick slot 4"),
	"session_quick_switch_5":               keybind("<leader>5", "Switch to session in quick slot 5"),
	"session_quick_switch_6":               keybind("<leader>6", "Switch to session in quick slot 6"),
	"session_quick_switch_7":               keybind("<leader>7", "Switch to session in quick slot 7"),
	"session_quick_switch_8":               keybind("<leader>8", "Switch to session in quick slot 8"),
	"session_quick_switch_9":               keybind("<leader>9", "Switch to session in quick slot 9"),

	"stash_delete": keybind("ctrl+d", "Delete stash entry"),

	"model_provider_list":      keybind("ctrl+a", "Open provider list from model dialog"),
	"model_favorite_toggle":    keybind("ctrl+f", "Toggle model favorite status"),
	"model_list":               keybind("<leader>m", "List available models"),
	"model_cycle_recent":       keybind("f2", "Next recently used model"),
	"model_cycle_recent_reverse": keybind("shift+f2", "Previous recently used model"),
	"model_cycle_favorite":     keybind("none", "Next favorite model"),
	"model_cycle_favorite_reverse": keybind("none", "Previous recently used model"),
	"mcp_list":                 keybind("none", "List MCP servers"),
	"provider_connect":         keybind("none", "Connect provider"),
	"console_org_switch":       keybind("none", "Switch console organization"),
	"agent_list":               keybind("<leader>a", "List agents"),
	"agent_cycle":              keybind("tab", "Next agent"),
	"agent_cycle_reverse":      keybind("shift+tab", "Previous agent"),
	"variant_cycle":            keybind("ctrl+t", "Cycle model variants"),
	"variant_list":             keybind("none", "List model variants"),

	"messages_page_up":          keybind("pageup,ctrl+alt+b", "Scroll messages up by one page"),
	"messages_page_down":        keybind("pagedown,ctrl+alt+f", "Scroll messages down by one page"),
	"messages_line_up":          keybind("ctrl+alt+y", "Scroll messages up by one line"),
	"messages_line_down":        keybind("ctrl+alt+e", "Scroll messages down by one line"),
	"messages_half_page_up":     keybind("ctrl+alt+u", "Scroll messages up by half page"),
	"messages_half_page_down":   keybind("ctrl+alt+d", "Scroll messages down by half page"),
	"messages_first":            keybind("ctrl+g,home", "Navigate to first message"),
	"messages_last":             keybind("ctrl+alt+g,end", "Navigate to last message"),
	"messages_next":             keybind("none", "Navigate to next message"),
	"messages_previous":         keybind("none", "Navigate to previous message"),
	"messages_last_user":        keybind("none", "Navigate to last user message"),
	"messages_copy":             keybind("<leader>y", "Copy message"),
	"messages_undo":             keybind("<leader>u", "Undo message"),
	"messages_redo":             keybind("<leader>r", "Redo message"),
	"messages_toggle_conceal":   keybind("<leader>h", "Toggle code block concealment in messages"),
	"tool_details":              keybind("none", "Toggle tool details visibility"),
	"display_thinking":          keybind("none", "Toggle thinking blocks visibility"),

	"prompt_submit":               keybind("none", "Submit prompt"),
	"prompt_editor_context_clear": keybind("none", "Clear editor context"),
	"prompt_skills":               keybind("none", "Open skill selector"),
	"prompt_stash":                keybind("none", "Stash prompt"),
	"prompt_stash_pop":            keybind("none", "Pop stashed prompt"),
	"prompt_stash_list":           keybind("none", "List stashed prompts"),
	"workspace_set":               keybind("none", "Set workspace"),

	"input_clear":                  keybind("ctrl+c", "Clear input field"),
	"input_paste":                  keybind(map[string]any{"key": "ctrl+v", "preventDefault": false}, "Paste from clipboard"),
	"input_submit":                 keybind("return", "Submit input"),
	"input_newline":                keybind("shift+return,ctrl+return,alt+return,ctrl+j", "Insert newline in input"),
	"input_move_left":              keybind("left,ctrl+b", "Move cursor left in input"),
	"input_move_right":             keybind("right,ctrl+f", "Move cursor right in input"),
	"input_move_up":                keybind("up", "Move cursor up in input"),
	"input_move_down":              keybind("down", "Move cursor down in input"),
	"input_select_left":            keybind("shift+left", "Select left in input"),
	"input_select_right":           keybind("shift+right", "Select right in input"),
	"input_select_up":              keybind("shift+up", "Select up in input"),
	"input_select_down":            keybind("shift+down", "Select down in input"),
	"input_line_home":              keybind("ctrl+a", "Move to start of line in input"),
	"input_line_end":               keybind("ctrl+e", "Move to end of line in input"),
	"input_select_line_home":       keybind("ctrl+shift+a", "Select to start of line in input"),
	"input_select_line_end":        keybind("ctrl+shift+e", "Select to end of line in input"),
	"input_visual_line_home":       keybind("alt+a", "Move to start of visual line in input"),
	"input_visual_line_end":        keybind("alt+e", "Move to end of visual line in input"),
	"input_select_visual_line_home": keybind("alt+shift+a", "Select to start of visual line in input"),
	"input_select_visual_line_end":  keybind("alt+shift+e", "Select to end of visual line in input"),
	"input_buffer_home":            keybind("home", "Move to start of buffer in input"),
	"input_buffer_end":             keybind("end", "Move to end of buffer in input"),
	"input_select_buffer_home":     keybind("shift+home", "Select to start of buffer in input"),
	"input_select_buffer_end":      keybind("shift+end", "Select to end of buffer in input"),
	"input_delete_line":            keybind("ctrl+shift+d", "Delete line in input"),
	"input_delete_to_line_end":     keybind("ctrl+k", "Delete to end of line in input"),
	"input_delete_to_line_start":   keybind("ctrl+u", "Delete to start of line in input"),
	"input_backspace":              keybind("backspace,shift+backspace", "Backspace in input"),
	"input_delete":                 keybind("ctrl+d,delete,shift+delete", "Delete character in input"),
	"input_undo":                   keybind("ctrl+-,super+z", "Undo in input"),
	"input_redo":                   keybind("ctrl+.,super+shift+z", "Redo in input"),
	"input_word_forward":           keybind("alt+f,alt+right,ctrl+right", "Move word forward in input"),
	"input_word_backward":          keybind("alt+b,alt+left,ctrl+left", "Move word backward in input"),
	"input_select_word_forward":    keybind("alt+shift+f,alt+shift+right", "Select word forward in input"),
	"input_select_word_backward":   keybind("alt+shift+b,alt+shift+left", "Select word backward in input"),
	"input_delete_word_forward":    keybind("alt+d,alt+delete,ctrl+delete", "Delete word forward in input"),
	"input_delete_word_backward":   keybind("ctrl+w,ctrl+backspace,alt+backspace", "Delete word backward in input"),
	"input_select_all":             keybind("super+a", "Select all in input"),
	"history_previous":             keybind("up", "Previous history item"),
	"history_next":                 keybind("down", "Next history item"),

	"dialog.select.prev":     keybind("up,ctrl+p", "Move to previous dialog item"),
	"dialog.select.next":     keybind("down,ctrl+n", "Move to next dialog item"),
	"dialog.select.page_up":  keybind("pageup", "Move up one page in dialog"),
	"dialog.select.page_down": keybind("pagedown", "Move down one page in dialog"),
	"dialog.select.home":     keybind("home", "Move to first dialog item"),
	"dialog.select.end":      keybind("end", "Move to last dialog item"),
	"dialog.select.submit":   keybind("return", "Submit selected dialog item"),
	"dialog.prompt.submit":   keybind("return", "Submit dialog prompt"),
	"dialog.mcp.toggle":      keybind("space", "Toggle MCP in MCP dialog"),
	"dialog.move_session.new":     keybind("ctrl+m", "New project copy"),
	"dialog.move_session.delete":  keybind("ctrl+d", "Delete project copy"),
	"dialog.move_session.refresh": keybind("ctrl+r", "Refresh project copies"),
	"prompt.autocomplete.prev":    keybind("up,ctrl+p", "Move to previous autocomplete item"),
	"prompt.autocomplete.next":    keybind("down,ctrl+n", "Move to next autocomplete item"),
	"prompt.autocomplete.hide":    keybind("escape", "Hide autocomplete"),
	"prompt.autocomplete.select":  keybind("return", "Select autocomplete item"),
	"prompt.autocomplete.complete": keybind("tab", "Complete autocomplete item"),
	"permission.prompt.fullscreen": keybind("ctrl+f", "Toggle permission prompt fullscreen"),
	"plugins.toggle":               keybind("space", "Toggle plugin"),
	"dialog.plugins.install":       keybind("shift+i", "Install plugin from plugin dialog"),

	"terminal_suspend":    keybind("ctrl+z", "Suspend terminal"),
	"terminal_title_toggle": keybind("none", "Toggle terminal title"),
	"tips_toggle":         keybind("<leader>h", "Toggle tips on home screen"),
	"plugin_manager":      keybind("none", "Open plugin manager dialog"),
	"plugin_install":      keybind("none", "Install plugin"),

	"which_key_toggle":         keybind("ctrl+alt+k", "Toggle which-key panel"),
	"which_key_layout_toggle":  keybind("ctrl+alt+shift+k", "Switch which-key layout"),
	"which_key_pending_toggle": keybind("ctrl+alt+shift+p", "Toggle which-key pending preview"),
	"which_key_group_previous": keybind("ctrl+alt+left,ctrl+alt+[", "Previous which-key group"),
	"which_key_group_next":     keybind("ctrl+alt+right,ctrl+alt+]", "Next which-key group"),
	"which_key_scroll_up":      keybind("ctrl+alt+up,ctrl+alt+p", "Scroll which-key up"),
	"which_key_scroll_down":    keybind("ctrl+alt+down,ctrl+alt+n", "Scroll which-key down"),
	"which_key_page_up":        keybind("ctrl+alt+pageup", "Page which-key up"),
	"which_key_page_down":      keybind("ctrl+alt+pagedown", "Page which-key down"),
	"which_key_home":           keybind("ctrl+alt+home", "Jump to first which-key binding"),
	"which_key_end":            keybind("ctrl+alt+end", "Jump to last which-key binding"),

	// The yolo-specific display entry (deviation 208) — display-only.
	"prompt_soft_newline": keybind("\\+enter", "Soft-enter a newline (trailing backslash)"),
}

// CommandMap is the ported upstream binding→command map (keybind.ts:256-420)
// — verbatim.
var CommandMap = map[string]string{
	"app_exit":                           "app.exit",
	"app_debug":                          "app.debug",
	"app_console":                        "app.console",
	"app_heap_snapshot":                  "app.heap_snapshot",
	"app_toggle_animations":              "app.toggle.animations",
	"app_toggle_file_context":            "app.toggle.file_context",
	"app_toggle_diffwrap":                "app.toggle.diffwrap",
	"app_toggle_paste_summary":           "app.toggle.paste_summary",
	"app_toggle_session_directory_filter": "app.toggle.session_directory_filter",
	"command_list":                       "command.palette.show",
	"help_show":                          "help.show",
	"docs_open":                          "docs.open",
	"diff_open":                          "diff.open",
	"diff_close":                         "diff.close",
	"diff_toggle":                        "diff.toggle",
	"diff_expand":                        "diff.expand",
	"diff_expand_all":                    "diff.expand_all",
	"diff_collapse":                      "diff.collapse",
	"diff_switch_focus":                  "diff.switch_focus",
	"diff_next_hunk":                     "diff.next_hunk",
	"diff_previous_hunk":                 "diff.previous_hunk",
	"diff_next_file":                     "diff.next_file",
	"diff_previous_file":                 "diff.previous_file",
	"diff_toggle_file_tree":              "diff.toggle_file_tree",
	"diff_single_patch":                  "diff.single_patch",
	"diff_switch_source":                 "diff.switch_source",
	"diff_toggle_view":                   "diff.toggle_view",
	"diff_help":                          "diff.help",
	"editor_open":                        "prompt.editor",
	"theme_list":                         "theme.switch",
	"theme_switch_mode":                  "theme.switch_mode",
	"theme_mode_lock":                    "theme.mode.lock",
	"sidebar_toggle":                     "session.sidebar.toggle",
	"scrollbar_toggle":                   "session.toggle.scrollbar",
	"status_view":                        "opencode.status",
	"debug_view":                         "opencode.debug",
	"session_export":                     "session.export",
	"session_copy":                       "session.copy",
	"session_move":                       "session.move",
	"session_new":                        "session.new",
	"session_list":                       "session.list",
	"session_timeline":                   "session.timeline",
	"session_fork":                       "session.fork",
	"session_rename":                     "session.rename",
	"session_delete":                     "session.delete",
	"session_share":                      "session.share",
	"session_unshare":                    "session.unshare",
	"session_interrupt":                  "session.interrupt",
	"session_background":                 "session.background",
	"session_compact":                    "session.compact",
	"session_toggle_timestamps":          "session.toggle.timestamps",
	"session_toggle_generic_tool_output": "session.toggle.generic_tool_output",
	"session_queued_prompts":             "session.queued_prompts",
	"session_child_first":                "session.child.first",
	"session_child_cycle":                "session.child.next",
	"session_child_cycle_reverse":        "session.child.previous",
	"session_parent":                     "session.parent",
	"session_pin_toggle":                 "session.pin.toggle",
	"session_quick_switch_1":             "session.quick_switch.1",
	"session_quick_switch_2":             "session.quick_switch.2",
	"session_quick_switch_3":             "session.quick_switch.3",
	"session_quick_switch_4":             "session.quick_switch.4",
	"session_quick_switch_5":             "session.quick_switch.5",
	"session_quick_switch_6":             "session.quick_switch.6",
	"session_quick_switch_7":             "session.quick_switch.7",
	"session_quick_switch_8":             "session.quick_switch.8",
	"session_quick_switch_9":             "session.quick_switch.9",
	"stash_delete":                       "stash.delete",
	"model_provider_list":                "model.dialog.provider",
	"model_favorite_toggle":              "model.dialog.favorite",
	"model_list":                         "model.list",
	"model_cycle_recent":                 "model.cycle_recent",
	"model_cycle_recent_reverse":         "model.cycle_recent_reverse",
	"model_cycle_favorite":               "model.cycle_favorite",
	"model_cycle_favorite_reverse":       "model.cycle_favorite_reverse",
	"mcp_list":                           "mcp.list",
	"provider_connect":                   "provider.connect",
	"console_org_switch":                 "console.org.switch",
	"agent_list":                         "agent.list",
	"agent_cycle":                        "agent.cycle",
	"agent_cycle_reverse":                "agent.cycle.reverse",
	"variant_cycle":                      "variant.cycle",
	"variant_list":                       "variant.list",
	"messages_page_up":                   "session.page.up",
	"messages_page_down":                 "session.page.down",
	"messages_line_up":                   "session.line.up",
	"messages_line_down":                 "session.line.down",
	"messages_half_page_up":              "session.half.page.up",
	"messages_half_page_down":            "session.half.page.down",
	"messages_first":                     "session.first",
	"messages_last":                      "session.last",
	"messages_next":                      "session.message.next",
	"messages_previous":                  "session.message.previous",
	"messages_last_user":                 "session.messages_last_user",
	"messages_copy":                      "messages.copy",
	"messages_undo":                      "session.undo",
	"messages_redo":                      "session.redo",
	"messages_toggle_conceal":            "session.toggle.conceal",
	"tool_details":                       "session.toggle.actions",
	"display_thinking":                   "session.toggle.thinking",
	"prompt_submit":                      "prompt.submit",
	"prompt_editor_context_clear":        "prompt.editor_context.clear",
	"prompt_skills":                      "prompt.skills",
	"prompt_stash":                       "prompt.stash",
	"prompt_stash_pop":                   "prompt.stash.pop",
	"prompt_stash_list":                  "prompt.stash.list",
	"workspace_set":                      "workspace.set",
	"input_clear":                        "prompt.clear",
	"input_paste":                        "prompt.paste",
	"input_submit":                       "input.submit",
	"input_newline":                      "input.newline",
	"input_move_left":                    "input.move.left",
	"input_move_right":                   "input.move.right",
	"input_move_up":                      "input.move.up",
	"input_move_down":                    "input.move.down",
	"input_select_left":                  "input.select.left",
	"input_select_right":                 "input.select.right",
	"input_select_up":                    "input.select.up",
	"input_select_down":                  "input.select.down",
	"input_line_home":                    "input.line.home",
	"input_line_end":                     "input.line.end",
	"input_select_line_home":             "input.select.line.home",
	"input_select_line_end":              "input.select.line.end",
	"input_visual_line_home":             "input.visual.line.home",
	"input_visual_line_end":              "input.visual.line.end",
	"input_select_visual_line_home":      "input.select.visual.line.home",
	"input_select_visual_line_end":       "input.select.visual.line.end",
	"input_buffer_home":                  "input.buffer.home",
	"input_buffer_end":                   "input.buffer.end",
	"input_select_buffer_home":           "input.select.buffer.home",
	"input_select_buffer_end":            "input.select.buffer.end",
	"input_delete_line":                  "input.delete.line",
	"input_delete_to_line_end":           "input.delete.to.line.end",
	"input_delete_to_line_start":         "input.delete.to.line.start",
	"input_backspace":                    "input.backspace",
	"input_delete":                       "input.delete",
	"input_undo":                         "input.undo",
	"input_redo":                         "input.redo",
	"input_word_forward":                 "input.word.forward",
	"input_word_backward":                "input.word.backward",
	"input_select_word_forward":          "input.select.word.forward",
	"input_select_word_backward":         "input.select.word.backward",
	"input_delete_word_forward":          "input.delete.word.forward",
	"input_delete_word_backward":         "input.delete.word.backward",
	"input_select_all":                   "input.select.all",
	"history_previous":                   "prompt.history.previous",
	"history_next":                       "prompt.history.next",
	"terminal_suspend":                   "terminal.suspend",
	"terminal_title_toggle":              "terminal.title.toggle",
	"tips_toggle":                        "tips.toggle",
	"plugin_manager":                     "plugins.list",
	"plugin_install":                     "plugins.install",
	"which_key_toggle":                   "which-key.toggle",
	"which_key_layout_toggle":            "which-key.layout.toggle",
	"which_key_pending_toggle":           "which-key.pending.toggle",
	"which_key_group_previous":           "which-key.group.previous",
	"which_key_group_next":               "which-key.group.next",
	"which_key_scroll_up":                "which-key.scroll.up",
	"which_key_scroll_down":              "which-key.scroll.down",
	"which_key_page_up":                  "which-key.page.up",
	"which_key_page_down":                "which-key.page.down",
	"which_key_home":                     "which-key.home",
	"which_key_end":                      "which-key.end",
}

// bindingValue is the resolved form of one binding: the matchable sequences
// (empty = disabled).
type bindingValue struct {
	enabled bool
	seqs    []string
}

// resolveValue normalizes a BindingValue into matchable sequences
// (false/"none" → disabled; a string → one seq; a list → each item; a map
// with a "key" field → a binding object; a map with a "name" field → a
// keystroke object). The object flags (event/preventDefault/fallthrough)
// have no yolo referent (deviation 209) — the matcher is press-only.
func resolveValue(v BindingValue) (bindingValue, error) {
	switch t := v.(type) {
	case nil:
		return bindingValue{}, nil
	case bool:
		if !t {
			return bindingValue{}, nil
		}
		return bindingValue{}, fmt.Errorf("invalid keybind value: %v", v)
	case string:
		if t == "" || t == "none" {
			return bindingValue{}, nil
		}
		// A string may carry multiple comma-separated sequences (the upstream
		// default format, e.g. "ctrl+c,ctrl+d,<leader>q", "escape,q");
		// split them into distinct matchable seqs ("+" is the modifier join —
		// never a sequence separator).
		var seqs []string
		for _, part := range strings.Split(t, ",") {
			s := strings.TrimSpace(part)
			if s == "" || s == "none" {
				continue
			}
			seqs = append(seqs, s)
		}
		if len(seqs) == 0 {
			return bindingValue{}, nil
		}
		return bindingValue{enabled: true, seqs: seqs}, nil
	case []any:
		out := bindingValue{enabled: true}
		for _, item := range t {
			sub, err := resolveValue(item)
			if err != nil {
				return bindingValue{}, err
			}
			out.seqs = append(out.seqs, sub.seqs...)
		}
		return out, nil
	case map[string]any:
		if key, ok := t["key"]; ok {
			switch k := key.(type) {
			case string:
				if k == "" || k == "none" {
					return bindingValue{}, nil
				}
				return bindingValue{enabled: true, seqs: []string{k}}, nil
			case map[string]any:
				ks, err := stringifyKeyStroke(k)
				if err != nil {
					return bindingValue{}, err
				}
				return bindingValue{enabled: true, seqs: []string{ks}}, nil
			default:
				return bindingValue{}, fmt.Errorf("invalid keybind object key: %T", key)
			}
		}
		if _, ok := t["name"]; ok {
			ks, err := stringifyKeyStroke(t)
			if err != nil {
				return bindingValue{}, err
			}
			return bindingValue{enabled: true, seqs: []string{ks}}, nil
		}
		return bindingValue{}, fmt.Errorf("invalid keybind object: missing key")
	default:
		return bindingValue{}, fmt.Errorf("invalid keybind value: %T", v)
	}
}

// stringifyKeyStroke is the port of the upstream stringifyKeyStroke: the
// name + the modifier flags (the KeyStroke schema field order,
// keybind.ts:8-15) joined with "+".
func stringifyKeyStroke(m map[string]any) (string, error) {
	name, _ := m["name"].(string)
	if name == "" {
		return "", fmt.Errorf("invalid keystroke: missing name")
	}
	var mods []string
	for _, field := range []string{"ctrl", "shift", "meta", "super", "hyper"} {
		if b, ok := m[field].(bool); ok && b {
			mods = append(mods, field)
		}
	}
	if len(mods) == 0 {
		return name, nil
	}
	return strings.Join(append(mods, name), "+"), nil
}

// keyNameAliases is the upstream KEY_ALIASES (keymap.tsx:112-117) — the
// matching-side normalization (both sides go through it).
var keyNameAliases = map[string]string{
	"enter":  "return",
	"esc":    "escape",
	"pgdown": "pagedown",
	"pgup":   "pageup",
}

// keyAliasesDisplay is the yolo display alias set (deviation 214): the
// upstream keyNameAliases display {pageup→pgup, pagedown→pgdn, delete→del}
// + escape→esc (the yolo surface convention — the select hint "esc close").
var keyAliasesDisplay = map[string]string{
	"pageup":   "pgup",
	"pagedown": "pgdn",
	"delete":   "del",
	"escape":   "esc",
}

// modifierAliasDisplay is the upstream modifierAliases (keymap.tsx:200-202).
var modifierAliasDisplay = map[string]string{"meta": "alt"}

func normalizeKeyName(name string) string {
	name = strings.ToLower(name)
	if a, ok := keyNameAliases[name]; ok {
		return a
	}
	return name
}

// parseSeq splits a sequence into the modifier set + the alias-normalized
// base key. The base is a single token; extra tokens after the base →
// invalid (false).
func parseSeq(seq string) (mods map[string]bool, base string, ok bool) {
	parts := strings.Split(seq, "+")
	mods = map[string]bool{}
	for i, p := range parts {
		switch strings.ToLower(p) {
		case "ctrl", "alt", "meta", "shift", "super", "hyper":
			mods[strings.ToLower(p)] = true
			continue
		default:
			if i+1 != len(parts) {
				return nil, "", false
			}
			return mods, normalizeKeyName(p), true
		}
	}
	return nil, "", false
}

// pressedBase returns the pressed key's modifier set + alias-normalized base
// name from the Keystroke() string (the fixed mod order ctrl, alt, shift,
// meta, hyper, super — the base is the last token).
func pressedBase(k tea.KeyPressMsg) (mods map[string]bool, base string) {
	mods = map[string]bool{}
	parts := strings.Split(k.Keystroke(), "+")
	for i, p := range parts {
		if i == len(parts)-1 {
			base = normalizeKeyName(p)
		} else {
			mods[strings.ToLower(p)] = true
		}
	}
	return mods, base
}

// keyMatchesSeq reports whether k matches one binding sequence (the
// <leader> token never matches raw — the pending mechanism owns it; the
// caller passes non-leader sequences).
func keyMatchesSeq(k tea.KeyPressMsg, seq string) bool {
	if strings.Contains(seq, "<"+LeaderToken+">") {
		return false
	}
	sm, sb, ok := parseSeq(seq)
	if !ok {
		return false
	}
	pm, pb := pressedBase(k)
	if sb != pb {
		return false
	}
	if len(sm) != len(pm) {
		return false
	}
	for m := range sm {
		if !pm[m] {
			return false
		}
	}
	return true
}

// leaderSplit separates a stored <leader> token: (has, rest) — rest is the
// sequence remainder after the token (the second key of the pending
// sequence).
func leaderSplit(seq string) (has bool, rest string) {
	const tok = "<" + LeaderToken + ">"
	i := strings.Index(seq, tok)
	if i < 0 {
		return false, seq
	}
	return true, strings.TrimPrefix(seq[i+len(tok):], "+")
}

// formatKeySequence is the port of the upstream formatKeySequence
// (keymap.tsx:206-208) with the yolo display aliases (deviation 214): the
// <leader> token → the resolved leader key (the remainder space-joined),
// the display aliases, the modifier alias meta→alt.
func formatKeySequence(seq, leader string) string {
	const tok = "<" + LeaderToken + ">"
	if i := strings.Index(seq, tok); i >= 0 {
		rest := strings.TrimPrefix(seq[i+len(tok):], "+")
		if rest == "" {
			return leader
		}
		return leader + " " + rest
	}
	parts := strings.Split(seq, "+")
	out := make([]string, len(parts))
	for i, p := range parts {
		low := strings.ToLower(p)
		if i == len(parts)-1 {
			if d, ok := keyAliasesDisplay[low]; ok {
				p = d
			}
		} else if d, ok := modifierAliasDisplay[low]; ok {
			p = d
		}
		out[i] = p
	}
	return strings.Join(out, "+")
}

// formatJoin is the multi-sequence display join (deviation 214: the upstream
// keymap-library join is not visible from the repo).
const formatJoin = " / "

// formatSequences renders the resolved sequences for display ("" =
// disabled → the caller renders "none").
func formatSequences(seqs []string, leader string) string {
	formatted := make([]string, 0, len(seqs))
	for _, seq := range seqs {
		formatted = append(formatted, formatKeySequence(seq, leader))
	}
	return strings.Join(formatted, formatJoin)
}
```

**Step 4 — gate.** `go test ./internal/tui/ -run 'TestKeymap' -count=1` → PASS, then FULL gate: `go vet ./... && go test ./...` + `gofmt -l .` empty.

**Step 5 — commit + close the bead.**
`git add internal/tui/keymap.go internal/tui/keymap_test.go && git commit -m "feat: keymap registry - upstream default bindings"`
`bd close <S4.1 bead> --reason "registry green: 185 verbatim entries + CommandMap, value-shape decoder, alias matcher, display formatter" --json`

---

### Task S4.2: Keymap registry: per-context groups + runtime remap (bead `yolo-oae.5.2`, expected id `yolo-oae.5.3`)

**Files:** extend `internal/tui/keymap.go` (the `sort` import; `modeEntry` + `Keymap` + `NewKeymap`/`Set`/`Seqs`/`Match`/`MatchPending`/`Format`/`leaderDisplay`/`Current`/`Push`; `contextGroups`); extend `internal/tui/keymap_test.go` (`pressLeader` + `TestKeymapNew`/`TestKeymapSet`/`TestKeymapMatchPending`/`TestKeymapModes`/`TestKeymapFormat`/`TestKeymapDispatch`); `internal/tui/app.go` (`App.keymap`/`App.pendingLeader`; NewApp builds the default keymap; `leaderTimeoutMsg` + its `updateMsg` case); `internal/tui/keys.go` (the `handleKey` rewire + `handleAppKeys`/`leaderTick`/`matchLeaderContinuation`/`matchBase`/`dispatchCommand`); `internal/tui/session.go` (`handleSessionKey` registry-backed page up/down/rename/interrupt); `internal/tui/dialog.go` (drop the `dlgModelKey`/`dlgAgentsKey` var lines); re-baseline the ctrl+p/ctrl+a tests (`model_test.go`, `agent_test.go`, `tui_suite_test.go`).

**Interfaces:** produces: `modeEntry{id int, mode string}`; `Keymap{bindings map[string]bindingValue, modes []modeEntry, nextID int}`; `NewKeymap(overrides map[string]any) (*Keymap, error)` (the ported `parse`: unknown keys → `unrecognized keybind(s): <sorted names>`, the absent name → its default); `(*Keymap).Set(name string, v BindingValue) error` (the runtime remap — immediately effective, every keypress re-reads the table); `(*Keymap).Seqs(name string) []string`; `(*Keymap).Match(name string, k tea.KeyPressMsg) bool`; `(*Keymap).MatchPending(name string, k tea.KeyPressMsg) bool`; `(*Keymap).Format(name string) string` ("none" when disabled, else the formatted seqs joined by " / "); `(*Keymap).leaderDisplay() string`; `(*Keymap).Current() string`; `(*Keymap).Push(mode string) func()`; `contextGroups map[string][]string`; `(*App).handleAppKeys(k) ([]tea.Cmd, bool)`; `leaderTick() tea.Cmd`; `(*App).matchLeaderContinuation(k) (string, bool)`; `(*App).matchBase(k) (string, bool)`; `(*App).dispatchCommand(name string) []tea.Cmd`; `leaderTimeoutMsg`.

**Upstream parity notes:** the mode stack is the port of `createOpencodeModeStack` (keymap.tsx:53-100) — `Push` splices out THAT frame by identity (not mode name); `Current` = the top mode or `base`. `NewKeymap` is the port of `parse` (keybind.ts:449-458) — the unknown-key error lowercases to the Go convention (`unrecognized keybind(s): ...`). The leader is the port of `registerTimedLeader` + `registerEscapeClearsPendingSequence` (keymap.tsx:220-228): the leader seqs arm `tea.Tick(LeaderTimeout=2000ms)`; a second key matching a `<leader>` continuation dispatches; a non-matching second key clears AND re-dispatches (the key is not lost — the upstream keymap-library replay is not verifiable from the repo; deviation 211); a leader keypress while pending re-arms. The dispatch scoping is deviation 211 (behavior/medium): the yolo-specific surface keys (home up/down/enter/n, the session alt+e/alt+t, the prompt soft-enter, the dialog payload keys) stay the current `key.Binding` surfaces; the registry is the single source for the ported upstream bindings. The **ctrl+p remap**: ctrl+p now matches `command_list` (consumed-but-inert at S4.2; the palette lands in S4.4) — the model dialog's opener frees to `<leader>m` + `/model`. The **ctrl+a remap**: ctrl+a is no longer an opener (it frees to the prompt input — the upstream `input_line_home` default "ctrl+a"); the agent dialog's opener frees to `<leader>a` + `/agents`. The V1 pins (pgup/pgdn, `\`+enter) are untouched.

**Step 1 — write the failing tests.** Extend `internal/tui/keymap_test.go`:

```go
// pressLeader is the default leader keypress (ctrl+x, LeaderDefault).
func pressLeader() tea.KeyPressMsg { return tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl} }

func TestKeymapNew(t *testing.T) {
	// The unknown-key error (the ported parse).
	if _, err := NewKeymap(map[string]any{"nope": "ctrl+z"}); err == nil ||
		!strings.Contains(err.Error(), "unrecognized keybind(s): nope") {
		t.Fatalf("unknown key err = %v, want the unrecognized message", err)
	}
	// The present name is overridden; the absent name keeps its default.
	km, err := NewKeymap(map[string]any{"command_list": "ctrl+k"})
	if err != nil {
		t.Fatal(err)
	}
	if !km.Match("command_list", tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl}) {
		t.Fatal("the override command_list=ctrl+k must match ctrl+k")
	}
	if km.Match("command_list", tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl}) {
		t.Fatal("the override must REPLACE the default (ctrl+p no longer matches)")
	}
	if !km.Match("leader", pressLeader()) {
		t.Fatal("the leader default must survive an unrelated override")
	}
}

func TestKeymapSet(t *testing.T) {
	km, _ := NewKeymap(nil)
	if err := km.Set("command_list", "ctrl+j"); err != nil {
		t.Fatal(err)
	}
	if !km.Match("command_list", tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl}) {
		t.Fatal("Set must take effect immediately")
	}
	if km.Match("command_list", tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl}) {
		t.Fatal("the old binding must no longer match after Set")
	}
	if err := km.Set("nope", "ctrl+z"); err == nil {
		t.Fatal("Set on an unknown name must error")
	}
	if err := km.Set("command_list", "none"); err != nil {
		t.Fatal(err)
	}
	if km.Match("command_list", tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl}) {
		t.Fatal("a none Set must disable the binding")
	}
}

func TestKeymapMatchPending(t *testing.T) {
	km, _ := NewKeymap(nil)
	if !km.MatchPending("model_list", press('m')) {
		t.Fatal("model_list <leader>m must match the continuation 'm'")
	}
	if km.MatchPending("model_list", press('a')) {
		t.Fatal("model_list must not match the continuation 'a'")
	}
	if km.MatchPending("command_list", press('p')) {
		t.Fatal("command_list (ctrl+p, no <leader>) must have no continuation")
	}
}

func TestKeymapModes(t *testing.T) {
	km, _ := NewKeymap(nil)
	if got := km.Current(); got != BaseMode {
		t.Fatalf("Current() = %q, want base (the empty stack)", got)
	}
	release := km.Push("session")
	if got := km.Current(); got != "session" {
		t.Fatalf("Current() after push = %q, want session", got)
	}
	release()
	if got := km.Current(); got != BaseMode {
		t.Fatalf("Current() after release = %q, want base", got)
	}
	// The identity splice: two pushes of the SAME mode, releasing the first
	// leaves the second (identity, not mode-name, matching).
	r1 := km.Push("session")
	r2 := km.Push("session")
	r1()
	if got := km.Current(); got != "session" {
		t.Fatalf("Current() after releasing the first of two = %q, want session (identity splice)", got)
	}
	r2()
}

func TestKeymapFormat(t *testing.T) {
	km, _ := NewKeymap(nil)
	if got := km.Format("app_exit"); got != "ctrl+c / ctrl+d / ctrl+x q" {
		t.Fatalf("Format(app_exit) = %q, want the comma-list display", got)
	}
	if got := km.Format("help_show"); got != "none" {
		t.Fatalf("Format(help_show) = %q, want none", got)
	}
	if got := km.Format("model_list"); got != "ctrl+x m" {
		t.Fatalf("Format(model_list) = %q, want ctrl+x m", got)
	}
}

func TestKeymapDispatch(t *testing.T) {
	t.Run("ctrl+c opens the quit dialog (app_exit)", func(t *testing.T) {
		a := testApp()
		a.handleKey(ctrlCKey)
		d, ok := a.dlg.top()
		if !ok || d.kind != dlgQuit {
			t.Fatalf("after ctrl+c: top=%+v (ok=%v), want the quit dialog", d, ok)
		}
	})

	t.Run("ctrl+p is consumed but inert at S4.2 (the palette remap)", func(t *testing.T) {
		a := testApp()
		a.handleKey(pressCtrlP())
		if d, ok := a.dlg.top(); ok || a.pendingLeader {
			t.Fatalf("ctrl+p must open no dialog at S4.2: top=%+v pending=%v", d, a.pendingLeader)
		}
	})

	t.Run("leader+m opens the model dialog", func(t *testing.T) {
		a := modelFixture()
		a.handleKey(pressLeader())
		a.Cmds = nil
		a.handleKey(press('m'))
		d, ok := a.dlg.top()
		if !ok || d.kind != dlgModel || d.model == nil {
			t.Fatalf("after leader+m: top=%+v, want the model dialog", d)
		}
	})

	t.Run("leader+a opens the agent dialog", func(t *testing.T) {
		a := agentApp()
		a.handleKey(pressLeader())
		a.Cmds = nil
		a.handleKey(press('a'))
		d, ok := a.dlg.top()
		if !ok || d.kind != dlgAgents || d.agent == nil {
			t.Fatalf("after leader+a: top=%+v, want the agent dialog", d)
		}
	})

	t.Run("a non-matching second key clears the leader and is not lost", func(t *testing.T) {
		a := testApp()
		a.handleKey(pressLeader())
		a.Cmds = nil
		a.handleKey(press('z'))
		if a.pendingLeader {
			t.Fatal("the leader must clear on a non-matching second key")
		}
		if a.prompt.input.Value() != "z" {
			t.Fatalf("prompt = %q, want z (the key was not lost)", a.prompt.input.Value())
		}
	})

	t.Run("leader is ignored while a dialog is on top", func(t *testing.T) {
		a := modelFixture()
		a.dlg.push(dialog{kind: dlgQuit})
		a.handleKey(pressLeader())
		if a.pendingLeader {
			t.Fatal("the leader must not arm while a dialog is open")
		}
	})
}
```

**Step 2 — confirm FAIL.** `go test ./internal/tui/ -run 'TestKeymap' -count=1` → FAIL (build fails: undefined `NewKeymap`, `Keymap`, `pressLeader`, `a.keymap`, `a.pendingLeader` — the expected red).

**Step 3 — minimal implementation.**

(a) Extend `internal/tui/keymap.go` (add `"sort"` to the imports, after `"fmt"`):

```go
// modeEntry is one mode-stack frame (the ported createOpencodeModeStack
// entry: the identity id + the mode name).
type modeEntry struct {
	id   int
	mode string
}

// Keymap is the runtime keymap (S4.2): the resolved bindings (the single
// source for every ported upstream binding), the mode stack, and the display
// helpers. Every keypress re-reads the table, so a Set is immediately
// effective (the ported runtime-remap semantics).
type Keymap struct {
	bindings map[string]bindingValue
	modes    []modeEntry
	nextID   int
}

// NewKeymap is the port of the upstream parse (keybind.ts:449-458): the
// unknown keys error (sorted, the Go lowercase convention), then every
// default with its override. A nil/empty overrides map = the defaults.
func NewKeymap(overrides map[string]any) (*Keymap, error) {
	if overrides != nil {
		var unknown []string
		for name := range overrides {
			if _, ok := Definitions[name]; !ok {
				unknown = append(unknown, name)
			}
		}
		if len(unknown) > 0 {
			sort.Strings(unknown)
			return nil, fmt.Errorf("unrecognized keybind(s): %s", strings.Join(unknown, ", "))
		}
	}
	km := &Keymap{bindings: make(map[string]bindingValue, len(Definitions))}
	for name, def := range Definitions {
		v := def.Default
		if overrides != nil {
			if ov, ok := overrides[name]; ok {
				v = ov
			}
		}
		bv, err := resolveValue(v)
		if err != nil {
			return nil, fmt.Errorf("keybind %q: %w", name, err)
		}
		km.bindings[name] = bv
	}
	return km, nil
}

// Set is the runtime remap: it re-resolves the named binding to v (immediately
// effective — every keypress re-reads the table). An unknown name errors (the
// same unrecognized message as NewKeymap).
func (km *Keymap) Set(name string, v BindingValue) error {
	if _, ok := Definitions[name]; !ok {
		return fmt.Errorf("unrecognized keybind: %s", name)
	}
	bv, err := resolveValue(v)
	if err != nil {
		return fmt.Errorf("keybind %q: %w", name, err)
	}
	km.bindings[name] = bv
	return nil
}

// Seqs returns the named binding's matchable seqs (the <leader> seqs included;
// the caller filters — Match/MatchPending do).
func (km *Keymap) Seqs(name string) []string { return km.bindings[name].seqs }

// Match reports whether k matches any seq of the named binding (keyMatchesSeq
// already rejects the <leader> seqs — the pending mechanism owns them).
func (km *Keymap) Match(name string, k tea.KeyPressMsg) bool {
	for _, seq := range km.bindings[name].seqs {
		if keyMatchesSeq(k, seq) {
			return true
		}
	}
	return false
}

// MatchPending reports whether k matches the named binding's <leader>
// continuation (the remainder after the token — the second key of the pending
// sequence).
func (km *Keymap) MatchPending(name string, k tea.KeyPressMsg) bool {
	for _, seq := range km.bindings[name].seqs {
		has, rest := leaderSplit(seq)
		if !has || rest == "" {
			continue
		}
		if keyMatchesSeq(k, rest) {
			return true
		}
	}
	return false
}

// Format is the display form of the named binding: "none" when disabled, else
// the formatted seqs joined by the formatJoin (" / ").
func (km *Keymap) Format(name string) string {
	bv := km.bindings[name]
	if !bv.enabled || len(bv.seqs) == 0 {
		return "none"
	}
	return formatSequences(bv.seqs, km.leaderDisplay())
}

// leaderDisplay is the resolved leader key display (the "leader" binding's
// first seq; the default "ctrl+x"). It does NOT recurse through Format (which
// would be circular for the leader itself).
func (km *Keymap) leaderDisplay() string {
	bv := km.bindings["leader"]
	if !bv.enabled || len(bv.seqs) == 0 {
		return LeaderDefault
	}
	return formatKeySequence(bv.seqs[0], LeaderDefault)
}

// Current is the top mode or base (the ported createOpencodeModeStack
// current).
func (km *Keymap) Current() string {
	if n := len(km.modes); n > 0 {
		return km.modes[n-1].mode
	}
	return BaseMode
}

// Push registers a mode and returns its release func (the ported identity
// splice — it removes THAT frame by id, not by mode name).
func (km *Keymap) Push(mode string) func() {
	id := km.nextID
	km.nextID++
	km.modes = append(km.modes, modeEntry{id: id, mode: mode})
	return func() {
		for i := range km.modes {
			if km.modes[i].id == id {
				km.modes = append(km.modes[:i], km.modes[i+1:]...)
				return
			}
		}
	}
}

// contextGroups is the yolo context→binding-name groups (the upstream
// context/mode-scoped bindings have no single referent file — the groups are
// the yolo port; deviation 211). The base group is the app-level openers (any
// route, no dialog, no pending permission) in match order; the session group
// is the session-route registry keys.
var contextGroups = map[string][]string{
	BaseMode: {
		"which_key_toggle", "which_key_layout_toggle", "which_key_pending_toggle",
		"command_list", "app_exit", "model_list", "agent_list", "status_view",
		"theme_list", "session_new", "session_list",
	},
	"session": {
		"messages_page_up", "messages_page_down", "session_interrupt", "session_rename",
	},
}
```

(b) `internal/tui/app.go` — add the two `App` fields (after `retrySuppressed`), build the default keymap in `NewApp`, and add the `leaderTimeoutMsg` type + its `updateMsg` case:

```go
// (App struct, after retrySuppressed)
	keymap        *Keymap // the keymap registry (S4.2)
	pendingLeader bool    // the leader pending state is armed

// (a new msg type, near the other msgs)
// leaderTimeoutMsg clears the pending leader (the ported registerTimedLeader
// timeout — the pending sequence expires after LeaderTimeout).
type leaderTimeoutMsg struct{}
```

In `NewApp`, before the `a := &App{...}` literal, build the default keymap and add the fields:
```go
	// the keymap registry (S4.2): the defaults (the config overrides are
	// applied by SetKeybinds, S4.3). NewKeymap(nil) never errors (no unknown
	// keys; the defaults are valid).
	km, _ := NewKeymap(nil)
	a := &App{
		...
		keymap:          km,
		pendingLeader:   false,
		retrySuppressed: map[string]bool{},
	}
```

In `updateMsg`, add the case (alongside the other `tea.Msg` cases):
```go
	case leaderTimeoutMsg:
		a.pendingLeader = false
		return nil
```

(c) `internal/tui/keys.go` — rewire `handleKey` (replace the opener-ladder `switch` with the registry dispatch) and add the helpers:

```go
// handleKey is the app key dispatcher: permission > dialog > the keymap
// registry (app-level bindings, S4.2) > slash menu > route > prompt. A
// pending permission ask owns every key; while a dialog is open it owns the
// keys; otherwise the keymap registry owns the app-level bindings (the leader
// + the base context group); while the slash menu is open it owns the keys;
// routes handle their navigation keys; everything else falls through to the
// always-focused prompt input.
func (a *App) handleKey(k tea.KeyPressMsg) []tea.Cmd {
	if len(a.store.Pending) > 0 {
		if d, ok := a.dlg.top(); ok && d.kind == dlgPerm && d.perm != nil {
			return d.perm.handleKey(a, k)
		}
		return (&permDlg{}).handleKey(a, k)
	}
	if d, ok := a.dlg.top(); ok {
		return a.handleDialogKey(d, k)
	}
	// S4.2: the keymap registry owns the app-level bindings (any route, no
	// dialog). The leader mechanism first, then the base context group.
	if cmds, done := a.handleAppKeys(k); done {
		return cmds
	}
	if a.prompt.slashActive() {
		return a.handleMenuKey(k)
	}
	switch a.route {
	case routeSession:
		if cmds, done := a.handleSessionKey(k); done {
			return cmds
		}
	default:
		if cmds, done := a.handleHomeKey(k); done {
			return cmds
		}
	}
	return a.handlePromptKey(k)
}

// handleAppKeys dispatches the keymap registry's app-level bindings (any
// route, no dialog, no pending permission): the leader mechanism first, then
// the base context group in order. It reports whether the key was consumed;
// unhandled keys fall through to the slash menu / route / prompt.
func (a *App) handleAppKeys(k tea.KeyPressMsg) ([]tea.Cmd, bool) {
	km := a.keymap
	// The leader binding arms (or re-arms, while pending) the pending state
	// and consumes the key (a leader keypress while pending re-arms).
	if km.Match("leader", k) {
		a.pendingLeader = true
		return []tea.Cmd{leaderTick()}, true
	}
	if a.pendingLeader {
		// A second key: match a <leader> continuation (base group order); a
		// match dispatches; a miss clears and re-dispatches (the key is not
		// lost — deviation 211).
		a.pendingLeader = false
		if name, ok := a.matchLeaderContinuation(k); ok {
			return a.dispatchCommand(name), true
		}
		if cmds, done := a.matchBase(k); done {
			return cmds, true
		}
		return nil, false // fall through to the slash menu / route / prompt
	}
	if cmds, done := a.matchBase(k); done {
		return cmds, true
	}
	return nil, false
}

// leaderTick arms the leader timeout (the ported registerTimedLeader tick).
func leaderTick() tea.Cmd {
	return tea.Tick(LeaderTimeout, func(time.Time) tea.Msg { return leaderTimeoutMsg{} })
}

// matchLeaderContinuation matches a second key against the base group's
// <leader> continuations (base group order); it returns the matched binding
// name.
func (a *App) matchLeaderContinuation(k tea.KeyPressMsg) (string, bool) {
	for _, name := range contextGroups[BaseMode] {
		if a.keymap.MatchPending(name, k) {
			return name, true
		}
	}
	return "", false
}

// matchBase matches the base context group in order (app_exit's ctrl+d seq is
// prompt-owned — skipped, the upstream input-layer-wins semantics; the prompt
// is always focused in yolo). It returns the matched binding name.
func (a *App) matchBase(k tea.KeyPressMsg) (string, bool) {
	for _, name := range contextGroups[BaseMode] {
		skipCtrlD := name == "app_exit"
		for _, seq := range a.keymap.Seqs(name) {
			if skipCtrlD && seq == "ctrl+d" {
				continue
			}
			if keyMatchesSeq(k, seq) {
				return name, true
			}
		}
	}
	return "", false
}

// dispatchCommand runs a referent-bearing registry command. The command_list
// (S4.4) and which_key_* (S4.6) cases are consumed but inert at S4.2 time
// (the cases land in those tasks).
func (a *App) dispatchCommand(name string) []tea.Cmd {
	switch name {
	case "app_exit":
		a.dlg.push(dialog{kind: dlgQuit})
	case "model_list":
		return a.openModelDialog()
	case "agent_list":
		return a.openAgentDialog()
	case "status_view":
		return a.openStatusDialog()
	case "theme_list":
		return a.openThemeListDialog()
	case "session_new":
		return a.emit(a.createSessionCmd())
	case "session_list":
		return a.openSessionListDialog()
	case "provider_connect":
		return a.openProviderDialog()
	case "help_show":
		a.pushModal(dialog{kind: dlgHelp}, dlgMedium, nil)
	// command_list (S4.4) and which_key_* (S4.6) are consumed but inert here.
	}
	return nil
}
```

(d) `internal/tui/session.go` — rewire `handleSessionKey` (registry-backed page up/down/rename/interrupt; the alt+e/alt+t yolo surface toggles and the esc-when-idle return-home stay the surface behavior — deviation 211's scope). The `sessKeyMap.PageUp`/`PageDown`/`Rename` fields are superseded by the registry (kept as the V1-pin documentation); `Expand`/`Think` remain the surface. Replace the four `key.Matches(k, sessKeyMap.PageUp/PageDown/Rename)` + `key.Matches(k, escBinding)` arms with:

```go
	// S4.2: the registry-backed session keys (the messages_page_up/down
	// defaults add ctrl+alt+b/f; the V1 pgup/pgdn pins are the first seqs).
	case a.keymap.Match("messages_page_up", k):
		a.sess.vm.PageUp()
		a.sess.following = false
		return nil, true
	case a.keymap.Match("messages_page_down", k):
		a.sess.vm.PageDown()
		a.sess.following = a.sess.vm.AtBottom()
		return nil, true
	// (the alt+e Expand + alt+t Think arms are UNCHANGED — the yolo surface)
	// S4.2: the registry-backed rename (the upstream session_rename default
	// ctrl+r).
	case a.keymap.Match("session_rename", k):
		if a.curSessionID == "" {
			return nil, false
		}
		a.openSessionRenameDialog(a.curSessionID)
		return nil, true
	// S4.2: the registry-backed interrupt (the upstream session_interrupt
	// default escape); the esc-when-idle return-home is the yolo surface
	// behavior (deviation 211's scope).
	case a.keymap.Match("session_interrupt", k):
		if sessionBusy(&a.store) {
			return a.emit(a.abortCmd()), true
		}
		a.route = routeHome
		a.curSessionID = ""
		return a.emit(a.hydrateCmd()), true
```

(e) `internal/tui/dialog.go` — drop the two opener var lines (the model/agent openers are registry-driven; `choiceThis`/`choiceDef` stay):
```go
var (
	choiceThis = key.NewBinding(key.WithKeys("a"))
	choiceDef  = key.NewBinding(key.WithKeys("b"))
)
```

(f) Re-baseline the ctrl+p/ctrl+a tests (the S4.2 remap). `model_test.go`: `TestModelDialogOpen` "ctrl+p opens the model dialog" → "leader+m opens the model dialog" (`a.handleKey(pressLeader())`; `a.Cmds = nil`; `a.handleKey(press('m'))`; assert the model dialog); "ctrl+p is ignored while a dialog is on top" → "leader is ignored while a dialog is on top" (push `dlgQuit`, `a.handleKey(pressLeader())`, assert no model dialog + `!a.pendingLeader`); the teatest `TestTUIModelDialog` `tm.Send(pressCtrlP())` → the `/model` slash command (`for _, r := range "/model" { tm.Send(press(r)) }` then `tm.Send(press(tea.KeyEnter))`) + update the doc comment. `agent_test.go`: the three twin re-baselines (`leader+a`; `/agents` in `TestTUIAgentDialog`). `tui_suite_test.go` `TestTUIDialogs`: the `tm.Send(pressCtrlP())` → `/model` slash + enter and the `tm.Send(pressCtrlA())` → `/agents` slash + enter (the `capture("Help", "Press ctrl+p ...")` line is unchanged — `paletteShortcut()` still renders "ctrl+p" until S4.7).

**Step 4 — gate.** `go test ./internal/tui/ -run 'TestKeymap|TestModelDialog|TestAgentDialog|TestTUI' -count=1` → PASS, then FULL gate: `go vet ./... && go test ./...` + `gofmt -l .` empty (the re-baselined ctrl+p/ctrl+a tests + the V1 pgup/pgdn pins green).

**Step 5 — commit + close the bead.**
`git add internal/tui/keymap.go internal/tui/keymap_test.go internal/tui/app.go internal/tui/keys.go internal/tui/session.go internal/tui/dialog.go internal/tui/model_test.go internal/tui/agent_test.go internal/tui/tui_suite_test.go && git commit -m "feat: keymap registry - context groups + runtime remap"`
`bd close <S4.2 bead> --reason "context groups + runtime remap green: NewKeymap/Set/Match/MatchPending/mode stack, the leader mechanism, the ctrl+p/ctrl+a remaps re-baselined, V1 pins green" --json`

---

### Task S4.3: Keybinds config schema under `yolo.jsonc` (bead `yolo-oae.5.3`, expected id `yolo-oae.5.4`)

**Files:** `internal/protocol/config.go` (the `Config.Keybinds` field); `internal/tui/app.go` (`(*App).SetKeybinds`); `cmd/yolo/main.go` (the `SetKeybinds(cfg.Keybinds)` wiring between `NewApp` and `program.Run`); `internal/config/config_test.go` (the `keybinds` field parse + deep-merge test); `internal/tui/keymap_test.go` (`TestAppSetKeybinds`).

**Interfaces:** produces: `protocol.Config.Keybinds map[string]any json:"keybinds,omitempty"` (the binding value = string | keystroke object | array | `false`/`"none"` — the raw shape `SetKeybinds` hands to `NewKeymap`); `(*App).SetKeybinds(overrides map[string]any) error` (rebuilds the keymap with the config overrides — an unknown keybind is a config error).

**Upstream parity notes:** the `keybinds` field is the port of the upstream `tui.keybinds` config surface (the per-binding overrides `parse` consumes). The value shape is the `BindingValueSchema` (keybind.ts:28-33) — `SetKeybinds` hands the raw map to `NewKeymap`, which reuses `resolveValue` (the S4.1 value-shape decoder, including the comma-split and the object flags — data-only, deviation 209). The `omitempty` keeps every `GET /config` golden byte-identical (no test config carries a `keybinds` field — grep-verified at detail time: no `keybinds` in `internal/server/testdata/`). No server change is required (the field is config-file-only; the GET /config marshal omits it when nil). The merge is the existing `Merge` deep-merge (the `keybinds` map recurses per-name — project wins per-name over global).

**Step 1 — write the failing tests.** (a) Extend `internal/config/config_test.go`:

```go
func TestKeybindsFieldParsesAndMerges(t *testing.T) {
	global := t.TempDir()
	work := t.TempDir()
	write(t, filepath.Join(global, "yolo.jsonc"), `{"keybinds":{"command_list":"ctrl+k"}}`)
	mid := filepath.Join(work, "mid")
	write(t, filepath.Join(mid, "yolo.jsonc"), `{"keybinds":{"model_list":"<leader>m"}}`)
	cfg, err := config.Loader{Env: nil}.LoadAt(global, mid)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Keybinds) != 2 {
		t.Fatalf("keybinds = %d entries, want 2 (the deep merge of the two maps)", len(cfg.Keybinds))
	}
	if got := cfg.Keybinds["command_list"]; got != "ctrl+k" {
		t.Fatalf("keybinds.command_list = %v, want ctrl+k (global kept)", got)
	}
	if got := cfg.Keybinds["model_list"]; got != "<leader>m" {
		t.Fatalf("keybinds.model_list = %v, want <leader>m (project added)", got)
	}
}
```

(b) Extend `internal/tui/keymap_test.go`:

```go
func TestAppSetKeybinds(t *testing.T) {
	a := testApp()
	if got := a.keymap.Format("command_list"); got != "ctrl+p" {
		t.Fatalf("default command_list = %q, want ctrl+p", got)
	}
	if err := a.SetKeybinds(map[string]any{"command_list": "ctrl+k"}); err != nil {
		t.Fatal(err)
	}
	if got := a.keymap.Format("command_list"); got != "ctrl+k" {
		t.Fatalf("command_list after SetKeybinds = %q, want ctrl+k", got)
	}
	if !a.keymap.Match("command_list", tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl}) {
		t.Fatal("the SetKeybinds override must match ctrl+k")
	}
	if err := a.SetKeybinds(map[string]any{"nope": "ctrl+z"}); err == nil {
		t.Fatal("SetKeybinds on an unknown key must error (a config error)")
	}
}
```

**Step 2 — confirm FAIL.** `go test ./internal/config/ ./internal/tui/ -run 'TestKeybindsFieldParsesAndMerges|TestAppSetKeybinds' -count=1` → FAIL (build fails: `cfg.Keybinds` undefined; `a.SetKeybinds` undefined — the expected red).

**Step 3 — minimal implementation.**

(a) `internal/protocol/config.go` — add the field to the `Config` struct:
```go
type Config struct {
	Profile      *Profile                  `json:"profile,omitempty"`
	Model        string                    `json:"model,omitempty"`
	Agent        string                    `json:"agent,omitempty"`
	Provider     map[string]ProviderConfig `json:"provider,omitempty"`
	Permission   map[string]any            `json:"permission,omitempty"`
	Instructions []string                  `json:"instructions,omitempty"`
	Theme        string                    `json:"theme,omitempty"`
	ToolOutput   *ToolOutput               `json:"tool_output,omitempty"`
	Agents       map[string]CustomAgent    `json:"agents,omitempty"`
	// Keybinds is the yolo.jsonc keybinds overrides (S4.3): the binding name
	// → the raw binding value (string | keystroke object | array |
	// false/"none"). omitempty keeps the GET /config goldens byte-identical.
	Keybinds map[string]any `json:"keybinds,omitempty"`
}
```

(b) `internal/tui/app.go` — add `SetKeybinds` (after `NewApp`):
```go
// SetKeybinds applies the yolo.jsonc keybinds overrides to the keymap
// registry (S4.3): it rebuilds the keymap from the defaults + the overrides.
// An unknown keybind name is a config error (returned to the caller — the
// CLI fails the start, matching the other config-load failures). A nil
// overrides map is a no-op rebuild of the defaults.
func (a *App) SetKeybinds(overrides map[string]any) error {
	km, err := NewKeymap(overrides)
	if err != nil {
		return err
	}
	a.keymap = km
	return nil
}
```

(c) `cmd/yolo/main.go` — wire the overrides between `NewApp` (line 235) and `program.Run` (line 246). Insert after `app := tui.NewApp(cl, store.State{}, sessionID, engine)`:
```go
	// the keybinds config (S4.3): apply the yolo.jsonc keybinds overrides to
	// the keymap registry (an unknown keybind is a config error — fail the
	// start, matching the other config-load failures above).
	if err := app.SetKeybinds(cfg.Keybinds); err != nil {
		fmt.Fprintf(os.Stderr, "yolo: %v\n", err)
		drain(deps, srv)
		return 1
	}
```

**Step 4 — gate.** `go test ./internal/config/ ./internal/tui/ -run 'TestKeybindsFieldParsesAndMerges|TestAppSetKeybinds' -count=1` → PASS, then FULL gate: `go vet ./... && go test ./...` + `gofmt -l .` empty (the `GET /config` goldens stay byte-identical — `omitempty`).

**Step 5 — commit + close the bead.**
`git add internal/protocol/config.go internal/tui/app.go cmd/yolo/main.go internal/config/config_test.go internal/tui/keymap_test.go && git commit -m "feat: keybinds config schema (yolo.jsonc keybinds field)"`
`bd close <S4.3 bead> --reason "keybinds schema green: Config.Keybinds field + SetKeybinds + main.go wiring; GET /config goldens byte-identical (omitempty)" --json`

---

### Task S4.4: Command palette: overlay over `GET /command` + fuzzy filter (bead `yolo-oae.5.4`, expected id `yolo-oae.5.5`)

**Files:** `internal/tui/dialog.go` (the `dlgPalette` kind + the `modalInner`/`handleDialogKey` cases); `internal/tui/commands.go` (`openPaletteDialog`/`paletteOptions`/`commandBindings`); `internal/tui/keys.go` (the `command_list` case in `dispatchCommand` — the S4.2 remap lands); new `internal/tui/palette_test.go` (`TestPaletteOptions`/`TestPaletteOpen`/`TestPaletteDispatch`).

**Interfaces:** produces: `dlgPalette` (the new `dialogKind` — the palette rides the `dialog.sel` payload, the S2.9 convention); `(*App).openPaletteDialog() []tea.Cmd` (pushes the `dlgPalette` select modal, `dlgMedium`); `paletteOptions(a *App) []selectOption` (the merged command list, the footer from the registry); `commandBindings map[string]string` (the yolo command name → the registry binding name, the referent subset). The `command_list` `dispatchCommand` case (ctrl+p → the palette) lands here.

**Upstream parity notes:** the palette is the port of `component/command-palette.tsx` (read in full at detail time): the DialogSelect "Commands" over the palette-namespace commands (yolo: the merged command list — the 4 local commands first, then the `GET /command` catalog, the slash-menu convention; the Suggested bucket is NOT ported — the yolo wire `protocol.Command` has no `suggested` flag, deviation 212). The options = `{title: name minus the leading "/", description, category, footer: formatKeyBindings, value}` (yolo: `title`/`description`/`footer`/`value`; the `category` is unused by the yolo select render). The **fuzzy filter** rides the S2.5 `selectModel.filtered()` (the title×2 + category×1 port — the S2.5 fuzzy is active, no `skipFilter`). The **run-on-enter** is wired in S4.5 (the `onSelect` is nil at S4.4 — the palette opens and filters, enter is inert until S4.5). The `dlgMedium` size is the upstream DialogSelect default (the S3 convention).

**Step 1 — write the failing tests.** New `internal/tui/palette_test.go`:

```go
package tui

import (
	"testing"

	"github.com/kido5217/yolo/internal/protocol"
)

func TestPaletteOptions(t *testing.T) {
	a := testApp()
	a.store.Commands = []protocol.Command{
		{Name: "/help", Description: "Show help"},
		{Name: "/new", Description: "New session"},
		{Name: "/model", Description: "List models"},
		{Name: "/agents", Description: "List agents"},
		{Name: "/quit", Description: "Quit"},
	}
	opts := paletteOptions(a)
	if len(opts) != 9 {
		t.Fatalf("palette = %d options, want 9 (4 local + 5 server)", len(opts))
	}
	if opts[0].title != "sessions" {
		t.Fatalf("first option = %q, want sessions (the local /sessions first)", opts[0].title)
	}
	byTitle := map[string]selectOption{}
	for _, o := range opts {
		byTitle[o.title] = o
	}
	if byTitle["model"].footer != "ctrl+x m" {
		t.Fatalf("/model footer = %q, want ctrl+x m", byTitle["model"].footer)
	}
	if byTitle["help"].footer != "" {
		t.Fatalf("/help footer = %q, want blank (help_show = none)", byTitle["help"].footer)
	}
	if byTitle["quit"].footer != "ctrl+c / ctrl+d / ctrl+x q" {
		t.Fatalf("/quit footer = %q, want the app_exit comma-list display", byTitle["quit"].footer)
	}
}

func TestPaletteOpen(t *testing.T) {
	a := testApp()
	a.store.Commands = []protocol.Command{{Name: "/help", Description: "Show help"}}
	a.openPaletteDialog()
	d, ok := a.dlg.top()
	if !ok || d.kind != dlgPalette || d.sel == nil {
		t.Fatalf("after openPaletteDialog: top=%+v (ok=%v), want the palette select", d, ok)
	}
}

func TestPaletteDispatch(t *testing.T) {
	a := testApp()
	a.store.Commands = []protocol.Command{{Name: "/help", Description: "Show help"}}
	a.handleKey(pressCtrlP()) // command_list → the palette (the S4.2 remap lands)
	d, ok := a.dlg.top()
	if !ok || d.kind != dlgPalette {
		t.Fatalf("after ctrl+p: top=%+v (ok=%v), want the palette", d, ok)
	}
}
```

**Step 2 — confirm FAIL.** `go test ./internal/tui/ -run 'TestPalette' -count=1` → FAIL (build fails: undefined `dlgPalette`, `openPaletteDialog`, `paletteOptions` — the expected red).

**Step 3 — minimal implementation.**

(a) `internal/tui/dialog.go` — add `dlgPalette` to the `dialogKind` const block (after `dlgThemes`), the `modalInner` case, and the `handleDialogKey` case:
```go
	// (dialogKind const block, after dlgThemes)
	dlgPalette
```
```go
	// (modalInner, after the dlgHelp case)
	case dlgPalette:
		if d.sel != nil {
			return d.sel.view(w, h, a.theme)
		}
```
```go
	// (handleDialogKey, after the dlgThemes case)
	case dlgPalette:
		if d.sel != nil {
			return d.sel.handleKey(a, k)
		}
		a.dlg.pop()
		return nil
```

(b) `internal/tui/commands.go` — add `openPaletteDialog`/`paletteOptions`/`commandBindings`:
```go
// commandBindings maps the yolo command names to the registry binding names
// (the referent subset — the commands with a registry default; the palette
// footer shows the registry's Format for each, blank when "none").
var commandBindings = map[string]string{
	"/help":     "help_show",
	"/new":      "session_new",
	"/model":    "model_list",
	"/agents":   "agent_list",
	"/quit":     "app_exit",
	"/sessions": "session_list",
	"/connect":  "provider_connect",
	"/status":   "status_view",
	"/themes":   "theme_list",
}

// openPaletteDialog pushes the command palette select modal (S4.4): the
// options = the merged command list (the 4 local commands first, then the
// GET /command catalog — the slash-menu convention; an empty pre-hydrate
// catalog degrades to the locals). Each option's footer = the registry
// binding's Format (the commandBindings referent subset; blank when "none").
// The onSelect is wired in S4.5 (the run-on-enter) — at S4.4 the palette
// opens and filters (the S2.5 fuzzy) but enter is inert.
func (a *App) openPaletteDialog() []tea.Cmd {
	m := selectNew("Commands", "Filter commands", paletteOptions(a), nil, nil, nil)
	a.pushModal(dialog{kind: dlgPalette, sel: m}, dlgMedium, nil)
	return nil
}

// paletteOptions builds the palette select options from the merged command
// list (the 4 local commands first, then the GET /command catalog).
func paletteOptions(a *App) []selectOption {
	var opts []selectOption
	for _, c := range a.mergedCommands() {
		footer := ""
		if bn, ok := commandBindings[c.Name]; ok {
			if f := a.keymap.Format(bn); f != "none" {
				footer = f
			}
		}
		opts = append(opts, selectOption{
			title:       strings.TrimPrefix(c.Name, "/"),
			description: c.Description,
			footer:      footer,
			value:       c.Name,
		})
	}
	return opts
}
```
(`commands.go` already imports `strings` — no new import.)

(c) `internal/tui/keys.go` — add the `command_list` case to `dispatchCommand` (the S4.2 remap lands):
```go
	case "command_list":
		return a.openPaletteDialog()
```

**Step 4 — gate.** `go test ./internal/tui/ -run 'TestPalette' -count=1` → PASS, then FULL gate: `go vet ./... && go test ./...` + `gofmt -l .` empty.

**Step 5 — commit + close the bead.**
`git add internal/tui/dialog.go internal/tui/commands.go internal/tui/keys.go internal/tui/palette_test.go && git commit -m "feat: command palette - overlay + fuzzy filter"`
`bd close <S4.4 bead> --reason "palette overlay green: dlgPalette select over the merged command list, the registry footers, the ctrl+p remap lands, the S2.5 fuzzy filter active" --json`

---

### Task S4.5: Command palette: arrow nav + enter runs + esc closes (bead `yolo-oae.5.5`, expected id `yolo-oae.5.6`)

**Files:** `internal/tui/commands.go` (wire the `onSelect` in `openPaletteDialog` + add `paletteSelectPick`); `internal/tui/palette_test.go` (extend the imports + `TestPaletteSelectPick`/`TestPaletteNav`/`TestPaletteEsc`/`TestTUICommandPalette`).

**Interfaces:** produces: `(*App).paletteSelectPick(o selectOption)` (the run-on-enter: `closeTopModal()` then `runCommand(o.value.(string))`). The arrow nav rides the S2.5 `selectModel.handleKey` (up/down move with wraparound); the esc close rides the S2.2 modal stack (`handleDialogKey`'s esc/ctrl+c → `closeTopModal`). S4.5 lands the run wiring (the S4.4 inert `onSelect` becomes live) + the verification tests + the teatest golden.

**Upstream parity notes:** the run-on-enter is the port of the upstream palette's `onSelect` (command-palette.tsx:56-59: `dialog.clear()` + `dispatchCommand`). The arrow nav + esc close are the S2.5 `selectModel` + the S2.2 modal stack (the upstream DialogSelect nav + the dialog esc — no yolo-specific nav; the yolo select is the DialogSelect port). The Suggested bucket stays unported (deviation 212 — S4.4's note).

**Step 1 — write the failing tests.** Extend `internal/tui/palette_test.go` (add `time`/`tea`/`teatest`/`testutil`/`client`/`store` to the imports):

```go
func TestPaletteSelectPick(t *testing.T) {
	a := testApp()
	a.store.Commands = []protocol.Command{{Name: "/help", Description: "Show help"}}
	a.openPaletteDialog()
	d, _ := a.dlg.top()
	sel := d.sel
	sel.sel = 0 // the local /sessions (first)
	sel.submit(a)
	d, ok := a.dlg.top()
	if ok && d.kind == dlgPalette {
		t.Fatal("the palette must close after a run")
	}
	if d.kind != dlgSessions {
		t.Fatalf("after the palette run: top=%+v, want the session-list dialog", d)
	}
}

func TestPaletteNav(t *testing.T) {
	a := testApp()
	a.store.Commands = []protocol.Command{
		{Name: "/help", Description: "Show help"},
		{Name: "/new", Description: "New session"},
	}
	a.openPaletteDialog()
	d, _ := a.dlg.top()
	sel := d.sel
	n := len(sel.filtered())
	if sel.sel != 0 {
		t.Fatalf("initial sel = %d, want 0", sel.sel)
	}
	sel.handleKey(a, press(tea.KeyDown))
	if sel.sel != 1 {
		t.Fatalf("sel after down = %d, want 1", sel.sel)
	}
	sel.handleKey(a, press(tea.KeyUp))
	sel.handleKey(a, press(tea.KeyUp)) // wraps to the last
	if sel.sel != n-1 {
		t.Fatalf("sel after wrap-up = %d, want last (%d)", sel.sel, n-1)
	}
}

func TestPaletteEsc(t *testing.T) {
	a := testApp()
	a.store.Commands = []protocol.Command{{Name: "/help", Description: "Show help"}}
	a.openPaletteDialog()
	a.handleKey(press(tea.KeyEscape))
	if d, ok := a.dlg.top(); ok {
		t.Fatalf("after esc: top=%+v, want the palette closed", d)
	}
}

func TestTUICommandPalette(t *testing.T) {
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))

	teatest.WaitFor(t, tm.Output(), hasLine("New session"), teatest.WithDuration(5*time.Second))

	// S4.4: ctrl+p opens the command palette (the remap).
	tm.Send(pressCtrlP())
	teatest.WaitFor(t, tm.Output(), hasLine("Commands"), teatest.WithDuration(5*time.Second))

	// filter to "help" (the S2.5 fuzzy narrows), enter runs /help.
	for _, r := range "help" {
		tm.Send(press(r))
	}
	tm.Send(press(tea.KeyEnter))
	teatest.WaitFor(t, tm.Output(), hasLine("Help"), teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
```

**Step 2 — confirm FAIL.** `go test ./internal/tui/ -run 'TestPalette' -count=1` → FAIL (`TestPaletteSelectPick` red: the S4.4 `onSelect` is nil, so `submit` does nothing — the palette stays open and no command runs; the nav/esc pass — they ride the existing select + modal stack).

**Step 3 — minimal implementation.** `internal/tui/commands.go` — wire the `onSelect` in `openPaletteDialog` (the S4.4 inert `nil` becomes the run callback) and add `paletteSelectPick`:

```go
// (openPaletteDialog — the onSelect wires the run-on-enter, S4.5)
func (a *App) openPaletteDialog() []tea.Cmd {
	m := selectNew("Commands", "Filter commands", paletteOptions(a), nil,
		func(app *App, o selectOption) { app.paletteSelectPick(o) }, nil)
	a.pushModal(dialog{kind: dlgPalette, sel: m}, dlgMedium, nil)
	return nil
}

// paletteSelectPick is the palette's onSelect (S4.5): it closes the palette
// and runs the selected command (the run-on-enter contract — the port of the
// upstream dialog.clear() + dispatchCommand).
func (a *App) paletteSelectPick(o selectOption) {
	a.closeTopModal()
	if v, ok := o.value.(string); ok {
		a.runCommand(v)
	}
}
```

**Step 4 — gate.** `go test ./internal/tui/ -run 'TestPalette' -count=1` → PASS, then FULL gate: `go vet ./... && go test ./...` + `gofmt -l .` empty (the teatest `TestTUICommandPalette` green).

**Step 5 — commit + close the bead.**
`git add internal/tui/commands.go internal/tui/palette_test.go && git commit -m "feat: command palette - nav, run, esc"`
`bd close <S4.5 bead> --reason "palette nav/run/esc green: the run-on-enter wired (closeTopModal + runCommand), the S2.5 nav + S2.2 esc verified, the teatest golden lands" --json`

---

### Task S4.6: Which-key: pending prefix-group overlay (registry-driven) (bead `yolo-oae.5.6`, expected id `yolo-oae.5.7`)

**Files:** new `internal/tui/whichkey.go` (the overlay: `whichKeyEntry`/`whichKeyGroup`/`whichKeyCategory`/`whichKeyGrouped`/`(*Keymap).whichKeyEntries`/`(*App).whichKeyView`); `internal/tui/view.go` (render `wk` in the non-modal path: `viewSession` takes `wk` into the below-viewport overlay budget, and both routes append `wk` after `dlg`); new `internal/tui/whichkey_test.go`.

**Interfaces:** produces: `whichKeyEntry{key, label, group string, continues bool}`; `whichKeyGroup{label string, entries []whichKeyEntry}`; `whichKeyCategory(name string) string` (binding-name prefix → group label, deviation 213); `whichKeyGrouped(entries []whichKeyEntry) []whichKeyGroup` (group by label, groups sorted by label, entries within a group sorted continues-desc/label/key — the upstream `grouped`, which-key.tsx:144-156); `(*Keymap).whichKeyEntries() []whichKeyEntry` (the held leader's continuation bindings for the current context — registry-driven); `(*App).whichKeyView(w int) string` (the overlay string, `""` when the leader is not pending / a modal is open / no entries).

**Upstream parity notes:** the overlay is the port of the upstream which-key feature (which-key.tsx) — the `Entry`/`Group` types (which-key.tsx:66-77), `activeKeyEntry`/`grouped` (which-key.tsx:126-156). Three yolo divergences (deviation 207, behavior/low): (1) the overlay is **always available** (the upstream `enabled:false` feature flag + KV-gated layout are dropped — no KV, the layout is the in-memory overlay); (2) the overlay is **in-memory** (no `which_key_layout`/`which_key_pending_preview` KV — the toggles `which_key_toggle`/`which_key_layout_toggle`/`which_key_pending_toggle` are registry-defined but INERT in yolo: the overlay appears when the leader is *held*, not toggled); (3) the overlay is **context-filtered** — it lists only the current context group's leader-continuation bindings (the upstream `active()` is context-aware; the yolo referent is the S4.2 `contextGroups` map), so the inert unwired bindings (the 9 `session_quick_switch_*`, `messages_*`, the `session_timeline`/`export`/`compact`/`queued_prompts`) do not clutter the base overlay. Deviation 213 (render/low): the group label is the binding-name prefix (the upstream `commandAttrs.category` is not a yolo field); the entry label is the binding's `Definitions` description (the upstream `commandAttrs.title ?? bindingAttrs.desc`). The `continues` flag is carried (the upstream `+key` continuation label, which-key.tsx:138) but the default bindings are all leaf continuations (`continues=false`); it is future-proofing for a nested prefix. The overlay is **non-interactive** (a passive display dismissed by the leader timeout / continuation — the upstream group-nav/scroll/page bindings are not wired). Geometry (which-key.tsx constants MIN8/MAX16/0.3, MIN_COL/MAX_COL) is referenced but the v1 overlay is a compact line-per-group panel (the context filter bounds the entry count, so the upstream multi-column/scroll layout is not needed).

**Step 1 — write the failing tests.** New `internal/tui/whichkey_test.go`:

```go
package tui

import (
	"strings"
	"testing"
)

func TestWhichKeyEntriesBase(t *testing.T) {
	a := testApp() // home route, base mode
	a.pendingLeader = true
	v := stripANSI(a.whichKeyView(80))
	if v == "" {
		t.Fatal("the overlay must render while the leader is pending")
	}
	for _, want := range []string{"Leader", "Agent", "App", "Model", "Session", "Status", "Theme"} {
		if !strings.Contains(v, want) {
			t.Errorf("overlay missing group %q:\n%s", want, v)
		}
	}
	for _, want := range []string{
		"Exit the application", "List available models", "List agents",
		"View status", "List available themes", "Create a new session", "List all sessions",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("overlay missing label %q:\n%s", want, v)
		}
	}
}

func TestWhichKeyHiddenWhenNotPending(t *testing.T) {
	a := testApp()
	if v := a.whichKeyView(80); v != "" {
		t.Fatalf("the overlay must be empty when the leader is not pending, got:\n%s", stripANSI(v))
	}
}

func TestWhichKeyHiddenInModal(t *testing.T) {
	a := testApp()
	a.pendingLeader = true
	a.openModelDialog()
	if d, ok := a.dlg.top(); !ok || !d.modal {
		t.Fatal("the model dialog must be a modal (the overlay preconditions)")
	}
	if v := a.whichKeyView(80); v != "" {
		t.Fatalf("the overlay must be empty while a modal is open, got:\n%s", stripANSI(v))
	}
}

func TestWhichKeyRegistryDriven(t *testing.T) {
	a := testApp()
	a.pendingLeader = true
	if err := a.keymap.Set("model_list", "none"); err != nil {
		t.Fatal(err)
	}
	v := stripANSI(a.whichKeyView(80))
	if strings.Contains(v, "List available models") {
		t.Fatalf("a disabled binding's entry must not render:\n%s", v)
	}
}
```

**Step 2 — confirm FAIL.** `go test ./internal/tui/ -run 'TestWhichKey' -count=1` → FAIL (build fails: undefined `whichKeyView`, `whichKeyEntries` — the expected red).

**Step 3 — minimal implementation.** New `internal/tui/whichkey.go`:

```go
package tui

import (
	"sort"
	"strings"
)

// whichKeyEntry is one row of the which-key overlay (the port of the upstream
// Entry, which-key.tsx:66-71): the continuation key, the binding's display
// label, its category group and the continuation flag (deviation 213).
type whichKeyEntry struct {
	key       string
	label     string
	group     string
	continues bool
}

// whichKeyGroup is a category bucket (the port of the upstream Group,
// which-key.tsx:74-77).
type whichKeyGroup struct {
	label   string
	entries []whichKeyEntry
}

// whichKeyCategory maps a binding name to its overlay group by its prefix
// (deviation 213 — the upstream command category is not a yolo field; the
// binding-name prefix is the yolo referent).
func whichKeyCategory(name string) string {
	switch {
	case strings.HasPrefix(name, "which_key"):
		return "Keymap"
	case strings.HasPrefix(name, "app_"), strings.HasPrefix(name, "sidebar_"):
		return "App"
	case strings.HasPrefix(name, "command_"):
		return "Commands"
	case strings.HasPrefix(name, "help_"):
		return "Help"
	case strings.HasPrefix(name, "diff_"):
		return "Diff"
	case strings.HasPrefix(name, "editor_"):
		return "Editor"
	case strings.HasPrefix(name, "theme_"):
		return "Theme"
	case strings.HasPrefix(name, "status_"):
		return "Status"
	case strings.HasPrefix(name, "session_"):
		return "Session"
	case strings.HasPrefix(name, "stash_"):
		return "Stash"
	case strings.HasPrefix(name, "model_"):
		return "Model"
	case strings.HasPrefix(name, "mcp_"):
		return "MCP"
	case strings.HasPrefix(name, "provider_"):
		return "Provider"
	case strings.HasPrefix(name, "agent_"):
		return "Agent"
	case strings.HasPrefix(name, "messages_"):
		return "Messages"
	case strings.HasPrefix(name, "prompt_"):
		return "Prompt"
	case strings.HasPrefix(name, "input_"):
		return "Input"
	case strings.HasPrefix(name, "workspace_"):
		return "Workspace"
	case strings.HasPrefix(name, "dialog."), strings.HasPrefix(name, "permission."):
		return "Dialog"
	default:
		return "Other"
	}
}

// whichKeyGrouped buckets the entries by group (the port of the upstream
// grouped, which-key.tsx:144-156): groups sorted by label; entries within a
// group sorted continues-desc, then label, then key.
func whichKeyGrouped(entries []whichKeyEntry) []whichKeyGroup {
	m := map[string][]whichKeyEntry{}
	var order []string
	for _, e := range entries {
		if _, ok := m[e.group]; !ok {
			order = append(order, e.group)
		}
		m[e.group] = append(m[e.group], e)
	}
	sort.Strings(order)
	out := make([]whichKeyGroup, 0, len(order))
	for _, label := range order {
		es := m[label]
		sort.Slice(es, func(i, j int) bool {
			if es[i].continues != es[j].continues {
				return es[j].continues
			}
			if es[i].label != es[j].label {
				return es[i].label < es[j].label
			}
			return es[i].key < es[j].key
		})
		out = append(out, whichKeyGroup{label: label, entries: es})
	}
	return out
}

// whichKeyEntries returns the held leader's continuation bindings for the
// current context (S4.6): the enabled bindings in the current mode's context
// group whose sequence carries the <leader> prefix, one entry per binding
// (the continuation key + the binding description + the prefix-derived
// category). The overlay lists what the held leader can dispatch here
// (deviation 207(3): the context filter keeps the inert unwired bindings out).
func (km *Keymap) whichKeyEntries() []whichKeyEntry {
	leader := km.bindings["leader"]
	if !leader.enabled || len(leader.seqs) == 0 {
		return nil
	}
	var out []whichKeyEntry
	for _, name := range contextGroups[km.Current()] {
		if name == "leader" {
			continue
		}
		bv := km.bindings[name]
		if !bv.enabled {
			continue
		}
		for _, seq := range bv.seqs {
			has, rest := leaderSplit(seq)
			if !has {
				continue
			}
			out = append(out, whichKeyEntry{
				key:       formatKeySequence(rest, km.leaderDisplay()),
				label:     Definitions[name].Description,
				group:     whichKeyCategory(name),
				continues: false,
			})
			break // one entry per binding (the first leader seq)
		}
	}
	return out
}

// whichKeyView renders the pending prefix-group overlay (S4.6): the held
// leader's continuation bindings for the current context, grouped by
// category. Empty when the leader is not pending, a modal dialog is open (the
// modal frame owns the frame), or the current context has no
// leader-continuation bindings.
func (a *App) whichKeyView(w int) string {
	if !a.pendingLeader {
		return ""
	}
	if d, ok := a.dlg.top(); ok && d.modal {
		return ""
	}
	entries := a.keymap.whichKeyEntries()
	if len(entries) == 0 {
		return ""
	}
	groups := whichKeyGrouped(entries)
	var lines []string
	lines = append(lines, a.theme.TextMuted().Render("Leader ("+a.keymap.leaderDisplay()+")"))
	for _, g := range groups {
		var parts []string
		for _, e := range g.entries {
			parts = append(parts, e.key+" "+e.label)
		}
		body := "  " + g.label + ": " + strings.Join(parts, "   ")
		for _, l := range strings.Split(wrapLine(body, w), "\n") {
			lines = append(lines, a.theme.TextMuted().Render(l))
		}
	}
	return strings.Join(lines, "\n")
}
```

`internal/tui/view.go` — render the overlay in the non-modal path. `viewSession` takes `wk` and counts it in the below-viewport overlay budget; `view()` builds `wk` and appends it after `dlg` on both routes:

```go
// view() — after the dlg/menu/perm/toasts block:
//	wk := a.whichKeyView(w)
// and in the route switch:
//	if a.route == routeSession {
//		b.WriteString(a.viewSession(menu, perm, toasts, dlg, wk))
//	} else {
//		b.WriteString(a.home.render(&a.store, w, a.theme))
//	}
// ...
//	if dlg != "" {
//		b.WriteString("\n" + dlg)
//	}
//	if wk != "" {
//		b.WriteString("\n" + wk)
//	}

// viewSession(menu, perm, toasts, dlg, wk string) — the overlay budget now
// includes wk:
//	overlays := 0
//	for _, v := range []string{perm, toasts, dlg, wk} {
//		if v != "" {
//			overlays += 1 + strings.Count(v, "\n")
//		}
//	}
```

**Step 4 — gate.** `go test ./internal/tui/ -run 'TestWhichKey' -count=1` → PASS, then FULL gate: `go vet ./... && go test ./...` + `gofmt -l .` empty (the existing home/session teatest goldens are untouched — the overlay is empty in every non-leader-pending flow).

**Step 5 — commit + close the bead.**
`git add internal/tui/whichkey.go internal/tui/whichkey_test.go internal/tui/view.go && git commit -m "feat: which-key - prefix group overlay"`
`bd close <S4.6 bead> --reason "which-key overlay green: the pending prefix-group panel renders the held leader's context-filtered continuation bindings from the registry; hidden when not pending / in a modal; registry-driven (a remap changes it)" --json`

---

### Task S4.7: Which-key: registry integration — /help + footer hints render from it (bead `yolo-oae.5.7`, expected id `yolo-oae.5.8`)

**Files:** `internal/tui/dialog.go` (rewire `paletteShortcut` to the registry); `internal/tui/help_test.go` (add `TestHelpPaletteHintFromRegistry`).

**Interfaces:** produces: a registry-driven `(*App).paletteShortcut() string` — the single registry-integration seam for the key-hint surfaces, now `a.keymap.Format("command_list")` (the S4.1 default is `ctrl+p`, so the existing `/help` + teatest goldens are **byte-identical**). No new signature.

**Upstream parity notes:** the `/help` palette hint (the "Press \<key\> to see all available actions…" line) is the port of the upstream help surface's palette hint, now driven by the keymap registry instead of the pre-S4 yolo constant `ctrl+p` (deviation 195, now resolved by the rewire). Scoping (deviation 208, behavior/info): the V1-pinned help line `"pgup/pgdn scroll · \+enter newline"` and the home/session footer help lines (`helpText` = `↑/↓ move · enter open · n new · /help`, `sessionHelp` = `pgup/pgdn scroll · alt+e expand · alt+t think · esc abort/back`) are **kept hardcoded byte-identical** — their yolo-surface referents (the home `n`, the session `alt+e`/`alt+t`, the `\`+enter soft-enter) are not registry bindings (deviations 210/211), so only the *palette* hint is registry-driven. The footer's status segments (model · agent · tokens · cost · conn, `footerView`) are store-derived, not key hints, so they are untouched. The `paletteShortcut` seam is the single registry-integration point: any future footer hint that reports the palette keybind routes through it.

**Step 1 — write the failing test.** `internal/tui/help_test.go`:

```go
func TestHelpPaletteHintFromRegistry(t *testing.T) {
	a := testApp()
	// S4.7: the default palette hint is the registry's command_list binding
	// (byte-identical to the pre-S4 "ctrl+p" — the existing goldens hold).
	if got := a.paletteShortcut(); got != "ctrl+p" {
		t.Fatalf("default palette hint = %q, want ctrl+p (the registry default)", got)
	}
	// A remap is reflected in the hint and in the /help body (registry-driven).
	if err := a.keymap.Set("command_list", "ctrl+k"); err != nil {
		t.Fatal(err)
	}
	if got := a.paletteShortcut(); got != "ctrl+k" {
		t.Fatalf("remapped palette hint = %q, want ctrl+k", got)
	}
	v := stripANSI(a.helpDialogView(80, 24, a.theme))
	if !strings.Contains(v, "Press ctrl+k to see all available actions") {
		t.Fatalf("the /help palette hint must reflect the remap:\n%s", v)
	}
	// The V1-pinned line is untouched (kept byte-identical).
	if !strings.Contains(v, "pgup/pgdn scroll \u00B7 \\+enter newline") {
		t.Fatalf("the V1-pinned help line must stay byte-identical:\n%s", v)
	}
}
```

**Step 2 — confirm FAIL.** `go test ./internal/tui/ -run 'TestHelpPaletteHintFromRegistry' -count=1` → FAIL (`paletteShortcut` still returns the hardcoded `"ctrl+p"`, so the `ctrl+k` assertions are red).

**Step 3 — minimal implementation.** `internal/tui/dialog.go` — rewire `paletteShortcut` to the registry:

```go
// paletteShortcut is the palette keybind the hint surfaces report (the
// registry-integration seam, deviation 195 resolved at S4.7): the
// registry's command_list binding, formatted for display. The S4.1 default
// is "ctrl+p", so the /help + teatest goldens are byte-identical.
func (a *App) paletteShortcut() string { return a.keymap.Format("command_list") }
```

**Step 4 — gate.** `go test ./internal/tui/ -run 'TestHelp|TestTUI' -count=1` → PASS (the existing `help_test.go` "Press ctrl+p to see all available actions and commands in any context." pin and the `tui_suite_test.go` help capture are byte-identical — the default remap is a no-op), then FULL gate: `go vet ./... && go test ./...` + `gofmt -l .` empty.

**Step 5 — commit + close the bead.**
`git add internal/tui/dialog.go internal/tui/help_test.go && git commit -m "feat: which-key - /help + footer hints from registry"`
`bd close <S4.7 bead> --reason "which-key registry integration green: the /help palette hint (the paletteShortcut seam) renders from the keymap registry (byte-identical default, a remap changes it); the V1-pinned + yolo-surface hint lines stay hardcoded (dev 208/210/211)" --json`

---

## S4 slice gate (slice bead `yolo-oae.5`)

NOT a task bead; runs after all child beads close. Mirror the S0 slice gate
shape: (1) module gate `go vet ./... && go test ./...` + `gofmt -l .` empty
(incl. `TestImportsDirection` + the S4 teatest goldens; the V1 keymap pins
green); (2) user-run smoke (NOT CI): in a real TTY — remap a binding via
`yolo.jsonc` and via the runtime remap, open the command palette (filter +
run), and hold a key prefix to see the which-key overlay; (3) append any
forced DEVIATIONS.md entries this slice named (with severity, same-commit
rule — root principle 2; spec §9 risk 5: the fuzzy ranking order is checked
in the S8 diff, small scoring tweak if visibly off); (4) PROGRESS.md
one-line status pointer; (5) commit
`docs: checkpoint — S4 done, next is S5 detail pass`; (6)
`bd close yolo-oae.5 --reason "all 7 child beads closed, gate green" --json`.
