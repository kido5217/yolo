package session

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
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
var ErrSessionBusy = errors.New("session busy")

// errRoundEnded marks a round already finalized inside openStream (the
// overflow path): the caller ends the turn idle without reading a stream.
var errRoundEnded = errors.New("round ended")

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

// titleCancel carries one title stream's cancel as a pointer so a stale
// title's exit can check identity against the tracked entry (func values
// are incomparable; only == nil is allowed on them).
type titleCancel struct{ cancel context.CancelFunc }

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

	mu        sync.Mutex
	busy      map[string]context.CancelFunc
	shells    map[string]*tool.Shell
	titleCtx  map[string]*titleCancel
	titleWait sync.WaitGroup
	deleted   map[string]struct{}
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
	row, err := e.db.GetSession(sessionID)
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
	e.mu.Unlock()

	now := e.clock()
	msgID := protocol.NewID("msg")
	partID := protocol.NewID("prt")
	if err := e.db.CreateMessage(storage.MessageRow{
		ID: msgID, SessionID: sessionID, Role: "user", Agent: row.Agent, TimeCreated: now,
	}); err != nil {
		e.releaseBusy(sessionID)
		return SendResult{}, err
	}
	userPart := protocol.Part{
		ID: partID, SessionID: sessionID, MessageID: msgID,
		Type: "text", Text: text, Time: protocol.PartTime{Start: now},
	}
	userPartRow, err := storage.ProtocolToPart(userPart)
	if err != nil {
		e.releaseBusy(sessionID)
		return SendResult{}, fmt.Errorf("session: persist user part: %w", err)
	}
	if err := e.db.UpsertPart(userPartRow); err != nil {
		e.releaseBusy(sessionID)
		return SendResult{}, err
	}
	userMsg := protocol.Message{
		ID: msgID, SessionID: sessionID, Role: "user", Agent: row.Agent,
		Time:  protocol.MessageTime{Created: now},
		Model: &protocol.MessageModel{ProviderID: info.ID, ModelID: model.ID},
	}
	e.publish(protocol.EventTypeMessageUpdated, protocol.MessageUpdatedProps{SessionID: sessionID, Info: userMsg})
	e.publish(protocol.EventTypeMessagePartUpdated, protocol.MessagePartUpdatedProps{SessionID: sessionID, Part: userPart, Time: now})

	t := newTurn(sessionID, row, info, model)
	e.maybeScheduleTitle(t, text)

	go e.runTurn(turnCtx, t, onDone)
	return SendResult{MessageID: msgID, PartID: partID}, nil
}

// Status reports "idle" or "busy" for the session.
func (e *Engine) Status(sessionID string) string {
	e.mu.Lock()
	_, active := e.busy[sessionID]
	e.mu.Unlock()
	if active {
		return protocol.StatusBusy
	}
	return protocol.StatusIdle
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
// practice) before closing the shell.
func (e *Engine) settleAndClose(sessionID string, s *tool.Shell) {
	deadline := time.Now().Add(2 * time.Second)
	for {
		e.mu.Lock()
		_, busy := e.busy[sessionID]
		e.mu.Unlock()
		if !busy || time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
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
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		e.mu.Lock()
		drained := len(e.busy) == 0
		e.mu.Unlock()
		if drained || time.Now().After(deadline) {
			break
		}
		select {
		case <-ctx.Done():
		case <-tick.C:
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

// releaseBusy drops the session's busy entry without publishing a status:
// the turn goroutine never started, so no client observed a busy — a lone
// idle would be a transition with no observed start (spec §3.1 B).
func (e *Engine) releaseBusy(sessionID string) {
	e.mu.Lock()
	delete(e.busy, sessionID)
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
}

// round is one model round's assistant message state: the row id, its
// creation time, and the message identity re-published on finalize.
type round struct {
	id  string
	now int64
	msg protocol.Message
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
		e.publish(protocol.EventTypeSessionStatus, protocol.SessionStatusProps{
			SessionID: t.sessionID,
			Status:    protocol.SessionStatus{Type: protocol.StatusIdle},
		})
		e.mu.Lock()
		delete(e.busy, t.sessionID)
		e.mu.Unlock()
		if onDone != nil {
			onDone(turnErr)
		}
	}()
	e.publish(protocol.EventTypeSessionStatus, protocol.SessionStatusProps{
		SessionID: t.sessionID,
		Status:    protocol.SessionStatus{Type: protocol.StatusBusy},
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
	sys, hist, herr := e.loadHistory(t)
	if herr != nil {
		turnErr = herr
		return
	}
	t.sys = sys
	t.hist = hist

	for i := 0; i < maxToolRounds; i++ {
		req, err := e.buildRequest(t)
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
	turnErr = errors.New("session: max tool rounds exceeded")
}

// buildRequest assembles the next model request: system prompt entries, the
// persisted history (LOCKED mapping, see messagesFor) and the tool schemas
// visible under the session ruleset (re-read each round so "always" replies
// and rule changes apply from the next round).
func (e *Engine) buildRequest(t *turn) (llm.Request, error) {
	messages, err := e.messagesFor(t)
	if err != nil {
		return llm.Request{}, err
	}
	tools, err := e.toolSchemaList(t.sessionID)
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

// messagesFor maps the persisted history onto the LLM request.
//
// LOCKED mapping (plan Task 16):
//   - system prompt entries lead as separate RoleSystem messages;
//   - user messages join their text parts with "\n"; plan reminders attach to
//     the LAST user message;
//   - assistant messages carry text only (reasoning excluded) plus ToolCalls
//     derived from their completed/error tool parts (Args = persisted state
//     input);
//   - every tool part produces one RoleTool message right after its assistant
//     (completed -> output, error -> error text);
//   - empty assistant messages are skipped;
//   - the request mirrors the persisted history 1:1 (upstream
//     message-v2.toModelMessagesEffect): a tool round ends with the TOOL
//     result — the user message is NEVER re-appended (deviation 77: the
//     plan's re-append made the model see its instruction re-issued every
//     round, which looped weak models into re-running tools).
//
// loadHistory builds the turn's system prompts and the full in-memory
// history snapshot once (⑪). messagesFor maps this snapshot; the mapping is
// unchanged (LOCKED).
func (e *Engine) loadHistory(t *turn) ([]string, []protocol.MessageWithParts, error) {
	sys, err := BuildSystemPrompt(t.row.ProjectDir, t.model, t.model.ID, t.info.ID)
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
	rows, err := e.db.ListMessages(t.sessionID)
	if err != nil {
		return nil, nil, err
	}
	hist := make([]protocol.MessageWithParts, 0, len(rows))
	for _, r := range rows {
		prs, err := e.db.ListParts(r.ID)
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

func (e *Engine) messagesFor(t *turn) ([]llm.Message, error) {
	out := make([]llm.Message, 0, len(t.sys)+8)
	for _, s := range t.sys {
		out = append(out, llm.Message{Role: llm.RoleSystem, Content: s})
	}
	lastUserIdx := -1
	for i := range t.hist {
		if t.hist[i].Info.Role == "user" {
			lastUserIdx = i
		}
	}
	reminders := PlanReminders(t.hist, t.agent)
	for i, mw := range t.hist {
		switch mw.Info.Role {
		case "user":
			content := joinTextParts(mw.Parts)
			if i == lastUserIdx {
				content = appendReminders(content, reminders)
			}
			out = append(out, llm.Message{Role: llm.RoleUser, Content: content})
		case "assistant":
			var texts []string
			var calls []llm.ToolCall
			var toolMsgs []llm.Message
			for _, p := range mw.Parts {
				switch {
				case p.Type == "text" && p.Text != "":
					texts = append(texts, p.Text)
				case p.Type == "tool" && p.State != nil &&
					(p.State.Status == "completed" || p.State.Status == "error"):
					args, err := json.Marshal(p.State.Input)
					if err != nil || len(args) == 0 {
						args = json.RawMessage("{}")
					}
					calls = append(calls, llm.ToolCall{ID: p.ID, Name: p.Tool, Args: args})
					content := p.State.Output
					if p.State.Status == "error" {
						content = p.State.Error
					}
					toolMsgs = append(toolMsgs, llm.Message{Role: llm.RoleTool, ToolCallID: p.ID, Content: content})
				}
			}
			if len(texts) == 0 && len(calls) == 0 {
				continue
			}
			out = append(out, llm.Message{Role: llm.RoleAssistant, Content: strings.Join(texts, "\n"), ToolCalls: calls})
			if len(toolMsgs) > 0 {
				out = append(out, toolMsgs...)
			}
		}
	}
	return out, nil
}

func joinTextParts(parts []protocol.Part) string {
	texts := make([]string, 0, len(parts))
	for _, p := range parts {
		if p.Type == "text" && p.Text != "" {
			texts = append(texts, p.Text)
		}
	}
	return strings.Join(texts, "\n")
}

func appendReminders(content string, reminders []string) string {
	if len(reminders) == 0 {
		return content
	}
	blocks := strings.Join(reminders, "\n\n")
	if content == "" {
		return blocks
	}
	return content + "\n\n" + blocks
}

// runRound streams one model round into a new assistant message. Part
// bookkeeping: the current text/reasoning part is created on its first delta
// (one message.part.updated), upserted per delta with a message.part.delta
// event, and finalized (message.part.updated with the end time) at round
// end. Tool parts are created "running" before execution and finalized
// "completed"/"error" afterwards. The tool part id IS the model call id:
// call ids are not persisted elsewhere, and the history replay needs them to
// pair assistant ToolCalls with RoleTool results.
//
// Lifecycle (LOCKED, plan Task 18): pre-stream transient failures retry up
// to maxRetryAttempts with backoff (emitting session.status retry) while
// no part of the round is persisted; a mid-stream failure fails the turn
// (no retry) keeping the partial text; context overflow (usage or API 400)
// stops the turn with a synthetic note; the per-turn tool step budget ends
// the turn idle before the next call beyond it is executed.
func (e *Engine) runRound(ctx context.Context, t *turn, req llm.Request) (bool, error) {
	// Per-round context: the real drivers' stream goroutines block their
	// send on this ctx, so cancelling it on every round exit unblocks any
	// abandoned stream (e.g. the tool-step budget drop) instead of leaking
	// its goroutine and connection until process shutdown.
	roundCtx, roundCancel := context.WithCancel(ctx)
	defer roundCancel()
	// The assistant row exists before the first stream attempt so a failed
	// round still finalizes a (possibly empty) assistant message.
	r := &round{id: protocol.NewID("msg"), now: e.clock()}
	if err := e.db.CreateMessage(storage.MessageRow{
		ID: r.id, SessionID: t.sessionID, Role: "assistant", Agent: t.agent, TimeCreated: r.now,
	}); err != nil {
		return false, err
	}
	r.msg = protocol.Message{
		ID: r.id, SessionID: t.sessionID, Role: "assistant", Agent: t.agent,
		Time:  protocol.MessageTime{Created: r.now},
		Model: &protocol.MessageModel{ProviderID: t.info.ID, ModelID: t.model.ID},
	}
	e.publish(protocol.EventTypeMessageUpdated, protocol.MessageUpdatedProps{
		SessionID: t.sessionID, Info: r.msg,
	})
	e.lg.Info("round start", "session_id", t.sessionID, "round", r.id)

	stream, err := e.openStream(roundCtx, t, r, req)
	if err != nil {
		if err == errRoundEnded {
			return false, nil
		}
		return false, err
	}

	type textState struct {
		id    string
		start int64
		buf   strings.Builder
	}
	var textSt, reasonSt textState

	saveDelta := func(st *textState, kind, delta string) {
		// ⑩: no per-delta DB write (O(n²) for long responses); the text
		// accumulates in st.buf and finalizePart is the sole upsert. The
		// wire (delta event) is unchanged; a crash mid-turn loses the
		// in-flight text (accepted trade, spec §4).
		st.buf.WriteString(delta)
		e.publish(protocol.EventTypeMessagePartDelta, protocol.MessagePartDeltaProps{
			SessionID: t.sessionID, MessageID: r.id, PartID: st.id, Field: kind, Delta: delta,
		})
	}
	startPart := func(st *textState, kind, delta string) {
		st.id = protocol.NewID("prt")
		st.start = e.clock()
		st.buf.WriteString(delta)
		p := protocol.Part{
			ID: st.id, SessionID: t.sessionID, MessageID: r.id,
			Type: kind, Text: st.buf.String(),
			Time: protocol.PartTime{Start: st.start},
		}
		// ⑩: created+delta go to the wire only; finalizePart persists.
		e.publish(protocol.EventTypeMessagePartUpdated, protocol.MessagePartUpdatedProps{
			SessionID: t.sessionID, Part: p, Time: e.clock(),
		})
		e.publish(protocol.EventTypeMessagePartDelta, protocol.MessagePartDeltaProps{
			SessionID: t.sessionID, MessageID: r.id, PartID: st.id, Field: kind, Delta: delta,
		})
	}
	finalizePart := func(st *textState, kind string) {
		if st.id == "" {
			return
		}
		p := protocol.Part{
			ID: st.id, SessionID: t.sessionID, MessageID: r.id,
			Type: kind, Text: st.buf.String(),
			Time: protocol.PartTime{Start: st.start, End: e.clock()},
		}
		row, perr := storage.ProtocolToPart(p)
		if perr != nil {
			e.lg.Error("persist part marshal failed", "part_id", p.ID, "session_id", t.sessionID, "error", perr)
			return
		}
		if err := e.db.UpsertPart(row); err != nil {
			e.lg.Error("persist part failed", "part_id", p.ID, "session_id", t.sessionID, "error", err)
		}
		e.publish(protocol.EventTypeMessagePartUpdated, protocol.MessagePartUpdatedProps{
			SessionID: t.sessionID, Part: p, Time: e.clock(),
		})
	}

	var (
		usage       *llm.Usage
		finish      string
		sawToolPart bool
	)
	for {
		p, err := stream.Next(roundCtx)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			// Stream loss (in practice ctx cancel: Abort). Partial text is
			// kept; the turn ends with the ctx error (log only, non-fatal).
			finalizePart(&textSt, "text")
			finalizePart(&reasonSt, "reasoning")
			e.finishRound(t, r, usage, finish)
			return false, err
		}
		if p.Usage != nil {
			usage = p.Usage
		}
		if p.Finish != "" {
			finish = p.Finish
		}
		if p.Err != nil {
			finalizePart(&textSt, "text")
			finalizePart(&reasonSt, "reasoning")
			if ctx.Err() != nil {
				e.finishRound(t, r, usage, finish)
				return false, ctx.Err()
			}
			if isOverflowError(p.Err) {
				e.lg.Info("overflow detected", "session_id", t.sessionID, "model", t.model.ID, "reason", "api_error")
				e.saveSynthetic(t, r, overflowNote(t.model, 0, p.Err))
				e.finishRound(t, r, usage, finish)
				return false, nil
			}
			// Mid-stream failure after content: keep the partial text, note
			// the error on a synthetic part (excluded from history replay —
			// the model never sees it) and fail the turn (no retry).
			e.saveSynthetic(t, r, p.Err.Error())
			e.finishRound(t, r, usage, finish)
			return false, fmt.Errorf("llm stream error: %w", p.Err)
		}
		switch p.Kind {
		case "text":
			if p.Text == "" {
				continue
			}
			if textSt.id == "" {
				startPart(&textSt, "text", p.Text)
			} else {
				saveDelta(&textSt, "text", p.Text)
			}
		case "reasoning":
			if p.Text == "" {
				continue
			}
			if reasonSt.id == "" {
				startPart(&reasonSt, "reasoning", p.Text)
			} else {
				saveDelta(&reasonSt, "reasoning", p.Text)
			}
		case "tool":
			sawToolPart = true
			finalizePart(&textSt, "text")
			finalizePart(&reasonSt, "reasoning")
			// A tool round that continues the text stream after the tool
			// call starts a NEW text block (fresh part id, upstream parity)
			// instead of re-using the finalized part's id (troubleshoot-3).
			textSt.id = ""
			textSt.buf.Reset()
			reasonSt.id = ""
			reasonSt.buf.Reset()
			if t.toolCalls >= maxToolSteps {
				// Step budget exhausted: the remaining calls of this stream
				// are dropped (not persisted, not executed); the turn ends
				// idle and onDone(nil).
				e.lg.Info("max tool steps reached", "session_id", t.sessionID, "steps", maxToolSteps)
				e.finishRound(t, r, usage, finish)
				return false, nil
			}
			t.toolCalls++
			e.executeTool(roundCtx, t, r, p)
		}
	}
	finalizePart(&textSt, "text")
	finalizePart(&reasonSt, "reasoning")
	if finish == "" {
		finish = "stop"
	}
	// Overflow: the round's input already exceeds the model context; the
	// turn ends with a synthetic note (v1 has no compaction).
	if usage != nil && t.model.Context > 0 && usage.Input > t.model.Context {
		e.lg.Info("overflow detected", "session_id", t.sessionID, "model", t.model.ID, "reason", "usage", "input", usage.Input)
		e.saveSynthetic(t, r, overflowNote(t.model, usage.Input, nil))
		e.finishRound(t, r, usage, finish)
		return false, nil
	}
	e.finishRound(t, r, usage, finish)

	// A round continues when the model finished with tool_calls or emitted
	// any tool part (scripted drivers set Finish inconsistently).
	return finish == "tool_calls" || sawToolPart, nil
}

// openStream starts the model stream and retries pre-stream transient
// failures (429/5xx/net) with backoff while nothing of the round is
// persisted (emitting session.status retry). Every failure path finalizes
// the assistant message first; overflow 400s end the round with a synthetic
// note and no error (the turn ends idle).
func (e *Engine) openStream(ctx context.Context, t *turn, r *round, req llm.Request) (llm.PartStream, error) {
	drv := e.driverFor(t.info.ID, t.model)
	// One reusable timer for the pre-stream retry backoffs (no fresh
	// allocation per attempt); the zero timer has already fired, so
	// drain that tick before the first Reset re-arms it.
	retry := time.NewTimer(0)
	<-retry.C
	defer retry.Stop()
	var stream llm.PartStream
	for attempt := 1; ; attempt++ {
		var sErr error
		stream, sErr = drv.Stream(ctx, req)
		if sErr == nil {
			return stream, nil
		}
		if ctx.Err() != nil {
			e.finishRound(t, r, nil, "")
			return llm.PartStream{}, ctx.Err()
		}
		if !llm.IsTransient(sErr) {
			if isOverflowError(sErr) {
				e.lg.Info("overflow detected", "session_id", t.sessionID, "model", t.model.ID, "reason", "api_error")
				e.saveSynthetic(t, r, overflowNote(t.model, 0, sErr))
				e.finishRound(t, r, nil, "")
				// The round is already finalized: the caller ends the
				// turn idle without reading a stream.
				return llm.PartStream{}, errRoundEnded
			}
			// Pre-stream failure: keep the decoded provider text on a
			// synthetic note (excluded from history replay) and fail the
			// turn (mid-stream parity).
			e.saveSynthetic(t, r, sErr.Error())
			e.finishRound(t, r, nil, "")
			return llm.PartStream{}, sErr
		}
		if attempt >= maxRetryAttempts {
			e.finishRound(t, r, nil, "")
			// Retry-exhaustion framing belongs in the boundary error: the
			// send boundary logs the turn error exactly once.
			return llm.PartStream{}, fmt.Errorf(
				"transient retries exhausted after %d attempts (session=%s): %w",
				maxRetryAttempts, t.sessionID, sErr)
		}
		delay := e.backoff(attempt)
		e.publish(protocol.EventTypeSessionStatus, protocol.SessionStatusProps{
			SessionID: t.sessionID,
			Status: protocol.SessionStatus{
				Type: protocol.StatusRetry, Attempt: attempt,
				Message: sErr.Error(), Next: delay.Milliseconds(),
			},
		})
		retry.Reset(delay)
		select {
		case <-retry.C:
		case <-ctx.Done():
			e.finishRound(t, r, nil, "")
			return llm.PartStream{}, ctx.Err()
		}
	}
}

// finishRound completes the assistant message row and re-publishes
// message.updated with the final state, deriving cost/tokens from the round's
// usage (nil-safe). It is called on every round-exit path (success, failure,
// abort, retry exhaustion, overflow, max-steps).
func (e *Engine) finishRound(t *turn, r *round, usage *llm.Usage, finish string) {
	var tok protocol.Tokens
	cost := 0.0
	if usage != nil {
		tok = protocol.Tokens{
			Input:     int64(usage.Input),
			Output:    int64(usage.Output),
			Reasoning: int64(usage.Reasoning),
			Cache:     protocol.CacheTokens{Read: int64(usage.CacheRead), Write: int64(usage.CacheWrite)},
		}
		cost = (float64(usage.Input)*t.model.CostIn + float64(usage.Output)*t.model.CostOut +
			float64(usage.CacheRead)*t.model.CostCacheRead + float64(usage.CacheWrite)*t.model.CostCacheWrite) / 1e6
	}
	end := e.clock()
	e.lg.Info("round end", "session_id", t.sessionID, "round", r.id, "latency_ms", end-r.now, "finish", finish)
	if err := e.db.UpdateMessage(storage.MessageRow{
		ID: r.id, SessionID: t.sessionID, Role: "assistant", Agent: t.agent,
		Cost: cost, Tokens: tok, TimeCreated: r.now, TimeCompleted: &end,
	}); err != nil {
		// Best-effort: a failed write must not strand the TUI in a
		// cost-less/incomplete final state, so the final message.updated
		// still goes out.
		e.lg.Error("update message failed", "message_id", r.id, "session_id", t.sessionID, "error", err)
	}
	r.msg.Cost = cost
	r.msg.Tokens = &tok
	if finish != "" {
		r.msg.Finish = finish
	}
	r.msg.Time.Completed = end
	e.publish(protocol.EventTypeMessageUpdated, protocol.MessageUpdatedProps{
		SessionID: t.sessionID, Info: r.msg,
	})
	// ⑪: append the completed round to the turn's in-memory snapshot so the
	// next round's request sees it without a DB re-query.
	if mw, aerr := e.roundAsMessage(t, r); aerr == nil {
		t.hist = append(t.hist, mw)
	}
}

// roundAsMessage builds the snapshot entry for a completed assistant round:
// its final message info + non-synthetic parts (⑪).
func (e *Engine) roundAsMessage(t *turn, r *round) (protocol.MessageWithParts, error) {
	prs, err := e.db.ListParts(r.id)
	if err != nil {
		return protocol.MessageWithParts{}, err
	}
	parts := make([]protocol.Part, 0, len(prs))
	for _, pr := range prs {
		p, err := storage.PartToProtocol(pr)
		if err != nil {
			return protocol.MessageWithParts{}, err
		}
		if isSyntheticPart(p) {
			continue
		}
		parts = append(parts, p)
	}
	return protocol.MessageWithParts{Info: r.msg, Parts: parts}, nil
}

// saveSynthetic persists an engine-generated text part (mid-stream error
// note, overflow note) flagged Synthetic: it shows in the TUI but
// messagesFor excludes it from history replay, so the model never sees it.
func (e *Engine) saveSynthetic(t *turn, r *round, text string) {
	syn := true
	start := e.clock()
	p := protocol.Part{
		ID: protocol.NewID("prt"), SessionID: t.sessionID, MessageID: r.id,
		Type: "text", Text: text, Synthetic: &syn,
		Time: protocol.PartTime{Start: start, End: e.clock()},
	}
	row, perr := storage.ProtocolToPart(p)
	if perr != nil {
		e.lg.Error("persist part marshal failed", "part_id", p.ID, "session_id", t.sessionID, "error", perr)
		return
	}
	if err := e.db.UpsertPart(row); err != nil {
		e.lg.Error("persist part failed", "part_id", p.ID, "session_id", t.sessionID, "error", err)
	}
	e.publish(protocol.EventTypeMessagePartUpdated, protocol.MessagePartUpdatedProps{
		SessionID: t.sessionID, Part: p, Time: e.clock(),
	})
}

// isSyntheticPart reports whether a part is engine-generated and excluded
// from history replay.
func isSyntheticPart(p protocol.Part) bool {
	return p.Synthetic != nil && *p.Synthetic
}

// overflowPatterns ports opencode v1.18.18's curated context-overflow
// classifier (packages/llm/src/provider-error.ts `patterns`) byte-faithfully:
// 27 entries, case-insensitive.
var overflowPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)prompt is too long`),
	regexp.MustCompile(`(?i)request_too_large`),
	regexp.MustCompile(`(?i)input is too long for requested model`),
	regexp.MustCompile(`(?i)exceeds the context window`),
	regexp.MustCompile(`(?i)exceeds (?:the )?(?:model'?s )?maximum context length(?: of [\d,]+ tokens?|\s*\([\d,]+\))`),
	regexp.MustCompile(`(?i)input token count.*exceeds the maximum`),
	regexp.MustCompile(`(?i)tokens in request more than max tokens allowed`),
	regexp.MustCompile(`(?i)maximum prompt length is \d+`),
	regexp.MustCompile(`(?i)reduce the length of the messages`),
	regexp.MustCompile(`(?i)maximum context length is \d+ tokens`),
	regexp.MustCompile(`(?i)exceeds (?:the )?maximum allowed input length of [\d,]+ tokens?`),
	regexp.MustCompile(`(?i)input \(\d+ tokens\) is longer than the model'?s context length \(\d+ tokens\)`),
	regexp.MustCompile(`(?i)exceeds the limit of \d+`),
	regexp.MustCompile(`(?i)exceeds the available context size`),
	regexp.MustCompile(`(?i)greater than the context length`),
	regexp.MustCompile(`(?i)context window exceeds limit`),
	regexp.MustCompile(`(?i)exceeded model token limit`),
	regexp.MustCompile(`(?i)context[_ ]length[_ ]exceeded`),
	regexp.MustCompile(`(?i)request entity too large`),
	regexp.MustCompile(`(?i)context length is only \d+ tokens`),
	regexp.MustCompile(`(?i)input length.*exceeds.*context length`),
	regexp.MustCompile(`(?i)prompt too long; exceeded (?:max )?context length`),
	regexp.MustCompile(`(?i)too large for model with \d+ maximum context length`),
	regexp.MustCompile(`(?i)prompt has [\d,]+ tokens?, but the configured context size is [\d,]+ tokens?`),
	regexp.MustCompile(`(?i)model_context_window_exceeded`),
	regexp.MustCompile(`(?i)too many tokens`),
	regexp.MustCompile(`(?i)token limit exceeded`),
}

// overflowExclusions — upstream `exclusions` (AND-NOT: a hit means NOT
// overflow, even if a pattern also matches).
var overflowExclusions = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^(throttling error|service unavailable):`),
	regexp.MustCompile(`(?i)rate limit`),
	regexp.MustCompile(`(?i)too many requests`),
}

// overflowNoBodyRe — the upstream synthesized message form for a bare
// 400/413 with no body.
var overflowNoBodyRe = regexp.MustCompile(`(?i)^4(00|13)\s*(status code)?\s*\(no body\)`)

// isOverflowError reports whether an API (non-stream) error is a
// context-overflow rejection. Port of upstream provider-error.ts
// isContextOverflow + opencode provider/error.ts parseAPICallError:
// exclusions AND-NOT the curated patterns; a 413 (any body), a 400/413 with
// an empty body, or a decoded body whose error.code is
// "context_length_exceeded" is overflow by status. Task ④'s decoded
// *llm.APIError makes the provider 400 path live again.
func isOverflowError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, re := range overflowExclusions {
		if re.MatchString(msg) {
			return false
		}
	}
	texts := []string{msg}
	var api *llm.APIError
	if errors.As(err, &api) {
		switch {
		case api.Status == 413:
			return true
		case (api.Status == 400 || api.Status == 413) && len(bytes.TrimSpace(api.Body)) == 0:
			return true
		}
		var env struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if json.Unmarshal(api.Body, &env) == nil && env.Error.Code == "context_length_exceeded" {
			return true
		}
		// Upstream's classifier input (error.ts message()) includes the
		// raw response body when the decoded message is unhelpful, so the
		// curated patterns also run against the body (e.g. a
		// model_context_window_exceeded code with a short message).
		if len(api.Body) > 0 {
			texts = append(texts, string(api.Body))
		}
	}
	for _, text := range texts {
		for _, re := range overflowPatterns {
			if re.MatchString(text) {
				return true
			}
		}
	}
	return overflowNoBodyRe.MatchString(msg)
}

// overflowNote renders the fixed overflow text. input > 0 comes from the
// round's usage; otherwise apiErr carries the provider message.
func overflowNote(model provider.Model, input int, apiErr error) string {
	txt := fmt.Sprintf(
		"context overflow: model context %d exceeded by input %d tokens; the turn stopped. "+
			"(v1 has no compaction — shorten the conversation or pick a larger-context model.)",
		model.Context, input,
	)
	if apiErr != nil {
		txt += "\nupstream: " + apiErr.Error()
	}
	return txt
}

// executeTool runs one model-issued tool call through the LOCKED permission
// gates, then the tool itself:
//
//  1. doom check (sliding 3-identical window on the turn's call history;
//     the doom ask fires BEFORE the part goes "running");
//  2. hidden guard (a tool denied by a wildcard rule is not offered to the
//     model; if it is called anyway, the part errors "tool not available");
//  3. external-directory gate on tool.External paths outside the session
//     dir (the part is "running" first so the TUI shows the pending state);
//  4. core ask with Resources/Always from tool.Patterns.
//
// Every path finalizes the part. Deny -> "permission rejected"; a ctx
// cancel while parked (Abort) -> "aborted"; the model continues either way.
func (e *Engine) executeTool(ctx context.Context, t *turn, r *round, p llm.Part) {
	name := p.Name
	callID := p.CallID
	if callID == "" {
		callID = protocol.NewID("prt")
	}
	raw := p.Args
	if len(raw) == 0 && p.Text != "" {
		// scripted drivers carry the args JSON in Text (locked convention)
		raw = json.RawMessage(p.Text)
	}
	fail := func(stage int64, msg string) {
		e.saveToolPart(t, r, toolPart{
			callID: callID,
			name:   name,
			state: protocol.ToolState{
				Status: "error",
				Input:  map[string]any{},
				Error:  msg,
				Time:   protocol.PartTime{Start: e.clock(), End: stage},
			},
		})
	}
	// gateFail finalizes the part for a failed permission gate (service
	// error, deny, or ctx cancel while parked).
	gateFail := func() {
		msg := "permission rejected"
		if ctx.Err() != nil {
			msg = "aborted"
		}
		fail(e.clock(), msg)
	}

	input := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &input); err != nil {
			fail(e.clock(), "invalid tool arguments: "+err.Error())
			return
		}
	}
	tl, ok := e.tools[name]
	if !ok {
		start := e.clock()
		fail(start, "unknown tool "+name)
		return
	}
	rules, err := e.rulesetForRow(t.row)
	if err != nil {
		fail(e.clock(), err.Error())
		return
	}
	if hidden := permission.Hidden(rules, []string{name})[name]; hidden {
		start := e.clock()
		fail(start, "tool not available")
		return
	}
	resources, always, err := tl.Patterns(raw)
	if err != nil {
		fail(e.clock(), err.Error())
		return
	}
	external, err := tl.External(raw)
	if err != nil {
		fail(e.clock(), err.Error())
		return
	}

	// (1) doom check: the third identical call of the turn asks before it
	// runs (sliding window; a "once" reply does not extend the exemption).
	key := permission.CallKey{Tool: name, Hash: callKeyHash(raw)}
	if permission.DoomLoopDue(t.doomHist, key) {
		e.lg.Info("doom loop trigger", "session_id", t.sessionID, "tool", name)
		d := e.perm.EvaluateRules(t.agent, t.cfgRules, "doom_loop", []string{name})
		doomReq := permission.Request{
			RequestID: protocol.NewID("perm"), SessionID: t.sessionID, Agent: t.agent,
			Permission: "doom_loop", Tool: tl.ID(),
			Resources: []string{name},
			CallID:    callID, MessageID: r.id,
			PreDecision: d, CreatedAt: e.clock(),
			CfgRules: t.cfgRules,
		}
		decision, err := e.perm.Ask(ctx, doomReq)
		if err != nil {
			fail(e.clock(), err.Error())
			return
		}
		if decision != permission.Allow {
			msg := "permission rejected"
			if ctx.Err() != nil {
				msg = "aborted"
			}
			now := e.clock()
			e.saveToolPart(t, r, toolPart{
				callID: callID,
				name:   name,
				state: protocol.ToolState{
					Status:   "error",
					Input:    input,
					Error:    msg,
					Metadata: map[string]any{"reason": "doom_loop"},
					Time:     protocol.PartTime{Start: now, End: now},
				},
			})
			t.doomHist = append(t.doomHist, key)
			return
		}
	}
	t.doomHist = append(t.doomHist, key)

	start := e.clock()
	e.saveToolPart(t, r, toolPart{
		callID: callID,
		name:   name,
		state:  protocol.ToolState{Status: "running", Input: input, Time: protocol.PartTime{Start: start}},
	})

	// (3) external-directory gate.
	for _, ext := range external {
		abs := ext
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(t.row.ProjectDir, abs)
		}
		abs = filepath.Clean(abs)
		if inside, _ := withinDir(t.row.ProjectDir, abs); inside {
			continue
		}
		pattern := filepath.Dir(abs) + "/*"
		d := e.perm.EvaluateRules(t.agent, t.cfgRules, "external_directory", []string{pattern})
		extReq := permission.Request{
			RequestID: protocol.NewID("perm"), SessionID: t.sessionID, Agent: t.agent,
			Permission: "external_directory", Tool: tl.ID(),
			Resources: []string{pattern},
			CallID:    callID, MessageID: r.id,
			PreDecision: d, CreatedAt: e.clock(),
			CfgRules: t.cfgRules,
		}
		decision, aerr := e.perm.Ask(ctx, extReq)
		if aerr != nil || decision != permission.Allow {
			gateFail()
			return
		}
	}

	// (4) core permission.
	d := e.perm.EvaluateRules(t.agent, t.cfgRules, tl.Permission(), resources)
	preq := permission.Request{
		RequestID: protocol.NewID("perm"), SessionID: t.sessionID, Agent: t.agent,
		Permission: tl.Permission(), Tool: tl.ID(),
		Resources: resources, Always: always,
		CallID: callID, MessageID: r.id,
		PreDecision: d, CreatedAt: e.clock(),
		CfgRules: t.cfgRules,
	}
	decision, err := e.perm.Ask(ctx, preq)
	if err != nil {
		fail(e.clock(), err.Error())
		return
	}
	if decision != permission.Allow {
		gateFail()
		return
	}

	env := &tool.Env{
		Dir:       t.row.ProjectDir,
		Shell:     e.shellFor(t.sessionID, t.row.ProjectDir),
		Limits:    e.limitsFor(t.cfg),
		OutputDir: e.outputDir,
		Storage:   e.db,
		SessionID: t.sessionID,
		Log:       e.lg,
	}
	e.lg.Info("tool start", "session_id", t.sessionID, "tool", name)
	toolStart := e.clock()
	out, runErr := tl.Run(ctx, raw, env)
	if runErr != nil {
		msg := runErr.Error()
		if ctx.Err() != nil {
			// Abort while the tool ran: label it plainly; the process was
			// already force-killed via ctx.
			msg = "aborted"
		}
		e.lg.Error("tool failed", "session_id", t.sessionID, "tool", name, "error", msg)
		e.saveToolPart(t, r, toolPart{
			callID: callID,
			name:   name,
			state: protocol.ToolState{
				Status:   "error",
				Input:    input,
				Title:    out.Title,
				Output:   out.Text,
				Error:    msg,
				Metadata: out.Meta,
				Time:     protocol.PartTime{Start: start, End: e.clock()},
			},
		})
		return
	}
	{
		args := []any{"session_id", t.sessionID, "tool", name, "latency_ms", e.clock() - toolStart}
		if v, ok := out.Meta["exit"]; ok {
			args = append(args, "exit_code", v)
		}
		if v, ok := out.Meta["truncated"]; ok {
			args = append(args, "truncated", v)
		}
		e.lg.Info("tool end", args...)
	}
	e.saveToolPart(t, r, toolPart{
		callID: callID,
		name:   name,
		state: protocol.ToolState{
			Status:   "completed",
			Input:    input,
			Title:    out.Title,
			Output:   out.Text,
			Metadata: out.Meta,
			Time:     protocol.PartTime{Start: start, End: e.clock()},
		},
	})
}

// toolPart is one tool part save: the call identity plus the persisted tool
// state (the start/end times live in state.Time).
type toolPart struct {
	callID string
	name   string
	state  protocol.ToolState
}

func (e *Engine) saveToolPart(t *turn, r *round, tp toolPart) {
	p := protocol.Part{
		ID: tp.callID, SessionID: t.sessionID, MessageID: r.id,
		Type: "tool", Tool: tp.name, CallID: tp.callID, State: &tp.state,
	}
	row, perr := storage.ProtocolToPart(p)
	if perr != nil {
		e.lg.Error("persist part marshal failed", "part_id", p.ID, "session_id", t.sessionID, "error", perr)
		return
	}
	if err := e.db.UpsertPart(row); err != nil {
		e.lg.Error("persist part failed", "part_id", p.ID, "session_id", t.sessionID, "error", err)
	}
	e.publish(protocol.EventTypeMessagePartUpdated, protocol.MessagePartUpdatedProps{
		SessionID: t.sessionID, Part: p, Time: e.clock(),
	})
}

// maybeScheduleTitle fires the one-shot title generation for the session's
// first user message when the title is still the default.
func (e *Engine) maybeScheduleTitle(t *turn, userText string) {
	if t.row.Title != "" && t.row.Title != "New session" {
		return
	}
	msgs, err := e.db.ListMessages(t.sessionID)
	if err != nil {
		return
	}
	for _, m := range msgs {
		if m.Role == "assistant" {
			return
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	tc := &titleCancel{cancel: cancel}
	e.mu.Lock()
	e.titleCtx[t.sessionID] = tc
	e.mu.Unlock()
	e.titleWait.Add(1)
	go e.generateTitle(ctx, tc, t, userText)
}

// generateTitle best-effort: errors are dropped (title stays the default).
func (e *Engine) generateTitle(ctx context.Context, tc *titleCancel, t *turn, userText string) {
	defer tc.cancel()
	defer e.titleWait.Done()
	defer e.dropTitleCtx(t.sessionID, tc)
	cfg, _ := e.loadCfg(t.row.ProjectDir)
	req := llm.Request{
		Model:   t.model.ID,
		APIKey:  e.apiKey(t.info.ID, cfg),
		BaseURL: t.info.BaseURL,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: TitlePrompt()},
			{Role: llm.RoleUser, Content: userText},
		},
	}
	stream, err := e.driverFor(t.info.ID, t.model).Stream(ctx, req)
	if err != nil {
		return
	}
	var sb strings.Builder
	for {
		p, err := stream.Next(ctx)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return
		}
		if p.Kind == "text" {
			sb.WriteString(p.Text)
		}
	}
	title := strings.TrimSpace(strings.SplitN(sb.String(), "\n", 2)[0])
	runes := []rune(title)
	if len(runes) > 50 {
		title = string(runes[:50])
	}
	if title == "" {
		return
	}
	if err := e.db.UpdateSession(t.sessionID, storage.SessionRow{Title: title, TimeUpdated: e.clock()}); err != nil {
		return
	}
	updated, err := e.db.GetSession(t.sessionID)
	if err != nil {
		return
	}
	msgs, err := e.db.ListMessages(t.sessionID)
	if err != nil {
		return
	}
	e.publish(protocol.EventTypeSessionUpdated, protocol.SessionUpdatedProps{
		SessionID: t.sessionID,
		Info:      storage.SessionFromRow(updated, msgs),
	})
}

// dropTitleCtx removes the session's tracked title cancel — but only when
// it still holds THIS one: a newer title scheduled in the meantime
// replaces the entry, and a stale exit must not drop the newer cancel (it
// must stay cancellable by Abort/Shutdown).
func (e *Engine) dropTitleCtx(sessionID string, tc *titleCancel) {
	e.mu.Lock()
	if e.titleCtx[sessionID] == tc {
		delete(e.titleCtx, sessionID)
	}
	e.mu.Unlock()
}

// callKeyHash returns the sha256 hex of the canonical (sorted-key) JSON form
// of the tool args, so deep-equal inputs hash equal regardless of key order.
func callKeyHash(raw json.RawMessage) string {
	var v any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &v); err != nil {
			// unparseable args fall through to the raw bytes; the doom window
			// only ever sees well-formed calls (validated before use).
			v = string(raw)
		}
	}
	b, err := json.Marshal(v)
	if err != nil {
		b = raw
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// withinDir reports whether p is inside (or is) root.
func withinDir(root, p string) (bool, error) {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false, err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false, nil
	}
	return true, nil
}
