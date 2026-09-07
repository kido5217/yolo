# 0.10.0 slash menu — Design

Date: 2026-09-07 · Status: draft (pending user approval; seeds the slash-menu 0.10.0
execution epic) · Bead: `yolo-old.4`
Upstream referent: opencode v1.18.18 `component/prompt/autocomplete.tsx`
(`/tmp/opencode-upstream/packages/tui/src/component/prompt/autocomplete.tsx`) —
**reference, not contract** (root principle 2). Scope: the `/` slash menu full
parity — the above-input anchored bordered dropdown, theme-token chrome,
selection highlight, mouse hover/select, `tab`=complete, frecency scoring,
`esc` semantics, the home hint row, the empty-state text, and the
`DEVIATIONS.md` re-baselines. No new dependencies (stdlib + the existing
allowlist only). This spec seeds the slash-menu 0.10.0 execution epic; the
`@` mention picker (yolo-old.5) and the `ctrl+p` palette (yolo-old.6) are
sibling specs.

## 1. Destination (the full-parity bar, slash menu only)

The `/` slash menu becomes a pixel-faithful port of upstream's autocomplete:
the input line shows a `> ` prompt; typing `/` opens a **bordered dropdown
anchored above the input** (same left edge and width as the prompt box) that
filters the command list by fuzzy match, ranks by **frecency** (recent +
frequent first, prefix-match ×2), caps at 10 rows, and is driven by **mouse**
(hover moves the selection, click selects) as well as **keyboard** (up/down
with wrap, `enter` executes, `tab` completes, `esc` clears). The chrome is
theme-token (a `theme.border` split left/right border, a `backgroundMenu`
inner fill, a `primary`-background / `selectedForeground` selection row), and
the empty-filter state renders `No matching items`. Every deviation from
upstream is logged in `DEVIATIONS.md`. This spec covers the **slash menu only**;
the `@` picker and the palette are out of scope (they reuse the shared
dropdown primitive and land in their own specs).

## 2. Current state (file:line)

What exists today, from the yolo-side inventory (research/yolo-menus) and the
live code:

- **Command menu** — `menuItems` (prompt.go:112-168): fuzzy over canonical+alias
  names, prefix match ×2, sort by score desc, cap `maxPickerOptions` (10,
  mention.go:21). No frecency factor. `runCommand` (commands.go:237) executes the
  selected command and clears the input (run-on-enter).
- **Slash render** — `menuView` (prompt.go:173-198): an **ungated** plain row
  (no border, no inner background), a `cursorStyle` selection highlight
  (style.go:31 — bold + primary fg), label `th.Text()` and description
  `th.TextMuted()`, the empty line `no match` (prompt.go:178).
- **@-picker render** — `acView` (prompt.go:222-248): the same plain style.
- **Selection** — `moveMenuSel` (prompt.go:251-257): up/down move the selection
  index with **wraparound** (`((sel+d)%n+n)%n`); no scroll window (the cap-10
  list is fully rendered, the selection index is the row).
- **Keys** — `handleMenuKey` (keys.go:193-213): up/down → `moveMenuSel` (wrap);
  `enter` → `runCommand` if a match else `clearPrompt`; `esc` → `clearPrompt`
  (clears the **entire** input). Ladder (keys.go:19-56): permission > dialog >
  `handleAppKeys` (base group: `agent_cycle`=tab, `agent_cycle_reverse`=shift+tab,
  keymap.go:135-136, 818-820) > `handleMenuKey` > `handleAcKey` > route > prompt.
  So today `tab` with the menu open cycles the agent (deviation 289), because
  `handleAppKeys` precedes `handleMenuKey`.
- **Placement** — home: the slash menu + `@`-picker are left-aligned **overlay
  rows** in the home frame, appended after the fixed stack (logo/box/hint) and
  before the footer (home.go:515-522). Session: the menu is appended after the
  transcript viewport and before the prompt line, reducing the viewport height
  (view.go:33-56, viewSession 90-117). The home box geometry: `boxL`/`boxW`
  (home.go:486-487), `boxW = min(75, contentW)` (home.go:137-140).
- **Frecency** — file-path frecency only (the S5.3/S5.4 `@`-picker surface):
  `freq []frecencyEntry` (app.go:116), `kvFrecencyKey = "prompt_frecency"`
  (app.go:580), `loadFrecency`/`saveFrecency` (app.go:619-633), the
  `freq/(1+age_days)` score (frecency.go:27-36), the `@`-picker ranking
  (mention.go:144-188), and the touch (`updateFrecency`+`saveFrecency`,
  acInsert mention.go:193-207). No command-name frecency.
- **Theme tokens** — `SelectedForeground` (theme/styles.go:77-97); `BackgroundMenu`,
  `Background`, `Primary`, `Border`, `Text`, `TextMuted` (theme/styles.go:99-114).
- **Mouse** — absent (no `tea.MouseMsg` case in `App.Update`/`updateMsg`,
  app.go:316-329; `tea.NewProgram(app)` without mouse opts, main.go:768).
- **Home hint** — `homeHintLine` (home.go:355): `seg("agent_cycle"," agents")`
  + `seg("command_list"," commands")` → `tab agents  ctrl+p commands`.

## 3. Design

### 3.1 The dropdown primitive (`internal/tui/dropdown.go`)

A new shared primitive both the slash menu and (the sibling `@` spec) the
`@`-picker build on. It renders a **bordered row list** anchored above the
input. Chrome (theme-token, per the settled design grill):

- **Border**: a split left/right border (a vertical `┃`-class rune) colored
  `theme.border`, **no top or bottom edges**.
- **Inner fill**: `theme.backgroundMenu` behind every row (the inner background).
- **Rows**: padding 1 (one leading space after the left border).
- **Selection row**: background `theme.primary`, foreground
  `theme.selectedForeground` (the `SelectedForeground` luminance rule,
  theme/styles.go:77-97).
- **Unselected rows**: label `theme.text`; the slash description text
  `theme.textMuted`.
- **Width** = the prompt box width (home `boxW`, home.go:486-487; the session
  prompt-line width); **height** = `min(10, count, space-above)` — capped at
  10, clamped to the rows available above the anchor; **no scroll** (all
  rendered rows are the ranked top-N).
- **Row content** (slash): the command name (`/name`) + the description, two
  columns (name left, description right-offset), mirroring the current
  `menuView` two-column layout (prompt.go:184-189) but inside the bordered box.

**Anchors** (replaces the 0.8.0 left-overlay placement, §4.1 entry 1):

- **Home** — the dropdown sits **above the box's top edge**, at the **same left
  edge and width** as the box (`boxL`/`boxW`); the **logo is overlaid** while
  open (the dropdown paints over the logo rows).
- **Session** — the dropdown sits **above the prompt line**, **overlaying the
  transcript tail** (the viewport keeps its full height; the dropdown is painted
  over the bottom transcript rows — the upstream `position:absolute`/`zIndex 100`
  semantics, autocomplete.tsx:722-732). This replaces the current below-transcript
  block that shrinks the viewport (view.go:47-56).

**Home, open** (selected row = `primary` bg; logo overlaid while open):

```
        ┃
        ┃  /help        Show the help dialog      ┃
        ┃  /model       List models               ┃  ← selected (primary bg)
        ┃  /agents      List agents               ┃
        ┃  /status      Show session status       ┃
        ┃  /themes      Browse themes             ┃
        ┃                                        ← box row0 (┃ + fill)
        ┃  Ask anything... "Fix a TODO…"          ← box row1 (input)
        ┃                                        ← box row2
        ┃  Build · Qwen3.8-27B kido               ← box row3 (meta)
        ╹▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀      ← box row4 (bottom)
        tab complete  ctrl+p commands            ← hint row (context-aware)
```

**Session, open** (the dropdown covers the transcript tail):

```
   some transcript line
   another transcript line
        ┃  /help        Show the help dialog      ┃
        ┃  /model       List models               ┃  ← selected (primary bg)
        ┃  /agents      List agents               ┃
   > ┃                                      ← prompt line
```

**Selection model** — up/down move the selection index with **wraparound**
(current `moveMenuSel`, prompt.go:251-257, unchanged); **no scroll window** —
the cap-10 list is fully rendered and the selection index is the row.

**Mouse** — cell-based, scoped to the slash dropdown (the `@` spec reuses the
same hit-test): `tea.WithMouseCellMotion` on the Program options (main.go:768),
a `case tea.MouseMsg` in `App.Update`/`updateMsg` (app.go:316-329) routed to the
dropdown when the slash menu is open, and **row hit-testing** that maps the
mouse cell's row (relative to the dropdown's first row) to a row index.
**Hover** (mouse motion over a row) moves the selection to that row; **click**
(mouse release on a row) selects that row = the `enter` action for the slash
menu (run-on-enter: run the selected command). The `@` spec's click = insert the
path (acInsert); the primitive exposes a select callback the owner implements.

### 3.2 Key behavior

The ladder (keys.go:19-56) keeps its precedence except the **tab interception**:
when the slash menu is open, `tab` is handled in the menu path **before**
`handleAppKeys` (no keymap rebind — the `prompt.autocomplete.complete` binding,
keymap.go:223, is the referent). `shift+tab` is unchanged (`agent_cycle_reverse`).

| key         | menu open                                             | menu closed                        |
|-------------|-------------------------------------------------------|------------------------------------|
| `up`/`down` | move selection, wraparound (keys.go:196-198)          | normal input movement              |
| `enter`     | **run** the selected command (run-on-enter, KEPT)     | send the typed text                |
| `tab`       | **complete**: replace whole input with `/name ` (trailing space), menu closes | cycle the agent (deviation 289) |
| `shift+tab` | cycle the agent (reverse, unchanged)                  | cycle the agent (reverse)          |
| `esc`       | **clear the entire input** (parity `/`-mode, KEPT)    | (route-level behavior)             |

- **`enter`** (menu open) — run-on-enter: `runCommand` executes the selected
  command immediately (keys.go:202-205 → commands.go:237); the input clears and
  the command runs. Upstream's select-replaces-input-on-enter is **not** adopted.
- **`tab`** (menu open) — the hybrid completion: the whole input is replaced
  with `/name ` (trailing space) and the menu **closes** (the upstream
  `/`-completion, autocomplete.tsx:456-462). `tab` (menu closed) still cycles
  the agent; `shift+tab` is always `agent_cycle_reverse`.
- **`esc`** (menu open) — clear the entire input (keys.go:208-210), the upstream
  `/`-mode `hide()` semantics (autocomplete.tsx:650-661) — parity, kept.

**Home hint row** — context-aware first segment (home.go:355): `tab complete`
while the slash menu is open, `tab agents` when it is closed (the shortcut stays
`Format("agent_cycle")` = `tab`); the `ctrl+p commands` segment is unchanged.
The home tip (bottom, the pool entry) is unchanged.

### 3.3 Frecency (command-name)

Command-name frecency scoring for the `/` menu, mirroring the file-path
frecency (app.go:580) and the `@`-picker ranking (mention.go:144-188):

- **Storage** — a new KV key (e.g. `command_frecency`) in the same theme KV as
  `prompt_frecency` (app.go:580); the entry shape mirrors `frecencyEntry`
  (frecency.go:10-14) keyed by **command name** (`{name, frequency, lastOpen}`);
  loaded at boot (a `loadCommandFrecency` mirroring `loadFrecency`, app.go:619)
  and persisted on touch (a `saveCommandFrecency` mirroring `saveFrecency`,
  app.go:629). TUI-local (not a server surface).
- **Formula** — `freq / (1 + age_days)` (the `frecencyScore`, frecency.go:27-36,
  reused); the rank = fuzzy score ×(2 if prefix match) ×(1 + commandFrecency)
  (the `@`-picker's weighting, mention.go:173-177); **threshold 0**, **cap 10**
  (`maxPickerOptions`), **empty query = the full command list** (frecency-rank
  all, the upstream `/` empty-query behavior).
- **Touch policy** — a command-name touch (frequency+1, `lastOpen`=now) is
  recorded **when a command is executed via the slash menu** (the `enter`
  run-on-enter or a mouse click on the selected row), persisted to the
  `command_frecency` key (mirroring the file-path touch at acInsert,
  mention.go:193-207).

### 3.4 Empty-state text

The slash menu's empty-filter line renders **`No matching items`** (upstream
verbatim, autocomplete.tsx:742-746, `textMuted`), replacing `no match`
(prompt.go:178). The `@`-picker's `acView` (`no match`, prompt.go:227) is the
`@` spec's (yolo-old.5); the palette already renders `No results found`
(select.go:316, parity). This spec changes the slash menu only.

## 4. Deviations

### 4.1 Planned `DEVIATIONS.md` entries

Appended at implementation with the next-free entry numbers (the current tail is
**307**, so these land at 308+). Exact wording drafts (house style; each cites
principle 2 and a severity from the existing vocabulary):

1. **home placement re-spec** — `behavior/low`:
   > `0.10.0 slash menu: the above-box anchored dropdown replaces the 0.8.0
   > left-overlay placement (behavior/low, 2026-09-07): the 0.8.0 plan
   > (2026-09-07-0.8.0-start-screen-parity.md, Step 4 homeView) specified the
   > slash menu / `@`-picker as left-aligned overlay rows in the home frame
   > (home.go:515-522) and appended after the session transcript (view.go:47-56);
   > the parity dropdown re-anchors the slash menu ABOVE the input — home: above
   > the box's top edge, same left edge and width as the box (boxL/boxW,
   > home.go:486-487), the logo overlaid while open; session: above the prompt
   > line, overlaying the transcript tail (the upstream absolute-position
   > semantics, autocomplete.tsx:722-732). The new anchored placement supersedes
   > the 0.8.0 left-overlay placement; the bordered chrome (theme.border split
   > border + backgroundMenu fill + primary selection row) was never specified
   > by the 0.8.0 plan and lands here. Logged per principle 2.

2. **`tab` completion (supersedes 289)** — `behavior/low`:
   > `0.10.0 slash menu: tab completes the selected command while the menu is
   > open, superseding the agent cycle (behavior/low, 2026-09-07): deviation 289
   > (0.8.0) wired tab/shift+tab to cycle the pending agent on any route
   > INCLUDING while the slash menu is open (the ladder precedence —
   > handleAppKeys precedes handleMenuKey, keys.go:41-46); the 0.10.0 tab-hybrid
   > intercepts tab in the menu path first — with the slash menu open, tab
   > completes the selected command (replaces the whole input with /name and a
   > trailing space, the menu closes — the upstream /-completion,
   > autocomplete.tsx:456-462); tab with the menu closed still cycles the agent
   > (deviation 289's closed-menu behavior) and shift+tab is unchanged
   > (agent_cycle_reverse). No keymap rebind — the menu handler intercepts tab
   > before the base-group agent_cycle (the prompt.autocomplete.complete binding,
   > keymap.go:223, is the referent); the home hint's first segment is
   > context-aware (tab complete while the menu is open, tab agents when closed —
   > homeHintLine, home.go:355). Supersedes deviation 289's open-menu tab
   > behavior. Logged per principle 2.

3. **`esc` semantics note** — `behavior/info`:
   > `0.10.0 slash menu: esc clears the whole input with the menu open — parity,
   > kept (behavior/info, 2026-09-07): with the slash menu open, esc clears the
   > ENTIRE input (handleMenuKey's esc case → clearPrompt, keys.go:208-210), the
   > upstream /-mode hide() semantics (clears the whole input,
   > autocomplete.tsx:650-661) — PARITY, kept unchanged. This note pins the kept
   > behavior and distinguishes it from the `@`-picker's esc (strips the
   > @-trigger+query, keeps the prefix — the 259-class deviation, the `@` spec
   > yolo-old.5) and the palette's esc (closes the dialog, prompt untouched). No
   > behavior change; the entry records the deviation posture (parity for the
   > slash menu, the 259-class deviation for `@`). Logged per principle 2.

4. **empty-state text** — `render/cosmetic`:
   > `0.10.0 slash menu: the empty-state text is `No matching items`, upstream
   > verbatim (render/cosmetic, 2026-09-07): the slash menu's empty-filter line
   > renders `no match` (menuView, prompt.go:178); the parity text is
   > `No matching items` (upstream verbatim, autocomplete.tsx:742-746,
   > textMuted). The `@`-picker's acView (prompt.go:227) is the `@` spec's
   > (yolo-old.5); the palette already renders `No results found` (select.go:316,
   > parity). This entry is the slash menu only. The change re-baselines the
   > teatest substring legs that assert the empty text (§5). Logged per
   > principle 2.

### 4.2 Kept / superseded entries

- **Kept: run-on-enter (deviation 226 restated).** With the slash menu open,
  `enter` executes the selected command immediately (runCommand, keys.go:202-205
  → commands.go:237) — the input clears and the command runs. Upstream's
  enter = select-replaces-input (the input becomes `/name` and the menu stays for
  the user to add args) is **not** adopted; yolo's run-on-enter is LOCKED. This
  spec's `tab`-hybrid (§4.1 entry 2) makes `tab` the upstream-style completion
  (replaces the input, menu closes, no run), while `enter` keeps run-on-enter.
  Deviation 226's execute-on-enter contract is unchanged.
- **Superseded/extended:** deviation 289 (0.8.0 — `tab` cycles the agent while
  the slash menu is open) is superseded by §4.1 entry 2. Deviation 226 (S5.5
  `/`-picker upgrade) is extended by this spec (the fuzzy upgrade stays; the
  parity chrome, frecency, mouse, and `tab`-complete land here). The 259 parity
  gap class is **partially closed** for the slash menu: the chrome now matches
  upstream (the content/rows still differ — yolo's command set is a subset of
  upstream's 32, which stays). The 0.8.0-plan home bottom-left overlay placement
  is superseded by §4.1 entry 1. Deviations 212 (palette Suggested bucket) and
  225 (`@` FFF) are the sibling specs' concerns (yolo-old.6 / yolo-old.5).

## 5. Re-baselines (tests / pins / mock)

Per the yolo inventory's re-baseline inventory; one line each (file:line):

- `TestMenuViewWraps` (overflow_test.go:49) — pins the `menuView` wrap + row
  geometry; re-baseline if the border/background changes the row shape (the
  dropdown now renders a bordered box, so the wrap assertion is re-anchored to
  the new chrome).
- `TestPromptMenuFilter` / `TestPromptMenuFuzzy` / `TestPromptMenuKeys`
  (prompt_test.go:78-101, 103-122, 124-158) — assert `a.prompt.sel` + the command
  list (not SGR); likely pass unchanged, but re-verify the `enter`-on-no-match
  pin (prompt_test.go:132) still holds with the new `esc`/`enter` handling.
- `TestPromptSendAndSlashMenu` (app_test.go:123) and the `@`-picker teatest legs
  (app_test.go:249) — substring-based; re-baseline **only** if the chrome changes
  their asserted item text (the item text is unchanged, so likely no re-baseline;
  verify).
- The **empty-state text** change (`no match` → `No matching items`) re-baselines
  any teatest substring leg that asserts `no match` on the slash menu (search the
  `_test.go` for `no match` before the gate — the `@`/palette legs are out of
  scope).
- **sha256 pins** — `TestParityFixturesPinned` (parity_fixture_test.go:34) pins
  the **upstream** reference fixtures (`testdata/parity/upstream/*.screen.json`,
  incl. `prompt-slash`) + `MANIFEST.json`/`catalog-pin.json`/`canned.json` — the
  frozen upstream reference, **NOT re-baselined** by yolo's menu work (it does not
  capture yolo's empty-state text). No yolo-side sha256 pin captures the slash
  menu's `no match` text (the teatest legs are substring-based, the unit tests
  assert `a.prompt.sel`). **No pin re-baseline** for the empty-state text.
- **Home mock** — the mock currently shows the **clean home** (menu closed).
  Add a **second mock file** `docs/superpowers/mockups/home-mock-200x50-slash-open.txt`,
  generated the same way (a hand-assembled `TestHomeMockRender`-style render,
  home_mock_test.go:178) showing the **dropdown open above the box** with a few
  command items and the selected row highlighted; the existing clean-home mock is
  unchanged.

## 6. Work slices

Numbered slices for the execution epic (the plan pins exact commit messages; the
spec states what each slice delivers). **Order constraints: `dropdown.go` first;
re-baselines last.**

- **S1 — `dropdown.go` primitive + tests.** Build the shared bordered-dropdown
  primitive (§3.1 chrome): the split `theme.border` border, the `backgroundMenu`
  fill, the row padding, the `primary`/`selectedForeground` selection row, the
  `text`/`textMuted` label/description, the width/height clamp (`min(10, count,
  space-above)`), and the row hit-test (cell → row index). Acceptance: a unit
  test renders the primitive at a fixed size with N rows and a selection index and
  pins the bordered box, the selection-row SGR (`primary` bg), and the height clamp
  (10 rows, and the space-above clamp). Delivers: the primitive the slash menu
  (S2) and the `@` spec build on.
- **S2 — slash menu rewiring onto the primitive.** Re-point `menuItems`/`menuView`
  to render through `dropdown.go` (the two-column name+description rows inside the
  bordered box); keep the fuzzy ranking (S4 adds frecency). Acceptance: the slash
  menu renders the bordered box (the unit test pins the open-menu box, the selected
  row, and the `No matching items` empty line) and the existing
  `TestPromptMenu*`/`TestPromptMenuKeys` legs pass (re-anchored where the chrome
  changes the geometry). Delivers: the slash menu on the shared primitive.
- **S3 — placement anchors (home + session).** Home: anchor the dropdown above
  the box's top edge at `boxL`/`boxW`, overlaying the logo while open (replace the
  left-overlay rows, home.go:515-522). Session: anchor it above the prompt line,
  overlaying the transcript tail (the viewport keeps full height; drop the
  viewport-height reduction, view.go:47-56 / viewSession 90-117). Acceptance: a
  home render test pins the dropdown above the box (the selected row at the box
  left edge, the logo overlaid); a session render test pins the dropdown above the
  prompt line. Delivers: the §3.1 anchors.
- **S4 — frecency store + scoring.** Add the `command_frecency` KV key + entry
  shape (mirroring `prompt_frecency`, app.go:580), the boot load + touch save
  (mirroring app.go:619-633), and the command-name scoring in `menuItems`
  (fuzzy ×2 prefix ×(1 + commandFrecency), threshold 0, cap 10, empty query =
  full list); record the touch on command execution via the slash menu.
  Acceptance: a unit test pins the ranking (a frequently-used command ranks above
  a rarely-used one with equal fuzzy score) and the touch (executing a command
  bumps its frequency + persists). Delivers: §3.3.
- **S5 — mouse (slash dropdown).** `tea.WithMouseCellMotion` on the Program
  options (main.go:768); a `case tea.MouseMsg` in `App.Update`/`updateMsg`
  (app.go:316-329) routed to the dropdown when the slash menu is open; row
  hit-testing (hover = move selection, click = select = run-on-enter). Acceptance:
  a teatest leg drives a mouse hover (the selection moves to the hovered row) and a
  mouse click (the selected command runs). Delivers: §3.1 mouse, scoped to the
  slash dropdown.
- **S6 — key behavior (`tab`/`esc`) + hint row.** Wire the `tab` interception
  (the menu path intercepts `tab` before `handleAppKeys`, no keymap rebind):
  `tab` with the menu open completes the selected command (`/name ` trailing
  space, menu closes); `tab` closed cycles the agent; `shift+tab` unchanged;
  `esc` with the menu open clears the entire input (kept). The home hint's first
  segment becomes context-aware (`tab complete` open / `tab agents` closed,
  home.go:355). Acceptance: a key test pins `tab`-open = input `/name ` + menu
  closed + command NOT run; `tab`-closed = agent cycled; `esc`-open = input
  cleared; the hint renders `tab complete` while the menu is open. Delivers: §3.2.
- **S7 — empty text + re-baselines + mock.** Change the empty-state text to
  `No matching items` (prompt.go:178); re-baseline the tests per §5 (the wrap
  test, the teatest legs, the empty-text legs); add the second mock file
  `home-mock-200x50-slash-open.txt` (the dropdown open above the box, the selected
  row highlighted); append the §4.1 `DEVIATIONS.md` entries (308+) and
  re-baseline `PROGRESS.md`. Acceptance: `go vet ./... && go test ./...` green; the
  new mock renders deterministically; the DEVIATIONS entries land with the next
  free numbers. Delivers: §3.4, §4, §5. **Re-baselines land last (after S1-S6).**

## 7. Open questions / out of scope

- **Out of scope (sibling specs):** the `@` mention picker (yolo-old.5 — reuses
  `dropdown.go`, its own frecency `no match` text, the 259-class `esc`
  strip-@-trigger behavior) and the `ctrl+p` palette (yolo-old.6 — reuses
  `dropdown.go`, the Suggested-bucket frecency, deviation 212).
- **Shared-foundation epic structure** — the wayfinder map defers the
  shared-foundation epic (the `dropdown.go` + frecency-store + mouse plumbing the
  three specs share) to a sharpened decision **after the three specs** (slash,
  `@`, palette) land; this spec seeds only the slash slice of that foundation.
- **Command-set subset** — yolo's command list is a subset of upstream's 32
  (deviation 226); the menu ranks whatever `mergedCommands` (commands.go:171)
  returns. Adding upstream's full command set is a separate concern.
- **`shell`-mode interaction** — the `@`-picker is only active in `shell` mode
  (keys.go:47); the slash menu is mode-independent. No change to the mode gate.

## 8. Zero telemetry

This spec adds no telemetry: the command-name frecency is TUI-local (the same
theme KV as `prompt_frecency`, app.go:580), the dropdown/mouse/key work is
pure-client (internal/tui imports `internal/protocol` + `internal/tui/*` only,
principle 4), and no `OTEL_*`/usage surface is touched (principle 1).
