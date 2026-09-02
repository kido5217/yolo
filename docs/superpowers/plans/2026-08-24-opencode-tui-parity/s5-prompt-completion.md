# S5 — prompt completion + attention (slice bead `yolo-oae.6`)

Complete the prompt: ↑/↓ history recall with KV persistence, the ported
frecency ranking, the @-file and /-command autocomplete pickers, and the
terminal bell on turn completion/error.

**State: fully detailed** — the 5-step TDD detail for all 6 tasks is in the
`## S5 detail` section below (Slice Detail Protocol rule 2); execution may
start at task S5.1.

## Binding task table

Pointer only (FROZEN — Slice Detail Protocol rule 1): `plan.md` →
`## Task inventory` → `### S5 — prompt completion + attention (slice bead yolo-oae.6)`.
Bead titles, scope, and pinned commit messages live there and may not be
changed by a detail pass; any required change = STOP + explicit user
approval + re-record in plan.md.

## Dep gate

None — the @-picker's fuzzy filter reuses `sahilm/fuzzy` from the S2 gate.

## Exact upstream sources (the detail pass reads these AT DETAIL TIME)

`/tmp/opencode-upstream` @ v1.18.18:

- `packages/tui/src/component/prompt/index.tsx` — the prompt view: history
  wiring, autocomplete trigger, cursor (S0.10 already themed the cursor;
  read for the wiring only).
- `packages/tui/src/component/prompt/history.tsx` +
  `packages/tui/src/prompt/history.tsx` — recall + persistence
  (S5.1/S5.2).
- `packages/tui/src/component/prompt/frecency.tsx` +
  `packages/tui/src/prompt/frecency.tsx` — the scoring algorithm (S5.3 —
  port the scoring verbatim).
- `packages/tui/src/component/prompt/autocomplete.tsx` — the @-file +
  /-command pickers (S5.4/S5.5; the /-picker reads GET /command; spec §9
  open item: check here whether the file walk honors `.gitignore`).
- `packages/tui/src/feature-plugins/system/notifications.ts` — the bell
  conditions (S5.6 — port the CONDITIONS, not the audio).

## yolo anchors

- `internal/tui/prompt.go` — the prompt view (exists; verified 2026-08-25)
  + its tests.
- `internal/tui/theme/` — the S0.7 KV file: the prompt history persistence
  target.
- `internal/protocol/` — GET /command for the /-picker.
- workspace file access for the @-picker — read-only, path-validated (no
  new dep; `internal/tui` stays import-pure — root principle 4).

## Detail pass (protocol)

One writing-plans pass, one subagent, `thinking=high`, dispatched by the
root session strictly sequentially (root principle 7) — per the Slice
Detail Protocol in plan.md: it fills this file (after this section) with the
full 5-step TDD detail for each task in the binding table (failing test
code, implementation code, gate, pinned commit), reading the named upstream
files at that moment. It commits as
`docs: TUI parity plan — detail S5 tasks` on its own bead
(`bd create "detail S5 plan tasks" --parent=yolo-oae.6 --json`).

## S5 detail

Detail pass 2026-09-02. Deviations tail at detail time = 220; S5 entries
start at 221. Breadcrumb note (DEVIATIONS.md entry 221, severity info): the
frozen S5 table names the task beads `yolo-oae.6.1`–`6.6`, but the S5 detail
bead consumed `yolo-oae.6.1` (created + claimed before the detail pass; the
S1 "detail-bead-last" precedent is impossible because the detail pass
precedes slice start, as in S2/dev 165, S3/dev 188, S4/dev 206). The 6 task
beads therefore land in table order at `yolo-oae.6.2`–`yolo-oae.6.7`
(S5.1→.2, S5.2→.3, S5.3→.4, S5.4→.5, S5.5→.6, S5.6→.7); the frozen titles and
pinned commit messages are unchanged. No code or wire impact.

### Detail-pass findings (read AT DETAIL TIME, 2026-09-02 — binding)

1. **Upstream history** (`packages/tui/src/prompt/history.tsx` +
   `component/prompt/index.tsx` + `config/keybind.ts` @ v1.18.18):
   - `PromptInfo = {input, mode?, parts[]}`; `MAX_HISTORY_ENTRIES = 50`.
   - `parsePromptHistory(text)`: split `"\n"` → filter Boolean → `JSON.parse`
     each → `slice(-50)` (most-recent LAST).
   - `isDuplicateEntry(prev, next)`: `JSON.stringify` equality.
   - `move(direction: 1|-1, input)`: the guard `current.input !== input &&
     input.length` → no-op (a recall is aborted when the input was edited away
     from the last-recalled text and is non-empty); `next = index + direction`;
     `Math.abs(next) > history.length || next > 0` → no-op (beyond the oldest /
     beyond present). `index` runs 0 (present) … -len (oldest).
   - `append(item)`: a duplicate of the newest entry resets the index to 0 and
     does NOT re-add; otherwise push; `> 50` → keep the last 50.
   - Persistence: `prompt-history.jsonl` under `paths.state` (one JSON per
     line; rewritten on trim).
   - `keybind.ts:199-200`: `history_previous: "up"`, `history_next: "down"`;
     `DRAFT_RETENTION_MIN_CHARS = 20` (index.tsx:104); `history.append` after
     every send (index.tsx:1122) and on `clearPrompt` when
     `input.trim().length >= 20` or the prompt has parts (index.tsx:1273-1278).
2. **Upstream frecency** (`packages/tui/src/prompt/frecency.tsx`):
   `FrecencyEntry = {path, frequency, lastOpen}`; `MAX_FRECENCY_ENTRIES = 1000`;
   `calculateFrecency(entry?)` = `frequency / (1 + (Date.now() - lastOpen) /
   86_400_000)` (absent → 0); `parseFrecency` dedupes by path (last wins),
   sorts `lastOpen` desc, slices 1000; `updateFrecency(filePath)` = the
   cwd-resolved absolute path, `frequency+1`, `lastOpen=now`, refresh-or-append,
   then the parse (dedupe + sort + cap); persistence `frecency.jsonl`.
3. **Upstream @-/slash pickers** (`component/prompt/autocomplete.tsx` +
   `prompt/display.ts`): the @-trigger = `mentionTriggerIndex` (display.ts:38-47)
   — the last `@` at the start or preceded by whitespace whose following text
   has no whitespace; the @-files come from `sdk.client.v2.fs.find({query,
   limit:"20", location})` (the FFF server search — **NO yolo referent**,
   frozen wire); the /-commands come from `sync.data.command` (GET /command —
   yolo has this: `store.Commands` + `localCommands`); the non-file fuzzy is a
   `sahilm/fuzzy`-equivalent (threshold 0 for `/`, 0.5 for `@`), `limit 10`,
   `scoreFn` = the fuzzy score ×2 for a prefix match × `(1 + frecency)`;
   selection: an @-file → `insertPart` + `frecency.updateFrecency(path)`; a
   /-command → the input is replaced with `/<name> `.
4. **Upstream bell** (`feature-plugins/system/notifications.ts`): the
   conditions — `question.asked` → "Question needs input" (dedup by id);
   `permission.asked` → "Permission needs input" (dedup by id); `session.status`
   busy|retry → the active set, clear errored; `session.status` idle →
   "Session done" only when it was active and not errored; `session.error`
   while active → errored + the error bell (MessageAbortedError → "Session
   aborted"; the SSE-timeout → "Model stopped responding"; else "Session
   error").
5. **spec §9 open item (ANSWERED):** whether the @-file walk honors
   `.gitignore` — **YES**. Upstream's file search uses the FFF git-aware Rust
   scanner (search.ts:128-133, 213) with a ripgrep fallback (search.ts:35-48,
   the ripgrep default-ignore, no `--no-ignore`) — BOTH paths honor
   `.gitignore`.
6. **yolo surface (verified at detail time):**
   - `internal/tui/prompt.go`: `promptModel{input textinput.Model, sel int,
     draft strings.Builder}`, `slashActive()`, `menuItems(cmds)`,
     `menuView(cmds, w, th)` (renders the items it is passed), `moveMenuSel(n, d)`
     (wraparound), `commandAliases{"/quit": {"/exit"}}`.
   - `internal/tui/keys.go`: `handleKey` ladder (permission > dialog > the S4.2
     registry > slash menu (`slashActive` → `handleMenuKey`) > route >
     `handlePromptKey`); `handleMenuKey` (up/down via `homeKeyMap`, enter
     executes via `runCommand`, esc + enter-on-no-match clear the input);
     `handlePromptKey` (enter → `promptEnter`, else `inputUpdate`) — **the
     up/down hook site** (the session route reaches it; `handleSessionKey`
     handles only pgup/pgdn, alt+e, alt+t, ctrl+r, esc — NOT up/down; the home
     route's up/down is consumed earlier by `handleHomeKey`, home.go:330-335).
   - `internal/tui/commands.go`: `sendMessageCmd(text)` + `sendMsg{err}` +
     `applySend` (success: clear input + draft + the S3.7 `retrySuppressed`
     clear) — **the append hook site**; `mergedCommands()` (locals first, then
     `store.Commands`); `runCommand` (execute-on-enter — LOCKED; `/new` issues
     CreateSession); `localCommands()`.
   - `internal/tui/store/store.go`: `State.Status protocol.SessionStatus` (zero
     = idle, current-session-only); `applySessionStatus` (current-scoped); the
     message family is current-scoped.
   - `internal/protocol`: `Event{ID, Type, Properties}`; the event set
     (message.updated, session.status, permission.asked, …) — **NO
     `question.*`, NO `session.error`** (the deviation-227 referent gaps);
     `SessionStatus{Type "idle"|"busy"|"retry", Attempt, Message, Next}` + the
     consts `SessionStatusIdle/Busy/Retry`; `SessionStatusProps{SessionID,
     Status}`; `MessageError{Type "unknown"|"aborted"|"overflow", Message}`,
     `Message.Error *MessageError` (message.go:38); `MessageUpdatedProps
     {SessionID, Info Message}`; `PermissionAskedProps{ID, SessionID,
     Permission, …}`; `Command{Name, Description, Template, Hints}`.
   - `internal/tui/client/client.go`: `Service{BaseURL, Dir string, HC,
     Backoff}`, `New(base, dir)`; `Dir` = the scope dir (abs; `""` = the server
     work dir) — `a.Dir` (promoted) is the @-walk root; `ListCommands(ctx)`.
   - `internal/tui/theme`: `KV{Get(key, def) any, Set(key, val), Flush(),
     Close()}` (kv.go — the in-memory store is the source of truth; a single
     writer goroutine; the queue is never closed); `Engine` wraps `kv *KV`
     (engine.go:37) with the public seams `KVPath()` (389) / `FlushKV()` (396) /
     `Close()` (402) — **NO generic `KV() *KV` seam yet** (S5.2 adds it). A KV
     value survives a process restart as JSON: a Go `[]string` reloads as `[]any`
     of `string` (the coerce helper below).
   - `internal/tui/view.go`: the slash menu is the single overlay `menu :=
     a.prompt.menuView(a.mergedCommands(), w, a.theme)` (view.go:33), rendered
     below the prompt (view.go:41-43).
   - Test harness: `testApp(sessions...) *recApp` (home_test.go:29 — the dummy
     client `client.New("http://127.0.0.1:9","")`, Dir `""`, NIL engine);
     `newRecApp(c, s, startSessionID)` (rec_test.go:20 — nil engine);
     `themeApp(t) (*recApp, *theme.Engine)` (themedlg_test.go:20 — a REAL engine
     over `t.TempDir`, `client.New(..., "")`); `recApp{*App, Cmds []tea.Cmd}`;
     `press(r)` (home_test.go:36 — Up/Down/Enter/Esc/Left/Right/Backspace
     Code-only), `typeStr`, `hasToast`, `stripANSI`, `testNow`, `ctrlCKey`;
     `hasLine`/`hasLines`/`suiteType` (tui_suite_test.go); `testutil.Boot(t)` /
     `testutil.BootWithDriver(t, drv)` / `testutil.BootWithDriverConfig(t, drv,
     cfg)` → `*testutil.TestServer{URL, Dir}`; `fake.New(...)`.
   - bubbles v2.2.1 `textinput.Model`: `SetValue(s)`, `Value()`, `CursorEnd()`,
     `SetCursor(pos)`, `Position()`, `Focus()`, `View()`. bubbletea v2.0.9:
     `tea.Raw(r any) tea.Cmd` (raw.go:35 — "prints the given string to the
     terminal without any formatting"; the execute path, alt-screen-independent,
     and its bytes land in `teatest` `tm.Output()`) — `tea.Printf` is
     alt-screen-inert (**NOT** the bell seam). `sahilm/fuzzy v0.1.3`: `Find
     (pattern string, data []string) Matches`, `Matches = []Match{Str string,
     Index int, MatchedIndexes []int, Score int}` (best-first).
7. **spec §6/§9:** the Prompt line — history (↑/↓ recall, persisted in the KV
   file), frecency ranking (ported scoring, persisted), @-autocomplete (`@` →
   fuzzy picker over local files, `/` → slash commands, sahilm/fuzzy,
   TUI-local), `\`+enter newline preserved; the Attention line — the terminal
   bell on turn completion / error (ported `notifications.ts` conditions). §9
   open item: the `.gitignore` question → answered above (upstream honors it).

### Design decisions (binding)

**Shared prompt-completion state (the `App` fields, land in S5.1):**
`hist []string` (the prompt history, most-recent LAST; S5.1 in-memory, S5.2
KV-persisted), `histIdx int` (0 = present, -1 = newest, -len = oldest),
`histText string` (the text last set by a recall — the dirty guard),
`histOrig string` (the draft captured at the first up-press, restored at
present). S5.3 adds `freq []frecencyEntry`; S5.4 adds `walkRoot string` +
`walked []string` (the cached @-picker walk); S5.6 adds `attention
attentionState`. Constants land with each task: `maxHistoryEntries = 50` +
`draftRetentionMin = 20` (S5.1), `maxFrecencyEntries = 1000` + `dayMs =
86_400_000` (S5.3), `maxWalkFiles = 1000` + `maxWalkDepth = 8` +
`maxPickerOptions = 10` (S5.4).

**S5.1 (recall):** the history is GLOBAL (on `App`, not per-session) and is a
**text-only entry** (deviation 222 — yolo's prompt is single-line text with no
parts/mode, so the entry is the composed string; the upstream
`PromptInfo{input, mode, parts}` collapses to `input`). Recall (up/down) is a
**session-route prompt behavior**: it is hooked in `handlePromptKey` (the home
route's up/down stays home-list navigation — `handleHomeKey`, deviation-210
class; the slash menu / @-picker own up/down while open, so recall only runs
when both are closed). `recallHistory(dir)` ports the upstream `move` guard
verbatim (a recall is aborted when the input was edited away from the
last-recalled text and is non-empty; `index` runs 0 … -len).
`appendHistory(text)` is called on a successful send (`applySend` — `sendMsg`
gains a `text` field) and on a prompt clear that retains a ≥20-char draft
(`clearPrompt`, the `DRAFT_RETENTION_MIN_CHARS` port); it dedupes the newest
entry (a duplicate resets to present without re-adding) and caps at 50.
`clearPrompt()` (the new helper) routes the prompt-clear sites (the home esc,
the slash-menu esc + enter-on-no-match) so a long unsent draft is retained;
`runCommand` keeps its own `SetValue("")` (a command is not prompt history).
The in-memory list starts empty each boot (the KV load is S5.2).

**S5.2 (KV persistence):** the history persists in the **theme KV file** under
the key `prompt_history` (deviation 223 — the KV is the sanctioned TUI-local
persistence surface; the upstream `prompt-history.jsonl` file is not
introduced). `Engine.KV() *KV` is the additive seam (S5.2); `loadHistory()`
(in `NewApp`, after `retheme`) reads the key and **coerces** the value — a Go
`[]string` in-run or a JSON `[]any` after a process-restart reload (the
`coerceStrings` helper, deviation 223); `saveHistory()` (called from
`appendHistory`) writes the current list. A nil engine (the `testApp` runs)
skips both.

**S5.3 (frecency):** the subsystem is pure functions over `[]frecencyEntry`
(`frecencyEntry{Path, Frequency, LastOpen}`) — `frecencyScore(e, now)
float64` (the ported `frequency / (1 + age-days)`), `parseFrecency` (dedupe by
path last-wins + sort lastOpen desc + cap 1000), `updateFrecency(entries,
relPath, now)` (refresh-or-append, then parse). The keys are **scope-relative
paths** (deviation 224 — the upstream cwd-resolved absolute path; the relative
form keeps the KV portable within the scope dir). Persisted in the KV under
`prompt_frecency` (deviation 223); `loadFrecency()` (in `NewApp`) coerces the
reloaded value (`coerceFrecency`); `saveFrecency()` is called from the
@-picker's selection (S5.4). The ranking is consumed by the @-picker (S5.4).

**S5.4 (@-picker):** the @-file picker is a prompt-attached overlay (the
slash-menu pattern) shown when the input has an active @-trigger
(`mentionTriggerIndex` — the ported upstream `display.ts` rule). It is
**TUI-local** (deviation 225 — the upstream `sdk.client.v2.fs.find` has no yolo
referent, frozen wire): `walkFiles(root)` walks the scope dir (`a.Dir`)
honoring a **static ignore set** (`.git`, `node_modules`, …) + a **minimal
`.gitignore` parse** (the non-comment lines, a trailing `/` stripped; a path is
ignored when a pattern equals one of its path segments — the full gitignore
semantics are NOT ported; spec §9 confirmed upstream honors `.gitignore`, so
the minimal parse is a sanctioned reduction). The walk is capped (`maxWalkFiles
= 1000`, `maxWalkDepth = 8`) and cached (re-walked only when the scope dir
changes). The options are `fuzzy.Find` over the walked files by the @-query,
each score ×2 for a prefix match × `(1 + frecencyScore)` (the upstream
`scoreFn`), sorted desc, capped at `maxPickerOptions = 10`; an empty query
lists all (frecency-ranked). Selection (enter) inserts the path by
**replacing the @-query with the path text** (deviation-222 class — yolo has
no file "parts"/chips; the path is plain text) + `updateFrecency` +
`saveFrecency`. esc removes the @-trigger (keeping the prefix); other keys feed
the input (re-filtering). The view reuses the slash-menu rendering (muted +
`cursorStyle` + `wrapLine`).

**S5.5 (/-picker):** the existing slash menu (`menuItems`, a `promptModel`
prefix+alias filter) is **upgraded to a fuzzy-ranked picker** (the ported
upstream /-autocomplete): `menuItems` becomes an `App` method returning the
fuzzy-ranked merged commands — `fuzzy.Find` over the canonical + alias names,
×2 for a prefix match, capped at `maxPickerOptions = 10`; an **empty query
returns the merged list in order** (deviation 226 — the upstream `/` lists all
commands; the order is preserved). Execute-on-enter is **preserved** (LOCKED —
`handleMenuKey` + `runCommand` unchanged; selecting a row runs the command, the
input clears, the same as today). The alias handling is preserved (a match on
an alias maps to the canonical command, deduped by name). `menuView` stays a
`promptModel` render method (it renders the items it is passed); the
`TestPromptMenuFilter` table re-baselines to the fuzzy behavior.

**S5.6 (bell):** the terminal bell is `bell() = tea.Raw("\a")` (the
alt-screen-independent execute path; `tea.Printf` is inert on the alt screen).
The conditions are ported from `notifications.ts` to the **current session**
(deviation 227): (a) a `permission.asked` for the current session → bell
(deduped by the ask id); (b) `session.status` busy|retry → active, clear
errored (no bell); (c) `session.status` idle → bell ONLY when it was active and
not errored (the "done" bell); (d) a current `message.updated` carrying a
non-nil `Message.Error` → errored + bell (the turn-error bell; the upstream
`session.error` has no yolo referent — the current message's `Error` is the
referent, and the `aborted`/error message distinction is dropped because the
yolo bell is terminal-only with no message). **DROPPED (no yolo referent,
deviation 227):** the `question.asked` condition (yolo has no question dialog —
deferred, spec §1) and the SSE-timeout "Model stopped responding" condition.
The hook `onAttention(ev) tea.Cmd` runs in the `EventMsg` case (after
`store.Apply`) and its cmd is batched into the applied event's cmd.

### Task S5.1: Prompt history: ↑/↓ recall + tests (bead `yolo-oae.6.1`, expected id `yolo-oae.6.2`)

**Files:** modify `internal/tui/prompt.go` (`recallHistory`/`historyTextAt`/`appendHistory`/`clearPrompt` + `maxHistoryEntries`/`draftRetentionMin`), `internal/tui/keys.go` (`handlePromptKey` up/down hook), `internal/tui/commands.go` (`sendMsg.text` + `applySend` append + the `clearPrompt` sites), `internal/tui/home.go` (the home esc → `clearPrompt`), `internal/tui/app.go` (the `hist`/`histIdx`/`histText`/`histOrig` fields); new `internal/tui/prompt_hist_test.go`.

**Interfaces:** produces `App.hist []string`, `App.histIdx int`, `App.histText string`, `App.histOrig string`; `App.recallHistory(dir int)` (dir -1 = up/older, +1 = down/newer), `App.historyTextAt(i int) string`, `App.appendHistory(text string)`, `App.clearPrompt()`; the `maxHistoryEntries = 50` + `draftRetentionMin = 20` consts; `sendMsg{text string; err error}` (the additive `text` field — the zero value is today's behavior). The `handlePromptKey` up/down hook + the `applySend`/`clearPrompt` append sites are internal (no new exported surface).

**Upstream parity notes:** history.tsx — the recall guard + index (0 … -len), dedupe-newest, cap 50, `DRAFT_RETENTION_MIN_CHARS = 20`; keybind.ts up/down; the append on send + on clearPrompt. Deviation 222 (text-only entry). The home-route up/down stays home-list nav (deviation 210 class).

**Step 1 — write the failing tests.** New `internal/tui/prompt_hist_test.go`:

```go
package tui

import (
	"fmt"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
)

func TestRecallHistory(t *testing.T) {
	t.Run("up walks newest-first, down restores the draft", func(t *testing.T) {
		a := testApp()
		a.route = routeSession
		a.hist = []string{"a", "b", "c"} // c is the newest
		a.prompt.input.SetValue("draft")
		a.recallHistory(-1)
		if got := a.prompt.input.Value(); got != "c" {
			t.Fatalf("first up = %q, want c (newest)", got)
		}
		a.recallHistory(-1)
		if got := a.prompt.input.Value(); got != "b" {
			t.Fatalf("second up = %q, want b", got)
		}
		a.recallHistory(-1)
		if got := a.prompt.input.Value(); got != "a" {
			t.Fatalf("third up = %q, want a (oldest)", got)
		}
		a.recallHistory(-1) // beyond the oldest: no-op
		if got := a.prompt.input.Value(); got != "a" {
			t.Fatalf("fourth up must clamp at oldest, got %q", got)
		}
		a.recallHistory(1)
		if got := a.prompt.input.Value(); got != "b" {
			t.Fatalf("down = %q, want b", got)
		}
		a.recallHistory(1) // to present
		if got := a.prompt.input.Value(); got != "draft" {
			t.Fatalf("down to present = %q, want the captured draft", got)
		}
		a.recallHistory(1) // beyond present: no-op
		if got := a.prompt.input.Value(); got != "draft" {
			t.Fatalf("down beyond present must clamp, got %q", got)
		}
	})

	t.Run("dirty guard: an edited recall aborts nav", func(t *testing.T) {
		a := testApp()
		a.route = routeSession
		a.hist = []string{"a", "b"}
		a.prompt.input.SetValue("c")
		a.recallHistory(-1) // up -> b
		a.prompt.input.SetValue("b2") // the user edits
		a.recallHistory(-1) // dirty -> no-op
		if got := a.prompt.input.Value(); got != "b2" {
			t.Fatalf("dirty nav must not move, got %q", got)
		}
	})

	t.Run("empty history is a no-op", func(t *testing.T) {
		a := testApp()
		a.prompt.input.SetValue("x")
		a.recallHistory(-1)
		if got := a.prompt.input.Value(); got != "x" {
			t.Fatalf("empty history must not change the input, got %q", got)
		}
	})
}

func TestAppendHistory(t *testing.T) {
	t.Run("dedupes the newest and caps at 50", func(t *testing.T) {
		a := testApp()
		a.appendHistory("x")
		a.appendHistory("x") // duplicate: no add
		if len(a.hist) != 1 {
			t.Fatalf("dedupe: %d entries, want 1", len(a.hist))
		}
		for i := 0; i < 55; i++ {
			a.appendHistory(fmt.Sprintf("e%d", i))
		}
		if len(a.hist) != 50 {
			t.Fatalf("cap: %d entries, want 50", len(a.hist))
		}
		if a.hist[0] != "e6" || a.hist[49] != "e54" {
			t.Fatalf("cap dropped the wrong end: first=%q last=%q", a.hist[0], a.hist[49])
		}
	})
}

func TestPromptHistoryKey(t *testing.T) {
	t.Run("up/down on the session route recall (menu closed)", func(t *testing.T) {
		a := testApp()
		a.route = routeSession
		a.hist = []string{"one", "two"}
		a.handleKey(press(tea.KeyUp))
		if got := a.prompt.input.Value(); got != "two" {
			t.Fatalf("up = %q, want two (newest)", got)
		}
		a.handleKey(press(tea.KeyDown))
		if got := a.prompt.input.Value(); got != "" {
			t.Fatalf("down to present = %q, want empty (no draft captured)", got)
		}
	})
}

// TestTUIPromptHistoryRecall is the teatest leg: the real stack, a real
// session, the pre-seeded history is recalled through the real key pipeline.
func TestTUIPromptHistoryRecall(t *testing.T) {
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))
	teatest.WaitFor(t, tm.Output(), hasLine("New session"), teatest.WithDuration(5*time.Second))
	tm.Send(press('n'))
	teatest.WaitFor(t, tm.Output(), hasLine("esc abort/back"), teatest.WithDuration(5*time.Second))
	a.hist = []string{"alpha bravo"} // pre-seed the live app's history
	tm.Send(press(tea.KeyUp))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return a.prompt.input.Value() == "alpha bravo"
	}, teatest.WithDuration(5*time.Second))
	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
```

**Step 2 — confirm FAIL.** `go test ./internal/tui/ -run 'TestRecallHistory|TestAppendHistory|TestPromptHistoryKey|TestTUIPromptHistoryRecall' -count=1` → FAIL (build fails: undefined `a.hist`, `recallHistory`, `appendHistory`, `clearPrompt`, `sendMsg.text` — the expected red).

**Step 3 — minimal implementation.**
- `app.go`: add the `hist []string`, `histIdx int`, `histText string`, `histOrig string` fields to `App`.
- `prompt.go`: add `maxHistoryEntries = 50` + `draftRetentionMin = 20`; `App.recallHistory(dir int)` (the ported guard + `index` 0 … -len, capturing `histOrig` on the first up-press), `App.historyTextAt(i int)` (present → `histOrig`, else `hist[len(hist)+i]`), `App.appendHistory(text string)` (the trim-empty guard, the dedupe-newest reset, the cap 50; resets `histIdx`/`histText`), `App.clearPrompt()` (retain a ≥`draftRetentionMin`-char draft via `appendHistory`, then clear the input + reset `histIdx`/`histText`).
- `keys.go`: `handlePromptKey` — before the enter check, `if key.Matches(k, homeKeyMap.Up) { a.recallHistory(-1); return nil }` + `if key.Matches(k, homeKeyMap.Down) { a.recallHistory(1); return nil }`.
- `commands.go`: `sendMsg` gains a `text string` field; `sendMessageCmd` returns `sendMsg{text: text, err: err}`; `applySend` success path calls `a.appendHistory(m.text)` (after the input/draft clear).
- `home.go` + `keys.go`: the prompt-clear sites (the home esc `handleHomeKey`, the slash-menu esc + enter-on-no-match `handleMenuKey`) call `a.clearPrompt()` instead of `a.prompt.input.SetValue("")`.

**Step 4 — gate.** `go test ./internal/tui/ -run 'TestRecallHistory|TestAppendHistory|TestPromptHistoryKey|TestTUIPromptHistoryRecall' -count=1` → PASS, then FULL gate: `go vet ./... && go test ./...` + `gofmt -l .` empty. (The existing slash-menu + dialog teatest suites must stay green: the up/down hook is session-route-only and the `clearPrompt` sites preserve the existing clear behavior.)

**Step 5 — commit + close the bead.**
`git add internal/tui/prompt.go internal/tui/keys.go internal/tui/commands.go internal/tui/home.go internal/tui/app.go internal/tui/prompt_hist_test.go && git commit -m "feat: prompt history - up/down recall"`
`bd close yolo-oae.6.2 --reason "prompt history recall green: up/down nav, dirty guard, dedupe+cap, send/clear append, teatest" --json`

---

### Task S5.2: Prompt history: persistence in KV (dedupe, cap) + tests (bead `yolo-oae.6.2`, expected id `yolo-oae.6.3`)

**Files:** modify `internal/tui/theme/engine.go` (the additive `KV() *KV` seam), `internal/tui/app.go` (the `kvHistoryKey` const + `loadHistory`/`saveHistory` + `coerceStrings` + the `NewApp` `loadHistory` call), `internal/tui/prompt.go` (`appendHistory` calls `saveHistory`); new `internal/tui/prompt_kv_test.go`.

**Interfaces:** produces `theme.Engine.KV() *KV` (the additive seam — the engine stays the owner), `App.loadHistory()` + `App.saveHistory()`, the `coerceStrings(v any) []string` helper, the `kvHistoryKey = "prompt_history"` const. The `appendHistory` persistence hook is internal.

**Upstream parity notes:** history.tsx persistence (`prompt-history.jsonl`) → the KV key (deviation 223). The reload coerce (a JSON `[]any` after a restart) is the yolo KV surface's behavior (deviation 223).

**Step 1 — write the failing tests.** New `internal/tui/prompt_kv_test.go`:

```go
package tui

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// historyEngine wires a fresh engine over a shared KV path (the restart
// round-trip needs two engines on the same file).
func historyEngine(t *testing.T, kvPath string) (*recApp, *theme.Engine) {
	t.Helper()
	dir := filepath.Dir(kvPath)
	e, err := theme.New(theme.EngineOptions{
		KVPath:        kvPath,
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
	return ra, e
}

func TestPromptHistoryKVPersistence(t *testing.T) {
	t.Run("append persists and reloads across a restart", func(t *testing.T) {
		dir := t.TempDir()
		kvPath := filepath.Join(dir, "kv.json")
		ra, e := historyEngine(t, kvPath)
		ra.appendHistory("one")
		ra.appendHistory("two")
		_ = e.Close() // final flush + the writer stops
		ra2, _ := historyEngine(t, kvPath) // a fresh engine on the SAME file
		if len(ra2.hist) != 2 || ra2.hist[0] != "one" || ra2.hist[1] != "two" {
			t.Fatalf("reload across restart: %v (want [one two])", ra2.hist)
		}
	})

	t.Run("nil engine: the history stays empty in-memory", func(t *testing.T) {
		a := testApp() // nil engine
		a.appendHistory("x")
		if len(a.hist) != 1 || a.hist[0] != "x" {
			t.Fatalf("in-memory append still works: %v", a.hist)
		}
	})
}
```

**Step 2 — confirm FAIL.** `go test ./internal/tui/ -run 'TestPromptHistoryKVPersistence' -count=1` → FAIL (build fails: undefined `e.KV()`, `a.loadHistory`, `saveHistory`, `kvHistoryKey` — the expected red).

**Step 3 — minimal implementation.**
- `theme/engine.go`: `func (e *Engine) KV() *KV { return e.kv }` (the additive seam, beside `KVPath`).
- `app.go`: `const kvHistoryKey = "prompt_history"`; `func coerceStrings(v any) []string` (handles `[]string` and `[]any` of `string`); `func (a *App) loadHistory()` (nil-engine guard; `a.hist = coerceStrings(a.engine.KV().Get(kvHistoryKey, nil))`); `func (a *App) saveHistory()` (nil-engine guard; `a.engine.KV().Set(kvHistoryKey, a.hist)`); the `NewApp` end calls `a.loadHistory()` (after `retheme`).
- `prompt.go`: `appendHistory` calls `a.saveHistory()` at the end.

**Step 4 — gate.** `go test ./internal/tui/ -run 'TestPromptHistoryKVPersistence' -count=1` → PASS, then FULL gate: `go vet ./... && go test ./...` + `gofmt -l .` empty. (The `theme` package gate: `go test ./internal/tui/theme/` — the `KV()` seam is additive; the existing KV tests stay green.)

**Step 5 — commit + close the bead.**
`git add internal/tui/theme/engine.go internal/tui/app.go internal/tui/prompt.go internal/tui/prompt_kv_test.go && git commit -m "feat: prompt history - KV persistence"`
`bd close yolo-oae.6.3 --reason "prompt history KV persistence green: KV seam, load/save, restart reload + coerce" --json`

---

### Task S5.3: Frecency-ranked recall (ported scoring, persisted) + tests (bead `yolo-oae.6.3`, expected id `yolo-oae.6.4`)

**Files:** new `internal/tui/frecency.go` (the `frecencyEntry` + `maxFrecencyEntries`/`dayMs` + `frecencyScore`/`parseFrecency`/`updateFrecency`), modify `internal/tui/app.go` (the `freq` field + `kvFrecencyKey` + `loadFrecency`/`saveFrecency` + `coerceFrecency` + the `NewApp` `loadFrecency` call); new `internal/tui/frecency_test.go`.

**Interfaces:** produces `frecencyEntry{Path string, Frequency int, LastOpen int64}`, `frecencyScore(e *frecencyEntry, now int64) float64`, `parseFrecency(entries []frecencyEntry) []frecencyEntry`, `updateFrecency(entries []frecencyEntry, relPath string, now int64) []frecencyEntry`, `coerceFrecency(v any) []frecencyEntry`, `App.loadFrecency()` + `App.saveFrecency()`, the `maxFrecencyEntries = 1000` + `dayMs = 86_400_000` + `kvFrecencyKey = "prompt_frecency"` consts, `App.freq []frecencyEntry`. The @-picker consumes these (S5.4).

**Upstream parity notes:** frecency.tsx — the scoring, the parse (dedupe last-wins + sort lastOpen desc + cap 1000), the update (refresh-or-append + parse). Deviation 224 (scope-relative keys), 223 (KV persistence).

**Step 1 — write the failing tests.** New `internal/tui/frecency_test.go`:

```go
package tui

import (
	"fmt"
	"math"
	"testing"
)

func TestFrecencyScore(t *testing.T) {
	now := int64(1_000_000_000_000)
	t.Run("score = frequency / (1 + age-days)", func(t *testing.T) {
		e := frecencyEntry{Frequency: 10, LastOpen: now - dayMs} // one day old
		if got := frecencyScore(&e, now); math.Abs(got-5.0) > 1e-9 {
			t.Fatalf("one day old = %v, want 5", got)
		}
		e2 := frecencyEntry{Frequency: 5, LastOpen: now} // just now
		if got := frecencyScore(&e2, now); math.Abs(got-5.0) > 1e-9 {
			t.Fatalf("just now = %v, want 5", got)
		}
		if got := frecencyScore(nil, now); got != 0 {
			t.Fatalf("absent = %v, want 0", got)
		}
	})
}

func TestUpdateFrecency(t *testing.T) {
	entries := []frecencyEntry{{Path: "a", Frequency: 1, LastOpen: 1}}
	got := updateFrecency(entries, "a", 100)
	if len(got) != 1 || got[0].Frequency != 2 || got[0].LastOpen != 100 {
		t.Fatalf("refresh a: %v", got)
	}
	got = updateFrecency(got, "b", 200)
	if len(got) != 2 {
		t.Fatalf("append b: %v", got)
	}
	if got[0].Path != "b" {
		t.Fatalf("sort lastOpen desc: %v", got)
	}
}

func TestParseFrecency(t *testing.T) {
	t.Run("dedupe by path (last wins)", func(t *testing.T) {
		entries := []frecencyEntry{{Path: "a", Frequency: 1, LastOpen: 1}, {Path: "a", Frequency: 9, LastOpen: 2}}
		got := parseFrecency(entries)
		if len(got) != 1 || got[0].Frequency != 9 {
			t.Fatalf("dedupe last-wins: %v", got)
		}
	})
	t.Run("cap at 1000", func(t *testing.T) {
		big := make([]frecencyEntry, 1200)
		for i := range big {
			big[i] = frecencyEntry{Path: fmt.Sprintf("p%d", i), Frequency: 1, LastOpen: int64(i)}
		}
		if got := parseFrecency(big); len(got) != maxFrecencyEntries {
			t.Fatalf("cap: %d, want %d", len(got), maxFrecencyEntries)
		}
	})
}
```

**Step 2 — confirm FAIL.** `go test ./internal/tui/ -run 'TestFrecencyScore|TestUpdateFrecency|TestParseFrecency' -count=1` → FAIL (build fails: undefined `frecencyEntry`, `frecencyScore`, `updateFrecency`, `parseFrecency`, `dayMs`, `maxFrecencyEntries` — the expected red).

**Step 3 — minimal implementation.**
- `frecency.go`: the `frecencyEntry` type + `maxFrecencyEntries = 1000` + `dayMs = 86_400_000`; `frecencyScore` (the ported `frequency / (1 + age-days)`; absent/zero-frequency → 0; a negative age clamps to 0); `parseFrecency` (dedupe by path last-wins, sort lastOpen desc, cap `maxFrecencyEntries`); `updateFrecency` (refresh the matching path — `frequency+1`, `lastOpen=now` — or append a fresh `frecencyEntry{Path, Frequency: 1, LastOpen: now}`, then `parseFrecency`).
- `app.go`: `const kvFrecencyKey = "prompt_frecency"`; `App.freq []frecencyEntry`; `func coerceFrecency(v any) []frecencyEntry` (handles `[]frecencyEntry` and `[]any` of `map[string]any`, reading `path`/`frequency`/`lastOpen`); `App.loadFrecency()` (nil-engine guard; `a.freq = parseFrecency(coerceFrecency(a.engine.KV().Get(kvFrecencyKey, nil)))`); `App.saveFrecency()` (nil-engine guard; `a.engine.KV().Set(kvFrecencyKey, a.freq)`); the `NewApp` end calls `a.loadFrecency()` (after `loadHistory`).

**Step 4 — gate.** `go test ./internal/tui/ -run 'TestFrecencyScore|TestUpdateFrecency|TestParseFrecency' -count=1` → PASS, then FULL gate: `go vet ./... && go test ./...` + `gofmt -l .` empty.

**Step 5 — commit + close the bead.**
`git add internal/tui/frecency.go internal/tui/app.go internal/tui/frecency_test.go && git commit -m "feat: prompt frecency ranking"`
`bd close yolo-oae.6.4 --reason "frecency ranking green: scoring, parse (dedupe+sort+cap), update, KV load/save" --json`

---

### Task S5.4: @-autocomplete: fuzzy file picker (sahilm/fuzzy) + tests (bead `yolo-oae.6.4`, expected id `yolo-oae.6.5`)

**Files:** new `internal/tui/mention.go` (the `mentionTriggerIndex` + `walkFiles` + the gitignore helpers + `mentionOptions`/`walkedFiles`/`freqFor`/`acInsert` + the `maxWalkFiles`/`maxWalkDepth`/`maxPickerOptions` consts + `walkIgnore`), modify `internal/tui/prompt.go` (`mentionActive`/`acQuery`/`acView`), `internal/tui/keys.go` (the `handleAcKey` + the `mentionActive` branch in `handleKey`), `internal/tui/view.go` (the @-picker overlay); new `internal/tui/mention_test.go`.

**Interfaces:** produces `mentionTriggerIndex(value string) (int, bool)`, `walkFiles(root string) []string`, `App.mentionOptions() []selectOption`, `App.walkedFiles() []string`, `App.freqFor(relPath string) *frecencyEntry`, `App.acInsert(rel string)`, `App.handleAcKey(k tea.KeyPressMsg) []tea.Cmd`, `promptModel.mentionActive() bool`/`acQuery() string`/`acView(opts []selectOption, w int, th theme.Theme) string`, the `App.walkRoot`/`walked` fields, the `maxWalkFiles = 1000` + `maxWalkDepth = 8` + `maxPickerOptions = 10` consts. Consumes S5.3 (`updateFrecency`, `saveFrecency`, `frecencyScore`), S5.1 (`histIdx`/`histText` reset), `sahilm/fuzzy`.

**Upstream parity notes:** autocomplete.tsx — the @-trigger (`mentionTriggerIndex`), the @-files (the TUI-local walk — deviation 225; the upstream `fs.find` has no yolo referent), the fuzzy + prefix×2 + `(1+frecency)` ranking + limit 10; display.ts `mentionTriggerIndex`; spec §9 (`.gitignore` honored → the minimal parse, deviation 225). The selection inserts the path as text (deviation-222 class — no file "parts"/chips).

**Step 1 — write the failing tests.** New `internal/tui/mention_test.go`:

```go
package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
)

func TestMentionTriggerIndex(t *testing.T) {
	tests := []struct {
		in   string
		want int
		ok   bool
	}{
		{"", -1, false},
		{"hello", -1, false},
		{"@", 0, true},
		{"@f", 0, true},
		{"fix @f", 4, true},
		{"fix@f", -1, false},
		{"fix @f el", -1, false},
	}
	for _, tc := range tests {
		idx, ok := mentionTriggerIndex(tc.in)
		if idx != tc.want || ok != tc.ok {
			t.Fatalf("mentionTriggerIndex(%q) = (%d,%v), want (%d,%v)", tc.in, idx, ok, tc.want, tc.ok)
		}
	}
}

func TestWalkFiles(t *testing.T) {
	dir := t.TempDir()
	mk := func(rel string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte("x"), 0o644)
	}
	mk("alpha.go")
	mk("src/gamma.go")
	mk("node_modules/dep.js")
	mk(".git/config")
	got := walkFiles(dir)
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "alpha.go") || !strings.Contains(joined, "src/gamma.go") {
		t.Fatalf("walk missed fixture files:\n%s", joined)
	}
	if strings.Contains(joined, "node_modules") || strings.Contains(joined, ".git") {
		t.Fatalf("walk must skip the static ignore set:\n%s", joined)
	}
}

func TestMentionOptions(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"alpha.go", "beta.go", "alpha_beta.go"} {
		os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644)
	}
	a := testApp()
	a.Service.Dir = dir
	a.prompt.input.SetValue("@al")
	opts := a.mentionOptions()
	if len(opts) == 0 {
		t.Fatal("no options for @al")
	}
	// the prefix match (alpha.go) ranks first (the x2 prefix boost)
	if opts[0].value.(string) != "alpha.go" {
		t.Fatalf("top option = %v, want alpha.go (the prefix match)", opts[0].value)
	}
}

func TestAcInsert(t *testing.T) {
	a := testApp()
	a.Service.Dir = t.TempDir()
	a.prompt.input.SetValue("see @fil")
	a.acInsert("alpha.go")
	if got := a.prompt.input.Value(); got != "see alpha.go" {
		t.Fatalf("insert = %q, want the path replacing the @-query", got)
	}
	if len(a.freq) != 1 || a.freq[0].Path != "alpha.go" || a.freq[0].Frequency != 1 {
		t.Fatalf("frecency not recorded: %v", a.freq)
	}
}

// TestTUIAtPicker is the teatest leg: a real stack, the @-picker filters the
// walked files and enter inserts the selected path.
func TestTUIAtPicker(t *testing.T) {
	ts := testutil.Boot(t)
	os.WriteFile(filepath.Join(ts.Dir, "alpha.go"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(ts.Dir, "beta.go"), []byte("x"), 0o644)
	c := client.New(ts.URL, ts.Dir) // scope (and the walk) to the server work dir
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))
	teatest.WaitFor(t, tm.Output(), hasLine("New session"), teatest.WithDuration(5*time.Second))
	tm.Send(press('n'))
	teatest.WaitFor(t, tm.Output(), hasLine("esc abort/back"), teatest.WithDuration(5*time.Second))
	suiteType(tm, "see @a")
	teatest.WaitFor(t, tm.Output(), hasLine("alpha.go"), teatest.WithDuration(5*time.Second))
	tm.Send(press(tea.KeyEnter))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return strings.Contains(a.prompt.input.Value(), "alpha.go")
	}, teatest.WithDuration(5*time.Second))
	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
```

**Step 2 — confirm FAIL.** `go test ./internal/tui/ -run 'TestMentionTriggerIndex|TestWalkFiles|TestMentionOptions|TestAcInsert|TestTUIAtPicker' -count=1` → FAIL (build fails: undefined `mentionTriggerIndex`, `walkFiles`, `mentionOptions`, `acInsert`, `handleAcKey`, `acView`, `mentionActive` — the expected red).

**Step 3 — minimal implementation.**
- `mention.go`: `mentionTriggerIndex` (the ported `display.ts` rule: the last `@` at the start or preceded by a space/tab whose following text has no space/tab); `walkIgnore` (the static set: `.git`, `node_modules`, `vendor`, `dist`, `build`, `target`, `.next`, `coverage`, `__pycache__`, `.venv`, `venv`); `gitignorePatterns(root) []string` (the non-comment non-blank lines of `<root>/.gitignore`, trailing `/` stripped; empty if absent) + `ignoredByGitignore(patterns []string, rel string) bool` (true when a pattern equals one of rel's path segments); `walkFiles(root)` (the depth-capped walk, `maxWalkFiles = 1000` / `maxWalkDepth = 8`, skipping `walkIgnore` + gitignore dirs and files, returning slash-relative paths); `App.walkedFiles` (the cache, re-walk when `a.Dir` changes); `App.freqFor` (the frecency entry for a path, nil when absent); `App.mentionOptions` (`fuzzy.Find` over `walkedFiles` by the @-query, ×2 prefix boost, `×(1+frecencyScore)`, sort desc, cap `maxPickerOptions = 10`; an empty query → all, frecency-ranked; each option `value` = the path); `App.acInsert` (replace the @-query with the path, reset `histIdx`/`histText` + `prompt.sel`, `updateFrecency` + `saveFrecency`).
- `prompt.go`: `mentionActive` (the value has an active @-trigger), `acQuery` (the @-query = the value after the trigger), `acView` (the muted + `cursorStyle` + `wrapLine` option rows, reusing the slash-menu rendering).
- `keys.go`: `handleAcKey` (up/down `moveMenuSel`, enter `acInsert` the selected option's path, esc removes the @-trigger keeping the prefix, else `inputUpdate`); the `handleKey` ladder adds `if a.prompt.mentionActive() { return a.handleAcKey(k) }` after the slash-menu branch.
- `view.go`: the @-picker overlay — `acMenu := ""`; `if a.prompt.mentionActive() { acMenu = a.prompt.acView(a.mentionOptions(), w, a.theme) }`; rendered like `menu` (below the prompt).

**Step 4 — gate.** `go test ./internal/tui/ -run 'TestMentionTriggerIndex|TestWalkFiles|TestMentionOptions|TestAcInsert|TestTUIAtPicker' -count=1` → PASS, then FULL gate: `go vet ./... && go test ./...` + `gofmt -l .` empty. (`TestImportsDirection` must pass: `mention.go` imports only `internal/protocol`, `internal/tui/*`, stdlib, `sahilm/fuzzy` — the `internal/glob` package is NOT used.)

**Step 5 — commit + close the bead.**
`git add internal/tui/mention.go internal/tui/prompt.go internal/tui/keys.go internal/tui/view.go internal/tui/mention_test.go && git commit -m "feat: @-autocomplete - fuzzy file picker"`
`bd close yolo-oae.6.5 --reason "@-file picker green: trigger, TUI-local walk + .gitignore, fuzzy + frecency rank, insert, teatest" --json`

---

### Task S5.5: /-autocomplete: slash-command picker (GET /command) + tests (bead `yolo-oae.6.5`, expected id `yolo-oae.6.6`)

**Files:** modify `internal/tui/prompt.go` (`menuItems` moves from `promptModel` to `App` as the fuzzy-ranked picker; `menuView` renders the items it is passed), `internal/tui/keys.go` + `internal/tui/view.go` (the `menuItems` call sites → `a.menuItems()`), re-baseline `internal/tui/prompt_test.go` (`TestPromptMenuFilter` → the fuzzy cases + the new `TestPromptMenuFuzzy`).

**Interfaces:** `menuItems` moves to `App`: `func (a *App) menuItems() []protocol.Command` (the fuzzy-ranked merged commands; an empty query → the merged list in order; nil when the menu is closed). The call sites (`view.go:33`, `handleMenuKey`) use `a.menuItems()`; `menuView` stays a `promptModel` render method (it renders the items it is passed). Execute-on-enter is unchanged (`handleMenuKey` + `runCommand`).

**Upstream parity notes:** autocomplete.tsx — the /-picker is fuzzy (threshold 0, limit 10, prefix×2); the commands come from GET /command (yolo: `store.Commands` + `localCommands`). Deviation 226 (empty query = merged order; execute-on-enter preserved).

**Step 1 — write the failing tests.** Re-baseline `internal/tui/prompt_test.go`: the `TestPromptMenuFilter` table pins the stable cases (the menu is closed / the empty query lists all merged / a clear command resolves to itself / no match is empty), and `TestPromptMenuFuzzy` pins the fuzzy ordering + the alias:

```go
func TestPromptMenuFilter(t *testing.T) {
	tests := []struct {
		in   string
		want []string // nil = menu closed
	}{
		{"", nil},
		{"hello", nil},
		{"/", []string{"/sessions", "/connect", "/status", "/themes", "/help", "/new", "/model", "/agents", "/quit"}},
		{"/model", []string{"/model"}},
		{"/quit", []string{"/quit"}},
		{"/zz", []string{}},
	}
	for _, tt := range tests {
		t.Run("in="+tt.in, func(t *testing.T) {
			a := testApp()
			a.store.Commands = testCommands()
			a.prompt.input.SetValue(tt.in)
			got := a.menuItems()
			gotNames := []string(nil)
			if got != nil {
				gotNames = make([]string, 0, len(got))
				for _, c := range got {
					gotNames = append(gotNames, c.Name)
				}
			}
			if len(gotNames) != len(tt.want) {
				t.Fatalf("in=%q got %v, want %v", tt.in, gotNames, tt.want)
			}
			for i := range tt.want {
				if gotNames[i] != tt.want[i] {
					t.Fatalf("in=%q got %v, want %v", tt.in, gotNames, tt.want)
				}
			}
		})
	}
}

func TestPromptMenuFuzzy(t *testing.T) {
	a := testApp()
	a.store.Commands = testCommands()
	// "m" is a prefix of "model" and a subsequence of "themes": the prefix
	// match (x2 boost) ranks first.
	a.prompt.input.SetValue("/m")
	got := a.menuItems()
	if len(got) == 0 {
		t.Fatal("no matches for /m")
	}
	if got[0].Name != "/model" {
		t.Fatalf("top match = %q, want /model (the prefix boost)", got[0].Name)
	}
	// the alias is preserved: /exit maps to the canonical /quit
	a.prompt.input.SetValue("/exit")
	got = a.menuItems()
	if len(got) == 0 || got[0].Name != "/quit" {
		t.Fatalf("alias /exit -> /quit, got %v", got)
	}
}
```

**Step 2 — confirm FAIL.** `go test ./internal/tui/ -run 'TestPromptMenuFilter|TestPromptMenuFuzzy' -count=1` → FAIL (the `a.menuItems()` `App` method is undefined; `TestPromptMenuFuzzy`'s prefix-boost + alias assertions fail — the expected red).

**Step 3 — minimal implementation.**
- `prompt.go`: replace the `promptModel.menuItems(cmds)` method with `App.menuItems()` (the fuzzy-ranked picker: build the candidate set from `mergedCommands()` canonical + alias names, `fuzzy.Find(query, data)`, ×2 for a prefix match, sort desc, cap `maxPickerOptions = 10`, dedupe by canonical name; an empty query returns `mergedCommands()`; nil when `!slashActive()`). `menuView` renders the items it is passed directly (it no longer filters).
- `keys.go` + `view.go`: the `menuItems` call sites become `a.menuItems()` (the `view.go:33` `menuView` arg + `handleMenuKey`'s `items`).
- `prompt_test.go`: re-baseline `TestPromptMenuFilter` (the stable cases above) + add `TestPromptMenuFuzzy` (the prefix-boost ordering + the alias).

**Step 4 — gate.** `go test ./internal/tui/ -run 'TestPromptMenuFilter|TestPromptMenuFuzzy' -count=1` → PASS, then FULL gate: `go vet ./... && go test ./...` + `gofmt -l .` empty. (The load-bearing slash-menu teatest suites — `TestTUIDialogs` (types `/model`/`/agents`/`/help` → the dialogs), `TestTUIFullTurn`, `TestPromptMenuKeys` (the wraparound) — must stay green: a clear command still resolves to the right top match, the empty query still lists all commands, and execute-on-enter is unchanged. Re-baseline `TestPromptMenuKeys` only if the fuzzy item count changes its wrap pin.)

**Step 5 — commit + close the bead.**
`git add internal/tui/prompt.go internal/tui/keys.go internal/tui/view.go internal/tui/prompt_test.go && git commit -m "feat: /-autocomplete - slash command picker"`
`bd close yolo-oae.6.6 --reason "slash-command picker green: fuzzy rank + prefix boost + cap, alias, empty=merged, execute-on-enter preserved" --json`

---

### Task S5.6: Terminal bell on turn completion / error (ported `notifications.ts` conditions) + tests (bead `yolo-oae.6.6`, expected id `yolo-oae.6.7`)

**Files:** new `internal/tui/attention.go` (the `attentionState` + `bell` + `onAttention`), modify `internal/tui/app.go` (the `attention` field + the `EventMsg` hook); new `internal/tui/attention_test.go`.

**Interfaces:** produces `attentionState{active, errored bool, lastPermID string}`, `bell() tea.Cmd` (= `tea.Raw("\a")`), `App.onAttention(ev protocol.Event) tea.Cmd` (nil or the bell), `App.attention attentionState`. The `EventMsg` hook (the bell cmd batched into the applied event's cmd) is internal.

**Upstream parity notes:** notifications.ts — the ported conditions (deviation 227): the permission ask (deduped by id), the busy/retry active set, the idle-after-active not-errored "done" bell, the turn-error bell (the current `Message.Error` — the `session.error` referent gap); DROPPED the `question.asked` + SSE-timeout conditions (no yolo referent). `tea.Raw` is the alt-screen-independent bell seam.

**Step 1 — write the failing tests.** New `internal/tui/attention_test.go`:

```go
package tui

import (
	"bytes"
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
)

func attentionApp() *recApp {
	a := testApp()
	a.route = routeSession
	a.curSessionID = "s1"
	return a
}

func statusEv(t string) protocol.Event {
	ev, _ := protocol.MakeEvent(protocol.EventTypeSessionStatus, protocol.SessionStatusProps{SessionID: "s1", Status: protocol.SessionStatus{Type: t}})
	return ev
}

func msgErrEv(typ string) protocol.Event {
	ev, _ := protocol.MakeEvent(protocol.EventTypeMessageUpdated, protocol.MessageUpdatedProps{
		SessionID: "s1",
		Info:      protocol.Message{ID: "m1", Error: &protocol.MessageError{Type: typ, Message: "boom"}},
	})
	return ev
}

func TestAttentionBell(t *testing.T) {
	t.Run("done: idle after busy rings", func(t *testing.T) {
		a := attentionApp()
		if c := a.onAttention(statusEv("busy")); c != nil {
			t.Fatalf("busy must not ring, got %v", c)
		}
		if c := a.onAttention(statusEv("idle")); c == nil {
			t.Fatal("idle after busy must ring the done bell")
		}
	})
	t.Run("done: idle after an error does not ring", func(t *testing.T) {
		a := attentionApp()
		a.onAttention(statusEv("busy"))
		a.onAttention(msgErrEv("unknown"))
		if c := a.onAttention(statusEv("idle")); c != nil {
			t.Fatalf("idle after an error must not ring the done bell, got %v", c)
		}
	})
	t.Run("error: the current message error rings", func(t *testing.T) {
		a := attentionApp()
		if c := a.onAttention(msgErrEv("unknown")); c == nil {
			t.Fatal("a turn error must ring")
		}
	})
	t.Run("permission ask rings, deduped by id", func(t *testing.T) {
		a := attentionApp()
		ev, _ := protocol.MakeEvent(protocol.EventTypePermissionAsked, protocol.PermissionAskedProps{ID: "p1", SessionID: "s1", Permission: "bash"})
		if c := a.onAttention(ev); c == nil {
			t.Fatal("a permission ask must ring")
		}
		ev2, _ := protocol.MakeEvent(protocol.EventTypePermissionAsked, protocol.PermissionAskedProps{ID: "p1", SessionID: "s1", Permission: "bash"})
		if c := a.onAttention(ev2); c != nil {
			t.Fatalf("a duplicate permission ask must not re-ring, got %v", c)
		}
	})
	t.Run("a non-current session's status is ignored", func(t *testing.T) {
		a := attentionApp()
		ev, _ := protocol.MakeEvent(protocol.EventTypeSessionStatus, protocol.SessionStatusProps{SessionID: "other", Status: protocol.SessionStatus{Type: "busy"}})
		a.onAttention(ev)
		if a.attention.active {
			t.Fatal("a non-current session's busy must not set active")
		}
	})
}

// TestTUIAttentionBell is the teatest leg: a real turn completes -> the
// session goes busy then idle -> the done bell (tea.Raw("\a")) lands in the
// captured output.
func TestTUIAttentionBell(t *testing.T) {
	drv := fake.New(fake.Turn{Parts: []llm.Part{{Kind: "text", Text: "done", Finish: "stop"}}})
	ts := testutil.BootWithDriver(t, drv)
	c := client.New(ts.URL, ts.Dir)
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))
	teatest.WaitFor(t, tm.Output(), hasLine("New session"), teatest.WithDuration(5*time.Second))
	tm.Send(press('n'))
	teatest.WaitFor(t, tm.Output(), hasLine("esc abort/back"), teatest.WithDuration(5*time.Second))
	suiteType(tm, "hello")
	tm.Send(press(tea.KeyEnter))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return strings.Contains(stripANSI(string(b)), "done") && bytes.Contains(b, []byte{0x07})
	}, teatest.WithDuration(10*time.Second))
	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
```

**Step 2 — confirm FAIL.** `go test ./internal/tui/ -run 'TestAttentionBell|TestTUIAttentionBell' -count=1` → FAIL (build fails: undefined `a.onAttention`, `a.attention`, `bell` — the expected red).

**Step 3 — minimal implementation.**
- `attention.go`: `attentionState{active, errored bool, lastPermID string}`; `bell() tea.Cmd` (= `tea.Raw("\a")`); `App.onAttention(ev protocol.Event) tea.Cmd` (the four ported conditions, current-session-scoped: the `permission.asked` deduped by id; the `session.status` busy|retry → active + clear errored; the `session.status` idle → the done bell only when active and not errored; the current `message.updated` with a non-nil `Error` → errored + the bell).
- `app.go`: `App.attention attentionState`; the `EventMsg` case — after `a.onSessionStatus(prev, m.Event)` + `a.sess.isDirty = true`, `cmd := a.eventPump()`, `if b := a.onAttention(m.Event); b != nil { cmd = tea.Batch(cmd, b) }`, `return a.afterApply(cmd)`.

**Step 4 — gate.** `go test ./internal/tui/ -run 'TestAttentionBell|TestTUIAttentionBell' -count=1` → PASS, then FULL gate: `go vet ./... && go test ./...` + `gofmt -l .` empty.

**Step 5 — commit + close the bead.**
`git add internal/tui/attention.go internal/tui/app.go internal/tui/attention_test.go && git commit -m "feat: terminal bell on turn completion/error"`
`bd close yolo-oae.6.7 --reason "terminal bell green: permission/done/error conditions ported, question + SSE-timeout dropped, teatest bell byte" --json`

---

## S5 slice gate (slice bead `yolo-oae.6`)

NOT a task bead; runs after all child beads close. Mirror the S0 slice gate
shape: (1) module gate `go vet ./... && go test ./...` + `gofmt -l .` empty
(incl. `TestImportsDirection` + the S5 teatest goldens); (2) user-run smoke
(NOT CI): in a real TTY — ↑/↓ recall across a restart (KV persistence), the
@-file picker, the /-command picker, and the bell on turn completion/error;
(3) append any forced DEVIATIONS.md entries this slice named (with
severity, same-commit rule — root principle 2); (4) PROGRESS.md one-line
status pointer; (5) commit
`docs: checkpoint — S5 done, next is S6 detail pass`; (6)
`bd close yolo-oae.6 --reason "all 6 child beads closed, gate green" --json`.
