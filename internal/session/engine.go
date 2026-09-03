package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"

	"github.com/kido5217/yolo/internal/auth"
	"github.com/kido5217/yolo/internal/bus"
	"github.com/kido5217/yolo/internal/llm"
	"github.com/kido5217/yolo/internal/log"
	"github.com/kido5217/yolo/internal/permission"
	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/provider"
	"github.com/kido5217/yolo/internal/storage"
	"github.com/kido5217/yolo/internal/tool"
)

// ErrSessionBusy is returned by Send when the session already has an active
// turn.
var ErrSessionBusy = errors.New("session: already busy")

// errRoundEnded marks a round already finalized inside streamWithRetry
// (the overflow path): the caller ends the turn idle without reading a
// stream.
var errRoundEnded = errors.New("round ended")

// errMaxToolRounds ends the turn when the tool round budget is exhausted:
// a non-failure in the yolo model (idle, no wire error — the error is the
// onDone log site only), so the fail-path error surface skips it.
var errMaxToolRounds = errors.New("session: max tool rounds exceeded")

// maxToolRounds caps the tool round-trips of one turn.
const maxToolRounds = 50

// maxRetryAttempts caps stream attempts per round (initial request included).
const maxRetryAttempts = 4

// maxToolSteps caps tool calls per turn; the remaining calls of the final
// stream beyond the budget are dropped and the turn ends idle.
const maxToolSteps = 50

// Deps is the engine's dependency set.
type Deps struct {
	DB   *storage.DB
	Bus  *bus.Bus
	Prov *provider.Registry
	Perm *permission.Service

	Tools   map[string]tool.Tool
	DataDir string // ~/.yolo-equivalent (AGENTS.md walk-up, plan agent rules)
	Cfg     func(projectDir string) (*protocol.Config, error)
	// Log receives turn-level diagnostics; nil = no-op.
	Log *log.Logger

	// Drivers, when set for a provider id, overrides the registry driver
	// (tests wire the scripted fake).
	Drivers map[string]llm.Driver
	// Clock returns ms timestamps; nil = time.Now.
	Clock func() int64
	// Backoff returns the delay before retry attempt n (1-based); nil =
	// 1s × 2^(n-1) × jitter uniform(0.8, 1.2).
	Backoff func(attempt int) time.Duration
}

type Engine struct {
	db        *storage.DB
	bus       *bus.Bus
	prov      *provider.Registry
	perm      *permission.Service
	tools     map[string]tool.Tool
	schemas   map[string]json.RawMessage // marshalled tool Schema per id, built once in New
	lg        *log.Logger
	dataDir   string
	outputDir string // dataDir/tool-output (upstream TRUNCATION_DIR); "" if unset
	cfg       func(projectDir string) (*protocol.Config, error)
	drivers   map[string]llm.Driver
	clock     func() int64
	backoff   func(attempt int) time.Duration

	mu   sync.Mutex
	busy map[string]context.CancelFunc
	// turnDone is closed when the session's in-flight turn ends: WaitIdle
	// awaiters observe the done event instead of polling the busy flag.
	turnDone  map[string]chan struct{}
	shells    map[string]*tool.Shell
	titleCtx  map[string]*titleCancel
	titleWait sync.WaitGroup
	// deleted suppresses events for closed sessions (troubleshoot-5). It is
	// unbounded by design: an entry can never be removed safely (a late event
	// or a late turn must stay suppressed), and a session-id set per process
	// lifetime is small in practice.
	deleted map[string]struct{}
}

// New builds the engine from its deps. DB, Bus, Prov, Perm and Tools are
// required: a miswired dep is a construction error, not a nil panic deep in
// an un-recovered turn goroutine (which would crash the single binary).
func New(d Deps) (*Engine, error) {
	switch {
	case d.DB == nil:
		return nil, errors.New("session: Deps.DB required")
	case d.Bus == nil:
		return nil, errors.New("session: Deps.Bus required")
	case d.Prov == nil:
		return nil, errors.New("session: Deps.Prov required")
	case d.Perm == nil:
		return nil, errors.New("session: Deps.Perm required")
	case d.Tools == nil:
		return nil, errors.New("session: Deps.Tools required")
	}
	// Schemas are marshalled once (encoding is deterministic), so every
	// round serves the same wire bytes without re-building the schema maps.
	schemas := make(map[string]json.RawMessage, len(d.Tools))
	for id, t := range d.Tools {
		raw, err := json.Marshal(t.Schema())
		if err != nil {
			return nil, fmt.Errorf("session: tool %q schema: %w", id, err)
		}
		schemas[id] = raw
	}
	clock := d.Clock
	if clock == nil {
		clock = func() int64 { return time.Now().UnixMilli() }
	}
	backoff := d.Backoff
	if backoff == nil {
		backoff = defaultBackoff
	}
	lg := d.Log
	if lg == nil {
		lg = log.Noop()
	}
	return &Engine{
		db:        d.DB,
		bus:       d.Bus,
		prov:      d.Prov,
		perm:      d.Perm,
		tools:     d.Tools,
		schemas:   schemas,
		lg:        lg,
		dataDir:   d.DataDir,
		outputDir: outputDirFor(d.DataDir),
		cfg:       d.Cfg,
		drivers:   d.Drivers,
		clock:     clock,
		backoff:   backoff,
		busy:      map[string]context.CancelFunc{},
		turnDone:  map[string]chan struct{}{},
		shells:    map[string]*tool.Shell{},
		titleCtx:  map[string]*titleCancel{},
		deleted:   map[string]struct{}{},
	}, nil
}

// defaultBackoff is the production retry delay after a failed attempt
// (1-based): 1s × 2^(attempt-1) scaled by a uniform jitter in [0.8, 1.2].
func defaultBackoff(attempt int) time.Duration {
	base := time.Second << uint(attempt-1)
	jitter := 0.8 + 0.4*rand.Float64()
	return time.Duration(float64(base) * jitter)
}

// outputDirFor maps the data dir to the truncated-bash output dir (upstream
// TRUNCATION_DIR = Global.Path.data + "tool-output"); unset data dir → "".
func outputDirFor(dataDir string) string {
	if dataDir == "" {
		return ""
	}
	return filepath.Join(dataDir, "tool-output")
}

// SendResult identifies the persisted user message and its text part.
type SendResult struct {
	MessageID, PartID string
}

// Send persists the user message and spawns the turn goroutine. It returns
// ErrSessionBusy when a turn is already active for the session.
func (e *Engine) Send(ctx context.Context, sessionID, text string, onDone func(error)) (SendResult, error) {
	row, err := e.db.GetSession(ctx, sessionID)
	if err != nil {
		return SendResult{}, err
	}
	info, model, err := e.prov.Resolve(row.Model)
	if err != nil {
		return SendResult{}, err
	}

	e.mu.Lock()
	if _, active := e.busy[sessionID]; active {
		e.mu.Unlock()
		return SendResult{}, ErrSessionBusy
	}
	turnCtx, cancel := context.WithCancel(ctx)
	e.busy[sessionID] = cancel
	e.turnDone[sessionID] = make(chan struct{})
	e.mu.Unlock()

	now := e.clock()
	msgID := protocol.NewID("msg")
	partID := protocol.NewID("prt")
	if err := e.db.CreateMessage(ctx, storage.MessageRow{
		ID: msgID, SessionID: sessionID, Role: "user", Agent: row.Agent, TimeCreated: now,
	}); err != nil {
		e.endTurn(sessionID)
		return SendResult{}, err
	}
	userPart := protocol.Part{
		ID: partID, SessionID: sessionID, MessageID: msgID,
		Type: "text", Text: text, Time: protocol.PartTime{Start: now},
	}
	userPartRow, err := storage.ProtocolToPart(userPart)
	if err != nil {
		e.endTurn(sessionID)
		return SendResult{}, fmt.Errorf("session: persist user part: %w", err)
	}
	if err := e.db.UpsertPart(ctx, userPartRow); err != nil {
		e.endTurn(sessionID)
		return SendResult{}, err
	}
	userMsg := protocol.Message{
		ID: msgID, SessionID: sessionID, Role: "user", Agent: row.Agent,
		Time:  protocol.MessageTime{Created: now},
		Model: &protocol.MessageModel{ProviderID: info.ID, ModelID: model.ID},
	}
	e.publish(protocol.EventTypeMessageUpdated, protocol.MessageUpdatedProps{SessionID: sessionID, Info: userMsg})
	e.publish(protocol.EventTypeMessagePartUpdated, protocol.MessagePartUpdatedProps{
		SessionID: sessionID, Part: userPart, Time: now,
	})

	t := newTurn(sessionID, row, info, model)
	e.maybeScheduleTitle(ctx, t, text)

	go e.runTurn(turnCtx, t, onDone)
	return SendResult{MessageID: msgID, PartID: partID}, nil
}

// Status reports "idle" or "busy" for the session.
func (e *Engine) Status(sessionID string) string {
	e.mu.Lock()
	_, active := e.busy[sessionID]
	e.mu.Unlock()
	if active {
		return protocol.SessionStatusBusy
	}
	return protocol.SessionStatusIdle
}

// Abort cancels the active turn of the session (and its title side-call).
// The turn cancel is invoked under the busy-map lock: a turn starting in the
// window gets its own cancel, never the previous turn's (TOCTOU, row 4).
func (e *Engine) Abort(sessionID string) bool {
	e.mu.Lock()
	cancel, active := e.busy[sessionID]
	if tc := e.titleCtx[sessionID]; tc != nil {
		tc.cancel()
	}
	if active {
		cancel()
	}
	e.mu.Unlock()
	return active
}

// WaitIdle blocks until the session's in-flight turn releases the busy flag
// (its done signal) or ctx is done, returning ctx.Err() on cancellation.
// An idle or unknown session returns nil immediately. It awaits the done
// event instead of polling Status; the channel is captured under the engine
// lock, so a turn that ends between the capture and the select has already
// closed it and the wait returns at once. Note that it observes a single
// settlement: a NEW turn that starts after the capture is not awaited (the
// method then returns nil while the session is busy again) — callers that
// must observe an idle state re-check Status (settleAndClose loops).
func (e *Engine) WaitIdle(ctx context.Context, sessionID string) error {
	e.mu.Lock()
	ch := e.turnDone[sessionID]
	e.mu.Unlock()
	if ch == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ch:
		return nil
	}
}

// Close tears down the session's per-work resources: it aborts the
// in-flight turn, suppresses further events for the session, and closes the
// bash shell only after the turn settles (bounded wait, then hard close).
// Deleting a session must not leave a live turn publishing events for a gone
// session or a post-Close tool call re-spawning a leaked shell
// (troubleshoot-5; deviation 94 — upstream lets the main turn run on).
func (e *Engine) Close(sessionID string) {
	e.Abort(sessionID)
	e.mu.Lock()
	e.deleted[sessionID] = struct{}{}
	s, ok := e.shells[sessionID]
	delete(e.shells, sessionID)
	e.mu.Unlock()
	if ok {
		e.settleAndClose(sessionID, s)
	}
}

// settleAndClose waits for the session's in-flight turn to release the busy
// flag (bounded to 2 s; the Close abort unblocks it, so the wait is short in
// practice) before closing the shell. It awaits the settle event (WaitIdle)
// instead of a status poll; the recheck loop covers a fresh turn that takes
// the busy slot in the settle window (the wait stays bounded by the ctx).
func (e *Engine) settleAndClose(sessionID string, s *tool.Shell) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for e.Status(sessionID) == protocol.SessionStatusBusy {
		if err := e.WaitIdle(ctx, sessionID); err != nil {
			break
		}
	}
	_ = s.Close()
}

// Shutdown drains the engine for process exit: it cancels every active
// turn, waits for the turn goroutines to release (at most 5 s, or until
// ctx is done), then releases all session shells.
func (e *Engine) Shutdown(ctx context.Context) {
	e.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(e.busy)+len(e.titleCtx))
	for _, cancel := range e.busy {
		cancels = append(cancels, cancel)
	}
	for _, tc := range e.titleCtx {
		cancels = append(cancels, tc.cancel)
	}
	e.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	deadline := time.Now().Add(5 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	waitCtx, waitCancel := context.WithDeadline(context.Background(), deadline)
	defer waitCancel()
	for {
		e.mu.Lock()
		ids := make([]string, 0, len(e.busy))
		for id := range e.busy {
			ids = append(ids, id)
		}
		e.mu.Unlock()
		if len(ids) == 0 || time.Now().After(deadline) {
			break
		}
		// Await this snapshot's settles instead of ticking: a turn that
		// ends and a replacement that starts in between surface on the
		// next snapshot pass (the wait stays bounded by waitCtx).
		for _, id := range ids {
			if err := e.WaitIdle(waitCtx, id); err != nil {
				break
			}
		}
	}
	// Title side-calls are tracked the same way: the bounded wait keeps
	// Shutdown from returning while a title goroutine can still touch the
	// store or publish session.updated (design-3 fold-in).
	fin := make(chan struct{})
	go func() {
		e.titleWait.Wait()
		close(fin)
	}()
	remain := time.Until(deadline)
	if remain < 0 {
		remain = 0
	}
	select {
	case <-fin:
	case <-time.After(remain):
	}
	e.mu.Lock()
	shells := e.shells
	e.shells = map[string]*tool.Shell{}
	e.mu.Unlock()
	for _, s := range shells {
		_ = s.Close()
	}
}

// endTurn releases the session's busy flag and signals the done channel (the
// WaitIdle awaiters) in one atomic step. It is also the Send failure path:
// the turn goroutine never started, so no client observed a busy — a lone
// idle status would be a transition with no observed start (spec §3.1 B);
// Send's error paths therefore call endTurn without publishing.
func (e *Engine) endTurn(sessionID string) {
	e.mu.Lock()
	delete(e.busy, sessionID)
	if ch := e.turnDone[sessionID]; ch != nil {
		close(ch)
		delete(e.turnDone, sessionID)
	}
	e.mu.Unlock()
}

func (e *Engine) publish(t string, props any) {
	if e.eventSuppressed(props) {
		return
	}
	ev, err := protocol.MakeEvent(t, props)
	if err != nil {
		// A marshal failure here would kill the whole event stream, so it
		// must be diagnosable, not silently dropped.
		e.lg.Error("event marshal failed", "type", t, "error", err)
		return
	}
	e.bus.Publish(ev)
}

// eventSuppressed reports whether the event belongs to a closed (deleted)
// session: post-DELETE the engine must not publish further events for it.
// The engine publishes exactly these five prop shapes, so the switch is
// closed.
func (e *Engine) eventSuppressed(props any) bool {
	var sid string
	switch p := props.(type) {
	case protocol.SessionStatusProps:
		sid = p.SessionID
	case protocol.MessageUpdatedProps:
		sid = p.SessionID
	case protocol.MessagePartUpdatedProps:
		sid = p.SessionID
	case protocol.MessagePartDeltaProps:
		sid = p.SessionID
	case protocol.SessionUpdatedProps:
		sid = p.SessionID
	default:
		return false
	}
	e.mu.Lock()
	_, del := e.deleted[sid]
	e.mu.Unlock()
	return del
}

func (e *Engine) loadCfg(projectDir string) (*protocol.Config, error) {
	if e.cfg == nil {
		return &protocol.Config{}, nil
	}
	return e.cfg(projectDir)
}

func (e *Engine) apiKey(providerID string, cfg *protocol.Config) string {
	if k, ok := auth.ResolveKey(providerID, cfg, os.LookupEnv); ok {
		return k
	}
	return ""
}

func (e *Engine) driverFor(providerID string, m provider.Model) llm.Driver {
	var d llm.Driver
	if dd, ok := e.drivers[providerID]; ok {
		d = dd
	} else {
		d = e.prov.DriverFor(m)
	}
	return loggingDriver{inner: d, provider: providerID, model: m.ID, lg: e.lg}
}

func (e *Engine) limitsFor(cfg *protocol.Config) tool.Limits {
	if cfg != nil && cfg.ToolOutput != nil {
		return tool.Limits{MaxLines: cfg.ToolOutput.MaxLines, MaxBytes: cfg.ToolOutput.MaxBytes}
	}
	return tool.Limits{}
}

// shellFor returns the session's lazily-spawned bash shell, or nil for a
// closed session (a late tool call of the aborted turn gets the handled
// "shell is not initialized" tool error instead of re-spawning a leaked
// shell).
func (e *Engine) shellFor(sessionID, dir string) *tool.Shell {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, del := e.deleted[sessionID]; del {
		return nil
	}
	s, ok := e.shells[sessionID]
	if !ok {
		s = tool.NewShell(dir, tool.Limits{})
		e.shells[sessionID] = s
	}
	return s
}

// turn is the per-turn state threaded through the agent loop: the session
// identity, the resolved provider/model, the turn config + permission rules,
// and the per-turn doom history and tool-step budget (both reset each Send).
type turn struct {
	sessionID string
	agent     string
	row       storage.SessionRow
	info      provider.Info
	model     provider.Model
	cfg       *protocol.Config
	cfgRules  []protocol.Rule
	apiKey    string

	doomHist  []permission.CallKey
	toolCalls int

	// ⑪: per-turn history snapshot — system prompts + the full message
	// history, loaded once at turn start and appended per round, so
	// messagesFor maps memory instead of re-querying every row each round.
	sys  []string
	hist []protocol.MessageWithParts

	// lastMsgID is the current round's assistant message id (set at round
	// creation): the turn's terminal error surfaces on this row when the
	// turn fails (runTurn's deferred exit).
	lastMsgID string
}

// newTurn assembles the per-turn state; the agent defaults to "build".
func newTurn(sessionID string, row storage.SessionRow, info provider.Info, model provider.Model) *turn {
	agent := row.Agent
	if agent == "" {
		agent = "build"
	}
	return &turn{
		sessionID: sessionID,
		agent:     agent,
		row:       row,
		info:      info,
		model:     model,
		cfgRules:  []protocol.Rule{},
	}
}

// runTurn drives the model/tool rounds until the model stops calling tools.
// The session leaves "busy" on exit and onDone fires exactly once.
func (e *Engine) runTurn(ctx context.Context, t *turn, onDone func(error)) {
	var turnErr error
	defer func() {
		if rec := recover(); rec != nil {
			// A panic (tool/driver/DB) must not crash the single binary and
			// must not report false success: the turn finalizes as failed
			// through the normal exit path (idle + onDone(err)).
			turnErr = fmt.Errorf("session: turn panicked: %v", rec)
			e.lg.Error("turn panicked", "session_id", t.sessionID,
				"panic", fmt.Sprintf("%v", rec), "stack", string(debug.Stack()))
		}
		if errors.Is(turnErr, context.Canceled) {
			e.lg.Info("turn aborted", "session_id", t.sessionID, "reason", "context_canceled")
		}
		// Surface the turn failure on the wire BEFORE the idle publish:
		// the TUI's idle-bell condition (c) rings only when not errored, so
		// the error event first suppresses the done-bell — one bell, not
		// two (upstream order is step.failed then idle). The success path
		// keeps its original idle-only order.
		switch {
		case turnErr != nil && t.lastMsgID != "" && !errors.Is(turnErr, errMaxToolRounds):
			e.surfaceTurnError(ctx, t, turnErr)
		case turnErr != nil && t.lastMsgID == "" && !errors.Is(turnErr, errMaxToolRounds):
			// Failure before any round started (e.g. loadHistory): there is
			// no message to attach the error to — the idle status is the
			// surface, so skip silently-ish.
			e.lg.Info("turn failed before any round; the idle status is the surface",
				"session_id", t.sessionID, "error", turnErr)
		}
		e.publish(protocol.EventTypeSessionStatus, protocol.SessionStatusProps{
			SessionID: t.sessionID,
			Status:    protocol.SessionStatus{Type: protocol.SessionStatusIdle},
		})
		e.endTurn(t.sessionID)
		if onDone != nil {
			onDone(turnErr)
		}
	}()
	e.publish(protocol.EventTypeSessionStatus, protocol.SessionStatusProps{
		SessionID: t.sessionID,
		Status:    protocol.SessionStatus{Type: protocol.SessionStatusBusy},
	})

	cfg, cfgErr := e.loadCfg(t.row.ProjectDir)
	if cfgErr == nil && cfg != nil {
		// Invalid permission entries degrade to no config rules (config
		// load is non-fatal per turn).
		if rules, perr := protocol.ParsePerms(cfg.Permission); perr == nil {
			t.cfgRules = rules
		}
	}
	t.cfg = cfg
	if cfgErr != nil {
		e.lg.Error("config load failed", "path", t.row.ProjectDir, "error", cfgErr)
	} else {
		e.lg.Info("config loaded", "path", t.row.ProjectDir)
	}
	e.lg.Info("turn start", "session_id", t.sessionID, "agent", t.agent, "model", t.model.ID)
	if key, source, ok := auth.ResolveKeyWithSource(t.info.ID, t.cfg, os.LookupEnv); ok {
		t.apiKey = key
		e.lg.Info("auth resolved", "provider", t.info.ID, "source", source)
	}
	// ⑪: the history snapshot is loaded once per turn; each round appends
	// its completed assistant message (finishRound).
	sys, hist, herr := e.loadHistory(ctx, t)
	if herr != nil {
		turnErr = herr
		return
	}
	t.sys = sys
	t.hist = hist

	for i := 0; i < maxToolRounds; i++ {
		req, err := e.buildRequest(ctx, t)
		if err != nil {
			turnErr = err
			return
		}
		more, err := e.runRound(ctx, t, req)
		if err != nil {
			// Abort (context.Canceled) is user-initiated, not a failure.
			// The TUI already got the part-level finalization; the send
			// boundary (onDone consumer) is the single log site.
			turnErr = err
			return
		}
		if !more {
			return
		}
		if cerr := ctx.Err(); cerr != nil {
			turnErr = cerr
			return
		}
	}
	turnErr = errMaxToolRounds
}

// surfaceTurnError persists the turn's terminal error on the last round's
// assistant message and re-publishes the FULL message as message.updated
// with Info.Error. It runs in runTurn's deferred exit, before the idle
// status publish (see there for the load-bearing order), and is
// best-effort: a persist or fetch failure logs and still returns so the
// idle path runs (the onDone log line is the single log site for the error
// text).
func (e *Engine) surfaceTurnError(ctx context.Context, t *turn, turnErr error) {
	me := &protocol.MessageError{Type: "unknown", Message: turnErr.Error()}
	if errors.Is(turnErr, context.Canceled) {
		me = &protocol.MessageError{Type: "aborted", Message: "aborted by the user"}
	}
	// The abort path carries a cancelled ctx; WithoutCancel keeps the error
	// write and the row fetch from being dropped (the finishRound idiom).
	dctx := context.WithoutCancel(ctx)
	if err := e.db.SetMessageError(dctx, t.lastMsgID, *me); err != nil {
		e.lg.Error("persist message error failed", "message_id", t.lastMsgID, "session_id", t.sessionID, "error", err)
	}
	row, err := e.db.GetMessage(dctx, t.lastMsgID)
	if err != nil {
		e.lg.Error("load message for error surface failed",
			"message_id", t.lastMsgID, "session_id", t.sessionID, "error", err)
		return
	}
	info := protocol.Message{
		ID: row.ID, SessionID: row.SessionID, Role: row.Role, Agent: row.Agent,
		Cost:   row.Cost,
		Tokens: &row.Tokens,
		Time:   protocol.MessageTime{Created: row.TimeCreated},
	}
	if row.TimeCompleted != nil {
		info.Time.Completed = *row.TimeCompleted
	}
	// The TUI store's upsertMessage REPLACES the whole Info, so the
	// re-publish carries the full message, not just the error field
	// (a minimal publish would wipe role/time/agent/cost from the row).
	info.Error = me
	e.publish(protocol.EventTypeMessageUpdated, protocol.MessageUpdatedProps{SessionID: t.sessionID, Info: info})
}

// buildRequest assembles the next model request: system prompt entries, the
// persisted history (LOCKED mapping, see messagesFor) and the tool schemas
// visible under the session ruleset (re-read each round so "always" replies
// and rule changes apply from the next round).
func (e *Engine) buildRequest(ctx context.Context, t *turn) (llm.Request, error) {
	messages, err := e.messagesFor(t)
	if err != nil {
		return llm.Request{}, err
	}
	tools, err := e.toolSchemaList(ctx, t.sessionID)
	if err != nil {
		return llm.Request{}, err
	}
	return llm.Request{
		Model:    t.model.ID,
		APIKey:   t.apiKey,
		BaseURL:  t.info.BaseURL,
		Messages: messages,
		Tools:    tools,
	}, nil
}

func (e *Engine) loadHistory(ctx context.Context, t *turn) ([]string, []protocol.MessageWithParts, error) {
	sys, err := BuildSystemPrompt(t.row.ProjectDir, t.model.ID, t.info.ID)
	if err != nil {
		return nil, nil, err
	}
	if t.cfg != nil {
		for _, p := range t.cfg.Instructions {
			abs := p
			if !filepath.IsAbs(abs) {
				abs = filepath.Join(t.row.ProjectDir, abs)
			}
			if b, err := os.ReadFile(abs); err == nil {
				sys = append(sys, string(b))
			}
		}
	}
	rows, err := e.db.ListMessages(ctx, t.sessionID)
	if err != nil {
		return nil, nil, err
	}
	hist := make([]protocol.MessageWithParts, 0, len(rows))
	for _, r := range rows {
		prs, err := e.db.ListParts(ctx, r.ID)
		if err != nil {
			return nil, nil, err
		}
		parts := make([]protocol.Part, 0, len(prs))
		for _, pr := range prs {
			p, err := storage.PartToProtocol(pr)
			if err != nil {
				return nil, nil, err
			}
			if isSyntheticPart(p) {
				// Engine-generated notes (error/overflow) are excluded from
				// history replay: the model must never see them.
				continue
			}
			parts = append(parts, p)
		}
		hist = append(hist, protocol.MessageWithParts{
			Info: protocol.Message{
				ID: r.ID, SessionID: r.SessionID, Role: r.Role, Agent: r.Agent,
				Time: protocol.MessageTime{Created: r.TimeCreated},
			},
			Parts: parts,
		})
	}
	return sys, hist, nil
}
