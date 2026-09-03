# S6 — home completion (slice bead `yolo-oae.7`)

Complete the home route: the startup spinner, the rotating sha256-pinned
tips, the session-destination view, and the footer key hints rendered from
the S4 keymap registry.

**State: fully detailed** — the 5-step TDD detail for all 5 tasks is in the
`## S6 detail` section below (Slice Detail Protocol rule 2); execution may
start at task S6.1.

## Binding task table

Pointer only (FROZEN — Slice Detail Protocol rule 1): `plan.md` →
`## Task inventory` → `### S6 — home completion (slice bead yolo-oae.7)`.
Bead titles, scope, and pinned commit messages live there and may not be
changed by a detail pass; any required change = STOP + explicit user
approval + re-record in plan.md.

## Dep gate

None.

## Exact upstream sources (the detail pass reads these AT DETAIL TIME)

`/tmp/opencode-upstream` @ v1.18.18:

- `packages/tui/src/routes/home.tsx` — the home route: startup state +
  session list (S6.1/S6.4 context).
- `packages/tui/src/routes/home/session-destination.tsx` — S6.4.
- `packages/tui/src/feature-plugins/home/tips.tsx` — the tips DATA (S6.2 —
  sha256-pinned port, root principle 3).
- `packages/tui/src/feature-plugins/home/tips-view.tsx` — rotation cadence
  + rendering (S6.3).
- `packages/tui/src/component/startup-loading.tsx` — the spinner (S6.1).
- `packages/tui/src/feature-plugins/home/footer.tsx` — the footer key
  hints (S6.5 — S0.9 already ported the token map; S6.5 renders hints FROM
  the S4 keymap registry — cross-slice dependency: S4.1–S4.2 must be
  closed first; the strict slice order (S6 runs only after S4 closes)
  satisfies it).

## yolo anchors

- `internal/tui/home.go` — the home route surface.
- `internal/tui/footer.go` — the footer key-hints surface (S6.5).
- the S4 keymap registry — cross-slice: S6.5 renders the hints from it.
- `internal/protocol/` — the sessions DTO for the destination view.

## Detail pass (protocol)

One writing-plans pass, one subagent, `thinking=high`, dispatched by the
root session strictly sequentially (root principle 7) — per the Slice
Detail Protocol in plan.md: it fills this file (after this section) with the
full 5-step TDD detail for each task in the binding table (failing test
code, implementation code, gate, pinned commit), reading the named upstream
files at that moment. It commits as
`docs: TUI parity plan — detail S6 tasks` on its own bead
(`bd create "detail S6 plan tasks" --parent=yolo-oae.7 --json`).

## S6 detail

Detail pass 2026-09-03. Deviations tail at detail time = 232; S6 entries
start at 233. Breadcrumb note (DEVIATIONS.md entry 233, severity info): the
frozen S6 table names the task beads `yolo-oae.7.1`–`7.5`, but the S6 detail
bead consumed `yolo-oae.7.1` (created + claimed before the detail pass; the
S1 "detail-bead-last" precedent is impossible because the detail pass
precedes slice start, as in S2/dev 165, S3/dev 188, S4/dev 206, S5/dev 221).
The 5 task beads therefore land in table order at `yolo-oae.7.2`–
`yolo-oae.7.6` (S6.1→.2, S6.2→.3, S6.3→.4, S6.4→.5, S6.5→.6); the frozen
titles and pinned commit messages are unchanged. No code or wire impact.

### Detail-pass findings (read AT DETAIL TIME, 2026-09-03 — binding)

1. **Upstream home route + session destination** (`home.tsx`,
   `session-destination.tsx`, `footer.tsx`, `runtime.tsx` @ v1.18.18):
   `home.tsx` composes the logo slot + prompt (maxWidth 75) + the
   `home_bottom` slot (tips plugin + which-key HomeHint) + the `home_footer`
   slot (footer plugin) — the upstream home has NO session list (the yolo
   list is the yolo-specific S0.9 port). `session-destination.tsx` (41 L):
   `HomeSessionDestination = {type:"directory", directory, subdirectory} |
   {type:"new"}`; the context memo = selected ?? `{directory:
   sync.path.directory || cwd, subdirectory: false}` + `setDestination` +
   `clear`; consumers = the footer Directory segment + `prompt/move.tsx`
   (the move/worktree flow — **no yolo wire referent**). `footer.tsx`: the
   home footer = [Directory] [Mcp] [Version]; Directory =
   `abbreviateHome(dir, paths.home)` + `:branch` when `dir === cwd`
   (textMuted) — **no key hints in the upstream home footer** (the yolo spec
   redefines the home footer as the key-hints line). `runtime.tsx:3-10`
   `abbreviateHome(input, home)`: home empty → input; `rel = path.relative`
   → `""` ⇒ `"~"`, starts-with-`..`/absolute ⇒ input, else `"~/" + rel`.
2. **Upstream tips** (`tips.tsx` + `tips-view.tsx` @ v1.18.18): the tips
   DATA lives in `tips-view.tsx` (NOT `tips.tsx` — the brief's pointer):
   `themeCount` (L6, the built-in theme count), `parse()` (L47-66, the
   `{highlight}…{/highlight}` split: odd capture parts highlighted),
   `NO_MODELS_TIP` (L71, verbatim below), `TIPS` (L164-283, **99 entries** —
   static strings + `(shortcuts) => …` dynamic templates, `press()` /
   `commandText()` / `shortcutText()` helpers), and the two platform
   conditionals `INPUT_UNDO_TIP` (win32) + `TERMINAL_SUSPEND_TIP` (L285-287).
   `tips.tsx` (the plugin wrapper) owns the VISIBILITY: `hidden` = the
   `tips_hidden` KV flag toggled by the `tips.toggle` palette command
   (default binding `<leader>h`, `keybind.ts:225`); `first` =
   `session.count() === 0`; `connected` = some provider `id !== "opencode"`
   OR with a model `cost?.input !== 0`; `show = (!first || !connected) &&
   !hidden` (L40-47). **The "rotation" is a one-shot random pick per
   component mount — NO timer**: `tipOffset = Math.random()` at creation,
   `tip = tips[Math.floor(tipOffset * tips.length)] ?? NO_MODELS_TIP`, and
   `connected === false` forces `NO_MODELS_TIP`. Re-pick happens on every
   home-route mount. Render: `● Tip ` prefix in `theme.warning` + the
   word-wrapped parts (highlight parts `theme.text`, the rest
   `theme.textMuted`).
3. **Upstream startup loading** (`startup-loading.tsx` @ v1.18.18): the
   state machine over `ready`: a 500 ms `wait` before the first show
   (flash-avoidance); when `ready` arrives: if already shown and
   `elapsed >= 3000` → hide immediately, else a `hold` timer of
   `3000 - elapsed` (the shown span is always ≥ 3000 ms once shown). Text:
   "Loading plugins..." (not ready) / "Finishing startup..." (ready, L8).
   Render: absolute bottom-center box (bottom 1), `backgroundPanel` padding
   1, the Spinner in `textMuted`.
4. **Upstream which-key HomeHint** (`which-key.tsx:176-181` @ v1.18.18): the
   `home_bottom` line "Show keyboard shortcuts with <trigger>" — muted text,
   the trigger span in `subtle` (borderSubtle), `wrapMode="none"`, maxWidth
   75 centered, paddingTop 1; trigger = the `which_key_toggle` command
   shortcut (default `ctrl+alt+k`, `keybind.ts:229`), falling back to the
   command name "toggle" when empty.
5. **yolo surface (verified at detail time):**
   - `home.go`: `homeModel{cursor, now}` (L20-23); `homeKeyMap` (L25-37,
     the home-surface keys — NOT registry bindings, deviation 210);
     `helpText` const (L102); `render` → `renderClamped(s, w, th, maxRows)`
     (L104-134: logo + "New session" + rows + `BorderSubtle` divider +
     `dimWrapped(th, helpText, w)` — the last line); the tagged-word wrap
     machinery `rowLines`/`wTag`/`joinRowLine`/`writeRowLine` (L153-323);
     `handleHomeKey` (L328), `homeEnter` (L347). Render call sites:
     `view.go:44` (home route) + `view.go:180` (the modal clamp — the
     clamped output truncates over-tall chrome at `panelTop`; the new bottom
     lines are inside the clamped render and truncate under modals —
     accepted, the modal owns the frame; the `modalChromeMin` home count
     `4+1+1+help` is unchanged).
   - `app.go`: `App` fields (`engine *theme.Engine`, `theme theme.Theme`,
     `keymap *Keymap`, `spinIdx`, …); `NewApp` (L113: `homeModel{now:
     nowMillis}` L122, tail `retheme(); loadHistory(); loadFrecency()`
     L148-150); `Update` → `updateMsg` (L192); the `spinMsg` case
     (L249-254: `a.spinIdx++`; re-arms `a.spinTick()` when
     `a.statusSeg() != ""`); `loadHistory`/`saveHistory` (L409-423, the KV
     pattern: `a.engine.KV().Get(kvHistoryKey, nil)` / `.Set(…)`, nil-engine
     skips); `kvHistoryKey`/`kvFrecencyKey` consts (L385/427).
   - `hydrate.go`: `applyHydrate` (L89): `case m.notFound` → lastErr + home
     + `quitCmd()`; `case m.err` → lastErr + nil; `case a.route ==
     routeSession` → store updates + **nil** (L114); `default:` (home) →
     store updates, falls through to the shared **nil** return — the two
     success paths are the `loadDone` hook sites.
   - `view.go`: `view()` (L25) — home route L43-44, the `lastErr` block
     L64-67, the footer `b.WriteString("\n" + a.footerView())` L73 — the
     loading line slots between the `lastErr` block and the footer.
     `viewModal` (L153): home chrome = `a.home.renderClamped(…,
     panelTop-4-1-1-help)` L180.
   - `keys.go`: `dispatchCommand(name) []tea.Cmd` (L127: the cases
     `command_list`/`app_exit`/`model_list`/`agent_list`/`status_view`/
     `theme_list`/`session_new`/`session_list`/`provider_connect`/
     `help_show`; the which_key_* case is consumed-but-inert, deviation
     207) — the `tips_toggle` case site. `matchLeaderContinuation` /
     `matchBase` iterate `contextGroups[BaseMode]` (in order).
   - `keymap.go`: `Definitions` (L49, the package-level default map —
     token-integrity referent); `Format(name)` (L766: "none" when disabled,
     else the formatted seqs joined by `formatJoin = " / "` L656);
     `leaderDisplay()` (L778: the leader's first seq; default
     `ctrl+x`); `Set(name, BindingValue) error` (L721); `Seqs(name)`
     (L735); `contextGroups[BaseMode]` (L818-822) — **`tips_toggle` is
     ABSENT** (S6.3 appends it at the END). Defaults used by the tips:
     `command_list ctrl+p`, `model_list <leader>m`, `theme_list
     <leader>t`, `status_view <leader>s`, `session_new <leader>n`,
     `session_list <leader>l`, `session_rename ctrl+r`,
     `session_interrupt escape` (displays `esc`),
     `messages_page_up pageup,ctrl+alt+b`, `messages_page_down
     pagedown,ctrl+alt+f`, `prompt_soft_newline "\+enter"` (byte-identical
     display, deviation 208), `tips_toggle <leader>h` (L230; CommandMap
     `tips_toggle → tips.toggle` L402), `leader ctrl+x` (L50).
   - `footer.go`: `footerFrames` (5 braille frames), `spinMsg`/`spinTick()`
     (100 ms), `spinFrame()`, `statusSeg()`, `footerView()`.
   - `store.go` L15: `State{Sessions []protocol.Session; …; Providers
     []protocol.Provider; …}`. `protocol`: `Provider{ID string; Models
     map[string]Model}` (provider.go:34-40); `Model.Cost ModelCost`
     (L21-32); `ModelCost{Input float64; …}` (L14-19).
   - `theme`: `AllThemes() (map[string]ThemeJson, error)` (theme.go:27,
     the 33 embedded assets); style accessors `Text/TextMuted/Warning/
     BorderSubtle/BackgroundPanel/…` (styles.go:99-115); the lipgloss v2
     chain `.Padding(0, 1).Width(w).Background(c).Render(s)` (the codebase
     idiom, dialog.go:322); the zero Theme paints nothing (S0.7).
   - `dimWrapped(th, s, w)` (dialog.go:812, the single-tone dim wrap);
     `wrapLine(s, w)` (wrap.go:18); `runeWidth` (wrap.go).
   - config/CLI referents (tips port): `{env:NAME}` substitution (config.go
     `envPat`); NO `{file:path}` / `$schema` / temperature / steps /
     per-agent tools / MCP / plugins; `Config{Model, Agent, Provider,
     Permission (pattern maps), Instructions, Theme ("system" honored),
     Keybinds, …}`; agents build/plan/yolo; the permission builtins
     `doom_loop` / `external_directory`; the tools bash/edit/glob/grep/read/
     write/todowrite; CLI subcommands `serve` / `auth list|add|…` /
     `profile …` / `version` (NO `run` / `--continue` / `debug` /
     `upgrade` / `agent create`); `YOLO_PRINT_LOGS=1` stderr mirror;
     providers `kido` (Qwen, keyless default) + `opencode` ("OpenCode
     Zen"); global config under `~/.config/yolo/`; project config
     `yolo.jsonc`.
   - Home-entry sites (the tip re-pick hooks, S6.3): `app.go:123` (NewApp
     initial `route: routeHome`), `hydrate.go:101` (notFound → home),
     `session.go:645` (esc when idle → home), `sessionsdlg.go:308` +
     `:329` (session delete → home).
   - test harness: `testApp(sessions ...protocol.Session) *recApp`
     (home_test.go:29 — dummy client `client.New("http://127.0.0.1:9", "")`
     ⇒ `Dir == ""`, nil engine, `home.now = testNow`); `newRecApp(c, s,
     startSessionID)` (rec_test.go:20 — `*recApp{*App, Cmds}`, `App.Update`
     promoted); `themeApp(t)` (themedlg_test.go:20, the real engine over a
     t.TempDir KV); `press(rune)` (home_test.go:36); `stripANSI`
     (home_test.go:20); `testNow` (home_test.go:24); `hasLine`
     (permission_test.go:256), `hasLines` (tui_suite_test.go:161),
     `suiteType` (tui_suite_test.go:27); `testutil.Boot(t)` (real in-process
     server; `ts.Dir` = the absolute scope dir) and `testutil.BootWithDriver`
     (provider catalog = `provider.NewStaticForTest()` ⇒ kido + opencode
     static ⇒ `connected == true` on a real boot); `refModel` (home_test.go).
     Pin pattern (root principle 3): `TestLogoBlockPinned` (logo_test.go:31
     — `sha256` of the canonical text vs the constant, "re-baseline the pin
     in the same commit" fatal message).
   - home render pins: `TestHomeRenderLockedLayout` (home_test.go:81 — the
     byte-exact layout over `logoPlainLines` + rows + divider + help line;
     L109 pins the help line) re-baselines as the bottom lines land (S6.3
     adds the `● Tip ` line for testApp — 0 sessions + 0 providers ⇒
     `first` + `!connected` ⇒ the `NO_MODELS_TIP` line shows; S6.5 adds the
     hint-only line — testApp `Dir == ""` ⇒ destination omitted; S6.4 adds
     nothing to testApp); `TestHomeRenderWraps` (overflow_test.go:99 — the
     fits-width + substring assertions; the new lines fit by
     construction). `home_theme_test.go:251` constructs `homeModel{cursor: 0}`
     directly (the `tips`/`footer` seams must be nil-guarded in
     `renderClamped` — that test only calls `renderRow`, which is
     seam-free).
   - "Show keyboard shortcuts" does NOT exist anywhere in the yolo code yet
     (the S4.7 design decision landed only the `paletteShortcut()` =
     `a.keymap.Format("command_list")` rewire at dialog.go:353 +
     `TestHelpPaletteHintFromRegistry`; the home hint is S6.5's).
6. **The tips port set (binding — deviation 234):** 37 entries (the
   upstream `TIPS` order, the referent survivors only) + `NO_MODELS_TIP`
   (verbatim); 64 of the upstream's 99 + 2 platform conditionals are
   dropped (no yolo referent). The exact adapted text is the Task S6.2 Step
   3 block; the drop disposition list is deviation 234.
7. **spec §6/§9:** the Home route line — the startup-loading spinner while
   hydrating; the session-destination view; the rotating tips (sha256 pin,
   principle 3); the home footer = the key hints from the keymap registry.
   No new §9 open items from this slice.

### Design decisions (binding)

**Shared home-bottom surface (lands per task, in order).** After the
divider + help line, the home bottom gains: the tips line (S6.3), then the
footer line (S6.4 destination + S6.5 hint, `" · "`-joined, empty parts
omitted). `homeModel` gets two injected render seams (the `now` pattern,
nil-guarded — the direct-construct tests rely on the guard): `tips func(w
int) string` (S6.3) + `footer func(w int) string` (S6.4), wired in
`NewApp` to `a.homeTipsLine` / `a.homeFooterLine`; `renderClamped` appends
each non-empty line after the help line (tips first, footer second). The
tips line word-wraps at `w` with the tagged-word scheme ported from
`rowLines` (the `● Tip ` prefix run + the alternating highlight/muted part
runs, runs merged per visual line in SEQUENCE — unlike `rowLine`'s
fixed-order buckets, the tip runs interleave, so the join merges
consecutive same-kind words); the footer line is the single-tone
`dimWrapped` (upstream's two-tone subtle trigger span is NOT ported — the
yolo home bottom lines are single-tone dim, the help-line convention).
`modalChromeMin` is unchanged (the bottom lines live inside the clamped
render; under a modal they truncate at `panelTop` — the modal owns the
frame).

**S6.1 (startup spinner):** `startup.go` owns the ported state machine:
fields `loadShown bool`, `loadReady bool`, `loadStamp time.Time` on `App`;
`loadShowMsg{}` / `loadDoneMsg{}` msgs; `loadArm()` (the 500 ms `tea.Tick`,
added to the `Init()` batch, nil when already ready); `loadDone()` (called
from the two `applyHydrate` success paths — hide immediately when never
shown, else hold `max(0, 3000ms - elapsed)` via `tea.Tick`, nil when
already ready); the `spinMsg` case re-arms `spinTick()` when `loadShown`
(the line re-uses the 100 ms footer frames via `spinFrame()`); the
`loadShowMsg` case arms `loadShown` + `loadStamp` + `spinTick()`, the
`loadDoneMsg` case hides. Render: `loadingView(w)` — home route only
(`a.route == routeHome`), `spinFrame() + " " + text`, `textMuted` inside a
`BackgroundPanel` `Padding(0, 1)` pill, centered at `w` (`(w - width)/2`
spaces; no centering when `w <= width`), inserted in `view()` between the
`lastErr` block and the footer line. Text: **not-ready "Loading
sessions..."/ready "Finishing startup..."** (deviation 237 — the upstream
"Loading plugins..." has no yolo referent; "Finishing startup..." is
verbatim). A hydrate ERROR keeps the spinner spinning (the line stays —
still not hydrated; a later successful hydrate, e.g. the resync pump, runs
`loadDone` — upstream-identical: `ready` stays false). The teatest leg is a
minimal boot smoke (the real boot hydrates faster than the 500 ms arm, so
the spinner is NOT observable there — NO absence assertions; the state
machine is unit-pinned with direct msg injection + stamp manipulation).

**S6.2 (tips data + pin):** `tips.go` = the 37-entry `tips []string` +
`noModelsTip` const (upstream verbatim) + `tipBindings []string` (the 12
`<binding>` tokens) + `themeCount` (the `theme.AllThemes()` count, 33) +
the `parseTip` port (the `tipPart{text, hi}` + the
`{highlight}(.+?)\{/highlight\}` regex split, upstream `parse()` L47-66).
Adaptations (deviation 234): product names (opencode→yolo,
opencode.json/tui.json→yolo.jsonc, ~/.config/opencode/→~/.config/yolo/,
.opencode/themes→.yolo/themes, `opencode serve`→`yolo serve`,
`--print-logs`→`YOLO_PRINT_LOGS=1`, webfetch→glob), referent drops (no
`!` shell, /undo//redo//share//init//compact//export//timeline//review,
image paste/drag-drop, MCP, plugins, worktree/parent-child, pin/quick-switch
(deviation 189), CLI surface, $schema/{file:}, formatters/LSP, share,
docker), the `/models`→`/model` rename, the `75+` qualifier drop, and the
`/rename` command → the `session_rename` keybind (yolo has no /rename
command; rename is the `ctrl+r` key). The dynamic tips carry the `<name>`
tokens INSIDE the `{highlight}` spans (the substitution inserts plain text
rendered bright — upstream-identical shape); the multi-sequence display is
`Format(name)` (" / "-joined — deviation 234 note; identical to upstream's
first-seq display under the default config, where every dynamic binding is
single-sequence). The pin (principle 3) = the sha256 of the canonical form
(`noModelsTip` first, then the 37 tips, each line + `"\n"`) recorded in
`wantTipsPinnedSHA256` — the constant is computed at Step 3 (the test
prints it) and re-baselined in the same commit (the
`TestLogoBlockPinned` pattern); the pin records THIS ported content, not
the upstream original. `parseTip` + the pin + the token-integrity + the
shape tests land in S6.2; the substitutions/render/visibility land in
S6.3.

**S6.3 (tips rotation + rendering):** `tipText()` = the token
substitution (`<name>` → `a.keymap.Format(name)` for the 12 `tipBindings`,
then `{theme_count}` → `strconv.Itoa(themeCount)`) of `tips[tipIdx %
len(tips)]`, or `noModelsTip` when `!tipsConnected()` (the upstream
`connected === false` force). Visibility (the upstream L40-47 port,
verbatim semantics): `tipsFirst()` = `len(a.store.Sessions) == 0`;
`tipsConnected()` = some provider `ID != "opencode"` OR with a model
`Cost.Input != 0` (the `store.Providers` + `protocol.ModelCost.Input`
referents); `tipsVisible()` = `!tipsHidden && (!tipsFirst() ||
!tipsConnected())`. Rotation cadence: the one-shot per-home-entry re-pick
— `tipIdx int` on `App` (seeded by `repickTip()` in `NewApp` and at the
four home-entry sites: `hydrate.go:101`, `session.go:645`,
`sessionsdlg.go:308`, `sessionsdlg.go:329`), `repickTip()` = `tipIdx = int(a.tipRand() *
float64(len(tips)))`, `tipRand func() float64` (default `math/rand.Float64`
— the test seam; NO timer, deviation 235 note — the upstream
`Math.random()` per mount is ported as the per-home-entry re-pick, which
is the yolo observable equivalent: every entry to the home route shows a
fresh random tip). `homeTipsLine(w)` = `""` when `!tipsVisible()`, else
the `● Tip ` prefix (theme `Warning`) + the `parseTip` parts (highlight →
`Text`, rest → `TextMuted`) wrapped at `w` by the ported tagged-word
scheme (`tipRun{text, kind}` — kind 0 prefix / 1 muted / 2 text —
`tipLines`/`joinTipLine`/`writeTipLine`, the `rowLines` port with
in-sequence run merging). `tipsHidden` persists over the theme KV
(`kvTipsHiddenKey = "tips_hidden"`, the S5.2 KV seam, deviation 223 —
`loadTipsHidden()` in `NewApp` after `loadFrecency`, nil-engine
in-memory): the `dispatchCommand` case `"tips_toggle"` flips the flag +
`KV().Set` (nil return — bubbletea re-renders after every Update), and
`tips_toggle` is APPENDED at the END of `contextGroups[BaseMode]` (the
`<leader>h` default then dispatches; the upstream palette ENTRY for the
command is not ported — the yolo palette lists the wire catalog + local
slash commands; the toggle is reachable via the keybind only, deviation
235). Test seams: `tipRand` (deterministic picks) + the `tipIdx` field
(direct selection); the teatest leg asserts the `● Tip ` prefix only (the
tip text is random) — presence after `n` → esc → home on the real boot
(providers present ⇒ `connected`, one session ⇒ `!first` ⇒ visible);
toggle + group wiring are unit-pinned (the teatest absence assertion is
forbidden by the buffer-drain rule).

**S6.4 (session destination):** `abbrevHome(dir, home)` = the pure port of
`runtime.tsx:3-10` (`filepath.Rel`: `""`/`"."` → `~` rules; outside-home /
relative / error → the raw input; `home == ""` or `dir == ""` → input).
`sessionDestination()` = `abbrevHome(a.Service.Dir, a.homeDir())` — the
scope directory IS the new-session destination (yolo has one scope; the
upstream selected-directory / "new" workspace union collapses to the scope
dir, deviation 236). `homeDir()` = `a.homeDirFunc()` when set (the test
seam; default `os.UserHomeDir`). When `a.Service.Dir == ""` (the TUI
cannot know the server work dir — testApp) the destination segment is
OMITTED. The `homeFooterLine(w)` = `""` when no part, else `dimWrapped(a.
theme, dest, w)` (S6.5 extends it to the parts join). The upstream
selection state machine (`setDestination`/`clear`/pending), the
move-session flow, and the `:branch` suffix are DROPPED (no yolo referent,
deviation 236). No teatest leg (the render is unit-pinned with an
injected `homeDirFunc` + a fixed `Dir`; the real-boot line is covered by
S6.5's leg which pins the hint suffix).

**S6.5 (footer key hints from the registry):** `homeShortcutsHint()` =
`"Show keyboard shortcuts with " + a.keymap.Format("leader")` — trigger =
the LEADER key, NOT the upstream `which_key_toggle` (inert in yolo,
deviation 207 — the yolo referent for "show keyboard shortcuts" is the
leader key that opens the which-key overlay), and `""` when the leader
binding is disabled (`Format == "none"` — the overlay is then unreachable;
upstream's fallback-to-command-name is not ported: a disabled binding
omits the line, deviation 238). `homeFooterLine(w)` becomes the parts
join: `[destination (S6.4), hint (S6.5)]` `" · "`-joined (the help-line
separator convention), `dimWrapped`, `""` when empty. The yolo-surface
home keys (up/down/enter/n) remain hardcoded in the help line (deviations
210/211) — the hint line is the registry-rendered part.
`TestHomeRenderLockedLayout` re-baselines at S6.5 (the testApp footer line
becomes the hint-only line `"Show keyboard shortcuts with ctrl+x"`); the
teatest leg pins the hint on the real boot (the destination prefix is the
unpredictable TempDir path — pinned only in the unit leg).

### Task S6.1: Home: startup-loading spinner while hydrating + tests (bead `yolo-oae.7.1`, expected id `yolo-oae.7.2`)

**Files:** new `internal/tui/startup.go` (the state machine + `loadingView`), modify `internal/tui/app.go` (the 3 fields, the `loadArm`/`loadDone`/`loadingText` methods, the two `updateMsg` cases, the `spinMsg` re-arm, the `Init()` batch), `internal/tui/hydrate.go` (the two `applyHydrate` success paths → `a.loadDone()`), `internal/tui/view.go` (the loading-line insertion), new `internal/tui/startup_test.go`.

**Interfaces:** `type loadShowMsg struct{}`, `type loadDoneMsg struct{}`, `const startupShowDelay = 500 * time.Millisecond`, `const startupMinHold = 3 * time.Second`, `const startupTextLoading = "Loading sessions..."`, `const startupTextReady = "Finishing startup..."`; `App.loadShown bool` / `App.loadReady bool` / `App.loadStamp time.Time`; `func (a *App) loadArm() tea.Cmd`, `func (a *App) loadDone() tea.Cmd`, `func (a *App) loadingText() string`, `func (a *App) loadingView(w int) string`.

**Upstream parity notes:** the 500 ms flash-avoidance wait + the ≥ 3000 ms
shown hold + the two-state text (startup-loading.tsx); the spinner frame
re-uses the yolo 100 ms footer frames (`spinFrame`, the S0.x braille
set) instead of the upstream Spinner component (the yolo home has one
spinner idiom); the render slot is a bottom line between `lastErr` and the
footer (the upstream absolute bottom-center box, minus the z-index /
absolute positioning — the yolo frame composes lines, not absolute
layers); the text adaptation is deviation 237.

**Step 1 — failing test** (`internal/tui/startup_test.go`):

```go
package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/bubbletea/v2"
	"github.com/kido5217/yolo/internal/tui/store"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/server/testutil"
)

// TestStartupLoadingStateMachine pins the ported state machine (upstream
// startup-loading.tsx: the 500 ms arming, the min-3 s hold once shown, the
// ready text swap, the no-op ready-twice).
func TestStartupLoadingStateMachine(t *testing.T) {
	a := testApp()
	if a.loadShown || a.loadReady {
		t.Fatal("fresh app must start unshown + unready")
	}
	a.Update(loadShowMsg{})
	if !a.loadShown {
		t.Fatal("loadShowMsg must arm the shown state")
	}
	if a.loadReady {
		t.Fatal("loadShowMsg must not mark ready")
	}
	if got := a.loadingText(); got != startupTextLoading {
		t.Fatalf("loading text = %q, want %q", got, startupTextLoading)
	}
	if got := a.loadDone(); got == nil {
		t.Fatal("ready while shown must return the hold tick")
	}
	if !a.loadShown {
		t.Fatal("the hold must keep the line shown")
	}
	if got := a.loadingText(); got != startupTextReady {
		t.Fatalf("ready text = %q, want %q", got, startupTextReady)
	}
	a.Update(loadDoneMsg{})
	if a.loadShown {
		t.Fatal("loadDoneMsg must hide the line")
	}
	if got := a.loadDone(); got != nil {
		t.Fatal("a second ready must be a no-op (nil)")
	}
}

// TestStartupLoadingReadyBeforeShow pins the fast-hydrate path: ready
// before the 500 ms tick fired ⇒ the line never shows; a late tick after
// ready is a no-op; an expired hold hides immediately.
func TestStartupLoadingReadyBeforeShow(t *testing.T) {
	a := testApp()
	if got := a.loadDone(); got != nil {
		t.Fatal("ready before show must return nil (no hold)")
	}
	if a.loadShown {
		t.Fatal("ready before show must leave the line unshown")
	}
	a.Update(loadShowMsg{})
	if a.loadShown {
		t.Fatal("loadShowMsg after ready must be ignored")
	}

	b := testApp()
	b.Update(loadShowMsg{})
	b.loadStamp = time.Now().Add(-startupMinHold - time.Second)
	if got := b.loadDone(); got != nil {
		t.Fatal("an expired hold must hide immediately (nil)")
	}
	if b.loadShown {
		t.Fatal("the expired hold must leave the line hidden")
	}
}

// TestStartupLoadingRender pins the home-only bottom line and its slot
// (after the help line content, before the footer line).
func TestStartupLoadingRender(t *testing.T) {
	a := testApp()
	a.Update(loadShowMsg{})
	if got := stripANSI(a.loadingView(80)); !strings.Contains(got, startupTextLoading) {
		t.Fatalf("loading line = %q, want %q", got, startupTextLoading)
	}
	got := stripANSI(a.view())
	if !strings.Contains(got, startupTextLoading) {
		t.Fatalf("home view missing the loading line:\n%s", got)
	}
	i := strings.Index(got, startupTextLoading)
	if i < 0 || i > strings.Index(got, helpText) {
		t.Fatalf("the loading line must render after the help line (i=%d):\n%s", i, got)
	}
	a.route = routeSession
	if a.loadingView(80) != "" {
		t.Fatal("the session route must not show the loading line")
	}
}

// TestStartupLoadingBootTest is the teatest smoke: the real-boot home
// renders (the real boot hydrates faster than the 500 ms arm, so the
// spinner is not asserted here — absence assertions are forbidden by the
// buffer-drain rule; the state machine is unit-pinned above).
func TestStartupLoadingBootTest(t *testing.T) {
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))
	teatest.WaitFor(t, tm.Output(), hasLine("New session"), teatest.WithDuration(5*time.Second))
	tm.Send(ctrlCKey)
	tm.Send(press('y'))
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
```

(Imports per the package test idiom: `teatest "charm.land/bubbletea/v2/teatest"` as used by `tui_suite_test.go`; the `teatest`/`ctrlCKey` names are the package-existing test identifiers — `ctrlCKey` per `TestTUIFullTurn`.)

**Step 2 — confirm FAIL:** `go test ./internal/tui/ -run 'TestStartupLoading'` → build fails (undefined `loadShowMsg`, `loadDoneMsg`, `startupTextLoading`, `a.loadShown`, `a.loadDone`, `a.loadingText`, `a.loadingView`). That is the red.

**Step 3 — minimal implementation:**

- `internal/tui/startup.go` (new): the consts + the two msg types (no
  methods — the state lives on `App`).
- `internal/tui/app.go`:
  - the 3 fields on `App` (next to `spinIdx`, a comment per the field-style):
    ```go
    // S6.1 startup loading spinner (deviation 237): the shown/ready
    // state + the shown stamp (the min-hold origin).
    loadShown  bool
    loadReady  bool
    loadStamp  time.Time
    ```
  - the `updateMsg` cases (next to `case spinMsg`):
    ```go
    case loadShowMsg:
        if a.loadReady {
            return nil
        }
        a.loadShown = true
        a.loadStamp = time.Now()
        return a.spinTick()
    case loadDoneMsg:
        a.loadShown = false
        return nil
    ```
    and the `spinMsg` re-arm extension: `if a.statusSeg() != "" ||
    a.loadShown { return a.spinTick() }`.
  - the methods:
    ```go
    // loadArm arms the 500 ms show tick (Init batch; nil when already
    // ready — the hydrate raced the arm).
    func (a *App) loadArm() tea.Cmd {
        if a.loadReady {
            return nil
        }
        return tea.Tick(startupShowDelay, func(time.Time) tea.Msg { return loadShowMsg{} })
    }

    // loadDone marks hydration done: nil when already ready; when the
    // line never showed, hide-free no-op; else a hold so the shown span
    // is always >= startupMinHold.
    func (a *App) loadDone() tea.Cmd {
        if a.loadReady {
            return nil
        }
        a.loadReady = true
        if !a.loadShown {
            return nil
        }
        left := startupMinHold - time.Since(a.loadStamp)
        if left <= 0 {
            a.loadShown = false
            return nil
        }
        return tea.Tick(left, func(time.Time) tea.Msg { return loadDoneMsg{} })
    }

    // loadingText is the two-state spinner text (deviation 237: the
    // upstream "Loading plugins..." has no yolo referent).
    func (a *App) loadingText() string {
        if a.loadReady {
            return startupTextReady
        }
        return startupTextLoading
    }

    // loadingView is the home-route-only bottom line (between lastErr and
    // the footer); "" = not shown.
    func (a *App) loadingView(w int) string {
        if !a.loadShown || a.route != routeHome {
            return ""
        }
        line := a.spinFrame() + " " + a.loadingText()
        padded := a.theme.BackgroundPanel().Padding(0, 1).Render(a.theme.TextMuted().Render(line))
        width := runeWidth(line) + 2
        if w <= width {
            return padded
        }
        return strings.Repeat(" ", (w-width)/2) + padded
    }
    ```
  - `Init()`: append `a.loadArm()` to the returned cmd batch (the
    hydrateCmd + eventPump + resyncPump list).
- `internal/tui/hydrate.go`: `applyHydrate` — the `case a.route ==
  routeSession:` success path `return nil` → `return a.loadDone()`; the
  `default:` (home) path falls through to the shared final `return nil` —
  give it an explicit `return a.loadDone()` (the notFound/err cases stay).
- `internal/tui/view.go`: in `view()`, after the `lastErr` block and before
  the footer line:
  ```go
  if line := a.loadingView(w); line != "" {
      b.WriteString("\n" + line)
  }
  ```

**Step 4 — gate:** `go vet ./... && go test ./...` green; `gofmt -l .`
empty.

**Step 5 — commit** the pinned message `feat: home - startup loading
spinner` (branch `new_tui`, never `main`), then
`bd close yolo-oae.7.2 --reason "S6.1 done: the ported startup-loading
state machine (500 ms arm, min-3 s hold, two-state text) + the home-only
bottom line; unit state-machine pins + the teatest boot smoke" --json`.

### Task S6.2: Home: rotating tips — ported tips data + sha256 pin + tests (bead `yolo-oae.7.2`, expected id `yolo-oae.7.3`)

**Files:** new `internal/tui/tips.go` (the data + `parseTip`), new
`internal/tui/tips_test.go` (the pin + shape + token-integrity + parse
tests).

**Interfaces:** `const noModelsTip string` (upstream verbatim), `var tips
[]string` (37 entries — the block below, byte-for-byte), `var tipBindings
[]string` (12 names), `var themeCount int` (the `theme.AllThemes()` len),
`type tipPart struct{ text string; hi bool }`, `var tipHighlightRe *regexp.Regexp`
(`\{highlight\}(.+?)\{/highlight\}`), `func parseTip(s string) []tipPart`,
`var tipTokenRe *regexp.Regexp` (`<([a-z_]+)>`).

**Upstream parity notes:** the data is the ported subset of
`TIPS` (tips-view.tsx:164-283) + `NO_MODELS_TIP` (L71) — 99 upstream
entries reduce to 37 (deviation 234 carries the disposition list); the
`{highlight}…{/highlight}` markup + `parse()` port verbatim (L47-66); the
dynamic tips carry `<name>` tokens inside the highlight spans (substituted
by S6.3's `tipText` — the S6.2 tests treat them as opaque text);
`themeCount` = the upstream `themeCount` (L6, the built-in theme count —
the yolo referent `theme.AllThemes()`).

**Step 1 — failing test** (`internal/tui/tips_test.go`):

```go
package tui

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

// wantTipsPinnedSHA256 pins the ported tips set (root principle 3: the
// pin records the current intended content — the PORTED set, deviation
// 234; an intentional change re-baselines the pin in the same commit).
// Canonical form: noModelsTip first, then the 37 tips in order, each line
// followed by "\n". The constant is computed at Step 3 (the test prints
// the live hash) and re-baselined in the same commit.
const wantTipsPinnedSHA256 = "" // re-baseline at Step 3

func tipsPinnedText() string {
	var b strings.Builder
	b.WriteString(noModelsTip)
	b.WriteByte('\n')
	for _, t := range tips {
		b.WriteString(t)
		b.WriteByte('\n')
	}
	return b.String()
}

func TestTipsPinned(t *testing.T) {
	sum := sha256.Sum256([]byte(tipsPinnedText()))
	if got := hex.EncodeToString(sum[:]); got != wantTipsPinnedSHA256 {
		t.Fatalf("tips sha256 = %s, want %s — re-baseline the pin in the same commit", got, wantTipsPinnedSHA256)
	}
}

// TestTipsShape pins the ported-set size (a silent drop/insert would be a
// data regression the pin catches too — the count is the cheap leg).
func TestTipsShape(t *testing.T) {
	if len(tips) != 37 {
		t.Fatalf("tips = %d entries, want 37", len(tips))
	}
	if noModelsTip == "" {
		t.Fatal("noModelsTip must be set")
	}
}

// TestTipsTokenIntegrity pins that every <binding> token in the tips
// resolves to a keymap binding (a dangling token would render "none"
// mid-sentence) and that every tipBindings entry is actually used.
func TestTipsTokenIntegrity(t *testing.T) {
	all := noModelsTip + "\n" + strings.Join(tips, "\n")
	for _, m := range tipTokenRe.FindAllStringSubmatch(all, -1) {
		if _, ok := Definitions[m[1]]; !ok {
			t.Fatalf("tip token <%s> has no keymap binding", m[1])
		}
	}
	for _, b := range tipBindings {
		if !strings.Contains(all, "<"+b+">") {
			t.Fatalf("tipBindings %s unused in the tips", b)
		}
	}
	if !strings.Contains(all, "{theme_count}") {
		t.Fatal("a tip must use the {theme_count} token")
	}
}

// TestParseTip pins the {highlight} markup port (upstream parse()).
func TestParseTip(t *testing.T) {
	parts := parseTip("Run {highlight}/connect{/highlight} to add an AI provider and start coding")
	if len(parts) != 3 || parts[0].hi || !parts[1].hi || parts[2].hi ||
		parts[0].text != "Run " || parts[1].text != "/connect" || parts[2].text != " to add an AI provider and start coding" {
		t.Fatalf("parse = %+v", parts)
	}
	parts = parseTip("plain text")
	if len(parts) != 1 || parts[0].hi || parts[0].text != "plain text" {
		t.Fatalf("plain parse = %+v", parts)
	}
	parts = parseTip("{highlight}a{/highlight} mid {highlight}b{/highlight}")
	if len(parts) != 5 {
		t.Fatalf("mixed parse = %+v", parts)
	}
}
```

**Step 2 — confirm FAIL:** `go test ./internal/tui/ -run 'TestTips|TestParseTip'` → build fails (undefined `noModelsTip`, `tips`, `tipBindings`, `tipTokenRe`, `parseTip`). That is the red.

**Step 3 — minimal implementation** (`internal/tui/tips.go`, new):

```go
package tui

import (
	"regexp"

	"github.com/kido5217/yolo/internal/tui/theme"
)

// noModelsTip is the no-provider nudge (upstream NO_MODELS_TIP,
// tips-view.tsx:71, verbatim).
const noModelsTip = "Run {highlight}/connect{/highlight} to add an AI provider and start coding"

// tips is the ported tips set (S6.2 — root principle 3 pinned data; the
// pin records THIS ported content, the deviation 234 reduction). Order =
// the upstream TIPS order (tips-view.tsx:164-283) filtered to the yolo
// referent set. {highlight}…{/highlight} marks the bright runs; the
// <binding> and {theme_count} tokens are substituted at render time
// (tipText, S6.3).
var tips = []string{
	"Type {highlight}@{/highlight} followed by a filename to fuzzy search and reference files",
	"Use {highlight}/model{/highlight} or {highlight}<model_list>{/highlight} to switch between available AI models",
	"Use {highlight}/themes{/highlight} or {highlight}<theme_list>{/highlight} to switch between {theme_count} built-in themes",
	"Use {highlight}/new{/highlight} or {highlight}<session_new>{/highlight} to start a fresh conversation session",
	"Use {highlight}/sessions{/highlight} or {highlight}<session_list>{/highlight} to list and continue sessions",
	"Press {highlight}<command_list>{/highlight} to see all available actions and commands",
	"Run {highlight}/connect{/highlight} to add API keys for supported LLM providers",
	"The leader key is {highlight}<leader>{/highlight}; combine with other keys for quick actions",
	"Use {highlight}<messages_page_up>{/highlight}/{highlight}<messages_page_down>{/highlight} to navigate through conversation history",
	"Press {highlight}<prompt_soft_newline>{/highlight} to add newlines in your prompt",
	"Press {highlight}<session_interrupt>{/highlight} to stop the AI mid-response",
	"Switch to {highlight}Plan{/highlight} agent for suggestions without making changes",
	"Create {highlight}yolo.jsonc{/highlight} for server and TUI settings",
	"Place your global settings in {highlight}~/.config/yolo/{/highlight}",
	"Configure {highlight}model{/highlight} in config to set your default model",
	"Override any keybind in {highlight}yolo.jsonc{/highlight} via the {highlight}keybinds{/highlight} section",
	"Set any keybind to {highlight}none{/highlight} to disable it completely",
	"Configure per-agent permissions for {highlight}edit{/highlight}, {highlight}bash{/highlight}, and {highlight}glob{/highlight} tools",
	`Use patterns like {highlight}"git *": "allow"{/highlight} for granular bash permissions`,
	`Set {highlight}"rm -rf *": "deny"{/highlight} to block destructive commands`,
	`Configure {highlight}"git push": "ask"{/highlight} to require approval before pushing`,
	"Run {highlight}yolo serve{/highlight} for headless API access to the core server",
	"Run {highlight}yolo auth list{/highlight} to see all configured providers",
	`Use {highlight}"theme": "system"{/highlight} to match your terminal's colors`,
	"Create JSON theme files in the {highlight}.yolo/themes/{/highlight} directory",
	"Themes support dark/light variants for both modes",
	"Use numeric xterm color codes 0-255 in custom theme JSON",
	"Use {highlight}{env:VAR_NAME}{/highlight} for environment variables in config",
	"Use {highlight}instructions{/highlight} in config to load additional rules files",
	"Permission {highlight}doom_loop{/highlight} prevents infinite tool call loops",
	"Permission {highlight}external_directory{/highlight} protects files outside project",
	"Set {highlight}YOLO_PRINT_LOGS=1{/highlight} to see detailed logs in stderr",
	"Use {highlight}/status{/highlight} or {highlight}<status_view>{/highlight} to see system status info",
	"Use {highlight}/connect{/highlight} with OpenCode Zen for curated, tested models",
	"Commit your project's {highlight}AGENTS.md{/highlight} file to Git for team sharing",
	"Use {highlight}/help{/highlight} to show the help dialog",
	"Press {highlight}<session_rename>{/highlight} to rename the current session",
}

// tipBindings is the <binding> token set the tips templates may use
// (tipText substitutes each with keymap.Format(name); the integrity test
// enforces both directions).
var tipBindings = []string{
	"model_list", "theme_list", "session_new", "session_list", "command_list",
	"leader", "messages_page_up", "messages_page_down", "prompt_soft_newline",
	"session_interrupt", "status_view", "session_rename",
}

// themeCount is the {theme_count} token value (the upstream themeCount,
// the built-in theme count — the yolo referent theme.AllThemes()).
var themeCount = func() int {
	m, err := theme.AllThemes()
	if err != nil {
		return 0
	}
	return len(m)
}()

var (
	tipHighlightRe = regexp.MustCompile(`\{highlight\}(.+?)\{/highlight\}`)
	tipTokenRe     = regexp.MustCompile(`<([a-z_]+)>`)
)

// tipPart is one run of a parsed tip: the text + the highlight flag
// (the bright run; the rest renders muted).
type tipPart struct {
	text string
	hi   bool
}

// parseTip splits the {highlight}…{/highlight} markup into its parts
// (upstream parse(), tips-view.tsx:47-66): the highlighted parts are the
// bright runs, the rest the muted.
func parseTip(s string) []tipPart {
	var parts []tipPart
	last := 0
	for _, m := range tipHighlightRe.FindAllStringSubmatchIndex(s, -1) {
		if m[0] > last {
			parts = append(parts, tipPart{s[last:m[0]], false})
		}
		parts = append(parts, tipPart{s[m[2]:m[3]], true})
		last = m[1]
	}
	if last < len(s) {
		parts = append(parts, tipPart{s[last:], false})
	}
	return parts
}
```

Then **re-baseline the pin**: `go test ./internal/tui/ -run TestTipsPinned`
fails and prints the live hash — paste it into `wantTipsPinnedSHA256` and
re-run until green (root principle 3, the same-commit rule).

**Step 4 — gate:** `go vet ./... && go test ./...` green; `gofmt -l .`
empty.

**Step 5 — commit** the pinned message `feat: home - rotating tips
(ported, sha256-pinned)`, then `bd close yolo-oae.7.3 --reason "S6.2 done:
the ported 37-entry tips set + NO_MODELS_TIP (sha256-pinned, root
principle 3), the parseTip port, the token vocabulary + integrity tests"
--json`.

### Task S6.3: Home: tips rotation cadence + rendering + tests (bead `yolo-oae.7.3`, expected id `yolo-oae.7.4`)

**Files:** modify `internal/tui/tips.go` (the rendering + the ported
visibility — the `tipRun`/`tipLine`/`tipLines`/`joinTipLine`/`writeTipLine`
machinery + `tipText`), `internal/tui/home.go` (the `tips` seam field +
the `renderClamped` line), `internal/tui/app.go` (`tipIdx`, `tipRand`,
`tipsHidden`, `kvTipsHiddenKey`, `repickTip`, `loadTipsHidden`,
`tipsFirst`/`tipsConnected`/`tipsVisible`, `homeTipsLine`, the
`dispatchCommand` case, the `NewApp` wiring), `internal/tui/keys.go`
(the `tips_toggle` case), `internal/tui/keymap.go` (`tips_toggle`
appended at the END of `contextGroups[BaseMode]`), `internal/tui/hydrate.go`
+ `internal/tui/session.go` + `internal/tui/sessionsdlg.go` (the 4 home-entry
`repickTip()` calls), new `internal/tui/tips_render_test.go` (+ the
teatest leg in `internal/tui/tui_suite_test.go` or `tips_render_test.go` —
the package idiom).

**Interfaces:** `App.tipIdx int`, `App.tipRand func() float64`,
`App.tipsHidden bool`, `const kvTipsHiddenKey = "tips_hidden"`, `func
(a *App) repickTip()`, `func (a *App) loadTipsHidden()`, `func (a *App)
tipsFirst() bool`, `func (a *App) tipsConnected() bool`, `func (a *App)
tipsVisible() bool`, `func (a *App) tipText() string`, `func (a *App)
homeTipsLine(w int) string`; `homeModel.tips func(w int) string`; `type
tipRun struct{ text string; kind int }` (kind 0 prefix / 1 muted / 2
text), `type tipLine struct{ runs []tipRun }`, `func tipLines(prefix
string, parts []tipPart, w int) []tipLine`, `func joinTipLine(words []tipWord)
tipLine`, `type tipWord struct{ word string; kind int }`, `func
writeTipLine(b *strings.Builder, l tipLine, th theme.Theme)`.

**Upstream parity notes:** the visibility logic ports verbatim
(tips.tsx:40-47, the `first`/`connected`/`hidden` semantics over the yolo
`store.Providers` referents); the rotation is the one-shot per-mount
random pick (the upstream `Math.random()` — NO timer), ported as the
per-home-entry re-pick (the 4 home-entry sites + `NewApp`); the `● Tip `
prefix (theme `warning`) + the highlight/muted parts render ports the
tips-view.tsx render; the `tips_hidden` KV gate + the `tips.toggle`
command port over the theme KV + the existing `tips_toggle` binding
(`<leader>h`), the upstream palette entry NOT ported (deviation 235).

**Step 1 — failing test** (`internal/tui/tips_render_test.go`):

```go
package tui

import (
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/store"
	"github.com/kido5217/yolo/internal/tui/theme"
	"github.com/kido5217/yolo/internal/tui/client"
)

// TestTipsVisibilityMatrix pins the ported visibility (upstream
// tips.tsx:40-47): first = no sessions, connected = a non-opencode
// provider or an opencode model with input cost, hidden gates all.
func TestTipsVisibilityMatrix(t *testing.T) {
	kido := []protocol.Provider{{ID: "kido", Models: map[string]protocol.Model{"q": {ID: "q", Cost: protocol.ModelCost{Input: 0.5}}}}}
	opencodeZero := []protocol.Provider{{ID: "opencode", Models: map[string]protocol.Model{"z": {ID: "z", Cost: protocol.ModelCost{}}}}}
	opencodePaid := []protocol.Provider{{ID: "opencode", Models: map[string]protocol.Model{"z": {ID: "z", Cost: protocol.ModelCost{Input: 1}}}}}

	a := testApp() // 0 sessions, 0 providers
	if a.tipsFirst() != true || a.tipsConnected() != false {
		t.Fatalf("fresh = first + !connected (got %v/%v)", a.tipsFirst(), a.tipsConnected())
	}
	if !a.tipsVisible() {
		t.Fatal("fresh boot: !connected must show the tips (the NO_MODELS nudge)")
	}
	a.store.Sessions = []protocol.Session{{Title: "s1"}}
	if !a.tipsVisible() {
		t.Fatal("a session + no providers: the tips stay visible (NO_MODELS)")
	}
	a.store.Providers = kido
	if !a.tipsVisible() {
		t.Fatal("a session + a non-opencode provider: visible")
	}
	b := testApp() // 0 sessions + kido: first + connected → hidden
	b.store.Providers = kido
	if b.tipsVisible() {
		t.Fatal("first + connected must hide the tips")
	}
	c := testApp()
	c.store.Providers = opencodeZero
	if c.tipsConnected() {
		t.Fatal("opencode-only with zero input cost must be !connected")
	}
	d := testApp()
	d.store.Providers = opencodePaid
	if !d.tipsConnected() {
		t.Fatal("opencode with an input-cost model must be connected")
	}
	a.tipsHidden = true
	if a.tipsVisible() {
		t.Fatal("the hidden flag must gate all visibility")
	}
}

// TestTipsConnectedRealBoot pins the real-boot state (the static catalog
// kido + opencode ⇒ connected ⇒ fresh boot first + connected ⇒ hidden).
func TestTipsConnectedRealBoot(t *testing.T) {
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	a.Update(a.hydrateCmd()) // hmm — hydrateCmd is a tea.Cmd; drive it via the real loop:
}
```

NOTE (binding): the `TestTipsConnectedRealBoot` leg above is a SKETCH — the
executor drives the real hydrate through the teatest program (NOT
`a.Update(a.hydrateCmd())`, which would execute the cmd's func value
incorrectly) and asserts after the boot `WaitFor(hasLine("New session"))`:
`a.tipsConnected() == true` + `a.tipsVisible() == false` (0 sessions on a
fresh boot). If the executor judges the post-boot store read racy, the
leg collapses into the teatest presence leg below (the unit matrix above
already pins the connected logic) — record the choice in the task
handoff. (The state-machine pins above are the contract; the real-boot
leg is smoke.)

```go
// TestTipTextSubstitutions pins the token substitution: <binding> →
// keymap.Format (registry-driven, remap-sensitive, "none" when
// disabled), {theme_count} → the AllThemes count, and the NO_MODELS
// force when !connected.
func TestTipTextSubstitutions(t *testing.T) {
	a := testApp() // 0 providers → !connected → the NO_MODELS force
	if got := a.tipText(); got != noModelsTip {
		t.Fatalf("tipText = %q, want the NO_MODELS force", got)
	}

	b := testApp()
	b.store.Providers = []protocol.Provider{{ID: "kido", Models: map[string]protocol.Model{"q": {ID: "q", Cost: protocol.ModelCost{Input: 0.5}}}}}
	idx := -1
	for i, s := range tips {
		if strings.Contains(s, "<session_new>") {
			idx = i
		}
	}
	if idx < 0 {
		t.Fatal("no tip uses <session_new>")
	}
	b.tipIdx = idx
	got := b.tipText()
	if !strings.Contains(got, b.keymap.Format("session_new")) {
		t.Fatalf("tipText missing the session_new binding form: %q", got)
	}
	if strings.Contains(got, "<session_new>") {
		t.Fatalf("unsubstituted token: %q", got)
	}
	// remap → the display follows the registry
	if err := b.keymap.Set("session_new", "ctrl+n"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := b.tipText(); !strings.Contains(got, "ctrl+n") {
		t.Fatalf("remapped binding not reflected: %q", got)
	}
	// {theme_count}
	tcIdx := -1
	for i, s := range tips {
		if strings.Contains(s, "{theme_count}") {
			tcIdx = i
		}
	}
	b.tipIdx = tcIdx
	if got := b.tipText(); !strings.Contains(got, strconv.Itoa(len(theme.AllThemesMust()))) {
		t.Fatalf("theme_count not substituted: %q", got)
	}
}
```

NOTE (binding): `theme.AllThemesMust()` does not exist — the executor uses
`themeCount` directly (the package var, S6.2): `!strings.Contains(got,
strconv.Itoa(themeCount))`. The sketch above shows the intent (the count
from the same referent `homeTipsLine` substitutes).

```go
// TestTipLinesWrap pins the tagged-word wrap + the in-sequence run merge
// (the rowLines port): a tip that fits renders one line with the runs in
// SEQUENCE (prefix → muted → text → muted …); a wrapped line keeps the
// order per visual line.
func TestTipLinesWrap(t *testing.T) {
	parts := parseTip("Run {highlight}/connect{/highlight} to add an AI provider and start coding")
	lines := tipLines("● Tip ", parts, 80)
	if len(lines) != 1 {
		t.Fatalf("fitted tip = %d lines, want 1", len(lines))
	}
	joined := ""
	for _, r := range lines[0].runs {
		joined += r.text
	}
	if got := stripANSI(joined); got != "● Tip Run /connect to add an AI provider and start coding" {
		t.Fatalf("joined runs = %q", got)
	}
	kinds := []int{}
	for _, r := range lines[0].runs {
		kinds = append(kinds, r.kind)
	}
	// prefix, muted("Run "), text("/connect"), muted(" to add …")
	if len(kinds) != 4 || kinds[0] != 0 || kinds[1] != 1 || kinds[2] != 2 || kinds[3] != 1 {
		t.Fatalf("run kinds = %v, want [0 1 2 1]", kinds)
	}
	// wrap: narrow width → multiple lines, each in-sequence
	lines = tipLines("● Tip ", parts, 20)
	if len(lines) < 3 {
		t.Fatalf("wrapped tip = %d lines, want >= 3", len(lines))
	}
	for _, l := range lines {
		if len(l.runs) == 0 {
			t.Fatal("an empty visual line")
		}
	}
}

// TestTipsToggle pins the <leader>h toggle: the dispatch flips the flag
// (persisted over the theme KV when the engine is present) and the group
// wiring reaches dispatchCommand.
func TestTipsToggle(t *testing.T) {
	a := testApp()
	a.store.Sessions = []protocol.Session{{Title: "s1"}}
	if !a.tipsVisible() {
		t.Fatal("visible pre-toggle (a session, no providers → NO_MODELS)")
	}
	if cmds := a.dispatchCommand("tips_toggle"); cmds != nil {
		t.Fatalf("the toggle must not emit cmds (got %d)", len(cmds))
	}
	if !a.tipsHidden || a.tipsVisible() {
		t.Fatal("the toggle must hide the tips")
	}
	a.dispatchCommand("tips_toggle")
	if a.tipsHidden || !a.tipsVisible() {
		t.Fatal("the second toggle must restore visibility")
	}
	// the BaseMode group carries the binding (the <leader>h reachability)
	found := false
	for _, name := range contextGroups[BaseMode] {
		if name == "tips_toggle" {
			found = true
		}
	}
	if !found {
		t.Fatal("tips_toggle must be in the BaseMode context group")
	}
}

// TestTipsTogglePersists pins the KV round-trip (the S5.2 seam).
func TestTipsTogglePersists(t *testing.T) {
	a, e := themeApp(t)
	a.tipsHidden = false
	a.dispatchCommand("tips_toggle")
	if !a.tipsHidden {
		t.Fatal("toggle must set the flag")
	}
	e.Close()
	b, _ := themeApp(t)
	b.store.Sessions = []protocol.Session{{Title: "s1"}}
	if !b.tipsHidden {
		t.Fatal("the hidden flag must persist across restart (KV)")
	}
}
```

NOTE (binding): `themeApp(t)` returns `(*recApp, *theme.Engine)` over a
SHARED temp dir per test — for the round-trip the executor uses ONE
`themeApp` instance: toggle + `e.Close()`, then a second `NewApp` over the
SAME engine dir (the executor wires the second app via `NewApp(c, s, "",
theme.New(...KVPath: same dir...))` — the `themeApp` helper's dir is
internal; the minimal path is to build the second engine with the same
`KVPath` the first used — the executor records the exact wiring in the
handoff). The contract: after close + re-load, `tips_hidden == true`.

```go
// TestTipsHomeEntryRepick pins the per-home-entry re-pick (the upstream
// per-mount Math.random, no timer): each entry re-rolls with the seeded
// tipRand; the render picks tips[tipIdx % len].
func TestTipsHomeEntryRepick(t *testing.T) {
	a := testApp()
	a.store.Sessions = []protocol.Session{{Title: "s1"}}
	a.store.Providers = []protocol.Provider{{ID: "kido", Models: map[string]protocol.Model{"q": {ID: "q", Cost: protocol.ModelCost{Input: 0.5}}}}}
	var picks []float64
	i := 0
	a.tipRand = func() float64 {
		defer func() { i++ }()
		return float64(i) / float64(len(tips)) // 0, 1/n, 2/n, …
	}
	a.repickTip()
	first := a.tipIdx
	a.repickTip()
	if a.tipIdx == first {
		t.Fatal("a home entry must re-pick (a fresh random tip)")
	}
	if a.tipIdx < 0 || a.tipIdx >= len(tips) {
		t.Fatalf("tipIdx out of range: %d", a.tipIdx)
	}
	_ = picks
}

// TestHomeTipsLineRender pins the seam + the line shape (the ● Tip prefix
// in the warning tone, the parts wrapped at w) and the hidden/first
// gating through homeTipsLine.
func TestHomeTipsLineRender(t *testing.T) {
	a := testApp() // fresh: visible (NO_MODELS), tipIdx seeded
	line := a.homeTipsLine(80)
	if !strings.HasPrefix(stripANSI(line), "● Tip ") {
		t.Fatalf("tips line = %q, want the '● Tip ' prefix", line)
	}
	// hidden → the line is omitted
	a.tipsHidden = true
	if a.homeTipsLine(80) != "" {
		t.Fatal("hidden must omit the line")
	}
	// the renderClamped seam: a direct-construct homeModel (nil seams)
	// must not panic (the home_theme_test.go:251 pattern)
	var zero homeModel
	if _, _, _, ok := zero.renderClampedSafe(); !ok {
		t.Fatal("nil-seam renderClamped must not panic")
	}
}
```

NOTE (binding): `renderClampedSafe` does not exist — the executor's
nil-seam leg is the direct call `zero.renderClamped(&store.State{}, 80,
theme.Theme{})` wrapped in the existing zero-theme test idiom (the point:
`renderClamped` must nil-guard BOTH seams; assert no panic + the plain
layout). No new method is introduced.

The teatest leg (real boot; the presence assertion only — the tip text is
random, the buffer-drain rule forbids the absence leg):

```go
// TestTipsTeatestPresence: home (fresh, tips hidden — first + connected)
// → n (create + open) → esc (home) → the tips line shows (a session +
// the static catalog ⇒ visible; the prefix is pinned, the text random).
func TestTipsTeatestPresence(t *testing.T) {
	drv := fake.New()
	ts := testutil.BootWithDriver(t, drv)
	c := client.New(ts.URL, ts.Dir)
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))
	teatest.WaitFor(t, tm.Output(), hasLine("New session"), teatest.WithDuration(5*time.Second))
	tm.Send(press('n'))
	teatest.WaitFor(t, tm.Output(), hasLine("esc abort/back"), teatest.WithDuration(5*time.Second))
	tm.Send(press(tea.KeyEscape))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		full := stripANSI(string(b))
		return hasLine("New session")(b) && strings.Contains(full, "● Tip ")
	}, teatest.WithDuration(5*time.Second))
	tm.Send(ctrlCKey)
	tm.Send(press('y'))
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
```

(One merged `WaitFor` for the multi-token terminal state — the
buffer-drain rule; `fake.New()` with zero turns is the idle driver, the
S5.3/4 suites' idiom for flow-only legs.)

**Step 2 — confirm FAIL:** `go test ./internal/tui/ -run 'TestTips|TestTip|TestHomeTipsLine'` → build fails (undefined `tipsFirst`, `tipsConnected`, `tipsVisible`, `tipText`, `tipIdx`, `tipRand`, `tipsHidden`, `tipLines`, `homeTipsLine`, `repickTip`, the `tips_toggle` dispatch case). That is the red.

**Step 3 — minimal implementation:**

- `internal/tui/tips.go`: the rendering + visibility helpers:
  ```go
  // tipWord is one word of a tips line tagged with its run kind (0 the
  // "● Tip " prefix, 1 muted, 2 the highlighted text).
  type tipWord struct {
      word string
      kind int
  }

  // tipRun is one styled run of a visual tips line; the runs stay in
  // SEQUENCE (the parts interleave — unlike rowLine's fixed-order
  // buckets, joinTipLine merges consecutive same-kind words in order).
  type tipRun struct {
      text string
      kind int
  }

  type tipLine struct {
      runs []tipRun
  }

  // tipLines wraps the "● Tip " prefix + the parsed parts at w with the
  // rowLines word-wrap contract (word boundaries, over-long tokens
  // hard-split at the width, single-space rejoin).
  func tipLines(prefix string, parts []tipPart, w int) []tipLine {
      words := []tipWord{{prefix, 0}}
      for _, p := range parts {
          kind := 1
          if p.hi {
              kind = 2
          }
          for _, f := range strings.Fields(p.text) {
              words = append(words, tipWord{f, kind})
          }
      }
      plain := prefix
      for _, p := range parts {
          plain += p.text
      }
      if w < 1 || plain == "" {
          return []tipLine{{runs: []tipRun{{prefix, 0}}}}
      }
      effW := w
      if effW < 1 {
          effW = 1
      }
      var (
          lines []tipLine
          cur   []tipWord
          curW  int
      )
      flush := func() {
          if len(cur) == 0 {
              return
          }
          lines = append(lines, joinTipLine(cur))
          cur, curW = cur[:0], 0
      }
      for _, wd := range words {
          fw := runeWidth(wd.word)
          if fw > effW {
              flush()
              for rest := wd.word; len(rest) > 0 {
                  chunk, r := cutWidth(rest, effW)
                  lines = append(lines, joinTipLine([]tipWord{{chunk, wd.kind}}))
                  rest = r
              }
              continue
          }
          switch {
          case len(cur) == 0:
              cur, curW = append(cur, wd), fw
          case curW+1+fw <= effW:
              cur, curW = append(cur, wd), curW+1+fw
          default:
              flush()
              cur, curW = append(cur, wd), fw
          }
      }
      flush()
      return lines
  }

  // joinTipLine joins one visual line's tagged words into its in-sequence
  // runs (a join space belongs to the PRECEDING word's run; a
  // line-boundary boundary drops it, the rowLine contract).
  func joinTipLine(ws []tipWord) tipLine {
      var l tipLine
      for i, wd := range ws {
          r := tipRun{text: wd.word, kind: wd.kind}
          if i < len(ws)-1 {
              r.text += " "
          }
          if n := len(l.runs); n > 0 && l.runs[n-1].kind == wd.kind {
              l.runs[n-1].text += r.text
          } else {
              l.runs = append(l.runs, r)
          }
      }
      return l
  }

  // writeTipLine renders one visual line's runs: kind 0 the warning
  // prefix, 1 muted, 2 the bright text.
  func writeTipLine(b *strings.Builder, l tipLine, th theme.Theme) {
      for _, r := range l.runs {
          switch r.kind {
          case 0:
              b.WriteString(th.Warning().Render(r.text))
          case 2:
              b.WriteString(th.Text().Render(r.text))
          default:
              b.WriteString(th.TextMuted().Render(r.text))
          }
      }
  }

  // tipText is the token-substituted tip text (the <binding> tokens →
  // keymap.Format, {theme_count} → the theme count), or the NO_MODELS
  // force when !connected (the upstream connected === false force).
  func (a *App) tipText() string {
      if !a.tipsConnected() {
          return noModelsTip
      }
      t := tips[a.tipIdx%len(tips)]
      for _, b := range tipBindings {
          t = strings.ReplaceAll(t, "<"+b+">", a.keymap.Format(b))
      }
      return strings.ReplaceAll(t, "{theme_count}", strconv.Itoa(themeCount))
  }

  // homeTipsLine is the home tips line (the homeModel.tips seam body):
  // "" when hidden (the upstream (!first || !connected) && !hidden gate).
  func (a *App) homeTipsLine(w int) string {
      if !a.tipsVisible() {
          return ""
      }
      lines := tipLines("● Tip ", parseTip(a.tipText()), w)
      var b strings.Builder
      for i, l := range lines {
          if i > 0 {
              b.WriteByte('\n')
              b.WriteString("●")
          }
          writeTipLine(&b, l, a.theme)
      }
      return b.String()
  }
  ```
  (The continuation-line lead `●` is the prefix-glyph indent — mirrors
  `renderRow`'s content-aligned continuation; the executor keeps it
  width-consistent with the prefix run.)
- `internal/tui/app.go`:
  - fields: `tipIdx int` / `tipRand func() float64` / `tipsHidden bool`
    (+ the `const kvTipsHiddenKey = "tips_hidden"` near the KV keys).
  - `NewApp`: `tipRand: math/rand.Float64` in the struct literal; after
    `a.loadFrecency()`: `a.loadTipsHidden(); a.repickTip()`.
  - the methods:
    ```go
    func (a *App) repickTip() { a.tipIdx = int(a.tipRand() * float64(len(tips))) }

    // loadTipsHidden restores the tips_hidden flag (the S5.2 KV seam).
    func (a *App) loadTipsHidden() {
        if a.engine == nil {
            return
        }
        a.tipsHidden = a.engine.KV().Get(kvTipsHiddenKey, false).(bool)
    }

    // tipsFirst/tipsConnected/tipsVisible port the upstream visibility
    // (tips.tsx:40-47) over the yolo store referents.
    func (a *App) tipsFirst() bool { return len(a.store.Sessions) == 0 }

    func (a *App) tipsConnected() bool {
        for _, p := range a.store.Providers {
            if p.ID != "opencode" {
                return true
            }
            for _, m := range p.Models {
                if m.Cost.Input != 0 {
                    return true
                }
            }
        }
        return false
    }

    func (a *App) tipsVisible() bool {
        return !a.tipsHidden && (!a.tipsFirst() || !a.tipsConnected())
    }
    ```
  - the `homeModel` seam wiring (after the struct literal): `a.home.tips =
    func(w int) string { return a.homeTipsLine(w) }` — NOTE (binding): the
    `tips` seam is wired by S6.3 but the `footer` seam by S6.4; S6.3 wires
    ONLY `tips` (the `footer` field does not exist yet — S6.4 adds the
    field + its wiring).
- `internal/tui/home.go`: the `tips func(w int) string` field on
  `homeModel` (+ comment: the S6.3 seam, nil-guarded); `renderClamped`
  after the help line:
  ```go
  if h.tips != nil {
      if line := h.tips(w); line != "" {
          b.WriteByte('\n')
          b.WriteString(line)
      }
  }
  ```
- `internal/tui/keys.go`: the `dispatchCommand` case:
  ```go
  case "tips_toggle":
      a.tipsHidden = !a.tipsHidden
      if a.engine != nil {
          a.engine.KV().Set(kvTipsHiddenKey, a.tipsHidden)
      }
  ```
- `internal/tui/keymap.go`: `contextGroups[BaseMode]` — append
  `"tips_toggle"` at the END (after `session_rename`).
- the home-entry re-pick hooks (`repickTip()` call): `hydrate.go:101`
  (the notFound case, next to `a.route = routeHome`), `session.go:645`
  (the esc-when-idle case), `sessionsdlg.go:308` + `:329` (the two
  delete→home branches).
- `internal/tui/home_test.go`: re-baseline `TestHomeRenderLockedLayout` —
  the testApp (0 sessions, 0 providers, nil engine) now renders the
  `NO_MODELS_TIP` line after the help line (the tips seam wired via
  `NewApp`; `tipIdx` seeded deterministically enough that the line is
  always the NO_MODELS text — `!connected` forces it regardless of
  `tipIdx`): the expected layout gains the line
  `stripANSI` = `● Tip ` + the NO_MODELS plain text
  (`Run /connect to add an AI provider and start coding`), one line (fits
  80). The pin is the locked-layout re-baseline (same commit).

**Step 4 — gate:** `go vet ./... && go test ./...` green (incl.
`TestHomeRenderLockedLayout` re-baselined + the teatest presence leg);
`gofmt -l .` empty.

**Step 5 — commit** the pinned message `feat: home - tips rotation +
rendering`, then `bd close yolo-oae.7.4 --reason "S6.3 done: the ported
tips visibility (first/connected/hidden), the per-home-entry random
re-pick (no timer), the ● Tip tagged-word render, the tips_hidden KV
toggle (<leader>h) + group wiring; unit matrix + wrap + toggle +
persistence + the teatest presence leg" --json`.

### Task S6.4: Home: session-destination view + tests (bead `yolo-oae.7.4`, expected id `yolo-oae.7.5`)

**Files:** new `internal/tui/destination.go` (`abbrevHome` +
`sessionDestination` + `homeFooterLine`), `internal/tui/app.go` (the
`homeDirFunc` seam field + the `homeDir` method + the `home.footer`
seam wiring), `internal/tui/home.go` (the `footer func(w int) string`
seam field + the `renderClamped` line), new
`internal/tui/destination_test.go`.

**Interfaces:** `func abbrevHome(dir, home string) string`, `func (a *
App) homeDir() string`, `func (a *App) sessionDestination() string`,
`func (a *App) homeFooterLine(w int) string`, `App.homeDirFunc func()
string`, `homeModel.footer func(w int) string`.

**Upstream parity notes:** `abbreviateHome` ports verbatim
(runtime.tsx:3-10, the `path.relative` semantics over `filepath.Rel` —
the TUI runs linux-only, the root env); the destination = the scope
directory (the upstream selected ?? `sync.path.directory || cwd` default
— yolo has one scope, the selection state machine has no referent,
deviation 236); the render slot = the home footer line (the upstream
footer Directory segment — the yolo home footer is the key-hints line, so
the destination rides its FIRST part, S6.5 completes the line).

**Step 1 — failing test** (`internal/tui/destination_test.go`):

```go
package tui

import (
	"strings"
	"testing"
)

// TestAbbrevHome pins the ported abbreviateHome (upstream runtime.tsx:3-10).
func TestAbbrevHome(t *testing.T) {
	cases := []struct{ dir, home, want string }{
        {"/home/u/proj", "/home/u", "~/proj"},
        {"/home/u", "/home/u", "~"},
        {"/etc", "/home/u", "/etc"},
        {"/home/u2/x", "/home/u", "/home/u2/x"},
        {"/home/u/../etc", "/home/u", "/etc"},
        {"", "/home/u", ""},
        {"/home/u", "", "/home/u"},
        {"/tmp/xyz/001", "/home/u", "/tmp/xyz/001"},
    }
    for _, c := range cases {
        if got := abbrevHome(c.dir, c.home); got != c.want {
            t.Fatalf("abbrevHome(%q, %q) = %q, want %q", c.dir, c.home, got, c.want)
        }
    }
}

// TestSessionDestination pins the scope-dir resolution + the ""-Dir
// omission (testApp) + the homeDir seam.
func TestSessionDestination(t *testing.T) {
	a := testApp() // Dir "" → the server work dir is unknown → omitted
	if a.sessionDestination() != "" {
		t.Fatal("an empty Dir must omit the destination")
	}
	a.Service.Dir = "/home/u/proj"
	a.homeDirFunc = func() string { return "/home/u" }
	if got := a.sessionDestination(); got != "~/proj" {
		t.Fatalf("destination = %q, want %q", got, "~/proj")
	}
	// outside the home dir → the raw path
	a.Service.Dir = "/tmp/xyz/001"
	if got := a.sessionDestination(); got != "/tmp/xyz/001" {
		t.Fatalf("outside destination = %q", got)
	}
}

// TestHomeFooterLine pins the S6.4 line shape (destination only — the
// hint joins at S6.5): the dimmed single line, omitted when empty, and
// its render slot (after the help line + tips line, before the footer).
func TestHomeFooterLine(t *testing.T) {
	a := testApp()
	if a.homeFooterLine(80) != "" {
		t.Fatal("no destination (Dir == \"\") must omit the line")
	}
	a.Service.Dir = "/home/u/proj"
	a.homeDirFunc = func() string { return "/home/u" }
	if got := stripANSI(a.homeFooterLine(80)); got != "~/proj" {
		t.Fatalf("footer line = %q, want %q", got, "~/proj")
	}
	// render slot: home view carries the line after the help line
	out := stripANSI(a.view())
	h := strings.Index(out, helpText)
	d := strings.Index(out, "~/proj")
	if h < 0 || d < 0 || d < h {
		t.Fatalf("destination line must follow the help line (h=%d d=%d):\n%s", h, d, out)
	}
	// a wrapped destination line still renders (the dimWrapped at w)
	a.Service.Dir = "/home/u/this/is/a/rather/long/destination/path/for/wrapping"
	if got := a.homeFooterLine(20); got == "" {
		t.Fatal("a long destination must render (wrapped)")
	}
}
```

**Step 2 — confirm FAIL:** `go test ./internal/tui/ -run 'TestAbbrevHome|TestSessionDestination|TestHomeFooterLine'` → build fails (undefined `abbrevHome`, `sessionDestination`, `homeFooterLine`, `homeDirFunc`). That is the red.

**Step 3 — minimal implementation:**

- `internal/tui/destination.go` (new):
  ```go
  package tui

  import (
      "os"
      "path/filepath"
  )

  // abbrevHome is the ported abbreviateHome (upstream runtime.tsx:3-10):
  // "~" at the home dir, "~/rel" under it, the raw path outside (or when
  // home is unknown).
  func abbrevHome(dir, home string) string {
      if dir == "" || home == "" {
          return dir
      }
      rel, err := filepath.Rel(home, dir)
      if err != nil || rel == ".." || strings.HasPrefix(rel, "../") || filepath.IsAbs(rel) {
          return dir
      }
      if rel == "." {
          return "~"
      }
      return "~/" + rel
  }

  // homeDir is the home dir for the abbreviation (the test seam
  // homeDirFunc overrides; default os.UserHomeDir).
  func (a *App) homeDir() string {
      if a.homeDirFunc != nil {
          return a.homeDirFunc()
      }
      h, _ := os.UserHomeDir()
      return h
  }

  // sessionDestination is the new-session destination (the scope dir,
  // home-abbreviated — the upstream selected ?? cwd default; the
  // selection state machine has no yolo referent, deviation 236).
  // "" when the scope dir is unknown (a.Service.Dir == "").
  func (a *App) sessionDestination() string {
      return abbrevHome(a.Service.Dir, a.homeDir())
  }

  // homeFooterLine is the home footer line (the homeModel.footer seam
  // body): S6.4 the destination part only; S6.5 joins the hint part.
  func (a *App) homeFooterLine(w int) string {
      d := a.sessionDestination()
      if d == "" {
          return ""
      }
      return dimWrapped(a.theme, d, w)
  }
  ```
- `internal/tui/app.go`: the `homeDirFunc func() string` field (+
  comment); `NewApp` — wire `a.home.footer = func(w int) string {
  return a.homeFooterLine(w) }` (next to the S6.3 `a.home.tips` wiring).
- `internal/tui/home.go`: the `footer func(w int) string` field on
  `homeModel` (+ comment: the S6.4 seam, nil-guarded); `renderClamped`
  after the tips block:
  ```go
  if h.footer != nil {
      if line := h.footer(w); line != "" {
          b.WriteByte('\n')
          b.WriteString(line)
      }
  }
  ```

**Step 4 — gate:** `go vet ./... && go test ./...` green; `gofmt -l .`
empty. (No `TestHomeRenderLockedLayout` re-baseline — the testApp `Dir ==
""` omits the line; the locked layout is unchanged at this step.)

**Step 5 — commit** the pinned message `feat: home - session destination
view`, then `bd close yolo-oae.7.5 --reason "S6.4 done: the ported
abbrevHome + the scope-dir destination + the home footer line's
destination part (omitted for the unknown server work dir); unit table +
resolution + render-slot pins" --json`.

### Task S6.5: Home: footer key hints from the keymap registry + tests (bead `yolo-oae.7.5`, expected id `yolo-oae.7.6`)

**Files:** `internal/tui/destination.go` (`homeShortcutsHint` + the
`homeFooterLine` parts join), `internal/tui/destination_test.go` (the
hint tests + the joined-line test), `internal/tui/home_test.go`
(`TestHomeRenderLockedLayout` re-baseline — the hint-only line), the
teatest leg (`TestFooterHintTeatest` in `destination_test.go` or the
suite file — the package idiom).

**Interfaces:** `func (a *App) homeShortcutsHint() string`;
`homeFooterLine` extended to the parts join (signature unchanged).

**Upstream parity notes:** the HomeHint text ("Show keyboard shortcuts
with <trigger>") ports verbatim (which-key.tsx:176-181); the trigger =
the LEADER key (the yolo referent — the upstream `which_key_toggle` is
inert, deviation 207; deviation 238), rendered via `keymap.Format` (the
registry, remap-sensitive); the line omits when the leader binding is
disabled (upstream's fallback-to-command-name is not ported — a disabled
leader makes the overlay unreachable); the two-tone subtle span is not
ported (the yolo home bottom lines are single-tone dim); the join with
the S6.4 destination part is `" · "` (the help-line separator convention).

**Step 1 — failing test** (`internal/tui/destination_test.go`):

```go
// TestHomeShortcutsHint pins the registry-rendered hint: the default
// leader (ctrl+x), the remap-sensitivity, and the leader-disabled
// omission.
func TestHomeShortcutsHint(t *testing.T) {
	a := testApp()
	if got := a.homeShortcutsHint(); got != "Show keyboard shortcuts with ctrl+x" {
		t.Fatalf("hint = %q, want the default-leader form", got)
	}
	if err := a.keymap.Set("leader", "ctrl+j"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := a.homeShortcutsHint(); got != "Show keyboard shortcuts with ctrl+j" {
		t.Fatalf("remapped hint = %q, want the ctrl+j form", got)
	}
	if err := a.keymap.Set("leader", "none"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if a.homeShortcutsHint() != "" {
		t.Fatal("a disabled leader must omit the hint")
	}
}

// TestHomeFooterLineWithHint pins the S6.5 parts join: destination +
// hint " · "-joined, each part omittable, the dimmed single line.
func TestHomeFooterLineWithHint(t *testing.T) {
	a := testApp()
	a.Service.Dir = "/home/u/proj"
	a.homeDirFunc = func() string { return "/home/u" }
	if got := stripANSI(a.homeFooterLine(80)); got != "~/proj · Show keyboard shortcuts with ctrl+x" {
		t.Fatalf("joined line = %q", got)
	}
	// destination omitted (Dir "") → the hint-only line
	b := testApp()
	if got := stripANSI(b.homeFooterLine(80)); got != "Show keyboard shortcuts with ctrl+x" {
		t.Fatalf("hint-only line = %q", got)
	}
	// both omitted (leader none) → ""
	if err := b.keymap.Set("leader", "none"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if b.homeFooterLine(80) != "" {
		t.Fatal("no parts must omit the line")
	}
	// the render slot: the line is the LAST home-bottom line (after the
	// tips line on the testApp)
	c := testApp()
	out := stripANSI(c.view())
	if !strings.HasSuffix(strings.TrimSpace(out), "Show keyboard shortcuts with ctrl+x") {
		t.Fatalf("the footer hint line must be the last home line:\n%s", out)
	}
}

// TestFooterHintTeatest pins the hint on the real boot (the destination
// prefix is the unpredictable TempDir path — the unit leg pins it; the
// teatest leg pins the registry-rendered hint suffix).
func TestFooterHintTeatest(t *testing.T) {
	drv := fake.New()
	ts := testutil.BootWithDriver(t, drv)
	c := client.New(ts.URL, ts.Dir)
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		full := stripANSI(string(b))
		return hasLine("New session")(b) && strings.Contains(full, "Show keyboard shortcuts with ctrl+x")
	}, teatest.WithDuration(5*time.Second))
	tm.Send(ctrlCKey)
	tm.Send(press('y'))
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
```

**Step 2 — confirm FAIL:** `go test ./internal/tui/ -run 'TestHomeShortcutsHint|TestHomeFooterLineWithHint|TestFooterHintTeatest'` → build fails (undefined `homeShortcutsHint`; the joined-line assertions fail behaviorally once it exists — the build fail is the red).

**Step 3 — minimal implementation:**

- `internal/tui/destination.go`:
  ```go
  // homeShortcutsHint is the registry-rendered hint (the upstream
  // which-key HomeHint text, deviation 238): the trigger is the leader
  // key (the which-key overlay's opener — the upstream which_key_toggle
  // is inert, deviation 207); "" when the leader binding is disabled
  // (Format "none" — the overlay is then unreachable).
  func (a *App) homeShortcutsHint() string {
      trigger := a.keymap.Format("leader")
      if trigger == "none" {
          return ""
      }
      return "Show keyboard shortcuts with " + trigger
  }
  ```
  and `homeFooterLine` becomes the parts join:
  ```go
  func (a *App) homeFooterLine(w int) string {
      var parts []string
      if d := a.sessionDestination(); d != "" {
          parts = append(parts, d)
      }
      if h := a.homeShortcutsHint(); h != "" {
          parts = append(parts, h)
      }
      if len(parts) == 0 {
          return ""
      }
      return dimWrapped(a.theme, strings.Join(parts, " \u00B7 "), w)
  }
  ```
  (`import "strings"` added to destination.go.)
- `internal/tui/home_test.go`: re-baseline `TestHomeRenderLockedLayout` —
  the testApp footer line (Dir "" ⇒ destination omitted) becomes the
  hint-only line `Show keyboard shortcuts with ctrl+x` appended after the
  S6.3 tips line; the expected layout constant gains that line (the
  locked-layout re-baseline, same commit). `TestHomeRenderWraps` needs no
  change (the hint line fits 80; the substring assertions hold).

**Step 4 — gate:** `go vet ./... && go test ./...` green (incl. the
re-baselined locked layout + the teatest leg); `gofmt -l .` empty.

**Step 5 — commit** the pinned message `feat: home - footer key hints
from registry`, then `bd close yolo-oae.7.6 --reason "S6.5 done: the
registry-rendered home hint line (the leader-key trigger, remap-
sensitive, leader-disabled omission) + the footer parts join; unit hint
+ join pins + the teatest real-boot leg + the locked-layout re-baseline"
--json`.

## S6 slice gate (slice bead `yolo-oae.7`)

NOT a task bead; runs after all child beads close. Mirror the S0 slice gate
shape: (1) module gate `go vet ./... && go test ./...` + `gofmt -l .` empty
(incl. `TestImportsDirection` + the S6 teatest goldens; the S6.2 tips sha256
pin must not be dangling — root principle 3); (2) user-run smoke (NOT CI):
on home in a real TTY — the startup spinner during hydration, the tips
rotating on cadence, the footer key hints, and the session-destination
view; (3) append any forced DEVIATIONS.md entries this slice named (with
severity, same-commit rule — root principle 2); (4) PROGRESS.md one-line
status pointer; (5) commit
`docs: checkpoint — S6 done, next is S7 detail pass`; (6)
`bd close yolo-oae.7 --reason "all 5 child beads closed, gate green" --json`.
