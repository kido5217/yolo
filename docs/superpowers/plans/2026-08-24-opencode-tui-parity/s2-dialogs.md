# S2 — dialog system (slice bead `yolo-oae.3`)

Port opencode's dialog system — the modal stack, the huh field dialogs
(alert/confirm/input), and the select component — then restyle the
permission, model, and agent dialogs on top of it.

**State: fully detailed** — the 5-step TDD detail for all 10 tasks is in
the `## S2 detail` section below; execution may start at task S2.1.

## Binding task table

Pointer only (FROZEN — Slice Detail Protocol rule 1): `plan.md` →
`## Task inventory` → `### S2 — dialog system (slice bead yolo-oae.3)`.
Bead titles, scope, and pinned commit messages live there and may not be
changed by a detail pass; any required change = STOP + explicit user
approval + re-record in plan.md.

## Dep gate

`charm.land/huh/v2` v2.0.3 + `github.com/sahilm/fuzzy` v0.1.3 (tasks S2.1) —
dep-proposal bead first (root AGENTS.md dependency policy: evidence from
live web search — maintenance, license, pure Go, transitive surface;
approval gate = STOP before `go get`; both modules land as task S2.1).

## Exact upstream sources (the detail pass reads these AT DETAIL TIME)

`/tmp/opencode-upstream` @ v1.18.18:

- `packages/tui/src/ui/dialog.tsx` — the modal stack: centered overlay,
  focus capture, esc, stackable, result callback.
- `packages/tui/src/ui/dialog-select.tsx` — the select core: `Option`
  732–791, the active-row box 667–678, the fuzzy filter, categories,
  actions, footer hints, scroll acceleration (S0.9 already ported the home
  LIST row tokens — S2 ports the component itself).
- `packages/tui/src/ui/dialog-alert.tsx`,
  `packages/tui/src/ui/dialog-confirm.tsx`,
  `packages/tui/src/ui/dialog-prompt.tsx` — the field dialogs
  (huh-equivalents).
- `packages/tui/src/component/dialog-model.tsx` (S2.9),
  `packages/tui/src/component/dialog-agent.tsx` (S2.10) — the restyle
  sources.
- `packages/tui/src/routes/session/permission.tsx` — the S2.8 permission
  dialog on the select stack.

## yolo anchors

- `internal/tui/dialog.go` — the existing dialog surface; its
  `title`/`divider` statics in `internal/tui/style.go` get consumed here
  (statics yield to theme tokens).
- `internal/tui/app.go` — overlay/layer composition for the modal stack.
- `internal/tui/theme/` — theming via `StyleConfig` from the resolved theme
  tokens.
- bubbles v2 + the S2 dep gate (`huh`/`sahilm/fuzzy`) — the new interactive
  primitives.

## Detail pass (protocol)

One writing-plans pass, one subagent, `thinking=high`, dispatched by the
root session strictly sequentially (root principle 7) — per the Slice
Detail Protocol in plan.md: it fills this file (after this section) with the
full 5-step TDD detail for each task in the binding table (failing test
code, implementation code, gate, pinned commit), reading the named upstream
files at that moment. It commits as
`docs: TUI parity plan — detail S2 tasks` on its own bead
(`bd create "detail S2 plan tasks" --parent=yolo-oae.3 --json`).

## S2 detail

Detail pass 2026-08-27. Deviations tail at detail time = 164; S2 entries
start at 165. Breadcrumb note (DEVIATIONS.md entry 165, severity info):
the frozen table names the task beads `yolo-oae.3.1`–`3.10`, but the S2
detail pass consumed bead numbers `yolo-oae.3.1` (detail bead, claimed) and
`.2`/`.3` (duplicate detail beads, closed) before the task beads were
created (S1 precedent puts the detail bead LAST — it could not for S2
because the detail pass precedes slice start). The 10 task beads land in
table order at `yolo-oae.3.4`–`yolo-oae.3.13`; titles and pinned commit
messages are unchanged. The mapping table (task → bead id) lives in the
slice gate record.

### Detail-pass findings (read at detail time, 2026-08-27 — binding)

1. **huh v2.0.3 API (verified from the module cache, `charm.land/huh/v2@v2.0.3`):**
   - `huh.NewForm(groups ...*huh.Group) *huh.Form`; `*huh.Form` IS a
     `tea.Model` (Init/Update/View); fields: `f.State`
     (`StateNormal`/`StateCompleted`/`StateAborted`), `f.Get(key any) any`,
     `f.GetBool(key)`, `f.GetString(key)`; options `.WithTheme(huh.Theme)`,
     `.WithWidth(int)`, `.WithHeight(int)`, `.WithShowHelp(bool)`,
     `.WithKeyMap(huh.KeyMap)`.
   - `huh.NewGroup(fields ...huh.Field)` → `.Title(string)`,
     `.Description(string)`.
   - `huh.NewConfirm()` → `.Title(string)`, `.Description(string)`,
     `.Affirmative(string)`, `.Negative(string)`, `.Value(*bool)`.
     **Empty `Negative("")` renders the single affirmative button and
     disables the reject keys** (this is how alert reuses confirm).
     Confirm keymap: `h`/`l`/`left`/`right` toggle, `y`/`Y` accept,
     `n`/`N` reject, `enter` submit, `shift+tab`/`tab` prev/next.
   - `huh.NewInput()` → `.Title(string)`, `.Description(string)`,
     `.Value(*string)`, `.Placeholder(string)`, `.Prompt(string)` (default
     `">"`), embeds a bubbles textinput (Init/Update/View pass-through).
   - **Default form keymap Quit = `ctrl+c` ONLY — esc is NOT bound.** The
     app stack keeps owning esc (S2.2); ctrl+c closes the top modal (both
     are "Close dialog" bindings upstream, dialog.tsx:109-124).
   - `huh.Theme` is an interface `Theme(isDark bool) *huh.Styles` (+ a
     `ThemeFunc` adapter). `huh.Styles`: `Form.FormStyles{Base}`;
     `Group.GroupStyles{Base,Title,Description}`; `Focused`/`Blurred`
     `huh.FieldStyles` (incl. `Base`, `Title`, `Description`,
     `TextInput huh.TextInputStyles{Cursor,CursorText,Placeholder,Prompt,Text}`,
     `FocusedButton`, `BlurredButton`); `Help help.Styles`. Default
     `Focused.Base` carries a thick LEFT border (`BorderStyle(ThickBorder).
     BorderLeft(true)`) — the standard huh look. Our theme sets
     `Focused.Base`/`Blurred.Base` to empty styles (borderless, upstream
     dialog look; deviation 167 covers the residual look gap).
   - huh v2.0.3 go.mod direct requires (all at or below yolo's pins — MVS
     keeps yolo's): bubbles v2.0.0, bubbletea v2.0.2, lipgloss v2.0.1,
     catppuccin/go v0.2.0, x/ansi v0.11.6 (graph has v0.11.8), x/exp/ordered
     v0.1.0, x/exp/strings v0.0.0-20240722160745, x/term v0.2.2 (already a
     yolo direct dep), x/xpty v0.1.3, hashstructure/v2 v2.0.2. New module
     lines vs the 69-module baseline (2026-08-27, `go list -m all | wc -l`):
     catppuccin/go, x/exp/ordered, x/exp/strings, x/xpty, hashstructure/v2,
     x/conpty, x/errors, creack/pty = **8 new** + huh + fuzzy = 10 new lines
     (live delta after `go get` is authoritative; note: `go list -m all`
     already lists fuzzy v0.1.3 in the baseline module graph even though
     nothing requires it — a graph quirk, not a require).
2. **fuzzy v0.1.3 (verified from the module cache):** single package, zero
   go.mod requires. `fuzzy.Find(pattern string, data []string) Matches`
   (sorted by score desc), `FindNoSort`, `Matches = []Match{Index int,
   Score int}`; case-insensitive subsequence matching. Weighted multi-key
   (upstream `fuzzysort.go(keywords, {keys:["title","category"],
   scoreFn: r => r[0].score*2 + r[1].score})`) ports to two `fuzzy.Find`
   calls (titles ×2, categories ×1) summed per index, stable sort.
3. **teatest v2** (`tm.Send(m tea.Msg)`) forwards arbitrary msgs, so the
   modal/huh/select units can drive `tea.KeyPressMsg` and
     `tea.WindowSizeMsg` directly; SGR goldens stay pen-diff style
     (S0.9/S1.3 conventions: `TTY_FORCE=1` + `TERM=xterm-256color` →
     `38;5;N`/`48;5;N`; contiguous tokens, substring asserts).
4. **ANSI256 indices of the opencode-dark tokens (via x/ansi v0.11.8
   `Convert256` — binding for the S2.8–S2.10 SGR goldens):**
   background #0a0a0a→**232**, backgroundPanel #141414→**233**,
   backgroundElement #1e1e1e→**234**, primary #fab283→**216**,
   text #eeeeee→**255**, textMuted #808080→**244**, warning #f5a742→**215**,
   accent #9d7cd8→**140**, borderSubtle #3c3c3c→**237**.
5. **The server does NOT populate `PermissionAskedProps.Metadata`** (the
   core `req.Meta` is never set on permission requests — verified in
   `internal/server` + `internal/session`), and the wire has no diff field.
   So S2.8's permission detail lines come from the **store part lookup**:
   `Part.State.Input map[string]any` matched by
   `PermissionToolRef{MessageID, CallID}` → `Part.CallID` (the tool input
   JSON — e.g. `file_path`, `command`, `pattern`). This is strictly less
   than upstream's rich `Meta` (title/subtitle/labels + `MetaDiff` diff
   view) — deviation 169.
6. **Re-baseline inventory (the ONLY goldens S2 touches):**
   - S2.2: none (new surface; existing model/agent/quit/help blocks are
     left non-modal and byte-identical this task).
   - S2.3/S2.4/S2.5/S2.6/S2.7: none (new surface; unit tests only).
   - S2.8: `permission_test.go` (all 12 pins), `tui_suite_test.go`
     `TestTUIFullTurn` `hasPermDialogEcho` assertion, `overflow_test.go`
     (1 pin referencing the permission dialog).
   - S2.9: `model_test.go` (all), `tui_suite_test.go` `TestTUIDialogs`
     ("Qwen*" → "● Qwen" row), `overflow_test.go` (model pins).
   - S2.10: `agent_test.go` (all), `tui_suite_test.go` `TestTUIDialogs`
     (agent row).

### Design decisions (binding)

**Modal stack (S2.2)** — port of `dialog.tsx`:
- The panel is a horizontally centered box, top at `max(h/4, minChrome)`
  (upstream overlay `paddingTop = floor(H/4)`, panel `paddingTop = 1`,
  `width = size`, `maxWidth = W-2`), backgroundPanel fill, no border.
- Backdrop = plain terminal-background lines (deviation 166: the upstream
  `rgba(0,0,0,0.15)` dim has no SGR-alpha equivalent; the blank cover is
  the honest degradation).
- `esc` AND `ctrl+c` close the top modal (upstream binds both,
  dialog.tsx:109-124); a payload implementing `modalCanceler` gets first
  refusal on esc (inner state, e.g. the model/agent sub-choice).
- While a modal is open: prompt, menu, toasts, lastErr are suppressed; the
  route chrome renders clamped (session viewport shrinks; home recent list
  empties) so the frame stays exactly H lines; the footer stays on the
  last line.
- Quit/help dialogs stay NON-modal append blocks (yolo-specific; upstream
  has no quit/help dialog — design note, not a deviation; their locked
  text is untouched).
- The existing model/agent/permission rendering is untouched by S2.2;
  model/agent flip to modal in S2.9/S2.10.

**huh embedding (S2.3/S2.4)** — the form is a child model:
- The app owns esc/ctrl+c (stack close); every other key is forwarded to
  `form.Update(k)`; `WindowSizeMsg` is forwarded on open (huh sizes itself
  on the first WindowSizeMsg).
- `f.State` watch: `StateCompleted` → read the value → fire the dialog's
  callback → pop; `StateAborted` → pop (no callback).
- alert = one Confirm field, `Affirmative("ok")`, `Negative("")` (single
  button — upstream alert is a lone "ok" pill, dialog-alert.tsx).
- confirm = one Confirm field, `Affirmative("Confirm")`,
  `Negative("Cancel")`, `.Value(&true)` (upstream starts on "confirm",
  dialog-confirm.tsx: default active pill = confirm).
- input = one Input field, `.Value(&initial)`, `.Placeholder(...)`,
  prompt `">"` (upstream dialog-prompt.tsx).
- Theme: a `themeDialog(th theme.Theme) huh.Theme` adapter maps the
  resolved tokens (title bold text, description textMuted, buttons:
  affirmative = primary bg + SelectedForeground fg, negative = textMuted
  fg; borderless `Base`s — see finding 1).
- S2.3/S2.4 ship as pure units (no production opener yet — the first
  production call sites land with S2.8–S2.10 and S3); SGR goldens for the
  rendered forms land then.

**Select (S2.5–S2.7)** — new file `internal/tui/select.go`, port of
`dialog-select.tsx`:
- `selectOption{title, description, details []string, footer, category,
  value any, disabled}`; `selectModel{title, placeholder, options,
  isCurrent func(selectOption) bool, onSelect func(*App, selectOption),
  onMove func(selectOption), sel, top, filter string, input
  textinput.Model, actions []selectAction, hints []footerHint, focAct
  int}`.
- S2.5: the filtered list (fuzzy, finding 2; disabled options excluded —
  upstream `filter` removes them entirely), selection with wrap, home/end,
  enter → `onSelect`; non-matching keys feed the filter input.
- S2.6: category header rows (accent fg bold, indent 3, blank row between
  groups; hidden while filtering — upstream `flat`), per-option detail
  rows (textMuted, truncateMiddle, indent 7), the `footer` right tail
  (textMuted), scroll window over ROWS (headers+details count), visible
  rows = `min(len, floor(h/2)-6)` (upstream `height` memo).
- S2.7: actions (left side, `key.Binding`, tab/shift+tab focus,
  enter/executes the focused one — upstream `actions`), footer hints
  (right side, textMuted, `key` + `desc`), scroll acceleration: pgup/pgdn
  = ±10 (upstream `CustomSpeedScroll(3)` ≈ 3× page — yolo pins ±10 rows,
  no `getScrollAcceleration` env machinery — deviation 170).
- Active row = full-row paint in primary bg with SelectedForeground text,
  bold title (dialog-select.tsx:667-678 + Option 732-791; reuses the S0.9
  home SELECT token chain); current option (not active) = `●` gutter +
  primary fg; other rows = text fg title + textMuted description tail;
  zero Theme degrades to plain rows with the `cursorStyle`-bold active
  title.
- S2.5–S2.7 ship as pure units (unit tests; no production opener until
  S2.9/S2.10).

**Permission restyle (S2.8)** — on the select stack:
- Header row: `△` warning fg + `Permission required` (upstream title).
- Body: the store-part `Input` map rendered as `key: value` lines
  (truncateMiddle at the row width; up to 4 lines, then `…`) — deviation
  169 (no rich Meta, no diff view).
- Pill row: `[Allow once] [Allow always] [Reject]` — selected pill =
  warning bg + SelectedForeground(theme, warning) fg bold; unselected =
  textMuted fg (upstream: selected = accent bg + accent fg; yolo pins
  warning per the S1 toast/error lineage — the pill tokens are yolo's,
  deviation 167's look note covers it).
- Keys: `1`/`2`/`3` select+confirm (yolo pin), `left`/`right` move
  selection, `enter` confirms the selected pill, `esc` = reject (yolo pin;
  upstream esc = close = reject-equivalent).
- The permission dialog is a plain (non-huh, non-select-model) dialog
  item — `dialog{kind: kindPermission, perm: permDlg}` — modal,
  `dlgMedium`; it does NOT use selectModel (upstream permission.tsx is
  itself a bespoke component, not a DialogSelect — faithful).
- Re-baseline: `permission_test.go`, `tui_suite_test.go`
  `hasPermDialogEcho`, `overflow_test.go` (inventory, finding 6).

**Model restyle (S2.9)** — two-pane → flat select (deviation 168, the one
user-visible behavior change):
- Options = catalog models: `title = model.Name`, `category =
  provider.Name`, `description = ctx + cost`, `footer = provider status`;
  current model → `isCurrent`. Enter on a model → the existing `a`/`b`
  sub-choice (yolo pin: `a` = this model only, `b` = all models of the
  provider) — the sub-choice is the select's inner state
  (`modalCanceler`). Filter over titles+categories.
- Modal, `dlgLarge` (upstream dialog-model.tsx is `size="large"`).
- Re-baseline: `model_test.go`, `tui_suite_test.go` `TestTUIDialogs`,
  `overflow_test.go` (finding 6).

**Agent restyle (S2.10)** — list → select:
- Options = agents: `title = name`, `description = agent description`,
  current agent → `isCurrent`. Enter → the existing `a`/`b` sub-choice
  (yolo pin: `a` = set as default, `b` = use in this session). Filter over
  titles. Modal, `dlgMedium` (upstream dialog-agent.tsx: `size="medium"`,
  plain DialogSelect).
- Re-baseline: `agent_test.go`, `tui_suite_test.go` `TestTUIDialogs`
  (finding 6).

**Deviation entries this slice owns (numbers sequential from 165):**
165 (info) task-bead id shift (breadcrumb above); 166 (low) modal backdrop
= blank cover, no alpha dim; 167 (low) huh field-dialog look ≠ upstream
borderless pills (themed as close as huh allows, borderless Base); 169
(low) permission detail = store-part input lines, no rich Meta/diff;
168 (medium) model dialog two-pane → flat select (behavior change,
approved by the spec's S2 row); 170 (info) scroll acceleration pinned ±10
rows, no env-based `getScrollAcceleration`.

---

### Task S2.1: Dep proposal huh v2.0.3 + sahilm/fuzzy v0.1.3 (bead `yolo-oae.3.1`, expected id `yolo-oae.3.4`)

**Files:** `go.mod`, `go.sum` (step 2, post-approval only), root `AGENTS.md` (allowlist), `docs/superpowers/PROGRESS.md`.

**Interfaces:** consumes the root dependency policy (allowlist + agent-proposable, extensive live web search evidence, approval gate before `go get`). Produces: `charm.land/huh/v2` v2.0.3 + `github.com/sahilm/fuzzy` v0.1.3 as direct requires. No other code changes — nothing imports either module yet (huh imports land in S2.3, fuzzy in S2.5), so **no `go mod tidy` anywhere in S2** (tidy between S2.3 and S2.5 would strip the unimported fuzzy require — the deviation-148 lesson).

**Upstream parity notes:** n/a (dependency landing; both are the plan's named modules for the huh field dialogs and the select's subsequence fuzzy filter).

**Step 1 — collect the live evidence, file the dep-proposal bead, then STOP for approval.**
Run at module root (live values bind; treat prior knowledge as stale):

```sh
# huh (charm.land vanity → github.com/charmbracelet/huh)
curl -s https://api.github.com/repos/charmbracelet/huh | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["license"]["spdx_id"], d["pushed_at"], d["open_issues_count"])'
curl -s https://api.github.com/repos/charmbracelet/huh/git/refs/tags/v2.0.3 | python3 -c 'import json,sys; print(json.load(sys.stdin)["object"]["sha"])'
go list -m -versions charm.land/huh/v2
# fuzzy
curl -s https://api.github.com/repos/sahilm/fuzzy | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["license"]["spdx_id"], d["pushed_at"], d["open_issues_count"])'
curl -s https://api.github.com/repos/sahilm/fuzzy/git/refs/tags/v0.1.3 | python3 -c 'import json,sys; print(json.load(sys.stdin)["object"]["sha"])'
go list -m -versions github.com/sahilm/fuzzy
# transitive surface (module cache only, repo untouched)
go mod download charm.land/huh/v2@v2.0.3 github.com/sahilm/fuzzy@v0.1.3
cat "$(go env GOMODCACHE)/cache/download/charm.land/huh/v2/@v/v2.0.3.mod"
cat "$(go env GOMODCACHE)/cache/download/github.com/sahilm/fuzzy/@v/v0.1.3.mod"
# pure-Go check
grep -rl 'import "C"\|#include' "$(go env GOMODCACHE)/charm.land/huh/v2@v2.0.3/" "$(go env GOMODCACHE)/github.com/sahilm/fuzzy@v0.1.3/" --include='*.go' || echo "no cgo"
# baseline module count
go list -m all | wc -l   # 69 as of 2026-08-27 (fuzzy already appears in the
                         # graph listing though unrequired — a quirk, noted)
```

Expected (verified at detail time; live re-check binds): huh MIT, active Charm repo; v2.0.3 direct requires all at or below yolo's pins (bubbles v2.0.0 ≤ v2.2.1, bubbletea v2.0.2 ≤ v2.0.9, lipgloss v2.0.1 ≤ v2.0.6, x/term v0.2.2 = pin) — MVS keeps yolo's pins; new module lines ≈ 8 (catppuccin/go, x/exp/ordered, x/exp/strings, x/xpty, hashstructure/v2, x/conpty, x/errors, creack/pty) + the two direct requires. fuzzy: MIT (check live — the repo LICENSE is MIT per the API license field), single package, **zero requires**. No cgo in either. Stdlib is inadequate for both (no form-field primitives in stdlib; subsequence fuzzy matching with scoring is a well-trodden single-file algorithm worth taking maintained).

File the dep-proposal bead (child of the slice, S1/S0 precedent), description = the evidence above:

```sh
bd create "dep proposal: add charm.land/huh/v2 v2.0.3 + github.com/sahilm/fuzzy v0.1.3 (S2 dialogs)" --parent=yolo-oae.3 -p 1 --description "<the evidence + transitive surface + checklist verdicts>" --json
```

Then **STOP** and report the evidence to the user; wait for explicit approval. No `go get` before approval.

**Step 2 — (after approval) land the dep.**

```sh
go list -m all | wc -l                    # record the baseline
go get charm.land/huh/v2@v2.0.3 github.com/sahilm/fuzzy@v0.1.3
grep -nE "huh|fuzzy" go.mod               # both direct requires, no // indirect
go build ./...                            # unused requires must not break the build
go list -m all | wc -l                    # record the delta for the report
```

**Step 3 — smoke render (scratch module, yolo's exact pins, never the repo).**
The smoke resolves huh UNDER yolo's charm pins (the real MVS risk — huh's code against bubbles v2.2.1 / bubbletea v2.0.9 / lipgloss v2.0.6):

```sh
mkdir -p /tmp/opencode/huh-smoke && cd /tmp/opencode/huh-smoke
cat > go.mod <<'EOF'
module smoke

go 1.25

require (
	charm.land/bubbles/v2 v2.2.1
	charm.land/bubbletea/v2 v2.0.9
	charm.land/huh/v2 v2.0.3
	charm.land/lipgloss/v2 v2.0.6
	github.com/sahilm/fuzzy v0.1.3
)
EOF
cat > main.go <<'EOF'
package main

import (
	"fmt"

	"charm.land/huh/v2"
	"github.com/sahilm/fuzzy"
)

func main() {
	v := true
	f := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title("smoke").Description("render check").
			Affirmative("Confirm").Negative("Cancel").Value(&v),
	))
	f.WithWidth(60)
	fmt.Println(f.View())
	fmt.Println(fuzzy.Find("qwn", []string{"Claude", "Qwen"}))
}
EOF
go mod tidy && go run .
```

Expected: the rendered confirm (title, description, both pills) and the fuzzy match `[{Index:1 Score:...}]`. Any build failure = the MVS risk materialized → STOP, report, no partial landing.

**Step 4 — gate + allowlist + PROGRESS fact.**
Run `go vet ./... && go test ./...` + `gofmt -l .` (must print nothing) at module root. Append to the root `AGENTS.md` allowlist paragraph (after the glamour entry):

> `charm.land/huh/v2` v2.0.3 (user-approved 2026-08-27, bead `<actual S2.1 bead id>` — huh field dialogs: alert/confirm/input; direct import `charm.land/huh/v2`), `github.com/sahilm/fuzzy` v0.1.3 (same approval — subsequence fuzzy filter for the select/palette; direct import `github.com/sahilm/fuzzy`)

Append the PROGRESS.md fact (one line, the S1.1 pattern): "S2.1 landed huh v2.0.3 + sahilm/fuzzy v0.1.3 (MVS delta `<N>` modules; smoke render green under yolo pins)."

**Step 5 — commit + close the bead.**
`git add go.mod go.sum AGENTS.md docs/superpowers/PROGRESS.md && git commit -m "deps: add huh v2.0.3 + sahilm/fuzzy v0.1.3 (dialogs)"`
`bd close <S2.1 bead> --reason "huh v2.0.3 + fuzzy v0.1.3 landed, smoke green, gate green" --json`

---

### Task S2.2: Modal dialog stack (bead `yolo-oae.3.2`, expected id `yolo-oae.3.5`)

**Files:** modify `internal/tui/dialog.go`, `internal/tui/keys.go`, `internal/tui/view.go`; modify `internal/tui/home.go` (renderClamped split); new `internal/tui/dialog_test.go`.

**Interfaces:** consumes `dialogStack` (App.dlg), `App.theme`, `a.size`, `a.route`, `escBinding`, `a.footerView()`, the session chrome (title/viewport/divider/sessionHelp) and home chrome (logo/New-session/divider/helpText). Produces: `dlgSize` (`dlgMedium`=60, `dlgLarge`=88, `dlgXLarge`=116, `width()`), `dialog{modal bool, size dlgSize, onClose func(*App)}`, `App.pushModal(item dialog, size dlgSize, onClose func(*App))`, `App.replaceModal(...)`, `App.closeTopModal()`, `App.clearModals()`, `modalCanceler` interface (`cancelInner(tea.KeyPressMsg) bool`), `modelDlg.cancelInner` / `agentDlg.cancelInner`, `App.modalInner(*dialog, w, h int) string`, `App.modalChromeMin()`, `App.sessionChrome(w, vh int) string`, `homeModel.renderClamped(s, w, th, maxRows int)`. The existing model/agent/quit/help rendering stays byte-identical (they flip to modal in S2.9/S2.10).

**Upstream parity notes:** `dialog.tsx` — `DialogSize` medium 60 / large 88 / xlarge 116; the overlay is absolute with `paddingTop = floor(H/4)`, the panel `width = size`, `maxWidth = W-2`, `paddingTop = 1`; backdrop `rgba(0,0,0,0.15)` → **deviation 166** (low): plain terminal-background lines (no SGR-alpha dim); `esc` AND `ctrl+c` are both "Close dialog" bindings (dialog.tsx:109-124); the result callback fires on close. Upstream's `onMouseUp` dismiss has no yolo analog (no mouse support — capability note, not a deviation). yolo's quit/help dialogs stay NON-modal append blocks (yolo-specific; upstream has no quit/help dialog — design note).

**Step 1 — write the failing tests.** New `internal/tui/dialog_test.go`:

```go
package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// pushTestModal pushes a modal item carrying an empty (two-pane-era)
// modelDlg payload: its view renders the "Model" title + "  loading…" line,
// enough to pin the overlay frame (S2.9 flips modelDlg to the select and
// re-points these payloads at a fixture catalog).
func pushTestModal(t *testing.T, a *App, size dlgSize, onClose func(*App)) {
	t.Helper()
	a.pushModal(dialog{kind: dlgModel, model: &modelDlg{}}, size, onClose)
}

func TestModalStackOps(t *testing.T) {
	a := testApp()
	closed := []string{}
	a.pushModal(dialog{kind: dlgModel, model: &modelDlg{}}, dlgMedium, func(*App) { closed = append(closed, "first") })
	a.pushModal(dialog{kind: dlgModel, model: &modelDlg{}}, dlgLarge, func(*App) { closed = append(closed, "second") })
	if got := len(a.dlg.items); got != 2 {
		t.Fatalf("stack depth = %d, want 2", got)
	}
	top, _ := a.dlg.top()
	if !top.modal || top.size != dlgLarge {
		t.Fatalf("top = %+v, want modal dlgLarge", top)
	}
	a.closeTopModal()
	if len(a.dlg.items) != 1 || strings.Join(closed, ",") != "second" {
		t.Fatalf("closeTopModal: depth=%d closed=%v, want 1/[second]", len(a.dlg.items), closed)
	}
	a.closeTopModal()
	if len(a.dlg.items) != 0 || strings.Join(closed, ",") != "second,first" {
		t.Fatalf("second close: depth=%d closed=%v", len(a.dlg.items), closed)
	}
	a.pushModal(dialog{kind: dlgModel, model: &modelDlg{}}, dlgMedium, func(*App) { closed = append(closed, "old") })
	a.replaceModal(dialog{kind: dlgModel, model: &modelDlg{}}, dlgMedium, func(*App) { closed = append(closed, "new") })
	if len(a.dlg.items) != 1 {
		t.Fatalf("replaceModal: depth=%d, want 1", len(a.dlg.items))
	}
	if top, _ = a.dlg.top(); top.size != dlgMedium || strings.Join(closed, ",") != "second,first,old" {
		t.Fatalf("replaceModal top/closed = %v/%v", top.size, closed)
	}
	a.pushModal(dialog{kind: dlgModel, model: &modelDlg{}}, dlgMedium, func(*App) { closed = append(closed, "c2") })
	a.clearModals()
	if len(a.dlg.items) != 0 || strings.Join(closed, ",") != "second,first,old,c2" {
		t.Fatalf("clearModals: depth=%d closed=%v", len(a.dlg.items), closed)
	}
	// non-modal items are untouched by the modal ops
	a.dlg.push(dialog{kind: dlgQuit})
	a.clearModals()
	if len(a.dlg.items) != 1 {
		t.Fatalf("clearModals must keep non-modal items: %+v", a.dlg.items)
	}
	if d, _ := a.dlg.top(); d.kind != dlgQuit || d.modal {
		t.Fatalf("survivor = %+v, want non-modal dlgQuit", d)
	}
}

func TestModalEscAndCtrlCCloseTop(t *testing.T) {
	a := testApp()
	closed := 0
	a.pushModal(dialog{kind: dlgModel, model: &modelDlg{}}, dlgMedium, func(*App) { closed++ })
	a.pushModal(dialog{kind: dlgModel, model: &modelDlg{}}, dlgMedium, func(*App) { closed++ })
	a.handleKey(press(tea.KeyEscape))
	if len(a.dlg.items) != 1 || closed != 1 {
		t.Fatalf("esc: depth=%d closed=%d, want 1/1", len(a.dlg.items), closed)
	}
	a.handleKey(ctrlCKey)
	if len(a.dlg.items) != 0 || closed != 2 {
		t.Fatalf("ctrl+c: depth=%d closed=%d, want 0/2", len(a.dlg.items), closed)
	}
}

func TestModalInnerCancelEscFirst(t *testing.T) {
	a := testApp()
	mdl := &modelDlg{hasSubChoice: true}
	a.pushModal(dialog{kind: dlgModel, model: mdl}, dlgMedium, nil)
	a.handleKey(press(tea.KeyEscape))
	if mdl.hasSubChoice {
		t.Fatalf("first esc must close the subchoice")
	}
	if len(a.dlg.items) != 1 {
		t.Fatalf("subchoice esc must keep the dialog: depth=%d, want 1", len(a.dlg.items))
	}
	a.handleKey(press(tea.KeyEscape))
	if len(a.dlg.items) != 0 {
		t.Fatalf("second esc must close the dialog: depth=%d", len(a.dlg.items))
	}
}

func TestModalFrameLayout(t *testing.T) {
	a := testApp()
	a.size = tea.WindowSizeMsg{Width: 80, Height: 24}
	a.route = routeHome
	a.pushModal(dialog{kind: dlgModel, model: &modelDlg{}}, dlgMedium, nil)
	lines := strings.Split(a.view(), "\n")
	if len(lines) != 24 {
		t.Fatalf("frame = %d lines, want 24", len(lines))
	}
	// panel: medium 60, lead (80-60)/2 = 10; home chrome = logo 4 + New 1 +
	// rows 0 + divider 1 + help 1 = 7 > 24/4 = 6 → panelTop = 7; the panel
	// top-padding line → "Model" on line 8, "  loading…" on line 9.
	if want := strings.Repeat(" ", 10) + "Model"; !strings.HasPrefix(lines[8], want) {
		t.Fatalf("line 8 = %q, want prefix %q", stripANSI(lines[8]), want)
	}
	if want := strings.Repeat(" ", 10) + "  loading…"; !strings.HasPrefix(lines[9], want) {
		t.Fatalf("line 9 = %q, want prefix %q", stripANSI(lines[9]), want)
	}
	// the prompt line is suppressed while a modal is open
	if strings.Contains(a.view(), "> ") {
		t.Fatalf("prompt must be hidden under the modal:\n%s", a.view())
	}
	// the footer stays on the last line
	if !strings.Contains(lines[23], "no model") {
		t.Fatalf("footer line = %q, want the home footer", stripANSI(lines[23]))
	}
}

func TestModalFrameSessionClamp(t *testing.T) {
	a := testApp()
	a.size = tea.WindowSizeMsg{Width: 80, Height: 10}
	a.route = routeSession
	a.pushModal(dialog{kind: dlgModel, model: &modelDlg{}}, dlgMedium, nil)
	lines := strings.Split(a.view(), "\n")
	if len(lines) != 10 {
		t.Fatalf("frame = %d lines, want 10", len(lines))
	}
	// session chrome min = title 1 + viewport 1 + divider 1 + help 1 = 4;
	// 10/4 = 2 < 4 → panelTop = 4; the panel starts at line 5 (padding line 4).
	if want := strings.Repeat(" ", 10) + "Model"; !strings.HasPrefix(lines[5], want) {
		t.Fatalf("line 5 = %q, want prefix %q", stripANSI(lines[5]), want)
	}
	if !strings.Contains(lines[0], "session") {
		t.Fatalf("title line = %q, want the session title", stripANSI(lines[0]))
	}
}
```

**Step 2 — run to verify it fails.**
`go test ./internal/tui/ -run 'TestModal' -count=1` → FAIL: compile errors (`undefined: pushModal`, `undefined: dlgMedium`, `dialog` has no field `modal`, etc.) — the modal surface does not exist yet.

**Step 3 — write the minimal implementation.**

`internal/tui/dialog.go` — extend the item type and add the stack ops:

```go
// dlgSize is the modal panel width (upstream DialogSize: medium 60, large
// 88, xlarge 116; clamped to the terminal width by the stack).
type dlgSize int

const (
	dlgMedium dlgSize = iota
	dlgLarge
	dlgXLarge
)

// width is the size's panel width in columns.
func (s dlgSize) width() int {
	switch s {
	case dlgLarge:
		return 88
	case dlgXLarge:
		return 116
	default:
		return 60
	}
}
```

Extend the item (the existing fields stay):

```go
type dialog struct {
	kind    dialogKind
	model   *modelDlg
	agent   *agentDlg
	modal   bool        // true: rendered as the overlay frame (S2.2)
	size    dlgSize     // the panel width, modal only
	onClose func(*App)  // the stack-pop callback (upstream result callback)
}
```

Add `modalCanceler` + the esc vetoes (after the `dialogStack` methods):

```go
// modalCanceler is a modal payload that consumes esc for its own inner state
// before the stack closes it (upstream: the model/agent subchoice).
type modalCanceler interface {
	cancelInner(tea.KeyPressMsg) bool
}

// cancelInner consumes esc while the subchoice overlay is open (the next esc
// closes the dialog).
func (m *modelDlg) cancelInner(tea.KeyPressMsg) bool {
	if m.hasSubChoice {
		m.hasSubChoice = false
		return true
	}
	return false
}

// cancelInner is the agentDlg twin.
func (m *agentDlg) cancelInner(tea.KeyPressMsg) bool {
	if m.hasSubChoice {
		m.hasSubChoice = false
		return true
	}
	return false
}

// dialogCanceler is the payload's esc veto, if it has one.
func dialogCanceler(d dialog) (modalCanceler, bool) {
	switch {
	case d.model != nil:
		return d.model, true
	case d.agent != nil:
		return d.agent, true
	}
	return nil, false
}
```

Add the stack ops (after `push`/`pop`/`top`):

```go
// pushModal pushes a modal item (upstream <Dialog> push): it owns the keys
// until esc/ctrl+c or its own completion closes it; onClose fires when the
// stack pops it.
func (a *App) pushModal(item dialog, size dlgSize, onClose func(*App)) {
	item.modal = true
	item.size = size
	item.onClose = onClose
	a.dlg.push(item)
}

// closeTopModal pops the top modal and fires its onClose (esc/ctrl+c).
func (a *App) closeTopModal() {
	d, ok := a.dlg.top()
	if !ok || !d.modal {
		return
	}
	a.dlg.pop()
	if d.onClose != nil {
		d.onClose(a)
	}
}

// replaceModal closes the top modal (firing its onClose) and pushes the
// replacement (upstream DialogProvider replace).
func (a *App) replaceModal(item dialog, size dlgSize, onClose func(*App)) {
	a.closeTopModal()
	a.pushModal(item, size, onClose)
}

// clearModals closes every modal top-down (non-modal items stop the walk).
func (a *App) clearModals() {
	for {
		d, ok := a.dlg.top()
		if !ok || !d.modal {
			return
		}
		a.dlg.pop()
		if d.onClose != nil {
			d.onClose(a)
		}
	}
}
```

Add `modalInner` (the payload content for the frame; S2.3/S2.5 grow the switch):

```go
// modalInner renders the top modal's payload content at the panel width
// (the stack draws the panel chrome; the payload supplies the inner lines).
func (a *App) modalInner(d *dialog, w, h int) string {
	switch d.kind {
	case dlgModel:
		if d.model != nil {
			return d.model.view(&a.store, w, a.theme)
		}
	case dlgAgents:
		if d.agent != nil {
			return d.agent.view(&a.store, w, a.theme)
		}
	}
	return ""
}
```

`internal/tui/keys.go` — the modal branch at the top of `handleDialogKey` + the ctrl+c binding:

```go
var dlgCtrlC = key.NewBinding(key.WithKeys("ctrl+c"))

func (a *App) handleDialogKey(d dialog, k tea.KeyPressMsg) []tea.Cmd {
	if d.modal && (key.Matches(k, escBinding) || key.Matches(k, dlgCtrlC)) {
		if c, ok := dialogCanceler(d); ok && c.cancelInner(k) {
			return nil
		}
		a.closeTopModal()
		return nil
	}
	// …the existing per-kind switch, unchanged…
}
```

`internal/tui/view.go` — the modal branch + the frame renderer:

```go
func (a *App) view() string {
	if d, ok := a.dlg.top(); ok && d.modal {
		return a.viewModal()
	}
	// …the existing composition, unchanged…
}

// modalChromeMin is the route chrome's minimum line count (the panel top
// never climbs above it): session = title + 1 viewport + divider + help,
// home = logo + new-session + divider + help.
func (a *App) modalChromeMin() int {
	switch a.route {
	case routeSession:
		return 1 + 1 + 1 + len(strings.Split(wrapLine(sessionHelp, a.termWidth()), "\n"))
	default:
		return 4 + 1 + 1 + len(strings.Split(wrapLine(helpText, a.termWidth()), "\n"))
	}
}

// sessionChrome renders the session route's chrome for a viewport of vh
// lines: title, transcript viewport, divider, the (possibly wrapped) help.
func (a *App) sessionChrome(w, vh int) string {
	if vh < 1 {
		vh = 1
	}
	a.sess.sync(&a.store, w, vh, a.theme, a.spinFrame())
	t := "session"
	if a.store.Current != nil {
		t = a.store.Current.Title
	}
	var b strings.Builder
	b.WriteString(title.Render(t) +
		"\n" + a.sess.vm.View() +
		"\n" + dividerLineRendered)
	for _, l := range strings.Split(wrapLine(sessionHelp, w), "\n") {
		b.WriteString("\n" + a.theme.TextMuted().Render(l))
	}
	return b.String()
}

// viewModal renders the modal frame (port of dialog.tsx): the route chrome
// clamped to the panel top, plain blank backdrop lines (deviation 166 —
// the upstream rgba(0,0,0,0.15) dim has no SGR equivalent), the centered
// panel (backgroundPanel fill, width min(size, w-2), top padding 1, top at
// max(h/4, chromeMin)) and the footer on the last line. Prompt, menu,
// toasts and lastErr are suppressed while a modal is open.
func (a *App) viewModal() string {
	w, h := a.size.Width, a.size.Height
	if w < 1 {
		w = 80
	}
	if h < 1 {
		h = 24
	}
	d, _ := a.dlg.top()
	panelW := int(d.size.width())
	if panelW > w-2 {
		panelW = w - 2
	}
	innerLines := strings.Split(a.modalInner(&d, panelW, h), "\n")
	panelTop := max(h/4, a.modalChromeMin())
	avail := h - panelTop - 1 // the footer line
	if avail < 1 {
		avail = 1
	}
	n := min(len(innerLines)+1, avail) // +1: the panel top-padding line
	var chrome string
	switch a.route {
	case routeSession:
		help := len(strings.Split(wrapLine(sessionHelp, w), "\n"))
		chrome = a.sessionChrome(w, panelTop-1-1-help)
	default:
		help := len(strings.Split(wrapLine(helpText, w), "\n"))
		chrome = a.home.renderClamped(&a.store, w, a.theme, panelTop-4-1-1-help)
	}
	chromeLines := strings.Split(chrome, "\n")
	for len(chromeLines) < panelTop {
		chromeLines = append(chromeLines, "")
	}
	if len(chromeLines) > panelTop {
		chromeLines = chromeLines[:panelTop]
	}
	bg := a.theme.BackgroundPanel().Width(panelW)
	panel := []string{bg.Render("")}
	for i := 0; i < n-1 && i < len(innerLines); i++ {
		panel = append(panel, bg.Render(innerLines[i]))
	}
	lead := strings.Repeat(" ", (w-panelW)/2)
	var b strings.Builder
	write := func(l string) {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(l)
	}
	for _, l := range chromeLines {
		write(l)
	}
	for _, l := range panel {
		write(lead + l)
	}
	for i := panelTop + len(panel); i < h-1; i++ {
		write("")
	}
	write(a.footerView())
	return b.String()
}
```

Refactor `viewSession` to build on `sessionChrome` (same output — the existing session tests must stay green):

```go
func (a *App) viewSession(menu, perm, toasts, dlg string) string {
	w := a.size.Width
	if w < 1 {
		w = 80
	}
	overlays := 0
	for _, v := range []string{perm, toasts, dlg} {
		if v != "" {
			overlays += 1 + strings.Count(v, "\n")
		}
	}
	if a.lastErr != "" {
		overlays++
	}
	menuLines := 0
	if menu != "" {
		menuLines = 1 + strings.Count(menu, "\n")
	}
	help := len(strings.Split(wrapLine(sessionHelp, w), "\n"))
	vh := a.size.Height - 1 - 1 - help - 1 - 1 - menuLines - overlays
	return a.sessionChrome(w, vh)
}
```

`internal/tui/home.go` — the renderClamped split (render keeps its exact output):

```go
func (h *homeModel) render(s *store.State, w int, th theme.Theme) string {
	return h.renderClamped(s, w, th, -1)
}

// renderClamped is render with the recent-session row count capped (maxRows
// -1 = all; the modal stack, S2.2, clamps the chrome so the panel fits).
func (h *homeModel) renderClamped(s *store.State, w int, th theme.Theme, maxRows int) string {
	h.clampCursor(s)
	rows := h.visible(s)
	if maxRows >= 0 && len(rows) > maxRows {
		rows = rows[:maxRows]
	}
	var b strings.Builder
	b.WriteString(renderLogo(th))
	b.WriteByte('\n')
	b.WriteString(h.renderRow(0, "New session", "", w, th))
	b.WriteByte('\n')
	for i, se := range rows {
		t, meta := lineParts(se, h.now())
		b.WriteString(h.renderRow(i+1, t, meta, w, th))
		b.WriteByte('\n')
	}
	b.WriteString(th.BorderSubtle().Render(dividerLine()))
	b.WriteByte('\n')
	b.WriteString(dimWrapped(th, helpText, w))
	return b.String()
}
```

**Step 4 — run to verify it passes, then gate.**
`go test ./internal/tui/ -run 'TestModal' -count=1` → PASS. Then the full gate at module root: `go vet ./... && go test ./...` (all pre-existing suites green — the session/home/quit/help paths are unchanged) + `gofmt -l .` prints nothing.

**Step 5 — commit + close the bead.**
`git add internal/tui/dialog.go internal/tui/keys.go internal/tui/view.go internal/tui/home.go internal/tui/dialog_test.go && git commit -m "feat: modal dialog stack (overlay, focus, esc, stack)"`
`bd close <S2.2 bead> --reason "modal stack green: ops, esc/ctrl+c, inner-cancel, frame layout" --json`
Log **deviation 166** (low) in `docs/superpowers/DEVIATIONS.md` (same commit): modal backdrop = blank terminal-background lines; the upstream `rgba(0,0,0,0.15)` dim has no SGR-alpha equivalent.

---

### Task S2.3: huh field dialogs — alert + confirm (bead `yolo-oae.3.3`, expected id `yolo-oae.3.6`)

**Files:** new `internal/tui/huhdlg.go`; modify `internal/tui/dialog.go` (`dlgForm` kind + `form *huhFormDlg` field + `modalInner`/`handleDialogKey` dispatch + `openFormModal`); new `internal/tui/huhdlg_test.go`.

**Interfaces:** consumes S2.2 (the modal stack, esc/ctrl+c close, `pushModal`), the resolved theme (token accessors), huh v2.0.3 (S2.1). Produces: `huhFormDlg` (the form payload: `form *huh.Form`, `onConfirm func(*App, *huh.Form)`), `App.openFormModal(form *huh.Form, size dlgSize, onConfirm func(*App, *huh.Form), onClose func(*App)) []tea.Cmd`, `themeDialog(th theme.Theme) huh.Theme`, `buildAlertForm(th theme.Theme, title, description string) *huh.Form`, `buildConfirmForm(th theme.Theme, title, description string) *huh.Form`. **No production call site yet** — the first land with S2.8–S2.10 and S3 (unit surface only; the rendered-form SGR goldens land then).

**Upstream parity notes:** `dialog-alert.tsx` = title + message + a single "ok" button (return confirms); `dialog-confirm.tsx` = title + message + confirm/cancel pills starting on confirm (left/right/return). yolo ports both as one-group `huh.Form`s: confirm field with `Affirmative("Confirm")`/`Negative("Cancel")` and `Value(&true)` (upstream's default-active pill = confirm), alert = the same confirm field with `Affirmative("ok")` and `Negative("")` (huh renders the lone affirmative button and disables the reject keys — the port of the single ok pill). **Deviation 167** (low): the huh field look ≠ the upstream borderless pills (huh's structural affordances — its own keymap, button padding — cannot be fully suppressed; the theme strips the default thick-left border and maps the tokens as close as huh allows).

**Step 1 — write the failing tests.** New `internal/tui/huhdlg_test.go`:

```go
package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"charm.land/huh/v2"
)

var enterKey = tea.KeyPressMsg{Code: tea.KeyEnter}

func TestConfirmFormSubmit(t *testing.T) {
	a := testApp()
	confirmed := false
	a.openFormModal(buildConfirmForm(a.theme, "do it?", "the thing"), dlgMedium,
		func(*App, *huh.Form) { confirmed = true }, nil)
	a.handleKey(enterKey)
	if !confirmed || len(a.dlg.items) != 0 {
		t.Fatalf("confirmed=%v depth=%d, want true/0", confirmed, len(a.dlg.items))
	}
}

func TestConfirmFormEscCancels(t *testing.T) {
	a := testApp()
	confirmed, cancelled := false, false
	a.openFormModal(buildConfirmForm(a.theme, "t", "d"), dlgMedium,
		func(*App, *huh.Form) { confirmed = true },
		func(*App) { cancelled = true })
	a.handleKey(press(tea.KeyEscape))
	if confirmed || !cancelled || len(a.dlg.items) != 0 {
		t.Fatalf("confirmed=%v cancelled=%v depth=%d, want false/true/0",
			confirmed, cancelled, len(a.dlg.items))
	}
}

func TestConfirmFormKeysToggle(t *testing.T) {
	a := testApp()
	var got bool
	a.openFormModal(buildConfirmForm(a.theme, "t", "d"), dlgMedium,
		func(_ *App, f *huh.Form) { got = f.GetBool("confirm") }, nil)
	// the pills start on "confirm" (true); left toggles to "cancel"
	a.handleKey(tea.KeyPressMsg{Code: tea.KeyLeft})
	a.handleKey(enterKey)
	if got {
		t.Fatalf("left must move the selection to cancel (false)")
	}
}

func TestAlertFormSingleButton(t *testing.T) {
	f := buildAlertForm(theme.Theme{}, "heads up", "something happened")
	v := strings.ToLower(f.View())
	if !strings.Contains(v, "ok") {
		t.Fatalf("alert view missing the ok button: %q", v)
	}
	if strings.Contains(v, "cancel") || strings.Contains(v, "confirm") {
		t.Fatalf("alert view leaked a second button: %q", v)
	}
}
```

**Step 2 — run to verify it fails.**
`go test ./internal/tui/ -run 'Test(Confirm|Alert)Form' -count=1` → FAIL: compile errors (`undefined: buildConfirmForm`, `undefined: openFormModal`, `undefined: dlgForm`).

**Step 3 — write the minimal implementation.** New `internal/tui/huhdlg.go`:

```go
package tui

import (
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/tui/theme"
)

// huhFormDlg is the huh-form payload of a modal field dialog (S2.3): the
// form is a child tea.Model — the stack forwards every non-esc key, watches
// f.State, and fires onConfirm on completion (esc/ctrl+c close via the
// stack, firing the dialog's onClose).
type huhFormDlg struct {
	form      *huh.Form
	onConfirm func(*App, *huh.Form)
}

// handleKey forwards the key to the form and completes the dialog on the
// form's State transitions (upstream: submit returns the form's value).
// Note: huh's Model is a v1-style interface — Form.Update returns (Model,
// tea.Cmd), and *Form.Update always returns itself; assert the concrete
// type back onto f.form.
func (f *huhFormDlg) handleKey(a *App, k tea.KeyPressMsg) []tea.Cmd {
	m, cmd := f.form.Update(k)
	if fm, ok := m.(*huh.Form); ok {
		f.form = fm
	}
	switch f.form.State {
	case huh.StateCompleted:
		done := f.onConfirm
		f.onConfirm = nil
		a.closeTopModal()
		if done != nil {
			done(a, f.form)
		}
		return nil
	case huh.StateAborted:
		a.closeTopModal()
		return nil
	}
	if cmd != nil {
		return []tea.Cmd{cmd}
	}
	return nil
}

// openFormModal pushes a huh-form modal: the form sizes itself on the last
// terminal size, its Init cmd seeds the fields, onConfirm fires on submit
// and onClose on esc/ctrl+c.
func (a *App) openFormModal(form *huh.Form, size dlgSize, onConfirm func(*App, *huh.Form), onClose func(*App)) []tea.Cmd {
	f := &huhFormDlg{form: form}
	a.pushModal(dialog{kind: dlgForm, form: f}, size, onClose)
	cmds := a.emit(form.Init())
	if a.size.Width > 0 {
		if m, cc := f.form.Update(tea.WindowSizeMsg{Width: a.size.Width, Height: a.size.Height}); cc != nil {
			if fm, ok := m.(*huh.Form); ok {
				f.form = fm
			}
			cmds = append(cmds, cc)
		}
	}
	return cmds
}

// buildAlertForm is the upstream dialog-alert (a single ok pill).
func buildAlertForm(th theme.Theme, title, description string) *huh.Form {
	v := true
	return huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Key("confirm").
			Title(title).Description(description).
			Affirmative("ok").Negative("").Value(&v),
	)).WithTheme(themeDialog(th)).WithShowHelp(false)
}

// buildConfirmForm is the upstream dialog-confirm (confirm/cancel pills,
// starting on confirm).
func buildConfirmForm(th theme.Theme, title, description string) *huh.Form {
	v := true
	return huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Key("confirm").
			Title(title).Description(description).
			Affirmative("Confirm").Negative("Cancel").Value(&v),
	)).WithTheme(themeDialog(th)).WithShowHelp(false)
}

// themeDialog maps the resolved theme to huh's Styles (deviation 167 —
// borderless field boxes, the upstream dialog look; tokens from the
// resolved theme; a zero Theme degrades to huh's default palette).
func themeDialog(th theme.Theme) huh.Theme {
	return huh.ThemeFunc(func(isDark bool) *huh.Styles {
		s := huh.ThemeBase(isDark)
		s.Focused.Base = lipgloss.NewStyle() // strip the default thick-left border
		s.Blurred.Base = lipgloss.NewStyle()
		s.FieldSeparator = lipgloss.NewStyle().SetString("\n")
		if c, ok := th.Color("text"); ok {
			fg := lipgloss.Color(c.Hex()[:7])
			s.Focused.Title = lipgloss.NewStyle().Foreground(fg).Bold(true)
			s.Blurred.Title = lipgloss.NewStyle().Foreground(fg)
		}
		if c, ok := th.Color("textMuted"); ok {
			muted := lipgloss.Color(c.Hex()[:7])
			s.Focused.Description = lipgloss.NewStyle().Foreground(muted)
			s.Blurred.Description = lipgloss.NewStyle().Foreground(muted)
			s.Group.Description = s.Blurred.Description
		}
		if c, ok := th.Color("primary"); ok {
			bg := lipgloss.Color(c.Hex()[:7])
			sel := th.SelectedForeground(c)
			fg := lipgloss.Color(sel.Hex()[:7])
			s.Focused.FocusedButton = lipgloss.NewStyle().
				Padding(0, 2).MarginRight(1).
				Foreground(fg).Background(bg).Bold(true)
		}
		if c, ok := th.Color("textMuted"); ok {
			s.Focused.BlurredButton = lipgloss.NewStyle().
				Padding(0, 2).MarginRight(1).
				Foreground(lipgloss.Color(c.Hex()[:7]))
		}
		return s
	})
}
```

`internal/tui/dialog.go` — the kind + field + dispatch:

```go
const (
	dlgNone dialogKind = iota
	dlgQuit
	dlgHelp
	dlgModel
	dlgAgents
	dlgForm
)

type dialog struct {
	kind    dialogKind
	model   *modelDlg
	agent   *agentDlg
	form    *huhFormDlg // dlgForm payload (S2.3)
	modal   bool
	size    dlgSize
	onClose func(*App)
}
```

In `modalInner`, add:

```go
	case dlgForm:
		if d.form != nil {
			return d.form.form.View()
		}
```

In `handleDialogKey`'s per-kind switch, add:

```go
	case dlgForm:
		if d.form == nil {
			a.dlg.pop()
			return nil
		}
		return d.form.handleKey(a, k)
```

Note: the esc/ctrl+c modal branch (S2.2) stays ABOVE this switch — esc closes the form dialog via the stack (firing onClose) without consulting the form; that is the cancel path.

**Step 4 — run to verify it passes, then gate.**
`go test ./internal/tui/ -run 'Test(Confirm|Alert)Form' -count=1` → PASS. Gate: `go vet ./... && go test ./...` + `gofmt -l .` empty. (huh is now imported — the S2.1 require stays put; do NOT tidy, fuzzy is still unimported.)

**Step 5 — commit + close the bead.**
`git add internal/tui/huhdlg.go internal/tui/dialog.go internal/tui/huhdlg_test.go && git commit -m "feat: huh field dialogs - alert + confirm (themed)"`
`bd close <S2.3 bead> --reason "alert + confirm forms green (submit, esc-cancel, toggle, single-button)" --json`
Log **deviation 167** (low) in `docs/superpowers/DEVIATIONS.md` (same commit): the huh field dialogs deviate in look from the upstream borderless pills (huh's structural affordances; themed borderless as close as huh allows).

---

### Task S2.4: huh field — themed input (bead `yolo-oae.3.4`, expected id `yolo-oae.3.7`)

**Files:** modify `internal/tui/huhdlg.go` (buildInputForm); modify `internal/tui/huhdlg_test.go` (the input tests).

**Interfaces:** consumes S2.3 (openFormModal, themeDialog, huhFormDlg). Produces: `buildInputForm(th theme.Theme, title, description, placeholder, initial string) *huh.Form` — the value reads back with `f.GetString("value")`. No production call site yet (S3's rename/status/theme prompts).

**Upstream parity notes:** `dialog-prompt.tsx` = title + description + a single text input (placeholder + initial value), return submits the typed value, esc cancels. The yolo port is the one-group form with the Input field, `Prompt` unset (huh's `">"` prompt stays — part of deviation 167's look note).

**Step 1 — write the failing tests.** Append to `internal/tui/huhdlg_test.go`:

```go
func TestInputFormTypedSubmit(t *testing.T) {
	a := testApp()
	var got string
	a.openFormModal(buildInputForm(a.theme, "rename", "the session", "session name", "old"), dlgMedium,
		func(_ *App, f *huh.Form) { got = f.GetString("value") }, nil)
	a.handleKey(press('h'))
	a.handleKey(press('i'))
	a.handleKey(enterKey)
	if got != "oldhi" || len(a.dlg.items) != 0 {
		t.Fatalf("got=%q depth=%d, want oldhi/0", got, len(a.dlg.items))
	}
}

func TestInputFormEscCancels(t *testing.T) {
	a := testApp()
	cancelled := false
	a.openFormModal(buildInputForm(a.theme, "t", "d", "ph", "x"), dlgMedium,
		func(*App, *huh.Form) { t.Fatalf("confirm must not fire on esc") },
		func(*App) { cancelled = true })
	a.handleKey(press(tea.KeyEscape))
	if !cancelled || len(a.dlg.items) != 0 {
		t.Fatalf("cancelled=%v depth=%d, want true/0", cancelled, len(a.dlg.items))
	}
}

func TestInputFormPlaceholder(t *testing.T) {
	v := buildInputForm(theme.Theme{}, "t", "d", "session name", "").View()
	if !strings.Contains(v, "session name") {
		t.Fatalf("placeholder missing from the view: %q", v)
	}
}
```

**Step 2 — run to verify it fails.**
`go test ./internal/tui/ -run 'TestInputForm' -count=1` → FAIL: compile error (`undefined: buildInputForm`).

**Step 3 — write the minimal implementation.** Append to `internal/tui/huhdlg.go`:

```go
// buildInputForm is the upstream dialog-prompt (a single text input with
// placeholder + initial value; return submits, esc cancels).
func buildInputForm(th theme.Theme, title, description, placeholder, initial string) *huh.Form {
	v := initial
	return huh.NewForm(huh.NewGroup(
		huh.NewInput().Key("value").
			Title(title).Description(description).
			Placeholder(placeholder).Value(&v),
	)).WithTheme(themeDialog(th)).WithShowHelp(false)
}
```

**Step 4 — run to verify it passes, then gate.**
`go test ./internal/tui/ -run 'TestInputForm' -count=1` → PASS. Gate: `go vet ./... && go test ./...` + `gofmt -l .` empty.

**Step 5 — commit + close the bead.**
`git add internal/tui/huhdlg.go internal/tui/huhdlg_test.go && git commit -m "feat: huh field - themed input (rename/prompt)"`
`bd close <S2.4 bead> --reason "input form green (typed submit, esc-cancel, placeholder)" --json`

---

### Task S2.5: Select core — options, navigation, fuzzy filter (bead `yolo-oae.3.5`, expected id `yolo-oae.3.8`)

**Files:** new `internal/tui/select.go`; modify `internal/tui/dialog.go` (`dlgModel`/`dlgAgents` dispatch gains the `select` payload); new `internal/tui/select_test.go`.

**Interfaces:** consumes S2.2 (the modal stack), S2.1 (sahilm/fuzzy), the S0.9 home SELECT token chain (`th.Color("primary")` + `th.SelectedForeground()` + lipgloss paint — see `writeRowLine`). Produces: `selectOption{title, description, footer, category, value any, disabled bool}`, `selectModel` (the DialogSelect state machine: `title`, `placeholder`, `options`, `isCurrent func(selectOption) bool`, `onSelect func(*App, selectOption)`, `onMove func(selectOption)`, `sel`, `top`, `filter`, `input textinput.Model`), `selectNew(title, placeholder string, options []selectOption, isCurrent, onSelect, onMove) *selectModel`, `selectModel.filtered() []selectOption`, `selectModel.handleKey(a *App, k tea.KeyPressMsg) []tea.Cmd`, `selectModel.view(w, h int, th theme.Theme) string`. `dialog` gains `select *selectModel` (the S2.9/S2.10 payloads; no production opener until then).

**Upstream parity notes:** `dialog-select.tsx` — the filtered list excludes `disabled` options entirely (the `filter` memo); the fuzzy filter ports `fuzzysort.go(keywords, {keys:["title","category"], scoreFn: r => r[0].score*2 + r[1].score})` to two `fuzzy.Find` calls (titles ×2 + categories ×1, summed per index, stable sort by score desc); navigation wraps (up/down), home/end jump, enter selects; a non-empty needle resets the selection to the top; the active row is the full-row selection paint (dialog-select.tsx:667-678 + Option 732-791) reusing the S0.9 token chain; the current option carries the `●` gutter in primary (non-active rows); the visible window is `min(len, floor(h/2)-6)` rows (upstream `height` memo).

**Step 1 — write the failing tests.** New `internal/tui/select_test.go`:

```go
package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func selTestOptions() []selectOption {
	return []selectOption{
		{title: "Alpha", description: "first", category: "Group A"},
		{title: "Beta", description: "second", category: "Group A"},
		{title: "Gamma", description: "third", category: "Group B"},
		{title: "Broken", disabled: true},
	}
}

// pushSelectModal pushes a select as the top modal (the production openers
// land in S2.9/S2.10; the stack contract is S2.2's).
func pushSelectModal(t *testing.T, a *App, m *selectModel) {
	t.Helper()
	a.pushModal(dialog{kind: dlgModel, select: m}, dlgMedium, nil)
}

func TestSelectNavigationWrap(t *testing.T) {
	a := testApp()
	moved := []string{}
	m := selectNew("Test", "Search", selTestOptions(), nil,
		func(*App, selectOption) {},
		func(o selectOption) { moved = append(moved, o.title) })
	pushSelectModal(t, a, m)
	a.handleKey(downKey)
	a.handleKey(downKey)
	a.handleKey(downKey) // wraps to 0 (3 enabled options — "Broken" is disabled)
	a.handleKey(upKey)   // wraps to 2
	if m.sel != 2 {
		t.Fatalf("sel = %d, want 2 (wrap)", m.sel)
	}
	if strings.Join(moved, ",") != "Beta,Gamma,Alpha,Gamma" {
		t.Fatalf("onMove = %v", moved)
	}
}

func TestSelectEnterAndJump(t *testing.T) {
	a := testApp()
	var picked selectOption
	m := selectNew("Test", "Search", selTestOptions(), nil,
		func(_ *App, o selectOption) { picked = o }, nil)
	pushSelectModal(t, a, m)
	a.handleKey(homeKeyTest)
	a.handleKey(enterKey)
	if picked.title != "Alpha" {
		t.Fatalf("picked = %q, want Alpha", picked.title)
	}
	a.handleKey(endKey)
	a.handleKey(enterKey)
	if picked.title != "Gamma" {
		t.Fatalf("picked = %q, want Gamma (end)", picked.title)
	}
}

func TestSelectFuzzyFilter(t *testing.T) {
	a := testApp()
	var picked selectOption
	m := selectNew("Test", "Search", selTestOptions(), nil,
		func(_ *App, o selectOption) { picked = o }, nil)
	pushSelectModal(t, a, m)
	a.handleKey(press('g')) // only Gamma matches
	if len(m.filtered()) != 1 || m.filtered()[0].title != "Gamma" {
		t.Fatalf("filtered = %v, want [Gamma]", m.filtered())
	}
	a.handleKey(enterKey)
	if picked.title != "Gamma" {
		t.Fatalf("picked = %q, want Gamma", picked.title)
	}
}

func TestSelectFuzzyWeighting(t *testing.T) {
	// a title hit (×2) must outrank a category hit (×1) on the same needle
	opts := []selectOption{
		{title: "Quiet", category: ""},
		{title: "Other", category: "quiet group"},
	}
	m := selectNew("T", "S", opts, nil, nil, nil)
	l := m.filtered()
	_ = l
	m.filter = "quiet"
	l = m.filtered()
	if len(l) != 2 || l[0].title != "Quiet" {
		t.Fatalf("weighted order = %v, want [Quiet Other]", titlesOf(l))
	}
}

func TestSelectViewLayout(t *testing.T) {
	a := testApp()
	m := selectNew("Test", "Search", selTestOptions(), nil, nil, nil)
	out := strings.Split(m.view(60, 24, a.theme), "\n")
	if !strings.Contains(out[0], "Test") {
		t.Fatalf("title row = %q", out[0])
	}
	if !strings.Contains(out[1], "Search") {
		t.Fatalf("filter row = %q, want the placeholder", out[1])
	}
	if !strings.Contains(out[2], "Alpha") || !strings.Contains(out[3], "Beta") || !strings.Contains(out[4], "Gamma") {
		t.Fatalf("option rows missing: %q", out[2:5])
	}
	if !strings.Contains(out[len(out)-1], "esc close") {
		t.Fatalf("hint row = %q", out[len(out)-1])
	}
	// a no-match needle renders the empty state
	m.filter = "zzz"
	out = strings.Split(m.view(60, 24, a.theme), "\n")
	if !strings.Contains(out[len(out)-1], "No results found") {
		t.Fatalf("empty state missing: %q", out)
	}
}

func titlesOf(l []selectOption) []string {
	out := make([]string, len(l))
	for i, o := range l {
		out[i] = o.title
	}
	return out
}

var (
	downKey     = tea.KeyPressMsg{Code: tea.KeyDown}
	upKey       = tea.KeyPressMsg{Code: tea.KeyUp}
	homeKeyTest = tea.KeyPressMsg{Code: tea.KeyHome}
	endKey      = tea.KeyPressMsg{Code: tea.KeyEnd}
)
```

**Step 2 — run to verify it fails.**
`go test ./internal/tui/ -run 'TestSelect' -count=1` → FAIL: compile errors (`undefined: selectNew`, `undefined: selectOption`, `dialog` has no field `select`).

**Step 3 — write the minimal implementation.** New `internal/tui/select.go`:

```go
// select.go — the port of upstream DialogSelect (dialog-select.tsx), the
// shared list primitive behind the modal dialogs (S2.9 model, S2.10 agent,
// the S3 dialogs). S2.5 lands the option list + navigation + the fuzzy
// filter; S2.6 adds categories, details and the footer tail; S2.7 adds
// actions, the footer hints and the scroll acceleration.

package tui

import (
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sahilm/fuzzy"

	"github.com/kido5217/yolo/internal/tui/theme"
)

// selectOption is one selectable row (port of DialogSelectOption; the JSX
// rendering fields have no Go analog — the row render lives in selectModel).
type selectOption struct {
	title       string
	description string
	footer      string
	category    string
	value       any
	disabled    bool // excluded from the filtered list entirely (upstream)
}

// selectModel is the DialogSelect state machine.
type selectModel struct {
	title       string
	placeholder string
	options     []selectOption
	isCurrent   func(selectOption) bool
	onSelect    func(*App, selectOption)
	onMove      func(selectOption)
	sel         int
	top         int // first visible row (S2.6 counts rendered rows)
	filter      string
	input       textinput.Model
}

// selectNew builds a select (isCurrent/onMove/onSelect may be nil).
func selectNew(title, placeholder string, options []selectOption,
	isCurrent func(selectOption) bool, onSelect func(*App, selectOption),
	onMove func(selectOption)) *selectModel {
	m := &selectModel{
		title:       title,
		placeholder: placeholder,
		options:     options,
		isCurrent:   isCurrent,
		onSelect:    onSelect,
		onMove:      onMove,
	}
	m.input = textinput.New()
	m.input.Prompt = ""
	m.input.Placeholder = placeholder
	m.input.SetWidth(40)
	return m
}

// filtered is the live list (upstream `filtered` memo): disabled options are
// excluded entirely; an empty needle returns the rest in order, otherwise
// the fuzzy hits sorted by the weighted score (title ×2, category ×1 — the
// port of the fuzzysort keys/scoreFn, dialog-select.tsx:154-173).
func (m *selectModel) filtered() []selectOption {
	enabled := make([]selectOption, 0, len(m.options))
	for _, o := range m.options {
		if !o.disabled {
			enabled = append(enabled, o)
		}
	}
	if m.filter == "" {
		return enabled
	}
	n := len(enabled)
	titles := make([]string, n)
	cats := make([]string, n)
	for i, o := range enabled {
		titles[i] = o.title
		cats[i] = o.category
	}
	score := make([]int, n)
	for _, hit := range fuzzy.Find(m.filter, titles) {
		score[hit.Index] += 2 * hit.Score
	}
	for _, hit := range fuzzy.Find(m.filter, cats) {
		score[hit.Index] += hit.Score
	}
	type scored struct {
		opt selectOption
		s   int
	}
	var hits []scored
	for i, o := range enabled {
		if score[i] > 0 {
			hits = append(hits, scored{o, score[i]})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].s > hits[j].s })
	out := make([]selectOption, len(hits))
	for i, h := range hits {
		out[i] = h.opt
	}
	return out
}

// syncFilter reads the filter input and resets the selection when the needle
// becomes non-empty (upstream: filter>0 → moveTo(0)).
func (m *selectModel) syncFilter() {
	if f := m.input.Value(); f != m.filter {
		m.filter = f
		if f != "" {
			m.sel = 0
		}
	}
}

// handleKey drives the select while the modal stack owns the frame: arrows
// move with wraparound, home/end jump, enter submits the selection; every
// other key feeds the fuzzy filter input (esc/ctrl+c are consumed by the
// stack first — S2.2).
func (m *selectModel) handleKey(a *App, k tea.KeyPressMsg) []tea.Cmd {
	switch {
	case key.Matches(k, homeKeyMap.Up):
		m.move(-1)
	case key.Matches(k, homeKeyMap.Down):
		m.move(1)
	case key.Matches(k, selHomeKey):
		m.jump(0)
	case key.Matches(k, selEndKey):
		m.jump(-1)
	case key.Matches(k, homeKeyMap.Enter):
		m.submit(a)
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(k)
		m.syncFilter()
		if cmd != nil {
			return []tea.Cmd{cmd}
		}
		return nil
	}
	return nil
}

var (
	selHomeKey = key.NewBinding(key.WithKeys("home"))
	selEndKey  = key.NewBinding(key.WithKeys("end"))
)

// move steps the selection with wraparound (upstream move, 290-297).
func (m *selectModel) move(d int) {
	l := m.filtered()
	if len(l) == 0 {
		return
	}
	m.sel = ((m.sel + d) % len(l) + len(l)) % len(l)
	if m.onMove != nil {
		m.onMove(l[m.sel])
	}
}

// jump lands the selection at the start (0) or end (-1) of the list.
func (m *selectModel) jump(i int) {
	l := m.filtered()
	if len(l) == 0 {
		return
	}
	if i < 0 {
		i = len(l) - 1
	}
	m.sel = i
	if m.onMove != nil {
		m.onMove(l[m.sel])
	}
}

// submit fires onSelect on the selected option (upstream select).
func (m *selectModel) submit(a *App) {
	l := m.filtered()
	if len(l) == 0 || m.onSelect == nil {
		return
	}
	m.onSelect(a, l[m.sel])
}

// view renders the select's inner lines (the modal stack draws the panel
// chrome — S2.2): the title row, the filter input row, the visible option
// window (upstream height = min(rows, floor(h/2)-6)) and the keymap hint row.
func (m *selectModel) view(w, h int, th theme.Theme) string {
	m.syncFilter()
	l := m.filtered()
	visible := h/2 - 6
	if visible < 1 {
		visible = 1
	}
	if visible > len(l) {
		visible = len(l)
	}
	if m.sel < m.top {
		m.top = m.sel
	}
	if m.sel >= m.top+visible {
		m.top = m.sel - visible + 1
	}
	if m.top > len(l)-visible {
		m.top = max(0, len(l)-visible)
	}
	m.input.SetWidth(max(1, w-4))
	var b strings.Builder
	b.WriteString(title.Render(m.title))
	b.WriteByte('\n')
	b.WriteString("  " + m.input.View())
	b.WriteByte('\n')
	if len(l) == 0 {
		b.WriteString(th.TextMuted().Render("  No results found"))
		return b.String()
	}
	for i := m.top; i < min(m.top+visible, len(l)); i++ {
		if i > m.top {
			b.WriteByte('\n')
		}
		b.WriteString(m.rowLine(l[i], i == m.sel, m.isCurrent != nil && m.isCurrent(l[i]), th, w))
	}
	b.WriteByte('\n')
	b.WriteString(th.TextMuted().Render("  \u2191/\u2193 move \u00B7 enter select \u00B7 esc close"))
	return b.String()
}

// rowLine renders one option row with the S0.9 home SELECT token chain
// (dialog-select's active row 667-678 + Option 732-791): the active row is
// the full-row paint in the selection background (theme primary) with the
// SelectedForeground text and the bold title; the current option carries the
// "●" gutter in primary (non-active rows) or the selection fg; other rows:
// the title in the text token, the description tail in textMuted. A zero
// Theme degrades to plain rows with the cursorStyle-bold active title.
func (m *selectModel) rowLine(o selectOption, active, cur bool, th theme.Theme, w int) string {
	gutter := "  "
	desc := ""
	if o.description != "" {
		desc = "  " + o.description
	}
	if active {
		bg, ok := th.Color("primary")
		if !ok {
			return cursorStyle(th).Render(gutter+o.title) + desc
		}
		sel := th.SelectedForeground()
		fg := lipgloss.Color(sel.Hex()[:7])
		bgC := lipgloss.Color(bg.Hex()[:7])
		head := lipgloss.NewStyle().Foreground(fg).Background(bgC).Bold(true)
		tail := lipgloss.NewStyle().Foreground(fg).Background(bgC)
		if cur {
			gutter = "● "
		}
		return lipgloss.NewStyle().Background(bgC).Width(w).Render(head.Render(gutter+o.title) + tail.Render(desc))
	}
	gutterSty := th.TextMuted()
	if cur {
		gutter = "● "
		gutterSty = th.Primary()
	}
	line := gutterSty.Render(gutter) + th.Text().Render(o.title)
	if desc != "" {
		line += th.TextMuted().Render(desc)
	}
	return line
}
```

`internal/tui/dialog.go` — the payload field + dispatch (S2.9/S2.10 flip the openers; the two-pane payloads stay for S2.2's test fixtures):

```go
type dialog struct {
	kind    dialogKind
	model   *modelDlg
	agent   *agentDlg
	form    *huhFormDlg
	sel     *selectModel // S2.5: the select payload (dlgModel/dlgAgents from S2.9/10)
	modal   bool
	size    dlgSize
	onClose func(*App)
}
```

In `modalInner`, extend the two cases (the select wins when present):

```go
	case dlgModel:
		if d.sel != nil {
			return d.sel.view(w, h, a.theme)
		}
		if d.model != nil {
			return d.model.view(&a.store, w, a.theme)
		}
	case dlgAgents:
		if d.sel != nil {
			return d.sel.view(w, h, a.theme)
		}
		if d.agent != nil {
			return d.agent.view(&a.store, w, a.theme)
		}
```

In `handleDialogKey`'s per-kind switch, extend the two cases:

```go
	case dlgModel:
		if d.sel != nil {
			return d.sel.handleKey(a, k)
		}
		if d.model == nil {
			a.dlg.pop()
			return nil
		}
		return d.model.handleKey(a, k)
	case dlgAgents:
		if d.sel != nil {
			return d.sel.handleKey(a, k)
		}
		if d.agent == nil {
			a.dlg.pop()
			return nil
		}
		return d.agent.handleKey(a, k)
```

And extend `dialogCanceler` (the select has no inner state in S2.5 — S2.9/10 wrap it):

```go
func dialogCanceler(d dialog) (modalCanceler, bool) {
	switch {
	case d.sel != nil:
		return nil, false
	case d.model != nil:
		return d.model, true
	case d.agent != nil:
		return d.agent, true
	}
	return nil, false
}
```

**Step 4 — run to verify it passes, then gate.**
`go test ./internal/tui/ -run 'TestSelect' -count=1` → PASS. Gate: `go vet ./... && go test ./...` + `gofmt -l .` empty. (fuzzy is now imported — both S2.1 requires are live.)

**Step 5 — commit + close the bead.**
`git add internal/tui/select.go internal/tui/dialog.go internal/tui/select_test.go && git commit -m "feat: select core - options, navigation, fuzzy filter"`
`bd close <S2.5 bead> --reason "select core green (wrap nav, home/end, enter, fuzzy filter + weighting, view layout)" --json`

---

### Task S2.6: Select — categories/groups + per-option details (bead `yolo-oae.3.6`, expected id `yolo-oae.3.9`)

**Files:** modify `internal/tui/select.go` (details field, `selLine` row model, `buildLines`, the flat-on-filter category headers, the footer tail, the row-count scroll window); new `internal/tui/locale.go` (truncateMiddle + titlecase ports); modify `internal/tui/select_test.go`.

**Interfaces:** consumes S2.5 (selectModel, filtered, rowLine, view). Produces: `selectOption.details []string`, `locale.go`'s `truncateMiddle(s string, w int) string` + `titlecase(s string) string` (ports of upstream `Locale.truncateMiddle` / `Locale.titlecase`, locale.ts — consumed here for details and in S2.8 for the perm info title), the `selLine{opt int, text string}` row model, `selectModel.buildLines(w int, th theme.Theme) []selLine`.

**Upstream parity notes:** `dialog-select.tsx` — category header rows (accent fg, bold, the group title; a blank row between groups; the headers are HIDDEN while the filter is active — the upstream `flat` behavior); per-option `details` rows (muted, indented under the row, truncateMiddle'd); the per-option `footer` right tail (muted, right-aligned to the row width); the scroll window counts rendered ROWS (headers + details + options), not options.

**Step 1 — write the failing tests.** Append to `internal/tui/select_test.go`:

```go
func TestSelectCategoriesRender(t *testing.T) {
	a := testApp()
	m := selectNew("Test", "Search", []selectOption{
		{title: "Alpha", category: "Group A"},
		{title: "Beta", category: "Group A"},
		{title: "Gamma", category: "Group B"},
	}, nil, nil, nil)
	lines := strings.Split(m.view(60, 24, a.theme), "\n")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "Group A") || !strings.Contains(joined, "Group B") {
		t.Fatalf("category headers missing:\n%s", joined)
	}
	// the blank row separates the groups (Group A's last row, blank, header)
	iA := -1
	iB := -1
	for i, l := range lines {
		if l == "   Group A" {
			iA = i
		}
		if l == "   Group B" {
			iB = i
		}
	}
	if iA == -1 || iB == -1 || lines[iB-1] != "" {
		t.Fatalf("header layout wrong (iA=%d iB=%d):\n%s", iA, iB, joined)
	}
	// filtering hides the headers (upstream flat)
	m.filter = "a"
	joined = strings.Join(strings.Split(m.view(60, 24, a.theme), "\n"), "\n")
	if strings.Contains(joined, "Group A") {
		t.Fatalf("headers must be hidden while filtering:\n%s", joined)
	}
}

func TestSelectDetailsAndFooter(t *testing.T) {
	a := testApp()
	m := selectNew("Test", "Search", []selectOption{
		{title: "Alpha", details: []string{"detail one", strings.Repeat("long detail ", 20)}, footer: "f1"},
	}, nil, nil, nil)
	lines := strings.Split(m.view(60, 24, a.theme), "\n")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "detail one") {
		t.Fatalf("detail row missing:\n%s", joined)
	}
	// the long detail is truncateMiddle'd to fit the row width
	for _, l := range lines {
		plain := stripANSI(l)
		if strings.Contains(plain, "long detail") && runeWidth(plain) > 60 {
			t.Fatalf("detail not clipped to the row width: %q", plain)
		}
	}
	// the footer tail sits at the right edge of its row
	for _, l := range lines {
		plain := strings.TrimRight(stripANSI(l), " ")
		if strings.Contains(plain, "Alpha") && strings.HasSuffix(plain, "f1") {
			return
		}
	}
	t.Fatalf("footer tail missing:\n%s", joined)
}

func TestSelectScrollWindowCountsRows(t *testing.T) {
	a := testApp()
	opts := make([]selectOption, 0, 20)
	for i := 0; i < 20; i++ {
		opts = append(opts, selectOption{
			title:   "Option " + string(rune('A'+i/26)) + string(rune('a'+i%26)),
			details: []string{"d"},
		})
	}
	m := selectNew("Test", "Search", opts, nil, nil, nil)
	// h=40 → visible = 40/2-6 = 14 rows; each option = 2 rows (row + detail)
	m.view(60, 40, a.theme)
	if m.top != 0 {
		t.Fatalf("initial top = %d, want 0", m.top)
	}
	for i := 0; i < 20; i++ { // walk the selection past the window
		m.move(1)
		m.view(60, 40, a.theme)
	}
	// 20 options × 2 rows = 40 rows; sel 19 → row 38-39 → top = 38-14+1 = 25
	if m.top < 20 {
		t.Fatalf("scroll did not follow the selection: top=%d", m.top)
	}
}
```

**Step 2 — run to verify it fails.**
`go test ./internal/tui/ -run 'TestSelect(Categories|Details|Scroll)' -count=1` → FAIL: the category headers / detail rows / footer tail are not rendered (the S2.5 view has no headers or details); `truncateMiddle` undefined.

**Step 3 — write the minimal implementation.** New `internal/tui/locale.go`:

```go
package tui

import "unicode"

// truncateMiddle is the port of upstream Locale.truncateMiddle (locale.ts):
// the head and tail survive, the middle collapses to "…" (width in columns,
// via runeWidth; short strings pass through).
func truncateMiddle(s string, w int) string {
	if w < 1 {
		return ""
	}
	if runeWidth(s) <= w {
		return s
	}
	r := []rune(s)
	if w == 1 {
		return string(r[:1])
	}
	room := w - 1
	head := room / 2
	tail := room - head
	out := make([]rune, 0, w)
	out = append(out, r[:head]...)
	out = append(out, '…')
	out = append(out, r[len(r)-tail:]...)
	return string(out)
}

// titlecase is the port of upstream Locale.titlecase (locale.ts): the first
// rune uppercased, the rest untouched.
func titlecase(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
```

`internal/tui/select.go` — the option gains the details:

```go
type selectOption struct {
	title       string
	description string
	details     []string
	footer      string
	category    string
	value       any
	disabled    bool
}
```

The row model + the list builder (replaces the inline window render in `view`):

```go
// selLine is one rendered line of the select list (the scroll window slices
// these): opt is the option index (-1 for a header/blank row).
type selLine struct {
	opt  int
	text string
}

// buildLines renders the full list (S2.6): the category header rows (accent
// bold, indent 3, a blank row between groups — hidden while filtering, the
// upstream `flat` behavior), each option row (S2.5's rowLine) and its
// detail rows (muted, indent 7, truncateMiddle'd), the per-option footer
// tail right-aligned to the row width.
func (m *selectModel) buildLines(w int, th theme.Theme) []selLine {
	l := m.filtered()
	flat := m.filter != ""
	var lines []selLine
	lastCat := ""
	for i, o := range l {
		if !flat && o.category != "" && o.category != lastCat {
			if lastCat != "" {
				lines = append(lines, selLine{opt: -1, text: ""})
			}
			lines = append(lines, selLine{opt: -1, text: th.Accent().Render("   " + o.category)})
			lastCat = o.category
		}
		active := i == m.sel
		cur := m.isCurrent != nil && m.isCurrent(o)
		var row string
		if o.footer != "" {
			row = m.rowWithFooter(o, active, cur, w, th)
		} else {
			row = m.rowLine(o, active, cur, th, w)
		}
		lines = append(lines, selLine{opt: i, text: row})
		for _, d := range o.details {
			lines = append(lines, selLine{opt: i, text: th.TextMuted().Render("       " + truncateMiddle(d, max(1, w-7)))})
		}
	}
	return lines
}
```

The row-with-footer contract: `rowWithFooter(o selectOption, active, cur bool, w int, th theme.Theme) string` builds the PLAIN content first (gutter + title + description), computes the right-tail gap from the plain rune widths, and renders the whole line in one pass — active: the S2.5 full-row paint with the footer inside the paint (the footer in the row's selection fg); unselected: the S2.5 segment styles with the footer muted. No post-hoc styled-string width stripping (that helper is test-only). `buildLines` calls `rowWithFooter` when `o.footer != ""`, else `rowLine` (S2.5 unchanged for the no-footer rows — the S2.5 tests stay green).

Rewrite `view` to slice the built rows (the scroll window now counts rows):

```go
func (m *selectModel) view(w, h int, th theme.Theme) string {
	m.syncFilter()
	lines := m.buildLines(w, th)
	visible := h/2 - 6
	if visible < 1 {
		visible = 1
	}
	if visible > len(lines) {
		visible = len(lines)
	}
	if len(lines) == 0 {
		var b strings.Builder
		b.WriteString(title.Render(m.title) + "\n  " + m.input.View() + "\n")
		b.WriteString(th.TextMuted().Render("  No results found"))
		return b.String()
	}
	// the selection's FIRST row anchors the window (S2.5: the option row)
	selRow := -1
	for i, l := range lines {
		if l.opt == m.sel {
			selRow = i
			break
		}
	}
	if selRow >= 0 {
		if selRow < m.top {
			m.top = selRow
		}
		if selRow >= m.top+visible {
			m.top = selRow - visible + 1
		}
	}
	if m.top > len(lines)-visible {
		m.top = max(0, len(lines)-visible)
	}
	m.input.SetWidth(max(1, w-4))
	var b strings.Builder
	b.WriteString(title.Render(m.title))
	b.WriteByte('\n')
	b.WriteString("  " + m.input.View())
	b.WriteByte('\n')
	for i := m.top; i < min(m.top+visible, len(lines)); i++ {
		if i > m.top {
			b.WriteByte('\n')
		}
		b.WriteString(lines[i].text)
	}
	b.WriteByte('\n')
	b.WriteString(th.TextMuted().Render("  \u2191/\u2193 move \u00B7 enter select \u00B7 esc close"))
	return b.String()
}
```

(Delete the S2.5 inline window code from `view`; `rowLine` stays as S2.5's. The `stripANSI` helper is test-only — in `footerTail` count the plain width with a local rune-width walk instead: the row is styled, so compute the gap from the PLAIN inputs — rework `footerTail` to take the plain row: `footerTail(plain string, footer string, w int, styled string, th theme.Theme) string` — the caller: active rows pass the pre-paint plain string. Implement it as: `func (m *selectModel) rowWithFooter(o selectOption, active, cur bool, w int, th theme.Theme) string` that builds the plain content first, computes the tail, then paints (active) or styles (plain) — replacing the `footerTail` free function.)

**Step 4 — run to verify it passes, then gate.**
`go test ./internal/tui/ -run 'TestSelect' -count=1` → PASS (S2.5's tests included — the no-category options render identically: no headers, no details, no footer). Gate: `go vet ./... && go test ./...` + `gofmt -l .` empty.

**Step 5 — commit + close the bead.**
`git add internal/tui/select.go internal/tui/locale.go internal/tui/select_test.go && git commit -m "feat: select - categories + per-option details"`
`bd close <S2.6 bead> --reason "categories (flat-on-filter), details, footer tail, row-count scroll window green" --json`

---

### Task S2.7: Select — actions + footer hints + scroll acceleration (bead `yolo-oae.3.7`, expected id `yolo-oae.3.10`)

**Files:** modify `internal/tui/select.go` (actions, hints, the focused-action state, the footer row, pgup/pgdn ±10); modify `internal/tui/select_test.go`.

**Interfaces:** consumes S2.6 (selectModel, view's row layout). Produces: `selectAction{key key.Binding, title string, run func(*App)}`, `footerHint{key, desc string}`, `selectModel.WithActions(actions []selectAction) *selectModel`, `selectModel.WithHints(hints []footerHint) *selectModel`, the `focAct int` state (-1 = none).

**Upstream parity notes:** `dialog-select.tsx` — the `actions` (left footer: each a key binding + title; the focused action is highlighted; tab/shift+tab cycle, the action's own key or enter-on-focus runs it) and the `hints` (right footer: `key` + `desc` pairs, muted). Scroll acceleration (upstream `getScrollAcceleration` + `CustomSpeedScroll(3)` — env-machined page scroll) → **deviation 170** (info): yolo pins pgup/pgdn = ±10 list rows (no env machinery).

**Step 1 — write the failing tests.** Append to `internal/tui/select_test.go` (the action tests use `key.NewBinding` — the test file's import block gains `"charm.land/bubbles/v2/key"`):

```go
var (
	selTabMsg      = tea.KeyPressMsg{Code: tea.KeyTab}
	selShiftTabMsg = tea.KeyPressMsg{Code: tea.KeyBackTab}
	selPgDnMsg     = tea.KeyPressMsg{Code: tea.KeyPgDn}
	selPgUpMsg     = tea.KeyPressMsg{Code: tea.KeyPgUp}
)

func TestSelectActions(t *testing.T) {
	a := testApp()
	favs, runs := 0, 0
	m := selectNew("Test", "Search", selTestOptions(), nil, nil, nil).
		WithActions([]selectAction{
			{key: key.NewBinding(key.WithKeys("f")), title: "Favorite", run: func(*App) { favs++ }},
			{key: key.NewBinding(key.WithKeys("r")), title: "Remove", run: func(*App) { runs++ }},
		})
	pushSelectModal(t, a, m)
	a.handleKey(press('f'))
	if favs != 1 {
		t.Fatalf("action key: favs=%d, want 1", favs)
	}
	a.handleKey(selTabMsg)
	if m.focAct != 0 {
		t.Fatalf("tab focus = %d, want 0", m.focAct)
	}
	a.handleKey(enterKey)
	if favs != 2 || runs != 0 {
		t.Fatalf("enter on the focused action must run it: favs=%d runs=%d", favs, runs)
	}
	a.handleKey(selShiftTabMsg) // wraps to the last action
	if m.focAct != 1 {
		t.Fatalf("shift+tab wrap = %d, want 1", m.focAct)
	}
}

func TestSelectFooterHints(t *testing.T) {
	a := testApp()
	m := selectNew("Test", "Search", selTestOptions(), nil, nil, nil).
		WithHints([]footerHint{{key: "ctrl+x", desc: "remove"}})
	lines := strings.Split(m.view(60, 24, a.theme), "\n")
	last := stripANSI(lines[len(lines)-1])
	if !strings.Contains(last, "ctrl+x") || !strings.Contains(last, "remove") {
		t.Fatalf("hint row = %q", last)
	}
}

func TestSelectScrollAcceleration(t *testing.T) {
	a := testApp()
	opts := make([]selectOption, 40)
	for i := range opts {
		opts[i] = selectOption{title: fmtOption(i)}
	}
	m := selectNew("Test", "Search", opts, nil, nil, nil)
	pushSelectModal(t, a, m)
	// h=40 → visible 14 rows; 40 options = 40 rows → the window can scroll
	a.handleKey(selPgDnMsg)
	m.view(60, 40, a.theme)
	if m.top != 10 {
		t.Fatalf("pgdn: top=%d, want 10 (±10 rows)", m.top)
	}
	a.handleKey(selPgDnMsg)
	m.view(60, 40, a.theme)
	if m.top != 20 {
		t.Fatalf("pgdn twice: top=%d, want 20", m.top)
	}
	a.handleKey(selPgUpMsg)
	m.view(60, 40, a.theme)
	if m.top != 10 {
		t.Fatalf("pgup: top=%d, want 10", m.top)
	}
}

func fmtOption(i int) string {
	return "Option " + string(rune('a'+i/26)) + string(rune('a'+i%26))
}
```

**Step 2 — run to verify it fails.**
`go test ./internal/tui/ -run 'TestSelect(Actions|FooterHints|ScrollAccel)' -count=1` → FAIL: compile errors (`WithActions`, `footerHint`, `focAct`, the pgup/pgdn behavior).

**Step 3 — write the minimal implementation.** Extend `selectModel` + the state in `select.go`:

```go
// selectAction is a footer action (upstream DialogSelectAction): its key
// triggers it, tab/shift+tab focus it, enter on the focus runs it.
type selectAction struct {
	key   key.Binding
	title string
	run   func(*App)
}

// footerHint is a right-footer hint (upstream: key + desc).
type footerHint struct {
	key  string
	desc string
}
```

(selectModel gains `actions []selectAction`, `hints []footerHint`, `focAct int` (init -1 in selectNew); the setters:)

```go
// WithActions attaches the left-footer actions (the focused one highlights).
func (m *selectModel) WithActions(actions []selectAction) *selectModel {
	m.actions = actions
	m.focAct = -1
	return m
}

// WithHints attaches the right-footer hints.
func (m *selectModel) WithHints(hints []footerHint) *selectModel {
	m.hints = hints
	return m
}
```

`handleKey` gains the page-scroll and action keys (final shape, replacing the S2.5 switch):

```go
// handleKey drives the select while the modal stack owns the frame (S2.7):
// an action's own key runs it; pgup/pgdn page-scroll the window;
// tab/shift+tab cycle the action focus; arrows move with wraparound,
// home/end jump, enter runs the focused action (or submits the selection);
// every other key feeds the fuzzy filter input (esc/ctrl+c are consumed
// by the stack first — S2.2).
func (m *selectModel) handleKey(a *App, k tea.KeyPressMsg) []tea.Cmd {
	for i := range m.actions {
		if key.Matches(k, m.actions[i].key) {
			m.actions[i].run(a)
			return nil
		}
	}
	switch {
	case key.Matches(k, selPgUpKey):
		m.pageScroll(-10)
	case key.Matches(k, selPgDnKey):
		m.pageScroll(10)
	case key.Matches(k, selTabKey):
		m.focusAction(+1)
	case key.Matches(k, selShiftTabKey):
		m.focusAction(-1)
	case key.Matches(k, homeKeyMap.Up):
		m.move(-1)
	case key.Matches(k, homeKeyMap.Down):
		m.move(1)
	case key.Matches(k, selHomeKey):
		m.jump(0)
	case key.Matches(k, selEndKey):
		m.jump(-1)
	case key.Matches(k, homeKeyMap.Enter):
		if m.focAct >= 0 {
			m.actions[m.focAct].run(a)
			return nil
		}
		m.submit(a)
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(k)
		m.syncFilter()
		if cmd != nil {
			return []tea.Cmd{cmd}
		}
		return nil
	}
	return nil
}
```

```go
var (
	selTabKey      = key.NewBinding(key.WithKeys("tab"))
	selShiftTabKey = key.NewBinding(key.WithKeys("shift+tab"))
	selPgUpKey     = key.NewBinding(key.WithKeys("pgup"))
	selPgDnKey     = key.NewBinding(key.WithKeys("pgdn"))
)

// pageScroll queues a window shift (deviation 170: ±10 rows pinned; the
// upstream env-machined getScrollAcceleration is not ported). The WINDOW
// moves (the selection stays); view() consumes the delta and re-anchors
// the window to the selection only when the selection itself changed
// (selectModel.lastSel — added in this task; S2.6's every-call re-anchor
// becomes the selection-change re-anchor):
//
//	if selRow >= 0 && selRow != m.lastSel { m.lastSel = selRow; <S2.6 anchor> }
//	else if m.pageDelta != 0 { m.top += m.pageDelta; m.pageDelta = 0; if m.top < 0 { m.top = 0 } }
//	// then the existing clamp (top > len(visible) → max(0, len-visible))
func (m *selectModel) pageScroll(rows int) {
	m.pageDelta += rows
}

// focusAction cycles the action focus (tab/shift+tab; no actions = no-op).
func (m *selectModel) focusAction(d int) {
	n := len(m.actions)
	if n == 0 {
		return
	}
	m.focAct = ((m.focAct+d)%n + n) % n
}
```

The footer row in `view` (replaces the S2.5 hint line when actions/hints exist; the S2.5 hint line stays as the no-actions fallback). The right-tail gap is computed from the PLAIN labels (before styling) — no styled-string width stripping:

```go
	// footer: actions left (focused highlighted), hints right (muted),
	// the S2.5 keymap hint when neither exists
	if len(m.actions) > 0 || len(m.hints) > 0 {
		var leftParts, leftPlain []string
		for i, ac := range m.actions {
			label := ac.key.Keys()[0] + " " + ac.title
			leftPlain = append(leftPlain, label)
			if i == m.focAct {
				leftParts = append(leftParts, cursorStyle(th).Render(label))
			} else {
				leftParts = append(leftParts, th.TextMuted().Render(label))
			}
		}
		rightPlain := strings.Join(hintTexts(m.hints), " \u00B7 ")
		line := strings.Join(leftParts, "   ")
		if rightPlain != "" {
			gap := w - 2 - plainJoinWidth(leftPlain, "   ") - runeWidth(rightPlain)
			if gap < 1 {
				gap = 1
			}
			line += strings.Repeat(" ", gap) + th.TextMuted().Render(rightPlain)
		}
		b.WriteString(line)
	} else {
		b.WriteString(th.TextMuted().Render("  \u2191/\u2193 move \u00B7 enter select \u00B7 esc close"))
	}
```

with the two tiny plain-width helpers (both in select.go):

```go
// hintTexts is the right-tail text of the hints (key + desc pairs).
func hintTexts(hints []footerHint) []string {
	out := make([]string, len(hints))
	for i, h := range hints {
		out[i] = h.key + " " + h.desc
	}
	return out
}

// plainJoinWidth is the rendered width of the joined parts (rune columns).
func plainJoinWidth(parts []string, sep string) int {
	w := 0
	for i, p := range parts {
		if i > 0 {
			w += runeWidth(sep)
		}
		w += runeWidth(p)
	}
	return w
}
```

**Step 4 — run to verify it passes, then gate.**
`go test ./internal/tui/ -run 'TestSelect' -count=1` → PASS (all S2.5/S2.6 tests stay green). Gate: `go vet ./... && go test ./...` + `gofmt -l .` empty.

**Step 5 — commit + close the bead.**
`git add internal/tui/select.go internal/tui/select_test.go && git commit -m "feat: select - actions, footer hints, scroll acceleration"`
`bd close <S2.7 bead> --reason "actions (keys, tab focus, enter), footer hints, pgup/pgdn ±10 green" --json`
Log **deviation 170** (info) in `docs/superpowers/DEVIATIONS.md` (same commit): the select's scroll acceleration is pinned to ±10 list rows (upstream's env-machined `getScrollAcceleration` is not ported).

---

### Task S2.8: Permission dialog restyle (on the modal stack) (bead `yolo-oae.3.8`, expected id `yolo-oae.3.11`)

**Files:** rewrite `internal/tui/permission.go` (permDlg + the ported `info()` + pills + key handler); modify `internal/tui/dialog.go` (`dlgPerm` kind, `perm *permDlg` field, `modalInner` case, `syncPermDialog`); modify `internal/tui/keys.go` (the permission branch dispatches to the perm dialog); modify `internal/tui/app.go` (`EventMsg` → `syncPermDialog`); re-baseline `internal/tui/permission_test.go`, `internal/tui/tui_suite_test.go` (`hasPermDialogEcho`), `internal/tui/overflow_test.go` (`TestPermissionViewWraps`); new `internal/tui/permission_theme_test.go` (the SGR golden).

**Interfaces:** consumes S2.2 (the modal stack), S2.6 (`locale.go`'s `titlecase`), the store (`store.Pending`, `store.Messages` → the part-input lookup), the client (`a.ReplyPermission`). Produces: `permDlg{sel int}` (`handleKey(a *App, k tea.KeyPressMsg) []tea.Cmd`, `view(st *store.State, w int, th theme.Theme) string`, `moveSel(d int)`), `permInfo(p protocol.PermissionAskedProps, input map[string]any) (icon, title, body string)` (the port of permission.tsx:195-380 over the part input — **deviation 169**, low: the wire carries no request `Meta` and no diff view; the part's `State.Input` map is the data source), `permReplies = []string{"once","always","reject"}`, `permReplyFor(k tea.KeyPressMsg, sel int) (string, bool)` (the key→reply-mode seam), `App.syncPermDialog()`, `App.permissionView(w int) string` (keeps its name — the top perm dialog's content; a fresh selection when none is on the stack, for the unit tests).

**Upstream parity notes:** `permission.tsx` — the header `△` (warning) + "Permission required" (text); the info row (muted icon, text title, the per-permission body line at paddingLeft 1: edit `→ Edit <path>`, read `→ Read <path>` + `Path:`, glob `✱ Glob "<p>"` + `Pattern:`, grep `✱ Grep "<p>"` + `Pattern:`, list `→ List <dir>` + `Path:`, bash `# Shell command` + `$ <cmd>`, task `# <Type> Task` + `◉ <desc>`, webfetch `% WebFetch <url>` + `URL:`, websearch `◈ <Provider> "<q>"` + `Query:`, external_directory `← Access external directory <dir>` + patterns, doom_loop `⟳ Continue after repeated failures`); the reply pills `Allow once / Allow always / Reject` (permission.tsx:405) with the selected pill painted (upstream: accent bg/fg — yolo pins the **warning** token per the S1 error/toast lineage; the look note rides deviation 167). The yolo pins stay: `1`/`2`/`3` reply directly, `esc` = reject; NEW (upstream's pill navigation): `left`/`right` move the selection with wrap, `enter` replies with the selected pill. The old `tool call: msg/call` line is dropped (internal — the info() title carries the what; deviation 169).

**Step 1 — write the failing tests.** Replace `internal/tui/permission_test.go`'s render/keys tests (keep the file's harness helpers `permProps`/`permApp` as-is):

```go
func TestPermissionRender(t *testing.T) {
	t.Run("bash ask with no part input", func(t *testing.T) {
		a := permApp()
		got := stripANSI(a.permissionView(80))
		want := "△ Permission required\n" +
			"  # Shell command\n" +
			"  patterns: ls *\n" +
			"  Always: ls, dir/*\n" +
			"Allow once  Allow always  Reject"
		if got != want {
			t.Errorf("permissionView mismatch:\ngot:\n%q\nwant:\n%q", got, want)
		}
	})

	t.Run("part input renders the body line", func(t *testing.T) {
		a := permApp()
		a.store.Messages = []protocol.MessageWithParts{{
			Info: protocol.Message{ID: "msg_1"},
			Parts: []protocol.Part{{
				ID: "prt_1", MessageID: "msg_1", CallID: "call_abcdef", Type: "tool",
				State: &protocol.ToolState{Input: map[string]any{"command": "echo hi"}},
			}},
		}}
		got := stripANSI(a.permissionView(80))
		if !strings.Contains(got, "  $ echo hi") {
			t.Errorf("body line missing:\n%q", got)
		}
	})

	t.Run("edit ask formats the path title", func(t *testing.T) {
		a := permApp()
		a.store.Pending[0].Permission = "edit"
		a.store.Pending[0].Patterns = []string{"/tmp/x.go"}
		a.store.Pending[0].Always = nil
		a.store.Pending[0].Tool = nil
		a.store.Messages = []protocol.MessageWithParts{{
			Parts: []protocol.Part{{
				CallID: "c1", Type: "tool",
				State: &protocol.ToolState{Input: map[string]any{"filePath": "/tmp/x.go"}},
			}},
		}}
		got := stripANSI(a.permissionView(80))
		if !strings.Contains(got, "  → Edit /tmp/x.go") {
			t.Errorf("edit title missing:\n%q", got)
		}
		if strings.Contains(got, "Always:") || strings.Contains(got, "patterns:") {
			t.Errorf("empty lines must be omitted:\n%q", got)
		}
	})

	t.Run("empty always omits the line", func(t *testing.T) {
		a := permApp()
		a.store.Pending[0].Always = nil
		if got := stripANSI(a.permissionView(80)); strings.Contains(got, "Always:") {
			t.Errorf("Always line must be omitted when empty:\n%q", got)
		}
	})
}

func TestPermissionPillKeys(t *testing.T) {
	if got, ok := permReplyFor(press('1'), 0); !ok || got != "once" {
		t.Fatalf("1 → %q/%v, want once", got, ok)
	}
	if got, ok := permReplyFor(press('2'), 0); !ok || got != "always" {
		t.Fatalf("2 → %q/%v, want always", got, ok)
	}
	if got, ok := permReplyFor(press('3'), 0); !ok || got != "reject" {
		t.Fatalf("3 → %q/%v, want reject", got, ok)
	}
	if got, ok := permReplyFor(press(tea.KeyEscape), 1); !ok || got != "reject" {
		t.Fatalf("esc → %q/%v, want reject (yolo pin)", got, ok)
	}
	if got, ok := permReplyFor(enterKey, 1); !ok || got != "always" {
		t.Fatalf("enter on sel 1 → %q/%v, want always", got, ok)
	}
	if _, ok := permReplyFor(press('x'), 0); ok {
		t.Fatalf("x must not reply")
	}
}

func TestPermissionPillMove(t *testing.T) {
	p := &permDlg{}
	p.moveSel(1)
	if p.sel != 1 {
		t.Fatalf("right: sel=%d, want 1", p.sel)
	}
	p.moveSel(1)
	if p.sel != 2 {
		t.Fatalf("right: sel=%d, want 2", p.sel)
	}
	p.moveSel(1)
	if p.sel != 0 {
		t.Fatalf("wrap: sel=%d, want 0", p.sel)
	}
	p.moveSel(-1)
	if p.sel != 2 {
		t.Fatalf("wrap back: sel=%d, want 2", p.sel)
	}
}

func TestPermissionStackSync(t *testing.T) {
	a := permApp()
	a.syncPermDialog()
	top, ok := a.dlg.top()
	if len(a.dlg.items) != 1 || !ok || top.kind != dlgPerm || !top.modal {
		t.Fatalf("sync must push the perm modal: %+v", a.dlg.items)
	}
	a.syncPermDialog() // idempotent
	if len(a.dlg.items) != 1 {
		t.Fatalf("sync must be idempotent: %+v", a.dlg.items)
	}
	a.store.Pending = nil
	a.syncPermDialog()
	if len(a.dlg.items) != 0 {
		t.Fatalf("drained queue must pop the perm modal: %+v", a.dlg.items)
	}
}
```

Re-baseline `internal/tui/tui_suite_test.go` (same commit):

```go
// hasPermDialogEcho matches this scenario's bash ask (the S2.8 restyle:
// the info() port reads the part input — "$ echo hi" — and the pills).
func hasPermDialogEcho(b []byte) bool {
	s := stripANSI(string(b))
	return strings.Contains(s, "Permission required") &&
		strings.Contains(s, "Shell command") &&
		strings.Contains(s, "$ echo hi") &&
		strings.Contains(s, "patterns: echo *") &&
		strings.Contains(s, "Allow once") &&
		strings.Contains(s, "Reject")
}
```

Re-baseline `internal/tui/permission_test.go`'s teatest matcher + the overlay-order test (same commit — the old-look tokens are gone):

```go
// hasPermDialog matches the real permission flow under the S2.8 restyle
// (header + the info() port + the pills).
func hasPermDialog(b []byte) bool {
	s := stripANSI(string(b))
	return strings.Contains(s, "Permission required") &&
		strings.Contains(s, "Shell command") &&
		strings.Contains(s, "patterns: ls *") &&
		strings.Contains(s, "Allow once") &&
		strings.Contains(s, "Reject")
}
```

`TestPermissionOverlayAbovePrompt` keeps its shape; its `permission · bash` token re-baselines to `Permission required`. The teatest scenario tests (`TestPermissionDialogKeyReply`, `TestPermissionDialogHTTPReply`, `TestPermissionKeyReplyWiring`) and the `applyPermReply` tests keep their logic unchanged (the key ladder now reaches the perm dialog through the S2.8 `handleKey` branch; the wire flow is untouched).

Re-baseline `internal/tui/overflow_test.go` `TestPermissionViewWraps`:

```go
func TestPermissionViewWraps(t *testing.T) {
	a := permApp()
	a.store.Pending[0].Patterns = []string{strings.Repeat("ls -la /very/long/path ", 6)}
	got := stripANSI(a.permissionView(20))
	fitsWidth(t, got, 20)
	if !strings.Contains(rejoined(got), "patterns: "+strings.TrimRight(strings.Repeat("ls -la /very/long/path ", 6), " ")) {
		t.Fatalf("permission text lost in wrap:\n%q", got)
	}
}
```

New `internal/tui/permission_theme_test.go` (the SGR golden under the real engine):

```go
package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/teatest/v2"
)

// TestPermissionDialogSGR pins the restyled permission dialog's paint
// (TTY_FORCE ANSI256): the warning header token (fg 215) and the selected
// pill's warning background (bg 215, deviation 167's pinned yolo token).
func TestPermissionDialogSGR(t *testing.T) {
	tm, ts := permFlowHarness(t)
	driveToPermDialog(t, tm, ts)
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		s := string(b)
		return strings.Contains(s, "38;5;215m") &&
			strings.Contains(s, "48;5;215m") &&
			strings.Contains(stripANSI(s), "Allow once")
	}, teatest.WithDuration(5*time.Second))
	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
```

**Step 2 — run to verify it fails.**
`go test ./internal/tui/ -run 'TestPermission' -count=1` → FAIL: compile errors (`undefined: permDlg`, `undefined: permReplyFor`, `undefined: dlgPerm`, `undefined: syncPermDialog`) + the render block mismatches the old look.

**Step 3 — write the minimal implementation.** Rewrite `internal/tui/permission.go`:

```go
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/store"
	"github.com/kido5217/yolo/internal/tui/theme"
)

var (
	permOnce   = key.NewBinding(key.WithKeys("1"))
	permAlways = key.NewBinding(key.WithKeys("2"))
	permReject = key.NewBinding(key.WithKeys("3", "esc"))
	permLeft   = key.NewBinding(key.WithKeys("left"))
	permRight  = key.NewBinding(key.WithKeys("right"))
)

// permReplies maps the pill index to the wire reply.
var permReplies = []string{"once", "always", "reject"}

// permDlg is the permission dialog payload (S2.8: the parked ask on the
// modal stack; sel = the highlighted pill).
type permDlg struct {
	sel int
}

// moveSel steps the pill with wraparound (upstream's pill navigation).
func (p *permDlg) moveSel(d int) {
	p.sel = ((p.sel + d) % len(permReplies) + len(permReplies)) % len(permReplies)
}

// permReplyFor maps a key to the reply mode (the test seam + the handler's
// core): 1/2/3 reply directly (yolo pin), esc = reject (yolo pin), enter
// replies with the selected pill.
func permReplyFor(k tea.KeyPressMsg, sel int) (string, bool) {
	switch {
	case key.Matches(k, permOnce):
		return "once", true
	case key.Matches(k, permAlways):
		return "always", true
	case key.Matches(k, permReject):
		return "reject", true
	case key.Matches(k, homeKeyMap.Enter):
		return permReplies[sel], true
	}
	return "", false
}

// handleKey drives the dialog: the reply keys, the pill navigation, and
// everything else ignored (the modal stack owns esc/ctrl+c too — the
// stack's esc path and the permReject binding both reject; the reply
// wins the key ladder because the permission branch precedes the dialog).
func (p *permDlg) handleKey(a *App, k tea.KeyPressMsg) []tea.Cmd {
	if reply, ok := permReplyFor(k, p.sel); ok {
		return a.emit(a.replyPermCmd(reply)...)
	}
	switch {
	case key.Matches(k, permLeft):
		p.moveSel(-1)
	case key.Matches(k, permRight):
		p.moveSel(1)
	}
	return nil
}

// permInfo is the port of the upstream info() (permission.tsx:195-380) over
// the store part's input map (deviation 169: the wire carries no request
// Meta; the part input is the data source; the EditBody diff view is
// dropped). The input keys follow the tool schemas (camelCase: filePath,
// pattern, command, url, query, path).
func permInfo(p protocol.PermissionAskedProps, input map[string]any) (icon, title, body string) {
	str := func(k string) string {
		if v, ok := input[k].(string); ok {
			return v
		}
		return ""
	}
	switch p.Permission {
	case "edit":
		return "→", "Edit " + str("filePath"), ""
	case "read":
		fp := str("filePath")
		if fp != "" {
			body = "Path: " + fp
		}
		return "→", "Read " + fp, body
	case "glob":
		pat := str("pattern")
		if pat != "" {
			body = "Pattern: " + pat
		}
		return "✱", fmt.Sprintf("Glob %q", pat), body
	case "grep":
		pat := str("pattern")
		if pat != "" {
			body = "Pattern: " + pat
		}
		return "✱", fmt.Sprintf("Grep %q", pat), body
	case "list":
		dir := str("path")
		if dir != "" {
			body = "Path: " + dir
		}
		return "→", "List " + dir, body
	case "bash":
		cmd := str("command")
		if cmd != "" {
			body = "$ " + cmd
		}
		return "#", "Shell command", body
	case "task":
		typ := str("subagent_type")
		if typ == "" {
			typ = "Unknown"
		}
		desc := str("description")
		if desc != "" {
			body = "◉ " + desc
		}
		return "#", titlecase(typ) + " Task", body
	case "webfetch":
		url := str("url")
		if url != "" {
			body = "URL: " + url
		}
		return "%", "WebFetch " + url, body
	case "websearch":
		query := str("query")
		if query != "" {
			body = "Query: " + query
		}
		return "◈", fmt.Sprintf("Search %q", query), body
	case "doom_loop":
		return "⟳", "Continue after repeated failures", "This keeps the session running despite repeated failures."
	default:
		return "⚙", "Call tool " + p.Permission, "Tool: " + p.Permission
	}
}

// partInput returns the parked ask's tool-part input map (the info() data
// source; nil when the part is not hydrated yet).
func partInput(st *store.State, p protocol.PermissionAskedProps) map[string]any {
	if p.Tool == nil {
		return nil
	}
	for _, m := range st.Messages {
		for _, pr := range m.Parts {
			if pr.CallID == p.Tool.CallID && (p.Tool.MessageID == "" || pr.MessageID == p.Tool.MessageID) && pr.State != nil {
				return pr.State.Input
			}
		}
	}
	return nil
}

// view renders the dialog content (the modal stack draws the frame —
// S2.2): the warning header, the info() icon+title (one row) + body, the
// patterns and Always lines, the reply pills (the selected pill painted in
// the warning token — deviation 167's yolo pin; unselected pills muted).
// `rows` are pre-styled lines; each wraps at w (patterns carry full
// command strings — the long line is styled as a whole then wrapped, same
// contract as the old permissionView). The body line sits at 4 columns
// (the upstream body box is paddingLeft 1 relative to the icon column).
func (p *permDlg) view(st *store.State, w int, th theme.Theme) string {
	if len(st.Pending) == 0 {
		return ""
	}
	ask := st.Pending[0]
	icon, title, body := permInfo(ask, partInput(st, ask))
	muted := th.TextMuted()
	rows := []string{
		th.Warning().Render("△ ") + th.Text().Render("Permission required"),
		"  " + muted.Render(icon) + th.Text().Render(" " + title),
	}
	if body != "" {
		rows = append(rows, "    "+muted.Render(body))
	}
	if len(ask.Patterns) > 0 {
		rows = append(rows, muted.Render("  patterns: "+strings.Join(ask.Patterns, ", ")))
	}
	if len(ask.Always) > 0 {
		rows = append(rows, muted.Render("  Always: "+strings.Join(ask.Always, ", ")))
	}
	rows = append(rows, p.pills(th))
	var b strings.Builder
	for i, r := range rows {
		if i > 0 {
			b.WriteByte('\n')
		}
		for j, l := range strings.Split(wrapLine(r, w), "\n") {
			if j > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(l)
		}
	}
	return b.String()
}
```

The pills:

```go
// pills renders the reply pill row: the selected pill painted in the
// warning token (warning bg + the SelectedForeground-on-warning fg, bold),
// the unselected pills muted (upstream: the selected pill is accent — yolo
// pins warning; deviation 167's look note).
func (p *permDlg) pills(th theme.Theme) string {
	labels := []string{"Allow once", "Allow always", "Reject"}
	parts := make([]string, len(labels))
	for i, l := range labels {
		if i == p.sel {
			bg, ok := th.Color("warning")
			if !ok {
				parts[i] = cursorStyle(th).Render(l)
				continue
			}
			fg := lipgloss.Color(th.SelectedForeground(bg).Hex()[:7])
			parts[i] = lipgloss.NewStyle().Foreground(fg).
				Background(lipgloss.Color(bg.Hex()[:7])).Bold(true).
				Padding(0, 1).Render(l)
		} else {
			parts[i] = th.TextMuted().Render(l)
		}
	}
	return strings.Join(parts, "  ")
}
```

`internal/tui/dialog.go` — the kind + field + the modal content + the sync:

```go
const (
	dlgNone dialogKind = iota
	dlgQuit
	dlgHelp
	dlgModel
	dlgAgents
	dlgForm
	dlgPerm
)

type dialog struct {
	kind    dialogKind
	model   *modelDlg
	agent   *agentDlg
	form    *huhFormDlg
	sel     *selectModel
	perm    *permDlg
	modal   bool
	size    dlgSize
	onClose func(*App)
}
```

In `modalInner`, add:

```go
	case dlgPerm:
		if d.perm != nil {
			return d.perm.view(&a.store, w, a.theme)
		}
```

The stack sync (the permission event path):

```go
// syncPermDialog keeps the permission modal in step with the parked asks
// (S2.8): pushed when the first ask parks, popped when the queue drains
// (the reply landed or the permission.replied event dropped it).
func (a *App) syncPermDialog() {
	has := false
	for _, d := range a.dlg.items {
		if d.kind == dlgPerm {
			has = true
			break
		}
	}
	if len(a.store.Pending) > 0 && !has {
		a.pushModal(dialog{kind: dlgPerm, perm: &permDlg{}}, dlgMedium, nil)
		return
	}
	if len(a.store.Pending) == 0 && has {
		for i := len(a.dlg.items) - 1; i >= 0; i-- {
			if a.dlg.items[i].kind == dlgPerm {
				a.dlg.items = append(a.dlg.items[:i], a.dlg.items[i+1:]...)
				break
			}
		}
	}
}
```

`internal/tui/keys.go` — the permission branch (the permission still owns every key; the top perm dialog drives it, with a stateless fallback):

```go
func (a *App) handleKey(k tea.KeyPressMsg) []tea.Cmd {
	if len(a.store.Pending) > 0 {
		if d, ok := a.dlg.top(); ok && d.kind == dlgPerm && d.perm != nil {
			return d.perm.handleKey(a, k)
		}
		return (&permDlg{}).handleKey(a, k)
	}
	// …the existing ladder, unchanged…
}
```

`internal/tui/app.go` — the event path:

```go
	case EventMsg:
		a.store.Live = true
		a.store.Apply(m.Event)
		a.syncPermDialog()
		// …the existing re-render logic, unchanged…
```

Keep `replyPermCmd`/`permReplyMsg` as-is (the wire flow is unchanged). `applyPermReply` gains one line — `a.syncPermDialog()` after the Pending update — so the dialog pops locally on a successful reply without waiting for the `permission.replied` SSE event (the event stays as the idempotent second path; a failed reply keeps the ask and the dialog, as today). Delete the old `permissionView` body (replace with):

```go
// permissionView renders the top permission dialog's content (S2.8; a
// fresh selection when no perm dialog is on the stack — the unit tests).
func (a *App) permissionView(w int) string {
	p := &permDlg{}
	if d, ok := a.dlg.top(); ok && d.kind == dlgPerm && d.perm != nil {
		p = d.perm
	}
	return p.view(&a.store, w, a.theme)
}
```

`short6` is now unused — delete it (and its test references, if any).

**Step 4 — run to verify it passes, then gate.**
`go test ./internal/tui/ -run 'TestPermission' -count=1` → PASS. Then the FULL gate — `go vet ./... && go test ./...` — because the teatest suites re-run the real permission flow (`TestTUIPermissionFlow` once/always/reject, `TestTUIFullTurn`'s perm echo). The re-baselined `hasPermDialogEcho` must hold under the real engine (the part input "$ echo hi" requires the part to be hydrated before the dialog renders — the driveToPermDialog sync point (`waitPending`) covers it). Pin discipline (principle 3): if a re-baselined pin string is off but the token chain (header/info/pills placement) is correct, re-pin to the actual first-green output in this same commit. `gofmt -l .` empty.

**Step 5 — commit + close the bead.**
`git add internal/tui/permission.go internal/tui/dialog.go internal/tui/keys.go internal/tui/app.go internal/tui/permission_test.go internal/tui/tui_suite_test.go internal/tui/overflow_test.go internal/tui/permission_theme_test.go && git commit -m "feat: permission dialog restyle (on select)"`
`bd close <S2.8 bead> --reason "permission restyle green: info() port, pills, 1/2/3+esc pin, SGR golden, teatest flow" --json`
Log **deviation 169** (low) in `docs/superpowers/DEVIATIONS.md` (same commit): the permission detail lines come from the store part's input map (the wire carries no request Meta); the upstream rich Meta + `MetaDiff` diff view are not ported.

---

### Task S2.9: Model dialog restyle (on select) (bead `yolo-oae.3.9`, expected id `yolo-oae.3.12`)

**Files:** rewrite `internal/tui/dialog.go`'s modelDlg (the two-pane picker → the flat select + the a/b subchoice — **deviation 168**, medium: the user-visible behavior change the spec's S2 row names), `openModelDialog`, `syncModelSel`, `applyCatalog`; `modalInner`'s model case (the view gains the `h` param); re-baseline `internal/tui/model_test.go` (all), `internal/tui/overflow_test.go` (`TestModelDlgViewWraps`), `internal/tui/tui_suite_test.go` (`TestTUIDialogs`'s model leg, if it pins the two-pane tokens); modify `internal/tui/permission.go`? NO.

**Interfaces:** consumes S2.5–S2.7 (selectModel: options, filter, the ● current gutter, the footer tail), S2.2 (the modal stack, `dlgLarge`), the existing patch flow (`patchDlgCmd`/`applyDlgPatch` — unchanged, the a/b subchoice feeds it). Produces: `modelDlg{sel *selectModel, hasSubChoice bool, pick string}`, `modelOptions(st *store.State) []selectOption`, `modelIsCurrentOpt(st *store.State) func(selectOption) bool`, `providerStatusText(auth *protocol.ProviderAuth) string` (split from `providerStatus`, which keeps its style), `modelSelIndex(st *store.State) int`. The locked `[a] this session  [b] set default` overlay text and the `model set: <ref>` toast stay byte-identical (yolo pins).

**Upstream parity notes:** `dialog-model.tsx` — upstream's model dialog is a two-pane DialogSelect (providers pane + models pane, favorites/recents, `size="large"`). yolo's S2.9 decision (the plan's S2.9 row, approved): a FLAT select over all catalog models — title = model name, category = provider name (the upstream categories affordance replaces the provider pane), description = the context + cost tail, footer = the provider status, the current model ●-marked; enter → the yolo-pinned a/b subchoice (the subchoice is yolo-specific — upstream applies directly). **Deviation 168** (medium): the two-pane picker is replaced by the flat select (behavior change, spec-approved); favorites/recents (upstream-only, KV-backed) are not ported (S3 scope if ever).

**Step 1 — write the failing tests.** Replace `internal/tui/model_test.go` (keep the harness helpers `pressTab`/`pressCtrlP`/`providerFixture`/`modelFixture`/`openModelAt` where they still apply; `pressTab` becomes unused for the model dialog but is used by no test → delete it if fully unused):

```go
// model_test.go — the S2.9 restyle: the flat model select (deviation 168)
// + the yolo-pinned a/b subchoice.

func TestModelDialogRender(t *testing.T) {
	t.Run("catalog flattens into the select, the current model is marked", func(t *testing.T) {
		a := openModelAt()
		got := stripANSI(a.dlg.model().view(&a.store, 80, 24, a.theme))
		lines := strings.Split(got, "\n")
		if !strings.Contains(lines[0], "Model") {
			t.Fatalf("title row = %q", lines[0])
		}
		if !strings.Contains(lines[1], "Search") {
			t.Fatalf("filter row = %q", lines[1])
		}
		// category headers + the rows (the zero-theme renders plain)
		if !strings.Contains(got, "   Kido") || !strings.Contains(got, "   OpenCode Zen") {
			t.Fatalf("category headers missing:\n%s", got)
		}
		// the current model: the ● gutter + the active-row title + the ctx/cost tail
		var qwenLine string
		for _, l := range lines {
			if strings.Contains(l, "Qwen") {
				qwenLine = l
			}
		}
		if !strings.Contains(qwenLine, "●") || !strings.Contains(qwenLine, "100k ctx") || !strings.Contains(qwenLine, "$0/$0") {
			t.Fatalf("current model row = %q", qwenLine)
		}
		if !strings.Contains(got, "Claude Opus 4.7") || !strings.Contains(got, "GPT-5 Nano") {
			t.Fatalf("catalog models missing:\n%s", got)
		}
	})

	t.Run("no providers renders the loading hint", func(t *testing.T) {
		a := modelFixture()
		a.store.Providers = nil
		a.openModelDialog()
		got := stripANSI(a.dlg.model().view(&a.store, 80, 24, a.theme))
		if !strings.Contains(got, "Model") || !strings.Contains(got, "loading…") {
			t.Fatalf("loading hint missing:\n%s", got)
		}
	})

	t.Run("filter narrows the list", func(t *testing.T) {
		a := openModelAt()
		a.handleKey(press('g')) // only "GPT-5 Nano" matches
		got := stripANSI(a.dlg.model().view(&a.store, 80, 24, a.theme))
		if strings.Contains(got, "Qwen") || strings.Contains(got, "Claude") {
			t.Fatalf("filter did not narrow:\n%s", got)
		}
		if !strings.Contains(got, "GPT-5 Nano") {
			t.Fatalf("filtered row missing:\n%s", got)
		}
	})

	t.Run("subchoice line is the locked [a]/[b] overlay", func(t *testing.T) {
		a := openModelAt()
		a.handleKey(press(tea.KeyEnter)) // the current (selected) model
		got := stripANSI(a.dlg.model().view(&a.store, 80, 24, a.theme))
		if !strings.Contains(got, "[a] this session  [b] set default") {
			t.Fatalf("subchoice missing:\n%s", got)
		}
	})
}

func TestModelDialogKeys(t *testing.T) {
	t.Run("esc closes the subchoice first, then the dialog", func(t *testing.T) {
		a := openModelAt()
		a.handleKey(press(tea.KeyEnter))
		a.handleKey(press(tea.KeyEscape))
		if a.dlg.model().hasSubChoice {
			t.Fatalf("first esc must close the subchoice only")
		}
		if a.dlg.empty() {
			t.Fatalf("the dialog must stay open")
		}
		a.handleKey(press(tea.KeyEscape))
		if !a.dlg.empty() {
			t.Fatalf("second esc must close the dialog")
		}
	})

	t.Run("a/b apply through the existing patch flow", func(t *testing.T) {
		a := openModelAt()
		a.handleKey(press(tea.KeyDown)) // to the next model (GPT-5 Nano? no — next option: Claude Opus 4.7)
		a.handleKey(press(tea.KeyEnter))
		a.handleKey(press('b')) // set default
		// the cmd chain is the existing patchDlgCmd (config patch)
		if len(a.Cmds) == 0 {
			t.Fatalf("b must emit the patch cmd")
		}
	})
}
```

`TestModelDialogApply` keeps its existing subtests (session patch, config
patch, the no-session toast path) and its `applyDlgPatch`-based asserts
unchanged; every two-pane driver step (pressTab, the pane moves) is
re-pointed at the select equivalents (press arrows to the model, enter to
open the subchoice, then a/b).

Re-baseline `internal/tui/overflow_test.go`:

```go
func TestModelDlgViewWraps(t *testing.T) {
	a := openModelAt()
	got := stripANSI(a.dlg.model().view(&a.store, 40, 24, a.theme))
	fitsWidth(t, got, 40)
	flat := strings.Join(strings.Fields(rejoined(got)), " ")
	for _, tok := range []string{"Qwen", "Claude Opus 4.7", "GPT-5 Nano"} {
		if !strings.Contains(flat, tok) {
			t.Fatalf("model dialog lost %q in wrap:\n%q", tok, got)
		}
	}
}
```

`TestTUIModelDialog` (the teatest) — re-point the driver steps at the flat select:

```go
	tm.Send(pressCtrlP())
	teatest.WaitFor(t, tm.Output(), hasModelDialog, teatest.WithDuration(5*time.Second))
	// type the filter to GPT-5 Nano, enter → the subchoice, a = this session
	tm.Send(press('g'))
	tm.Send(press(tea.KeyEnter))
	tm.Send(press('a'))
	teatest.WaitFor(t, tm.Output(), hasLine("model set: opencode/gpt-5-nano"), teatest.WithDuration(5*time.Second))
```

with the re-baselined matcher:

```go
func hasModelDialog(b []byte) bool {
	s := stripANSI(string(b))
	return strings.Contains(s, "Kido") &&
		strings.Contains(s, "OpenCode Zen") &&
		strings.Contains(s, "● Qwen") &&
		strings.Contains(s, "100k ctx")
}
```

(If `TestTUIDialogs` in `tui_suite_test.go` also drives the model dialog through the two-pane, re-point its steps the same way in this commit.)

**Step 2 — run to verify it fails.**
`go test ./internal/tui/ -run 'TestModelDialog|TestTUIModelDialog' -count=1` → FAIL: compile errors (the view signature, `modelOptions`, the two-pane fields gone) + the old pins fail against the new look.

**Step 3 — write the minimal implementation.** `internal/tui/dialog.go` — replace the two-pane modelDlg:

```go
// modelDlg is the ctrl+p / /model picker (S2.9: the two-pane picker is
// replaced by the flat select + the a/b subchoice — deviation 168).
type modelDlg struct {
	sel          *selectModel
	hasSubChoice bool
	pick         string // the "provider/id" the subchoice applies
}

// openModelDialog pushes the model select modal (dlgLarge — upstream
// dialog-model is size="large") and fetches the catalog. A nil sel = the
// pre-catalog loading hint; syncModelSel builds the select once the catalog
// is hydrated (open-time or on arrival).
func (a *App) openModelDialog() []tea.Cmd {
	mdl := &modelDlg{}
	a.pushModal(dialog{kind: dlgModel, model: mdl}, dlgLarge, nil)
	a.syncModelSel()
	return a.emit(a.fetchCatalogCmd())
}

// modelSelectPick is the select's onSelect: enter opens the a/b subchoice
// (yolo pin — the model applies through it, never directly).
func (a *App) modelSelectPick(o selectOption) {
	if d, ok := a.dlg.top(); ok && d.kind == dlgModel && d.model != nil {
		d.model.hasSubChoice = true
		if v, okk := o.value.(string); okk {
			d.model.pick = v
		}
	}
}

// handleKey: the subchoice owns a/b (the locked overlay); esc on the
// subchoice is the S2.2 cancelInner veto; everything else forwards to the
// select (enter re-opens the subchoice via onSelect on a different model).
func (m *modelDlg) handleKey(a *App, k tea.KeyPressMsg) []tea.Cmd {
	if m.hasSubChoice {
		switch {
		case key.Matches(k, choiceThis):
			if a.curSessionID == "" {
				a.toast("no session")
				return nil
			}
			return a.emit(a.patchDlgCmd("model", m.pick, true))
		case key.Matches(k, choiceDef):
			return a.emit(a.patchDlgCmd("model", m.pick, false))
		}
		return nil
	}
	return m.sel.handleKey(a, k)
}

// cancelInner consumes esc while the subchoice is open (S2.2's veto).
func (m *modelDlg) cancelInner(tea.KeyPressMsg) bool {
	if m.hasSubChoice {
		m.hasSubChoice = false
		return true
	}
	return false
}

// view renders the select + the subchoice overlay (the modal stack draws
// the frame; a nil sel — the pre-catalog open — renders the loading hint).
func (m *modelDlg) view(st *store.State, w, h int, th theme.Theme) string {
	if m.sel == nil {
		return title.Render("Model") + "\n" + th.TextMuted().Render("  loading…")
	}
	var b strings.Builder
	b.WriteString(m.sel.view(w, h, th))
	if m.hasSubChoice {
		b.WriteString("\n" + dimWrapped(th, "  [a] this session  [b] set default", w))
	}
	return b.String()
}

// modelOptions flattens the catalog into select options (providers in
// catalog order, their models in the stable sorted order — modelsOf):
// title = the model name, category = the provider name, description = the
// context + cost tail, footer = the provider status, value = "provider/id".
func modelOptions(st *store.State) []selectOption {
	var opts []selectOption
	for _, p := range st.Providers {
		for _, mm := range modelsOf(p) {
			opts = append(opts, selectOption{
				title:       mm.Name,
				description: fmtCtx(mm.Limit.Context) + " ctx  " + usd(mm.Cost.Input) + "/" + usd(mm.Cost.Output),
				footer:      providerStatusText(p.Auth),
				category:    p.Name,
				value:       p.ID + "/" + mm.ID,
			})
		}
	}
	return opts
}

// modelIsCurrentOpt is the select's isCurrent (value = "provider/id"; the
// session model or the config default).
func modelIsCurrentOpt(st *store.State) func(selectOption) bool {
	var ref protocol.ModelRef
	if cur := st.Current; cur != nil && cur.Model != nil {
		ref = *cur.Model
	} else if s, ok := st.Config["model"].(string); ok {
		if pid, mid, ok := splitModelRef(s); ok {
			ref = protocol.ModelRef{ProviderID: pid, ID: mid}
		}
	}
	return func(o selectOption) bool {
		v, _ := o.value.(string)
		return ref.ProviderID != "" && v == ref.ProviderID+"/"+ref.ID
	}
}

// modelSelIndex is the select index of the current model (-1 = none).
func modelSelIndex(st *store.State) int {
	var ref protocol.ModelRef
	if cur := st.Current; cur != nil && cur.Model != nil {
		ref = *cur.Model
	} else if s, ok := st.Config["model"].(string); ok {
		if pid, mid, ok := splitModelRef(s); ok {
			ref = protocol.ModelRef{ProviderID: pid, ID: mid}
		}
	}
	for i, o := range modelOptions(st) {
		if v, _ := o.value.(string); v == ref.ProviderID+"/"+ref.ID {
			return i
		}
	}
	return -1
}
```

Replace `syncModelSel` (the catalog-arrival re-seed):

```go
// syncModelSel (re)builds the open model select on the catalog: it builds
// the select when the catalog first hydrates (nil sel = loading) and
// re-points the selection at the session model (or the config default).
func (a *App) syncModelSel() {
	m := a.dlg.model()
	if m == nil {
		return
	}
	if m.sel == nil {
		if len(a.store.Providers) == 0 {
			return
		}
		m.sel = selectNew("Model", "Search", modelOptions(&a.store),
			modelIsCurrentOpt(&a.store), a.modelSelectPick, nil)
	}
	m.sel.options = modelOptions(&a.store)
	if i := modelSelIndex(&a.store); i >= 0 {
		m.sel.sel = i
	}
}
```

Delete the two-pane machinery: `modelPane`/`paneProviders`/`paneModels`, `dlgTabKey` (check: still used? the model dialog no longer tabs — but the agent dialog also doesn't use it… `dlgTabKey` was model-only → delete), `m.move`/`m.modelCount`/`m.selectedRef`/`m.modelCell`/`modelRow`/`styleSegment` (two-pane row helpers — delete), `m.currentProv`. KEEP `modelsOf`, `modelIsCurrent` (still used? `modelIsCurrent` was the two-pane marker — replaced by modelIsCurrentOpt → delete if unused), `fmtCtx`, `usd`, `splitModelRef`, `providerStatus` (rework):

```go
// providerStatusText is the locked dot + label (the select footer tail).
func providerStatusText(auth *protocol.ProviderAuth) string {
	switch {
	case auth != nil && auth.Status == "loaded":
		return "● loaded"
	case auth != nil && auth.RequiresKey && auth.Status == "missing":
		return "○ missing"
	default:
		return "· not-required"
	}
}

// providerStatus maps the wire auth state to the dot + label style (the
// transcript surfaces; the select tail uses providerStatusText plain).
func providerStatus(th theme.Theme, auth *protocol.ProviderAuth) (string, lipgloss.Style) {
	switch {
	case auth != nil && auth.Status == "loaded":
		return providerStatusText(auth), th.Success()
	case auth != nil && auth.RequiresKey && auth.Status == "missing":
		return providerStatusText(auth), th.Error()
	default:
		return providerStatusText(auth), th.TextMuted()
	}
}
```

`modalInner`'s model case (the view signature gains h — the S2.2-era call site):

```go
	case dlgModel:
		if d.sel != nil {
			return d.sel.view(w, h, a.theme)
		}
		if d.model != nil {
			return d.model.view(&a.store, w, h, a.theme)
		}
```

(`applyCatalog` keeps calling `a.syncModelSel()` + `a.syncAgentSel()` — no change.)

**Step 4 — run to verify it passes, then gate.**
`go test ./internal/tui/ -run 'TestModelDialog|TestTUIModelDialog|TestModelDialogPatchPaths' -count=1` → PASS. FULL gate: `go vet ./... && go test ./...` (the teatest suites: `TestTUIModelDialog`, `TestTUIDialogs`'s model leg, the overflow wrap) + `gofmt -l .` empty. Pin discipline: re-baseline any pin whose token chain is correct but whose string drifted, in this commit.

**Step 5 — commit + close the bead.**
`git add internal/tui/dialog.go internal/tui/model_test.go internal/tui/overflow_test.go internal/tui/tui_suite_test.go && git commit -m "feat: model dialog restyle (on select)"`
`bd close <S2.9 bead> --reason "model flat select green: catalog flatten, filter, ● current, a/b subchoice, SGR/teatest re-baselined" --json`
Log **deviation 168** (medium) in `docs/superpowers/DEVIATIONS.md` (same commit): the model dialog's two-pane picker is replaced by the flat select (spec-approved behavior change; favorites/recents not ported).

---

### Task S2.10: Agent dialog restyle (on select) (bead `yolo-oae.3.10`, expected id `yolo-oae.3.13`)

**Files:** rewrite `internal/tui/dialog.go`'s agentDlg (the list → the select + the a/b subchoice), `openAgentDialog`, `syncAgentSel`; re-baseline `internal/tui/agent_test.go` (all), `internal/tui/overflow_test.go` (`TestAgentDlgViewWraps`), `internal/tui/tui_suite_test.go` (`TestTUIDialogs`'s agent leg, if pinned).

**Interfaces:** consumes S2.5–S2.7 (selectModel), S2.2 (the modal stack, `dlgMedium` — upstream dialog-agent is `size="medium"`), the existing patch flow (unchanged). Produces: `agentDlg{sel *selectModel, hasSubChoice bool, pick string}`, `agentOptions(st *store.State) []selectOption` (title = the agent name, description = the agent description, value = the name, isCurrent = the session/config agent). The locked `[a] this session  [b] set default` overlay and the `agent set: <name>` toast stay byte-identical (yolo pins).

**Upstream parity notes:** `dialog-agent.tsx` — upstream's agent dialog is a plain DialogSelect (name + description rows, no subchoice — upstream applies directly). yolo keeps the a/b subchoice (yolo pin — the session-vs-default scope is a yolo concept). The `*` current marker becomes the select's ● gutter (deviation 168's family — the marker change rides 168's log entry; no new number).

**Step 1 — write the failing tests.** Replace `internal/tui/agent_test.go`'s render/keys tests, keeping the `openAgentAt` harness:

```go
func TestAgentDialogRender(t *testing.T) {
	t.Run("agents flatten into the select, the current one is marked", func(t *testing.T) {
		a := openAgentAt()
		got := stripANSI(a.dlg.agent().view(&a.store, 80, 24, a.theme))
		if !strings.Contains(got, "Agents") {
			t.Fatalf("title missing:\n%s", got)
		}
		if !strings.Contains(got, "●") {
			t.Fatalf("current agent gutter missing:\n%s", got)
		}
		// agentFixture (model_test.go): build, plan, yolo — the session
		// agent "build" carries the ● gutter
		for _, tok := range []string{"build", "plan", "yolo"} {
			if !strings.Contains(got, tok) {
				t.Fatalf("agent %q missing:\n%s", tok, got)
			}
		}
	})

	t.Run("no agents renders the loading hint", func(t *testing.T) {
		a := agentApp()
		a.store.Agents = nil
		a.openAgentDialog()
		a.Cmds = nil
		got := stripANSI(a.dlg.agent().view(&a.store, 80, 24, a.theme))
		if !strings.Contains(got, "loading…") {
			t.Fatalf("loading hint missing:\n%s", got)
		}
	})

	t.Run("filter narrows the list", func(t *testing.T) {
		a := openAgentAt()
		a.handleKey(press('b')) // only "build" matches
		got := stripANSI(a.dlg.agent().view(&a.store, 80, 24, a.theme))
		if !strings.Contains(got, "build") || strings.Contains(got, "yolo ") {
			t.Fatalf("filter did not narrow:\n%s", got)
		}
	})

	t.Run("subchoice line is the locked [a]/[b] overlay", func(t *testing.T) {
		a := openAgentAt()
		a.handleKey(press(tea.KeyEnter))
		got := stripANSI(a.dlg.agent().view(&a.store, 80, 24, a.theme))
		if !strings.Contains(got, "[a] this session  [b] set default") {
			t.Fatalf("subchoice missing:\n%s", got)
		}
	})
}
```

(Re-point the existing keys/apply/teatest tests the same way S2.9 re-pointed the model tests: the two-pane driver steps become select steps (arrows, enter, a/b); the `agent set: <name>` toast pins stay; `TestTUIAgentDialog`'s filter step uses a typed letter; the `hasAgentDialog` matcher keeps its name + the ● gutter token. `openAgentAt` keeps its shape — `a.dlg.agent()` still returns the payload.)

Re-baseline `internal/tui/overflow_test.go`:

```go
func TestAgentDlgViewWraps(t *testing.T) {
	a := openAgentAt()
	long := strings.Repeat("permits tools without prompts ", 6)
	a.store.Agents = []protocol.Agent{{Name: "build", Description: long}}
	got := stripANSI(a.dlg.agent().view(&a.store, 20, 24, a.theme))
	fitsWidth(t, got, 20)
	flat := strings.Join(strings.Fields(rejoined(got)), " ")
	if !strings.Contains(flat, "build") {
		t.Fatalf("agent text lost in wrap:\n%q", got)
	}
}
```

Re-baseline `hasAgentDialog` in `agent_test.go` (the `*` marker becomes the ● gutter):

```go
func hasAgentDialog(b []byte) bool {
	s := stripANSI(string(b))
	return strings.Contains(s, "Agents") &&
		strings.Contains(s, "● build") &&
		strings.Contains(s, "The default agent.") &&
		strings.Contains(s, "yolo") &&
		strings.Contains(s, "Yolo agent. Permits everything")
}
```

**Step 2 — run to verify it fails.**
`go test ./internal/tui/ -run 'TestAgentDialog|TestTUIAgentDialog' -count=1` → FAIL: compile errors (the agentDlg shape) + the old pins.

**Step 3 — write the minimal implementation.** `internal/tui/dialog.go` — replace the agentDlg:

```go
// agentDlg is the ctrl+a / /agents picker (S2.10: the plain list is the
// select + the a/b subchoice — the yolo scope pin; upstream applies
// directly).
type agentDlg struct {
	sel          *selectModel
	hasSubChoice bool
	pick         string
}

// openAgentDialog pushes the agent select modal (dlgMedium — upstream
// dialog-agent is size="medium") and fetches the catalog. A nil sel = the
// pre-catalog loading hint; syncAgentSel builds the select once the catalog
// is hydrated (open-time or on arrival).
func (a *App) openAgentDialog() []tea.Cmd {
	agd := &agentDlg{}
	a.pushModal(dialog{kind: dlgAgents, agent: agd}, dlgMedium, nil)
	a.syncAgentSel()
	return a.emit(a.fetchCatalogCmd())
}

// agentOptions is the select's option list: title = the name, description
// = the agent description, value = the name (the wire value).
func agentOptions(st *store.State) []selectOption {
	var opts []selectOption
	for _, x := range st.Agents {
		opts = append(opts, selectOption{
			title:       x.Name,
			description: x.Description,
			value:       x.Name,
		})
	}
	return opts
}

// agentIsCurrentOpt is the select's isCurrent (the session agent, falling
// back to the config agent).
func agentIsCurrentOpt(st *store.State) func(selectOption) bool {
	name := currentAgentName(st)
	return func(o selectOption) bool {
		v, _ := o.value.(string)
		return name != "" && v == name
	}
}

// agentSelectPick is the select's onSelect: enter opens the a/b subchoice
// (yolo pin).
func (a *App) agentSelectPick(o selectOption) {
	if d, ok := a.dlg.top(); ok && d.kind == dlgAgents && d.agent != nil {
		d.agent.hasSubChoice = true
		if v, okk := o.value.(string); okk {
			d.agent.pick = v
		}
	}
}

// handleKey: the subchoice owns a/b; esc on the subchoice is the S2.2
// veto; everything else forwards to the select.
func (m *agentDlg) handleKey(a *App, k tea.KeyPressMsg) []tea.Cmd {
	if m.hasSubChoice {
		switch {
		case key.Matches(k, choiceThis):
			if a.curSessionID == "" {
				a.toast("no session")
				return nil
			}
			return a.emit(a.patchDlgCmd("agent", m.pick, true))
		case key.Matches(k, choiceDef):
			return a.emit(a.patchDlgCmd("agent", m.pick, false))
		}
		return nil
	}
	return m.sel.handleKey(a, k)
}

// cancelInner consumes esc while the subchoice is open (S2.2's veto).
func (m *agentDlg) cancelInner(tea.KeyPressMsg) bool {
	if m.hasSubChoice {
		m.hasSubChoice = false
		return true
	}
	return false
}

// view renders the select + the subchoice overlay (nil sel = loading).
func (m *agentDlg) view(st *store.State, w, h int, th theme.Theme) string {
	if m.sel == nil {
		return title.Render("Agents") + "\n" + th.TextMuted().Render("  loading…")
	}
	var b strings.Builder
	b.WriteString(m.sel.view(w, h, th))
	if m.hasSubChoice {
		b.WriteString("\n" + dimWrapped(th, "  [a] this session  [b] set default", w))
	}
	return b.String()
}
```

Replace `syncAgentSel` (the catalog-arrival re-seed) and delete `m.selectedName`/the old `m.handleKey`/`m.view`:

```go
// syncAgentSel (re)builds the open agent select on the catalog: it builds
// the select when the catalog first hydrates (nil sel = loading) and
// re-points the selection at the current agent.
func (a *App) syncAgentSel() {
	ag := a.dlg.agent()
	if ag == nil {
		return
	}
	if ag.sel == nil {
		if len(a.store.Agents) == 0 {
			return
		}
		ag.sel = selectNew("Agents", "Search", agentOptions(&a.store),
			agentIsCurrentOpt(&a.store), a.agentSelectPick, nil)
	}
	ag.sel.options = agentOptions(&a.store)
	name := currentAgentName(&a.store)
	for i, x := range a.store.Agents {
		if x.Name == name {
			ag.sel.sel = i
			return
		}
	}
}
```

`modalInner`'s agents case:

```go
	case dlgAgents:
		if d.sel != nil {
			return d.sel.view(w, h, a.theme)
		}
		if d.agent != nil {
			return d.agent.view(&a.store, w, h, a.theme)
		}
```

**Step 4 — run to verify it passes, then gate.**
`go test ./internal/tui/ -run 'TestAgentDialog|TestTUIAgentDialog' -count=1` → PASS. FULL gate: `go vet ./... && go test ./...` + `gofmt -l .` empty. (This is the last S2 task — the full suite includes every S2 surface: the modal stack, the huh units, the select units, the permission/model/agent teatest flows.)

**Step 5 — commit + close the bead, then run the S2 slice gate.**
`git add internal/tui/dialog.go internal/tui/agent_test.go internal/tui/overflow_test.go internal/tui/tui_suite_test.go && git commit -m "feat: agent dialog restyle (on select)"`
`bd close <S2.10 bead> --reason "agent select green: flatten, filter, ● current, a/b subchoice, re-baselined" --json`
Then the **S2 slice gate** (the stub's gate section): module gate green; user-run TTY smoke (cycle the permission/model/agent dialogs + one select: filter, categories, footer hints, scroll acceleration); the deviation entries 165–170 are all in `DEVIATIONS.md` (165 info bead-id shift, 166 low backdrop, 167 low huh look, 169 low perm meta, 168 medium model flat, 170 info scroll ±10); `PROGRESS.md` one-line status pointer; commit `docs: checkpoint — S2 done, next is S3 detail pass`; `bd close yolo-oae.3 --reason "all 10 child beads closed, gate green" --json`.

## S2 slice gate (slice bead `yolo-oae.3`)

NOT a task bead; runs after all child beads close. Mirror the S0 slice gate
shape: (1) module gate `go vet ./... && go test ./...` + `gofmt -l .` empty
(incl. `TestImportsDirection` + the S2 teatest goldens); (2) user-run smoke
(NOT CI): in a real TTY, cycle the permission/model/agent dialogs and one
select (filter, categories, footer hints, scroll acceleration); (3) append
any forced DEVIATIONS.md entries this slice named (with severity,
same-commit rule — root principle 2; spec §9 risk 2: huh's look deviates
from the upstream borderless pills, so per-dialog deviations are expected
and unrecognizable dialogs get hand-rolled versions, logged); (4)
PROGRESS.md one-line status pointer; (5) commit
`docs: checkpoint — S2 done, next is S3 detail pass`; (6)
`bd close yolo-oae.3 --reason "all 10 child beads closed, gate green" --json`.
