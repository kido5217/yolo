# Yolo — Verified Facts (session memory)

Task status lives in beads (the release epic; `bd ready`) and in `git log
--oneline`. This file holds proven facts a resumed session must not
re-litigate. The append-only deviation audit log lives in `DEVIATIONS.md`
(items 1–66 frozen in `deviations-archive-v0.1.0.md`).

**Status (2026-08-27):** TUI parity S1 (transcript) landed on `new_tui`
(epic `yolo-oae`): glamour v2.0.1 transcript rendering — StyleConfig +
per-theme TranscriptRenderer/ReasoningRenderer (S1.2, dev 149), renderer
wired into text parts with SGR goldens (S1.3, devs 150–151),
Chroma/SubtleChroma syntax styling (S1.4, devs 152–153), GFM tables/task
lists/strikethrough (S1.5, dev 154), reasoning restyle (S1.6, devs
155–158), tool-row restyle (S1.7, devs 159–161), error-box + toast
restyle (S1.8, dev 162), 100 KB re-render benchmark + budget gate (S1.9,
dev 163 — brief fixture was 210,681 B not 100 KB, fixed to 104,981 B
spec shape; gate re-baselined 50→150 ms, measured min-of-5 ≈ 100 ms on
glamour v2.0.1). The S1 slice-gate transcript-fixture pty diff is
deferred to the S8 diff sweep (deviation 164; the pty-capture tooling is
S8's per spec §8 — the S0 precedent at deviation 125); the user-run TTY
smoke is on-demand, pending. S2 (dialogs) in progress: detail pass
landed (`s2-dialogs.md` fully detailed, 10 tasks, commit 74d4f17; the
task beads sit at `yolo-oae.3.4`–`3.13` — deviation 165), S2.1 deps huh
v2.0.3 + sahilm/fuzzy v0.1.3 landed (69→79 modules, MVS kept all yolo
pins, scratch smoke green), S2.2 modal dialog stack landed (dlgSize
60/88/116, push/replace/close/clear ops, esc+ctrl+c with the model/agent
subchoice veto, clamped-chrome overlay frame — blank backdrop per
deviation 166; deviations 166–169 logged), S2.3 huh field dialogs landed
(`huhdlg.go`: huhFormDlg form payload + openFormModal, themed via
`themeDialog` from the resolved theme tokens; buildAlertForm = lone ok
pill, buildConfirmForm = confirm/cancel pills starting on confirm;
submit/esc-cascade driven through App — huh's unexported form-progress
msgs are forwarded to the open form modal in `App.updateMsg`'s default
case; deviations 170–171 logged), S2.4 huh field input landed
(`buildInputForm` themed input — rename/prompt; deviation 172 logged),
S2.5 select core landed (`select.go`: selectOption/selectModel, fuzzy
weighted filter with the focus fix, wrap nav, home/end, enter; deviations
173–174 logged), S2.6 select categories/details landed (`locale.go`:
truncateMiddle + titlecase ports; `buildLines`/`selLine` row model —
flat-on-filter category headers, truncateMiddle'd detail rows, footer
tail via `rowWithFooter`; the scroll window now counts rendered rows;
deviation 175 logged), S2.7 select actions/hints/acceleration landed
(`select.go`: footer actions (own-key run, tab/shift+tab focus cycle
with wrap, enter-on-focus) + right-footer hints, pgup/pgdn ±10 row
scroll — the upstream env-machined `getScrollAcceleration` not ported;
the window re-anchor is now selection-change-driven via `lastSel`;
deviations 176–178 logged), S2.8 permission restyle landed
(`permission.go`: `permDlg` modal payload (sel pill) + `permInfo` info()
port over the part input (no request Meta on the wire) + the reply pills
(selected pinned to the warning token, unselected muted) + `handleKey`
(1/2/3 + esc reply, left/right pill nav with wrap, enter = selected) +
`partInput` + `view`/`permissionView` (the non-modal overlay path; wraps
at w); `dialog.go` `syncPermDialog` push/pop against parked asks
(idempotent) driven from `applyPermReply` (success only) + the
`permission.*` events in `EventMsg`; deviations 179–182 logged),
S2.9 model restyle landed (`dialog.go`: the two-pane picker replaced by
the flat `modelDlg{sel *selectModel, hasSubChoice, pick}` on the dlgLarge
modal — flat catalog select (title = model name, category = provider
name, description = ctx/cost tail, footer = provider status), ● gutter
on the session/config model, the yolo-pinned [a]/[b] subchoice via
`modelSelectPick`, `modelOptions`/`modelIsCurrentOpt`/`modelSelIndex` +
`providerStatusText`; the two-pane machinery deleted (panes, tab key,
move/modelCount/selectedRef/currentProv/modelCell/modelRow/styleSegment/
modelIsCurrent); `select.go` `rowWithFooter` tail-wrap via `wrapTailRow`
for over-wide footer rows — forced by the re-baselined overflow test,
the fit path byte-identical; deviations 183–184 logged), S2.10 agent
restyle landed (`dialog.go`: the plain list replaced by the flat
`agentDlg{sel *selectModel, hasSubChoice, pick}` on the dlgMedium modal —
select rows (title = name, description, value = name) with the ● gutter on
the session/config agent, the yolo-pinned [a]/[b] subchoice via
`agentSelectPick`, `agentOptions`/`agentIsCurrentOpt`; the list machinery
deleted (the int sel, selectedName, the wraparound key handler, the
row renderer); `select.go`: the `rowLine` description tail-wrap via
`wrapTailRow` + the keymap hint's `dimWrapped` wrap (yolo-ukc, forced by
the re-baselined overflow test — fit paths byte-identical, no select pin
re-baselined; the overflow test's post-open store swap is a no-op against
the select's frozen options — store set before open instead); deviations
185–186 logged).
Deviations 122–196 logged.
S2 done (10/10 child beads closed, slice gate green — the user-run TTY
smoke is on-demand, pending). The parked S2.8 wrap bug (yolo-kj6) is fixed:
`wrapLine` is now ANSI-aware (SGR = zero-width glue, never split inside an
escape) and the SGR golden's pill pin re-baselined to the un-wrapped layout
(dev 187).
S3 detail pass landed (`s3-dialogs-2.md` fully detailed, commit 8fb8bcc,
bead `yolo-oae.4.1` closed): 9 task sections S3.1–S3.9 with full 5-step TDD
+ the detail-pass findings (upstream @ v1.18.18 read at detail time; the
real `Engine` surface — `AllThemes`, `Set` persists the KV immediately,
`setMode`===pin quirk) + binding design decisions; task beads land at
`yolo-oae.4.2`–`.10` (id shift, dev 188) and the task-forced deviations are
pre-logged at detail time (devs 189–196: S3.1 pin/slots deferred + no
server-side search, S3.3 no workspace recovery, S3.4 no oauth
auth-method, S3.5 status = providers+agents only, S3.7 in-memory retry
gate + "Abort" pill, S3.6 `paletteShortcut()` accessor, S3.9 no default
keys for mode switch/lock — upstream "none").
S3.1 session-list landed (bead `yolo-oae.4.2`, commit 28517c2): the
additive select fields (`selectOption.bg`/`.gutter`,
`selectModel.skipFilter`/`.onFilter` — zero values byte-identical, no S2
golden re-baselined), the `sessionsDlg` modal on dlgMedium (options =
`store.Sessions` updated-desc, "Today"/`Mon Jan 2 2006` category via
`sessionCategory`, the current-session ● gutter + the busy/retry spinner
gutter from the one-shot `client.Status()` snapshot at open, skipFilter +
the client-side title substring filter per dev 190, two-step ctrl+d
delete — armed row "Press ctrl+d again to confirm" + the error-bg row,
`syncSessionSel` preserveSelection on session.updated/deleted, enter =
openSession + close + hydrate), the `/sessions` opener via
`localCommands()` merged client-side (`mergedCommands()` shared by the
slash display AND the `handleMenuKey` execution path — the server
catalog stays frozen at 5), and the teatest SGR golden with the armed
row's bg `48;5;246` (the step-1 scratch Convert256 output won over the
finding's 174 — deviation 143 corroborates the error token 246).
Deviations 189–190 (pre-logged) + 197 (execution adaptations: the
teatest-leg store seed, `CreateSession(ctx, title)`, the 2-arg
`view(w,h)`, the merged-list execution path; `TestPromptMenuKeys` wrap
count 5→6). The 9 S3 task beads were created on resume at the dev-188
IDs `yolo-oae.4.2`–`.10` (the detail pass left them uncreated).
Next: S3.2 session-rename (bead `yolo-oae.4.3`).
S3.2 session-rename landed (bead `yolo-oae.4.3`, commit 96f839b):
`rename.go` (`openSessionRenameDialog` — title seeded from `store.Sessions`
fallback `Current`, `buildInputForm(... "Rename Session", "", "Title", title)`
on the `dlgMedium` form modal, the empty-value no-op guard, `renameCmd` →
`PatchSession`, `applyRename`: err → toast, success → in-place title on
`store.Sessions` + `Current.Title`, NO success toast — upstream parity);
the session-route `ctrl+r` binding (`sessKeyMap.Rename`, matched before the
esc branch, gated on `curSessionID != ""`, the init cmds discarded so the
key is consumed with zero returned cmds — deviation 198a) and the
session-list `rename` action (ctrl+r, closes the list, opens the rename
dialog for the selected row — the S3.1-stubbed action). Deviation 198
(test-accuracy/plan-scope/low: 198a the discarded init cmds; 198b the
`driveCmds` harness continues when the emit sink appends mid-round — the
onConfirm patch cmd lands inside the submit cascade; 198c the plan's Files
list omits app.go's renameMsg case + the harness file). No pin re-baselined,
no select golden touched.
Next: S3.3 session-delete-failed (bead `yolo-oae.4.4`).
S3.3 session-delete-failed landed (bead `yolo-oae.4.4`, commit afd81b9):
`deletefailed.go` (the `deleteFailedDlg` payload — id/title/errMsg/active —
`openDeleteFailedDialog` on `dlgMedium`: the session list REPLACED when it is
on top, else pushed, active starts on retry; the 3-arg `view(w, h, th)`:
header row (bold "Failed to Delete Session" left / muted "esc" right,
space-between), the muted body (the session title + the wire error wrapped
at w-4, blank, "Choose how to proceed.") and the two option rows —
"Retry delete"/"Keep session" — the active row as the full-row primary-bg
paint with the `SelectedForeground(bg)` fg (the select active-row chain),
`handleKey`: left/up → retry, right/down → keep (two-row clamp, no wrap),
enter on keep → `closeTopModal` + re-hydrate, enter on retry → re-emit the
delete and STAY open); dialog.go wiring (the `dlgDeleteFailed` kind, the
payload field, the `dialogStack.deleteFailed()` accessor, the `modalInner`
+ `handleDialogKey` cases); `applySessionDelete` rewired — first failure
opens the dialog (title from `sessionTitle`: `store.Sessions` then
`Current`), a failed retry refreshes `errMsg` in place, a successful retry
closes it (and routes home + hydrates when the current session died).
Deviations 191 (pre-logged: the option-text adaptation) + 199
(test-accuracy/low: 199a the pinned test's 2-arg `view(80, 24)` call sites
→ 3-arg `view(80, 24, a.theme)` per the last-stated Step 3 call; 199b the
`press` harness (home_test.go) extended to `tea.KeyLeft`/`tea.KeyRight` —
it set `Text: string(r)` for unlisted runes, so `String()` never returned
"left"/"right" and the string-matched `key.Matches` could not fire; the
pinned test body is unchanged). 4 pinned tests green (render, keys,
failure leg, teatest SGR).
Next: S3.4 provider-dialog restyle (bead `yolo-oae.4.5`).
S3.4 provider-dialog restyle landed (bead `yolo-oae.4.5`, commit 47cfaab):
`providerdlg.go` (the `providerDlg` payload — the select + the th at open —
`openProviderDialog` on `dlgMedium` mirroring `openModelDialog`: push modal
+ `syncProviderSel` + `fetchCatalogCmd`; `providerOptions`: the ported
`PROVIDER_PRIORITY` (unknown → 99) + the (priority, name lc, ID) sort + the
"Popular"|"Providers" categories + the known-id description map + the
`providerStatusText` footer + the trailing "Other" custom option
(`__yolo_custom_provider__`); `normalizeCustomProviderID` (the ported regex
`^[a-z0-9][a-z0-9-_]*$` + the `@ai-sdk/` strip, "" when invalid); the flow:
select pick → the API-key form (known) or the "Other" id prompt (custom) —
deviation 192: no oauth, the key form opens directly; the id prompt:
invalid → the verbatim error toast + the prompt re-opens (a push — the
cascade already popped the submitted form); valid → the key form with the
yolo.jsonc saved-credential description; the key form: empty value →
re-open (the upstream `if (!value) return` guard), else `authCmd` →
`applyAuth`: err → toast (the dialog stays), success + catalog id →
`closeTopModal` + `openModelDialog` (the upstream `dialog.replace(DialogModel)`),
success + custom → the saved-credential info toast + close); dialog.go
wiring (the `dlgProvider` kind, the payload field, the
`dialogStack.provider()` accessor, the `modalInner` + `handleDialogKey`
cases, `applyCatalog` now also syncs `syncProviderSel`); commands.go: the
`/connect` local entry + the `runCommand` case; app.go: the `authMsg` case.
Deviations 192 (pre-logged: the oauth-method omission) + 200
(test-accuracy/plan-scope/low: 200a the pinned test's unused "context"
import dropped; 200b the plan's "localCommands already carries /connect from
S3.1" note was stale — the entry + `runCommand` case land here, the
`TestPromptMenuKeys` wrap count re-baselines 6→7; 200c the 2-arg `view(w,
h)` + the th-at-open capture (the 197c convention); 200d the pinned order
assertion's swapped priorities (the "anthropic (1) + openai (2)" comment)
re-baselined to the plan's own pinned map (openai 2 < anthropic 4); 200e
the render call `view(80, 24)` → `view(80, 26)` — the 7 built lines (2
category headers + the between-group blank) overflow the `h/2-6` = 6-line
select window, so the "Other" tail was unreachable; 200f the Fatalf's
invalid `a.dlg.top().kind` → a `top, _ := a.dlg.top()` binding; plus the
`applyCatalog`/`app.go` Files omissions (the 198c class), the re-open
`replaceModal` → `openFormModal` push resolution, and the `driveCmds`
replay switched to `a.updateMsg` — `Update` drains the toast TTL tick and
would expire a mid-cascade toast before the pinned assertion). 4 pinned test
functions green (normalize, render, flow, teatest /connect).
Next: S3.5 status dialog (bead `yolo-oae.4.6`).
S3.5 status dialog landed (bead `yolo-oae.4.6`, commit 4dc3cbe):
 `statusdlg.go` (the `openStatusDialog` opener on `dlgMedium` — push modal,
 no payload; `statusView(w, h int, th theme.Theme)`: the bold "Status" +
 muted "esc" header row (space-between at the panel width), then the
 Providers section (the count header `N Providers` text token, a bullet row
 per provider — the status-colored bullet via `providerStatus`
 (loaded→success, missing→error, else→textMuted) + the bold name — and the
 "No Providers" fallback) and the Agents section (the count header
 `N Agents`, a bullet row per agent — the success bullet + the bold name +
 the muted description wrapped at w-4, continuation lines indented under the
 name — and the "No Agents" fallback); no session section — the footer owns
 the session status); dialog.go wiring (the `dlgStatus` kind, the
 `modalInner` case → `a.statusView(w, h, a.theme)`, the `handleDialogKey`
 case — every key ignored, esc/ctrl+c close via the stack); commands.go:
 the `/status` local entry + the `runCommand` case. Deviations 193
 (pre-logged: content = providers + agents only, no session section) + 201
 (test-accuracy/plan-scope/low: 201a the pinned test's 2-arg
 `statusView(80, 24)` vs the interface/Step-3 3-arg — the last-stated call
 (the `modalInner` case) wins, the test's two call sites fixed to
 `statusView(80, 24, a.theme)`; 201b the plan's "localCommands already
 carries /status from S3.1" note was stale — the entry + `runCommand` case
 land here, the `TestPromptMenuKeys` wrap count re-baselines 7→8; 201c the
 plan's Files note names keys.go but the `handleDialogKey` case lives in
 dialog.go — keys.go unchanged, not in the commit). 3 pinned test functions
 green (render + fallback, esc/ctrl+c close).
S3.6 help-dialog restyle landed (bead `yolo-oae.4.7`, commit 4a7ae96):
 `dialog.go` (the `helpDialog` markdown table dropped — its `| key | action |`
 rows are superseded by the S4.7 registry rendering; `helpDialogView(w, h,
 th)`: the bold "Help" + muted "esc/enter" header row (space-between at the
 panel width), the muted body — the palette line `Press ` +
 `a.paletteShortcut()` + ` to see all available actions and commands in any
 context.` + a blank line + the locked V1 note `pgup/pgdn scroll ·
 \+enter newline` — and the right-aligned "ok" pill (pad 0 3, the primary
 bg + the `SelectedForeground` fg — the opencode token has no
 selectedListItemText → the fallback, 48;5;216 bg + 38;5;232 fg under the
 pinned test env); `paletteShortcut() string` = the pre-S4 yolo constant
 "ctrl+p" (S4.7 rewires it to the keymap registry)); the `dlgHelp` flip to
 modal: the `modalInner` case → `a.helpDialogView(w, h, a.theme)`, the
 `handleDialogKey` case — enter → `closeTopModal`, every other key ignored
 (esc/ctrl+c via the stack; the pre-S3 "any key closes" dropped), the
 `dlgHelp` case removed from `dialogStack.view` (the non-modal path);
 `commands.go`: the `/help` `runCommand` case → `pushModal(dlgHelp,
 dlgMedium)` (was the non-modal `dlg.push`); `help_test.go` re-written
 (the table-gone + plain-key ignored + enter/esc close legs) + the teatest
 SGR golden `TestTUIHelpDialog` (the "ok" pill's 48;5;216 bg + 38;5;232 fg,
 the merged token condition); the `TestTUIDialogs` help leg re-baselined
 (the markdown-table token → the palette line). Deviations 195 (pre-logged:
 the paletteShortcut accessor) + 202 (test-accuracy/plan-scope/low: 202a the
 pinned test line `a.helpDialogView(a, 80, 24)` + the Step-3 `modalInner`
 `a.helpDialogView(a, w, h, a.theme)` both carry a redundant leading `a` +
 omit `th` — the 3-arg `App.helpDialogView(w, h, th)` method is the codebase
 convention, the test re-baselines to `a.helpDialogView(80, 24, a.theme)`;
 202b the plan's Files note names view.go + keys.go but both cases live in
 dialog.go — view.go + keys.go unchanged, not in the commit). 3 pinned test
 functions green (view + keys + teatest SGR).
S3.7 retry-action dialog landed (bead `yolo-oae.4.8`, commit 2e600e7):
 `retrydlg.go` (`retryDlg{title, message, actionLabel, selected, th}` —
 0 = dismiss / "don't show again", 1 = the action; `view(w, h)` 2-arg with
 the theme captured at open (the 197(c) convention): the bold title + muted
 "esc" header (space-between), the muted message wrapped via `wrapLine` at
 `w-4`, the pills row "don't show again" left / the actionLabel right
 (space-between; the active pill = the primary bg + the
 `SelectedForeground` fg — the select active-row chain; the link line is
 unused — no BgPulse dep); `handleKey`: left/right/tab toggle, enter on the
 action → `closeTopModal` + `emit(abortCmd())` (the turn lands on the
 existing aborted flow), enter on the dismiss → close silently,
 esc/ctrl+c via the stack); `dialog.go`: the `dlgRetryAction` kind + the
 `retry` payload field + the `retryAction()` accessor (any open retry
 dialog, the model/precedent invariant) + the `modalInner` +
 `handleDialogKey` cases; `app.go`: `retrySuppressed map[string]bool`
 (initialized in `NewApp`; the S3.7 per-run gate — deviation 194): the
 `EventMsg` case captures `prev := a.store.Status.Type` BEFORE
 `store.Apply` and calls `onSessionStatus(prev, m.Event)` — the hook skips
 on a non-`session.status` event, a non-current session, a non-retry type,
 a non-idle prev (the idle→retry transition only), or an already-suppressed
 session; on the transition it sets the suppression (ANY open — dismiss or
 action — suppresses the rest of the run) and opens the dialog (title
 "Request failed", message = the wire `Message` + ` (retrying, attempt
 <n>)`, actionLabel "Abort"); the dialog opens even with other dialogs on
 the stack (the upstream empty-stack gate has no yolo referent);
 `commands.go`: the `applySend` SUCCESS path (after the `m.err != nil`
 guard) clears the suppression via `delete(a.retrySuppressed,
 a.curSessionID)` — `sendMsg` carries only the error, so the
 current-session key is the contract. Deviation 203
 (test-accuracy/plan-scope/low: 203a the pinned test called
 `onSessionStatus` one-arg but the Step-3 last-stated call pins the 2-arg
 `(prevType string, ev protocol.Event)` signature — the 2-arg wins, the
 test call sites pass prev explicitly; 203b the pinned 3-arg `view`
 interface loses to the pinned 2-arg test call; 203c the pinned test's
 unused `store` import dropped; Files gaps: keys.go unchanged — the
 `handleDialogKey` case lives in dialog.go; `commands.go` (the
 `applySend` home) in the commit). 4 pinned test functions green (render +
 keys + the 3-leg transition hook).
S3.8 theme-list dialog landed (bead `yolo-oae.4.9`, commit 99e3650):
 `themedlg.go` (`themeDlg{sel, th, initial, confirmed}` payload — `th`
 is a LIVE theme accessor (`func() theme.Theme` over `a.theme`): the
 pinned 2-arg `view(w, h)` takes no theme arg (the 197(c) class) and the
 upstream dialog renders inside the theme context, so a preview move
 re-themes the dialog's own rows; the at-open capture would keep the
 list in the stale palette while the preview switches); `view(w, h)`
 2-arg, `handleKey` forwards to the select (esc/ctrl+c via the stack);
 `themeOptions(e)` — the `AllThemes()` keys (builtins + customs +
 "system") sorted case-insensitively (the upstream localeCompare port);
 `openThemeListDialog` — `engine == nil` → toast "theme engine
 unavailable" + no push; else `initial = engine.Active()`,
 `selectNew("Themes", "Search", opts, isCurrent=engine.Active(),
 onSelect, onMove)` with `skipFilter=true`: onMove → `Set(name)` +
 `retheme` (the live preview — `Set` persists immediately, the upstream
 `theme.set` behavior); onSelect → Set + retheme + `confirmed=true` +
 `closeTopModal`; onFilter "" → restore `sel.options` + `Set(initial)`
 + retheme + re-anchor the selection at the initial; onFilter non-empty
 → the case-insensitive substring filter over the FULL list, `sel.sel=0`
 + `Set(first match)` + retheme (empty match → no Set); the stack
 onClose: `!confirmed` → `Set(initial)` + retheme (the upstream
 onCleanup); `dialog.go`: the `dlgThemes` kind + the `themes` payload
 field + the `themes()` accessor + the `modalInner` + `handleDialogKey`
 cases; `commands.go`: `localCommands` gains the `{"/themes", "List
 available themes"}` entry + the `runCommand` case; the `themeApp(t)`
 harness (themedlg_test.go) wires a REAL engine (the t.TempDir KV) into
 the recApp (consumed by S3.9). Deviation 204
 (test-accuracy/plan-scope/low: 204a the pinned render `view(80, 24)`
 re-baselined to `view(80, 80)` — 33 theme rows vs the 6-row select
 window, the 200(e) class; 204b the substring order walk re-baselined
 to the line-based gutter-stripped line match — the "orng" ⊂
 "lucent-orng" name-substring collision; 204c `press(tea.KeyBackspace)`
 special-cased in the harness (Code only, no Text — `Text: "\x7f"`
 String()s to "\x7f", the string-based `key.Matches` never fires the
 "backspace" binding, and the sanitizer drops the \x7f — a silent
 no-op); 204d keys.go unchanged — the `handleDialogKey` case lives in
 dialog.go (the 199/201(c) class); the `localCommands` entry landed
 incrementally (the 197/200(b)/201(b) precedent) + the
 `TestPromptMenuKeys` wrap count re-baselined 8→9 items; 204e the
 live-theme-accessor payload field). 2 pinned test functions green
 (render + the 4-leg flow); full gate green (`go vet ./... && go test
 ./...` + `gofmt -l .`).
S3.9 theme commands landed (bead `yolo-oae.4.10`, commit 3032022):
  `themecmds.go` — `themeSwitchMode()` = `Pin(the opposite of the
  current mode)` (the upstream `setMode` === pin quirk verbatim: the
  switch both switches and locks — `Pin` applies the mode + persists
  theme_mode_lock + theme_mode, then `retheme` refreshes the styles),
  `themeModeLock()` = `locked() ? free() : pin(store.mode)` (lock =
  pin the current mode — persists both keys; unlock clears both and
  re-resolves the mode from the cached terminal luminance); nil
  engine → toast "theme engine unavailable" + nil on both; dynamic
  titles `switchModeTitle` (shows the NEXT mode — "Switch to light
  mode"/"Switch to dark mode") + `modeLockTitle` ("Unlock theme mode"
  / "Lock theme mode") for the S4 registry + the unit tests (no
  dynamic command titles pre-S4); NO default keys (upstream "none" —
  deviation 196; the S4.1 registry carries the defaults + the remap).
  `theme/engine.go`: the `KVPath()` + `FlushKV()` seams;
  `theme/kv.go`: `KV.Flush` — the synchronous log-and-continue barrier
  (serialized with the writer via k.mu + the flock, idempotent — the
  promise-chain writer design is unchanged; deviation 205). 3 pinned
  test functions green (the pin-quirk switch + lock/unlock + KV
  wiring — the fresh engine on the same KV file sees the persisted
  theme, the raw file after Close carries the mode keys); full gate
  green (`go vet ./... && go test ./...` + `gofmt -l .`).
S8 done (5/5 child beads closed, slice gate green — the parity capture +
sweep smoke performed under COLORTERM=truecolor; the user-run TTY smoke
is on-demand, pending) — epic close + tag PENDING USER GO-AHEAD (epic
`yolo-oae`; the 17-surface sweep: 0 MATCH / 17 logged, devs 258–260).
Prior release: v0.4.3 (2026-08-24) — allowlisted dependency bump
(PR #20, branch `chore/deps-update`) merged to `main` + tagged `v0.4.3`
+ GitHub release cut: bubbletea v2.0.9, bubbles v2.2.1,
modernc.org/sqlite v1.57.0, teatest v2.0.0-20260823001701 (dev);
no code/wire changes, gate green. Prior release: v0.4.2 (PR #19, branch
`many_words`, beads `yolo-0ca` + `yolo-ukc`): transcript + every
below-viewport surface word-wraps at the terminal width (`wrapLine`;
`SetWidth(w-3)` prompt fix).
Prior release: v0.4.1 (PR #18, branch `code_review`, bead `yolo-lkh`):
corrupt profile configs no longer break `List`/name-based ops, `buildDeps`
pins the loader to the RESOLVED profile id, `FakeFromEnv` follows
`env nil = real env`. v0.4.0 (direction-change docs + config profiles,
deviation 121, PRs #14–#17, tag `v0.4.0`); v0.3.0 (PR #11/#12, tag
`v0.3.0`, epic `yolo-5hy` closed; 0.3.0 backlog frozen in
`docs/superpowers/deferred-archive.md`, `DEFERRED.md` reset).
TUI parity design approved (2026-08-24, spec
`2026-08-24-opencode-tui-parity-design.md`, epic `yolo-oae`): full copy of
opencode's TUI (style, design, colors) — scope TUI-only contract-backed,
strict-copy bar; 33 themes + theme engine, glamour v2.0.1 transcript
rendering, huh v2.0.3 field dialogs + ported select, sahilm/fuzzy v0.1.3,
command palette, which-key + configurable keymap, prompt
history/frecency/autocomplete, home/session completion, parity audit vs
upstream pty captures; 9 slices `yolo-oae.1`–`.9`. Plan done (2026-08-25,
branch `new_tui`): directory `plans/2026-08-24-opencode-tui-parity/` —
`plan.md` (binding 65-bead inventory + Slice Detail Protocol) +
`s0-theme-engine.md` (S0.1–S0.10 full 5-step TDD + slice gate — the active
slice) + 8 slice briefs (S1–S8, each gated on its own detail pass before
execution). Deviations 122–125 pre-drafted in the plan (logged in-commit at
execution time). Execution starts on user go-ahead.
The harness-testing scope remains a future spec.

## Root causes (archive, v0.1.3)

(1) `16d0483` (v0.1.2 datastruct-2) re-wrote the shared
end-marker regex without re-teaching `decodeMarker` — from the 2nd bash command on the
reported exit code was the marker counter and the cwd was never decoded (a latent extra
bug: `pwd`'s trailing newline in the base64 made the respawn `os.Stat` fail → always
respawned in the root dir); (2) the TUI never noticed its SSE stream dropping (silent
reconnect, no re-hydrate) — a lost `session.status` left the footer stuck on `busy`
forever ("hang") with a stale transcript ("nothing printed"); (3) bash output was
row-only until alt+e (upstream shows it inline); (4) truncated bash output reached the
model silently — `tail()` was ported without upstream's full-output save + `Full output
saved to:` marker (plan Task 11 pinned only `tail()`), so a 1036-of-1209-line CI-gate run
arrived mid-stream and the model re-ran the gate ~14×; (5) plan Task 16's LOCKED mapping
RE-APPENDED the newest user message at the tail of every tool-call round, so the model
re-saw its own instruction each round and re-ran tools in a loop even with (4) fixed —
upstream replays history 1:1 (round ends with the tool result). Decisive diff: the same
prompt+model does NOT loop in upstream opencode. Detail: deviations 73–77.

## Last completed

Slice S0 tasks 1–5 (2026-08-25, branch `new_tui`, epic `yolo-oae`, beads
`yolo-oae.1.1`–`.5` closed; every task reviewed clean): theme-engine
foundation — S0.1 embed 33 upstream theme JSONs + `ThemeJson` model
(`c9364ed`); S0.2 `resolveTheme` + 33×2 golden matrix + node oracle
(`6622f7d`); S0.3 `Theme` struct + 51 lipgloss style accessors
(`48bf062`); S0.4 `generateSystem` port — grays/muted/tint/terminalMode
(`9bdaab7`); S0.5 OSC 11/10/4 palette detection + `DetectStd` + x/term
promotion (`7de800a`, fix round `8619252`). Deviations 122–129.
v0.4.0 direction-change docs merged to `main` (2026-08-24, branch
`v0.4.0_spec`, epic `yolo-5u1`): spec `2026-08-24-v0.4.0-design.md`
(beads `yolo-5u1.1`), plan `2026-08-24-v0.4.0-direction-change.md`
(beads `yolo-5u1.3`), restated root `AGENTS.md` (purpose + principles 2–3),
`README.md` (intro + purpose note), internal DOX chain
(`internal/AGENTS.md` + `internal/protocol/AGENTS.md`), this record
(beads `yolo-5u1.2`); deviations 119–120; `TestDescPinned` now pins all
seven tool desc files. Docs-only scope: purpose change to harness
testing, opencode demoted to reference, pins as change gates.
Profile edit (2026-08-24, branch `feat/profile-edit`, beads `yolo-bjp`,
PR #16): `yolo profile edit <id_or_name> [-n name] [-d description]` —
change a profile's display name and/or description after creation.
`config.Edit` (Copy-style single-`yolo.jsonc` rewrite); absent flag keeps,
empty value clears (`-n ""` → name falls back to id; both empty drops the
`profile` element); id and active marker untouched; rename to own name =
no-op, collision with another profile = `ErrNameTaken`.
Profile support (2026-08-24, branch `feat/profiles`, beads `yolo-3pe`):
config profiles under `~/.config/yolo/<profile_id>/` with an active
marker + `yolo profile` CLI + per-run selection (`--profile` flag >
`YOLO_PROFILE` env > marker > `default` recovery). Deviation 121 (hard
deviation/high, no upstream counterpart). Implementation:
`internal/config/profile.go`, `protocol.Config.Profile`, profile-aware
`Loader.Load` / `Server.globalDir()` / `buildDeps`. Gate green
(`go vet` + `go test ./...` + `gofmt`).
0.3.0 Plan 2 (refactor slice) complete (2026-08-24, branch
`v0.3.0-plan-2`): all 16 wave-8 refactors closed as beads
`yolo-5hy.2.1`–`.16` (engine test-harness + engine 4-way split + runRound/
executeTool extracts, pure mapHistory, shared llm sseLoop + anRequest
builders, server contract-suite + handler splits, storage per-entity DAOs,
cmd/yolo deps.go, store per-event Apply, app.go 5-way split, read tool
extracts, Shell execTimeout + markerCmd, dialog payload ownership),
DEFERRED.md dispositions landed, close-out gate green incl. `-race` +
`golangci-lint`. Deviations 116–118 logged (runRound line target vs named
extracts; R16 test-reference rewrite vs "UNMODIFIED" pin; TUI prompt-suite
`-race` flake — brittle contiguous-substring WaitFors hardened to
strip-SGR + independent tokens).
0.3.0 Plan 1 (defect slice) complete (2026-08-24, branch `v0.3.0`): all 39
plan tasks closed as beads `yolo-5hy.1.1`–`.39` (engine lifecycle, storage,
tools, server, TUI, CLI/e2e, naming V1–V8, two hermetic benchmarks),
DEFERRED.md dispositions landed, close-out gate green incl. `-race` +
`golangci-lint`. Deviations 112–115 logged (plan test-code fixes in W/X/AC +
the race-tolerant draft amortization bound). Next: Plan 2 (refactors, 16
tasks, spec §4).
(Prior: v0.2.0 released 2026-08-23 — 16 tasks, deviations 78–88, `just
e2e-live` PASS pre-tag, epic `yolo-8vl` closed; v0.1.3 released — PR #7,
deviations 73–77. Detail in `git log --oneline`.)

## Key verified facts (so they don't get re-litigated)

- Permission engine = port of `packages/opencode/src/permission/index.ts` + matrices in
`agent/agent.ts` (build/plan/yolo verbatim, Task 10).
- Doom loop = sliding 3-identical window; wildcard-deny hides tool iff last matching rule
is `*` deny; `write`+`edit` both map to permission `edit`.
 - Pinned deps (2026-08-24 update, all allowlisted, gate green):
 `charm.land/bubbletea/v2` v2.0.9, `charm.land/lipgloss/v2` v2.0.6,
 `charm.land/bubbles/v2` v2.2.1, `modernc.org/sqlite` v1.57.0 (pure Go, no
 cgo), `tidwall/jsonc` v0.3.3; dev-only `teatest/v2`
 v2.0.0-20260823001701-96af6d2cb5f6.
- glamour v2.0.1 landed (S1.1, 16 new modules, live-verified evidence in bead
`yolo-oae.2.11`); its custom chroma map registers under the global "charm"
slot (first-write-wins) — yolo deletes the slot before every Render
 (Renderer.Render, internal/tui/theme/syntax.go) so the transcript (full)
 and reasoning (subtle) renderers + SIGUSR2 theme switches never cross-color.
- S2.1 landed huh v2.0.3 + sahilm/fuzzy v0.1.3 (MVS delta 10 modules; smoke
 render green under yolo pins).
- Module `github.com/kido5217/yolo`, Go ≥ 1.25 (installed 1.26.7).
- Single deliberate wire deviation: `x-yolo-directory` header.
- Config profiles (2026-08-24, deviation 121, beads `yolo-3pe`): global
config lives at `~/.config/yolo/<profile_id>/` (precedence `config.json`
< `yolo.json` < `yolo.jsonc`); id auto-generated 8-hex (first-run literal
`default`); `~/.config/yolo/active` = active marker; selection =
`--profile` flag > `YOLO_PROFILE` env > marker > `default` recovery;
`yolo profile list|add [name] [-d DESC]|use|edit REF [-n NAME]
[-d DESC]|remove|copy SRC NAME [-d DESC]`;
name unique + id-then-name resolution (dup name = ambiguous error); legacy
flat files ignored; data dir shared.
- Test gating: unit tests never hit network; `YOLO_LLM=fake` (+ `YOLO_FAKE_SCRIPT`) selects
the scripted fake driver; zen fixture gate = 57 models (42 openai + 15 anthropic,
7 google excluded).
- TUI import rule: non-test files under `internal/tui/` import only `internal/protocol` +
`internal/tui/*`; `_test.go` may use `internal/server/testutil` (escape hatch).
- TUI transcript word-wrap (2026-08-24, bead `yolo-0ca`): the bubbles
viewport hard-CLIPS over-width lines and the TUI binds no horizontal
scroll, so pre-wrap the transcript lost everything past the right edge
(unreadable; upstream ink word-wraps). `wrapLine` (`internal/tui/wrap.go`)
word-wraps at the viewport width (word boundaries, over-long tokens
hard-split, CJK/emoji = 2 columns, tab = separator, plain text only);
styled lines wrap before styling (`toolRow` returns style + plain);
`WindowSizeMsg` re-wraps via `sess.isDirty`. Tests: `TestWrapLine`,
`TestRenderMessagesWrapsLongLines`, `TestTUILongReplyWraps` (the last word
of a 1000-word single-line fake reply reaches the screen).
- `wrapLine` is ANSI-aware (2026-08-30, bead `yolo-kj6`): a CSI escape (SGR
styling) is zero-width glue — it counts toward no display width and a hard-split
never cuts inside it (`csiLen`/`ansiWidth`/`ansiCutWidth` in `wrap.go`), so a
styled-then-wrapped row wraps on its VISIBLE width and no corrupted escape reaches
the terminal; plain text wraps byte-identically (`ansiWidth` == `runeWidth`, so the
plain-text `runeWidth`/`cutWidth` callers in `home.go`/`select.go`/`locale.go` are
untouched). It matters for `permDlg.view`, which styles each row then wraps it
(S2.8): pre-fix the reply-pill row (`runeWidth` 157, `ansiWidth` 34) was buggily
wrapped at the 60-col panel — now one line; the SGR golden's pill pin re-baselined
to the un-wrapped layout (dev 187). Tests: `TestWrapLineANSIAware`,
`TestWrapLineANSIAwarePlainUnchanged`.
- TUI below-viewport surface wrap (2026-08-24, bead `yolo-ukc`): toasts,
the permission overlay, the slash menu, the model/agent dialogs (rows AND
hint lines, via `dimWrapped`), the home session rows and the `!` error line
all wrap at the terminal width (`App.termWidth()`, fallback 80) with the
same `wrapLine`; the session route's viewport height counts the wrapped
help line's real line count. The model dialog's cell hangs at the left-pane
column (`modelRow`); the left pane alone ≥ width degenerates to full-width
cell lines. Footer, divider and the locked quit/help dialogs stay
single-line. Prompt width arithmetic: bubbles v2 textinput `View` =
prompt(2) + `SetWidth` + cursor(1), so `WindowSizeMsg` sets `SetWidth(w-3)`
(pre-fix `w-2` left the prompt line 1 column past the edge). Tests:
`internal/tui/overflow_test.go` (7 wrap tests incl. the composed-frame fit).
- v1 behavior pins: keymap is pgup/pgdn scroll + `\`+enter newline (noted in /help; spec's
↑/↓ viewport scroll replaced); JSONC comments are NOT preserved when a config PATCH
rewrites `yolo.jsonc`.
- lipgloss v2 `Render()` appends a trailing SGR reset AFTER the styled input: trim padded
plain strings (`TrimRight`) BEFORE styling (a styled string's last bytes are `\x1b[m`, so a
post-style trim silently misses), and count display widths in runes
(`utf8.RuneCountInString`) — `·` is 2 bytes, `○` 3 (both 1 column). Both bit T27's two-pane
column math.
- e2e/endpoint facts: `scripts/e2e-live.sh` (entry point `just e2e-live`),
  validated PASS against the REAL `https://ai.kido.ws/v1` on 2026-08-24
  (post Plan 1 merge; the pre-tag re-validation of spec §5) — success shape:
  session list + rename round-trip (`GET /session` rows carry `.id`/`.title`/
  `.agent`; `PATCH /session/{id} {"title":…}` returns the post-update row;
  re-list shows the rename); config `theme` STRING round-trip
  (`PATCH /config {"theme":"aura"}` → 200 + `.theme` string, re-`GET` shows it
  persisted — the S0 map→string wire change); completed bash tool call + text
  reply; abort idle → `aborted:false`, busy → `aborted:true`; SIGTERM → exit 0.
  The new S3 wire-leg steps (3–4) were offline-validated 2026-09-02 via the
  `YOLO_LLM=fake` + `YOLO_FAKE_SCRIPT` driver (scripted glob turn; the yolo
  agent's catch-all permission allow means no prompt stall) — full run PASS,
  exit 0; live re-validation ran against the real endpoint the same day —
   full run PASS incl. the abort-while-busy leg (`aborted:true` observed live
   for the first time; script contract unchanged: on-demand, never CI).
   Re-run 2026-09-02 post-S4 (branch `new_tui` @ `1729dc2`, the S4-complete
   tree): full live PASS again — the `main.go` `SetKeybinds` startup wiring +
   the `keybinds` config field changed neither startup nor the wire shape.
   Re-run 2026-09-03 post-S8 (branch `new_tui` @ `3201d2f`, the S8-complete
   tree): full live PASS again (incl. the abort-while-busy leg `aborted:true`)
   — the wire surface (protocol/config/server/session) is unchanged since the
   1729dc2 run (S5–S8 are TUI + parity tooling only), so `e2e-live.sh` needed
   no wire-leg change (its header TTY-leg note extended to S5–S7 + the S8
   parity scripts).
   `ai.kido.ws` accepts ANY bearer token
   (private endpoint — key order env → auth.json → config).
  `GET /global/health` → `{"status":"ok"}`; `/session/{id}/message` rows =
  `{"info":{role,error:{type},...},"parts":[...]}` (jq: `.info.role`). Script
  mechanics: `req()` must set globals (never run inside `$(…)` — subshell
  loses `HTTP_STATUS`); boot from the scratch project dir (deviation 65).
- teatest v2 output mechanics (bit T28's suites): (a) each `WaitFor` drains the SHARED
output buffer — consecutive `WaitFor`s observe DISJOINT slices, so a multi-token terminal
state must be ONE merged condition, never two sequential waits (an idle app emits no new
frames for the second); probe `Read`s consume bytes later assertions need; (b) the fake
terminal is not a TTY → lipgloss strips EVERY style; pin `teatest.WithProgramOptions(tea.WithEnvironment([]string{"TTY_FORCE=1","TERM=xterm-256color"}))` for deterministic
ANSI256 SGR (derived from TERM alone, no terminfo; `charmbracelet/colorprofile` stays
indirect — never import directly, "no other deps" rule); (c) v2 `tea.Tick(d, f)` callback
is `func(time.Time) tea.Msg` (v1: `func() tea.Msg`); v2 programs handle `tea.QuitMsg`
internally.
- Shell end-marker wire form (v0.1.3, verified against live markers):
`__YOLO_END_{n}_{exit}_{b64(pwd incl. trailing \n)}` matched by the shared regex
`^__YOLO_END_(\d+)_([^\s]*)$` with `m[1]==n`; `decodeMarker` splits `m[2]` at the first
`_` (std base64 has no `_`) and trims `pwd`'s newline. The emitted marker has never
carried a colon (one stale comment claimed `_{n}_:`, a doc typo since the protocol's
first commit `ae0ff27`); the pre-v0.1.3 `decodeMarker` mis-parsed the GROUP positions,
not a separator. The first command (n=0) masked the group bug because counter==0.
- SSE drop contract (v0.1.3): `client.Events` returns `(events, resync)`; a ping per drop
(buffered, non-blocking); app re-hydrates the current route on `resyncMsg` (the bus has
no replay — gap events are unrecoverable, recovery is REST hydration from storage).
- go-udiff v0.4.1 pinned as the direct line-diff dependency (2026-08-23, 0.3.0 task N,
deviation 104) — the sole new dependency of 0.3.0 (root AGENTS.md allowlist, proposal #1).
- v0.4.0 direction change (2026-08-24, user-approved; spec
  `docs/superpowers/specs/2026-08-24-v0.4.0-design.md`): the original
  Qwen3.8-27B testing goal is complete (local Qwen 3.8 tested, stable,
  optimized). From v0.4.0 the project tests various LLM harnesses and
  frameworks — yolo drives/evaluates other harnesses (the subsystem itself is
  a future architectural spec). opencode v1.18.18 is a reference, not a
  contract: yolo may deviate (wire shapes, behavior, pinned text) on explicit
  user instruction, each deviation logged in `DEVIATIONS.md` with severity.
  The 21 sha256-pinned files (14 `session/prompt/*.txt` + 7 `tool/desc/*.txt`)
  record current intended content, not an upstream lock — an intentional
  change re-baselines the pin in the same commit.

- v0.4.0 post-release code review (2026-08-24, range a2379c1..9c37870, all
  findings fixed on branch code_review): a corrupt sibling profile config
  no longer breaks `List`/name-based `Resolve`/`Add`/`Remove` (id fallback,
  blank metadata); `buildDeps` pins the loader to the RESOLVED profile id
  and `FakeFromEnv` follows the `env nil = real env` convention (a bare
  nil map is not the real env); README documents the ignored pre-v0.4.0
  flat files and the `--profile` flag. User accepted beads-only tracking
   (no dated spec/plan) as sufficient for bounded features such as the
   profiles work (deviation 121); the spec-first workflow rule still
   applies to architectural work.
- Theme engine S0.1–S0.5 (2026-08-25, slice S0, branch `new_tui`):
  `internal/tui/theme` is a strict-copy port of the upstream opencode
  theme engine — the FLOAT operation order is binding (`Tint` blends in
  0–1; grays/muted do 0–255 floor/min/max) or the goldens drift. Golden
  harness: `scripts/tui-theme-golden.mjs` (node oracle) +
  `testdata/{theme,system,terminal-mode}-golden.json`; S0.4 fixed three
  oracle bugs and regenerated (0–1 terminalMode scale — #7f7f7f is
  "dark"; hexToRgb fixture plumbing; NaN collapse upstream does at
  `RGBA.fromInts` construction — `uint8(NaN)=0` on x86_64, e.g.
  `system.black.light` diff line-number bgs `#001200ff`/`#120000ff`,
  NOT `#000000ff`). OSC detection: single-buffer demux stores the
  probe's OSC 4;0 answer as `Palette[0]` first-wins (test-pinned,
  deviation 129 note — indistinguishable on real terminals; only
  `palette[0]` PRESENCE gates system-theme eligibility in S0.7);
  `DetectStd` probes via an owned `/dev/tty` in raw mode ONLY (deviation
  129) — no controlling terminal → `(zero, false)`, no system theme
  (spec §3); timeouts spec-pinned 100/100/100 ms vs upstream 300/300/5 s
  (deviation 128; `PaletteOptions` overridable). Follow-up bead
  `yolo-oae.1.12` (P1, blocked by S0.7): on Linux `close()` does not
  wake a kernel-blocked tty read — the probe pump can linger until the
  next input and discard it; fix (poll-with-timeout pump / inline
  reads) lands with S0.7's wiring.
- Dep promotion (2026-08-25, bead `yolo-oae.1.11`, user-approved
  proposal): `github.com/charmbracelet/x/term` v0.2.2 promoted indirect →
  direct (raw-mode tty for OSC palette detection); ZERO new modules —
  already in the module graph via bubbletea v2; now on the root
  AGENTS.md allowlist.
 - Deviation renumbering (slice S0, supersedes the plan's 122–125 map):
   the log now runs 122–129 (122/123 S0.2, 124 S0.3, 125/126 S0.4,
   127/128/129 S0.5); remaining plan entries keep their TEXT with shifted
   numbers: S0.7 config.theme wire → 130, S0.7 single-probe scoping → 131,
   slice-gate SGR quantization → 132 (cross-refs: S0.7 step 5,
   `config_test.go` comment, S0.10 DOX bullet, slice gate steps 4/5/7).
- TUI parity audit S8 (2026-09-03, slice `yolo-oae.9`, branch `new_tui`):
  the deterministic parity runtime + the 17-surface diff sweep landed; the
  sweep verdict is the record (the D7 close-or-log judgment — the report
  `plans/2026-08-24-opencode-tui-parity/parity-sweep-report.md` is the
  mechanical record): 0/17 MATCH, all 17 surfaces LOGGED — dev 258 (info,
  all 17: the bg-fill rendering model — upstream opentui repaints the full
  opaque background every frame, yolo bubbletea v2 diff-renders only its
  content cells), 259 (medium, 16: the content/layout divergence — the
  transcript viewport scroll-to-tail vs the full reply, the command-set
  subsets, the chrome — the expected "reference, not contract" port scope,
  root principle 2), 260 (medium, 1: the exit epilogue — yolo prints
  nothing after exit, upstream prints the opencode-branded `Continue
  opencode -s <SES>` epilogue; logged per the D8(1) fallback — a true
  MATCH would be a brand mismatch + a yolo behavior change beyond parity
  scope). The runtime: `internal/llm/mockllm` (the byte-deterministic
  canned OpenAI-compatible SSE — text/tool/todo turns, fixed ids
  `chatcmpl-canned01`/`call_canned1`/`call_canned2`, `created
  1700000000`, model `canned`, usage 12/40; `TestCannedMatchesDefault`
  pins the shared fixture) + `scripts/parity/mock` (127.0.0.1, the
  `MOCK_PORT=<port>` handshake) + `scripts/parity/{capture.sh,capture.py,
  normalize.py,sweep.py}` (user-run, never CI — `just parity-capture` /
  `just parity-sweep`; the npm `opencode-ai@1.18.18` runs hermetically
  against the mock: `OPENCODE_MODELS_PATH` catalog file, fetch + auto-
  update disabled; devs 254–255 — the fresh-HOME-per-run + 6 s boot-settle
  runtime adaptations + the `<DUR>`/`<EX>`/split-TS volatile masks) +
  `TestParityDump` (gated on `YOLO_PARITY_DUMP`, `t.Skip` when unset —
  the CI gate never renders it; the fake driver scripted from the shared
  canned book, `TestParityCannedConsistent`). Fixture pin:
  `internal/tui/testdata/parity/` — `canned.json` (shared),
   `catalog-pin.json` (the reduced `{openai}` catalog snapshot, re-fetched
   at capture — the committed sha supersedes the plan's detail-time
   snapshot `3df03cfe`),
  `upstream/` the 17 NORMALIZED screens + `MANIFEST.json` (npm 1.18.18,
  per-surface `{name,cols,rows,sha256}`; `TestParityFixturesPinned` fails
  on any drift) — re-baselined 2026-09-03 after the normalizer's faithful
  terminal replay (dev 257: LNM/IND/NEL/RI, pending-wrap, the erase ops,
  DECSTBM+SU/SD, tracked cursor moves, ESC 7/8/c, TAB, ST-aware OSC — the
   `[1 q` DECSCUSR fragment no longer leaks, so 14/17 fixtures
   re-baselined (help, session-rename, status were byte-identical under the
   new normalizer); the swapped-scroll-direction fix lands the `epilogue`
  exit lines at rows 20–21; the yolo temp-dir mask widened). The capture +
  sweep ran under `COLORTERM=truecolor` (the upstream 24-bit SGR confirmed
  — deviation 125 holds; the yolo side is ANSI256 — the expected
  color-space class inside 258/259); the D5 double-run determinism gate
  passed on every surface.
