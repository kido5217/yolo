# 0.10.0 — `@` mention picker full parity (visual + behavioral)

- Date: 2026-09-07
- Status: **draft (pending user approval)**
- Beads ticket: `yolo-old.5` (the wayfinder map epic `yolo-old`'s mention-picker
  ticket; the design grill `yolo-old.3` locked the five decisions this spec
  encodes)
- Upstream referent: opencode `v1.18.18`
  (`packages/tui/src/component/prompt/autocomplete.tsx`, the `@`-mode half of
  the shared prompt autocomplete)

**Destination (the full-parity bar, restated for the `@` picker):** the `@`
mention picker is a bordered dropdown anchored **above the input** (home: above
the box's top edge; session: above the prompt line), with **theme-token
chrome** (left/right border, menu background, selected-row highlight) and
**mouse hover/select**. `tab` completes (inserts the `@` part, upstream
insertPart semantics; a directory option expands and keeps the menu open),
`enter` inserts the selected part, **file-path frecency** ranks the rows (the
existing `prompt_frecency` store, reused as-is), `esc` strips the `@`-trigger
(yolo's current behavior — a logged 259-class deviation from upstream's pure
hide). Selection is file/directory rows only (the candidate-set deviation,
§4.1 entry 2). The `@` picker has no run-on-enter (enter inserts the part, as today).

This spec mirrors the house structure of the sibling slash spec
(`2026-09-07-slash-menu-parity-design.md`, `yolo-old.4`, merged): the same
sections, the same ASCII-sketch idiom, the same key-behavior table shape, the
same planned-DEVIATIONS and re-baseline formats, and the same slice naming.
Wherever the two menus overlap — the shared `internal/tui/dropdown.go`
primitive, the anchors, the empty-text idiom, the mouse model, the
`esc`/`tab` ladder semantics — this spec **reuses and cross-references** the
slash spec; it does not re-derive.

## 1. Sibling relationship + dependency

The `@` picker renders through the shared `internal/tui/dropdown.go`
primitive that the slash epic builds in its **S1** (spec'd in the slash spec
§3.1, slice S1: `dropdownView`/`dropdownRow`, split L/R border `theme.border`,
bg `theme.backgroundMenu`, selected row `primary` + `selectedForeground`,
width = prompt box width, height `min(10, count, space-above)`, cap 10 no
scroll). **Dependency: the mention epic starts after the slash epic's S1
lands** — the map defers the epic-structure decision to after the three specs,
so this records only the dependency fact. Two further touch-points are shared
slices with the slash epic (its S3 home-anchor re-position and S6
context-aware hint): whichever epic lands first introduces the shared rule,
the second extends it to the `@` menu (the shared surface is "whichever menu
is open", and the `@`-precedence rule below means only one menu is ever open).

## 2. Current state (file:line)

- **`acView`** (prompt.go:222-248): a boxless 2-column row — the path left, a
  `●` marker on the selected row (textMuted, the 0.3.0 idiom), the rest
  plain text (theme.text); the empty line reads `no match` (:227); capped at
  `maxPickerOptions` (:244). No theme chrome, no mouse, no anchoring.
- **`mentionOptions`** (mention.go:144-188): the candidate pipeline —
  `fuzzy.Find(q, files)` over the walked files (no threshold gate), a ×2
  prefix boost (`strings.HasPrefix(m.Str, q)`, :172-173), a ×(1 +
  `frecencyScore`) factor (:175, the ported upstream scoreFn; the comment
  calls out the no-FFF referent), stable-sorted score-desc, capped 10
  (`maxPickerOptions`, mention.go:21). Empty query → all walked files in
  frecency order (frecency-zero entries keep walk order).
- **The walk** (mention.go:16-30, 55-69, 90-127): the TUI-local capped walk
  (deviation 225) — `maxWalkFiles = 1000` (:16), `maxWalkDepth = 8` (:19), the
  static ignore set `walkIgnore` (:26-30) + a minimal gitignore parse
  (`gitignorePatterns`, :55-69, comments/negation unimplemented, deviation
  230). **Files only** — `walkFiles` appends non-directory entries
  (mention.go:112-121); no directory collection. Cached per scope dir
  (`walkedFiles`, :131-137; `a.walkRoot`/`a.walked`, app.go:120-121).
- **The trigger** (mention.go:36-50): the last `@` at start-of-value or after
  whitespace, with no whitespace from the `@` to the end of the value
  (upstream's display.ts rule, ported over the full value — yolo's model is
  value-derived, not cursor-relative). **Mode-independent**: `mentionActive`
  is a pure value check with no shell-mode gate (keys.go:47-49 is the ladder
  step, not a gate; the sibling slash spec's §7 note that the `@` picker is
  "only active in shell mode" is inaccurate). No change here.
- **Keys** (keys.go:220-246, `handleAcKey`): `up`/`down` → `moveMenuSel`
  wraparound (prompt.go:250-257); `enter` → `acInsert` (mention.go:193-207) —
  the selected path **replaces** the `@`-trigger + query as **plain text**
  (no `@`, no trailing space; the deviation-222-class plain-string prompt),
  no-op when nothing is selected; `esc` → strip the `@`-trigger + query, keep
  the prefix, cursor at the strip point, `sel`→0 (:236-243). `tab` is **not
  handled in the `@` path** — the ladder's `handleAppKeys` intercepts it
  first (the `agent_cycle` binding, keymap.go:135, the BaseMode group
  keymap.go:818-820 — deviation 289). `shift+tab` likewise cycles the agent
  (keymap.go:136).
- **Selection reset**: neither menu path resets `sel` when the query changes
  (upstream resets `selected = 0` on every query change, autocomplete.tsx:
  527-530) — a shared pre-existing edge; this spec ports the reset to the `@`
  path (the slash path stays the slash epic's).
- **Frecency** (app.go:116, 580, 619-633; frecency.go:10-14, 27-36, 67-77):
  the file-path store already exists — `a.freq []frecencyEntry` (in-memory),
  persisted in the yolo theme KV under `kvFrecencyKey = "prompt_frecency"`
  (app.go:580; the 0.7.0 S5.3 persistence, deviation 223), scored
  `frequency / (1 + age-days)` (`frecencyScore`, frecency.go:27-36), capped
  1000 entries (frecency.go:19). Touched on insert: `acInsert` →
  `updateFrecency` + `saveFrecency` (mention.go:205-206; the upstream
  referent, the touch on file-mention insert, autocomplete.tsx:237-239).
  The `@` picker **already consumes it** (mention.go:175) — the store is
  reused as-is; no store change (decision 7).
- **Placement** (view.go:33-37, 43-56): home — the `acMenu` lines are
  **left-aligned overlay lines** after the fixed home stack (home.go:515-517,
  the same region as the slash menu / permission / toasts / dialog /
  which-key); session — the `acMenu` is appended after the transcript
  viewport, **before the prompt line**, the viewport height budget reduced
  via `viewSession`'s menu parameters (view.go:46-52). The session placement
  is already the target layout; the home placement is not.
- **Mouse**: none on the `@` picker (no mouse events, no hover, no
  click-select). The app-level `tea.WithMouseCellMotion()` is not yet on
  (main.go:768) — the shared prerequisite the slash epic's S5 (the mouse
  slice) lands; this spec's S4 (mouse) also depends on it.
- **Chrome tokens** (theme/styles.go): all present — `Border()` :108,
  `BackgroundMenu()` :114, `Primary()` :101, `SelectedForeground()` :77-97
  (the adaptive contrast rule), `Text()` :99, `TextMuted()` :100. The slash
  S1 consumes the identical set. `truncateMiddle` already exists
  (locale.go:24).
- **Keymap** (keymap.go:219-223): the `prompt.autocomplete.*` bindings
  (`prev` up,ctrl+p / `next` down,ctrl+n / `hide` escape / `select` return /
  `complete` tab) are **registered but not in any context group** (keymap.go:
  816-820) — deviation 211's posture (the prompt keys stay off the registry;
  the handlers match keys directly). `tab`/`shift+tab` are the registry
  `agent_cycle`/`agent_cycle_reverse` (keymap.go:135-136).
- **Test legs** (mention_test.go): `TestMentionTriggerIndex` (:18),
  `TestWalkFiles` (:40), `TestMentionOptions` (:61), `TestAcInsert` (:79),
  `TestTUIAtPicker` (:94, teatest: type `@a`, wait for the `alpha.go` row,
  enter, assert the insert in the drained output). The home mock:
  `TestHomeMockRender` (home_mock_test.go:178) hand-assembles the clean
  200×50 frame and writes it (home_mock_test.go:350).
- **The server side** (the source decision's evidence): the yolo core server
  exposes **no file-find surface** — `internal/server` + `internal/protocol`
  have no `fs.find` endpoint (deviation 225: "yolo has no `fs.find` wire
  referent (frozen)"). The send wire is `{text, files:[...]}` with no parts
  (deviation 296). **Zero-MCP** by design (tips.go:60: "mcp config section +
  mcp_*: false — no MCP"; the `mcp_list` keymap binding is a ported-but-inert
  name). Agents exist on the wire (`protocol.Agent` with `Mode`/`IsHidden`,
  protocol/agent.go:14-21) but agent selection is session-level (the pending
  agent → `session.agent`); the wire has **no per-message agent override**
  (deviation 303).

## 3. Design

### 3.1 — The dropdown usage (cross-ref: slash spec §3.1, S1)

The `@` menu renders through `dropdownView`/`dropdownRow` (the slash S1
primitive) — no `acView` of its own. Chrome (locked decision 1):

- Split L/R border, `theme.border` (the `┃` glyph, the slash spec §3.1).
- Box bg `theme.backgroundMenu`; inner padding 1 col L/R (the slash
  `dropdownRow` idiom).
- Selected row bg `theme.primary`, text `theme.selectedForeground`
  (theme/styles.go:77-97). Unselected rows: text `theme.text`, description
  `theme.textMuted`.
- Width = the prompt box width: home → `boxW` (home.go:44-51, the box width,
  clamped to the content width); session → `w` (the prompt line spans the
  full content width; the session has no box).
- Height = `min(maxPickerOptions, row count, space-above)`; cap 10, no scroll
  (the `maxPickerOptions` cap, mention.go:21, unchanged).
- Empty state: the single muted line `No matching items` (§3.7).

**Row content** (decision 1 leaves the columns to this spec — the slash spec
has two-column rows, name + description `textMuted`; mirrored for `@`):

| column | content |
|---|---|
| 1 (path, `theme.text`) | the slash-relative path — `truncateMiddle` (locale.go:24) to the available inner width (the upstream file display, `Locale.truncateMiddle(filename, anchor.width − 4)`, autocomplete.tsx:707-713). **Directory options carry a trailing `/` kind marker** (e.g. `cmd/`) — matching the expansion target `@path/` (upstream shows directories unmarked; the marker is a yolo extension of the row display, not a pinned-text deviation). |
| 2 (description, `theme.textMuted`) | files: the containing directory (e.g. `internal/tui` for `internal/tui/app.go`; empty for root-level files) — context when the path column truncates. Directories: empty (the trailing `/` already carries the kind). |

The option carrier keeps the `selectOption` shape (select.go:26-36) with
`value` set to a small `mentionOption{path string; isDir bool}` (the
directory flag drives the tab-expand branch, §3.5).

### 3.2 — Placement / anchors (cross-ref: slash spec §3.1, S3)

- **Home** — the dropdown sits **above the box's top edge**, same left edge
  (`boxL`, home.go:487) and width (`boxW`, home.go:486), overlaying the
  logo/pad rows (upstream anchor, autocomplete.tsx:664-675; the slash spec
  §3.2 home rule — the @-menu line count is the only delta). The dropdown
  replaces the current left-aligned overlay lines (home.go:515-517) for the
  `@` menu.
- **Session** — above the prompt line, overlaying the transcript tail, the
  viewport height budget reduced by the dropdown line count
  (view.go:46-52 + `viewSession`'s menu parameters — **the current session
  placement is already the target**; the @ delta is chrome only, not
  placement).
- `esc`-to-reveal (slash spec §3.2): no change — the `@` menu has the same
  reveal semantics as the slash menu (the prompt line re-emerges when the
  menu closes; the `@` esc strip closes it).

### 3.3 — Candidate source + walk params (decision A)

**Keep the TUI-local capped walk (deviation 225 re-affirmed and re-scoped).**
The yolo core server has no file-find surface (frozen wire — §2); adding an
`fs.find` endpoint (upstream's server-side FFF, `sdk.client.v2.fs.find`,
autocomplete.tsx:304-313) is a core-server change — a new wire surface, DTO,
tests, and an FFF port — well beyond the TUI parity bar, which is visual +
behavioral and does not name a source. The walk params are unchanged:
`maxWalkFiles = 1000` (mention.go:16 — the cap keeps counting file entries),
`maxWalkDepth = 8` (:19), the static ignore set + minimal gitignore parse
(mention.go:26-30, 55-69; deviation 230 unchanged). **Re-scope: the walk now
also collects directories** (pre-order, alongside files) so directory
options exist for the tab-expand (locked decision 2). Candidate set: **files
+ directories only** — agents/MCP/reference candidates are not ported
(decision B, planned entry 2, §4.1).

### 3.4 — Filter / ranking semantics (decision C)

Upstream `@`-mode filter (extracted from the source, autocomplete.tsx:476-525):
the query is the trigger-to-cursor text (upstream strips a `#line-range`
suffix via `removeLineRange`, :27-30 — N/A to yolo, no line-range suffixes,
deviation-222 class); in `@` mode the **non-file** options (agents, reference
aliases, MCP resources) are `fuzzysort`-ed (`threshold 0.5`, `limit 10`,
`scoreFn` = ×2 when the matched target starts with `"@" + query`, ×(1 +
frecency) for options with a path), while the **file** options come pre-ranked
from the server-side FFF (trusted order, never re-fuzzed), are concatenated
**after** the fuzzied non-files, and the whole is sliced to 10; an exact
non-file alias match short-circuits to that alias alone (:481-486 — N/A to
yolo, no reference surface).

**The yolo port** (the file/directory set is the only candidate pool —
decision B; the upstream non-file pool is empty here, so the file set takes
the fuzzysort's place):

| upstream param | yolo port |
|---|---|
| `keys` = [display/value, aliases] | the slash-relative path (no trailing `/` — the insert value) is the single fuzzy target. yolo paths have no aliases; the `description` key is slash-only upstream. |
| `threshold: 0.5` | **a positive-fuzzy-score gate** — keep a match iff its sahilm/fuzzy score is `> 0` (bonuses outweigh penalties). Rationale: upstream's threshold is a 0..1 normalized fuzzysort scale; sahilm/fuzzy v0.1.3 exposes no threshold parameter and its integer score (first-char/camel/separator/adjacent bonuses minus unmatched/leading-char penalties) is not isomorphic. The gate is the sanctioned approximation of 0.5 — weak scattered subsequence matches are filtered, strong prefix/separator matches pass. Where the two libraries' scales diverge, the yolo gate wins (it is the yolo port). Planned entry 5 (§4.1). |
| `scoreFn` prefix ×2 | kept as today: `strings.HasPrefix(path, query)` (mention.go:172-173). Upstream's check is against the `@`-prefixed display (`target.startsWith("@" + query)`); for files it never fires upstream (files are FFF-pre-ranked, never fuzzysorted) — yolo's path-target prefix ×2 is the sanctioned port, unchanged. |
| `scoreFn` ×(1 + frecency) | kept as today: `1 + frecencyScore` (mention.go:175; the store, §2) — the frecency factor of the bar is already live; decision 7 resolves to **reuse the store as-is** (no extension). |
| `limit: 10` | `maxPickerOptions = 10` (mention.go:21), unchanged. |
| selection reset on query change (:527-530) | **ported**: `sel` → 0 whenever the `@`-query changes (fixes the stale-selection edge; the render's `i == pm.sel` and the enter guard `sel < len(opts)` currently miss when the list shrinks below `sel`). |
| empty query → full list (upstream: unsorted non-file + up to 20 FFF files) | yolo's analog, unchanged: all walked files + directories in frecency order (frecency-zero entries keep walk order), capped 10. |

The empty-query ordering and the non-empty ranking compose: score = fuzzy
score (gated `> 0`) × prefix-boost × (1 + frecency), stable-sorted desc — the
current `mentionOptions` shape (mention.go:168-186) with the gate and the
directory candidates added.

### 3.5 — Selection model

- `up`/`down` (arrows — `homeKeyMap.Up`/`.Down`, home.go:24-25; shared
  bindings): move the selection **with wraparound** (`moveMenuSel`,
  prompt.go:250-257) — the current behavior (keys.go:222-227), unchanged.
  Upstream's `ctrl+p`/`ctrl+n` alternates (keymap.go:219-220 registry
  entries) are **not bound** — deviation 211's posture (prompt keys off the
  registry), no change.
- `enter`: insert the selected part (§3.6), no-op when the list is empty
  (the current guard, keys.go:229-235 — the upstream `if (!selected) return`,
  autocomplete.tsx:554-556).
- `sel` → 0 on query change (§3.4) and on directory expansion (§3.6).

### 3.6 — Insert semantics (locked decision 2; the deviation-222 class
restated)

`acInsert` (mention.go:193-207) is reworked to the upstream insertPart text
semantics (autocomplete.tsx:172-240):

- **File option** (enter / tab / mouse click): the `@`-query range
  (trigger + query — the current `mentionTriggerIndex` + `acQuery` range) is
  replaced with **`@<path>`**, and a **trailing space is appended only if the
  character immediately after the replaced range is not already a space**
  (upstream: `needsSpace = charAfterCursor !== " "`, :179-185). The prefix
  before the `@` and any text after the range are preserved. In the current
  value-derived trigger model the query reaches the end of the input, so the
  trailing space is always appended. Example: `fix the bug @app` → select
  `internal/tui/app.go` → `fix the bug @internal/tui/app.go `. The menu
  closes (the inserted space invalidates the trigger — the value-derived
  analog of upstream's `hide()` + `onSelect`, :553-558).
- **Directory option + `tab`** (upstream `complete` → `expandDirectory`,
  :560-579, 619-632): the range is replaced with **`@<path>/`** (no trailing
  space), the menu **stays open** with **`sel` → 0** — the new query
  (`path/`) re-filters the candidates to that directory's subtree (the walk
  is slash-relative, so the fuzzy target set narrows naturally).
- **Directory option + `enter` / mouse click** (upstream `select` on a
  directory = `hide` + `insertPart`, no expansion): insert `@<path> ` (the
  file semantics — trailing space) and close the menu.
- The insert remains **plain text** into the single-line string prompt — the
  virtual extmark / file-part / chip styling is not ported (the
  deviation-222 class; planned entry 3, §4.1).
- **Frecency touch** (decision D, mirroring the slash spec's touch-on-action
  policy): touched **on part insert only** — the `@`-insert paths (enter /
  tab-on-file / mouse-click-on-file) call `updateFrecency` + `saveFrecency`
  as today (mention.go:205-206; the upstream referent, autocomplete.tsx:237-
  239, is the touch inside `insertPart`). **Not** touched on directory
  expansion (upstream's `expandDirectory` bypasses `insertPart`), on hover,
  or on `esc`.

### 3.7 — Empty-state text

`No matching items` (upstream verbatim, textMuted — autocomplete.tsx:691),
replacing today's `no match` (prompt.go:227). The `@` picker renders its own
empty line (the slash menu's `No matching commands`, the slash spec's
planned entry 4, §4.1, covers the slash only) — planned entry 6 (§4.1).

### 3.8 — Mouse (locked decision 6; cross-ref: slash spec S5, the mouse slice)

Cell-based, the same model as the slash spec: the app-level
`tea.WithMouseCellMotion()` (not yet on — the shared prerequisite the slash
S5 lands on the Program options, main.go:768) + `MouseMsg` handling in the
`@` path + row hit-testing over the dropdown rows. **Hover** (mouse move
over a row) = move the selection (`sel` → the row, the upstream
`onMouseOver` → `moveTo`, autocomplete.tsx:642-645); **click** (release over
a row) = the **enter action** — insert the selected part (`@<path> ` +
close; the upstream `onMouseUp` → `select()`, :646-649 — note the upstream
click is `select`, not `complete`: a directory row clicked inserts
`@<path> ` and closes, it does **not** expand; only `tab` expands). No drag
affordance (the slash spec's S4 scope: hover + click; the upstream drag =
hover + click-select, which the cell model already reproduces).

### 3.9 — `@`-precedence rule (parity item)

A value can satisfy both triggers (e.g. `/cmd @q` starts with `/` and carries
a valid `@`-trigger) — today **both menus render** (view.go:33 + :36 are
independent) and the ladder feeds the slash menu first (keys.go:41-43 before
:47-49). Upstream keeps a single `visible` field and checks the `@`-trigger
**before** the `/` check (autocomplete.tsx onInput), so the `@` menu wins.
**Port: when a valid `@`-trigger is present, the `@` menu is the only menu
open** — `slashActive` is suppressed for the render and the ladder
(`menuItems` returns nil while `mentionActive`; the `@` path owns the keys).
One-line gate; no deviation (it is the upstream behavior).

### 3.10 — Key behavior table

Menu open = a valid `@`-trigger is present (the picker renders, including
the `No matching items` empty state). Must agree with the slash spec's table
(its §3.2) on every shared row: `up`/`down` (move selection, wrap), `shift+tab`
(cycle agent reverse, unchanged), `tab`-closed (cycle agent, keymap.go:135),
any-other-key (feed the input, re-filter).

| input | `@` menu open | `@` menu closed |
|---|---|---|
| `up` / `down` | move selection, wraparound (keys.go:222-227) | normal input |
| `enter` | insert the selected part — file: `@<path> ` + close; directory: `@<path> ` + close (enter = the insert action, the upstream `select`); no-op when the list is empty (keys.go:229-235, reworked per §3.6) | normal (send / route keys) |
| `tab` | **complete the selected option (new — the S5 intercept, upstream `complete`)**: file → `@<path> ` + close (trailing space per §3.6); directory → expand to `@<path>/` + keep the menu open + `sel` → 0 | cycle agent (`agent_cycle`, keymap.go:135 — unchanged; the menu handler intercepts tab only while the `@` menu is active, the slash spec §3.2's tab-intercept idiom, its S6) |
| `shift+tab` | cycle agent reverse (unchanged — the intercept is `tab` only) | cycle agent reverse (unchanged) |
| `esc` | **strip the `@`-trigger + query, keep the prefix**; cursor at the strip point; `sel` → 0 (keys.go:236-243 — the current behavior, unchanged; the 259-class deviation from upstream's pure hide, planned entry 4, §4.1) | normal (route keys — home esc-exit / shell-mode exit, etc.) |
| any other key | feed the input, re-filter, `sel` → 0 on query change (keys.go:244-245) | normal |

**The home hint** (slash spec S6's context-aware idiom, extended): the
`tab agents` segment (home.go:371-372) becomes `tab complete` while **either**
menu is open (`slashActive || mentionActive`), else `tab agents`. Whichever
epic lands first introduces the check; the other extends it.

## 4. Deviations

### 4.1 Planned `DEVIATIONS.md` entries

The slash spec plans 4 entries at next-free **308–311** (its §4.1; the
current
tail is **307** — verified at draft time, DEVIATIONS.md ends at entry 307).
The entries below land **after** the slash spec's, at the next-free numbers at
implementation time — expected **312–317** (six entries); the numbers are
confirmed against the live tail when they land (other work may land in
between).

1. **0.10.0 `@` mention picker: the TUI-local capped walk re-affirmed
   (deviation 225 re-scoped) (behavior/low)** — the candidate source stays the
   TUI-local walk (mention.go:16-127): the core server has no `fs.find`
   surface (frozen wire, deviation 225) and adding one is a core-server change
   beyond the TUI parity bar (the bar is visual + behavioral and does not name
   a source). Re-scoped: the walk now also collects directories (pre-order,
   alongside files; the `maxWalkFiles = 1000` cap keeps counting file
   entries; `maxWalkDepth = 8` unchanged) for the tab-expand. Deviation 225's
   source decision is re-affirmed, not superseded.
2. **0.10.0 `@` mention picker: files + directories only — the
   agent/MCP/reference candidates are not ported (259-class, behavior/low)**
   — upstream `@` also offers agents (autocomplete.tsx:402-421), MCP
   resources (:423-438), and reference aliases (:439-446). Yolo ports none:
   the send wire is `{text, files:[...]}` with no per-message agent part
   (deviations 296/303; agent selection is session-level — the pending agent
   → `session.agent`), yolo is zero-MCP (tips.go:60; the `mcp_list` binding
   is ported-but-inert), and there is no project-reference surface. The
   intentional port scope, the 259 parity-gap class, consistent with the
   slash spec's command-set-subset posture.
3. **0.10.0 `@` mention picker: the insert is plain text `@<path> ` (the
   deviation-222 class re-stated) + the directory expand (behavior/low)** —
   locked decision 2: `tab`/`enter`/mouse-click replace the `@`-query range
   with `@<path>` + a trailing space only if the character after the replaced
   range is not a space (upstream `insertPart`, autocomplete.tsx:179-185; in
   the value-derived trigger model the space is always appended); a directory
   option + `tab` expands to `@<path>/` and keeps the menu open with `sel` → 0
   (upstream `expandDirectory`, :560-579). The insert remains a plain string
   into the single-line prompt — the virtual extmark / file part / chip is not
   ported (deviation 222's class: no parts mechanism); the inserted text now
   matches upstream's `insertPart` append string, superseding the prior
   bare-path insert (mention.go:193-207).
4. **0.10.0 `@` menu esc: strip the `@`-trigger + query, keep the prefix
   (259-class, behavior/low)** — locked decision 3: yolo's current `esc`
   (keys.go:236-243) strips the trigger + query and keeps the prefix;
   upstream's pure hide (in `@` mode `hide()` only sets `visible = false`, the
   input unchanged — autocomplete.tsx:650-661) is not adopted: yolo's menu is
   value-derived (no independent visible state), so a pure hide would leave a
   dangling `@query` with no menu to re-open it. Logged as a parity-gap
   (259 class), the same posture as the slash spec's esc note.
5. **0.10.0 `@`-filter threshold 0.5: ported as the positive-fuzzy-score
   gate (behavior/low)** — upstream's `fuzzysort` `threshold: 0.5`
   (autocomplete.tsx:502-511) is a 0..1 normalized scale; sahilm/fuzzy
   v0.1.3 exposes no threshold parameter and its integer score is not
   isomorphic. Port (per spec §3.4): keep a match iff its fuzzy score is
   `> 0` — the sanctioned approximation of 0.5 (weak scattered subsequence
   matches filtered; strong prefix/separator matches kept); where the two
   scales diverge, the yolo gate wins. The limit 10 and the `scoreFn`
   bonuses (prefix ×2, frecency ×(1+f)) port as-is (mention.go:170-180).
6. **0.10.0 `@`-picker empty-state text (render/cosmetic)** — the `@`-picker
   empty line is `No matching items` (upstream verbatim, textMuted —
   autocomplete.tsx:691), replacing `no match` (prompt.go:227). Mirrors the
   slash spec's planned empty-text entry (which covers the slash menu's
   `No matching commands` only).

### 4.2 Affirmations (no new entry — restated so the implementation audit
starts from the right place)

Deviation 225 (the TUI-local source —
re-affirmed, re-scoped per entry 1), deviation 223 (the frecency KV
persistence — unchanged, the store is reused as-is), deviation 224 (the
scope-relative frecency key — unchanged), deviation 230 (the minimal gitignore
parse — unchanged), deviation 211 (the prompt keys off the registry —
unchanged; the `prompt.autocomplete.*` entries stay unbound, keymap.go:
219-223), deviation 289 (the `tab`/`shift+tab` agent-cycle — the `@`
menu's `tab` intercept is the same class of change the slash spec logs
against 289; `shift+tab` stays the agent-cycle, no change).

## 5. Re-baselines (tests / pins / mock)

**Tests** (file:line; the `@`-picker legs of the yolo inventory):

- `TestAcInsert` (mention_test.go:79) — the insert semantics change (bare
  path → `@<path> ` + trailing space; the directory-expand branch is new):
  re-baseline + extend.
- `TestMentionOptions` (mention_test.go:61) — the ranking changes (the
  positive-score threshold gate; directory candidates in the pool; the
  empty-query pool now includes directories): re-baseline + extend.
- `TestWalkFiles` (mention_test.go:40) — the walk now collects directories
  (pre-order, alongside files): re-baseline + extend.
- `TestMentionTriggerIndex` (mention_test.go:18) — the trigger rule is
  unchanged: verify green, no re-baseline.
- **Clean-home legs** (`TestHomeViewFrame` homeview_test.go:47,
  `TestHomeFrameSGR` home_golden_test.go:43, `TestHomeBoxRightEdge`, the
  `TestHomeViewFits/Clamps/Overflow` legs) — pin the *clean* home (no `@`
  menu open); the anchor work changes only the open-menu frame: verify green,
  no re-baseline.
- `TestTUIAtPicker` (mention_test.go:94) — the teatest substrings (`@a`,
  `alpha.go`) hold under the new chrome and insert (`@alpha.go ` contains
  `alpha.go`); the bordered-dropdown frame may shift the assertions — verify;
  if the chrome changes the emitted rows, add the chrome assertions (the
  slash S2 idiom) and re-baseline.
- **New** — the `@` wrap leg in the overflow suite (the `TestMenuViewWraps`
  idiom, overflow_test.go:49 — the width-20 wrapped long path + the muted
  description column; the slash S2's open-menu-box unit-test idiom).
- **New** — the mock render test (the `TestHomeMockRender` idiom,
  home_mock_test.go:178) that writes the mention-open mock (§ below).
- **New** — the key/mouse legs (the slash S5/S6 idiom): `tab`-file insert,
  `tab`-directory expand (menu stays open, `sel` → 0), `enter`-directory
  insert + close, the `@`-precedence rule (`/cmd @q` → `@` menu only), the
  mouse hover + click legs.

**Pins** — the `@` spec touches no pinned fixture: the 4 sha256 pins (root
principle 3) are `TestLogoBlockPinned` (logo_test.go:31), `TestTipsPinned`
(tips_test.go:30), `TestDescPinned` (tool/read_test.go:197), and
`TestParityFixturesPinned` (parity_fixture_test.go:34 — the frozen upstream
reference fixtures, incl. the `prompt-mention` surface, +
`MANIFEST.json`/`catalog-pin.json`/`canned.json`). They pin static content or
the frozen upstream reference → **none is re-baselined by the yolo menu work**
(the clean home mock `home-mock-200x50.txt` is likewise unchanged — the clean
home has no `@` menu open). Verify all four green (the same no-re-baseline
posture as the slash spec §5).

**Mock (decision E)** — a **third** mock file,
`docs/superpowers/mockups/home-mock-200x50-mention-open.txt`, alongside the
clean mock and the slash spec's second file
(`home-mock-200x50-slash-open.txt`). Generated the same way: the
`TestHomeMockRender` idiom (hand-assembled 200×50 SGR frame from the resolved
theme tokens, home_mock_test.go:178-350; the new test writes the file).
Content: the home with the `@` picker open **above the box's top edge** (at
`boxL`, width `boxW`): split L/R border (`theme.border`), `backgroundMenu`
fill, a few deterministic file rows (the fixture temp dir: `alpha.go`,
`beta.go`, plus a `cmd/` directory row with its trailing-`/` marker), a
**selected row** (`primary` bg + `selectedForeground` — the adaptive-contrast
rule, theme/styles.go:77-97), the muted description column, the box below,
the hint line **`tab complete  ctrl+p commands`** (the context-aware hint —
the `@` menu open), the tip row. Human-readable reference, not a sha256 pin
(the slash-open mock's treatment).

## 6. Work slices

Six slices, mirroring the slash spec's S1–S7 idiom (its seven slices serve
the slash menu's run-on-enter; the `@` picker's enter-is-insert folds into
S3). **Order constraints: S1 needs the slash epic's S1 (`dropdown.go`)
first; the re-baselines, the mock, and the DEVIATIONS land last (S6).**

1. **S1 — the `@` picker chrome onto `dropdown.go`.** Render the `@` menu
   through `dropdownView`/`dropdownRow` (the slash S1 primitive; chrome +
   row content per §3.1, anchors per §3.2): home — the dropdown anchored
   above the box's top edge at `boxL`/`boxW`, replacing the overlay lines
   (home.go:515-517); session — the current placement (view.go:46-52) with
   the new chrome. The `@`-precedence gate (§3.9): `menuItems` nil while
   `mentionActive`. Acceptance: the `@` menu renders the slash S1's chrome
   (a `dropdownView` leg in the teatest suite, the slash S2 idiom); the home
   anchor is above the box edge (frame assertion); the precedence gate holds
   (`/cmd @q` → `@` menu only); the pins stay green.
2. **S2 — candidate source / filter / ranking.** The walk collects
   directories alongside files (pre-order; the caps unchanged, §3.3). The
   filter: the positive-fuzzy-score threshold gate (the threshold-0.5 port,
   §3.4); the keys = the slash-relative path; the prefix ×2 and the
   frecency ×(1+f) kept (mention.go:170-180); the cap 10 (mention.go:21).
   `sel` → 0 on query change (§3.4). Empty query = all files + directories in
   frecency order (capped 10). The `mentionOption{path, isDir}` value
   carrier. Acceptance: `TestWalkFiles`/`TestMentionOptions` re-baselined
   (the directory rows, the gate, the reset); a weak scattered subsequence
   match is filtered where a prefix match passes (the gate leg).
3. **S3 — insert semantics + frecency touch.** `acInsert` reworked per
   §3.6: `@<path> ` + the trailing-space rule (the character after the
   replaced range); the directory + `tab` expand (`@<path>/` + menu open +
   `sel` → 0); the directory + `enter`/click insert (`@<path> ` + close).
   Frecency touch on part insert only (the `updateFrecency` + `saveFrecency`
   pair, mention.go:205-206); no touch on expand/hover/esc. Acceptance:
   `TestAcInsert` re-baselined (the `@`-prefixed insert, the space rule, the
   expand branch); the touch legs (insert touches; expand does not).
4. **S4 — mouse.** The `@`-path mouse handling on the app-level
   `WithMouseCellMotion()` (main.go:768 — the shared S5 prerequisite):
   row hit-testing over the dropdown rows; hover = move the selection;
   click = the enter action (insert `@<path> ` + close — no expand on
   click, §3.8). The `a.mouseInput` semantics mirror the slash S5.
   Acceptance: the hover leg (hover over row
   N → `sel == N` → the next enter inserts N); the click leg (click a row →
   the insert appears in the drained output, the menu closes).
5. **S5 — keys + hint interaction.** The `tab` intercept in the `@` path
   (the ladder: the `@` handler owns `tab` while the `@` menu is active,
   before `handleAppKeys`'s `agent_cycle` — the slash S6's tab-intercept
   idiom, keys.go:33-41); `tab` = the complete action (§3.6: file insert /
   directory expand); `shift+tab` unchanged; `enter` / `esc` / `up`/`down`
   per §3.10 (esc unchanged — the strip, keys.go:236-243). The home hint:
   `tab complete` while either menu is open (`slashActive || mentionActive`),
   else `tab agents` (home.go:371-372; the slash S6 idiom extended).
   Acceptance: the table legs of §3.10 (the `tab` file/directory legs, the
   closed-`tab` agent-cycle, the hint flip).
6. **S6 — empty text + re-baselines + mock + DEVIATIONS (last).** The empty
   line → `No matching items` (prompt.go:227). All the re-baselines of §5
   (the test legs, the `acView` wrap leg, the `TestParityFixturesPinned`
   green verify). The third mock
   (`home-mock-200x50-mention-open.txt`, §5). The DEVIATIONS entries (§4.1,
   the live-tail numbers) + the PROGRESS.md facts (the `@`-picker 0.10.0
   parity line: the dropdown chrome + mouse + `tab`-complete + the threshold-
   0.5 gate + the candidate-set/source affirmations, the deviation count).
   Acceptance: `go vet ./... && go test ./...` green, `gofmt -l .` empty;
   the DEVIATIONS entries land at the next-free numbers; the mock renders
   deterministically (re-running the test reproduces the file byte-for-byte).

## 7. Open questions / out of scope

- The **command palette** (`yolo-old.6`) and the **slash epic**
  (`yolo-old.4`) — separate wayfinder tickets; this spec consumes the slash
  epic's S1 primitive and extends its S3/S6 shared slices, it does not
  re-scope them.
- The **epic-structure decision** (the map: after the three specs land) —
  the dependency fact is recorded above; the epic shape is not decided here.
- The **styled file parts / extmarks** (the upstream chip rendering inside
  the input) — the deviation-222 class, out of scope (the insert is plain
  text, §4.1 entry 3).
- The **server-side FFF / `fs.find` endpoint** — if the user later wants
  source parity (upstream's server-ranked file search), that is a core-server
  work item beyond the TUI parity bar; the bar does not require it (the
  source deviation, §4.1 entry 1).
- The **`ctrl+p`/`ctrl+n` autocomplete alternates** (upstream defaults,
  registered unbound at keymap.go:219-220) — deviation 211's posture; not
  bound, no change.
- The **slash menu's `sel`-reset edge on query change** (the shared pre-
  existing edge; the `@` path ports the reset, the slash path stays the
  slash epic's) — a possible slash-epic follow-up, noted, not specced here.
- The **mode-independence of the `@` trigger** (no shell-mode gate — the
  current behavior; the sibling slash spec's §7 note on this is inaccurate,
  §2) — out of scope, no change.
