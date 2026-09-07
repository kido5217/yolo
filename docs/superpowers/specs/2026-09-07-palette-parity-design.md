# 0.10.0 full parity — `ctrl+p` command palette

- **Date:** 2026-09-07
- **Status:** `draft (pending user approval)`
- **Beads:** `yolo-old.6` (the wayfinder map epic's 0.10.0 output — the `ctrl+p` palette leg of the three menu-parity specs; the last of the three).
- **Upstream referent:** opencode v1.18.18 `CommandPaletteDialog` (`command-palette.tsx`) rendered through `DialogSelect` (`ui/dialog-select.tsx`) on the `dialog.tsx` overlay (fullscreen dim + centered `backgroundPanel` panel).

**Destination (restated).** Full parity of the `ctrl+p` command palette — visual + behavioral. The palette stays a borderless centered `backgroundPanel` **modal** (width 60, top `h/4` — the upstream `dialog.tsx` shape, **NOT** the inline `dropdown.go` primitive) on a **fullscreen dim overlay**; it groups into category buckets (accent headers) plus the empty-filter-only **"Suggested"** seed bucket; it filters `fuzzysort`-parity (title×2 + category, no threshold, no limit — the scroll window); it keeps the arrow / enter / page / home / end key set with **run-on-enter**; `esc`/`ctrl+c` close (prompt untouched); and it renders `No results found` (already parity). The palette is **command-only**: its items are the merged command list (no agent-as-item, no file candidates — those are the `@`-picker's domain or have no yolo referent).

**Sibling relationship.** This is the third of the three 0.10.0 menu-parity specs (`yolo-old.4` slash, `yolo-old.5` mention, `yolo-old.6` this) and the last to land; it mirrors their house structure and stays consistent with every locked decision they encode, cross-referencing the shared matters instead of re-deriving them. The palette is a **modal**: it keeps its own borderless `backgroundPanel` treatment (`view.go:212`) and is **NOT** a `dropdown.go` consumer (that primitive is for the inline slash/`@` dropdowns; locked decision 1). It shares the selection-row style (`primary` bg + `selectedForeground` fg, `select.go:501-511`) and the theme tokens with the dropdowns. Two shared-fact corrections / clarifications this spec owns:

- **The mouse option is NOT on.** The sibling mention spec §1/§3.8 states "the app-level `tea.WithMouseCellMotion()` is already on (app.go:63 — the slash S1 prerequisite)." That is inaccurate: `cmd/yolo/main.go:768` is `tea.NewProgram(app)` with **no** `tea.WithMouse*` option, and `app.go:63` is the `routeHome`/`routeSession` const block. The option is a shared prerequisite that the **slash epic's S5 (the mouse slice)** lands first (map order: slash → mention → palette); the palette's mouse slice routes `MouseMsg` to the palette only after that. (See §6.)
- **The palette does not rank by frecency.** The upstream palette has no frecency (the filter is `fuzzysort` over `[title, category]`); the `command_frecency` KV key is the **slash epic's** (touch-on-run). The palette does **not** consume it — there is no shared-frecency dependency between the palette and the slash epic.

## 1. Current state (file:line)

- **Modal frame.** `viewModal` (`internal/tui/view.go:174-243`) is the port of `dialog.tsx:28-67`: panel top `max(h/4, chromeMin)` (`view.go:188`, the upstream `paddingTop=height/4`), centered via `lead=(w-panelW)/2` (`view.go:217`), `panelW` clamped to `w-2` (`view.go:184-185`, the upstream `maxWidth=width-2`), and `bg := theme.BackgroundPanel().Width(panelW)` (`view.go:212`) — the borderless `backgroundPanel` fill. The backdrop is **not dimmed**: the route chrome (home logo/box, session chrome) is clamped to `panelTop` rows above the panel and the lines below are blank (`view.go:196-230`). `view.go:171` reads *"deviation 166 — the upstream rgba(0,0,0,0.15) dim has no SGR equivalent."* (The comment's `0.15` is wrong — upstream is `150/255 ≈ 0.59`; §2.2.)
- **Palette open.** `openPaletteDialog` (`internal/tui/commands.go:196-201`) → `selectNew("Commands", "Filter commands", paletteOptions(a), nil, paletteSelectPick, nil)` → `pushModal(dlgPalette, dlgMedium)` (`commands.go:199`). `dlgMedium` = **60** columns (`internal/tui/dialog.go:61-63`), matching upstream's medium width.
- **Palette pick.** `paletteSelectPick` (`commands.go:206-211`): `closeTopModal()` + `runCommand(value)` — **run-on-enter** (already the ported behavior).
- **Palette options.** `paletteOptions` (`commands.go:215-230`) builds from `mergedCommands()` (`commands.go:171`) = the 4 local (`/sessions`, `/connect`, `/status`, `/themes`, `commands.go:138-153`) + the server catalog `a.store.Commands` (frozen at 5: `/help`, `/new`, `/model`, `/agents`, `/quit`). **9 commands total.** Each option sets `title` (name, `/`-trimmed), `description`, `footer` (the `commandBindings` referent subset → `keymap.Format`, blank when `"none"`), `value` (the command name). **No `category` is set** (flat list) and **no Suggested bucket** — deviation 212.
- **Select primitive.** `selectOption` (`internal/tui/select.go:26-36`) already carries `category`, `details`, `footer`, `value`, `bg`, `gutter` — the primitive supports grouping; the palette just doesn't populate `category` yet.
- **Filter.** `selectModel.filtered()` (`select.go`, the S2.5 filter) **already ports the upstream semantics**: `fuzzy.Find(filter, titles)` weighted ×2 + `fuzzy.Find(filter, cats)` weighted ×1, `score>0` gate (the "no explicit threshold"), `sort.SliceStable` by score, **no limit** (the scroll window). Today the palette's `cats` are empty (no category set), so the category axis is dormant.
- **Keys.** `selectModel.handleKey` (`select.go:183-232`): `up`/`down` wrap (`move`, `select.go:258-267`), `enter` → `submit` (`select.go:285-296`), `pgup`/`pgdown` → `pageScroll(±10)` (deviation 176 — ±10 pinned; the env-machined `getScrollAcceleration` not ported), `home`/`end` → `jump`, `tab`/`shift+tab` → `focusAction` (a no-op — the palette has no footer actions), default → the filter `textinput` + `syncFilter`. `esc` + `ctrl+c` close the modal via `handleDialogKey` (`dialog.go:447-452`, `escBinding` + `dlgCtrlC`, `keys.go:11`/`19`) → `closeTopModal` (the prompt is untouched). `ctrl+p` opens via the `command_list` binding (`keymap.go:61` default `ctrl+p`; → `openPaletteDialog`, `keys.go:138-139`).
- **Empty state.** `select.go:316` renders `"  No results found"` in `TextMuted` — **already parity** with upstream (`"No results found"`, `textMuted`, `dialog-select.tsx:602-608`).
- **Theme tokens.** `internal/tui/theme/styles.go`: `bg()` (`styles.go:32-38`) renders a **solid** background — lipgloss has no alpha compositing; `blended()` (`styles.go:51-71`) pre-blends a token over the `background` at an opacity (the `WarningSubtle` idiom). The token set is `border`/`backgroundMenu`/`primary`/`text`/`textMuted`/`backgroundPanel`/`backgroundElement`/`accent`/`selectedForeground`. **No `backgroundDim` / dim token exists** — the dim would be a computed blend, not a new token.
- **Mouse.** None: `main.go:768` `tea.NewProgram(app)` has no `tea.WithMouse*`; zero `MouseMsg` handling anywhere. (The shared prerequisite the slash epic's S5 (the mouse slice) lands.)
- **Deviation 166** (dim absent), **212** (Suggested bucket absent), **259** (content/layout subset), **176** (scroll ±10), **211** (select keys off the registry) — the palette's current posture; §3 re-maps them.

## 2. Design

### 2.1 Rendering / position / chrome

Port of the `dialog.tsx` overlay + `DialogSelect` list. The frame is already at parity for shape — **borderless `backgroundPanel` fill, width 60 (medium), `maxWidth=width−2`, top `h/4`, horizontally centered** (`view.go:184-188, 212, 217`). The only chrome gap is the dim (§2.2). The inner lines (select.go) are: the **title row** (`Commands`, `text` bold) with the `esc` hint right in `textMuted`; the **filter-input row** (placeholder `Filter commands`, `textMuted`, focus bg `backgroundPanel`, cursor `primary`, auto-focused on open); the **visible row window** (scroll window, `maxHeight = floor(h/2) − 6`); and the **footer row** (the `ctrl+p commands` keymap hint in `textMuted` — the palette has no footer actions). The backdrop behind the panel is the dim (§2.2).

```
  [ fullscreen dim: flat pre-blended dark backdrop — the rgba(0,0,0,150) port ]
  [ the route chrome behind the panel (home logo/box, session) is dimmed ]

                     ┌───────────────────────────────────────┐
                     │ Commands                           esc │  ← top h/4
                     ├───────────────────────────────────────┤
                     │ Filter commands                         │  ← filter input
                     │ Suggested                               │  ← accent, BOLD
                     │   ● model   list available models       │  ← primary row
                     │     new       start a new session       │
                     │ General                                 │  ← accent, BOLD
                     │   help        show the command help     │
                     │   status      view status               │
                     │ Provider                                │
                     │   connect     connect a provider        │
                     ├───────────────────────────────────────┤
                     │ ctrl+p commands                         │  ← footer
                     └───────────────────────────────────────┘
   (the panel is a borderless backgroundPanel FILL — w=60, centered, top h/4;
    the box edges above are the fill boundary, not a drawn lipgloss border)
   (rows: title text / selected row primary+selectedForeground bold /
    description textMuted / footer keybinding textMuted / category accent)
```

The `●` gutter marks the **current** option (upstream `current`); the selected (cursor) row is the full-row `primary` bg + `selectedForeground` fg, bold head (`select.go:501-511`) — the shared selection-row style. Rows are `paddingLeft/Right=3` (1 when current/gutter).

### 2.2 The dim (decision D — **port**, judgment call)

Upstream's dim is the `dialog.tsx:48` overlay background `RGBA.fromInts(0,0,0,150)` — black at **alpha 150/255 ≈ 0.59**, a fullscreen overlay (`zIndex` 3000) over the content, the panel on top. The content behind is dimmed (see-through).

yolo port: lipgloss renders **solid** backgrounds (`styles.go:32-38`), no alpha compositing. yolo already has the idiom to pre-blend a color over the `background` at an opacity — `blended()` (`styles.go:51-71`, the `WarningSubtle` mechanism). The dim is that mechanism with black at `150/255`: a new computed `DimBackdrop()` (a `Style` whose background is black pre-blended over `a.theme.Color("background")` at `150/255`), **NOT a new theme token** — the "no new tokens" grill call holds for the palette dim (it is a computed blend, consistent with `blended()`/`WarningSubtle()`).

`viewModal` (the shared `dialog.tsx` overlay port, `view.go:174`) paints the backdrop with `DimBackdrop()`: the full screen **behind the panel** — the route-chrome region above (the home logo/box, session chrome) and the blank region below — is filled with the dim color; the panel is drawn on top. This **dims the route chrome behind the modal** (parity — upstream's overlay dims the content behind it), replacing the current "bright chrome above + blank below."

**Approximation (the honest 259-class):** upstream shows the *dimmed content* behind the overlay (see-through); yolo's cell grid cannot darken already-rendered cells, so the port shows a **flat dimmed field** (content behind is replaced by the dim color, not darkened-through). The primary visual effect — the screen darkens so the panel stands out — is reproduced.

**Scope fact (for the epic/user):** the dim lives in the **shared** `viewModal`, so it dims **all** modals (model / agent / theme / session / permission / palette) — parity-correct, because upstream's `dialog.tsx` overlay is shared by every dialog. The palette spec carries this slice because **dev 166** (the dim gap) is the palette's parity deviation and the fix lives in the shared frame. **Supersedes dev 166** (re-logged): the "no SGR equivalent" rationale is retired — a pre-blended *solid* dark background **is** SGR-representable; the see-through property is the documented approximation. (Severity: `render/cosmetic`.) Alternative — keep dev 166 deferred (re-affirm) — is available if the user prefers the palette strictly palette-scoped; the recommendation is to port.

### 2.3 Buckets — "Suggested" (decision A) + bucket set (decision B)

**Suggested bucket (A — port the mechanism with a yolo-chosen seed).** Upstream's `list()` (`command-palette.tsx:64-76`): on the **empty filter** it returns `[...suggested entries (re-categorised "Suggested", value prefixed `suggested:`), ...all options]`; on a **non-empty** filter it returns just `all options` — so "Suggested" is a top bucket shown on open that **collapses on any filter**. `suggested` is per-command (a bool or `() => bool`) evaluated at option build, seeded in `app.tsx`. The yolo port keeps this exactly:

- **Mechanism (parity):** the palette's option list = `[Suggested entries (category "Suggested", the seeded commands)] + [plain entries (all 9 commands, their category buckets)]`. `selectModel.filtered()` gains one line: **when `m.filter != ""`, skip options whose `category == "Suggested"`** — so the Suggested bucket shows only on the empty filter and collapses on any filter (the suggested commands still match via their plain entries). The Suggested entries' `value` is the plain command name (run-on-enter-ready; upstream's `suggested:` value prefix is only for their internal dedup and is not needed — the selectModel renders all options, no value-dedup).
- **Yolo seed** (mapped from upstream's per-command seeds onto yolo's 9-command catalog; a client-side `isSuggested` in `commands.go`, alongside `commandBindings`):
  - `/model` — **always** (upstream `model.list` `true`, `app.tsx:632`).
  - `/new` — **when the current route is a session** (`a.route == routeSession`; upstream `session.new` when route is a session, `app.tsx:584`).
  - `/sessions` — **when ≥1 session exists** (`len(a.store.Sessions) > 0`; upstream `session.list` when `session.length > 0`, `app.tsx:574`).
  - `/connect` — **when a provider is not connected** (`!a.tipsConnected()`, `app.go:741-753`; upstream connect command when `!connected()`, `app.tsx:741`).
  - `console-org` — **not ported**: no yolo referent (console/org dropped from yolo's scope).
- **DEVIATIONS posture: supersedes dev 212** (re-logged). The earlier 212 rationale ("no frecency/frequently-used surface yet") was off-target — the Suggested bucket is **seed-driven, not frecency-driven**. (Severity: `behavior/low`.)

**Bucket set (B — client-side yolo-chosen categories).** The wire `protocol.Command` (`internal/protocol/agent.go:25-30`) is **Name/Description/Template/Hints — no `category` field**. Upstream groups by `command.category` (a registry/wire field); yolo has no wire category, and adding one is a core-server change beyond the TUI parity bar (the same frozen-wire reasoning the mention spec applies to `fs.find`). So yolo assigns **client-side categories** in `commands.go` (a small mapping beside `commandBindings`) and sets `selectOption.category` (the `select.go:31` field the primitive already renders as accent BOLD headers):

- **General:** `/help`, `/status`, `/themes`, `/quit`
- **Session:** `/new`, `/sessions`
- **Model:** `/model`
- **Provider:** `/connect`
- **Agent:** `/agents`
- **Suggested:** the seeded entries (decision A; overlap — the suggested commands also appear in their normal bucket on the empty filter, exactly as upstream's `list()` concatenates).

Upstream groups yolo has **no content for** (the `@`-picker's `files`, `modes`, or any group with no yolo referent) simply do not exist in yolo's palette — yolo's palette is **command-only** (file candidates belong to the `@`-picker, not the palette; yolo has an `/agents` command, so an **Agent** bucket is present). **DEVIATIONS posture: 259-class** (content/layout divergence — the bucket set is yolo-chosen, not upstream's wire-derived categories; the group names/assignments are a yolo-chosen port). (Severity: `behavior/low`.)

### 2.4 Item rows

Mirror the sibling row spec: the option's `title` (the command name, `/`-trimmed) in `text`, the `description` in `textMuted`, the `footer` (the registry keybinding, `textMuted`) — the selected (cursor) row is the full-row `primary` bg + `selectedForeground` fg with a bold head (`select.go:501-511`, the shared style). The `●` gutter marks the current option (`select.go:483-489`). Truncation at the panel width (upstream truncates palette titles at width 61).

### 2.5 Filter / ranking (decision C)

Already at parity (`filtered()`, select.go): fuzzy over **title×2 + category×1** (the upstream `scoreFn: title.score*2 + category.score`), `score>0` gate (the "no explicit threshold" — `fuzzy` returns positive scores only), `sort.SliceStable` by score, **no limit** (the scroll window, `maxHeight=floor(h/2)−6` — the palette scrolls; it does **not** cap at 10 like the slash/`@` pickers). Populating the categories (§2.3) activates the (currently dormant) category axis. `sel` resets on query change (parity; `submit` already clamps `sel`, `select.go:292-293`). **No frecency**: the palette does not rank by frecency (upstream parity) and does **not** consume the slash epic's `command_frecency` (the shared-frecency fact, stated at the top). (Severity: `behavior/info` — see §3.1, 321.)

### 2.6 Selection model (up/down/wrap)

Unchanged (parity): `up`/`down` move the selection with **wraparound** (`select.go:258-267`, the upstream `move`); `home`/`end` jump to top/bottom (`select.go:269-282`); `pgup`/`pgdown` shift the window by ±10 (`select.go:244`, deviation 176 — the env-machined acceleration is not ported). `scrollToSelection` on move (the window re-anchors to the selection when it changes).

### 2.7 Mouse (decision 4 — locked)

The shared cell-based model: `tea.WithMouseCellMotion()` on the Program options (the slash epic's S5 (the mouse slice) prerequisite — **not on yet**, `main.go:768`; §6) plus palette-scoped row hit-testing on `MouseMsg`, **active only while the palette is the top modal** (the `dlgPalette` path): hover (`MouseMsg` motion over a visible row) moves the selection to that row; click (mouse-down/up on a row) is the **enter action** — run the selected command (`paletteSelectPick`). Same model as the sibling specs' §3.8. No new token.

### 2.8 Key behavior (decision E)

| Key | Upstream (`dialog-select`/`dialog.tsx`) | yolo parity (this spec) |
|---|---|---|
| `ctrl+p` | `command_list` opens (any route) | open the palette — `command_list` binding (`keymap.go:61`), **kept** |
| typing | filters (fuzzy) | filter the options (title+category), **kept** |
| `up` / `down` | `dialog.select.prev` / `.next` (wrap) | move selection, wraparound, **kept** |
| `ctrl+p` / `ctrl+n` (as prev/next alternates) | bound as `prev`/`next` alternates | **not bound** (deviation 211 posture — the select keys stay off the registry; `ctrl+p` is the open key, not prev; the arrows are the prev/next) |
| `enter` / `return` | `submit` → run the selected command | **run-on-enter — KEPT**: `submit` → `closeTopModal` + `runCommand` (`commands.go:206-211`) |
| `esc` / `ctrl+c` | close the dialog (clear selection) | close the palette, **prompt untouched** — `handleDialogKey` → `closeTopModal` (`dialog.go:447-452`), **kept** |
| `pgup` / `pgdown` | `page_up`/`page_down` (±10) | ±10 window scroll (deviation 176 — ±10 pinned; the env-machined acceleration not ported), **kept** |
| `home` / `end` | `home`/`end` jump | jump to top/bottom, **kept** |
| `tab` / `shift+tab` | move footer-action focus (palette: none) | no-op (the palette has no footer actions), **kept** |

The `esc`/`ctrl+c` and `enter` rows are the locked rows (decisions 2 and 5) — agreed with the grill.

### 2.9 Selection behavior per kind (decision F)

The palette is **command-only**: every item runs via `runCommand` (`commands.go:233`). So per-kind semantics collapse to one: **enter or click = close the palette + run the selected command** — the current `paletteSelectPick` (`commands.go:206-211`), kept. Upstream's per-kind semantics (agent selects / file inserts) do not apply — upstream's palette is also command-only (`dispatchCommand`). Mapping: `/help`→help dialog, `/new`→create session (or the command endpoint), `/model`→model dialog, `/agents`→agent dialog, `/quit`→quit dialog, `/sessions`→session-list dialog, `/connect`→provider dialog, `/status`→status dialog, `/themes`→theme-list dialog. (Severity: covered by deviation 226 — run-on-enter is the contract — no new entry.)

### 2.10 Empty-state text

`No results found` (`select.go:316`, `textMuted`) — **already parity** with upstream (`dialog-select.tsx:602-608`, `textMuted`). **No change** (locked decision 3). (The 2-space indent vs upstream's `paddingLeft/Right=4` is part of the existing row chrome, not a text deviation; the *text* is parity.)

## 3. Deviations

### 3.1 Planned entries (next-free after 317 — the mention spec's 312–317; **CONFIRM the tail at implementation**)

- **318. `Suggested` bucket ported (supersedes 212) — `behavior/low`.** The empty-filter-only "Suggested" seed bucket is ported (mechanism: the `list()` port — Suggested top bucket on the empty filter, collapses on any filter; yolo seed: `/model` always, `/new` session-route, `/sessions` sessions-exist, `/connect` !connected; console-org dropped — no referent). Dev 212's "frequently-used not ported" rationale is off-target (the bucket is seed-driven, not frecency-driven); 212 is superseded.
- **319. Palette bucket set is client-chosen — `behavior/low` (259-class).** The wire `protocol.Command` has no `category`; yolo assigns client-side categories (General / Session / Model / Provider / Agent + Suggested) in `commands.go`. Upstream's wire-derived categories are not portable (no wire referent; adding one is a core-server change beyond the TUI bar). yolo's palette is command-only (no `files`/`modes` groups — those are the `@`-picker's or have no referent).
- **320. Palette dim ported (supersedes 166) — `render/cosmetic`.** The fullscreen dim is ported as a flat pre-blended backdrop (black over `background` at alpha `150/255 ≈ 0.59` — upstream `dialog.tsx:48` `RGBA.fromInts(0,0,0,150)`). lipgloss has no alpha compositing (`styles.go:32-38`), so the see-through property is the documented approximation (a flat dark field, not dimmed-content). No new theme token — a computed blend, the `blended()` idiom (`styles.go:51-71`); the "no new tokens" grill holds. Supersedes dev 166's "no SGR equivalent" (a pre-blended solid bg is SGR-representable). The dim is a shared `viewModal` change (dims all modals — parity with upstream's shared `dialog.tsx` overlay).
- **321. Palette filter at parity + no frecency — `behavior/info`.** The filter already ports the upstream `fuzzysort` semantics (title×2 + category×1, `score>0` gate, no limit — the scroll window); populating the categories activates the category axis. The palette ranks by **no frecency** (upstream parity) and does **not** consume the slash epic's `command_frecency` (the shared-frecency fact; the palette and slash epics have no frecency dependency).

### 3.2 Affirmations (re-affirmed as-is, no new entry)

- **176** (scroll ±10 pinned) — the palette `pgup`/`pgdown` page scroll is ±10; the upstream env-machined `getScrollAcceleration` is not ported. The palette is a new consumer of the same `selectModel`.
- **211** (select keys off the registry) — the palette's select keys (up/down/enter/pgup/pgdown/home/end) are matched directly by `selectModel.handleKey`, not the keymap registry; the `ctrl+p`/`ctrl+n` prev/next alternates are not bound (`ctrl+p` is the open key).
- **226** (run-on-enter) — the palette's execute-on-enter is part of 226's contract; no new entry.
- **259** (content/layout divergence) — yolo's 9-command catalog is a subset/rename of upstream's command set; the Suggested + bucket-set entries (318/319) are the palette's specific 259-class facets.

## 4. Re-baselines

- **Tests (palette legs, from the yolo inventory).** `TestPaletteOptions` (`palette_test.go:42`), `TestPaletteOpen` (`:108`), `TestPaletteDispatch` (`:133`), `TestPaletteSelectPick` (`:162`), `TestPaletteNav` (`:192`), `TestPaletteEsc` (`:216`), `TestTUICommandPalette` (`:250`). Shared `selectModel` legs the palette exercises: `TestSelectViewLayout` (`select_test.go:94`), `TestSelectNavigationWrap`, `TestSelectEnterAndJump`, `TestSelectFuzzyFilter`, `TestSelectFuzzyWeighting`, `TestSelectCategoriesRender`, `TestSelectDetailsAndFooter`, `TestSelectScrollWindowCountsRows`, `TestSelectActions`, `TestSelectFooterHints`, `TestSelectScrollAcceleration`.
  - Re-baseline targets: `TestPaletteOptions` (options now carry `category` + the `isSuggested` seed logic); the selectModel category/filter legs (the category axis is now active); the teatest `TestTUICommandPalette` (the dim + Suggested bucket may shift the SGR — verify; teatest is substring-based).
  - **New** legs: the Suggested bucket (empty filter → Suggested top group; non-empty → collapses to the plain filtered list); the client-side category set (the 5 buckets + Suggested); the dim backdrop (a modal-open render pins the backdrop carrying the pre-blended dim color, the panel at `h/4` centered, the panel bg `backgroundPanel`); the mouse legs (hover moves the selection; click runs the command).
- **Pins.** The 4 sha256 pins — `TestLogoBlockPinned`, `TestTipsPinned`, `TestDescPinned`, `TestParityFixturesPinned` — are **not re-baselined** (same no-re-baseline posture as the siblings). `TestParityFixturesPinned` captures the **frozen upstream** reference (incl. `fixtures/palette.screen.json`), not yolo's output — the yolo palette changes do not touch it. Verify all four green.
- **Mock (decision H — a 4th mock).** A **4th mock file** `home-mock-200x50-palette-open.txt` — the palette open from home (the fullscreen dim backdrop + the centered w=60 panel at `h/4`, the "Suggested" bucket + the category groups, the selected row `primary`+`selectedForeground`, the filter input, the `ctrl+p commands` footer). Generated the same way as the siblings (the `TestHomeMockRender` idiom, `home_mock_test.go:178` — a hand-assembled 200×50 SGR frame from the resolved tokens). Rationale: the palette is a **fullscreen modal** (distinct from the inline dropdowns' above-box placement), so a dedicated mock shows the dim + centered panel + Suggested bucket. The existing clean-home mock (`home-mock-200x50.txt`) is **unchanged** (no palette open there; the dim only renders while a modal is open).

## 5. Work slices (ordered)

Ordered so each slice lands with the gate green and a commit; the DEVIATIONS entries + PROGRESS.md facts land last (S6).

- **S1 — dim backdrop + chrome parity (decision D).** Port the dim into the shared `viewModal` (`view.go:174`): a computed `DimBackdrop()` (black pre-blended over `background` at `150/255`, the `blended()` idiom — **no new token**); paint the backdrop behind the panel (the route-chrome region + the below region) with it; the panel (w=60, top `h/4`, centered, `backgroundPanel`) unchanged. A unit render pins the dimmed backdrop + panel geometry. **Scope:** shared-modal (dims all modals — parity with the shared `dialog.tsx` overlay).
- **S2 — buckets + Suggested (decisions A + B).** Client-side `category` mapping in `commands.go` (General / Session / Model / Provider / Agent) set on `selectOption.category`; the `isSuggested` seed (`/model` always, `/new` session-route, `/sessions` sessions-exist, `/connect` !connected) + the Suggested option entries (category `"Suggested"`); the `filtered()` line that skips `category=="Suggested"` when `m.filter != ""` (the empty-filter-only collapse). Unit legs: the empty-filter render (Suggested top group + the category groups), the non-empty render (Suggested collapsed), the seed conditions.
- **S3 — filter parity + no-frecency (decision C).** Verify/populate the category axis in `filtered()` (title×2 + category×1 already ported); the `sel`-reset-on-query-change; the scroll window (no cap-10). State the no-frecency fact (no `command_frecency` consumption). Unit legs: the filter (title+category match, the weighting, the scroll window), the sel-reset. **No dependency on the slash epic** (the palette does not use `command_frecency`).
- **S4 — mouse (decision 4).** The `dlgPalette`-path `MouseMsg` routing (the app-level `tea.WithMouseCellMotion()` is the slash epic's S5 (the mouse slice) prerequisite — **not on yet**, `main.go:768`; if the slash epic hasn't landed it, this slice adds the option first): row hit-testing over the visible window; hover moves the selection; click = the enter action (`paletteSelectPick`). A teatest leg drives a hover (selection moves) and a click (the command runs).
- **S5 — keys + empty text (decisions E, 2, 3, 5).** Verify/port the key table (§2.8): `ctrl+p` open, `up`/`down` wrap, `enter` submit, `esc`/`ctrl+c` close (prompt untouched), `pgup`/`pgdown` ±10, `home`/`end` jump, typing filters, `tab`/`shift+tab` no-op; the `sel`-reset; the empty text (already parity — verify `select.go:316`, no change). Key-table legs.
- **S6 — re-baselines + mock + DEVIATIONS (last).** All §4 re-baselines (`TestPaletteOptions`, the selectModel category/filter legs, the teatest legs, the new Suggested/dim/mouse legs); the 4th mock (`home-mock-200x50-palette-open.txt`); the DEVIATIONS entries **318–321** + the §3.2 affirmations; the `docs/superpowers/PROGRESS.md` facts. **Acceptance:** `go vet ./... && go test ./...` green, `gofmt -l .` prints nothing; the DEVIATIONS numbers land at the next-free (318+, confirmed against the tail at implementation); the mock renders deterministically.

**Order constraint:** S1 (dim) first — it is the `viewModal` change the other slices render on. S2/S3 (buckets/filter) are independent of S1. S4 (mouse) depends on the shared `tea.WithMouseCellMotion()` option (slash S5 (the mouse slice); or added here if the slash epic hasn't landed). S6 last.

## 6. Open questions / out of scope

- **The two sibling specs** (slash `yolo-old.4`, mention `yolo-old.5`) are done — they are **inputs**, not open work.
- **The epic-structure decision** (the map: "after the three specs"). This is the **last** of the three specs to land — after it lands, the map's Not-yet-specified item (which epic structure to use for the three menu-parity epics) can be resolved.
- **The mouse option's home (a sibling-spec discrepancy to fix up).** The mention spec §1/§3.8 claims the app-level `tea.WithMouseCellMotion()` is "already on (app.go:63)." It is **not** — `main.go:768` is `tea.NewProgram(app)` with no mouse option, and `app.go:63` is the route const block. The option is a **shared prerequisite** the slash epic's S5 (the mouse slice) lands first (map order). The palette's S4 depends on it (or lands it). *Reported, not edited here — the root dispatches the fixup.* (This also affects the mention epic's S4 the same way.)
- **Out of scope:** the `@`-picker (the mention spec, `yolo-old.5`), the inline slash dropdown (the slash spec, `yolo-old.4`), the core-server command catalog (`/command`, the frozen 5 — the palette consumes it, does not change it), the `command_frecency` key (the slash epic's), the `dropdown.go` primitive (the inline menus' — the palette does not use it), the upstream `console`/`org` surfaces (dropped from yolo — no Suggested seed referent).

## 7. Zero telemetry

No telemetry is added: the palette reads only the local command catalog + the local store (sessions/providers) + the theme KV (none here — the Suggested seed is state-derived, not a persisted frecency). It sends nothing remote. The dim, buckets, Suggested, filter, mouse, and key changes are all local TUI behavior. The zero-telemetry statement (spec `2026-08-17-yolo-go-port-design.md` §1) is unchanged.
