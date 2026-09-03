# S7 — session completion (slice bead `yolo-oae.8`)

Complete the session route: the todo sidebar (latest `todowrite` part →
status-glyph list, keymap toggle), the full-message dialog, and the
session footer detail restyle.

**State: fully detailed** — the 5-step TDD detail for all 4 tasks is in
the `## S7 detail` section below (Slice Detail Protocol rule 2);
execution may start at task S7.1.

## Binding task table

Pointer only (FROZEN — Slice Detail Protocol rule 1): `plan.md` →
`## Task inventory` → `### S7 — session completion (slice bead yolo-oae.8)`.
Bead titles, scope, and pinned commit messages live there and may not be
changed by a detail pass; any required change = STOP + explicit user
approval + re-record in plan.md.

## Dep gate

None.

## Exact upstream sources (the detail pass reads these AT DETAIL TIME)

`/tmp/opencode-upstream` @ v1.18.18:

- `packages/tui/src/feature-plugins/sidebar/todo.tsx` +
  `packages/tui/src/component/todo-item.tsx` — the todo sidebar: latest
  `todowrite` part → status-glyph list (S7.1) + keymap toggle + layout
  (S7.2).
- `packages/tui/src/routes/session/sidebar.tsx` — the session sidebar
  layout.
- `packages/tui/src/routes/session/dialog-message.tsx` — the
  full-message view (S7.3).
- `packages/tui/src/routes/session/footer.tsx` — the session footer detail:
  model/agent/tokens/cost/spinner/connection, status dots 70–84 (S7.4 —
  S0.10 already themed the conn segment tokens; S7.4 refines the whole
  session footer).
- `packages/tui/src/routes/session/subagent-footer.tsx` — read for context
  (no yolo subagent surface; note it as out of scope unless the detail
  pass finds a contract-backed analog).

## yolo anchors

- `internal/tui/session.go` — the session route surface (sidebar + message
  view).
- `internal/tui/footer.go` — the session footer (S7.4).
- `internal/tui/dialog.go` — the dialog surface for the full-message view
  (S7.3).
- the S4 keymap registry — the S7.2 sidebar toggle binding.
- `internal/protocol/` — the `todowrite` part DTO — verify the wire shape
  carries enough for the status list (if not: spec §10 — no wire changes in
  this epic; the detail pass logs a deviation instead).

## Detail pass (protocol)

One writing-plans pass, one subagent, `thinking=high`, dispatched by the
root session strictly sequentially (root principle 7) — per the Slice
Detail Protocol in plan.md: it fills this file (after this section) with the
full 5-step TDD detail for each task in the binding table (failing test
code, implementation code, gate, pinned commit), reading the named upstream
files at that moment. It commits as
`docs: TUI parity plan — detail S7 tasks` on its own bead
(`bd create "detail S7 plan tasks" --parent=yolo-oae.8 --json`).

## S7 detail

Detail pass 2026-09-03. Deviations tail at detail time = 244; S7 entries
start at 245. Breadcrumb note (DEVIATIONS.md entry 245, severity info):
the frozen S7 table (plan.md) names the task beads `yolo-oae.8.1`–`8.4`,
but the S7 detail bead consumed `yolo-oae.8.1` (created + claimed before
the detail pass; the S1 "detail-bead-last" precedent is impossible because
the detail pass precedes slice start, as in S2/dev 165, S3/dev 188, S4/dev
206, S5/dev 221, S6/dev 233). The 4 task beads therefore land in table
order at `yolo-oae.8.2`–`yolo-oae.8.5` (S7.1→.2, S7.2→.3, S7.3→.4,
S7.4→.5); the frozen titles and pinned commit messages are unchanged. No
code or wire impact.

### Detail-pass findings (read AT DETAIL TIME, 2026-09-03 — binding)

1. **Upstream todo sidebar** (`feature-plugins/sidebar/todo.tsx` +
   `component/todo-item.tsx` + `routes/session/sidebar.tsx` +
   `routes/session/index.tsx` @ v1.18.18):
   - `todo.tsx` (49 L, the `sidebar_content` slot plugin, order 400): the
     `open` signal defaults `true`; `list = api.state.session.todo(session_id)`
     (the latest `todowrite` part, read from the upstream todos storage —
     NO yolo wire referent, the yolo referent is the part walk); the show
     gate `show = list.length > 0 && list.some(item => item.status !==
     "completed")` (L12) — the block hides for an empty OR all-completed
     list (a `cancelled` item counts as open); the header row is clickable
     when `list.length > 2` (mouse-down toggles `open`); the `▼`/`▶`
     triangle renders ONLY when `length > 2` (fg `theme.text`); the "Todo"
     title is bold `theme.text`; the list renders when `list.length <= 2
     || open()`.
   - `todo-item.tsx` (32 L): one line per item = glyph run + content. The
     glyph run: `[✓] ` (completed) / `[•] ` (in_progress) / `[ ] `
     (every other status — pending AND cancelled), each 4 columns
     (bracket + glyph + bracket + trailing space). The fg of the glyph AND
     the content is `theme.warning` when `status === "in_progress"`, else
     `theme.textMuted`. The content word-wraps in the flex-grow box
     (continuation lines align under the first content column).
   - `sidebar.tsx` (the sidebar shell): width **42**, `backgroundPanel`,
     paddingTop 1, paddingBottom 1, paddingLeft 2, paddingRight 2, height
     100% over the content column (the scrollbox + the prompt box); the
     narrow-terminal case is an absolute overlay behind the
     `rgba(0,0,0,0.70)` scrim — the yolo frame composes lines with no
     absolute/surface (deviation 166's plain-blank convention), so the
     port is inline-only.
   - `index.tsx:257-279, 674-681, 1339-1361`: the `sidebar` KV
     `"auto"|"hide"` (default `"auto"`) + the `sidebarOpen` forced-open
     signal; `wide = width > 120`; `sidebarVisible = !session.parentID &&
     (sidebarOpen || (sidebar === "auto" && wide))` (the subagent gate);
     `contentWidth = width - (visible ? 42 : 0) - 4`; the
     `session.sidebar.toggle` command: visible → `sidebar="hide"` +
     `sidebarOpen=false`, invisible → `sidebar="auto"` +
     `sidebarOpen=true` (the mode KV-persisted).
2. **Upstream dialog-message** (`routes/session/dialog-message.tsx` @
   v1.18.18): the "Message Actions" dialog — the clicked user message's
   text + the **Revert / Copy / Fork** buttons, opened by a **mouse click**
   on the user message (the `onMouseUp` at `index.tsx:1274`). The upstream
   `clipboard.ts` is Node-only (`execFile`/osascript/wl-copy/xclip/OSC 52).
   The yolo referents are **absent**: the client has no revert/fork
   endpoints (wire freeze, spec §10), no clipboard contract, and the TUI
   program is built without mouse (`cmd/yolo/main.go:245`
   `tea.NewProgram(app)`). S7.3 therefore ports the **dialog surface** as
   the full-message view (the spec §6 redefinition — the frozen task title
   already carries "(full-message view)"), with a keyboard opener; the
   content renders the message meta + **all parts** (the upstream dialog
   shows the message text + the action buttons only — no referent).
3. **Upstream session footer** (`routes/session/footer.tsx` +
   `component/prompt/index.tsx` @ v1.18.18): **`footer.tsx` is ORPHANED
   upstream — no TUI file imports it** (the rendered session bottom bar is
   the prompt bar). The detail conventions port from there + the prompt
   bar anyway: `footer.tsx` L27-61 = the welcome blink ("Get started"
   text + "/connect" muted, a 10 s on / 5 s off cycle, only when
   disconnected), L69-84 = the status dots (the `•` LSP dot fg
   success/muted, the `⊙` MCP dot fg error/success). `prompt/index.tsx`
   L264-282 = the usage memo: the last assistant message with
   `tokens.output > 0`; the total = input+output+reasoning+cache.read+
   cache.write; `pct = round(total / model.limit.context * 100)` when the
   limit is known; the token display = `Locale.number(total)`
   (`locale.ts:46-54`: `>= 1e6 → (n/1e6).toFixed(1)+"M"`, `>= 1e3 →
   (n/1e3).toFixed(1)+"K"`, else the plain string); the cost display =
   `Intl.NumberFormat("en-US", {style:"currency", currency:"USD"})` **only
   when `cost > 0`**. `subagent-footer.tsx` = the subagent session bottom
   (title regex `@(\w+) subagent` + the same usage shape): **no yolo
   subagent surface (no `Session.ParentID` on the wire) → out of scope**
   (the brief's "note it as out of scope" case — no contract-backed analog
   found).
4. **yolo surface (verified at detail time):**
   - `keymap.go`: `sidebar_toggle` already exists in the registry (L85,
     default `<leader>b`, "Toggle sidebar"; the command map entry L285
     `"sidebar_toggle": "session.sidebar.toggle"`) but is in **no**
     context group and has **no** dispatch case — inert. `contextGroups`
     (L815-826): the `BaseMode` group ends with `"tips_toggle"` (the S6.3
     first-referent pattern, deviation 235); the `"session"` group
     (`messages_page_up/down`, `session_interrupt`, `session_rename`) is
     per-key matched in `handleSessionKey` via `keymap.Match` — and
     leader sequences are out of reach there (`keyMatchesSeq` rejects
     `<leader>`), so the `<leader>b` dispatch must go through the
     `BaseMode` path (`matchBase`/`matchLeaderContinuation` →
     `dispatchCommand`).
   - `keys.go`: `dispatchCommand` (the `tips_toggle` case = the flip +
     KV-persist pattern, no cmds — bubbletea re-renders after every
     Update); `matchLeaderContinuation`/`matchBase` walk the `BaseMode`
     group in order.
   - `session.go`: `sessKeyMap` (L33-47) — the yolo-surface keys
     (`Expand` alt+e, `Think` alt+t, both unbound by textinput's
     DefaultKeyMap, the T25 note) — the S7.3 opener precedent;
     `handleSessionKey` (L578-647) the case list; `sessionModel{vm,
     expanded, following, isDirty, content}`; `sm.sync(st, w, h, th, spin)`
     (L78-93) re-renders the transcript at `w` when dirty + sets the
     viewport dimensions.
   - `view.go`: `viewSession` (L88-115) budgets `vh = h - 1 - 1 - help - 1
     - 1 - menu - acMenu - overlays`; `sessionChrome(w, vh)` (L131-148) =
     the title line + `vm.View()` (exactly `vh` lines, the bubbles v2.2.1
     `viewport.View()` pads every line to the viewport width and height)
     + the divider + the wrapped help — the S7.2 composition site.
   - `footer.go`: `footerView()` (L43-95) = the muted
     `model · agent · ↑in ↓out · $cost` main + the conn segment
     (`◌ reconnecting` Error / `● live` Success / `○ off` Error) + the
     status segment (muted; `statusSeg` = the spinner frame + `busy` /
     `retry n: msg`); `footerFrames` the 5 locked braille frames;
     `spinMsg`/`spinTick` the 100 ms tick.
   - `dialog.go`: the `dialogKind` const block (L24-39, ends at
     `dlgPalette`); the `dialog` payload struct (L67-80); `pushModal`
     (L192); the `modalInner` cases (L385-435); `handleDialogKey` (L441+:
     the generic esc/ctrl+c modal close first — a static read-only dialog
     needs **no** key case); `dlgMedium/Large/XLarge` = 60/88/116 columns,
     clamped to `w-2` by `viewModal`.
   - `statusdlg.go`: the read-only modal idiom — `openStatusDialog` pushes
     `dialog{kind: dlgStatus}, dlgMedium, nil` and returns `nil`;
     `statusHeaderRow` (the bold title left, the muted "esc" right,
     space-between at the panel width, the `runeWidth` pad) +
     `statusView(w, h, th)`. The S7.3 model.
   - `app.go`: `loadTipsHidden` (L527-533, the `engine == nil` guard +
     `KV().Get(key, default).(bool)`), `kvTipsHiddenKey` (L129); `NewApp`
     calls `loadTipsHidden()` at its tail (L179).
   - `footer_test.go`: the pinned footer lines — `↑123 ↓45`, `$0.0002`,
     `$1.2346`, `$0.0000` (S7.4 re-baselines the cost pins, root
     principle 3); the `footerApp(st)` helper (80x24, the `Current` /
     `Model` deep-copied).
   - `locale.go`: `truncateMiddle` / `titlecase` only (the import is
     `unicode`) — **no** `number()` (S7.4 adds the port + the `strconv`
     import).
   - `wrap.go`: `runeWidth(s)` (L162) — the CJK-aware display width
     (wide rune = 2 columns); `wrapLine` wraps at display width.
   - `protocol` (the wire referents): `Todo{Content, Status, Priority}`
     (session.go:53-58); `Part{Type, Tool, State *ToolState{Input
     map[string]any, Metadata map[string]any, Title, Output, Error, ...}}`
     (part.go); `Message{Role, Time MessageTime{Created int64}, Agent,
     Model *MessageModel, Cost float64, Tokens *Tokens{Input, Output,
     Reasoning int64, Cache CacheTokens{Read, Write int64}}, Error
     *MessageError}` (message.go); `Model.Limit ModelLimit{Context, Output
     int}` + `Provider{ID, Models map[string]Model}` (provider.go);
     `Session{Title, Model *ModelRef{ProviderID, ID}, Agent, Cost float64,
     Tokens}` + `SessionStatus{Type, Attempt, Message}` (session.go).
     `internal/tool/todowrite.go`: the statuses
     `pending|in_progress|completed|cancelled` (cancelled ≠ completed —
     counts as open under the visibility gate), the priorities
     `high|medium|low` (default medium), `Meta:
     map[string]any{"todos": []protocol.Todo}` → `tool_exec.go`
     `Metadata: out.Meta`.
   - **Wire-shape verdict: SUFFICIENT** — the `todowrite` part carries the
     list at `State.Input["todos"]` (the JSON-decoded `[]any` of
     `map[string]any` after a wire round-trip) and
     `State.Metadata["todos"]` (in-memory `[]protocol.Todo`; `[]any` after
     a wire round-trip); both decode via a marshal/unmarshal round-trip
     into `[]protocol.Todo` (the `Input`-first / `Metadata`-fallback order
     covers the in-memory and the hydrated cases). No wire changes (spec
     §10) — the brief's "log a deviation instead" branch is **not**
     triggered.
   - bubbles v2.2.1 textinput `DefaultKeyMap` (verified in the module
     cache): `right/ctrl+f`, `left/ctrl+b`, `alt+right/ctrl+right/alt+f`,
     `alt+left/ctrl+left/alt+b`, `alt+backspace/ctrl+w/ctrl+backspace`,
     `alt+delete/alt+d/ctrl+delete`, `ctrl+k`, `ctrl+u`,
     `backspace/ctrl+h`, `delete/ctrl+d`, `home/ctrl+a`, `end/ctrl+e`,
     `ctrl+v`, `tab`, `down/ctrl+n`, `up/ctrl+p` (+ enter) — **`alt+m` is
     unbound** (the S7.3 opener).
   - Test idioms: `testApp()` → `*recApp` (home_test.go:29, no `size` set
     — the tests that need it set `a.size` explicitly), `refModel(p, m)`
     (home_test.go:24); `press(r)`/`pressAlt(r)`/`pressLeader()` (the
     default leader `ctrl+x`, keymap_test.go:178) / `ctrlCKey`;
     `stripANSI`; the leader-driven unit idiom
     `a.handleKey(pressLeader()); a.handleKey(press('a'))`
     (agent_test.go:246-250); the KV round-trip idiom (the
     `TestTipsTogglePersists` shape, tips_render_test.go:187-215:
     `themeApp(t)`, `e.Close()`, `theme.New` over the same dir via
     `kvPathOf(e)`, `NewApp` with the second engine); the teatest session
     flow (`testutil.BootWithDriver` + the `fake.Turn` script,
     `tm.Send(press('n'))` new session, `suiteType`, the merged `WaitFor`
     over the buffer-drain rule, `ctrlCKey` + `press('y')` quit).
5. **Anomalies (report §f):** (i) the orphaned
   `routes/session/footer.tsx` (finding 3) — the S7.4 referent set is
   footer.tsx's detail conventions + the prompt-bar usage memo, not a
   rendered upstream file; (ii) `dialog-message.tsx` is the mouse-opened
   Revert/Copy/Fork actions dialog — the yolo S7.3 redefines it as the
   keyboard-opened full-message view (the spec §6 wording + the frozen
   task title anticipate it); (iii) the auto-show at `width > 120` is
   ported — on a wide terminal the yolo session viewport shrinks 42
   columns **by default** (the upstream behavior; the `sidebar_toggle`
   key is the user's escape).

### Design decisions (binding)

**S7.1 (todo list, `sidebar.go` new file):** `latestTodos(st *store.State)
[]protocol.Todo` walks `st.Messages` in order and returns the **last**
`todowrite` tool part with a decodable list (nil when none) — the upstream
`session.todo()` referent over the yolo part walk (finding 1: the upstream
storage read has no yolo wire referent). `todosFromPart` decodes
`State.Input["todos"]` (first) else `State.Metadata["todos"]` via a
marshal/unmarshal round-trip into `[]protocol.Todo` (ok=false when absent
or undecodable); an undecodable part does NOT shadow an earlier decodable
one (the walk skips it). `todoBlockVisible` ports the `show` gate (len > 0
&& some status != "completed" — cancelled counts as open). The render is
**always expanded**: the `open` state is a constant `true` (the upstream
mouse-toggle has no yolo referent — no mouse — so the `▶` collapsed state
is unreachable; the `▼` glyph renders per the upstream rule, only when
`len > 2`). The plain layout (`todoSidebarRows`) + the styled render
(`todoSidebarLines`) share one source: the header line (bold
`theme.text`: `▼ Todo` when `len > 2`, else `Todo`) + one line per item —
the glyph run (`[✓] `/`[•] `/`[ ] `, 4 columns) + the content word-wrapped
at `w - 4` (`wrapLine`), the fg = `theme.warning` (in_progress) else
`theme.textMuted` on the whole line (the upstream glyph + content share
the fg); the continuation lines carry the 4-column indent. No App state,
no layout — S7.1 is data + render only.

**S7.2 (toggle + layout):** the state on `App`: `sidebarMode string`
(default `"auto"`) + `sidebarOpen bool` +
`const kvSidebarModeKey = "sidebar_mode"` (the theme KV, the S6.3
`tips_hidden` pattern, deviation 223's seam; `loadSidebarMode()` at the
`NewApp` tail with the `loadTipsHidden` nil-engine guard).
`sidebarVisible() = sidebarOpen || (sidebarMode == "auto" &&
size.Width > 120)` — the upstream `sidebarVisible` minus the `parentID`
gate (no `Session.ParentID` on the wire — finding 3's out-of-scope
subagent surface). `toggleSidebar()` ports the command flip (visible →
`"hide"` + `open=false`; invisible → `"auto"` + `open=true`) + the KV
persist (the S6.3 no-cmds pattern — bubbletea re-renders after every
Update). Dispatch: `"sidebar_toggle"` appended at the **END** of the
`contextGroups[BaseMode]` group (the S6.3 first-referent pattern; the
default `<leader>b` is a leader continuation, so
`matchLeaderContinuation`/`matchBase` pick it up — `handleSessionKey`
cannot reach leader sequences) + the `dispatchCommand` case guards on
`a.route == routeSession` (**no-op on the home route** — the upstream home
has no `session.sidebar.toggle` command; the which-key overlay lists the
binding on every route, the `session_list` class). Layout:
`sessionChrome` composes the sidebar as the right `sidebarWidth` (42)
columns of the **transcript viewport lines only** (the title / divider /
help / prompt / dialog / toast lines stay full width — the yolo frame
composes full-width chrome lines; the upstream full-content span including
the prompt box is not ported, and the narrow-terminal scrim overlay has no
yolo referent — inline only). `leftW = w - 42` (the upstream `-4`
contentWidth gap is dropped — the `backgroundPanel` fill is the
separator); `sidebarLines(vh)` builds exactly `vh` lines (each padded to
42 columns: the content line styled with its fg, then the pad tail
`panel.Render(spaces)` — the deletefailed.go:77 fg+bg composition idiom;
an outer-width pad over an inner-styled line would not paint the tail):
line 0 = the blank paddingTop line (fully panel-painted), then the
session-title block (bold `theme.text`, wrapped at `sidebarInnerW` = 38 —
the upstream `sidebar_title` minus the sessionID / workspace / share
spans, no yolo referents), a blank gap line, then the todo block
(`todoSidebarRows` at `w=38` — hidden by the `todoBlockVisible` gate; the
upstream content box's paddingRight 1 is absorbed into the panel padding),
then blank lines to the bottom padding. Degenerate widths (`w <= 42`) hide
the sidebar (the viewport needs at least 1 column).

**S7.3 (dialog-message, full message view):** a new `dlgMessage` dialog
kind (the `dialogKind` block) with the snapshot payload `message
*protocol.MessageWithParts` — captured at open time (the **last**
`store.Messages` entry; the upstream captures the clicked message — the
yolo referent is the last message, the only keyboard-reachable one; the
payload is a header copy + the parts slice header — mid-modal part updates
re-render on the next frame, accepted: the dialog shows the live part
state of the message captured at open). Opener: the **yolo-surface** key
`alt+m` on `sessKeyMap.MessageView` (the Expand/Think precedent,
deviation 211's scope — **not** a registry binding; `alt+m` is verified
unbound by the bubbles v2.2.1 textinput DefaultKeyMap; the upstream has no
key referent — the opener is mouse-only) matched in `handleSessionKey`
(added after the Think case); the key is consumed even with no message
(`openMessageDialog` no-ops on an empty store — `alt+m` is prompt-unbound,
nothing falls through). Render: `messageView` (the `statusView` model):
the header row (the bold "Message" left, the muted "esc" right, the
`statusHeaderRow` idiom), the meta line (muted: `role · agent` (when
non-empty) `· created 15:04:05 · ↑in ↓out` (when the `Tokens` pointer is
set) `· $cost` (when > 0 — the S7.4 Intl shape, rendered pre-S7.4 as
`$%.2f` already)), then one block per part in order: the muted header
(`Text` / `Reasoning` / `Tool: <name>` + ` — <title>` when non-empty) +
the content word-wrapped at `w`, clamped at `msgPartMaxLines = 12` lines
with the `… (N more lines)` muted hint AFTER the head (the `headPreview`
idiom); a tool error renders in the `Error` fg (`error: <msg>`). The
`dlgLarge` (88) panel size. No key case — the generic esc/ctrl+c modal
close (`handleDialogKey` L441) dismisses. The upstream Revert/Copy/Fork
buttons + the Node clipboard have no yolo referent (finding 2) — dropped.

**S7.4 (footer restyle):** port `number(n int64) string` into `locale.go`
(the upstream `Locale.number` verbatim: `>= 1e6 →
FormatFloat(n/1e6, 'f', 1)+"M"`, `>= 1e3 → FormatFloat(n/1e3, 'f', 1)+"K"`,
else `FormatInt` — `strconv` import added). `footerView` restyle (the
shared main render — both routes): the tokens segment =
`"↑" + number(tokens.Input) + " ↓" + number(tokens.Output)` (the yolo
arrow segment set stays — the frozen segment list — the numbers get the
upstream compact format); the context percentage: when
`contextPct() >= 0` append ` (pct%)` to the tokens segment —
`contextPct` = the session-route-only resolution of `store.Current.Model`
over `store.Providers` to a `Limit.Context > 0` (the lazy-catalog referent,
deviation 241: the pct shows only after the model/agent dialog has
populated the catalog; -1 → omitted), `pct = int64(math.Round(100 *
total / float64(limit)))`, `total = Input+Output+Reasoning+Cache.Read+
Cache.Write` (the upstream usage-memo total over the yolo **session
aggregate** — the message-level walk has no store referent); the cost
segment = `fmt.Sprintf("$%.2f", cost)` (the `Intl en-US USD` convention:
`$` + 2 decimals) **omitted when `cost == 0`** (the upstream
`cost > 0 ? money.format(cost) : undefined`). The model / agent /
spinner / conn segments: unchanged (S0.10's theming; the conn dots are the
upstream L69-84 dot idiom, already ported; the spinner = the locked 5
frames + the `busy`/`retry` labels). The home route inherits the
`number()` + the cost-omit (the `↑0 ↓0` stays — `number(0) = "0"`; the
`$0.0000` → omitted). The welcome blink + the permission/LSP/MCP dot
segments have no yolo referent (no LSP/MCP wire; the welcome nudge is the
home NO_MODELS tip, the S6.2 surface; the permission overlay is the S2.8
modal) — out of scope for the frozen 6-segment footer. The pinned
`footer_test.go` re-baseline (root principle 3 — the pins record the
current intended content, the intentional change re-baselines in the same
commit): the `$0.0002` → `$0.00`, the `$1.2346` → `$1.23`, the
`$0.0000` → segment omitted (2 pins), the subtest name "cost rounds to
four decimals" → "cost rounds to two decimals (the S7.4 Intl shape)"; the
`↑123 ↓45` pins are UNCHANGED (`number` is the identity below 1000); the
`resync_test` / `home_theme_test` `Contains` checks are unaffected.

### Task S7.1: Todo sidebar: latest `todowrite` part → status-glyph list + tests (bead `yolo-oae.8.1`, expected id `yolo-oae.8.2`)

**Files:** new `internal/tui/sidebar.go` (the todo-list data + render),
new `internal/tui/sidebar_test.go` (the unit table).

**Interfaces:** `func latestTodos(st *store.State) []protocol.Todo`,
`func todosFromPart(p protocol.Part) ([]protocol.Todo, bool)`, `func
todoBlockVisible(todos []protocol.Todo) bool`, `type todoRow struct{plain
string; inProgress bool; header bool}`, `func todoSidebarRows(todos
[]protocol.Todo, w int) []todoRow`, `func todoSidebarLines(todos
[]protocol.Todo, w int, th theme.Theme) []string`, `func
todoGlyphRun(status string) string`.

**Upstream parity notes:** the `show` gate + the glyph fg + the
`▼`-only-when-`len > 2` header rule port verbatim (todo.tsx:12,
todo-item.tsx); the list referent = the last decodable `todowrite` part's
`State.Input["todos"]`/`State.Metadata["todos"]` (the wire-shape verdict:
SUFFICIENT — no wire change, spec §10; the upstream storage read has no
yolo wire referent); the `open` state is a constant `true` (no mouse
referent — the collapse is unreachable, deviation 246).

**Step 1 — failing test** (`internal/tui/sidebar_test.go`):

```go
package tui

import (
	"strings"
	"testing"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/store"
)

// todowritePart builds the wire-shape todowrite part: the JSON-decoded
// State.Input (the []any of map[string]any).
func todowritePart(todos ...map[string]any) protocol.Part {
	in := make([]any, 0, len(todos))
	for _, td := range todos {
		in = append(in, td)
	}
	return protocol.Part{
		Type:  "tool",
		Tool:  "todowrite",
		State: &protocol.ToolState{Input: map[string]any{"todos": in}},
	}
}

// todoTodos is the 3-status fixture (completed / in_progress / pending).
func todoTodos(t *testing.T) []protocol.Todo {
	t.Helper()
	return []protocol.Todo{
		{Content: "design", Status: "completed", Priority: "high"},
		{Content: "implement the todo sidebar", Status: "in_progress", Priority: "medium"},
		{Content: "wire the toggle", Status: "pending", Priority: "low"},
	}
}

// TestTodosFromPartWireShape pins the decode over both the wire-decoded
// State.Input (the []any of map[string]any) and the in-memory
// State.Metadata (the []protocol.Todo); the Input wins when both are
// present; an undecodable value is !ok.
func TestTodosFromPartWireShape(t *testing.T) {
	wire := []any{
		map[string]any{"content": "a", "status": "in_progress", "priority": "high"},
		map[string]any{"content": "b", "status": "pending"},
	}
	got, ok := todosFromPart(protocol.Part{Type: "tool", Tool: "todowrite",
		State: &protocol.ToolState{Input: map[string]any{"todos": wire}}})
	if !ok || len(got) != 2 || got[0].Content != "a" || got[0].Status != "in_progress" || got[0].Priority != "high" {
		t.Fatalf("wire input decode = %+v (ok=%v), want the 2 todos", got, ok)
	}
	mem, ok := todosFromPart(protocol.Part{Type: "tool", Tool: "todowrite",
		State: &protocol.ToolState{Metadata: map[string]any{"todos": []protocol.Todo{{Content: "c", Status: "completed"}}}}})
	if !ok || len(mem) != 1 || mem[0].Content != "c" {
		t.Fatalf("metadata decode = %+v (ok=%v), want the 1 todo", mem, ok)
	}
	both, ok := todosFromPart(protocol.Part{Type: "tool", Tool: "todowrite",
		State: &protocol.ToolState{
			Input:    map[string]any{"todos": wire},
			Metadata: map[string]any{"todos": []protocol.Todo{{Content: "c"}}},
		}})
	if !ok || len(both) != 2 || both[1].Content != "b" {
		t.Fatalf("input-first decode = %+v (ok=%v), want the input's 2 todos", both, ok)
	}
	if _, ok := todosFromPart(protocol.Part{Type: "tool", Tool: "todowrite",
		State: &protocol.ToolState{Input: map[string]any{"todos": "not-a-list"}}}); ok {
		t.Fatal("an undecodable todos value must be !ok")
	}
}

// TestLatestTodos pins the last-wins referent: the LAST todowrite part
// with a decodable list wins; non-todowrite tool parts and undecodable
// parts are skipped (no shadow); an empty store yields nil.
func TestLatestTodos(t *testing.T) {
	if latestTodos(&store.State{}) != nil {
		t.Fatal("an empty store must be nil")
	}
	a := todowritePart(map[string]any{"content": "first", "status": "pending"})
	b := todowritePart(map[string]any{"content": "second", "status": "in_progress"})
	latest := latestTodos(&store.State{Messages: []protocol.MessageWithParts{
		{Info: protocol.Message{ID: "m1"}, Parts: []protocol.Part{
			{Type: "tool", Tool: "bash",
				State: &protocol.ToolState{Input: map[string]any{"todos": []any{map[string]any{"content": "not-me"}}}}},
			a,
		}},
		{Info: protocol.Message{ID: "m2"}, Parts: []protocol.Part{b}},
	}})
	if len(latest) != 1 || latest[0].Content != "second" {
		t.Fatalf("latest = %+v, want the last todowrite's todo", latest)
	}
	bad := protocol.Part{Type: "tool", Tool: "todowrite",
		State: &protocol.ToolState{Input: map[string]any{"todos": "nope"}}}
	latest2 := latestTodos(&store.State{Messages: []protocol.MessageWithParts{
		{Info: protocol.Message{ID: "m1"}, Parts: []protocol.Part{a}},
		{Info: protocol.Message{ID: "m2"}, Parts: []protocol.Part{bad}},
	}})
	if len(latest2) != 1 || latest2[0].Content != "first" {
		t.Fatalf("decode failure = %+v, want the earlier decodable part (no shadow)", latest2)
	}
}

// TestTodoBlockVisible pins the upstream show gate (todo.tsx:12): the
// block is hidden for an empty list and an all-completed list; a
// cancelled item counts as open (status != "completed").
func TestTodoBlockVisible(t *testing.T) {
	if todoBlockVisible(nil) {
		t.Fatal("a nil list must be hidden")
	}
	if todoBlockVisible([]protocol.Todo{{Status: "completed"}, {Status: "completed"}}) {
		t.Fatal("an all-completed list must be hidden")
	}
	if !todoBlockVisible([]protocol.Todo{{Status: "completed"}, {Status: "cancelled"}}) {
		t.Fatal("a cancelled item counts as open")
	}
	if !todoBlockVisible([]protocol.Todo{{Status: "in_progress"}}) {
		t.Fatal("an in_progress list must show")
	}
}

// TestTodoSidebarLines pins the header + the per-item glyph lines (the
// stripANSI unit idiom — the fake terminal has no TTY, the styles strip):
// the ▼ header only when len > 2, the [✓]/[•]/[ ] glyph runs, the
// word-wrap at w-4, the 4-column continuation indent.
func TestTodoSidebarLines(t *testing.T) {
	a := testApp()

	var short []string
	for _, l := range todoSidebarLines([]protocol.Todo{
		{Content: "done thing", Status: "completed"},
		{Content: "doing thing", Status: "in_progress"},
	}, 38, a.theme) {
		short = append(short, stripANSI(l))
	}
	if want := "Todo\n[✓] done thing\n[•] doing thing"; strings.Join(short, "\n") != want {
		t.Fatalf("lines = %q, want %q (the ≤2 items: the bare header)", strings.Join(short, "\n"), want)
	}

	var full []string
	for _, l := range todoSidebarLines(todoTodos(t), 38, a.theme) {
		full = append(full, stripANSI(l))
	}
	if full[0] != "▼ Todo" {
		t.Fatalf("header = %q, want '▼ Todo' (the len > 2 rule)", full[0])
	}
	if full[1] != "[✓] design" || full[2] != "[•] implement the todo sidebar" || full[3] != "[ ] wire the toggle" {
		t.Fatalf("item lines = %q, want the [✓]/[•]/[ ] glyph runs", full[1:])
	}

	// the word-wrap at w-4 (w=16 → contentW=12): "alpha beta gamma"
	// wraps; the continuation lines indent 4 columns.
	var wrapped []string
	for _, l := range todoSidebarLines([]protocol.Todo{
		{Content: "alpha beta gamma", Status: "pending"},
	}, 16, a.theme) {
		wrapped = append(wrapped, stripANSI(l))
	}
	if len(wrapped) != 3 {
		t.Fatalf("wrapped line count = %d, want 3 (header + 2 content lines)", len(wrapped))
	}
	if wrapped[1] != "[ ] alpha beta" || wrapped[2] != "    gamma" {
		t.Fatalf("wrap = %q / %q, want the 4-column continuation indent", wrapped[1], wrapped[2])
	}
}
```

**Step 2 — confirm FAIL:** `go test ./internal/tui/ -run
'TestTodosFromPartWireShape|TestLatestTodos|TestTodoBlockVisible|TestTodoSidebarLines'`
→ build fails (undefined `latestTodos`, `todosFromPart`,
`todoBlockVisible`, `todoSidebarLines`, `todowritePart`). That is the red.

**Step 3 — minimal implementation:**

- `internal/tui/sidebar.go` (new):

```go
// sidebar.go — the todo sidebar (S7.1): the latest todowrite part → the
// status-glyph list. The upstream feature-plugins/sidebar/todo.tsx +
// component/todo-item.tsx @ v1.18.18 port: the show gate (todo.tsx:12),
// the [✓]/[•]/[ ] glyph runs + the warning/muted fg (todo-item.tsx), the
// ▼-only-when-len>2 header. The block is always expanded (the upstream
// mouse collapse has no yolo referent — no mouse; deviation 246). The list
// referent is the last todowrite part's State.Input["todos"] (the
// wire-decoded shape) or State.Metadata["todos"] (the in-memory shape) —
// both decode via a marshal/unmarshal round-trip (the S7 detail finding:
// the wire shape is sufficient, no wire change).

package tui

import (
	"encoding/json"
	"strings"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/store"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// latestTodos returns the last todowrite part's todo list (the upstream
// session.todo() referent over the yolo part walk): the LAST part of type
// tool / tool id "todowrite" with a decodable todos list; nil when none
// (an undecodable part is skipped — it does not shadow an earlier one).
func latestTodos(st *store.State) []protocol.Todo {
	var latest []protocol.Todo
	found := false
	for _, m := range st.Messages {
		for _, p := range m.Parts {
			if p.Type != "tool" || p.Tool != "todowrite" || p.State == nil {
				continue
			}
			if todos, ok := todosFromPart(p); ok {
				latest, found = todos, true
			}
		}
	}
	if !found {
		return nil
	}
	return latest
}

// todosFromPart decodes the part's todos list: State.Input["todos"] (the
// wire-decoded []any of map[string]any) first, else
// State.Metadata["todos"] (the in-memory []protocol.Todo — []any after a
// wire round-trip). A marshal/unmarshal round-trip normalizes both shapes
// into []protocol.Todo.
func todosFromPart(p protocol.Part) ([]protocol.Todo, bool) {
	v, ok := p.State.Input["todos"]
	if !ok {
		v, ok = p.State.Metadata["todos"]
	}
	if !ok {
		return nil, false
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	var todos []protocol.Todo
	if json.Unmarshal(raw, &todos) != nil {
		return nil, false
	}
	return todos, true
}

// todoBlockVisible ports the upstream show gate (todo.tsx:12): the block
// is hidden for an empty list and an all-completed list (a cancelled item
// counts as open — status != "completed").
func todoBlockVisible(todos []protocol.Todo) bool {
	if len(todos) == 0 {
		return false
	}
	for _, td := range todos {
		if td.Status != "completed" {
			return true
		}
	}
	return false
}

// todoGlyphRun is the upstream todo-item glyph run (bracket + glyph +
// bracket + trailing space, 4 columns): [✓] completed, [•] in_progress,
// [ ] every other status (pending, cancelled).
func todoGlyphRun(status string) string {
	switch status {
	case "completed":
		return "[✓] "
	case "in_progress":
		return "[•] "
	default:
		return "[ ] "
	}
}

// todoRow is the plain (unstyled) todo-block line: the display text + the
// item status (the fg source) + the header flag. The styled render (the
// S7.1 pin) and the S7.2 panel padding both derive from this layout.
type todoRow struct {
	plain      string
	inProgress bool
	header     bool
}

// todoSidebarRows is the plain todo-block layout: the header row (the ▼
// collapse glyph only when len > 2 — the block is always expanded,
// deviation 246) + one line per item (the glyph run + the content
// word-wrapped at w-4, the 4 continuation columns).
func todoSidebarRows(todos []protocol.Todo, w int) []todoRow {
	contentW := w - 4 // the glyph run width
	if contentW < 1 {
		contentW = 1
	}
	header := "Todo"
	if len(todos) > 2 {
		header = "▼ " + header
	}
	rows := []todoRow{{plain: header, header: true}}
	for _, td := range todos {
		ip := td.Status == "in_progress"
		cols := strings.Split(wrapLine(td.Content, contentW), "\n")
		for i, col := range cols {
			if i == 0 {
				rows = append(rows, todoRow{plain: todoGlyphRun(td.Status) + col, inProgress: ip})
			} else {
				rows = append(rows, todoRow{plain: "    " + col, inProgress: ip})
			}
		}
	}
	return rows
}

// todoSidebarLines renders the todo block with the upstream fg (the glyph
// + content share the fg — warning for in_progress, else textMuted; the
// header is the bold text token).
func todoSidebarLines(todos []protocol.Todo, w int, th theme.Theme) []string {
	rows := todoSidebarRows(todos, w)
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		switch {
		case r.header:
			out = append(out, th.Text().Render(r.plain))
		case r.inProgress:
			out = append(out, th.Warning().Render(r.plain))
		default:
			out = append(out, th.TextMuted().Render(r.plain))
		}
	}
	return out
}
```

**Step 4 — gate:** `go vet ./... && go test ./...` green (incl.
`TestImportsDirection` — the new file imports only `protocol` +
`internal/tui/*` + stdlib) + `gofmt -l .` empty.

**Step 5 — commit** the pinned message `feat: todo sidebar - todowrite
part to status list`, then `bd close yolo-oae.8.2 --reason "S7.1 done: the
latest-todowrite-part referent (wire-shape verdict: the State.Input /
Metadata todos decode via the round-trip — no wire change, spec §10) + the
status-glyph list render (the show gate, the [✓]/[•]/[ ] glyph runs, the
warning/muted fg, the ▼-only-when-len>2 header, the always-expanded state,
the w-4 word-wrap + 4-column continuation indent); unit table + wire-shape
pins" --json`.

### Task S7.2: Todo sidebar: keymap toggle + layout + tests (bead `yolo-oae.8.2`, expected id `yolo-oae.8.3`)

**Files:** modify `internal/tui/sidebar.go` (the panel constants + the
`sidebarLines` composition), `internal/tui/app.go` (the `sidebarMode` /
`sidebarOpen` fields + `kvSidebarModeKey` + `loadSidebarMode` +
`sidebarVisible` + `toggleSidebar` + the `NewApp` tail call),
`internal/tui/keymap.go` (`"sidebar_toggle"` appended at the END of
`contextGroups[BaseMode]`), `internal/tui/keys.go` (the
`dispatchCommand` case), `internal/tui/view.go` (the `sessionChrome`
sidebar composition), new tests in `internal/tui/sidebar_test.go` (the
toggle / persist / layout unit table + the teatest presence leg).

**Interfaces:** `App.sidebarMode string`, `App.sidebarOpen bool`, `const
kvSidebarModeKey = "sidebar_mode"`, `const sidebarWidth = 42`, `const
sidebarInnerW = 38`, `type sideRow struct{plain string; st lipgloss.Style}`,
`func (a *App) loadSidebarMode()`, `func (a *App) sidebarVisible() bool`,
`func (a *App) toggleSidebar()`, `func (a *App) sidebarLines(vh int)
[]string`.

**Upstream parity notes:** the `sidebarVisible` semantics port verbatim
minus the `parentID` gate (no `Session.ParentID` on the wire — finding 3's
out-of-scope subagent surface, deviation 246); the `session.sidebar.toggle`
command flip ports verbatim (index.tsx:674-681) over the theme KV (the
S6.3 `tips_hidden` pattern — the upstream `sidebar` KV "auto"/"hide" is
persisted the same way, deviation 235's seam class); the shell (width 42,
`backgroundPanel`, paddingTop/Bottom 1, paddingLeft/Right 2) ports over
the line-composed frame (the inline-only decision — no scrim/absolute
surface, deviation 246); the `sidebar_toggle` binding gets its first
referent (registry L85, inert until now — the dispatch path is deviation
247).

**Step 1 — failing test** (`internal/tui/sidebar_test.go`, appended — the
file exists from S7.1; its import block EXTENDS to the full block below,
the S7.1 `strings` / `testing` / `protocol` / `store` imports stay, the
new ones are added):

```go
import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
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
```

```go
// TestSidebarToggle pins the ported session.sidebar.toggle flip (upstream
// index.tsx:674-681): visible → "hide" + open=false; invisible → "auto" +
// open=true; the >120 auto-show rule; the home-route no-op (the dispatch
// guard). Driven through the real <leader>b dispatch (the S4.2
// leader-continuation idiom).
func TestSidebarToggle(t *testing.T) {
	// a narrow terminal (100 < 120): auto alone does not show.
	a := testApp()
	a.route = routeSession
	a.size = tea.WindowSizeMsg{Width: 100, Height: 30}
	if a.sidebarVisible() {
		t.Fatal("narrow + auto must be hidden (the >120 auto-show rule)")
	}
	a.handleKey(pressLeader())
	a.handleKey(press('b'))
	if !a.sidebarOpen || a.sidebarMode != "auto" || !a.sidebarVisible() {
		t.Fatalf("after <leader>b: open=%v mode=%q visible=%v, want auto+open+visible",
			a.sidebarOpen, a.sidebarMode, a.sidebarVisible())
	}
	a.handleKey(pressLeader())
	a.handleKey(press('b'))
	if a.sidebarOpen || a.sidebarMode != "hide" || a.sidebarVisible() {
		t.Fatalf("after the 2nd toggle: open=%v mode=%q visible=%v, want hide+closed+hidden",
			a.sidebarOpen, a.sidebarMode, a.sidebarVisible())
	}

	// a wide terminal (140 > 120): auto alone shows; the toggle hides.
	b := testApp()
	b.route = routeSession
	b.size = tea.WindowSizeMsg{Width: 140, Height: 30}
	if !b.sidebarVisible() {
		t.Fatal("wide + auto must show (the upstream wide() rule)")
	}
	b.toggleSidebar()
	if b.sidebarMode != "hide" || b.sidebarOpen || b.sidebarVisible() {
		t.Fatalf("wide toggle: mode=%q open=%v visible=%v, want hide+closed+hidden",
			b.sidebarMode, b.sidebarOpen, b.sidebarVisible())
	}

	// the home route: the dispatch is a no-op (the route guard — the state
	// is untouched; sidebarVisible() itself is route-INDEPENDENT arithmetic,
	// so it is not asserted here — the sidebar simply does not render on
	// home, sessionChrome is session-route-only).
	c := testApp()
	c.size = tea.WindowSizeMsg{Width: 140, Height: 30}
	c.handleKey(pressLeader())
	c.handleKey(press('b'))
	if c.sidebarOpen || c.sidebarMode != "auto" {
		t.Fatalf("home toggle: open=%v mode=%q, want the untouched auto state (the no-op)",
			c.sidebarOpen, c.sidebarMode)
	}
}

// TestSidebarTogglePersists pins the KV round-trip (the S6.3 theme-KV
// seam): the toggle writes sidebar_mode over the engine KV; a second app
// over the SAME KV file (the TestTipsTogglePersists idiom) reloads the
// "hide" mode in NewApp's loadSidebarMode.
func TestSidebarTogglePersists(t *testing.T) {
	a, e := themeApp(t)
	a.route = routeSession
	a.size = tea.WindowSizeMsg{Width: 140, Height: 30}
	a.dispatchCommand("sidebar_toggle")
	if a.sidebarMode != "hide" {
		t.Fatalf("toggle must persist the hide mode (got %q)", a.sidebarMode)
	}
	_ = e.Close() // drains the writer + final flush (idempotent; themeApp's cleanup re-closes)
	dir := filepath.Dir(kvPathOf(e))
	e2, err := theme.New(theme.EngineOptions{
		KVPath:        filepath.Join(dir, "kv.json"),
		GlobalYoloDir: dir,
		CWD:           dir,
		Palette:       func(context.Context) (theme.TerminalColors, bool) { return theme.TerminalColors{}, false },
	})
	if err != nil {
		t.Fatalf("theme.New (second): %v", err)
	}
	t.Cleanup(func() { _ = e2.Close() })
	if err := e2.Resolve(context.Background()); err != nil {
		t.Fatalf("theme.Resolve (second): %v", err)
	}
	b := &recApp{App: NewApp(client.New("http://127.0.0.1:9", ""), store.State{}, "", e2)}
	b.emitSink = func(cmds ...tea.Cmd) { b.Cmds = append(b.Cmds, cmds...) }
	if b.sidebarMode != "hide" {
		t.Fatalf("the sidebar mode must persist across restart (got %q, want hide)", b.sidebarMode)
	}
}

// TestSidebarLayout pins the sessionChrome composition: the sidebar is the
// right sidebarWidth columns of the viewport lines (the title / divider /
// help lines stay full width), the viewport wraps at w-42 (padded to the
// width by the bubbles viewport), and the panel carries the session title
// + the todo block.
func TestSidebarLayout(t *testing.T) {
	a := testApp()
	a.route = routeSession
	a.size = tea.WindowSizeMsg{Width: 140, Height: 30}
	a.store.Current = &protocol.Session{ID: "ses_1", Title: "my session"}
	a.store.Messages = []protocol.MessageWithParts{
		{Info: protocol.Message{ID: "m1", Role: "user"}, Parts: []protocol.Part{{Type: "text", Text: "plan it"}}},
		{Info: protocol.Message{ID: "m2", Role: "assistant"}, Parts: []protocol.Part{
			todowritePart(map[string]any{"content": "one", "status": "in_progress"}),
		}},
	}
	vh := 8
	helpRows := len(strings.Split(wrapLine(sessionHelp, 140), "\n"))
	got := a.sessionChrome(140, vh)
	rows := strings.Split(got, "\n")
	if len(rows) != 1+vh+1+helpRows {
		t.Fatalf("line count = %d, want %d (title + %d viewport + divider + help)",
			len(rows), 1+vh+1+helpRows, vh)
	}
	// every viewport row is exactly 140 columns (the w-42 left viewport,
	// padded to its width, + the 42-column panel).
	for i := 0; i < vh; i++ {
		if w := runeWidth(stripANSI(rows[1+i])); w != 140 {
			t.Fatalf("viewport row %d width = %d, want 140 (the w-42 left + 42 panel)", i, w)
		}
	}
	// the ▼ Todo header + the [•] item land in the viewport rows (the
	// right 42 columns).
	found := 0
	for i := 0; i < vh; i++ {
		l := stripANSI(rows[1+i])
		if strings.Contains(l, "▼ Todo") {
			found++
		}
		if strings.Contains(l, "[•] one") {
			found++
		}
	}
	if found != 2 {
		t.Fatalf("sidebar content rows: found=%d, want the ▼ Todo header + the [•] item\n%s", found, got)
	}
	// the session title lands in the panel (right of the transcript).
	if !strings.Contains(stripANSI(got), "my session") {
		t.Fatalf("the session title must render in the panel\n%s", got)
	}
	// the hidden case (the toggle off at the narrow width): the full-width
	// viewport, no panel.
	b := testApp()
	b.route = routeSession
	b.size = tea.WindowSizeMsg{Width: 100, Height: 30}
	b.store.Current = &protocol.Session{ID: "ses_1", Title: "my session"}
	rowsB := strings.Split(b.sessionChrome(100, 4), "\n")
	for i := 0; i < 4; i++ {
		if w := runeWidth(stripANSI(rowsB[1+i])); w != 100 {
			t.Fatalf("hidden-sidebar viewport row %d width = %d, want 100 (no panel)", i, w)
		}
	}
}

// TestSidebarTeatestPresence drives a real turn (the scripted fake driver
// emits a todowrite tool part) at the wide terminal size (140 > 120 → the
// auto-show): the ▼ Todo header + the 3 status-glyph items render. The
// merged WaitFor honors the buffer-drain rule (one condition, not two).
func TestSidebarTeatestPresence(t *testing.T) {
	drv := fake.New(
		fake.Turn{Parts: []llm.Part{{
			Kind:   "tool",
			Name:   "todowrite",
			CallID: "call_todo",
			Args:   json.RawMessage(`{"todos":[{"content":"file the beads","status":"in_progress"},{"content":"run the gate","status":"pending"},{"content":"design the sidebar","status":"completed"}]}`),
			Finish: "tool_calls",
		}}},
		fake.Turn{Parts: []llm.Part{{Kind: "text", Text: "all done"}}},
	)
	ts := testutil.BootWithDriver(t, drv)
	c := client.New(ts.URL, ts.Dir)
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(140, 24))

	teatest.WaitFor(t, tm.Output(), hasLine("New session"), teatest.WithDuration(5*time.Second))
	tm.Send(press('n'))
	teatest.WaitFor(t, tm.Output(), hasLine("esc abort/back"), teatest.WithDuration(5*time.Second))
	suiteType(tm, "plan it")
	tm.Send(press(tea.KeyEnter))

	var full string
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		full = stripANSI(string(b))
		return strings.Contains(full, "all done") &&
			strings.Contains(full, "▼ Todo") &&
			strings.Contains(full, "[•] file the beads") &&
			strings.Contains(full, "[ ] run the gate") &&
			strings.Contains(full, "[✓] design the sidebar")
	}, teatest.WithDuration(10*time.Second))

	tm.Send(ctrlCKey) // quit confirm
	tm.Send(press('y'))
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
```

**Step 2 — confirm FAIL:** `go test ./internal/tui/ -run
'TestSidebarToggle|TestSidebarTogglePersists|TestSidebarLayout|TestSidebarTeatestPresence'`
→ build fails (undefined `sidebarVisible`, `toggleSidebar`,
`loadSidebarMode`, `kvSidebarModeKey`, the `sidebarMode` / `sidebarOpen`
fields, `sidebarLines`, `sidebarWidth`). That is the red.

**Step 3 — minimal implementation:**

- `internal/tui/app.go` — the fields (beside `tipsHidden`), the const
  (beside `kvTipsHiddenKey`), the `NewApp` tail call (after
  `a.loadTipsHidden()`):

```go
// kvSidebarModeKey is the KV key the sidebar mode persists under (the S6.3
// theme-KV seam, deviation 223's class): "auto" | "hide".
const kvSidebarModeKey = "sidebar_mode"
```

```go
// loadSidebarMode restores the sidebar mode (the S7.2 KV seam; a nil
// engine stays at the "auto" default).
func (a *App) loadSidebarMode() {
	a.sidebarMode = "auto"
	if a.engine == nil {
		return
	}
	if m, ok := a.engine.KV().Get(kvSidebarModeKey, "auto").(string); ok && m == "hide" {
		a.sidebarMode = "hide"
	}
}

// sidebarVisible ports the upstream sidebarVisible (index.tsx:272-276)
// minus the subagent parentID gate (no Session.ParentID on the wire — the
// S7 detail finding): the forced open, or the auto mode at a wide
// terminal (>120 columns).
func (a *App) sidebarVisible() bool {
	if a.sidebarOpen {
		return true
	}
	return a.sidebarMode == "auto" && a.size.Width > 120
}

// toggleSidebar ports the session.sidebar.toggle command flip
// (index.tsx:674-681) + the KV persist (the S6.3 tips_toggle pattern — no
// cmds; bubbletea re-renders after every Update).
func (a *App) toggleSidebar() {
	if a.sidebarVisible() {
		a.sidebarMode = "hide"
		a.sidebarOpen = false
	} else {
		a.sidebarMode = "auto"
		a.sidebarOpen = true
	}
	if a.engine != nil {
		a.engine.KV().Set(kvSidebarModeKey, a.sidebarMode)
	}
}
```

- `internal/tui/keymap.go` — the `BaseMode` group (the END, the S6.3
  first-referent pattern):

```go
	BaseMode: {
		"which_key_toggle", "which_key_layout_toggle", "which_key_pending_toggle",
		"command_list", "app_exit", "model_list", "agent_list", "status_view",
		"theme_list", "session_new", "session_list", "tips_toggle", "sidebar_toggle",
	},
```

- `internal/tui/keys.go` — the `dispatchCommand` case (after the
  `tips_toggle` case):

```go
	case "sidebar_toggle":
		// S7.2: the sidebar only exists on the session route (the upstream
		// home has no session.sidebar.toggle command — a no-op elsewhere).
		// No cmds — bubbletea re-renders after the flip.
		if a.route != routeSession {
			return nil
		}
		a.toggleSidebar()
```

- `internal/tui/sidebar.go` — the panel constants + the composition:

```go
// sidebarWidth is the upstream sidebar shell width (sidebar.tsx).
const sidebarWidth = 42

// sidebarInnerW is the panel content width (the sidebarWidth minus the
// paddingLeft 2 + paddingRight 2; the upstream content box's paddingRight
// 1 is absorbed into the panel padding — deviation 246).
const sidebarInnerW = 38

// sideRow is the plain content line of the sidebar panel: the display text
// + the fg style (the panel bg is applied to the pad tail — the
// deletefailed.go:77 fg+bg composition idiom).
type sideRow struct {
	plain string
	st    lipgloss.Style
}

// sidebarLines renders the sidebar panel as exactly vh lines (each padded
// to sidebarWidth with the backgroundPanel fill — the upstream shell:
// backgroundPanel, paddingTop 1, paddingBottom 1, paddingLeft 2,
// paddingRight 2): the paddingTop blank line (fully panel-painted), the
// session-title block (the bold theme.text, wrapped at sidebarInnerW —
// the upstream sidebar_title minus the sessionID / workspace / share
// spans, no yolo referents), the blank gap line, then the todo block
// (S7.1, hidden by the todoBlockVisible gate), then blank lines to the
// bottom. The fg applies to the content; the pad tail carries the panel
// bg (an outer-width pad over an inner-styled line would not paint).
func (a *App) sidebarLines(vh int) []string {
	panel := a.theme.BackgroundPanel()
	var rows []sideRow
	title := "session"
	if a.store.Current != nil {
		title = a.store.Current.Title
	}
	for _, l := range strings.Split(wrapLine(title, sidebarInnerW), "\n") {
		rows = append(rows, sideRow{plain: l, st: a.theme.Text()})
	}
	if todos := latestTodos(&a.store); todoBlockVisible(todos) {
		rows = append(rows, sideRow{}) // the blank gap line (the panel paint only)
		for _, r := range todoSidebarRows(todos, sidebarInnerW) {
			switch {
			case r.header:
				rows = append(rows, sideRow{plain: r.plain, st: a.theme.Text()})
			case r.inProgress:
				rows = append(rows, sideRow{plain: r.plain, st: a.theme.Warning()})
			default:
				rows = append(rows, sideRow{plain: r.plain, st: a.theme.TextMuted()})
			}
		}
	}
	lines := make([]string, 0, vh)
	for i := 0; i < vh; i++ {
		plain := ""
		var st lipgloss.Style
		if i > 0 && i-1 < len(rows) { // i == 0: the paddingTop blank line
			plain, st = rows[i-1].plain, rows[i-1].st
		}
		body := ""
		if plain != "" {
			body = st.Render(plain)
		}
		if pad := sidebarWidth - runeWidth(plain); pad > 0 {
			body += panel.Render(strings.Repeat(" ", pad))
		}
		lines = append(lines, body)
	}
	return lines
}
```

(the import `lipgloss "charm.land/lipgloss/v2"` added to sidebar.go's
block)

- `internal/tui/view.go` — the `sessionChrome` sidebar composition
  (replacing the current body):

```go
// sessionChrome renders the session route's chrome for a viewport of vh
// lines: title, the transcript viewport (the todo sidebar — S7.2 — as the
// right sidebarWidth columns of the viewport lines, deviation 246),
// divider, the (possibly wrapped) help.
func (a *App) sessionChrome(w, vh int) string {
	if vh < 1 {
		vh = 1
	}
	var side []string
	leftW := w
	if a.sidebarVisible() && w > sidebarWidth {
		side = a.sidebarLines(vh)
		leftW = w - sidebarWidth
	}
	a.sess.sync(&a.store, leftW, vh, a.theme, a.spinFrame())
	t := "session"
	if a.store.Current != nil {
		t = a.store.Current.Title
	}
	var b strings.Builder
	b.WriteString(title.Render(t) + "\n")
	for i, row := range strings.Split(a.sess.vm.View(), "\n") {
		if i > 0 {
			b.WriteString("\n")
		}
		if side != nil && i < len(side) {
			row += side[i]
		}
		b.WriteString(row)
	}
	b.WriteString("\n" + dividerLineRendered)
	for _, l := range strings.Split(wrapLine(sessionHelp, w), "\n") {
		b.WriteString("\n" + a.theme.TextMuted().Render(l))
	}
	return b.String()
}
```

**Step 4 — gate:** `go vet ./... && go test ./...` green (incl.
`TestImportsDirection` — `sidebar.go` gains the `lipgloss` charm dep,
allowlisted) + `gofmt -l .` empty.

**Step 5 — commit** the pinned message `feat: todo sidebar - keymap
toggle + layout`, then `bd close yolo-oae.8.3 --reason "S7.2 done: the
sidebar toggle (the sidebar_toggle registry binding's first referent —
the BaseMode group append + the route-guarded dispatch case, deviation
247) + the ported visibility (the KV-persisted auto/hide mode, the >120
auto-show, the forced open) + the 42-column panel layout (the transcript
viewport lines only, deviation 246; the backgroundPanel fill, the session
title + the todo block); unit table + the wide-terminal teatest presence
leg" --json`.

### Task S7.3: Dialog-message (full-message view) + tests (bead `yolo-oae.8.3`, expected id `yolo-oae.8.4`)

**Files:** new `internal/tui/messagedlg.go` (the dialog render), modify
`internal/tui/dialog.go` (the `dlgMessage` kind + the `message` payload
field + the `modalInner` case), `internal/tui/session.go` (the
`sessKeyMap.MessageView` binding + the `handleSessionKey` case), new
`internal/tui/messagedlg_test.go` (the unit table).

**Interfaces:** `dlgMessage` (the `dialogKind` block), `dialog.message
*protocol.MessageWithParts` (the snapshot payload), `sessKeyMap.MessageView
key.Binding` (default `alt+m`), `const msgPartMaxLines = 12`, `func (a
*App) openMessageDialog()`, `func (a *App) messageHeaderRow(w int, th
theme.Theme) string`, `func (a *App) messageView(m *protocol.MessageWithParts,
w int, th theme.Theme) string`.

**Upstream parity notes:** the upstream `dialog-message.tsx` is the
"Message Actions" dialog (Revert/Copy/Fork, the mouse-clicked user message)
— the yolo redefinition (deviation 248): the full-message view over the
**last** message snapshot (no mouse, no revert/fork endpoints, no clipboard
contract — finding 2), opened by the **yolo-surface** `alt+m` (the
Expand/Think precedent, deviation 211's scope — NOT a registry binding;
`alt+m` verified unbound by the bubbles v2.2.1 textinput DefaultKeyMap).
The content renders the message meta + every part (text / reasoning / tool
output) — the upstream dialog content (the message text + the action
buttons) has no yolo referent (the parts list is the yolo "full" view);
each part's content clamps at `msgPartMaxLines` with the `… (N more
lines)` hint AFTER the head (the `headPreview` idiom). No key case — the
generic esc/ctrl+c modal close (`handleDialogKey`) dismisses (the
`statusdlg` idiom).

**Step 1 — failing test** (`internal/tui/messagedlg_test.go`):

```go
package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/protocol"
)

// TestMessageDialogOpen pins the opener (the S7.3 yolo-surface alt+m, the
// Expand/Think precedent): the session route + a message pushes the
// dlgMessage modal with the LAST message snapshot; esc closes it; with no
// message the key is consumed and the stack stays empty; the home route
// never opens it (the session keys are not dispatched there).
func TestMessageDialogOpen(t *testing.T) {
	a := testApp()
	a.route = routeSession
	a.store.Messages = []protocol.MessageWithParts{
		{Info: protocol.Message{ID: "m1", Role: "user"}},
		{Info: protocol.Message{ID: "m2", Role: "assistant"}},
	}
	a.handleKey(pressAlt('m'))
	d, ok := a.dlg.top()
	if !ok || !d.modal || d.kind != dlgMessage || d.message == nil || d.message.Info.ID != "m2" {
		t.Fatalf("after alt+m: top=%+v, want the dlgMessage modal with the last message", d)
	}
	a.handleKey(press(tea.KeyEscape))
	if d, ok := a.dlg.top(); ok && d.modal {
		t.Fatalf("esc must close the message dialog: top=%+v", d)
	}

	b := testApp()
	b.route = routeSession
	b.handleKey(pressAlt('m'))
	if d, ok := b.dlg.top(); ok && d.modal {
		t.Fatalf("no message: the stack must stay empty, top=%+v", d)
	}

	c := testApp()
	c.store.Messages = []protocol.MessageWithParts{{Info: protocol.Message{ID: "m1"}}}
	c.handleKey(pressAlt('m')) // the home route: the session keys are not dispatched
	if d, ok := c.dlg.top(); ok && d.modal {
		t.Fatalf("home route: the message dialog must not open, top=%+v", d)
	}
}

// TestMessageViewRender pins the full-message render (the stripANSI unit
// idiom): the header row, the meta line (the created time is formatted the
// same way in the expectation — TZ-independent), the per-part headers +
// content, the msgPartMaxLines clamp + the hint (AFTER the head), the
// error line.
func TestMessageViewRender(t *testing.T) {
	a := testApp()
	m := &protocol.MessageWithParts{
		Info: protocol.Message{
			ID:     "m1",
			Role:   "assistant",
			Agent:  "build",
			Time:   protocol.MessageTime{Created: 1_700_000_000_000},
			Tokens: &protocol.Tokens{Input: 123, Output: 45},
			Cost:   0.42,
		},
		Parts: []protocol.Part{
			{Type: "text", Text: "hello world"},
			{Type: "reasoning", Text: "think hard"},
			{Type: "tool", Tool: "bash", State: &protocol.ToolState{
				Title:  "Run command",
				Output: strings.Repeat("out\n", 39) + "out", // 40 lines, no trailing newline
				Error:  "boom",
			}},
		},
	}
	wantTime := time.UnixMilli(1_700_000_000_000).Format("15:04:05")
	got := stripANSI(a.messageView(m, 86, a.theme))
	if !strings.Contains(got, "Message") || !strings.Contains(got, "esc") {
		t.Fatalf("header row missing:\n%s", got)
	}
	if want := "assistant · build · " + wantTime + " · ↑123 ↓45 · $0.42"; !strings.Contains(got, want) {
		t.Fatalf("meta line %q missing:\n%s", want, got)
	}
	if !strings.Contains(got, "Text") || !strings.Contains(got, "hello world") {
		t.Fatalf("the text part is missing:\n%s", got)
	}
	if !strings.Contains(got, "Reasoning") || !strings.Contains(got, "think hard") {
		t.Fatalf("the reasoning part is missing:\n%s", got)
	}
	if !strings.Contains(got, "Tool: bash — Run command") {
		t.Fatalf("the tool header is missing:\n%s", got)
	}
	// the clamp: the 40-line output renders 12 head lines + the hint.
	n := 0
	for _, l := range strings.Split(got, "\n") {
		if l == "out" {
			n++
		}
	}
	if n != 12 {
		t.Fatalf("clamped output lines = %d, want 12 (msgPartMaxLines):\n%s", n, got)
	}
	if !strings.Contains(got, "… (28 more lines)") {
		t.Fatalf("the overflow hint is missing:\n%s", got)
	}
	if !strings.Contains(got, "error: boom") {
		t.Fatalf("the tool error line is missing:\n%s", got)
	}
	// the empty-parts omissions: the meta line without agent / tokens /
	// cost.
	m2 := &protocol.MessageWithParts{
		Info:  protocol.Message{ID: "m2", Role: "user", Time: protocol.MessageTime{Created: 1_700_000_000_000}},
		Parts: []protocol.Part{{Type: "text", Text: "hi"}},
	}
	got2 := stripANSI(a.messageView(m2, 86, a.theme))
	if want := "user · " + wantTime; !strings.Contains(got2, want) {
		t.Fatalf("the user meta line %q missing:\n%s", want, got2)
	}
}
```

**Step 2 — confirm FAIL:** `go test ./internal/tui/ -run
'TestMessageDialogOpen|TestMessageViewRender'` → build fails (undefined
`dlgMessage`, the `message` payload field, `openMessageDialog`,
`messageView`, the `MessageView` binding, `msgPartMaxLines`). That is the
red.

**Step 3 — minimal implementation:**

- `internal/tui/messagedlg.go` (new):

```go
// messagedlg.go — the full-message view (S7.3): the dlgMessage modal. The
// upstream routes/session/dialog-message.tsx @ v1.18.18 is the "Message
// Actions" dialog (Revert / Copy / Fork, the mouse-clicked user message) —
// the yolo redefinition (deviation 248): the full-message view over the
// LAST message snapshot (no mouse, no revert/fork endpoints, no clipboard
// contract), opened by the yolo-surface alt+m (the sessKeyMap Expand/Think
// precedent — deviation 211's scope; alt+m is unbound by the bubbles
// v2.2.1 textinput DefaultKeyMap). The content renders the message meta +
// every part (text / reasoning / tool output), each clamped at
// msgPartMaxLines with the "… (N more lines)" hint after the head (the
// headPreview idiom). No key case — the generic esc/ctrl+c modal close
// (handleDialogKey) dismisses.

package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// msgPartMaxLines is the per-part content clamp (the headPreview idiom —
// the 10-line bash output preview): the full-message view clamps every
// part's content block so the dialog fits the panel without scrolling.
const msgPartMaxLines = 12

// openMessageDialog pushes the full-message view: the LAST message
// snapshot (the yolo referent for the upstream clicked message) in the
// dlgLarge panel. A no-op with no message (the caller consumes the key).
func (a *App) openMessageDialog() {
	if len(a.store.Messages) == 0 {
		return
	}
	m := a.store.Messages[len(a.store.Messages)-1]
	a.pushModal(dialog{kind: dlgMessage, message: &m}, dlgLarge, nil)
}

// messageHeaderRow is the dialog header: the bold "Message" left, the
// muted "esc" right, space-between at the panel width (the statusHeaderRow
// idiom).
func (a *App) messageHeaderRow(w int, th theme.Theme) string {
	const t = "Message"
	pad := w - runeWidth(t) - runeWidth("esc")
	if pad < 0 {
		pad = 0
	}
	return title.Render(t) + strings.Repeat(" ", pad) + th.TextMuted().Render("esc")
}

// messageView renders the full-message dialog (the modal stack draws the
// panel chrome): the header row, the meta line (role · agent · created ·
// ↑in ↓out · $cost — the empty parts omitted), then one block per part:
// the muted header (Text / Reasoning / Tool: <name> — <title>) + the
// content word-wrapped at w, clamped at msgPartMaxLines with the hint
// after the head; a tool error renders in the Error fg.
func (a *App) messageView(m *protocol.MessageWithParts, w int, th theme.Theme) string {
	var b strings.Builder
	b.WriteString(a.messageHeaderRow(w, th))
	meta := []string{m.Info.Role}
	if m.Info.Agent != "" {
		meta = append(meta, m.Info.Agent)
	}
	meta = append(meta, time.UnixMilli(m.Info.Time.Created).Format("15:04:05"))
	if m.Info.Tokens != nil {
		meta = append(meta, "↑"+strconv.FormatInt(m.Info.Tokens.Input, 10)+" ↓"+strconv.FormatInt(m.Info.Tokens.Output, 10))
	}
	if m.Info.Cost > 0 {
		meta = append(meta, fmt.Sprintf("$%.2f", m.Info.Cost))
	}
	b.WriteString("\n" + th.TextMuted().Render(strings.Join(meta, " · ")))
	for _, p := range m.Parts {
		var header, content string
		switch p.Type {
		case "text":
			header, content = "Text", p.Text
		case "reasoning":
			header, content = "Reasoning", p.Text
		case "tool":
			header = "Tool: " + p.Tool
			if p.State != nil {
				if p.State.Title != "" {
					header += " — " + p.State.Title
				}
				content = p.State.Output
			}
		default:
			continue
		}
		b.WriteString("\n\n" + th.TextMuted().Render(header))
		if content != "" {
			rows := strings.Split(wrapLine(content, w), "\n")
			if len(rows) > msgPartMaxLines {
				overflow := len(rows) - msgPartMaxLines
				rows = rows[:msgPartMaxLines]
				for _, r := range rows {
					b.WriteString("\n" + th.Text().Render(r))
				}
				b.WriteString("\n" + th.TextMuted().Render("… ("+strconv.Itoa(overflow)+" more lines)"))
			} else {
				for _, r := range rows {
					b.WriteString("\n" + th.Text().Render(r))
				}
			}
		}
		if p.Type == "tool" && p.State != nil && p.State.Error != "" {
			b.WriteString("\n" + th.Error().Render("error: "+p.State.Error))
		}
	}
	return b.String()
}
```

- `internal/tui/dialog.go` — the kind (after `dlgPalette` in the const
  block), the payload field (in the `dialog` struct), the `modalInner`
  case (after the `dlgPalette` case):

```go
	dlgMessage // S7.3: the full-message view (the snapshot payload)
```

```go
	message *protocol.MessageWithParts // S7.3: the full-message payload (dlgMessage)
```

```go
	case dlgMessage:
		if d.message != nil {
			return a.messageView(d.message, w, a.theme)
		}
```

- `internal/tui/session.go` — the binding (the `sessKeyMap` struct +
  literal) + the `handleSessionKey` case (after the Think case):

```go
	// S7.3: the yolo-surface full-message view (deviation 248's scope —
	// the upstream dialog-message opener is a mouse click, no key referent;
	// alt+m is unbound by textinput's DefaultKeyMap, the alt+e / alt+t
	// precedent).
	MessageView: key.NewBinding(key.WithKeys("alt+m")),
```

```go
	case key.Matches(k, sessKeyMap.MessageView):
		a.openMessageDialog()
		return nil, true
```

**Step 4 — gate:** `go vet ./... && go test ./...` green (incl.
`TestImportsDirection`) + `gofmt -l .` empty.

**Step 5 — commit** the pinned message `feat: dialog-message (full
message view)`, then `bd close yolo-oae.8.4 --reason "S7.3 done: the
dlgMessage modal (the upstream dialog-message redefinition — deviation
248: the full-message view over the last-message snapshot, the yolo-
surface alt+m opener, the yolo-surface scope per deviation 211) + the
render (the Message/esc header, the role·agent·created·tokens·cost meta,
the per-part Text/Reasoning/Tool blocks, the 12-line clamp + the … (N
more lines) hint, the error line); unit opener + render pins" --json`.

### Task S7.4: Session footer detail restyle (model/agent/tokens/cost/spinner/connection) + tests (bead `yolo-oae.8.4`, expected id `yolo-oae.8.5`)

**Files:** modify `internal/tui/locale.go` (the `number` port + the
`strconv` import), `internal/tui/footer.go` (the restyled main render +
the `contextPct` helper + the `math` import), `internal/tui/footer_test.go`
(the re-baselined pins — Step 1), new `internal/tui/footer_restyle_test.go`
(the new pins — Step 1).

**Interfaces:** `func number(n int64) string` (locale.go), `func (a *App)
contextPct() int64` (footer.go, -1 when unknown).

**Upstream parity notes:** the referent set is footer.tsx's detail
conventions + the prompt-bar usage memo — `routes/session/footer.tsx` is
**orphaned upstream** (no import site; the rendered session bottom is the
prompt bar), so the port is the conventions, not a rendered file (finding
3, deviation 249): the `Locale.number` compact token format (locale.ts:46-54,
ported verbatim over the yolo `↑in ↓out` arrow segments — the frozen
segment set is kept), the `(pct%)` context segment (the prompt-bar usage
shape: `round(total / model.limit.context * 100)` — the total over the
yolo **session aggregate** `store.Current.Tokens`, the model resolved over
`store.Providers` — the lazy-catalog referent, deviation 241), the Intl
en-US USD cost shape (`$` + 2 decimals, omitted when `cost == 0`). The
model / agent / spinner / conn segments stay (S0.10's theming; the conn
dots are the upstream L69-84 idiom, already ported; the spinner = the
locked 5 frames). The welcome blink + the permission / LSP / MCP dot
segments have no yolo referent (finding 3) — out of scope. The
`footer_test.go` pins re-baseline (root principle 3).

**Step 1 — failing test:** new `internal/tui/footer_restyle_test.go` +
the re-baselined `internal/tui/footer_test.go` pins (the 8 `want` strings
+ the subtest name):

```go
package tui

import (
	"strings"
	"testing"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/store"
)

// TestNumber pins the ported Locale.number (locale.ts:46-54): the
// ≥1e6 → "1.2M" / ≥1e3 → "1.2K" compact form, the plain string below
// (the identity under 1000 — the existing ↑123 ↓45 pins stay).
func TestNumber(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0"},
		{42, "42"},
		{999, "999"},
		{1000, "1.0K"},
		{1234, "1.2K"},
		{12345, "12.3K"},
		{1000000, "1.0M"},
		{1234567, "1.2M"},
	}
	for _, tt := range tests {
		if got := number(tt.n); got != tt.want {
			t.Fatalf("number(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

// TestFooterTokensCompact pins the restyled tokens segment: the K/M
// compact form over the ↑in ↓out arrows (the frozen segment set is kept,
// the numbers get the upstream format).
func TestFooterTokensCompact(t *testing.T) {
	a := footerApp(store.State{
		Live:    true,
		Current: &protocol.Session{ID: "ses_1", Agent: "build",
			Model: refModel("kido", "q"),
			Tokens: protocol.Tokens{Input: 12345, Output: 678}},
	})
	if got := stripANSI(a.footerView()); !strings.Contains(got, "↑12.3K ↓678") {
		t.Fatalf("footer = %q, want the compact ↑12.3K ↓678 tokens segment", got)
	}
}

// TestFooterContextPct pins the (pct%) context segment: shown only when
// the session model resolves (over store.Providers — the lazy catalog
// referent, deviation 241) to a Limit.Context > 0; pct = round(100 *
// total / context), total = the session aggregate
// input+output+reasoning+cache.read+cache.write.
func TestFooterContextPct(t *testing.T) {
	mk := func(provs []protocol.Provider) string {
		st := store.State{
			Live:    true,
			Current: &protocol.Session{ID: "ses_1", Agent: "build",
				Model: refModel("kido", "q"),
				Tokens: protocol.Tokens{Input: 100, Output: 50, Reasoning: 25,
					Cache: protocol.CacheTokens{Read: 10, Write: 15}}},
		}
		st.Providers = provs
		return stripANSI(footerApp(st).footerView())
	}
	// total = 100+50+25+10+15 = 200 → 200/200 = 100%.
	if got := mk([]protocol.Provider{{ID: "kido", Models: map[string]protocol.Model{
		"q": {ID: "q", Limit: protocol.ModelLimit{Context: 200}},
	}}}); !strings.Contains(got, "(100%)") {
		t.Fatalf("footer = %q, want the (100%) context segment", got)
	}
	// 200/1000 → 20%.
	if got := mk([]protocol.Provider{{ID: "kido", Models: map[string]protocol.Model{
		"q": {ID: "q", Limit: protocol.ModelLimit{Context: 1000}},
	}}}); !strings.Contains(got, "(20%)") {
		t.Fatalf("footer = %q, want the (20%) context segment", got)
	}
	// no catalog: the segment is absent (the lazy referent); a zero context
	// limit: absent too.
	if got := mk(nil); strings.Contains(got, "%") {
		t.Fatalf("no-catalog footer = %q, want no context segment", got)
	}
	if got := mk([]protocol.Provider{{ID: "kido", Models: map[string]protocol.Model{
		"q": {ID: "q", Limit: protocol.ModelLimit{Context: 0}},
	}}}); strings.Contains(got, "%") {
		t.Fatalf("zero-limit footer = %q, want no context segment", got)
	}
}

// TestFooterCostOmitted pins the upstream cost convention (deviation 249):
// the Intl en-US USD shape "$%.2f", the segment omitted when cost == 0.
func TestFooterCostOmitted(t *testing.T) {
	a := footerApp(store.State{
		Live:    true,
		Current: &protocol.Session{ID: "ses_1", Agent: "build",
			Model: refModel("kido", "q")},
	})
	if got := stripANSI(a.footerView()); strings.Contains(got, "$") {
		t.Fatalf("zero-cost footer = %q, want the cost segment omitted", got)
	}
	b := footerApp(store.State{
		Live:    true,
		Current: &protocol.Session{ID: "ses_1", Agent: "build",
			Model: refModel("kido", "q"), Cost: 1.2346},
	})
	if got := stripANSI(b.footerView()); !strings.Contains(got, "$1.23") {
		t.Fatalf("footer = %q, want the $1.23 cost segment (2 decimals)", got)
	}
}
```

the `internal/tui/footer_test.go` re-baseline (root principle 3 — the
intentional change re-baselines the pin in the same commit; the `↑123 ↓45`
pins are UNCHANGED — `number` is the identity under 1000):

- L52: `"kido/q · build · ↑123 ↓45 · $0.0002 · ● live"` → `"kido/q · build · ↑123 ↓45 · $0.00 · ● live"`
- L58: `"kido/q · build · ↑123 ↓45 · $0.0002 · ○ off"` → `"kido/q · build · ↑123 ↓45 · $0.00 · ○ off"`
- L64: `"kido/q · build · ↑123 ↓45 · $0.0002 · ● live · ⠋ busy"` → `"kido/q · build · ↑123 ↓45 · $0.00 · ● live · ⠋ busy"`
- L72: `"kido/q · build · ↑123 ↓45 · $0.0002 · ● live · ⠋ retry 2: rate limited"` → `"kido/q · build · ↑123 ↓45 · $0.00 · ● live · ⠋ retry 2: rate limited"`
- L78: `"no model · build · ↑123 ↓45 · $0.0002 · ● live"` → `"no model · build · ↑123 ↓45 · $0.00 · ● live"`
- L81-84: the subtest name "cost rounds to four decimals" → "cost rounds to two decimals (the S7.4 Intl shape)"; the want `"kido/q · build · ↑123 ↓45 · $1.2346 · ● live"` → `"kido/q · build · ↑123 ↓45 · $1.23 · ● live"` (the mutate stays `Cost = 1.23456`)
- L90: `"kido/q · build · ↑0 ↓0 · $0.0000 · ● live"` → `"kido/q · build · ↑0 ↓0 · ● live"` (the zero-cost segment omitted)
- L95: `"no model · default · ↑0 ↓0 · $0.0000 · ● live"` → `"no model · default · ↑0 ↓0 · ● live"` (the home route, the zero-cost segment omitted)

**Step 2 — confirm FAIL:** `go test ./internal/tui/ -run
'TestNumber|TestFooterTokensCompact|TestFooterContextPct|TestFooterCostOmitted|TestFooterRender'`
→ build fails (undefined `number`). That is the red.

**Step 3 — minimal implementation:**

- `internal/tui/locale.go` — the import (`strconv` added) + the port:

```go
// number ports the upstream Locale.number (packages/tui/src/util/locale.ts:46-54):
// the ≥1e6 → "1.2M" / ≥1e3 → "1.2K" compact form, the plain string below.
func number(n int64) string {
	switch {
	case n >= 1_000_000:
		return strconv.FormatFloat(float64(n)/1e6, 'f', 1, 64) + "M"
	case n >= 1_000:
		return strconv.FormatFloat(float64(n)/1e3, 'f', 1, 64) + "K"
	default:
		return strconv.FormatInt(n, 10)
	}
}
```

- `internal/tui/footer.go` — the restyled main render (replacing the
  `muted := …; main := muted.Render(strings.Join(...))` block; the conn +
  status segments are unchanged) + the `contextPct` helper + the `math`
  import (the `strconv` import is REMOVED — the only use was the
  `strconv.FormatInt` tokens segment, now `number()`):

```go
	muted := a.theme.TextMuted()
	tokensSeg := "↑" + number(tokens.Input) + " ↓" + number(tokens.Output)
	if pct := a.contextPct(); pct >= 0 {
		tokensSeg += fmt.Sprintf(" (%d%%)", pct)
	}
	seg := model + " · " + agent + " · " + tokensSeg
	if cost > 0 {
		seg += " · " + fmt.Sprintf("$%.2f", cost)
	}
	parts := []string{muted.Render(seg)}
```

```go
// contextPct is the S7.4 restyle's context percentage (the upstream
// prompt-bar usage shape, prompt/index.tsx:264-282): the round(100 * total
// / context) when the session model resolves (over store.Providers — the
// lazy catalog referent, deviation 241) to a Limit.Context > 0; total =
// the session aggregate input+output+reasoning+cache.read+cache.write
// (the yolo referent for the upstream last-assistant-message total —
// deviation 249). -1 when unknown (the segment omitted).
func (a *App) contextPct() int64 {
	if a.route != routeSession || a.store.Current == nil {
		return -1
	}
	mr := a.store.Current.Model
	if mr == nil {
		return -1
	}
	for _, p := range a.store.Providers {
		m, ok := p.Models[mr.ID]
		if !ok || p.ID != mr.ProviderID {
			continue
		}
		if m.Limit.Context <= 0 {
			return -1
		}
		t := a.store.Current.Tokens
		total := t.Input + t.Output + t.Reasoning + t.Cache.Read + t.Cache.Write
		return int64(math.Round(100 * float64(total) / float64(m.Limit.Context)))
	}
	return -1
}
```

**Step 4 — gate:** `go vet ./... && go test ./...` green (incl.
`TestImportsDirection` + the re-baselined `TestFooterRender` + the
unaffected `resync_test` / `home_theme_test` Contains checks) + `gofmt -l
.` empty. The `footer_test.go` re-baseline is the S7.4 pin re-baseline
(root principle 3 — the pins record the current intended content; the
intentional cost/format change re-baselines in the same commit).

**Step 5 — commit** the pinned message `feat: session footer detail
restyle`, then `bd close yolo-oae.8.5 --reason "S7.4 done: the session
footer detail restyle (deviation 249 — the orphaned footer.tsx referent
shift: the Locale.number compact token format over the ↑in ↓out segments,
the (pct%) context segment (the session-aggregate total, the lazy-catalog
model resolution), the Intl en-US USD cost shape ($%.2f, omitted when
zero), the model/agent/spinner/conn segments unchanged); the number() port
+ the re-baselined pins (principle 3) + the new unit table" --json`.

## S7 slice gate (slice bead `yolo-oae.8`)

NOT a task bead; runs after all child beads close. Mirror the S0 slice gate
shape: (1) module gate `go vet ./... && go test ./...` + `gofmt -l .` empty
(incl. `TestImportsDirection` + the S7 teatest goldens); (2) user-run smoke
(NOT CI): in a real TTY — the todo sidebar (toggle + the status-glyph list
for a session with a `todowrite` part), the full-message dialog, and the
session footer segments (model/agent/tokens/cost/spinner/connection);
(3) append any forced DEVIATIONS.md entries this slice named (with
severity, same-commit rule — root principle 2); (4) PROGRESS.md one-line
status pointer; (5) commit
`docs: checkpoint — S7 done, next is S8 detail pass`; (6)
`bd close yolo-oae.8 --reason "all 4 child beads closed, gate green" --json`.
