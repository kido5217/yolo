package session

import (
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
	db      *storage.DB
	bus     *bus.Bus
	prov    *provider.Registry
	perm    *permission.Service
	tools   map[string]tool.Tool
	lg      *log.Logger
	dataDir string
	cfg     func(projectDir string) (*protocol.Config, error)
	drivers map[string]llm.Driver
	clock   func() int64
	backoff func(attempt int) time.Duration

	mu     sync.Mutex
	busy   map[string]context.CancelFunc
	shells map[string]*tool.Shell
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
	clock := d.Clock
	if clock == nil {
		clock = func() int64 { return time.Now().UnixMilli() }
	}
	backoff := d.Backoff
	if backoff == nil {
		backoff = defaultBackoff
	}
	return &Engine{
		db:      d.DB,
		bus:     d.Bus,
		prov:    d.Prov,
		perm:    d.Perm,
		tools:   d.Tools,
		lg:      d.Log,
		dataDir: d.DataDir,
		cfg:     d.Cfg,
		drivers: d.Drivers,
		clock:   clock,
		backoff: backoff,
		busy:    map[string]context.CancelFunc{},
		shells:  map[string]*tool.Shell{},
	}, nil
}

// defaultBackoff is the production retry delay after a failed attempt
// (1-based): 1s × 2^(attempt-1) scaled by a uniform jitter in [0.8, 1.2].
func defaultBackoff(attempt int) time.Duration {
	base := time.Second << uint(attempt-1)
	jitter := 0.8 + 0.4*rand.Float64()
	return time.Duration(float64(base) * jitter)
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
		e.idleAndRelease(sessionID)
		return SendResult{}, err
	}
	userPart := protocol.Part{
		ID: partID, SessionID: sessionID, MessageID: msgID,
		Type: "text", Text: text, Time: protocol.PartTime{Start: now},
	}
	if err := e.db.UpsertPart(storage.ProtocolToPart(userPart)); err != nil {
		e.idleAndRelease(sessionID)
		return SendResult{}, err
	}
	userMsg := protocol.Message{
		ID: msgID, SessionID: sessionID, Role: "user", Agent: row.Agent,
		Time:  protocol.MessageTime{Created: now},
		Model: &protocol.MessageModel{ProviderID: info.ID, ModelID: model.ID},
	}
	e.publish(protocol.EventTypeMessageUpdated, protocol.MessageUpdatedProps{SessionID: sessionID, Info: userMsg})
	e.publish(protocol.EventTypeMessagePartUpdated, protocol.MessagePartUpdatedProps{SessionID: sessionID, Part: userPart, Time: now})

	e.maybeScheduleTitle(sessionID, row, text)

	go e.runTurn(turnCtx, sessionID, row, info, model, onDone)
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

// Abort cancels the active turn of the session. It reports whether a turn
// was active.
func (e *Engine) Abort(sessionID string) bool {
	e.mu.Lock()
	cancel, active := e.busy[sessionID]
	e.mu.Unlock()
	if !active {
		return false
	}
	cancel()
	return true
}

// Close releases the session's per-work resources (the bash shell).
func (e *Engine) Close(sessionID string) {
	e.mu.Lock()
	s, ok := e.shells[sessionID]
	if ok {
		delete(e.shells, sessionID)
	}
	e.mu.Unlock()
	if ok {
		_ = s.Close()
	}
}

// Shutdown drains the engine for process exit: it cancels every active
// turn, waits for the turn goroutines to release (at most 5 s, or until
// ctx is done), then releases all session shells.
func (e *Engine) Shutdown(ctx context.Context) {
	e.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(e.busy))
	for _, cancel := range e.busy {
		cancels = append(cancels, cancel)
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
	e.mu.Lock()
	shells := e.shells
	e.shells = map[string]*tool.Shell{}
	e.mu.Unlock()
	for _, s := range shells {
		_ = s.Close()
	}
}

// idleAndRelease runs the turn-exit cleanup used when the turn goroutine
// never started.
func (e *Engine) idleAndRelease(sessionID string) {
	e.publish(protocol.EventTypeSessionStatus, protocol.SessionStatusProps{
		SessionID: sessionID,
		Status:    protocol.SessionStatus{Type: protocol.StatusIdle},
	})
	e.mu.Lock()
	delete(e.busy, sessionID)
	e.mu.Unlock()
}

func (e *Engine) publish(t string, props any) {
	ev, err := protocol.MakeEvent(t, props)
	if err != nil {
		// A marshal failure here would kill the whole event stream, so it
		// must be diagnosable, not silently dropped.
		e.lg.Errorf("session: marshal %s: %v", t, err)
		return
	}
	e.bus.Publish(ev)
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
	if d, ok := e.drivers[providerID]; ok {
		return d
	}
	return e.prov.DriverFor(m)
}

func (e *Engine) limitsFor(cfg *protocol.Config) tool.Limits {
	if cfg != nil && cfg.ToolOutput != nil {
		return tool.Limits{MaxLines: cfg.ToolOutput.MaxLines, MaxBytes: cfg.ToolOutput.MaxBytes}
	}
	return tool.Limits{}
}

// shellFor returns the session's lazily-spawned bash shell.
func (e *Engine) shellFor(sessionID, dir string) *tool.Shell {
	e.mu.Lock()
	defer e.mu.Unlock()
	s, ok := e.shells[sessionID]
	if !ok {
		s = tool.NewShell(dir, tool.Limits{})
		e.shells[sessionID] = s
	}
	return s
}

// runTurn drives the model/tool rounds until the model stops calling tools.
// The session leaves "busy" on exit and onDone fires exactly once.
func (e *Engine) runTurn(ctx context.Context, sessionID string, row storage.SessionRow, info provider.Info, model provider.Model, onDone func(error)) {
	var turnErr error
	defer func() {
		e.publish(protocol.EventTypeSessionStatus, protocol.SessionStatusProps{
			SessionID: sessionID,
			Status:    protocol.SessionStatus{Type: protocol.StatusIdle},
		})
		e.mu.Lock()
		delete(e.busy, sessionID)
		e.mu.Unlock()
		if onDone != nil {
			onDone(turnErr)
		}
	}()
	e.publish(protocol.EventTypeSessionStatus, protocol.SessionStatusProps{
		SessionID: sessionID,
		Status:    protocol.SessionStatus{Type: protocol.StatusBusy},
	})

	cfg, cfgErr := e.loadCfg(row.ProjectDir)
	cfgRules := []protocol.Rule{}
	if cfgErr == nil && cfg != nil {
		// Invalid permission entries degrade to no config rules (config
		// load is non-fatal per turn).
		if rules, perr := protocol.ParsePerms(cfg.Permission); perr == nil {
			cfgRules = rules
		}
	}
	// Always publish this turn's rules (empty when the load failed): a
	// broken config degrades to no config rules instead of silently
	// inheriting the previous turn's ruleset.
	e.perm.SetConfigRules(cfgRules)
	agent := row.Agent
	if agent == "" {
		agent = "build"
	}
	// Per-turn doom history (resets each Send): every model-issued tool call
	// appends one CallKey, identical runs detected on the last-3 window.
	var doomHist []permission.CallKey
	// Per-turn tool call budget (resets each Send; distinct from the
	// model round-trip cap above).
	toolCalls := 0

	for round := 0; round < maxToolRounds; round++ {
		req, err := e.buildRequest(sessionID, agent, row, info, model, cfg)
		if err != nil {
			turnErr = err
			return
		}
		more, err := e.runRound(ctx, sessionID, agent, row, cfg, cfgRules, info, model, req, &doomHist, &toolCalls)
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
func (e *Engine) buildRequest(sessionID, agent string, row storage.SessionRow, info provider.Info, model provider.Model, cfg *protocol.Config) (llm.Request, error) {
	messages, err := e.messagesFor(sessionID, agent, row, info, model, cfg)
	if err != nil {
		return llm.Request{}, err
	}
	tools, err := e.toolSchemaList(sessionID)
	if err != nil {
		return llm.Request{}, err
	}
	return llm.Request{
		Model:    model.ID,
		APIKey:   e.apiKey(info.ID, cfg),
		BaseURL:  info.BaseURL,
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
//   - the request ends with the newest user message, re-appended when the
//     history no longer ends with it (tool-call rounds).
func (e *Engine) messagesFor(sessionID, agent string, row storage.SessionRow, info provider.Info, model provider.Model, cfg *protocol.Config) ([]llm.Message, error) {
	sys, err := BuildSystemPrompt(row.ProjectDir, model, model.ID, info.ID)
	if err != nil {
		return nil, err
	}
	if cfg != nil {
		for _, p := range cfg.Instructions {
			abs := p
			if !filepath.IsAbs(abs) {
				abs = filepath.Join(row.ProjectDir, abs)
			}
			if b, err := os.ReadFile(abs); err == nil {
				sys = append(sys, string(b))
			}
		}
	}
	out := make([]llm.Message, 0, len(sys)+8)
	for _, s := range sys {
		out = append(out, llm.Message{Role: llm.RoleSystem, Content: s})
	}

	rows, err := e.db.ListMessages(sessionID)
	if err != nil {
		return nil, err
	}
	hist := make([]protocol.MessageWithParts, 0, len(rows))
	lastUserIdx := -1
	for i, r := range rows {
		prs, err := e.db.ListParts(r.ID)
		if err != nil {
			return nil, err
		}
		parts := make([]protocol.Part, 0, len(prs))
		for _, pr := range prs {
			p, err := storage.PartToProtocol(pr)
			if err != nil {
				return nil, err
			}
			if isSyntheticPart(p) {
				// Engine-generated notes (error/overflow) are excluded from
				// history replay: the model must never see them.
				continue
			}
			parts = append(parts, p)
		}
		if r.Role == "user" {
			lastUserIdx = i
		}
		hist = append(hist, protocol.MessageWithParts{
			Info: protocol.Message{
				ID: r.ID, SessionID: r.SessionID, Role: r.Role, Agent: r.Agent,
				Time: protocol.MessageTime{Created: r.TimeCreated},
			},
			Parts: parts,
		})
	}

	reminders := PlanReminders(hist, agent)
	var lastUserContent string
	var lastMappedRole llm.Role
	for i, mw := range hist {
		switch mw.Info.Role {
		case "user":
			content := joinTextParts(mw.Parts)
			if i == lastUserIdx {
				content = appendReminders(content, reminders)
				lastUserContent = content
			}
			out = append(out, llm.Message{Role: llm.RoleUser, Content: content})
			lastMappedRole = llm.RoleUser
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
			lastMappedRole = llm.RoleAssistant
			if len(toolMsgs) > 0 {
				out = append(out, toolMsgs...)
				lastMappedRole = llm.RoleTool
			}
		}
	}
	if lastUserIdx >= 0 && lastMappedRole != llm.RoleUser {
		out = append(out, llm.Message{Role: llm.RoleUser, Content: lastUserContent})
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
func (e *Engine) runRound(ctx context.Context, sessionID, agent string, row storage.SessionRow, cfg *protocol.Config, cfgRules []protocol.Rule, info provider.Info, model provider.Model, req llm.Request, doomHist *[]permission.CallKey, toolCalls *int) (bool, error) {
	// Per-round context: the real drivers' stream goroutines block their
	// send on this ctx, so cancelling it on every round exit unblocks any
	// abandoned stream (e.g. the tool-step budget drop) instead of leaking
	// its goroutine and connection until process shutdown.
	roundCtx, roundCancel := context.WithCancel(ctx)
	defer roundCancel()
	drv := e.driverFor(info.ID, model)
	// The assistant row exists before the first stream attempt so a failed
	// round still finalizes a (possibly empty) assistant message.
	now := e.clock()
	asstID := protocol.NewID("msg")
	if err := e.db.CreateMessage(storage.MessageRow{
		ID: asstID, SessionID: sessionID, Role: "assistant", Agent: agent, TimeCreated: now,
	}); err != nil {
		return false, err
	}
	asstMsg := protocol.Message{
		ID: asstID, SessionID: sessionID, Role: "assistant", Agent: agent,
		Time:  protocol.MessageTime{Created: now},
		Model: &protocol.MessageModel{ProviderID: info.ID, ModelID: model.ID},
	}
	e.publish(protocol.EventTypeMessageUpdated, protocol.MessageUpdatedProps{SessionID: sessionID, Info: asstMsg})

	// Pre-stream retry: transient failures (429/5xx/net) retry with
	// backoff while nothing of the round is persisted; non-transient
	// failures fail the round immediately (overflow 400s take the
	// graceful path below).
	var stream llm.PartStream
	// One reusable timer for the pre-stream retry backoffs (no fresh
	// allocation per attempt); the zero timer has already fired, so
	// drain that tick before the first Reset re-arms it.
	retry := time.NewTimer(0)
	<-retry.C
	defer retry.Stop()
	for attempt := 1; ; attempt++ {
		var sErr error
		stream, sErr = drv.Stream(roundCtx, req)
		if sErr == nil {
			break
		}
		if ctx.Err() != nil {
			e.finishRound(asstID, sessionID, agent, now, model, &asstMsg, nil, "")
			return false, ctx.Err()
		}
		if !llm.IsTransient(sErr) {
			if isOverflowError(sErr) {
				e.saveSynthetic(sessionID, asstID, overflowNote(model, 0, sErr))
				e.finishRound(asstID, sessionID, agent, now, model, &asstMsg, nil, "")
				return false, nil
			}
			e.finishRound(asstID, sessionID, agent, now, model, &asstMsg, nil, "")
			return false, sErr
		}
		if attempt >= maxRetryAttempts {
			e.finishRound(asstID, sessionID, agent, now, model, &asstMsg, nil, "")
			// Retry-exhaustion framing belongs in the boundary error: the
			// send boundary logs the turn error exactly once.
			return false, fmt.Errorf("transient retries exhausted after %d attempts (session=%s): %w", maxRetryAttempts, sessionID, sErr)
		}
		delay := e.backoff(attempt)
		e.publish(protocol.EventTypeSessionStatus, protocol.SessionStatusProps{
			SessionID: sessionID,
			Status: protocol.SessionStatus{
				Type: protocol.StatusRetry, Attempt: attempt,
				Message: sErr.Error(), Next: delay.Milliseconds(),
			},
		})
		retry.Reset(delay)
		select {
		case <-retry.C:
		case <-ctx.Done():
			e.finishRound(asstID, sessionID, agent, now, model, &asstMsg, nil, "")
			return false, ctx.Err()
		}
	}

	type textState struct {
		id    string
		start int64
		buf   strings.Builder
	}
	var textSt, reasonSt textState

	saveDelta := func(st *textState, kind, delta string) {
		st.buf.WriteString(delta)
		p := protocol.Part{
			ID: st.id, SessionID: sessionID, MessageID: asstID,
			Type: kind, Text: st.buf.String(),
			Time: protocol.PartTime{Start: st.start},
		}
		// Best-effort persistence: the delta still goes to the TUI.
		if err := e.db.UpsertPart(storage.ProtocolToPart(p)); err != nil {
			e.lg.Errorf("session: persist part %s (session=%s): %v", p.ID, sessionID, err)
		}
		e.publish(protocol.EventTypeMessagePartDelta, protocol.MessagePartDeltaProps{
			SessionID: sessionID, MessageID: asstID, PartID: st.id, Field: kind, Delta: delta,
		})
	}
	startPart := func(st *textState, kind, delta string) {
		st.id = protocol.NewID("prt")
		st.start = e.clock()
		st.buf.WriteString(delta)
		p := protocol.Part{
			ID: st.id, SessionID: sessionID, MessageID: asstID,
			Type: kind, Text: st.buf.String(),
			Time: protocol.PartTime{Start: st.start},
		}
		// Best-effort persistence: the part-created event still goes out.
		if err := e.db.UpsertPart(storage.ProtocolToPart(p)); err != nil {
			e.lg.Errorf("session: persist part %s (session=%s): %v", p.ID, sessionID, err)
		}
		e.publish(protocol.EventTypeMessagePartUpdated, protocol.MessagePartUpdatedProps{SessionID: sessionID, Part: p, Time: e.clock()})
		e.publish(protocol.EventTypeMessagePartDelta, protocol.MessagePartDeltaProps{
			SessionID: sessionID, MessageID: asstID, PartID: st.id, Field: kind, Delta: delta,
		})
	}
	finalizePart := func(st *textState, kind string) {
		if st.id == "" {
			return
		}
		p := protocol.Part{
			ID: st.id, SessionID: sessionID, MessageID: asstID,
			Type: kind, Text: st.buf.String(),
			Time: protocol.PartTime{Start: st.start, End: e.clock()},
		}
		if err := e.db.UpsertPart(storage.ProtocolToPart(p)); err != nil {
			e.lg.Errorf("session: persist part %s (session=%s): %v", p.ID, sessionID, err)
		}
		e.publish(protocol.EventTypeMessagePartUpdated, protocol.MessagePartUpdatedProps{SessionID: sessionID, Part: p, Time: e.clock()})
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
			e.finishRound(asstID, sessionID, agent, now, model, &asstMsg, usage, finish)
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
				e.finishRound(asstID, sessionID, agent, now, model, &asstMsg, usage, finish)
				return false, ctx.Err()
			}
			if isOverflowError(p.Err) {
				e.saveSynthetic(sessionID, asstID, overflowNote(model, 0, p.Err))
				e.finishRound(asstID, sessionID, agent, now, model, &asstMsg, usage, finish)
				return false, nil
			}
			// Mid-stream failure after content: keep the partial text, note
			// the error on a synthetic part (excluded from history replay —
			// the model never sees it) and fail the turn (no retry).
			e.saveSynthetic(sessionID, asstID, p.Err.Error())
			e.finishRound(asstID, sessionID, agent, now, model, &asstMsg, usage, finish)
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
			if *toolCalls >= maxToolSteps {
				// Step budget exhausted: the remaining calls of this stream
				// are dropped (not persisted, not executed); the turn ends
				// idle and onDone(nil).
				e.lg.Infof("session: max tool steps reached (session=%s, steps=%d)", sessionID, maxToolSteps)
				e.finishRound(asstID, sessionID, agent, now, model, &asstMsg, usage, finish)
				return false, nil
			}
			*toolCalls++
			e.executeTool(roundCtx, sessionID, agent, row, cfg, cfgRules, asstID, doomHist, p)
		}
	}
	finalizePart(&textSt, "text")
	finalizePart(&reasonSt, "reasoning")
	if finish == "" {
		finish = "stop"
	}
	// Overflow: the round's input already exceeds the model context; the
	// turn ends with a synthetic note (v1 has no compaction).
	if usage != nil && model.Context > 0 && usage.Input > model.Context {
		e.saveSynthetic(sessionID, asstID, overflowNote(model, usage.Input, nil))
		e.finishRound(asstID, sessionID, agent, now, model, &asstMsg, usage, finish)
		return false, nil
	}
	e.finishRound(asstID, sessionID, agent, now, model, &asstMsg, usage, finish)

	// A round continues when the model finished with tool_calls or emitted
	// any tool part (scripted drivers set Finish inconsistently).
	return finish == "tool_calls" || sawToolPart, nil
}

// finishRound completes the assistant message row and re-publishes
// message.updated with the final state, deriving cost/tokens from the round's
// usage (nil-safe). It is called on every round-exit path (success, failure,
// abort, retry exhaustion, overflow, max-steps).
func (e *Engine) finishRound(asstID, sessionID, agent string, now int64, model provider.Model, asstMsg *protocol.Message, usage *llm.Usage, finish string) {
	var tok protocol.Tokens
	cost := 0.0
	if usage != nil {
		tok = protocol.Tokens{
			Input:     int64(usage.Input),
			Output:    int64(usage.Output),
			Reasoning: int64(usage.Reasoning),
			Cache:     protocol.CacheTokens{Read: int64(usage.CacheRead), Write: int64(usage.CacheWrite)},
		}
		cost = (float64(usage.Input)*model.CostIn + float64(usage.Output)*model.CostOut +
			float64(usage.CacheRead)*model.CostCacheRead + float64(usage.CacheWrite)*model.CostCacheWrite) / 1e6
	}
	end := e.clock()
	if err := e.db.UpdateMessage(storage.MessageRow{
		ID: asstID, SessionID: sessionID, Role: "assistant", Agent: agent,
		Cost: cost, Tokens: tok, TimeCreated: now, TimeCompleted: &end,
	}); err != nil {
		// Best-effort: a failed write must not strand the TUI in a
		// cost-less/incomplete final state, so the final message.updated
		// still goes out.
		e.lg.Errorf("session: update message %s (session=%s): %v", asstID, sessionID, err)
	}
	asstMsg.Cost = cost
	asstMsg.Tokens = &tok
	if finish != "" {
		asstMsg.Finish = finish
	}
	asstMsg.Time.Completed = end
	e.publish(protocol.EventTypeMessageUpdated, protocol.MessageUpdatedProps{SessionID: sessionID, Info: *asstMsg})
}

// saveSynthetic persists an engine-generated text part (mid-stream error
// note, overflow note) flagged Synthetic: it shows in the TUI but
// messagesFor excludes it from history replay, so the model never sees it.
func (e *Engine) saveSynthetic(sessionID, asstID, text string) {
	syn := true
	start := e.clock()
	p := protocol.Part{
		ID: protocol.NewID("prt"), SessionID: sessionID, MessageID: asstID,
		Type: "text", Text: text, Synthetic: &syn,
		Time: protocol.PartTime{Start: start, End: e.clock()},
	}
	if err := e.db.UpsertPart(storage.ProtocolToPart(p)); err != nil {
		e.lg.Errorf("session: persist part %s (session=%s): %v", p.ID, sessionID, err)
	}
	e.publish(protocol.EventTypeMessagePartUpdated, protocol.MessagePartUpdatedProps{SessionID: sessionID, Part: p, Time: e.clock()})
}

// isSyntheticPart reports whether a part is engine-generated and excluded
// from history replay.
func isSyntheticPart(p protocol.Part) bool {
	return p.Synthetic != nil && *p.Synthetic
}

// overflowRe matches provider-side context-overflow API errors (400 "prompt
// too long" and friends). NOTE: "context" also matches context.Canceled —
// callers MUST check ctx.Err() first.
var overflowRe = regexp.MustCompile(`(?i)(context|tokens?|too long|exceeds)`)

// isOverflowError reports whether an API (non-stream) error is a
// context-overflow rejection.
func isOverflowError(err error) bool {
	return err != nil && overflowRe.MatchString(err.Error())
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
func (e *Engine) executeTool(ctx context.Context, sessionID, agent string, row storage.SessionRow, cfg *protocol.Config, cfgRules []protocol.Rule, asstID string, doomHist *[]permission.CallKey, p llm.Part) {
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
		e.saveToolPart(sessionID, asstID, callID, name, map[string]any{}, "error", "", "", msg, nil, e.clock(), stage)
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
	t, ok := e.tools[name]
	if !ok {
		start := e.clock()
		fail(start, "unknown tool "+name)
		return
	}
	rules, err := e.rulesetForRow(row)
	if err != nil {
		fail(e.clock(), err.Error())
		return
	}
	if hidden := permission.Hidden(rules, []string{name})[name]; hidden {
		start := e.clock()
		fail(start, "tool not available")
		return
	}
	resources, always, err := t.Patterns(raw)
	if err != nil {
		fail(e.clock(), err.Error())
		return
	}
	external, err := t.External(raw)
	if err != nil {
		fail(e.clock(), err.Error())
		return
	}

	// (1) doom check: the third identical call of the turn asks before it
	// runs (sliding window; a "once" reply does not extend the exemption).
	key := permission.CallKey{Tool: name, Hash: callKeyHash(raw)}
	if permission.DoomLoopDue(*doomHist, key) {
		d := e.perm.EvaluateRules(agent, e.dataDir, cfgRules, "doom_loop", []string{name})
		doomReq := permission.Request{
			RequestID: protocol.NewID("perm"), SessionID: sessionID, Agent: agent,
			Permission: "doom_loop", Tool: t.ID(),
			Resources: []string{name},
			CallID:    callID, MessageID: asstID,
			DecisionPre: d, CreatedAt: e.clock(),
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
			e.saveToolPart(sessionID, asstID, callID, name, input, "error", "", "", msg, map[string]any{"reason": "doom_loop"}, now, now)
			*doomHist = append(*doomHist, key)
			return
		}
	}
	*doomHist = append(*doomHist, key)

	start := e.clock()
	e.saveToolPart(sessionID, asstID, callID, name, input, "running", "", "", "", nil, start, 0)

	// (3) external-directory gate.
	for _, ext := range external {
		abs := ext
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(row.ProjectDir, abs)
		}
		abs = filepath.Clean(abs)
		if inside, _ := withinDir(row.ProjectDir, abs); inside {
			continue
		}
		pattern := filepath.Dir(abs) + "/*"
		d := e.perm.EvaluateRules(agent, e.dataDir, cfgRules, "external_directory", []string{pattern})
		extReq := permission.Request{
			RequestID: protocol.NewID("perm"), SessionID: sessionID, Agent: agent,
			Permission: "external_directory", Tool: t.ID(),
			Resources: []string{pattern},
			CallID:    callID, MessageID: asstID,
			DecisionPre: d, CreatedAt: e.clock(),
		}
		decision, aerr := e.perm.Ask(ctx, extReq)
		if aerr != nil || decision != permission.Allow {
			gateFail()
			return
		}
	}

	// (4) core permission.
	d := e.perm.EvaluateRules(agent, e.dataDir, cfgRules, t.Permission(), resources)
	preq := permission.Request{
		RequestID: protocol.NewID("perm"), SessionID: sessionID, Agent: agent,
		Permission: t.Permission(), Tool: t.ID(),
		Resources: resources, Always: always,
		CallID: callID, MessageID: asstID,
		DecisionPre: d, CreatedAt: e.clock(),
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
		Dir:       row.ProjectDir,
		Shell:     e.shellFor(sessionID, row.ProjectDir),
		Limits:    e.limitsFor(cfg),
		Storage:   e.db,
		SessionID: sessionID,
	}
	out, runErr := t.Run(ctx, raw, env)
	if runErr != nil {
		msg := runErr.Error()
		if ctx.Err() != nil {
			// Abort while the tool ran: label it plainly; the process was
			// already force-killed via ctx.
			msg = "aborted"
		}
		e.saveToolPart(sessionID, asstID, callID, name, input, "error", out.Title, out.Text, msg, out.Meta, start, e.clock())
		return
	}
	e.saveToolPart(sessionID, asstID, callID, name, input, "completed", out.Title, out.Text, "", out.Meta, start, e.clock())
}

func (e *Engine) saveToolPart(sessionID, asstID, callID, name string, input map[string]any, status, title, output, toolErr string, meta map[string]any, start, end int64) {
	st := &protocol.ToolState{
		Status:   status,
		Input:    input,
		Title:    title,
		Output:   output,
		Error:    toolErr,
		Metadata: meta,
		Time:     protocol.PartTime{Start: start, End: end},
	}
	p := protocol.Part{
		ID: callID, SessionID: sessionID, MessageID: asstID,
		Type: "tool", Tool: name, CallID: callID, State: st,
	}
	if err := e.db.UpsertPart(storage.ProtocolToPart(p)); err != nil {
		e.lg.Errorf("session: persist part %s (session=%s): %v", p.ID, sessionID, err)
	}
	e.publish(protocol.EventTypeMessagePartUpdated, protocol.MessagePartUpdatedProps{SessionID: sessionID, Part: p, Time: e.clock()})
}

// maybeScheduleTitle fires the one-shot title generation for the session's
// first user message when the title is still the default.
func (e *Engine) maybeScheduleTitle(sessionID string, row storage.SessionRow, userText string) {
	if row.Title != "" && row.Title != "New session" {
		return
	}
	msgs, err := e.db.ListMessages(sessionID)
	if err != nil {
		return
	}
	for _, m := range msgs {
		if m.Role == "assistant" {
			return
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	go e.generateTitle(ctx, cancel, sessionID, row, userText)
}

// generateTitle best-effort: errors are dropped (title stays the default).
func (e *Engine) generateTitle(ctx context.Context, cancel context.CancelFunc, sessionID string, row storage.SessionRow, userText string) {
	defer cancel()
	info, model, err := e.prov.Resolve(row.Model)
	if err != nil {
		return
	}
	cfg, _ := e.loadCfg(row.ProjectDir)
	req := llm.Request{
		Model:   model.ID,
		APIKey:  e.apiKey(info.ID, cfg),
		BaseURL: info.BaseURL,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: TitlePrompt()},
			{Role: llm.RoleUser, Content: userText},
		},
	}
	stream, err := e.driverFor(info.ID, model).Stream(ctx, req)
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
	if err := e.db.UpdateSession(sessionID, storage.SessionRow{Title: title, TimeUpdated: e.clock()}); err != nil {
		return
	}
	updated, err := e.db.GetSession(sessionID)
	if err != nil {
		return
	}
	msgs, err := e.db.ListMessages(sessionID)
	if err != nil {
		return
	}
	e.publish(protocol.EventTypeSessionUpdated, protocol.SessionUpdatedProps{
		SessionID: sessionID,
		Info:      storage.SessionFromRow(updated, msgs),
	})
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
