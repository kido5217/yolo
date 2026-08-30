# S3 — remaining contract-backed dialogs (slice bead `yolo-oae.4`)

Land the remaining contract-backed dialogs — session-list/rename/
delete-failed, provider, status, help, retry-action, theme-list — plus the
theme mode switch/lock keybinds wired to the S0 KV.

**State: fully detailed** — the 5-step TDD detail for all 9 tasks is in
the `## S3 detail` section below; execution may start at task S3.1.

## Binding task table

Pointer only (FROZEN — Slice Detail Protocol rule 1): `plan.md` →
`## Task inventory` → `### S3 — remaining contract-backed dialogs (slice bead yolo-oae.4)`.
Bead titles, scope, and pinned commit messages live there and may not be
changed by a detail pass; any required change = STOP + explicit user
approval + re-record in plan.md.

## Dep gate

None — `huh` + `sahilm/fuzzy` already landed via the S2 gate; the S3
dialogs reuse the S2 select + themed huh input primitives.

## Exact upstream sources (the detail pass reads these AT DETAIL TIME)

`/tmp/opencode-upstream` @ v1.18.18:

- `packages/tui/src/component/dialog-session-list.tsx` — S3.1, session-list
  dialog.
- `packages/tui/src/component/dialog-session-rename.tsx` — S3.2, rename
  (uses the themed huh input from S2.4).
- `packages/tui/src/component/dialog-session-delete-failed.tsx` — S3.3,
  delete-failed dialog.
- `packages/tui/src/component/dialog-provider.tsx` — S3.4, provider
  restyle.
- `packages/tui/src/component/dialog-status.tsx` — S3.5, status dialog
  (content contract read from upstream at detail time — spec §9 open
  items).
- `packages/tui/src/component/dialog-retry-action.tsx` — S3.7, retry-action
  (semantics read from upstream at detail time — spec §9 open items).
- `packages/tui/src/component/dialog-theme-list.tsx` — S3.8, theme-list
  (select over `theme.All()`; S3.9 wires KV + mode keybinds).
- `packages/tui/src/ui/dialog-help.tsx` — S3.6, help dialog; note it is
  keymap-registry-driven per the S4 handoff: S4.7 completes the
  registry-driven rendering of /help (frozen S4 table row); the S3.6
  detail pass defines the exact consumption contract at detail time.
- `packages/tui/src/context/kv.tsx` — the upstream KV store (the port
  reference for S3.9 mode switch/lock semantics on top of yolo's S0.7 KV
  file + Engine).
- `packages/tui/src/keymap.tsx` — the keybinds for theme mode switch/lock
  (S3.9).

## yolo anchors

- `internal/tui/dialog.go` — the existing dialog surface extended here.
- `internal/tui/commands.go` + `internal/tui/view.go` — the existing
  provider/model/status surfaces (verified 2026-08-25: there is no
  `menu.go` in the tree; the surfaces live here).
- `internal/tui/theme/` — `theme.All()`, Engine Set/Pin/Free, and the KV
  file (S0.7) for the theme-list + mode wiring.
- `internal/protocol/` — command DTOs, if needed.
- `internal/config/` — the config-side `theme` string + profile surface.

## Detail pass (protocol)

One writing-plans pass, one subagent, `thinking=high`, dispatched by the
root session strictly sequentially (root principle 7) — per the Slice
Detail Protocol in plan.md: it fills this file (after this section) with the
full 5-step TDD detail for each task in the binding table (failing test
code, implementation code, gate, pinned commit), reading the named upstream
files at that moment. It commits as
`docs: TUI parity plan — detail S3 tasks` on its own bead
(`bd create "detail S3 plan tasks" --parent=yolo-oae.4 --json`).

## S3 detail

Detail pass 2026-08-30. Deviations tail at detail time = 187; S3 entries
start at 188. Breadcrumb note (DEVIATIONS.md entry 188, severity info):
the frozen table names the task beads `yolo-oae.4.1`–`4.9`, but the S3
detail bead consumed `yolo-oae.4.1` (created + claimed before the detail
pass; the S1 "detail-bead-last" precedent is impossible because the
detail pass precedes slice start, as in S2 / deviation 165). The 9 task
beads therefore land in table order at `yolo-oae.4.2`–`yolo-oae.4.10`
(S3.1→.2, …, S3.9→.10); the frozen titles and pinned commit messages are
unchanged. No code or wire impact.

### Detail-pass findings (read at detail time, 2026-08-30 — binding)

1. **Theme engine API (`internal/tui/theme/engine.go`):**
   - `theme.New(theme.EngineOptions{KVPath, GlobalYoloDir, CWD, ConfigTheme string; Palette func(context.Context) (theme.TerminalColors, bool)}) (*Engine, error)` — the Palette func is called exactly once.
   - Methods: `AllThemes() map[string]theme.ThemeJson` (builtins + customs + "system"), `ActiveTheme() (theme.Theme, error)`, `Active() string`, `Mode() string` ("dark"|"light"), `Locked() bool`, `Ready() bool`, `Has(name) bool`, `Set(name) bool` — **persists the KV key `theme` immediately** (live previews persist too — the upstream `theme.set` behavior), `Pin(mode string)` — lock + apply + persist `theme_mode_lock` and `theme_mode`, `Free()` — clear the lock and both KV keys, `Apply(mode string)` — persists `theme_mode` only while locked, `ThemeModeEvent(mode string)` (a tea.Msg), `RefreshCustoms(ctx context.Context)`, `Close()`.
   - The KV file is `<data>/tui/kv.json` via `theme.OpenKV(path)` (kv.go: the in-memory store is the source of truth; a writer goroutine drains the queue; the queue is never closed).
   - Package-level `theme.AllThemes() (map[string]ThemeJson, error)` (theme.go:27) — builtins only. The frozen table's `theme.All()` name does not match the real API: the intent binds (a select over all theme names); the method name is a detail correction to `Engine.AllThemes()`.
   - `theme.DefaultName = "opencode"`; `theme.Theme{R Resolved, Name string, Mode string}`.
   - Test wiring: `theme.New(theme.EngineOptions{KVPath: filepath.Join(dir, "kv.json"), GlobalYoloDir: dir, CWD: dir, Palette: func(context.Context) (theme.TerminalColors, bool) { return theme.TerminalColors{}, false }})` + `e.Resolve(ctx)` + `t.TempDir()` (home_theme_test.go pattern).

2. **Upstream theme command semantics (`context/theme.tsx` + `app.tsx`, v1.18.18):**
   - `setMode` IS `pin` (theme.tsx:292): the upstream "switch mode" command **pins the opposite mode** — a quirk, ported verbatim.
   - `theme.switch_mode` → `setMode(mode() === "dark" ? "light" : "dark")`; `theme.mode.lock` → `locked() ? free() : lock()` (lock = `pin(store.mode)`).
   - Dynamic command titles: "Switch to light mode" / "Switch to dark mode"; "Unlock theme mode" / "Lock theme mode".
   - `theme.switch` → DialogThemeList.
   - KV keys (theme.tsx): `theme` (the active name), `theme_mode` ("dark"|"light"), `theme_mode_lock` (bool).

3. **Upstream default keybinds (`config/keybind.ts`):** `theme_list: <leader>t` → `theme.switch` (slashName "themes"); `theme_switch_mode: "none"` → `theme.switch_mode`; `theme_mode_lock: "none"` → `theme.mode.lock`; `session_list: <leader>l` (slashName "sessions", aliases resume/continue); `session_rename: ctrl+r`; `session_delete: ctrl+d`; `session_pin_toggle: ctrl+f`; `status_view: <leader>s` (command `opencode.status`, slashName "status"); `command_list: ctrl+p` → `command.palette.show` (note: yolo's ctrl+p currently opens the model dialog — the remap lands in S4.1); `provider_connect: "none"` → `provider.connect` (title "Connect provider", slashName "connect"). The S3 tasks land the slash names + the functions; **the key defaults (incl. the "none" pairs) land in the S4.1 registry** — the S3.9 commands carry no default keys (deviation 196).

4. **Wire / session facts (yolo):**
   - `client.Service` (embedded in `App`): ListSessions/CreateSession/GetSession/PatchSession/DeleteSession/ListMessages/SendMessage/Abort/Command/`Status(ctx) (map[string]string, error)` (GET /session/status → `{"sessions": {id: "idle"|"busy"|"retry"}}`, server.go:95, handlers_session.go:321)/ListProviders/GetConfig/PatchConfig/GlobalConfig/`Auth(ctx, providerID, key string, remove bool) error`/ListAgents/ListCommands/ListPermissions/ReplyPermission; sentinels ErrNotFound/ErrBusy/ErrBadRequest. **S3 adds no new client methods and no new wire endpoints.**
   - Server session delete (`handlers_session.go` `handleSessionDelete`): scopedSession (404 missing/out-of-scope) → ListMessages (500) → DB.DeleteSession (500) → engine close → emits `session.deleted` → 204. **No 409-busy path, no workspaces in yolo** (grounds the S3.3 adaptation).
   - Retry semantics (`session/round.go` `streamWithRetry`): pre-stream transient failures retried up to `maxRetryAttempts=4` with backoff; the wire `session.status` type "retry" (`Attempt` 1..3 + `Next`=delay) is emitted only when attempt < 4; the attempt-4 failure = exhaustion → NO wire event, the turn fails with the boundary error. Overflow 400 → synthetic note, turn idle. Wire `protocol.SessionStatus{Type, Attempt, Message, Next}` has no `action` field.
   - The upstream `DialogRetryAction` is a Go upsell only (free_tier_limit/account_rate_limit on the opencode/opencode-go providers), gated by a 24h KV window + a dontShow KV, shown only when the dialog stack is empty (routes/session/index.tsx:372).
   - store: `applySessionStatus` is current-session-only (`store.Status` single) → the session list needs the per-session snapshot from `GET /session/status`; `applySessionDeleted` drops the session from `Sessions` + clears `Current`; `upsertSession` is in-place. Events flow through the `EventMsg` case (app.go:164): `a.store.Apply(m.Event)` + hook (`a.syncPermDialog()`).

5. **Upstream dialog contracts (read in full at detail time, v1.18.18):**
   - `dialog-session-list.tsx`: DialogSelect "Sessions" (default size medium=60), `skipFilter={true}` + `onFilter={setSearch}` — the filter input STILL renders; typed text drives a **server-side** search (`sdk.client.session.list({search})`); the list-level fuzzy filter is skipped; `preserveSelection={true}`; `current` = the route's sessionID; `onMove` clears toDelete; the option title becomes "Press <deleteHint> again to confirm" + `bg=theme.error` while toDelete; options merge browse results with the current + pinned local sessions; a client-side title substring filter (`.filter(session => title.toLowerCase().includes(query))`); category = "Pinned" | ("Today" when the updated date == today, else the updated date's JS `toDateString`); order = updated-desc; the gutter = a spinner when busy/retry, else the quick-switch slot number; footer actions pin / delete (ctrl+d) / rename (ctrl+r); a delete failure → toast + the dialog is replaced by DialogSessionDeleteFailed (workspace recovery).
   - `dialog-session-rename.tsx`: DialogPrompt "Rename Session", initial value = the title, onConfirm → `update({title})` + clear; an empty value → no-op.
   - `dialog-session-delete-failed.tsx`: active "delete" | "restore"; options "Delete workspace" / "Restore to new workspace" (each with a muted description); return=confirm (result===false → stays open), left/up=delete, right/down=restore, esc close.
   - `dialog-provider.tsx`: `PROVIDER_PRIORITY {opencode:0, opencode-go:1, openai:2, github-copilot:3, anthropic:4, google:5}`, unknown=99; category "Popular" (<99) | "Providers"; a description map per known id; sort (priority, name lc, id); the "Other" custom option value `__opencode_custom_provider__`; the custom id regex `/^[a-z0-9][a-z0-9-_]*$/` + `@ai-sdk/` prefix strip; onSelect → auth-method select → key prompt → `auth.set`; on success the dialog is replaced by the model dialog. Toasts (verbatim): invalid id → error "Provider ids must start with a lowercase letter or number and only use lowercase letters, numbers, hyphens, and underscores"; the custom key prompt description "This only stores a credential. Configure the provider in opencode.json to use it."; saved custom → info `Saved credential for <id>. Configure it in opencode.json to use it.`
   - `dialog-status.tsx`: "Status" bold + "esc" muted; per section a count header row (text token), bullet rows = a status-colored bullet (success/error/warning/textMuted) + a bold name + a muted status detail; the per-section fallback "No X" (text token). Sections: MCP servers / LSP / formatters / plugins.
   - `dialog-retry-action.tsx`: title bold + "esc" muted; message muted; an optional link line; pills left "don't show again" / right the action; starts selected on the action; left/right/tab toggle; return confirms; esc dismisses; `show()` → Promise&lt;boolean&gt;.
   - `ui/dialog-help.tsx`: "Help" bold + "esc/enter" muted; body muted `Press {shortcut} to see all available actions and commands in any context.`; a right-aligned "ok" pill (pad 0 3, primary bg, selectedListItemText fg); return/escape close.
   - `dialog-theme-list.tsx`: title "Themes"; options = the case-insensitively sorted keys of `theme.all()`; onMove/onSelect → `theme.set` (persists immediately); onFilter: empty → set(initial), else set(first filtered); onCleanup: !confirmed → set(initial).
   - `ui/dialog-select.tsx`: `skipFilter` (line 155: the filtered memo returns all when skipFilter || renderFilter===false), `renderFilter` (hides the input), `preserveSelection`, `locked`; `onFilter` fires on the input value change (line 577).

6. **yolo anchors (verified at detail time):**
   - view.go: `view()` → top modal ? `viewModal()` : route+menu+perm+prompt+toasts+dlg+lastErr+footer; `viewModal` panelTop=max(h/4, modalChromeMin), inner via `modalInner(&d, panelW, h)`; the slash menu merge point is view.go:33 (`a.prompt.menuView(a.store.Commands, w, a.theme)`).
   - keys.go ladder: permission &gt; dialog &gt; model/agent openers &gt; slash &gt; route &gt; prompt; `handleDialogKey` currently pops on any key for the non-payload kinds (dialog.go:367 `a.dlg.pop() // dlgHelp: any key closes`) — the new modal kinds restructure this (the payloads own their keys; only esc/ctrl+c pop, via the stack).
   - commands.go `runCommand`: /help pushes the NON-modal dlgHelp; /quit // exit, /model, /agents, /new. prompt.go `menuItems(cmds)` filters by the / prefix + `commandAliases{"/quit": {"/exit"}}`; the server command catalog is FROZEN at 5 (handlers_catalog.go: /help, /new, /model, /agents, /quit — T20) → the new TUI slash commands (/sessions, /status, /themes, /connect) merge **client-side** (a local `[]protocol.Command` list + `store.Commands`, merged at view.go:33; `runCommand` gains the new cases) — the core is unchanged.
   - dialog.go: the `dialog` payload fields (model/agent/form/sel/perm); `pushModal(item, size, onClose)` (:126), `closeTopModal()` (:134), `replaceModal(...)` (:147); dlgSize 60/88/116; the `openModelDialog`/`openAgentDialog` pattern (push modal + syncSel + fetchCatalog); `providerStatusText(auth) string` (:661) + `providerStatus(th, auth) (string, lipgloss.Style)` (:674); the current help is `helpDialog(th)` (a markdown-ish table, dialog.go:~240-258).
   - select.go: `selectNew(title, placeholder string, options []selectOption, isCurrent func(selectOption) bool, onSelect func(*App, selectOption), onMove func(selectOption)) *selectModel` (:69), `WithActions([]selectAction)`, `WithHints([]footerHint)`, `filtered()` (fuzzy), `syncFilter()` (:150), `buildLines`/`rowLine(o, active, cur, th, w)` (:421) + `rowWithFooter`; the row's leading column = the `  ` / `● ` (isCurrent) logic.
   - huhdlg.go: `buildInputForm(th theme.Theme, title, description, placeholder, initial string) *huh.Form` (:107), `openFormModal(form, size, onConfirm, onClose) []tea.Cmd` (:73), `huhFormDlg{form, onConfirm}`; the `updateMsg` completion cascade (StateCompleted → closeTopModal + onConfirm; StateAborted → closeTopModal).
   - footer.go: `footerFrames` (5 braille), `spinTick` 100ms (`spinMsg{}`), `a.spinFrame()`; the busy tick already runs while the CURRENT session is busy (app.go:184, 311-313).
   - session.go: `handleSessionKey` (pgup/pgdn / expand alt+e / think alt+t / esc abort-or-home; reports (cmds, consumed)); the esc branch: busy → `a.emit(a.abortCmd())`, else → routeHome + hydrate; `sessionBusy(st)` (:53); `sessKeyMap` (the binding struct, session.go:29).
   - hydrate.go: `openSession(id)` (:162), `hydrateCmd`/`applyHydrate`.
   - style.go: `title` (bold), `divider` (ANSI 240), `cursorStyle(th)`.
   - theme/styles.go: the accessors Text/TextMuted/Primary/Secondary/Accent/Error/Warning/Success/Info/Border/BorderActive/BorderSubtle/Background, `SelectedForeground(bg ...Rgba) Rgba`, `Color(name) (Rgba, bool)`, `Rgba{R,G,B,A}.Hex()`.
   - Test harness: `testApp(sessions ...protocol.Session) *recApp` (dummy client, home_test.go:29), `newRecApp(c, s, startSessionID)`, `press(r rune)`, `ctrlCKey`, `enterKey` (huhdlg_test.go), `stripANSI`, `testNow`, the `updateKey` + `driveCmds` huh cascade (huhdlg_test.go), `suiteType` (tui_suite_test.go), `pressCtrlP`/`pressCtrlA` (model_test.go), `testutil.Boot(t)` / `testutil.BootWithDriver` for the real-stack client; the teatest SGR goldens pin TTY_FORCE=1 + TERM=xterm-256color, with the opencode dark tokens quantized through x/ansi v0.11.8 Convert256 (the algorithm documented in session_theme_test.go).
   - SGR tokens (opencode dark, derived from assets/opencode.json through the Convert256 to6Cube rules — v&lt;48→0, v&lt;115→1, else (v-35)/40 — with the HSLuv cube-vs-grey tie-break documented in session_theme_test.go; the primary=216 + text=255/textMuted=244/SelectedForeground=232 derivations are confirmed by the existing homeSGRTokens goldens; the error/success indices are verified at execution by the scratch derive command in step 1 of each SGR task): error #e06c75 → **174**; primary #fab283 → **216**; success #7fd88f → **114**; textMuted #808080 → **244**; text #eeeeee → **255**. The assertions match SGR param substrings ("48;5;174", "38;5;114", …) — the pen-diff merge may reorder params inside one CSI, so substring match (the redSGR / logoSGRTokens precedent).

### Design decisions (binding)

**Select additions (shared by S3.1/S3.4/S3.8; land in S3.1):**
- `selectModel.skipFilter bool` + `selectModel.onFilter func(string)`: when `skipFilter`, the input row STILL renders (placeholder "Search"), but `syncFilter` no longer feeds `m.filter` (the fuzzy memo) — the list shows all enabled options (upstream dialog-select.tsx:155) and the raw typed text instead calls `onFilter(value)` (upstream: the input value change fires it, line 577). Zero values = today's behavior; the S2 goldens (model/agent/permission selects) stay byte-identical.
- `selectOption.bg string` (a token name; "" = none) + `selectOption.gutter string` (a fixed 2-rune leading column; "" = the default `  ` / `● ` logic). `rowLine`/`rowWithFooter`: `o.bg != ""` → the row paints that token's bg + `SelectedForeground(that bg)` fg (the active-row chain) regardless of the selection — the toDelete row (S3.1); `o.gutter != ""` → rendered in the leading column in place of the marker logic.
- `selectOption.category` (existing, S2.6) is reused for the S3.1 date buckets.

**S3.1 (session list):** new kind `dlgSessions` + payload `sessionsDlg{sel *selectModel, status map[string]string, toDelete string}`; modal `dlgMedium` (the upstream DialogSelect default size = medium 60).
- Options: `store.Sessions` sorted by `Time.Updated` desc (upstream `orderByRecency`; the parentID filter N/A — yolo has no parent sessions); category = "Today" (the updated date == today, compared via `sessionCategory(updated, now time.Time) string` — a pure function, the JS `toDateString` port = `t.Format("Mon Jan 2 2006")`) or that date string; title = `session.Title`; description = `session.Directory` (the rowLine tail wraps/truncates it — the upstream directory detail); value = the session ID; isCurrent = value == `a.curSessionID` (pre-selects the row — upstream `current`); gutter = `a.spinFrame()` + " " when the snapshot status is busy/retry, else "" (the default marker) — the upstream slot number has no yolo referent (deviation 189); the snapshot is a one-shot `client.Status()` at open (store.Status is current-session-only — deviation 190): while the dialog is open and the CURRENT session is busy, the busy tick (app.go:311-313) advances the frames; other sessions' frames are static (the snapshot).
- skipFilter=true; onFilter = the client-side case-insensitive **title substring** filter (deviation 190: upstream skipFilter+onFilter = the server-side `session.list({search})` — the yolo wire has no search endpoint (frozen), so onFilter ports the upstream memo's client-side half: rebuild the filtered option list by `strings.Contains(strings.ToLower(title), needle)`).
- `preserveSelection`: while the dialog is open, a store event that mutates `Sessions` (session.updated/deleted — via the `EventMsg` hook `syncSessionSel`) re-anchors the selection at the same session ID (mirroring `syncModelSel`); if the ID is gone, clamp to the last row.
- Two-step delete (action key ctrl+d — the upstream session_delete default): the first press sets `toDelete = the selected id` → the row title becomes "Press ctrl+d again to confirm" + `bg "error"` (rebuild the options); onMove clears `toDelete` (upstream onMove); the second press on the same row issues `sessionDeleteCmd(id)`; on success → close the dialog (if the deleted session was the current route → routeHome + hydrate); on error → `openDeleteFailedDialog(id, title, err.Error())` (S3.3).
- The rename action key ctrl+r (the upstream session_rename default) → `openSessionRenameDialog(id)` (S3.2).
- enter on a row → `openSession(id)` + close + hydrate (upstream onSelect navigates to the session).
- The opener: `/sessions` (the upstream slashName; a local-merged slash command — the shared slash-merge decision below).
- **Deferred (deviation 189, low):** pin/unpin (ctrl+f), quick-switch 1-9, the slot gutter, the "Pinned" category — they require the upstream local session store (`context/local.tsx` KV) which yolo lacks; no new deps; the S4 registry carries the commands should they land.

**S3.2 (session rename):** openers: (a) the session-list ctrl+r action (S3.1); (b) the session route ctrl+r — an early binding in `handleSessionKey` (a `Rename` entry in `sessKeyMap`, matched before the esc branch; only when `a.curSessionID != ""`); the S4.2 registry owns the key later. Flow: `openSessionRenameDialog(id)` pushes a `dlgForm` modal `dlgMedium` via `openFormModal(buildInputForm(a.theme, "Rename Session", "", "Title", currentTitle), dlgMedium, onConfirm, nil)`; onConfirm: `value := f.GetString("value")` — empty → close only (the upstream `if (!value) return` guard); else `a.emit(a.renameCmd(id, value))`; `renameMsg{id, title string; err error}`; `applyRename`: err → toast (the error string); success → `store.upsertSession` in place (the new title; `Current` refreshed when it is the current session) + NO toast (upstream parity — update({title}) + clear).

**S3.3 (session delete failed):** new kind `dlgDeleteFailed` + payload `deleteFailedDlg{id, title, errMsg string, active int}`; modal `dlgMedium`.
- View: header "Failed to Delete Session" (bold) + "esc" (muted); body muted, wrapped at the panel width via `wrapLine`: `The session "<title>" could not be deleted: <errMsg>` + a blank line + `Choose how to proceed.`; two option rows (pill boxes): "Retry delete" / muted description "Try to delete the session again." and "Keep session" / "Cancel the delete and keep the session."; the active row paints the primary bg + `SelectedForeground(primary)` fg (the select active-row chain); the inactive rows plain.
- Keys: enter = confirm the active (0 → re-issue `sessionDeleteCmd(id)` — success closes the dialog; error refreshes the payload with the new errMsg and re-renders; 1 → close + `a.emit(a.hydrateCmd())` to re-sync the route); left/up → active 0, right/down → active 1 (no wrap — the two-row clamp); esc/ctrl+c → close (nothing re-hydrates — the store is already consistent through the SSE events).
- Trigger: any DELETE failure (404/500) from the S3.1 two-step delete (replaces the upstream toast + workspace-recover flow).
- **Deviation 191 (low):** the upstream options ("Delete workspace" / "Restore to new workspace") have no yolo referent (no workspaces; the server delete fails 404/500 only, no busy-409) — the shape/keys port verbatim; the options + body adapt to "Retry delete" / "Keep session" + the session title + the wire error.

**S3.4 (provider dialog):** a NEW surface — yolo has no pre-S3 provider dialog (parity note: "restyle" = bringing the provider surface to the upstream shape). New kind `dlgProvider` + payload `providerDlg{sel *selectModel}`; modal `dlgMedium` (the upstream DialogSelect default).
- Options: `store.Providers` (seeded by `fetchCatalogCmd` — reused; `syncProviderSel` mirrors `syncModelSel`), sorted by (priority, name lc, id) with the ported `PROVIDER_PRIORITY {opencode:0, opencode-go:1, openai:2, github-copilot:3, anthropic:4, google:5}`, unknown → 99; category "Popular" (priority &lt; 99) | "Providers"; a description map ported for the known ids, "" for the unknown (the yolo ids mostly land in "Providers" — no description); a trailing "Other" option (value `__yolo_custom_provider__` — the yolo constant; the upstream `__opencode_custom_provider__`).
- Row footer: `providerStatusText(provider.Auth)` (the existing select-footer tail); no isCurrent (the upstream provider select has none).
- onSelect: known provider → the auth-method select is SKIPPED (the yolo wire has no oauth — the client Auth is API-key-only; deviation 192): the API-key form = `buildInputForm(a.theme, "API key", "API key for <provider id>", "API key", "")`; onConfirm: empty → return (the upstream guard `if (!value) return`); else `a.emit(a.authCmd(id, key, false))`.
- Custom flow: `normalizeCustomProviderID(s) string` (the ported regex `^[a-z0-9][a-z0-9-_]*$` + the `@ai-sdk/` strip; "" when invalid) on the "Other" option's id prompt (`buildInputForm(a.theme, "Other", "", "Provider id", "")` — the upstream DialogPrompt "Other"); invalid → the error toast, upstream message verbatim ("Provider ids must start with a lowercase letter or number and only use lowercase letters, numbers, hyphens, and underscores") + the id prompt re-opens (the upstream `return promptCustomProviderID()` re-prompt); valid → the API-key form with the custom description "This only stores a credential. Configure the provider in yolo.jsonc to use it." (the upstream message adapted to yolo.jsonc).
- `authMsg{providerID string; custom bool; err error}`; `applyAuth`: success + the id in the catalog → close + `openModelDialog()` (upstream `dialog.replace(DialogModel)`); success + custom → the info toast `Saved credential for <id>. Configure it in yolo.jsonc to use it.` + close; error → toast (the error string) + the dialog stays (the upstream stays on the api-key failure).
- The opener: `/connect` (the upstream slashName; the keybind is "none" — no key in S3; the S4.1 registry carries it).
- **Deviation 192 (low):** no oauth wire endpoints (the client is API-key-only) → the auth-method select is not ported; the upstream console-managed provider metadata (descriptions beyond the known ids) is not ported (no wire referent).

**S3.5 (status dialog):** new kind `dlgStatus`; modal `dlgMedium`; a static view (no payload state).
- "Status" bold + "esc" muted (the header row, space-between at the panel width).
- Sections (the upstream MCP/LSP/formatters/plugins have no yolo wire → the ported content, deviation 193): **Providers** — the count header `N Providers` (text token); a bullet row per provider: the status-colored bullet via `providerStatus` (loaded→success, missing→error, else→textMuted — the existing mapping, dialog.go:674) + the bold name + the muted `providerStatusText` detail; the fallback "No Providers" (text token). **Agents** — the count header `N Agents`; a bullet row per agent: the success bullet + the bold name + the muted description; the fallback "No Agents". No session section (the footer owns the session status).
- Keys: esc/ctrl+c close (via the stack); every other key ignored (the payload owns the keys — the restructured `handleDialogKey`).
- The opener: `/status` (the upstream slashName "status"; the keybind &lt;leader&gt;s lands in S4.1).
- **Deviation 193 (low):** the content is providers + agents (the upstream sections have no yolo wire endpoints).

**S3.6 (help restyle):** `dlgHelp` becomes **modal** (pushed via `pushModal(dialog{kind: dlgHelp}, dlgMedium, nil)`; the `viewModal`/`modalInner` case).
- Upstream shape (ui/dialog-help.tsx): "Help" bold + "esc/enter" muted; the body muted = the palette line `Press <shortcut> to see all available actions and commands in any context.` (the shortcut from the `paletteShortcut()` accessor — below) + the locked yolo V1 note line `pgup/pgdn scroll · \+enter newline` (the AGENTS.md V1 pin, preserved in the modal body); a right-aligned "ok" pill (pad 0 3, primary bg, `SelectedForeground` fg — the opencode token: selectedListItemText is absent → the SelectedForeground fallback; under the pinned test env the pill bg asserts as 48;5;216 in the teatest leg).
- Keys: enter/esc/ctrl+c close; every other key ignored (was: any key closes).
- `paletteShortcut()` accessor (new method): pre-S4 returns the yolo constant `"ctrl+p"` (the upstream default S4.1 ports verbatim — note: today yolo's ctrl+p opens the model dialog; the remap lands in S4.1). S4.7 rewires the accessor to the keymap registry (the frozen S4 table row); the S3.6 detail defines the consumption contract: the help view calls `a.paletteShortcut()` exactly once per render.
- Supersedes the S2 "help non-modal" note: /help becomes modal (upstream parity); the quit dialog stays non-modal + locked (yolo-specific — the upstream has no quit dialog). The old `helpDialog(th)` markdown table is dropped; `quitDialogRendered` is unchanged. Re-baseline the help tests (help_test.go).
- **Deviation 195 (info):** the palette shortcut is driven through the `paletteShortcut()` accessor — the pre-S4 constant "ctrl+p"; S4.7 rewires it to the registry.

**S3.7 (retry action):** the ported component `retryDlg{title, message, actionLabel string, selected int}` (0 = dismiss/"don't show again", 1 = action); new kind `dlgRetryAction`; modal `dlgMedium`.
- View: title bold + "esc" muted; the message muted, wrapped; (the link line: plain centered muted — no BgPulse dep ported; unused by the yolo trigger); pills left "don't show again" / right the actionLabel; starts selected on the action (upstream `selected="action"`); the selected pill paints the primary bg + `SelectedForeground` fg.
- Keys: left/right/tab toggle; enter confirms (the action → close + `a.emit(a.abortCmd())` — the wire exists; the turn lands on the existing aborted flow; dismiss → close); esc dismisses (close).
- Trigger: the current-session `session.status` wire event on the **idle→retry transition** (once per turn): in the `EventMsg` case (app.go:164), after `store.Apply`, the new hook `a.onSessionStatus(m.Event)`: the event is a `session.status` for `a.curSessionID`, the previous `store.Status.Type` was "idle", the new type is "retry", and the per-run suppression is not set → open the dialog. Title "Request failed"; message = the wire `Message` + ` (retrying, attempt <n>)`; actionLabel "Abort". Suppression: `a.retrySuppressed map[string]bool` (sessionID → true after ANY dismiss/action; cleared on the next send for that session) — the upstream 24h KV window + dontShow KV is not ported (the theme KV is theme-owned — S0 scoping; no new KV surface).
- **Deviation 194 (low):** the in-memory per-run gate replaces the upstream KV 24h/dontShow gate; the action pill is "Abort" (the yolo wire) instead of the upstream upsell link; the link line is unused.

**S3.8 (theme list):** new kind `dlgThemes` + payload `themeDlg{sel *selectModel, initial string, confirmed bool}`; modal `dlgMedium` (the upstream DialogSelect default).
- `a.engine == nil` → toast "theme engine unavailable" (the zero-engine degradation; the tests use a real engine via t.TempDir — parity note).
- Options: the keys of `a.engine.AllThemes()` sorted case-insensitively (`sort.Slice` on `strings.ToLower` — the upstream localeCompare port); title = the theme name; no description/category; isCurrent = `a.engine.Active()`.
- onMove → `a.engine.Set(name)` + `a.retheme()` (live preview; `Set` persists immediately — the upstream `theme.set` behavior); onSelect → Set + retheme + `confirmed = true` + close; onFilter (skipFilter=true, as S3.1): "" → Set(initial)+retheme + re-anchor the selection at the initial; else → sel=0 + Set(first filtered)+retheme (the upstream onFilter); the stack close callback (onClose): !confirmed → Set(initial)+retheme (the upstream onCleanup).
- The select placeholder "Search".
- The opener: the `/themes` slash (the upstream slashName "themes"; the keybind &lt;leader&gt;t lands in S4.1).

**S3.9 (theme commands):** "KV wiring" = the end-to-end engine KV chain on the theme surface: `Set`/`Pin`/`Free`/`Apply` persist to the KV file and `retheme` picks up the active theme after a `Set` (the S0.7 wiring, verified + extended by the S3.8 dialog).
- `themeSwitchMode()` = `a.engine.Pin(the opposite of a.engine.Mode())` (the `setMode` === pin quirk, verbatim); the dynamic titles: "Switch to light mode" / "Switch to dark mode" (the titles exist as the `switchModeTitle(e *theme.Engine) string` helper for the S4 registry + the unit tests).
- `themeModeLock()` = `a.engine.Locked() ? a.engine.Free() : a.engine.Pin(a.engine.Mode())` (upstream `locked() ? free() : lock()`); the titles "Unlock theme mode" / "Lock theme mode" (`modeLockTitle(e *theme.Engine) string`).
- **No default keys** (upstream both "none", keybind.ts:79-80) — the functions + the unit tests land now; the S4.1 registry ports the "none" defaults + the remap. **Deviation 196 (low).**
- `engine == nil` → toast "theme engine unavailable".

**Shared — client-side slash merge (lands in S3.1, consumed by S3.4/S3.5/S3.9):** `localCommands() []protocol.Command` = `{"/sessions", "List all sessions"}, {"/connect", "Connect a provider"}, {"/status", "View status"}, {"/themes", "List available themes"}` — merged with `store.Commands` at the view.go:33 call site (the local ones first; no name collision with the frozen 5), and `runCommand` gains the cases: `/sessions` → `openSessionListDialog()`; `/connect` → `openProviderDialog()`; `/status` → `openStatusDialog()`; `/themes` → `openThemeListDialog()`. The core is unchanged (the wire catalog stays frozen at 5 — spec §10).

---

### Task S3.1: Session-list dialog (on select) (bead `yolo-oae.4.1`, expected id `yolo-oae.4.2`)

**Files:** new `internal/tui/sessionsdlg.go`; modify `internal/tui/dialog.go` (the `dlgSessions` kind, the `sessions *sessionsDlg` payload field, the `dialogStack.sessions()` accessor, the `modalInner` case), `internal/tui/select.go` (the additive `selectOption.bg`/`selectOption.gutter` + `selectModel.skipFilter`/`selectModel.onFilter` fields, `syncFilter`, `rowLine`/`rowWithFooter`), `internal/tui/keys.go` (`handleDialogKey` routing for the new kind), `internal/tui/commands.go` (`runCommand` case + `localCommands()`), `internal/tui/view.go` (the menu merge at :33), `internal/tui/home_test.go` (the harness keys `ctrlDKey`/`ctrlRKey`); new `internal/tui/sessionsdlg_test.go`.

**Interfaces:** consumes S2.5–S2.7 (`selectModel`, `selectNew`, `WithActions`), S2.2 (the modal stack, `pushModal`, `dlgMedium`), `store.Sessions`, `openSession` (hydrate.go), `a.spinFrame()` (footer.go), `client.Status`/`client.DeleteSession`, `wrapLine` (wrap.go). Produces: `dlgSessions` kind; `sessionsDlg{sel *selectModel, status map[string]string, toDelete string}` with `handleKey(a *App, k tea.KeyPressMsg) []tea.Cmd` + `view(w, h int, th theme.Theme) string`; `App.openSessionListDialog() []tea.Cmd` (pushes the modal, builds the options, pre-selects the current session, emits the status-snapshot cmd); `App.syncSessionSel()` (the `EventMsg` hook — re-anchors the selection at the current ID on session.updated/deleted while open); `sessionOptions(a *App, status map[string]string) []selectOption`; `sessionCategory(updated, now time.Time) string`; `App.sessionDeleteCmd(id string) tea.Cmd` + `sessionDeleteMsg{err error}` + `App.applySessionDelete(m sessionDeleteMsg) []tea.Cmd`; the `selectOption.bg`/`.gutter` + `selectModel.skipFilter`/`.onFilter` additive fields (zero values byte-identical — the S2 goldens untouched); `localCommands()` + the `/sessions` menu/`runCommand` wiring; the `ctrlDKey`/`ctrlRKey` harness keys.

**Upstream parity notes:** `dialog-session-list.tsx` (read at detail time — findings §5): DialogSelect "Sessions", `skipFilter` + `onFilter` (the filter input still renders; upstream the typed text drives a SERVER search — the yolo wire has no search endpoint (frozen), so `onFilter` ports the client-side half of the upstream memo: case-insensitive title substring — deviation 190, low); `preserveSelection` (the selection re-anchors at the session ID across store events — `syncSessionSel`); the current session pre-selected (upstream `current`); `onMove` clears toDelete; the toDelete row: title "Press ctrl+d again to confirm" + bg=theme.error (the `selectOption.bg "error"` + SGR 48;5;174 under the pinned test env); category = the updated date's `toDateString` port, "Today" when today (the upstream "Pinned" bucket + the pin/quick-switch/slot-gutter surfaces are deferred — they need the upstream local session KV store (`context/local.tsx`) which yolo lacks — deviation 189, low); order = updated-desc (`orderByRecency`; the parentID filter N/A); the gutter = the spinner frame when the status snapshot is busy/retry, else the default marker (the upstream slot number has no yolo referent — deviation 189); the status snapshot is a one-shot `GET /session/status` at open (store.Status is current-session-only — deviation 190); delete success → close (routeHome + hydrate when the current session died); delete failure → the S3.3 dialog; enter → navigate to the session (`openSession` + close + hydrate); the footer actions delete (ctrl+d) + rename (ctrl+r) (the upstream pin action is deferred — deviation 189). The `/sessions` opener is the upstream slashName (a local-merged slash — the server catalog is frozen at 5, spec §10; the upstream resume/continue aliases are deferred with 189).

**Step 1 — write the failing tests.** New `internal/tui/sessionsdlg_test.go`:

```go
package tui

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// sessionListFixture: three sessions; the updated-desc order is s3, s2, s1
// (s1 is the current).
func sessionListFixture() []protocol.Session {
	mk := func(id, title string, updated int64) protocol.Session {
		return protocol.Session{
			ID: id, Title: title, Directory: "/work/" + id,
			Time: protocol.SessionTime{Created: updated - 60_000, Updated: updated},
		}
	}
	return []protocol.Session{
		mk("s1", "alpha", 1_000),
		mk("s2", "beta", 2_000),
		mk("s3", "gamma", 3_000),
	}
}

func openSessionsDlg(t *testing.T, s []protocol.Session) *recApp {
	t.Helper()
	a := testApp(s...)
	a.curSessionID = "s1"
	a.openSessionListDialog()
	a.Cmds = nil // the status-snapshot cmd (dummy client; not executed here)
	return a
}

func TestSessionCategory(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000)
	older := time.UnixMilli(1_700_000_000_000)
	tests := []struct {
		name    string
		updated time.Time
		want    string
	}{
		{"today", now, "Today"},
		{"older", older, older.Format("Mon Jan 2 2006")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := sessionCategory(tc.updated, now); got != tc.want {
				t.Fatalf("category = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSessionListDialogRender(t *testing.T) {
	t.Run("updated-desc order, title, search input, current marker", func(t *testing.T) {
		a := openSessionsDlg(t, sessionListFixture())
		got := stripANSI(a.dlg.sessions().view(80, 24))
		if !strings.Contains(got, "Sessions") || !strings.Contains(got, "Search") {
			t.Fatalf("title/placeholder missing:\n%s", got)
		}
		i3, i2, i1 := strings.Index(got, "gamma"), strings.Index(got, "beta"), strings.Index(got, "alpha")
		if i3 < 0 || i2 < 0 || i1 < 0 || !(i3 < i2 && i2 < i1) {
			t.Fatalf("rows not in updated-desc order (gamma < beta < alpha):\n%s", got)
		}
		if !strings.Contains(got, "●") {
			t.Fatalf("current-session gutter missing:\n%s", got)
		}
	})

	t.Run("skipFilter: typed text client-filters the titles", func(t *testing.T) {
		a := openSessionsDlg(t, sessionListFixture())
		a.handleKey(press('g')) // only "gamma" contains g
		got := stripANSI(a.dlg.sessions().view(80, 24))
		if !strings.Contains(got, "gamma") || strings.Contains(got, "beta") || strings.Contains(got, "alpha") {
			t.Fatalf("client-side filter did not narrow to gamma:\n%s", got)
		}
	})

	t.Run("enter opens the selected session and closes", func(t *testing.T) {
		a := openSessionsDlg(t, sessionListFixture())
		a.handleKey(press(tea.KeyDown)) // wrap to the first row (gamma)
		a.handleKey(press(tea.KeyEnter))
		if !a.dlg.empty() || a.curSessionID != "s3" {
			t.Fatalf("open failed: empty=%v cur=%s", a.dlg.empty(), a.curSessionID)
		}
		if len(a.Cmds) == 0 {
			t.Fatal("no hydrate cmd emitted after the open")
		}
	})

	t.Run("two-step delete: arm, onMove clears, confirm emits", func(t *testing.T) {
		a := openSessionsDlg(t, sessionListFixture())
		a.handleKey(ctrlDKey) // arm on the current selection (alpha)
		got := stripANSI(a.dlg.sessions().view(80, 24))
		if !strings.Contains(got, "Press ctrl+d again to confirm") {
			t.Fatalf("armed title missing:\n%s", got)
		}
		a.handleKey(press(tea.KeyUp)) // onMove clears the armed state
		if got := stripANSI(a.dlg.sessions().view(80, 24)); strings.Contains(got, "Press ctrl+d again to confirm") {
			t.Fatalf("onMove must clear the armed row:\n%s", got)
		}
		a.handleKey(ctrlDKey) // re-arm on the new selection (beta)
		a.handleKey(ctrlDKey) // confirm
		if len(a.Cmds) != 1 {
			t.Fatalf("confirm emitted %d cmds, want 1 (the delete)", len(a.Cmds))
		}
		if a.dlg.empty() {
			t.Fatal("the dialog must stay open until the delete resolves")
		}
	})
}

func TestSessionListDeleteResolves(t *testing.T) {
	t.Run("success closes and goes home when the current session dies", func(t *testing.T) {
		ts := testutil.Boot(t)
		c := client.New(ts.URL, ts.Dir)
		seed, err := c.CreateSession(context.Background())
		if err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
		a := newRecApp(c, store.State{Sessions: []protocol.Session{seed}, Current: &seed}, seed.ID)
		t.Cleanup(a.Close)
		a.openSessionListDialog()
		a.Cmds = nil
		a.handleKey(ctrlDKey)
		a.handleKey(ctrlDKey)
		driveCmds(t, a) // the delete cmd round-trips; applySessionDelete fires
		if !a.dlg.empty() {
			t.Fatalf("the dialog must close on success: depth=%d", len(a.dlg.items))
		}
		if a.route != routeHome {
			t.Fatalf("the deleted session was current: route = %v, want home", a.route)
		}
	})
}

// TestTUISessionListDialog is the teatest leg: the real stack + the real
// engine, /sessions opens the dialog, the two-step delete arms the
// error-background row. ONE merged terminal condition (the multi-token
// state must be a single WaitFor — the shared buffer drains per wait),
// and the pinned TTY env for the SGR assertion.
func TestTUISessionListDialog(t *testing.T) {
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

	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	seed, err := c.CreateSession(context.Background())
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	a := NewApp(c, store.State{}, seed.ID, e)
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a,
		teatest.WithInitialTermSize(80, 24),
		teatest.WithProgramOptions(tea.WithEnvironment([]string{"TTY_FORCE=1", "TERM=xterm-256color"})),
	)

	teatest.WaitFor(t, tm.Output(), hasLines("New session"), teatest.WithDuration(5*time.Second))
	suiteType(tm, "/sessions")
	tm.Send(press(tea.KeyEnter))
	// ONE merged condition: the dialog title + the session row + the
	// search input (skipFilter keeps it rendered).
	teatest.WaitFor(t, tm.Output(), hasLines("Sessions", "Search"), teatest.WithDuration(5*time.Second))

	tm.Send(ctrlDKey)
	tm.Send(ctrlDKey)
	// ONE merged condition: the armed title (plain) + the error-background
	// SGR param (48;5;174 = opencode dark error #e06c75 under the pinned
	// env — the Convert256 derivation, findings §6).
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return strings.Contains(stripANSI(string(b)), "Press ctrl+d again to confirm") &&
			bytes.Contains(b, []byte("48;5;174"))
	}, teatest.WithDuration(5*time.Second))

	tm.Send(press(tea.KeyEscape))
	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
```

Also add to `internal/tui/home_test.go` (the shared harness keys, next to `ctrlCKey`):

```go
var (
	ctrlDKey = tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl}
	ctrlRKey = tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl}
)
```

Step 1 also derives the error SGR index (the scratch command — /tmp/opencode is user data, NEVER used; a fresh scratch module):

```sh
mkdir -p /tmp/yolo-sgr && cd /tmp/yolo-sgr
cat > go.mod <<'EOF'
module sgr

go 1.25

require github.com/charmbracelet/x/ansi v0.11.8
EOF
cat > main.go <<'EOF'
package main

import (
	"fmt"
	"image/color"

	ansi "github.com/charmbracelet/x/ansi"
)

func main() {
	for name, rgb := range map[string][3]int{"error": {224, 108, 117}, "success": {127, 216, 143}, "primary": {250, 178, 131}} {
		idx := ansi.Convert256(color.RGBA{uint8(rgb[0]), uint8(rgb[1]), uint8(rgb[2]), 255})
		fmt.Printf("%s -> %d\n", name, int(idx))
	}
}
EOF
go mod tidy && go run .
```

Expected: `error -> 174`, `success -> 114`, `primary -> 216` (primary 216 is already pinned by the homeSGRTokens goldens; if the scratch output disagrees, the scratch output wins — re-pin the token in the test comment, same commit).

**Step 2 — confirm FAIL.** `go test ./internal/tui/ -run 'TestSession|TestTUISessionList' -count=1` → FAIL (build fails: undefined `openSessionListDialog`, `sessionCategory`, `dlgDeleteFailed`, `sessionsDlg`, `ctrlDKey`, `localCommands` — the expected red).

**Step 3 — minimal implementation.**
- `select.go`: add `bg`/`gutter` to `selectOption`; `skipFilter bool`/`onFilter func(string)` to `selectModel`. `syncFilter`: when `skipFilter`, skip the fuzzy feed — instead `if f := m.input.Value(); m.onFilter != nil { m.onFilter(f) }` (no `m.sel = 0` reset — the client filter re-anchors through the onFilter callback). `filtered()`: `if m.skipFilter { return the enabled options, unfiltered }`. `rowLine`/`rowWithFooter`: `o.bg != ""` → paint the row with that token's bg + `SelectedForeground(that bg)` fg (the active-row chain) regardless of `active`; `o.gutter != ""` → the leading column is `o.gutter` (in place of the `  `/`● ` logic).
- `sessionsdlg.go`: the payload + options builder + handlers per the Interfaces. `openSessionListDialog`: build `sessionOptions(a, nil)` (the status snapshot fills in via `applySessionStatusSnapshot` — a new msg `statusSnapshotMsg{status map[string]string; err error}` emitted by `fetchSessionStatusCmd`; on arrival: store the map + rebuild the options if the dialog is open), `selectNew("Sessions", "Search", opts, isCurrent, onSelect, onMove).WithActions(delete/rename).skipFilter = true` (set the fields — `selectNew` keeps its signature; set the additive fields after construction), pre-select the current session's row, `pushModal(dialog{kind: dlgSessions, sessions: d}, dlgMedium, nil)`, emit `a.fetchSessionStatusCmd()`. onMove: `d.toDelete = ""` + rebuild the row title (clear the armed state) — rebuild = re-set `d.sel.options` (the select re-renders on the next frame). onSelect (enter): `a.closeTopModal()` + `a.openSession(id)` + `a.emit(a.hydrateCmd())`. Actions: delete → `if d.toDelete == id { a.emit(a.sessionDeleteCmd(id)) } else { d.toDelete = id; rebuild }`; rename → `a.closeTopModal()` + `a.openSessionRenameDialog(id)` (S3.2 lands the opener; until then the case body is the close — the ctrl+r action is added in S3.2's commit, so here the actions list is delete-only; S3.2 appends the rename action). `applySessionDelete`: success → if the dialog is open, `a.closeTopModal()`; if `a.curSessionID == the deleted id` → `a.route = routeHome; a.curSessionID = ""` + `a.emit(a.hydrateCmd())`; error → `a.openDeleteFailedDialog(id, title, m.err.Error())` (the S3.3 stub: until S3.3 lands, the error path toasts — S3.3's commit wires the dialog; the test's failure leg lands in S3.3's step, and this task's `TestSessionListDeleteResolves/failure` subtest is MOVED to S3.3 — here step 1's file carries only the success leg; the failure leg is written in S3.3).
- `keys.go`: `handleDialogKey` — a new case before the legacy pop: `if top.kind == dlgSessions && top.sessions != nil { return top.sessions.handleKey(a, k) }` (the select consumes every key; esc/ctrl+c are consumed by the stack first — S2.2, unchanged).
- `commands.go`: `localCommands()` + the `runCommand` case `/sessions` → `a.openSessionListDialog()`; `view.go:33`: `menu := a.prompt.menuView(append(localCommands(), a.store.Commands...), w, a.theme)`.
- The `EventMsg` hook (app.go:164, after `syncPermDialog`): `if m.Event.Type == protocol.EventTypeSessionUpdated || m.Event.Type == protocol.EventTypeSessionDeleted { a.syncSessionSel() }` — `syncSessionSel`: if the top is dlgSessions, re-anchor `d.sel.sel` at the current session ID (clamped).

**Step 4 — gate.** `go test ./internal/tui/ -run 'TestSession|TestTUISessionList' -count=1` → PASS, then FULL gate: `go vet ./... && go test ./...` + `gofmt -l .` empty (the S2 select goldens — model/agent/permission — must stay byte-identical: the additive fields are zero-valued there).

**Step 5 — commit + close the bead.**
`git add internal/tui/sessionsdlg.go internal/tui/sessionsdlg_test.go internal/tui/dialog.go internal/tui/select.go internal/tui/keys.go internal/tui/commands.go internal/tui/view.go internal/tui/app.go internal/tui/home_test.go && git commit -m "feat: session-list dialog (on select)"`
`bd close <S3.1 bead> --reason "session list green: order, client filter, two-step delete, /sessions, teatest SGR" --json`

---

### Task S3.2: Session-rename dialog (themed huh input) (bead `yolo-oae.4.2`, expected id `yolo-oae.4.3`)

**Files:** new `internal/tui/rename.go`; modify `internal/tui/session.go` (the `Rename` sessKeyMap binding + the early case in `handleSessionKey`), `internal/tui/sessionsdlg.go` (the ctrl+r select action — the S3.1-stubbed rename action); new `internal/tui/rename_test.go`.

**Interfaces:** consumes `buildInputForm` + `openFormModal` (huhdlg.go, S2.3), `client.PatchSession`, `store.upsertSession`. Produces: `App.openSessionRenameDialog(id string) []tea.Cmd` (the current title seeds the input; the form modal `dlgMedium`); `App.renameCmd(id, title string) tea.Cmd` + `renameMsg{id, title string; err error}` + `App.applyRename(m renameMsg) []tea.Cmd`; the session-route `ctrl+r` binding (the `Rename` entry in `sessKeyMap`, matched in `handleSessionKey` before the esc branch, only when `a.curSessionID != ""`); the session-list ctrl+r action (appended to the S3.1 actions list, calling `openSessionRenameDialog`).

**Upstream parity notes:** `dialog-session-rename.tsx` (findings §5): DialogPrompt "Rename Session", initial value = the title, onConfirm → `update({title})` + clear the dialog; empty value → no-op (the `if (!value) return` guard — ported). NO success toast (upstream parity — yolo toasts on error only). The keybind ctrl+r matches the upstream `session_rename: ctrl+r` default; the S4.2 registry takes the binding over (the S3 binding is the early route binding — the S4 handoff).

**Step 1 — write the failing tests.** New `internal/tui/rename_test.go`:

```go
package tui

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
)

func renameApp(t *testing.T) (*recApp, string) {
	t.Helper()
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	seed, err := c.CreateSession(context.Background())
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if _, err := c.PatchSession(context.Background(), seed.ID, map[string]any{"title": "alpha"}); err != nil {
		t.Fatalf("PatchSession: %v", err)
	}
	a := newRecApp(c, store.State{Sessions: []protocol.Session{{ID: seed.ID, Title: "alpha"}}, Current: &protocol.Session{ID: seed.ID, Title: "alpha"}}, seed.ID)
	t.Cleanup(a.Close)
	a.size = tea.WindowSizeMsg{Width: 80, Height: 24}
	return a, seed.ID
}

func TestSessionRenameDialog(t *testing.T) {
	t.Run("confirm patches the title and closes, no toast", func(t *testing.T) {
		a, id := renameApp(t)
		a.openSessionRenameDialog(id)
		if got := a.dlg.form(); got == nil {
			t.Fatal("the form modal must be on top")
		}
		// typed text appends at the cursor (end of the initial title)
		updateKey(a, press(' '))
		updateKey(a, press('2'))
		updateKey(a, enterKey)
		driveCmds(t, a) // the submit cascade + the rename cmd round-trip
		if len(a.dlg.items) != 0 {
			t.Fatalf("the dialog must close: depth=%d", len(a.dlg.items))
		}
		if a.store.Sessions[0].Title != "alpha 2" {
			t.Fatalf("title = %q, want %q", a.store.Sessions[0].Title, "alpha 2")
		}
		if len(a.toasts) != 0 {
			t.Fatalf("no success toast (upstream parity), got %v", a.toasts)
		}
	})

	t.Run("esc cancels without patching", func(t *testing.T) {
		a, id := renameApp(t)
		a.openSessionRenameDialog(id)
		updateKey(a, press(tea.KeyEscape))
		driveCmds(t, a)
		if len(a.dlg.items) != 0 || a.store.Sessions[0].Title != "alpha" {
			t.Fatalf("cancel leaked: depth=%d title=%q", len(a.dlg.items), a.store.Sessions[0].Title)
		}
	})

	t.Run("empty title closes without patching (the upstream guard)", func(t *testing.T) {
		a, id := renameApp(t)
		a.openSessionRenameDialog(id)
		// backspace the whole initial title ("alpha" = 5 chars)
		for i := 0; i < 5; i++ {
			updateKey(a, press(tea.KeyBackspace))
		}
		updateKey(a, enterKey)
		driveCmds(t, a)
		if len(a.dlg.items) != 0 || a.store.Sessions[0].Title != "alpha" {
			t.Fatalf("empty-title guard failed: depth=%d title=%q", len(a.dlg.items), a.store.Sessions[0].Title)
		}
	})

	t.Run("session route ctrl+r opens the dialog", func(t *testing.T) {
		a, _ := renameApp(t)
		a.route = routeSession
		cmds := a.handleKey(ctrlRKey)
		if len(cmds) != 0 {
			t.Fatalf("ctrl+r must be consumed (no cmds), got %d", len(cmds))
		}
		if a.dlg.form() == nil {
			t.Fatal("ctrl+r must open the rename form")
		}
	})
}
```

**Step 2 — confirm FAIL.** `go test ./internal/tui/ -run 'TestSessionRename' -count=1` → FAIL (build fails: undefined `openSessionRenameDialog` — the expected red).

**Step 3 — minimal implementation.**
- `rename.go`:
  - `openSessionRenameDialog(id)`: look up the session title (from `store.Sessions`, falling back to `Current`); `form := buildInputForm(a.theme, "Rename Session", "", "Title", title)`; `openFormModal(form, dlgMedium, onConfirm, nil)` where onConfirm: `value := f.GetString("value")`; `if value == "" { return }` (the dialog is already closed by the cascade); `return a.emit(a.renameCmd(id, value))` — return the cmds through the huhFormDlg onConfirm signature: the onConfirm is `func(*App, *huh.Form)` returning nothing — so `a.emit(a.renameCmd(...))` directly (emit returns cmds the caller ignores; the form's onConfirm fires AFTER closeTopModal per huhdlg.go's cascade — the cmd lands in `a.Cmds` via the emitSink in tests / the app loop in production).
  - `renameCmd(id, title)`: the closure `client.PatchSession(ctx, id, map[string]any{"title": title})` → `renameMsg{id, title, err}`.
  - `applyRename(m)`: `m.err != nil` → `a.toast(m.err.Error())`; else update the matching `store.Sessions` entry's Title + `Current.Title` when it is the current session (no toast — upstream parity).
- `session.go`: `sessKeyMap` gains `Rename: key.NewBinding(key.WithKeys("ctrl+r"))`; `handleSessionKey` gains the case before the esc branch: `case key.Matches(k, sessKeyMap.Rename): if a.curSessionID == "" { return nil, false }; return a.openSessionRenameDialog(a.curSessionID), true`.
- `sessionsdlg.go`: the S3.1 actions list gains the rename action (`ctrl+r` key, title "rename" — the upstream action label; run = `a.closeTopModal()` + `a.openSessionRenameDialog(the selected id)`).

**Step 4 — gate.** `go test ./internal/tui/ -run 'TestSessionRename' -count=1` → PASS, then FULL gate: `go vet ./... && go test ./...` + `gofmt -l .` empty.

**Step 5 — commit + close the bead.**
`git add internal/tui/rename.go internal/tui/rename_test.go internal/tui/session.go internal/tui/sessionsdlg.go && git commit -m "feat: session-rename dialog (huh input)"`
`bd close <S3.2 bead> --reason "rename green: huh input seed, patch + close, empty guard, esc, ctrl+r (route + list action)" --json`

---

### Task S3.3: Session-delete-failed dialog (bead `yolo-oae.4.3`, expected id `yolo-oae.4.4`)

**Files:** new `internal/tui/deletefailed.go`; modify `internal/tui/dialog.go` (the `dlgDeleteFailed` kind, the `deleteFailed *deleteFailedDlg` payload field, the `dialogStack.deleteFailed()` accessor, the `modalInner` case), `internal/tui/keys.go` (the `handleDialogKey` case); new `internal/tui/deletefailed_test.go`.

**Interfaces:** consumes S2.2 (the modal stack, `dlgMedium`), `wrapLine` (wrap.go), the theme accessors, `sessionDeleteCmd` (S3.1), `a.hydrateCmd()`. Produces: `dlgDeleteFailed` kind; `deleteFailedDlg{id, title, errMsg string, active int}` with `view(w, h int, th theme.Theme) string` + `handleKey(a *App, k tea.KeyPressMsg) []tea.Cmd`; `App.openDeleteFailedDialog(id, title, errMsg string) []tea.Cmd` (modal, active=0, replaces the session-list dialog — `replaceModal` when the session list is on top, else `pushModal`); the S3.1 `applySessionDelete` error path rewired from the toast stub to `openDeleteFailedDialog`.

**Upstream parity notes:** `dialog-session-delete-failed.tsx` (findings §5): the active "delete"|"restore" two-option layout; return=confirm (a result of `false` from the action keeps the dialog open — the yolo port: a failed retry keeps the dialog open with the refreshed error), left/up = the first option, right/down = the second, esc close — the SHAPE AND KEYS port verbatim. **Deviation 191 (low):** the upstream options ("Delete workspace" / "Restore to new workspace") have no yolo referent (no workspaces; the server delete fails 404/500 only — no busy-409, findings §4) — the options + body adapt: "Retry delete" ("Try to delete the session again.") / "Keep session" ("Cancel the delete and keep the session.") + the body `The session "<title>" could not be deleted: <errMsg>` + `Choose how to proceed.`

**Step 1 — write the failing tests.** New `internal/tui/deletefailed_test.go` (plus the S3.1-deferred failure leg, moved here):

```go
package tui

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
	"github.com/kido5217/yolo/internal/tui/theme"
)

func openDeleteFailedDlg() *recApp {
	a := testApp()
	a.openDeleteFailedDialog("s1", "alpha", "session not found")
	return a
}

func TestDeleteFailedDialogRender(t *testing.T) {
	a := openDeleteFailedDlg()
	got := stripANSI(a.dlg.deleteFailed().view(80, 24))
	for _, tok := range []string{
		"Failed to Delete Session",
		`The session "alpha" could not be deleted: session not found`,
		"Choose how to proceed.",
		"Retry delete", "Keep session",
	} {
		if !strings.Contains(got, tok) {
			t.Fatalf("token %q missing:\n%s", tok, got)
		}
	}
}

func TestDeleteFailedDialogKeys(t *testing.T) {
	t.Run("right moves the active row, enter-keep closes and re-hydrates", func(t *testing.T) {
		a := openDeleteFailedDlg()
		a.handleKey(press(tea.KeyRight))
		if a.dlg.deleteFailed().active != 1 {
			t.Fatalf("active = %d, want 1", a.dlg.deleteFailed().active)
		}
		a.handleKey(press(tea.KeyEnter))
		if !a.dlg.empty() || len(a.Cmds) != 1 {
			t.Fatalf("keep must close + hydrate: empty=%v cmds=%d", a.dlg.empty(), len(a.Cmds))
		}
	})

	t.Run("left clamps at 0, enter-retry re-emits the delete", func(t *testing.T) {
		a := openDeleteFailedDlg()
		a.handleKey(press(tea.KeyLeft)) // already at 0: clamped
		if a.dlg.deleteFailed().active != 0 {
			t.Fatalf("active = %d, want 0 (clamped)", a.dlg.deleteFailed().active)
		}
		a.handleKey(press(tea.KeyEnter))
		if len(a.Cmds) != 1 || a.dlg.empty() {
			t.Fatalf("retry must re-emit + stay open: cmds=%d empty=%v", len(a.Cmds), a.dlg.empty())
		}
	})

	t.Run("esc closes without acting", func(t *testing.T) {
		a := openDeleteFailedDlg()
		a.handleKey(press(tea.KeyEscape))
		if !a.dlg.empty() || len(a.Cmds) != 0 {
			t.Fatalf("esc must close silently: empty=%v cmds=%d", a.dlg.empty(), len(a.Cmds))
		}
	})
}

// TestSessionListDeleteFailureOpensDlg is the S3.1-deferred leg: a delete
// that 404s on the server opens the delete-failed dialog with the wire
// error.
func TestSessionListDeleteFailureOpensDlg(t *testing.T) {
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	ghost := protocol.Session{ID: "ghost", Title: "ghost", Time: protocol.SessionTime{Updated: 1}}
	a := newRecApp(c, store.State{Sessions: []protocol.Session{ghost}}, "other")
	t.Cleanup(a.Close)
	a.openSessionListDialog()
	a.Cmds = nil
	a.handleKey(ctrlDKey)
	a.handleKey(ctrlDKey)
	driveCmds(t, a) // the server 404s the delete
	top, ok := a.dlg.top()
	if !ok || top.kind != dlgDeleteFailed {
		t.Fatalf("top = %v, want dlgDeleteFailed", top.kind)
	}
	got := stripANSI(top.deleteFailed.view(80, 24))
	if !strings.Contains(got, `The session "ghost" could not be deleted:`) {
		t.Fatalf("wire error missing from the body:\n%s", got)
	}
}

// TestTUIDeleteFailedDialog is the teatest SGR leg: the active option row
// paints the primary background (48;5;216 — opencode dark primary, the
// homeSGRTokens-pinned index).
func TestTUIDeleteFailedDialog(t *testing.T) {
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
	a := NewApp(client.New("http://127.0.0.1:9", ""), store.State{}, "", e)
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a,
		teatest.WithInitialTermSize(80, 24),
		teatest.WithProgramOptions(tea.WithEnvironment([]string{"TTY_FORCE=1", "TERM=xterm-256color"})),
	)
	a.openDeleteFailedDialog("s1", "alpha", "session not found")

	// ONE merged condition: the plain header + both option labels + the
	// active-row primary bg SGR param.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		s := stripANSI(string(b))
		return strings.Contains(s, "Failed to Delete Session") &&
			strings.Contains(s, "Retry delete") &&
			strings.Contains(s, "Keep session") &&
			bytes.Contains(b, []byte("48;5;216"))
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
```

**Step 2 — confirm FAIL.** `go test ./internal/tui/ -run 'TestDeleteFailed|TestSessionListDeleteFailure|TestTUIDeleteFailed' -count=1` → FAIL (build fails: undefined `openDeleteFailedDialog`, `deleteFailedDlg`, `dlgDeleteFailed` — the expected red).

**Step 3 — minimal implementation.**
- `deletefailed.go`: the payload + view + keys per the Interfaces. The view: the header row (bold title left, muted "esc" right, space-between at the panel width); the body lines wrapped via `wrapLine` at `w-4`; the option rows: the active row paints the primary bg + `SelectedForeground(primary)` fg (the select active-row chain, dialog.go's rowLine precedent); the inactive rows plain (text token + the muted description). `handleKey`: enter → active 0: `a.emit(a.sessionDeleteCmd(d.id))` (stay open — a failed retry re-enters through `applySessionDelete` and refreshes `errMsg`; a success closes it); active 1: `a.closeTopModal()` + `return a.emit(a.hydrateCmd())`; left/up → active 0; right/down → active 1 (the two-row clamp — no wrap); esc/ctrl+c → close (handled by the stack — S2.2).
- `dialog.go`: the kind + payload field + accessor + the `modalInner` case (`d.deleteFailed.view(w, h, th)`).
- `keys.go`: the `handleDialogKey` case (before the legacy pop): `if top.kind == dlgDeleteFailed && top.deleteFailed != nil { return top.deleteFailed.handleKey(a, k) }`.
- `sessionsdlg.go`: the S3.1 `applySessionDelete` error path: replace the toast stub with `a.openDeleteFailedDialog(id, title, m.err.Error())` (the title from the store session before the delete).
- `openDeleteFailedDialog`: `replaceModal(dialog{kind: dlgDeleteFailed, deleteFailed: d}, dlgMedium, nil)` when the session list is on top, else `pushModal(...)` (the upstream `dialog.replace` semantics).

**Step 4 — gate.** `go test ./internal/tui/ -run 'TestDeleteFailed|TestSessionListDeleteFailure|TestTUIDeleteFailed' -count=1` → PASS, then FULL gate: `go vet ./... && go test ./...` + `gofmt -l .` empty.

**Step 5 — commit + close the bead.**
`git add internal/tui/deletefailed.go internal/tui/deletefailed_test.go internal/tui/dialog.go internal/tui/keys.go internal/tui/sessionsdlg.go && git commit -m "feat: session-delete-failed dialog"`
`bd close <S3.3 bead> --reason "delete-failed green: verbatim shape/keys, adapted options (dev 191), retry/keep/esc, failure leg, teatest SGR" --json`

---

### Task S3.4: Provider dialog restyle (bead `yolo-oae.4.4`, expected id `yolo-oae.4.5`)
**Files:** new `internal/tui/providerdlg.go`; modify `internal/tui/dialog.go` (the `dlgProvider` kind, the `provider *providerDlg` payload field, the `dialogStack.provider()` accessor, the `modalInner` case), `internal/tui/keys.go` (the `handleDialogKey` case), `internal/tui/commands.go` (the `runCommand` case `/connect` — `localCommands` already carries it from S3.1); new `internal/tui/providerdlg_test.go`.

**Interfaces:** consumes S2.5–S2.7 (`selectModel`), S2.2 (the modal stack, `dlgMedium`), S2.3 (`buildInputForm`, `openFormModal`), `fetchCatalogCmd`/`applyCatalog` (S2.9, dialog.go), `store.Providers`, `client.Auth`, `openModelDialog` (S2.9). Produces: `dlgProvider` kind; `providerDlg{sel *selectModel}`; `App.openProviderDialog() []tea.Cmd` (pushes the modal + emits `fetchCatalogCmd`, mirroring `openModelDialog`); `App.syncProviderSel()` (builds/re-points the select on the catalog — the `syncModelSel` mirror); `providerOptions(st *store.State) []selectOption`; `providerPriority(id string) int`; `providerDescription(id string) string`; `const customProviderIDValue = "__yolo_custom_provider__"`; `normalizeCustomProviderID(s string) string` ("" when invalid); the auth flow: `App.authCmd(providerID, key string) tea.Cmd` + `authMsg{providerID string; custom bool; err error}` + `App.applyAuth(m authMsg) []tea.Cmd`; the `handleDialogKey` case.

**Upstream parity notes:** `dialog-provider.tsx` (findings §5): the ported `PROVIDER_PRIORITY` + the "Popular"|"Providers" categories + the (priority, name lc, id) sort + the known-id description map (the yolo ids are mostly unknown → 99 → "Providers", no description — a content note, no deviation); the trailing "Other" custom option (the upstream value `__opencode_custom_provider__` → the yolo const); the custom id regex + the `@ai-sdk/` strip (`normalizeCustomProviderID`, the invalid-id error toast is upstream-verbatim); the key prompt guards (empty value → return) + the saved-custom info toast (the upstream message adapted to yolo.jsonc); on known-provider success the dialog is replaced by the model dialog (upstream `dialog.replace(DialogModel)`). **Deviation 192 (low):** the yolo wire has no oauth endpoints (the client `Auth` is API-key-only) → the upstream auth-method select is not ported (the API-key form opens directly); the console-managed provider metadata (descriptions beyond the known ids) is not ported (no wire referent). The `/connect` opener is the upstream slashName (the upstream keybind is "none" — no key in S3; the S4.1 registry carries it).

**Step 1 — write the failing tests.** New `internal/tui/providerdlg_test.go`:

```go
package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
)

// providerFixtureS3: a known "popular" id, an unknown yolo id, and the
// custom option. (model_test.go's providerFixture is the S2 shape — this
// fixture is the S3 sort-order check.)
func providerFixtureS3() []protocol.Provider {
	return []protocol.Provider{
		{ID: "kido", Name: "Kido", Auth: &protocol.ProviderAuth{Status: "loaded"}},
		{ID: "anthropic", Name: "Anthropic", Auth: &protocol.ProviderAuth{RequiresKey: true, Status: "missing"}},
		{ID: "openai", Name: "OpenAI", Auth: &protocol.ProviderAuth{Status: "not-required"}},
	}
}

func openProviderDlg(t *testing.T) *recApp {
	t.Helper()
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	a.openProviderDialog()
	a.applyCatalog(catalogMsg{provs: providerFixtureS3(), agents: agentFixture()})
	a.Cmds = nil
	return a
}

func TestNormalizeCustomProviderID(t *testing.T) {
	tests := []struct{ in, want string }{
		{"my-provider", "my-provider"},
		{"my_provider", "my_provider"},
		{"9lives", "9lives"},
		{"@ai-sdk/openai", "openai"},
		{"@ai-sdk/openrouter", "openrouter"},
		{"OpenAI", ""},            // uppercase
		{"-leading", ""},          // leading hyphen
		{"a b", ""},               // space
		{"", ""},
	}
	for _, tc := range tests {
		if got := normalizeCustomProviderID(tc.in); got != tc.want {
			t.Fatalf("normalize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestProviderDialogRender(t *testing.T) {
	t.Run("priority order, categories, Other tail, status footers", func(t *testing.T) {
		a := openProviderDlg(t)
		got := stripANSI(a.dlg.provider().view(80, 24))
		if !strings.Contains(got, "Connect a provider") {
			t.Fatalf("title missing:\n%s", got)
		}
		// anthropic (1) + openai (2) sort before the unknown kido (99);
		// the custom option is last.
		iA, iO, iK, iC := strings.Index(got, "Anthropic"), strings.Index(got, "OpenAI"), strings.Index(got, "Kido"), strings.Index(got, "Other")
		if iA < 0 || iO < 0 || iK < 0 || iC < 0 || !(iA < iO && iO < iK && iK < iC) {
			t.Fatalf("order wrong (anthropic < openai < kido < Other):\n%s", got)
		}
		for _, cat := range []string{"Popular", "Providers"} {
			if !strings.Contains(got, cat) {
				t.Fatalf("category %q missing:\n%s", cat, got)
			}
		}
		for _, tok := range []string{"● loaded", "○ missing", "· not-required"} {
			if !strings.Contains(got, tok) {
				t.Fatalf("status footer %q missing:\n%s", tok, got)
			}
		}
	})

	t.Run("empty catalog shows the loading hint", func(t *testing.T) {
		ts := testutil.Boot(t)
		c := client.New(ts.URL, ts.Dir)
		a := newRecApp(c, store.State{}, "")
		t.Cleanup(a.Close)
		a.openProviderDialog()
		a.Cmds = nil
		got := stripANSI(a.dlg.provider().view(80, 24))
		if !strings.Contains(got, "loading…") {
			t.Fatalf("loading hint missing:\n%s", got)
		}
	})
}

func TestProviderDialogFlow(t *testing.T) {
	t.Run("known provider: key form, auth success replaces with the model dialog", func(t *testing.T) {
		a := openProviderDlg(t)
		// select "OpenAI" (the second row) and enter
		a.handleKey(press(tea.KeyDown))
		a.handleKey(press(tea.KeyEnter))
		if a.dlg.form() == nil {
			t.Fatal("the API-key form must be on top")
		}
		got := stripANSI(a.dlg.form().form.View())
		if !strings.Contains(got, "API key") {
			t.Fatalf("the key form title missing:\n%s", got)
		}
		updateKey(a, press('k'))
		updateKey(a, press('3'))
		updateKey(a, enterKey)
		driveCmds(t, a) // the submit cascade + the auth cmd round-trip
		if a.dlg.model() == nil {
			t.Fatalf("success must replace the dialog with the model dialog: top=%v", a.dlg.top().kind)
		}
	})

	t.Run("invalid custom id: the verbatim error toast, the id prompt re-opens", func(t *testing.T) {
		a := openProviderDlg(t)
		// navigate to the "Other" row (the last) and enter
		for i := 0; i < 3; i++ {
			a.handleKey(press(tea.KeyDown))
		}
		a.handleKey(press(tea.KeyEnter))
		if a.dlg.form() == nil {
			t.Fatal("the custom-id prompt must be on top")
		}
		updateKey(a, press('B')) // "B" -> invalid (uppercase)
		updateKey(a, enterKey)
		driveCmds(t, a)
		if len(a.toasts) != 1 || !strings.Contains(a.toasts[len(a.toasts)-1].msg,
			"Provider ids must start with a lowercase letter or number") {
			t.Fatalf("invalid-id toast wrong: %v", a.toasts)
		}
		// the id prompt re-opened (the upstream re-prompt)
		if a.dlg.form() == nil {
			t.Fatal("the id prompt must re-open after the invalid id")
		}
	})

	t.Run("custom provider: auth success toasts the saved-credential note and closes", func(t *testing.T) {
		a := openProviderDlg(t)
		for i := 0; i < 3; i++ {
			a.handleKey(press(tea.KeyDown))
		}
		a.handleKey(press(tea.KeyEnter))
		updateKey(a, press('m'))
		updateKey(a, press('y'))
		updateKey(a, enterKey)
		driveCmds(t, a) // the id prompt resolves -> the key form
		if a.dlg.form() == nil {
			t.Fatal("the key form must be on top after the valid id")
		}
		updateKey(a, press('k'))
		updateKey(a, enterKey)
		driveCmds(t, a)
		if !a.dlg.empty() {
			t.Fatalf("the custom flow must close: depth=%d", len(a.dlg.items))
		}
		if len(a.toasts) != 1 || !strings.Contains(a.toasts[0].msg,
			"Saved credential for my. Configure it in yolo.jsonc to use it.") {
			t.Fatalf("saved-credential toast wrong: %v", a.toasts)
		}
	})
}

// TestTUIProviderDialog is the teatest leg: /connect opens the dialog on the
// real stack (the provider list from the real catalog).
func TestTUIProviderDialog(t *testing.T) {
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))

	teatest.WaitFor(t, tm.Output(), hasLines("New session"), teatest.WithDuration(5*time.Second))
	suiteType(tm, "/connect")
	tm.Send(press(tea.KeyEnter))
	// ONE merged condition: the title + the custom tail (the status footers
	// depend on the real catalog — kept out of the condition).
	teatest.WaitFor(t, tm.Output(), hasLines("Connect a provider", "Other"), teatest.WithDuration(5*time.Second))

	tm.Send(press(tea.KeyEscape))
	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
```

**Step 2 — confirm FAIL.** `go test ./internal/tui/ -run 'TestProvider|TestNormalizeCustom|TestTUIProviderDialog' -count=1` → FAIL (build fails: undefined `openProviderDialog`, `normalizeCustomProviderID`, `dlgProvider` — the expected red).

**Step 3 — minimal implementation.**
- `providerdlg.go`: the payload + options + flow per the Interfaces. `providerOptions`: sort a copy of `st.Providers` by (priority, `strings.ToLower(Name)`, ID); the category from the priority (&lt; 99 → "Popular", else "Providers"); the description from the ported map (`{"opencode": "OpenCode's hosted models", "opencode-go": "OpenCode Go models", "openai": "OpenAI models", "github-copilot": "GitHub Copilot", "anthropic": "Anthropic models", "google": "Google models"}` — the upstream description map, ported; unknown → ""); the row footer = `providerStatusText(p.Auth)` (the select's existing footer field); the trailing `selectOption{Title: "Other", Description: "Custom provider id", Value: customProviderIDValue}`. `normalizeCustomProviderID`: strip the leading `@ai-sdk/`, then the regex `^[a-z0-9][a-z0-9-_]*$` (return "" on a mismatch). `syncProviderSel`: the `syncModelSel` mirror (nil sel + no providers → the "loading…" line via the select's own empty state? — no: `syncModelSel` leaves sel nil and the view renders the loading hint the same way; mirror it). `providerSelectPick` (onSelect): custom → the id prompt form (`buildInputForm(a.theme, "Other", "", "Provider id", "")`); known → the key form (`buildInputForm(a.theme, "API key", "API key for "+id, "API key", "")`). The onConfirm closures: id prompt → `normalizeCustomProviderID`: invalid → `a.toast(the verbatim message)` + re-open the id prompt (`a.replaceModal(...)` with a fresh form); valid → open the key form (replacing) with the custom description. Key form → empty value → return (the guard; the dialog is already closed by the cascade — re-open? the upstream `if (!value) return` inside the awaited onConfirm leaves the prompt OPEN (the form's StateCompleted already closed it via the cascade)… the yolo huhFormDlg cascade closes on StateCompleted regardless — so the empty guard must re-open the form (replaceModal with a fresh one) to honor the upstream "stay" — implementation note: onConfirm empty → `a.openKeyForm(id, custom)` again). Non-empty → `a.emit(a.authCmd(id, key))` (the dialog stays until applyAuth decides). `applyAuth`: err → `a.toast(m.err.Error())` (stay open); success + the id in `store.Providers` → `a.closeTopModal()` + `a.openModelDialog()`; success + custom → `a.toast("Saved credential for "+id+". Configure it in yolo.jsonc to use it.")` + `a.closeTopModal()`.
- `dialog.go`: the kind + payload field + accessor + the `modalInner` case.
- `keys.go`: the `handleDialogKey` case (the select consumes the keys; the form modal is the existing dlgForm path).
- `commands.go`: the `runCommand` case `/connect` → `a.openProviderDialog()` (`localCommands()` already carries the entry — S3.1).

**Step 4 — gate.** `go test ./internal/tui/ -run 'TestProvider|TestNormalizeCustom|TestTUIProviderDialog' -count=1` → PASS, then FULL gate: `go vet ./... && go test ./...` + `gofmt -l .` empty.

**Step 5 — commit + close the bead.**
`git add internal/tui/providerdlg.go internal/tui/providerdlg_test.go internal/tui/dialog.go internal/tui/keys.go internal/tui/commands.go && git commit -m "feat: provider dialog restyle"`
`bd close <S3.4 bead> --reason "provider green: priority sort, categories, Other/custom flow, key form, auth + model replace, /connect" --json`

---

### Task S3.5: Status dialog (bead `yolo-oae.4.5`, expected id `yolo-oae.4.6`)

**Files:** new `internal/tui/statusdlg.go`; modify `internal/tui/dialog.go` (the `dlgStatus` kind + the `modalInner` case), `internal/tui/keys.go` (the `handleDialogKey` case), `internal/tui/commands.go` (the `runCommand` case `/status` — `localCommands` already carries it from S3.1); new `internal/tui/statusdlg_test.go`.

**Interfaces:** consumes S2.2 (the modal stack, `dlgMedium`), the theme accessors, `providerStatus`/`providerStatusText` (dialog.go:661/674), `store.Providers`/`store.Agents`, `wrapLine` (wrap.go). Produces: `dlgStatus` kind (no payload — a static view); `App.openStatusDialog() []tea.Cmd` (`pushModal(dialog{kind: dlgStatus}, dlgMedium, nil)`); `App.statusView(w, h int, th theme.Theme) string` (the modalInner renderer); the `handleDialogKey` case (every key ignored except the stack's esc/ctrl+c close).

**Upstream parity notes:** `dialog-status.tsx` (findings §5): the header row (bold "Status" + muted "esc", space-between) + the per-section shape (the count header row, the status-colored bullet + bold name + muted detail rows, the "No X" fallback) port VERBATIM; the sections adapt — the upstream MCP/LSP/formatters/plugins have no yolo wire endpoints → the content is **Providers** (the bullet via `providerStatus`: loaded→success, missing→error, else→textMuted; the detail = `providerStatusText`) + **Agents** (the success bullet; the detail = the description) — **deviation 193 (low)**; no session section (the footer owns the session status). The `/status` opener is the upstream slashName "status" (the upstream keybind &lt;leader&gt;s lands in S4.1).

**Step 1 — write the failing tests.** New `internal/tui/statusdlg_test.go`:

```go
package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/store"
)

func openStatusDlg() *recApp {
	a := testApp()
	a.store.Providers = []protocol.Provider{
		{ID: "kido", Name: "Kido", Auth: &protocol.ProviderAuth{Status: "loaded"}},
		{ID: "anthropic", Name: "Anthropic", Auth: &protocol.ProviderAuth{RequiresKey: true, Status: "missing"}},
	}
	a.store.Agents = agentFixture() // build, plan, yolo (model_test.go)
	a.openStatusDialog()
	return a
}

func TestStatusDialogRender(t *testing.T) {
	t.Run("providers + agents sections with the status details", func(t *testing.T) {
		a := openStatusDlg()
		got := stripANSI(a.statusView(80, 24))
		if !strings.Contains(got, "Status") || !strings.Contains(got, "esc") {
			t.Fatalf("header missing:\n%s", got)
		}
		if !strings.Contains(got, "2 Providers") || !strings.Contains(got, "3 Agents") {
			t.Fatalf("count headers missing:\n%s", got)
		}
		for _, tok := range []string{"Kido", "● loaded", "Anthropic", "○ missing", "build", "plan", "yolo"} {
			if !strings.Contains(got, tok) {
				t.Fatalf("token %q missing:\n%s", tok, got)
			}
		}
	})

	t.Run("empty sections render the No-X fallbacks", func(t *testing.T) {
		a := testApp()
		a.openStatusDialog()
		got := stripANSI(a.statusView(80, 24))
		if !strings.Contains(got, "No Providers") || !strings.Contains(got, "No Agents") {
			t.Fatalf("fallbacks missing:\n%s", got)
		}
	})

	t.Run("only esc/ctrl+c close; other keys are ignored", func(t *testing.T) {
		a := openStatusDlg()
		if cmds := a.handleKey(press('x')); len(cmds) != 0 || a.dlg.empty() {
			t.Fatalf("a plain key must be ignored: cmds=%d empty=%v", len(cmds), a.dlg.empty())
		}
		a.handleKey(press(tea.KeyEscape))
		if !a.dlg.empty() {
			t.Fatal("esc must close the status dialog")
		}
	})
}
```

**Step 2 — confirm FAIL.** `go test ./internal/tui/ -run 'TestStatusDialog' -count=1` → FAIL (build fails: undefined `openStatusDialog`, `statusView` — the expected red).

**Step 3 — minimal implementation.**
- `statusdlg.go`: `openStatusDialog` + `statusView` per the Interfaces. `statusView`: the header row (bold "Status" left, muted "esc" right, space-between at `w`); the Providers section: the count header (`len(providers)` + " Providers", text token, skipped when 0 — the fallback "No Providers" instead); per provider a bullet row: the `providerStatus` dot+label (styled) + the bold name + the muted detail, wrapped via `wrapLine` at `w-4`; the Agents section the same shape (the success bullet "•", the bold name, the muted description; the fallback "No Agents").
- `dialog.go`: the kind + the `modalInner` case (`a.statusView(w, h, a.theme)` — note: the case calls the App method, not a payload method, since there is no payload).
- `keys.go`: the `handleDialogKey` case: `if top.kind == dlgStatus { return nil }` (the keys are ignored — consumed; esc/ctrl+c are handled by the stack BEFORE this point — S2.2).
- `commands.go`: the `runCommand` case `/status` → `a.openStatusDialog()`.

**Step 4 — gate.** `go test ./internal/tui/ -run 'TestStatusDialog' -count=1` → PASS, then FULL gate: `go vet ./... && go test ./...` + `gofmt -l .` empty.

**Step 5 — commit + close the bead.**
`git add internal/tui/statusdlg.go internal/tui/statusdlg_test.go internal/tui/dialog.go internal/tui/keys.go internal/tui/commands.go && git commit -m "feat: status dialog"`
`bd close <S3.5 bead> --reason "status green: verbatim section shape, providers+agents content (dev 193), No-X fallbacks, /status" --json`

---

### Task S3.6: Help dialog restyle (keymap-registry-driven) (bead `yolo-oae.4.6`, expected id `yolo-oae.4.7`)

**Files:** modify `internal/tui/dialog.go` (the `helpDialog` replacement — the new modal help view + the `paletteShortcut()` accessor; the `dlgHelp` kind stays, its render + push path flip to modal), `internal/tui/commands.go` (`runCommand` `/help` → `pushModal`), `internal/tui/view.go` (the `viewModal`/`modalInner` case for `dlgHelp`), `internal/tui/keys.go` (the `handleDialogKey` case — enter/esc/ctrl+c close, other keys ignored); re-baseline `internal/tui/help_test.go` (all legs), `internal/tui/tui_suite_test.go` (the `TestTUIDialogs` help leg — the capture tokens change from the markdown table to the upstream shape).

**Interfaces:** consumes S2.2 (the modal stack, `dlgMedium`), the theme accessors (`Primary()`, `SelectedForeground()`, `Text()`/`TextMuted()`), the `title` style (style.go), `lipgloss` padding. Produces: `App.paletteShortcut() string` (the pre-S4 constant `"ctrl+p"` — the S4.7 rewires this accessor to the keymap registry; the consumption contract: the help view calls it exactly once per render); `helpDialogView(a *App, w, h int, th theme.Theme) string` (replaces `helpDialog(th)`); the modal push for `/help`; the teatest SGR golden (the "ok" pill primary bg 48;5;216).

**Upstream parity notes:** `ui/dialog-help.tsx` (findings §5): the shape ports verbatim — "Help" bold + "esc/enter" muted; the body muted `Press <shortcut> to see all available actions and commands in any context.`; the right-aligned "ok" pill (pad 0 3, primary bg, selectedListItemText fg — the opencode token has no selectedListItemText → the `SelectedForeground` fallback, the homeSGRTokens-pinned 38;5;232 fg + 48;5;216 bg under the pinned test env); return/escape close (every other key ignored — the yolo pre-S3 "any key closes" behavior is dropped). The body ADDS the locked yolo V1 note line `pgup/pgdn scroll · \+enter newline` (the AGENTS.md V1 pin — preserved from the pre-S3 `helpDialog` body; the upstream has no yolo note). **Deviation 195 (info):** the palette shortcut is driven through the `paletteShortcut()` accessor — the pre-S4 yolo constant "ctrl+p" (the upstream default S4.1 ports verbatim; today yolo's ctrl+p opens the model dialog — the S4.1 remap); S4.7 rewires the accessor to the registry. Supersedes the S2 "help non-modal" note: /help is now modal (upstream parity); the quit dialog stays non-modal + locked (yolo-specific — the upstream has no quit dialog). The old markdown table (the `| key | action |` rows) is DROPPED — the table's keymap lines are superseded by the S4.7 registry-driven rendering (the frozen S4 table row).

**Step 1 — re-write the help tests (failing).** Replace `internal/tui/help_test.go`:

```go
package tui

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
	"github.com/kido5217/yolo/internal/tui/theme"
)

func TestHelpDialogView(t *testing.T) {
	a := testApp()
	a.pushModal(dialog{kind: dlgHelp}, dlgMedium, nil)
	got := stripANSI(a.helpDialogView(a, 80, 24))
	for _, tok := range []string{
		"Help",
		"esc/enter",
		"Press ctrl+p to see all available actions and commands in any context.",
		"pgup/pgdn scroll \u00B7 \\+enter newline",
		"ok",
	} {
		if !strings.Contains(got, tok) {
			t.Fatalf("token %q missing:\n%s", tok, got)
		}
	}
	// the pre-S3 markdown table is gone
	for _, gone := range []string{"| enter |", "| pgup |"} {
		if strings.Contains(got, gone) {
			t.Fatalf("stale table token %q still present:\n%s", gone, got)
		}
	}
}

func TestHelpDialogKeys(t *testing.T) {
	a := testApp()
	a.pushModal(dialog{kind: dlgHelp}, dlgMedium, nil)
	// a plain key is ignored (pre-S3: any key closed)
	if cmds := a.handleKey(press('x')); len(cmds) != 0 || a.dlg.empty() {
		t.Fatalf("a plain key must be ignored: cmds=%d empty=%v", len(cmds), a.dlg.empty())
	}
	// enter closes
	a.handleKey(press(tea.KeyEnter))
	if !a.dlg.empty() {
		t.Fatal("enter must close the help dialog")
	}
	// esc closes
	a.pushModal(dialog{kind: dlgHelp}, dlgMedium, nil)
	a.handleKey(press(tea.KeyEscape))
	if !a.dlg.empty() {
		t.Fatal("esc must close the help dialog")
	}
}

// TestTUIHelpDialog is the teatest SGR golden: the modal help on the real
// stack — the "ok" pill paints the primary bg (48;5;216) + the
// SelectedForeground fg (38;5;232 — the homeSGRTokens-pinned indices).
func TestTUIHelpDialog(t *testing.T) {
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

	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	a := NewApp(c, store.State{}, "", e)
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a,
		teatest.WithInitialTermSize(80, 24),
		teatest.WithProgramOptions(tea.WithEnvironment([]string{"TTY_FORCE=1", "TERM=xterm-256color"})),
	)

	teatest.WaitFor(t, tm.Output(), hasLines("New session"), teatest.WithDuration(5*time.Second))
	suiteType(tm, "/help")
	tm.Send(press(tea.KeyEnter))
	// ONE merged condition: the plain header + the palette line + the V1
	// note + the ok pill's SGR params.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		s := stripANSI(string(b))
		return strings.Contains(s, "Help") &&
			strings.Contains(s, "Press ctrl+p to see all available actions") &&
			strings.Contains(s, "pgup/pgdn scroll") &&
			bytes.Contains(b, []byte("48;5;216")) &&
			bytes.Contains(b, []byte("38;5;232"))
	}, teatest.WithDuration(5*time.Second))

	tm.Send(press(tea.KeyEscape))
	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
```

AND re-baseline the `TestTUIDialogs` help leg in `tui_suite_test.go`: the capture `capture("Help", "| enter | send prompt |", "pgup/pgdn scroll \u00B7 \\+enter newline")` becomes `capture("Help", "Press ctrl+p to see all available actions", "pgup/pgdn scroll \u00B7 \\+enter newline")` (the markdown-table token is gone; the esc-close step is unchanged).

**Step 2 — confirm FAIL.** `go test ./internal/tui/ -run 'TestHelpDialog|TestTUIHelpDialog|TestTUIDialogs' -count=1` → FAIL (build fails: undefined `helpDialogView`, `paletteShortcut` — the expected red; the re-baselined `TestTUIDialogs` leg fails on the token until the view flips).

**Step 3 — minimal implementation.**
- `dialog.go`: drop `helpDialog(th)` (the markdown table); add `helpDialogView(a *App, w, h int, th theme.Theme) string`: the header row (bold "Help" left, muted "esc/enter" right, space-between at `w`); the body muted: the palette line `Press ` + `a.paletteShortcut()` + ` to see all available actions and commands in any context.` + a blank line + the V1 note line `pgup/pgdn scroll · \+enter newline` (the locked text — the S2 pin); the "ok" pill: `lipgloss.NewStyle().Padding("0", "3").Background(primary).Foreground(SelectedForeground-fg)` right-aligned at `w` (the pad 0 3 + the primary bg + the selectedListItemText fg port; the fg = `th.SelectedForeground()` — the opencode fallback, the homeSGRTokens 38;5;232 derivation). Add `func (a *App) paletteShortcut() string { return "ctrl+p" }` (the pre-S4 constant — the S4.7 rewires it; the comment notes the S4 handoff).
- `commands.go`: the `runCommand` case `/help`: `a.pushModal(dialog{kind: dlgHelp}, dlgMedium, nil)` (was the non-modal `a.dlg.push`).
- `view.go`: the `modalInner` case for `dlgHelp` → `a.helpDialogView(a, w, h, a.theme)`.
- `keys.go`: the `handleDialogKey` case for `dlgHelp`: enter → `a.closeTopModal()`; every other key → `return nil` (consumed, ignored — esc/ctrl+c are the stack's, S2.2).

**Step 4 — gate.** `go test ./internal/tui/ -run 'TestHelpDialog|TestTUIHelpDialog|TestTUIDialogs' -count=1` → PASS, then FULL gate: `go vet ./... && go test ./...` + `gofmt -l .` empty.

**Step 5 — commit + close the bead.**
`git add internal/tui/dialog.go internal/tui/commands.go internal/tui/view.go internal/tui/keys.go internal/tui/help_test.go internal/tui/tui_suite_test.go && git commit -m "feat: help dialog restyle (keymap-driven)"`
`bd close <S3.6 bead> --reason "help green: modal upstream shape, ok pill SGR, paletteShortcut accessor (dev 195), table dropped, re-baselined" --json`

---

### Task S3.7: Retry-action dialog (bead `yolo-oae.4.7`, expected id `yolo-oae.4.8`)

**Files:** new `internal/tui/retrydlg.go`; modify `internal/tui/dialog.go` (the `dlgRetryAction` kind, the `retry *retryDlg` payload field, the `dialogStack.retryAction()` accessor, the `modalInner` case), `internal/tui/keys.go` (the `handleDialogKey` case), `internal/tui/app.go` (the `EventMsg` hook `onSessionStatus` + the `retrySuppressed` field); new `internal/tui/retrydlg_test.go`.

**Interfaces:** consumes S2.2 (the modal stack, `dlgMedium`), the theme accessors, `a.abortCmd()` (session.go — the existing wire Abort), `store.Status` (the previous-type read), `protocol.EventTypeSessionStatus` + `SessionStatusProps` (protocol/event.go). Produces: `dlgRetryAction` kind; `retryDlg{title, message, actionLabel string; selected int}` (0 = dismiss/"don't show again", 1 = action) with `view(w, h int, th theme.Theme) string` + `handleKey(a *App, k tea.KeyPressMsg) []tea.Cmd`; `App.openRetryActionDialog(title, message, actionLabel string) []tea.Cmd` (modal, starts selected on the action); `App.onSessionStatus(ev protocol.Event)` (the idle→retry-transition hook); `App.retrySuppressed map[string]bool` (sessionID → suppressed this run; cleared on the next send for that session — the `applySend` success path).

**Upstream parity notes:** `dialog-retry-action.tsx` (findings §5): the component ports verbatim — the title bold + "esc" muted; the message muted; the pills left "don't show again" / right the action; starts selected on the action; left/right/tab toggle; return confirms; esc dismisses. The upstream TRIGGER differs (the Go upsell on free_tier_limit/account_rate_limit, the 24h KV + dontShow KV gate, only when the dialog stack is empty) — the yolo trigger is the current-session **idle→retry transition** on the `session.status` wire event (the attempts 1..3 the server emits before the attempt-4 exhaustion — findings §4): title "Request failed", the message = the wire `Message` + ` (retrying, attempt <n>)`, the action = "Abort" (the wire Abort exists). **Deviation 194 (low):** the in-memory per-run gate (`retrySuppressed`, cleared on the next send) replaces the upstream KV 24h/dontShow gate (the theme KV is theme-owned — S0 scoping; no new KV surface); the action pill is "Abort" (the yolo wire) instead of the upstream upsell link; the link line is unused (no BgPulse dep). The dialog opens even with other dialogs on the stack (the upstream empty-stack gate has no yolo referent — the yolo dialogs are TUI-local and the retry event is rare).

**Step 1 — write the failing tests.** New `internal/tui/retrydlg_test.go`:

```go
package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/store"
)

func openRetryDlg() *recApp {
	a := testApp()
	a.openRetryActionDialog("Request failed", "upstream overloaded (retrying, attempt 1)", "Abort")
	return a
}

func TestRetryDialogRender(t *testing.T) {
	a := openRetryDlg()
	got := stripANSI(a.dlg.retryAction().view(80, 24))
	for _, tok := range []string{
		"Request failed",
		"esc",
		"upstream overloaded (retrying, attempt 1)",
		"don't show again",
		"Abort",
	} {
		if !strings.Contains(got, tok) {
			t.Fatalf("token %q missing:\n%s", tok, got)
		}
	}
}

func TestRetryDialogKeys(t *testing.T) {
	t.Run("starts selected on the action; left/right/tab toggle", func(t *testing.T) {
		a := openRetryDlg()
		if a.dlg.retryAction().selected != 1 {
			t.Fatalf("starts selected = %d, want 1 (the action)", a.dlg.retryAction().selected)
		}
		a.handleKey(press(tea.KeyLeft))
		if a.dlg.retryAction().selected != 0 {
			t.Fatalf("left: selected = %d, want 0", a.dlg.retryAction().selected)
		}
		a.handleKey(press(tea.KeyRight))
		a.handleKey(pressTab())
		if a.dlg.retryAction().selected != 0 {
			t.Fatalf("right then tab: selected = %d, want 0", a.dlg.retryAction().selected)
		}
	})

	t.Run("enter-action aborts and closes", func(t *testing.T) {
		a := openRetryDlg()
		a.handleKey(press(tea.KeyEnter))
		if !a.dlg.empty() || len(a.Cmds) != 1 {
			t.Fatalf("the action must abort + close: empty=%v cmds=%d", a.dlg.empty(), len(a.Cmds))
		}
	})

	t.Run("enter-dismiss closes without aborting", func(t *testing.T) {
		a := openRetryDlg()
		a.handleKey(press(tea.KeyLeft))
		a.handleKey(press(tea.KeyEnter))
		if !a.dlg.empty() || len(a.Cmds) != 0 {
			t.Fatalf("the dismiss must close silently: empty=%v cmds=%d", a.dlg.empty(), len(a.Cmds))
		}
	})

	t.Run("esc dismisses", func(t *testing.T) {
		a := openRetryDlg()
		a.handleKey(press(tea.KeyEscape))
		if !a.dlg.empty() || len(a.Cmds) != 0 {
			t.Fatalf("esc must dismiss: empty=%v cmds=%d", a.dlg.empty(), len(a.Cmds))
		}
	})
}

func TestRetryTransitionHook(t *testing.T) {
	ev := func(prev, next string, attempt int) protocol.Event {
		props, _ := protocol.MakeEvent(protocol.EventTypeSessionStatus, protocol.SessionStatusProps{
			SessionID: "s1",
			Status:    protocol.SessionStatus{Type: next, Attempt: attempt, Message: "upstream overloaded"},
		})
		_ = prev
		return props
	}

	t.Run("idle -> retry on the current session opens the dialog once", func(t *testing.T) {
		a := testApp()
		a.curSessionID = "s1"
		a.store.Status = protocol.SessionStatus{Type: "idle"}
		a.onSessionStatus(ev("idle", "retry", 1))
		top, ok := a.dlg.top()
		if !ok || top.kind != dlgRetryAction {
			t.Fatalf("top = %v, want dlgRetryAction", top.kind)
		}
		// a second idle->retry for the same session is suppressed (per-run)
		a.store.Status = protocol.SessionStatus{Type: "idle"}
		a.onSessionStatus(ev("idle", "retry", 2))
		if n := len(a.dlg.items); n != 1 {
			t.Fatalf("the suppression leaked: depth = %d, want 1", n)
		}
	})

	t.Run("other session / other transitions do not open", func(t *testing.T) {
		a := testApp()
		a.curSessionID = "s1"
		a.store.Status = protocol.SessionStatus{Type: "idle"}
		// a different session
		other := protocol.SessionStatusProps{SessionID: "s2", Status: protocol.SessionStatus{Type: "retry"}}
		evOther, _ := protocol.MakeEvent(protocol.EventTypeSessionStatus, other)
		a.onSessionStatus(evOther)
		if !a.dlg.empty() {
			t.Fatal("a non-current session must not open the dialog")
		}
		// busy -> retry is not the idle->retry transition
		a.store.Status = protocol.SessionStatus{Type: "busy"}
		a.onSessionStatus(ev("busy", "retry", 1))
		if !a.dlg.empty() {
			t.Fatal("busy->retry must not open the dialog")
		}
	})

	t.Run("the suppression clears on the next send", func(t *testing.T) {
		a := testApp()
		a.curSessionID = "s1"
		a.store.Status = protocol.SessionStatus{Type: "idle"}
		a.onSessionStatus(ev("idle", "retry", 1))
		a.handleKey(press(tea.KeyLeft)) // dismiss
		a.handleKey(press(tea.KeyEnter))
		a.applySend(sendMsg{}) // the next send clears the suppression
		a.store.Status = protocol.SessionStatus{Type: "idle"}
		a.onSessionStatus(ev("idle", "retry", 2))
		if a.dlg.empty() {
			t.Fatal("the cleared suppression must allow the dialog again")
		}
	})
}
```

(add the harness key `pressTab()` — `tea.KeyPressMsg{Code: '\t'}` — next to the other harness keys in home_test.go.)

**Step 2 — confirm FAIL.** `go test ./internal/tui/ -run 'TestRetry' -count=1` → FAIL (build fails: undefined `openRetryActionDialog`, `retryDlg`, `dlgRetryAction`, `onSessionStatus`, `pressTab` — the expected red).

**Step 3 — minimal implementation.**
- `retrydlg.go`: the payload + view + keys per the Interfaces. The view: the header (bold title left, muted "esc" right); the message muted, wrapped via `wrapLine` at `w-4`; the pills row: the left "don't show again" + the right actionLabel, space-between; the selected pill paints the primary bg + the `SelectedForeground` fg (the select active-row chain). `handleKey`: left/right/tab → toggle `selected` (0/1); enter → selected 1: `a.closeTopModal()` + `return a.emit(a.abortCmd())`; selected 0: `a.closeTopModal()` (no cmds); esc/ctrl+c → the stack (S2.2).
- `dialog.go`: the kind + payload field + accessor + the `modalInner` case.
- `keys.go`: the `handleDialogKey` case.
- `app.go`: the `EventMsg` case gains the hook after `syncPermDialog`: `a.onSessionStatus(m.Event)`. `onSessionStatus`: unmarshal the `SessionStatusProps`; skip when `p.SessionID != a.curSessionID` or `p.Status.Type != protocol.SessionStatusRetry`; read the previous type from `store.Status.Type` (the store has already applied the new value — so the hook reads the PREVIOUS value: the hook runs BEFORE `store.Apply`? — no: the hook runs AFTER `store.Apply` (app.go:164 order). Implementation note: capture `prev := a.store.Status.Type` BEFORE calling `a.store.Apply` in the `EventMsg` case, then call `a.onSessionStatus(prev, m.Event)` — the signature is `onSessionStatus(prevType string, ev protocol.Event)`; the transition check = `prevType == "idle" && p.Status.Type == "retry"`; skip when `a.retrySuppressed[p.SessionID]`; set `a.retrySuppressed[p.SessionID] = true` (on ANY open — the dismiss and the action both suppress the rest of the run; the upstream dontShow semantics); open the dialog (title "Request failed", the message = `p.Status.Message + " (retrying, attempt " + itoa(p.Status.Attempt) + ")"`, the actionLabel "Abort"). `applySend` success path: `delete(a.retrySuppressed, id)`.
- `home_test.go`: the `pressTab` harness key.

**Step 4 — gate.** `go test ./internal/tui/ -run 'TestRetry' -count=1` → PASS, then FULL gate: `go vet ./... && go test ./...` + `gofmt -l .` empty.

**Step 5 — commit + close the bead.**
`git add internal/tui/retrydlg.go internal/tui/retrydlg_test.go internal/tui/dialog.go internal/tui/keys.go internal/tui/app.go internal/tui/home_test.go && git commit -m "feat: retry-action dialog"`
`bd close <S3.7 bead> --reason "retry-action green: verbatim pills/keys, idle->retry trigger, per-run gate (dev 194), Abort action" --json`

---

### Task S3.8: Theme-list dialog (select over `theme.All()`) (bead `yolo-oae.4.8`, expected id `yolo-oae.4.9`)

**Files:** new `internal/tui/themedlg.go`; modify `internal/tui/dialog.go` (the `dlgThemes` kind, the `themes *themeDlg` payload field, the `dialogStack.themes()` accessor, the `modalInner` case), `internal/tui/keys.go` (the `handleDialogKey` case), `internal/tui/commands.go` (the `runCommand` case `/themes` — `localCommands` already carries it from S3.1); new `internal/tui/themedlg_test.go`.

**Interfaces:** consumes S2.5–S2.7 (`selectModel`, the S3.1 `skipFilter`/`onFilter`), S2.2 (the modal stack, `dlgMedium`), the S0.7 engine (`AllThemes`, `Active`, `Set`), `a.retheme()` (app.go:290). Produces: `dlgThemes` kind; `themeDlg{sel *selectModel, initial string, confirmed bool}`; `App.openThemeListDialog() []tea.Cmd` (engine nil → toast "theme engine unavailable" + no push; else pushes the modal, builds the options, `initial = a.engine.Active()`); `themeOptions(e *theme.Engine) []selectOption` (the case-insensitively sorted `AllThemes()` keys); the onMove/onSelect/onFilter/close callbacks per the design decisions.

**Upstream parity notes:** `dialog-theme-list.tsx` (findings §5): ports verbatim — the title "Themes"; the options = the case-insensitively sorted keys of `theme.all()` (yolo: `Engine.AllThemes()` — builtins + customs + "system"; the frozen table's `theme.All()` name is the detail correction, findings §1); onMove/onSelect → `theme.set` (persists immediately — `Engine.Set` + `retheme`, the live preview); onFilter: empty → set(initial) + re-anchor, else set(first filtered) (the client-side filter through the S3.1 `onFilter` — the upstream is a case-insensitive substring; the yolo port: `strings.Contains(strings.ToLower(name), needle)`); onCleanup (!confirmed → set(initial)) — the stack's `onClose` callback. The select placeholder "Search" (the `skipFilter=true` input, the S3.1 field). The `/themes` opener is the upstream slashName "themes" (the upstream keybind &lt;leader&gt;t lands in S4.1). Parity note: the zero-engine degradation (the toast) — the tests use a real engine (t.TempDir).

**Step 1 — write the failing tests.** New `internal/tui/themedlg_test.go`:

```go
package tui

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// themeApp wires a REAL engine (the S0.7 KV + engine chain) into the app —
// the newRecApp helper hardcodes the nil engine, so this builds the recApp
// directly (the home_theme_test.go pattern).
func themeApp(t *testing.T) (*recApp, *theme.Engine) {
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
	ra := &recApp{App: NewApp(client.New("http://127.0.0.1:9", ""), store.State{}, "", e)}
	ra.emitSink = func(cmds ...tea.Cmd) { ra.Cmds = append(ra.Cmds, cmds...) }
	return ra, e
}

func TestThemeListDialogRender(t *testing.T) {
	a, e := themeApp(t)
	a.openThemeListDialog()
	got := stripANSI(a.dlg.themes().view(80, 24))
	if !strings.Contains(got, "Themes") {
		t.Fatalf("title missing:\n%s", got)
	}
	// the options are the case-insensitively sorted AllThemes keys
	want := make([]string, 0, len(e.AllThemes()))
	for name := range e.AllThemes() {
		want = append(want, name)
	}
	sort.Slice(want, func(i, j int) bool {
		return strings.ToLower(want[i]) < strings.ToLower(want[j])
	})
	last := -1
	for _, name := range want {
		i := strings.Index(got, name)
		if i < 0 {
			t.Fatalf("theme %q missing:\n%s", name, got)
		}
		if i < last {
			t.Fatalf("themes not in sorted order at %q:\n%s", name, got)
		}
		last = i
	}
	// the current theme carries the marker
	if !strings.Contains(got, "●") {
		t.Fatalf("current-theme marker missing:\n%s", got)
	}
}

func TestThemeListDialogFlow(t *testing.T) {
	t.Run("move previews the theme live (Set + retheme), select confirms", func(t *testing.T) {
		a, e := themeApp(t)
		a.openThemeListDialog()
		names := themeOptions(e)
		if len(names) < 2 {
			t.Skip("need at least two themes")
		}
		other := ""
		for _, o := range names {
			if v, ok := o.value.(string); ok && v != e.Active() {
				other = v
				break
			}
		}
		if other == "" {
			t.Skip("no non-active theme")
		}
		// move to the other theme: the live preview fires (Set + retheme)
		for i := 0; i < len(names); i++ {
			a.handleKey(press(tea.KeyDown))
			if e.Active() == other {
				break
			}
		}
		if e.Active() != other {
			t.Fatalf("the live preview must Set the moved theme: active = %s, want %s", e.Active(), other)
		}
		a.handleKey(press(tea.KeyEnter))
		if !a.dlg.empty() {
			t.Fatal("select must close the dialog")
		}
		if e.Active() != other {
			t.Fatalf("select must persist the theme: active = %s", e.Active())
		}
	})

	t.Run("filter-empty restores the initial theme", func(t *testing.T) {
		a, e := themeApp(t)
		initial := e.Active()
		a.openThemeListDialog()
		names := themeOptions(e)
		// move off the initial, then clear the filter -> the initial comes back
		a.handleKey(press(tea.KeyDown))
		if e.Active() == initial {
			a.handleKey(press(tea.KeyDown))
		}
		if e.Active() == initial {
			t.Skip("could not move off the initial theme")
		}
		// select to confirm a different theme, then re-open + clear the filter
		a.handleKey(press(tea.KeyEnter))
		moved := e.Active()
		a.openThemeListDialog()
		_ = names
		// type then delete the filter text: the onFilter("") restores the
		// dialog's initial (the active-at-open = moved)
		a.handleKey(press('a'))
		a.handleKey(press(tea.KeyBackspace))
		if e.Active() != moved {
			t.Fatalf("filter-clear must restore the initial: active = %s, want %s", e.Active(), moved)
		}
	})

	t.Run("esc without a select restores the initial theme", func(t *testing.T) {
		a, e := themeApp(t)
		initial := e.Active()
		a.openThemeListDialog()
		a.handleKey(press(tea.KeyDown)) // preview a different theme
		if e.Active() == initial {
			a.handleKey(press(tea.KeyDown))
		}
		if e.Active() == initial {
			t.Skip("could not move off the initial theme")
		}
		a.handleKey(press(tea.KeyEscape))
		if e.Active() != initial {
			t.Fatalf("esc must restore the initial: active = %s, want %s", e.Active(), initial)
		}
	})

	t.Run("zero engine toasts and does not open", func(t *testing.T) {
		a := testApp()
		a.openThemeListDialog()
		if !a.dlg.empty() {
			t.Fatal("the zero engine must not open the dialog")
		}
		if len(a.toasts) != 1 || !strings.Contains(a.toasts[0].msg, "theme engine unavailable") {
			t.Fatalf("the engine-unavailable toast missing: %v", a.toasts)
		}
	})
}
```

**Step 2 — confirm FAIL.** `go test ./internal/tui/ -run 'TestThemeList' -count=1` → FAIL (build fails: undefined `openThemeListDialog`, `themeOptions`, `themeDlg` — the expected red).

**Step 3 — minimal implementation.**
- `themedlg.go`: the payload + options + callbacks per the Interfaces. `openThemeListDialog`: `a.engine == nil` → `a.toast("theme engine unavailable")`, return nil; else `initial := a.engine.Active()`; `opts := themeOptions(a.engine)` (the `AllThemes()` keys, `sort.Slice` on `strings.ToLower`; title = the name, value = the name); `sel := selectNew("Themes", "Search", opts, isCurrent(active), onSelect, onMove)`; `sel.skipFilter = true`; `sel.onFilter = func(needle string) {...}` per the design ("" → `a.engine.Set(initial)` + `a.retheme()` + re-anchor the selection at the initial; else → filter the options by the case-insensitive substring, `sel.options = filtered`, `sel.sel = 0`, `a.engine.Set(filtered[0].value.(string))` + `a.retheme()` — guard the empty filtered list: no Set); `d := &themeDlg{sel: sel, initial: initial}`; `a.pushModal(dialog{kind: dlgThemes, themes: d}, dlgMedium, onClose)` where onClose: `if !d.confirmed { a.engine.Set(d.initial); a.retheme() }`. onMove: `a.engine.Set(o.value.(string))` + `a.retheme()`. onSelect: `a.engine.Set(...)` + `a.retheme()` + `d.confirmed = true` + `a.closeTopModal()`.
- `dialog.go`: the kind + payload field + accessor + the `modalInner` case.
- `keys.go`: the `handleDialogKey` case (the select consumes the keys).
- `commands.go`: the `runCommand` case `/themes` → `a.openThemeListDialog()`.

**Step 4 — gate.** `go test ./internal/tui/ -run 'TestThemeList' -count=1` → PASS, then FULL gate: `go vet ./... && go test ./...` + `gofmt -l .` empty.

**Step 5 — commit + close the bead.**
`git add internal/tui/themedlg.go internal/tui/themedlg_test.go internal/tui/dialog.go internal/tui/keys.go internal/tui/commands.go && git commit -m "feat: theme-list dialog (select over themes)"`
`bd close <S3.8 bead> --reason "theme list green: sorted AllThemes, live preview, filter restore, esc restore, /themes" --json`

---

### Task S3.9: Theme-list: KV wiring + mode switch/lock keybinds (bead `yolo-oae.4.9`, expected id `yolo-oae.4.10`)

**Files:** new `internal/tui/themecmds.go`; modify `internal/tui/themedlg.go` (the KV-persistence assertions hook — no code change expected, the tests bind the chain); new `internal/tui/themecmds_test.go`.

**Interfaces:** consumes the S0.7 engine (`Set`/`Pin`/`Free`/`Apply`/`Mode`/`Locked`, the KV file), `a.retheme()` (app.go:290), `theme.OpenKV` (the read-back assertion). Produces: `App.themeSwitchMode() []tea.Cmd` (`Pin` the opposite of `Mode()` — the `setMode` === pin quirk, verbatim); `App.themeModeLock() []tea.Cmd` (`Locked() ? Free() : Pin(Mode())`); `switchModeTitle(e *theme.Engine) string` ("Switch to light mode" / "Switch to dark mode"); `modeLockTitle(e *theme.Engine) string` ("Unlock theme mode" / "Lock theme mode"); the engine-nil toast guard ("theme engine unavailable"). **No default keybinds** (upstream both "none", keybind.ts:79-80 — deviation 196, low; the S4.1 registry ports the "none" defaults + the remap; the S4.2 per-context groups own the keys).

**Upstream parity notes:** `context/theme.tsx` + `config/keybind.ts` (findings §2–3): the `setMode` === `pin` quirk ports verbatim (the "switch mode" command pins the opposite mode); `theme.mode.lock` = `locked() ? free() : lock()` (lock = `pin(store.mode)`) ports verbatim; the dynamic titles port verbatim (they exist as the `switchModeTitle`/`modeLockTitle` helpers for the S4 registry + the unit tests — yolo has no dynamic command titles pre-S4). "KV wiring" = the end-to-end chain: every `Set`/`Pin`/`Free`/`Apply` persists to the KV file IMMEDIATELY (the S0.7 writer goroutine) and the app retheme picks up the active theme after a `Set` (the S3.8 live preview binds this through the dialog; this task binds it directly + the mode keys). **Deviation 196 (low):** the upstream default keys are "none" for both mode commands — the functions land without keys; the S4.1 registry carries the "none" defaults.

**Step 1 — write the failing tests.** New `internal/tui/themecmds_test.go`:

```go
package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kido5217/yolo/internal/tui/theme"
)

func kvPathOf(e *theme.Engine) string { return e.KVPath() }

func TestThemeSwitchMode(t *testing.T) {
	a, e := themeApp(t)

	t.Run("switch pins the opposite mode (the setMode === pin quirk)", func(t *testing.T) {
		prev := e.Mode()
		next := "light"
		if prev == "light" {
			next = "dark"
		}
		a.themeSwitchMode()
		if e.Mode() != next || !e.Locked() {
			t.Fatalf("switch = mode %q locked %v, want %s + locked (the pin quirk)", e.Mode(), e.Locked(), next)
		}
		if got, want := switchModeTitle(e), "Switch to "+prev+" mode"; got != want {
			t.Fatalf("title = %q, want %q (the next mode)", got, want)
		}
	})

	t.Run("again -> pins back, still locked", func(t *testing.T) {
		prev := e.Mode()
		next := "light"
		if prev == "light" {
			next = "dark"
		}
		a.themeSwitchMode()
		if e.Mode() != next || !e.Locked() {
			t.Fatalf("second switch = mode %q locked %v, want %s + locked", e.Mode(), e.Locked(), next)
		}
	})
}

func TestThemeModeLock(t *testing.T) {
	a, e := themeApp(t)

	t.Run("unlocked: lock pins the current mode, then unlocks", func(t *testing.T) {
		if e.Locked() {
			t.Fatal("the fresh engine must be unlocked")
		}
		if got := modeLockTitle(e); got != "Lock theme mode" {
			t.Fatalf("title = %q, want %q", got, "Lock theme mode")
		}
		a.themeModeLock()
		if !e.Locked() || e.Mode() != "dark" {
			t.Fatalf("lock = locked %v mode %q, want locked+dark", e.Locked(), e.Mode())
		}
		if got := modeLockTitle(e); got != "Unlock theme mode" {
			t.Fatalf("title = %q, want %q", got, "Unlock theme mode")
		}
		a.themeModeLock()
		if e.Locked() {
			t.Fatal("the second press must unlock")
		}
	})
}

func TestThemeKVWiring(t *testing.T) {
	a, e := themeApp(t)

	t.Run("Set persists to the KV file and retheme follows it", func(t *testing.T) {
		names := themeOptions(e)
		target := ""
		for _, o := range names {
			if v, ok := o.value.(string); ok && v != e.Active() {
				target = v
				break
			}
		}
		if target == "" {
			t.Skip("no non-active theme")
		}
		a.openThemeListDialog()
		// move to the target (the S3.8 live preview: Set + retheme)
		for i := 0; i < len(names) && e.Active() != target; i++ {
			a.handleKey(press(tea.KeyDown))
		}
		if e.Active() != target {
			t.Fatalf("active = %s, want %s", e.Active(), target)
		}
		// the KV file carries the persisted theme (the S0.7 writer has
		// flushed — the synchronous in-memory store + the writer goroutine;
		// the engine's Close flushes, but the KV read goes through the
		// in-memory store, so the file may lag: assert through the engine
		// re-read instead).
		// a fresh engine on the same KV file sees the persisted theme:
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
		defer func() { _ = e2.Close() }()
		if err := e2.Resolve(context.Background()); err != nil {
			t.Fatalf("theme.Resolve (second): %v", err)
		}
		if got := e2.Active(); got != target {
			t.Fatalf("the fresh engine sees active = %q, want %q (the KV persistence)", got, target)
		}
	})

	t.Run("Pin/Free persist the mode keys to the KV file", func(t *testing.T) {
		a.themeSwitchMode() // pins light
		if err := e.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
		// the raw KV file carries theme_mode + theme_mode_lock (the writer
		// flushed on Close)
		data, err := os.ReadFile(kvPathOf(e))
		if err != nil {
			t.Fatalf("read KV: %v", err)
		}
		for _, tok := range []string{"theme_mode", "theme_mode_lock"} {
			if !strings.Contains(string(data), tok) {
				t.Fatalf("the KV file missing %q:\n%s", tok, data)
			}
		}
	})
}
```

NOTE for step 3: `theme.Engine` must expose the KV path for the test — add `Engine.KVPath() string` (a one-line accessor returning `opts.KVPath`; if the engine already exposes it under a different name, reuse it — verify at execution, the engine struct holds `opts` — the accessor is the minimal seam).

**Step 2 — confirm FAIL.** `go test ./internal/tui/ -run 'TestThemeSwitchMode|TestThemeModeLock|TestThemeKVWiring' -count=1` → FAIL (build fails: undefined `themeSwitchMode`, `themeModeLock`, `switchModeTitle`, `modeLockTitle`, `KVPath` — the expected red).

**Step 3 — minimal implementation.**
- `themecmds.go`:
  - `themeSwitchMode()`: `a.engine == nil` → toast + nil; `mode := a.engine.Mode(); next := "light"; if mode == "light" { next = "dark" }; a.engine.Pin(next)` (the `setMode` === pin quirk — findings §2) + `a.retheme()` (Pin already applied the mode — retheme refreshes the styles; `Pin` persists `theme_mode_lock` + `theme_mode`).
  - `themeModeLock()`: `a.engine == nil` → toast + nil; `if a.engine.Locked() { a.engine.Free() } else { a.engine.Pin(a.engine.Mode()) }` + `a.retheme()` (Free clears the lock + both KV keys; Pin(Mode()) = the upstream `lock()`).
  - `switchModeTitle(e)`: `if e.Mode() == "dark" { return "Switch to light mode" }; return "Switch to dark mode"` (the dynamic title shows the NEXT mode — the upstream: the command registered with `mode() === "dark" ? "Switch to light mode" : "Switch to dark mode"`).
  - `modeLockTitle(e)`: `if e.Locked() { return "Unlock theme mode" }; return "Lock theme mode"`.
- `theme/engine.go`: the `KVPath() string` accessor (returns `e.opts.KVPath`).
- No keybinds (upstream "none" — deviation 196; the S4.1 registry).

**Step 4 — gate.** `go test ./internal/tui/ -run 'TestThemeSwitchMode|TestThemeModeLock|TestThemeKVWiring' -count=1` → PASS, then FULL gate: `go vet ./... && go test ./...` + `gofmt -l .` empty.

**Step 5 — commit + close the bead, then run the S3 slice gate.**
`git add internal/tui/themecmds.go internal/tui/themecmds_test.go internal/tui/theme/engine.go && git commit -m "feat: theme commands - KV wiring + mode switch/lock"`
`bd close <S3.9 bead> --reason "theme commands green: pin quirk verbatim, lock/unlock, KV persistence, no keys (dev 196)" --json`
Then the **S3 slice gate** (the stub's gate section): module gate green (`go vet ./... && go test ./...` + `gofmt -l .` empty, incl. `TestImportsDirection` + the S3 teatest goldens); the user-run TTY smoke (NOT CI): open the session list + rename a session + status + help + the retry-action (a scripted fake turn with a pre-stream failure) + the theme list (select a theme; switch/lock the mode); the deviation entries 188–196 are all in `DEVIATIONS.md` (188 info bead-id shift, 189 low session-list deferrals, 190 low no server search + status snapshot, 191 low delete-failed adaptation, 192 low provider no-oauth, 193 low status content, 194 low retry-action in-memory gate, 195 info paletteShortcut accessor, 196 low no default theme keys); the `PROGRESS.md` one-line status pointer; commit `docs: checkpoint — S3 done, next is S4 detail pass`; `bd close yolo-oae.4 --reason "all 9 child beads closed, gate green" --json`.

## S3 slice gate (slice bead `yolo-oae.4`)

NOT a task bead; runs after all child beads close. Mirror the S0 slice gate
shape: (1) module gate `go vet ./... && go test ./...` + `gofmt -l .` empty
(incl. `TestImportsDirection` + the S3 teatest goldens); (2) user-run smoke
(NOT CI): in a real TTY, open session-list, rename a session, status, help,
retry-action, and theme-list (select a theme; switch/lock the mode with the
new keybinds); (3) append any forced DEVIATIONS.md entries this slice named
(with severity, same-commit rule — root principle 2); (4) PROGRESS.md
one-line status pointer; (5) commit
`docs: checkpoint — S3 done, next is S4 detail pass`; (6)
`bd close yolo-oae.4 --reason "all 9 child beads closed, gate green" --json`.
