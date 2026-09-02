// Package tui is the bubbletea v2 frontend for yolo.
//
// The TUI is a pure client: it talks to the core server only through the wire
// contract (internal/protocol) via internal/tui/client. Non-test files import
// only internal/protocol, internal/tui/*, the standard library, and the charm
// deps.
package tui

import (
	"context"
	"encoding/json"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// EventMsg carries one server SSE event. It is exported so the test harness
// can drive the app with it.
type EventMsg struct{ Event protocol.Event }

// ThemeRefreshMsg re-arms the theme refresh debounce (the port of
// upstream themes.subscribeRefresh → refresh, theme.tsx:235-244);
// cmd/yolo sends it to the running program on every theme signal
// (SIGUSR2 via theme.WatchThemeSignals, S0.6).
type ThemeRefreshMsg struct{}

type themeReapplyMsg struct{} // 250 ms leg: regenerate the system theme
type themeCustomsMsg struct{} // 1000 ms leg: system theme + customs re-discovery

// leaderTimeoutMsg clears the pending leader (the ported registerTimedLeader
// timeout — the pending sequence expires after LeaderTimeout).
type leaderTimeoutMsg struct{}

// themeRefreshDelays mirrors upstream THEME_REFRESH_DELAYS
// (theme.tsx:82): the 250 ms leg re-generates the system theme; the
// 1000 ms leg (the last) also re-discovers customs.
var themeRefreshDelays = [2]time.Duration{250 * time.Millisecond, time.Second}

type route int

const (
	routeHome route = iota
	routeSession
)

// App is the root bubbletea model: routes, store, dialog stack and the SSE
// event pump.
type App struct {
	*client.Service
	store        store.State
	route        route
	curSessionID string
	home         homeModel
	sess         sessionModel
	prompt       promptModel
	dlg          dialogStack
	toasts       []toast
	toastSeq     int
	toastCmds    []tea.Cmd
	lastErr      string
	// theme engine (S0.7): nil = unthemed run (the zero Theme paints
	// nothing — S0.8+ views read a.theme, never hex)
	engine  *theme.Engine
	theme   theme.Theme
	spinIdx int // footer spinner frame
	// S6.1 startup loading spinner (deviation 237): the shown/ready
	// state + the shown stamp (the min-hold origin).
	loadShown bool
	loadReady bool
	loadStamp time.Time
	// tea plumbing
	size      tea.WindowSizeMsg
	eventCh   chan protocol.Event
	resyncCh  chan struct{} // SSE drop pings from the client
	resyncing bool          // a transient SSE drop's re-hydrate is in flight
	stop      context.CancelFunc
	emitSink  func(cmds ...tea.Cmd) // test seam, set from _test.go only
	// S3.7 retry-action per-run gate (deviation 194): sessionID → true after
	// ANY open (dismiss or action), cleared on the next send for that
	// session (the applySend success path).
	retrySuppressed map[string]bool
	keymap          *Keymap // the keymap registry (S4.2)
	pendingLeader   bool    // the leader pending state is armed
	// S5.1 prompt history: the entries (most-recent LAST, in-memory until
	// S5.2's KV load), the recall index (0 = present, -1 = newest, -len =
	// oldest), the text last set by a recall (the dirty guard), and the
	// draft captured on the first up-press (restored at present).
	hist     []string
	histIdx  int
	histText string
	histOrig string
	// S5.3 prompt frecency: the file-path ranking entries (scope-relative
	// keys, deviation 224), persisted under kvFrecencyKey (deviation 223);
	// the @-picker consumes the ranking (S5.4).
	freq []frecencyEntry
	// S5.4 @-picker: the cached walk of the scope dir (deviation 225) —
	// walkRoot is the scope dir the cached walk was taken of ("" = never
	// walked or the server work dir), walked its slash-relative paths.
	walkRoot string
	walked   []string
	// S5.6 attention bell: the ported notifications.ts conditions (deviation
	// 227), current-session-scoped.
	attention attentionState
	// S6.3 home tips: the current tip index (repicked per home entry — the
	// upstream per-mount Math.random, no timer; deviation 235 note), the
	// random seam (default math/rand.Float64 — tests seed it) and the
	// hidden flag (persisted over the theme KV under kvTipsHiddenKey,
	// deviation 223).
	tipIdx     int
	tipRand    func() float64
	tipsHidden bool
}

// kvTipsHiddenKey is the KV key the tips-hidden flag persists under (the
// S5.2 KV persistence surface, deviation 223).
const kvTipsHiddenKey = "tips_hidden"

// NewApp builds the root model. A non-empty startSessionID starts on that
// session (resume); empty starts at home. A nil engine runs without the
// theme engine (the zero Theme paints nothing). The prompt is always
// focused with a static (non-blinking) cursor.
func NewApp(c *client.Service, s store.State, startSessionID string, engine *theme.Engine) *App {
	ctx, cancel := context.WithCancel(context.Background())
	eventCh, resyncCh := c.Events(ctx)
	// the keymap registry (S4.2): the defaults (the config overrides are
	// applied by SetKeybinds, S4.3). NewKeymap(nil) never errors (no unknown
	// keys; the defaults are valid).
	km, _ := NewKeymap(nil)
	a := &App{
		Service:         c,
		store:           s,
		route:           routeHome,
		home:            homeModel{now: nowMillis},
		sess:            newSessionModel(80, 21),
		size:            tea.WindowSizeMsg{Width: 80, Height: 24},
		eventCh:         eventCh,
		resyncCh:        resyncCh,
		stop:            cancel,
		engine:          engine,
		keymap:          km,
		pendingLeader:   false,
		retrySuppressed: map[string]bool{},
		tipRand:         rand.Float64,
	}
	// S6.3: the home tips line seam (the footer seam is S6.4).
	a.home.tips = func(w int) string { return a.homeTipsLine(w) }
	in := textinput.New()
	// textinput's View is prompt(2) + width + cursor(1): size the value
	// area so the whole line fits the 80-column default terminal.
	in.SetWidth(77)
	st := in.Styles()
	st.Cursor.Blink = false
	in.SetStyles(st)
	in.Focus()
	a.prompt.input = in
	if startSessionID != "" {
		a.route = routeSession
		a.curSessionID = startSessionID
	}
	a.retheme()
	a.loadHistory()
	a.loadFrecency()
	a.loadTipsHidden()
	a.repickTip()
	return a
}

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

// Close stops the SSE pump. Call it once the program exits.
func (a *App) Close() { a.stop() }

// termWidth is the terminal width with the pre-WindowSizeMsg fallback (the
// session route uses the same 80 for the viewport).
func (a *App) termWidth() int {
	if a.size.Width < 1 {
		return 80
	}
	return a.size.Width
}

// Init hydrates the starting route and arms the SSE + resync pumps.
func (a *App) Init() tea.Cmd {
	cmds := []tea.Cmd{a.hydrateCmd(), a.eventPump(), a.loadArm()}
	if c := a.resyncPump(); c != nil {
		cmds = append(cmds, c)
	}
	return tea.Batch(cmds...)
}

// Update dispatches a message, then drains the toast ticks armed during the
// update and merges them into the returned cmd (each toast owns its 4s
// auto-clear tick).
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := a.updateMsg(msg)
	if c := a.drainToastCmds(); c != nil {
		if cmd == nil {
			cmd = c
		} else {
			cmd = tea.Batch(cmd, c)
		}
	}
	return a, cmd
}

func (a *App) updateMsg(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.size = m
		// textinput's View is prompt(2) + width + cursor(1): subtract all
		// three so the prompt line never exceeds the terminal width.
		if m.Width > 3 {
			a.prompt.input.SetWidth(m.Width - 3)
		}
		// The transcript is word-wrapped at the viewport width: re-wrap on
		// resize instead of clipping at the stale width.
		a.sess.isDirty = true
		return nil
	case EventMsg:
		a.store.Live = true
		// The previous status type (store.Apply already lands the new value
		// by the time the hook runs — S3.7 reads prev, not the store).
		prev := a.store.Status.Type
		a.store.Apply(m.Event)
		a.syncPermDialog()
		if m.Event.Type == protocol.EventTypeSessionUpdated || m.Event.Type == protocol.EventTypeSessionDeleted {
			a.syncSessionSel()
		}
		a.onSessionStatus(prev, m.Event)
		// Any applied event may have changed the transcript (message/part
		// family); re-render once instead of on every frame.
		a.sess.isDirty = true
		cmd := a.eventPump()
		// S5.6 attention bell (deviation 227): batch the bell cmd into the
		// applied event's cmd.
		if b := a.onAttention(m.Event); b != nil {
			cmd = tea.Batch(cmd, b)
		}
		return a.afterApply(cmd)
	case connLostMsg:
		a.store.Live = false
		return nil
	case resyncMsg:
		// The SSE stream dropped (the client is reconnecting): events
		// published in the gap are unrecoverable — re-hydrate the current
		// route over REST and re-arm the resync pump. The footer shows the
		// outage window until the re-hydrate completes (concurrency-4).
		a.resyncing = true
		a.sess.isDirty = true
		return tea.Batch(a.hydrateCmd(), a.resyncPump())
	case spinMsg:
		a.spinIdx++
		if a.statusSeg() != "" || a.loadShown {
			return a.spinTick()
		}
		return nil
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
	case permReplyMsg:
		return a.applyPermReply(m)
	case HydrateMsg:
		return a.hydrateCmd()
	case hydratedMsg:
		if a.resyncing {
			a.resyncing = false
			a.store.Live = true
		}
		return a.applyHydrate(m)
	case catalogMsg:
		return a.applyCatalog(m)
	case dlgPatchMsg:
		return a.applyDlgPatch(m)
	case sessionCreatedMsg:
		return a.applySessionCreated(m)
	case toastExpireMsg:
		a.removeToast(m.id)
		return nil
	case leaderTimeoutMsg:
		a.pendingLeader = false
		return nil
	case abortedMsg:
		if m.err != nil {
			a.lastErr = "abort: " + m.err.Error()
		}
		return nil
	case sendMsg:
		return a.applySend(m)
	case commandExecMsg:
		return a.applyCommandExec(m)
	case statusSnapshotMsg:
		return a.applySessionStatusSnapshot(m)
	case sessionDeleteMsg:
		return a.applySessionDelete(m)
	case renameMsg:
		return a.applyRename(m)
	case authMsg:
		return a.applyAuth(m)
	case tea.KeyPressMsg:
		cmds := a.handleKey(m)
		if len(cmds) == 0 {
			return nil
		}
		return tea.Batch(cmds...)
	case tea.InterruptMsg:
		// SIGINT during Run: the same as the ctrl+c keystroke (cli-2) —
		// route it through the full key ladder so a pending permission
		// ask or an open dialog still owns the keys.
		cmds := a.handleKey(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
		if len(cmds) == 0 {
			return nil
		}
		return tea.Batch(cmds...)
	case ThemeRefreshMsg:
		if a.engine == nil {
			return nil
		}
		return a.themeRefresh()
	case themeReapplyMsg:
		if a.engine != nil {
			a.engine.Reapply()
			a.retheme()
		}
		return nil
	case themeCustomsMsg:
		if a.engine != nil {
			// Upstream leg order (theme.tsx:239-243): refreshSystemTheme
			// FIRST, then syncCustomThemes on the last delay.
			a.engine.Reapply()
			_ = a.engine.RefreshCustoms(context.Background())
			a.retheme()
		}
		return nil
	default:
		// huh v2's form-progress messages (unexported group/field types,
		// driven by huh's NextField/SubmitCmd cmds) belong to the open form
		// modal — feed them back in so submit completes (S2.3).
		if f := a.dlg.form(); f != nil {
			cmds := f.forwardMsg(a, msg)
			if len(cmds) > 0 {
				return tea.Batch(cmds...)
			}
			return nil
		}
	}
	return nil
}

// themeRefresh arms the two refresh legs (upstream refresh,
// theme.tsx:235-244). A re-signal re-arms a second pair — bubbletea
// v2 has no tick cancellation; the legs are idempotent (they
// re-derive from the engine's cached state), so the outcome is
// unchanged.
func (a *App) themeRefresh() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(themeRefreshDelays))
	for i, d := range themeRefreshDelays {
		// Go ≥ 1.22: i and d are per-iteration (module requires 1.25).
		cmds = append(cmds, tea.Tick(d, func(time.Time) tea.Msg {
			if i == len(themeRefreshDelays)-1 {
				return themeCustomsMsg{}
			}
			return themeReapplyMsg{}
		}))
	}
	return tea.Batch(cmds...)
}

// retheme refreshes a.theme from the engine (the port of the upstream
// values() memo read, theme.tsx:256-267) and applies the theme to the
// prompt cursor (upstream cursorColor = theme.text, prompt/index.tsx:253;
// CursorStyle carries a Color but no bold field — bubbles v2.2.1
// textinput/styles.go:70-98). With no engine (or no `text` token),
// a.theme stays the zero Theme and the cursor keeps its default.
func (a *App) retheme() {
	if a.engine == nil {
		return
	}
	if th, err := a.engine.ActiveTheme(); err == nil {
		a.theme = th
		if c, ok := th.Color("text"); ok && c.A != 0 {
			st := a.prompt.input.Styles()
			st.Cursor.Color = lipgloss.Color(c.Hex()[:7])
			a.prompt.input.SetStyles(st)
		}
	}
}

// kvHistoryKey is the KV key the prompt history persists under (the
// S5.2 KV persistence surface; deviation 223).
const kvHistoryKey = "prompt_history"

// coerceStrings coerces a reloaded KV value to []string — the in-run
// []string or the JSON []any of string a process-restart reload yields
// (deviation 223). Anything else (absent/nil) is no history.
func coerceStrings(v any) []string {
	switch s := v.(type) {
	case []string:
		return s
	case []any:
		out := make([]string, 0, len(s))
		for _, e := range s {
			if t, ok := e.(string); ok {
				out = append(out, t)
			}
		}
		return out
	}
	return nil
}

// loadHistory restores the persisted prompt history from the KV (the
// S5.2 boot load, run in NewApp after retheme; a nil engine skips —
// the history stays empty in-memory).
func (a *App) loadHistory() {
	if a.engine == nil {
		return
	}
	a.hist = coerceStrings(a.engine.KV().Get(kvHistoryKey, nil))
}

// saveHistory persists the current prompt history to the KV (the
// S5.2 write path, called from appendHistory; a nil engine skips).
func (a *App) saveHistory() {
	if a.engine == nil {
		return
	}
	a.engine.KV().Set(kvHistoryKey, a.hist)
}

// kvFrecencyKey is the KV key the prompt frecency persists under (the
// S5.3 KV persistence surface, deviation 223).
const kvFrecencyKey = "prompt_frecency"

// coerceFrecency coerces a reloaded KV value to []frecencyEntry — the
// in-run []frecencyEntry or the JSON []any of map[string]any a
// process-restart reload yields (deviation 223). Anything else
// (absent/nil) is no frecency.
func coerceFrecency(v any) []frecencyEntry {
	switch s := v.(type) {
	case []frecencyEntry:
		return s
	case []any:
		out := make([]frecencyEntry, 0, len(s))
		for _, e := range s {
			m, ok := e.(map[string]any)
			if !ok {
				continue
			}
			f := frecencyEntry{}
			if p, ok := m["path"].(string); ok {
				f.Path = p
			} else {
				continue
			}
			if n, ok := m["frequency"].(float64); ok {
				f.Frequency = int(n)
			}
			if t, ok := m["lastOpen"].(float64); ok {
				f.LastOpen = int64(t)
			}
			out = append(out, f)
		}
		return out
	}
	return nil
}

// loadFrecency restores the persisted prompt frecency from the KV (the
// S5.3 boot load, run in NewApp after loadHistory; a nil engine skips —
// the frecency stays empty in-memory).
func (a *App) loadFrecency() {
	if a.engine == nil {
		return
	}
	a.freq = parseFrecency(coerceFrecency(a.engine.KV().Get(kvFrecencyKey, nil)))
}

// saveFrecency persists the current prompt frecency to the KV (the
// S5.3 write path, called from the @-picker selection in S5.4; a nil
// engine skips).
func (a *App) saveFrecency() {
	if a.engine == nil {
		return
	}
	a.engine.KV().Set(kvFrecencyKey, a.freq)
}

// repickTip re-rolls the tip index (the per-home-entry re-pick — the
// upstream per-mount Math.random, no timer; deviation 235 note).
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

// afterApply arms the footer spinner when a just-applied event left the
// session non-idle.
func (a *App) afterApply(cmd tea.Cmd) tea.Cmd {
	if a.statusSeg() == "" {
		return cmd
	}
	if cmd == nil {
		return a.spinTick()
	}
	return tea.Batch(cmd, a.spinTick())
}

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

// onSessionStatus is the S3.7 retry-action trigger: a session.status event
// for the current session on the idle->retry transition opens the
// retry-action dialog once per run (the per-run suppression — deviation 194).
// prevType is the store status type BEFORE the event applied (store.Apply
// already landed the new value by the time this hook runs).
func (a *App) onSessionStatus(prevType string, ev protocol.Event) {
	if ev.Type != protocol.EventTypeSessionStatus {
		return
	}
	var p protocol.SessionStatusProps
	if json.Unmarshal(ev.Properties, &p) != nil {
		return
	}
	if p.SessionID != a.curSessionID {
		return
	}
	if p.Status.Type != protocol.SessionStatusRetry {
		return
	}
	if prevType != protocol.SessionStatusIdle {
		return
	}
	if a.retrySuppressed[p.SessionID] {
		return
	}
	a.retrySuppressed[p.SessionID] = true
	a.openRetryActionDialog("Request failed",
		p.Status.Message+" (retrying, attempt "+strconv.Itoa(p.Status.Attempt)+")", "Abort")
}

// eventPump blocks on the SSE channel and delivers the next event. It re-arms
// itself on every event; on channel close it delivers connLostMsg and stops.
func (a *App) eventPump() tea.Cmd {
	ch := a.eventCh
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return connLostMsg{}
		}
		return EventMsg{Event: ev}
	}
}

// emit returns cmds unchanged; when a test sink is installed (emitSink), it
// also captures the non-nil cmds there.
func (a *App) emit(cmds ...tea.Cmd) []tea.Cmd {
	if a.emitSink != nil {
		nonNil := make([]tea.Cmd, 0, len(cmds))
		for _, c := range cmds {
			if c != nil {
				nonNil = append(nonNil, c)
			}
		}
		if len(nonNil) > 0 {
			a.emitSink(nonNil...)
		}
	}
	return cmds
}
