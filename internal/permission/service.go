package permission

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kido5217/yolo/internal/bus"
	"github.com/kido5217/yolo/internal/glob"
	"github.com/kido5217/yolo/internal/log"
	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/storage"
)

// ErrNoPending is returned by Reply when the request is not (or no longer)
// parked.
var ErrNoPending = errors.New("permission: no pending request")

// Request is one permission ask. DecisionPre carries the engine's
// pre-evaluation (Allow|Deny | AskAction); zero means "decide now". CallID and
// MessageID identify the originating tool call (wire tool ref); they may be
// empty for non-tool asks.
type Request struct {
	RequestID, SessionID, Agent string
	Permission                  string // action, e.g. "read"
	Tool                        string // tool name for the TUI
	Resources                   []string
	Always                      []string // suggested always patterns
	CallID, MessageID           string
	Meta                        map[string]any
	DecisionPre                 Decision
	CreatedAt                   int64
}

type pendingEntry struct {
	req Request
	ch  chan Decision
}

// Service enforces+blocks: the engine evaluates (EvaluateRules) and passes
// the verdict via DecisionPre; the service persists, parks, and resolves.
type Service struct {
	db  *storage.DB
	bus *bus.Bus
	lg  *log.Logger // nil = no-op

	mu       sync.Mutex
	pending  map[string]*pendingEntry
	cfgRules []protocol.Rule
	dataDir  string
}

func New(db *storage.DB, b *bus.Bus) *Service {
	return &Service{db: db, bus: b, pending: map[string]*pendingEntry{}}
}

// SetLogger sets the diagnostic logger (nil is a no-op). Production wiring
// calls this before the service is used; like SetDataDir it is a
// set-once-before-use seam, not a concurrent config surface.
func (s *Service) SetLogger(lg *log.Logger) {
	s.mu.Lock()
	s.lg = lg
	s.mu.Unlock()
}

// SetConfigRules stores the config permission rules used by DecisionFor
// between the builtins and the session's always rules.
func (s *Service) SetConfigRules(rules []protocol.Rule) {
	s.mu.Lock()
	s.cfgRules = rules
	s.mu.Unlock()
}

// SetDataDir sets the data directory used for the plan matrix in
// DecisionFor. The engine calls this at session start.
func (s *Service) SetDataDir(dir string) {
	s.mu.Lock()
	s.dataDir = dir
	s.mu.Unlock()
}

// EvaluateRules evaluates builtins + config rules for an action.
// Session always rules are not included (they need a session; see DecisionFor).
// Unknown (custom) agents fall back to the build matrix via BuiltinsFor.
func (s *Service) EvaluateRules(agent, dataDir string, cfgRules []protocol.Rule, action string, resources []string) Decision {
	rules := BuiltinsFor(agent, dataDir)
	all := make([]protocol.Rule, 0, len(rules)+len(cfgRules))
	all = append(all, rules...)
	all = append(all, cfgRules...)
	return Evaluate(all, action, resources)
}

// DecisionFor re-evaluates a request against builtins + config rules + the
// session's persisted always rules.
func (s *Service) DecisionFor(req Request) Decision {
	return s.decisionFor(req)
}

func (s *Service) decisionFor(req Request) Decision {
	dataDir := s.dataDir
	if v, ok := req.Meta["data_dir"].(string); ok {
		dataDir = v
	}
	// Unknown (custom) agents fall back to the build matrix via
	// BuiltinsFor (mirrors the engine's ruleset path).
	rules := BuiltinsFor(req.Agent, dataDir)
	s.mu.Lock()
	cfg := s.cfgRules
	s.mu.Unlock()
	always, err := s.db.AlwaysRules(req.SessionID)
	if err != nil {
		// Fail-safe: degrade to no always rules (re-asks at worst).
		s.lg.Errorf("permission: always rules (session=%s): %v", req.SessionID, err)
		always = []protocol.Rule{}
	}
	all := make([]protocol.Rule, 0, len(rules)+len(cfg)+len(always))
	all = append(all, rules...)
	all = append(all, cfg...)
	all = append(all, always...)
	// The catch-all "*" permission rule in the matrices stands in for
	// upstream's no-rule default of ALLOW only for known core actions;
	// unknown actions fall through to ask.
	wildcardOK := corePermissions[req.Permission]
	anyAsk := false
	for _, res := range req.Resources {
		last := findLastWithWildcard(all, req.Permission, res, wildcardOK)
		if last == nil {
			anyAsk = true
			continue
		}
		switch last.Action {
		case RuleDeny:
			return Deny
		case RuleAsk:
			anyAsk = true
		}
	}
	if anyAsk {
		return AskAction
	}
	return Allow
}

// Ask resolves a permission request. A decisive verdict persists the row and
// returns immediately; AskAction parks the request until Reply (or ctx
// cancel, which stores response='aborted' and denies).
func (s *Service) Ask(ctx context.Context, req Request) (Decision, error) {
	if req.RequestID == "" {
		return "", errors.New("permission: request_id required")
	}
	if req.CreatedAt == 0 {
		req.CreatedAt = now()
	}
	decision := req.DecisionPre
	if decision == "" || decision == AskAction {
		decision = s.decisionFor(req)
	}
	if decision != AskAction {
		if err := s.persist(req, storedResponse(decision)); err != nil {
			return "", err
		}
		return decision, nil
	}

	entry := &pendingEntry{req: req, ch: make(chan Decision, 1)}
	s.mu.Lock()
	if old, ok := s.pending[req.RequestID]; ok {
		s.deliver(old, Deny)
	}
	s.pending[req.RequestID] = entry
	s.mu.Unlock()

	if err := s.persist(req, ""); err != nil {
		s.mu.Lock()
		if s.pending[req.RequestID] == entry {
			delete(s.pending, req.RequestID)
		}
		s.mu.Unlock()
		s.deliver(entry, Deny)
		return "", err
	}
	s.publishAsked(req)

	select {
	case d := <-entry.ch:
		return d, nil
	case <-ctx.Done():
		s.resolve(req.RequestID, Deny, "aborted", "reject", false)
		return Deny, nil
	}
}

// Reply answers a parked request: "once" | "always" | "reject".
func (s *Service) Reply(requestID, response string) error {
	switch response {
	case "once", "always", "reject":
	default:
		return errors.New("permission: invalid response " + response)
	}
	s.mu.Lock()
	e, ok := s.pending[requestID]
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("%w: %s", ErrNoPending, requestID)
	}
	req := e.req
	switch response {
	case "once":
		s.resolve(requestID, Allow, "once", "once", false)
	case "reject":
		s.resolve(requestID, Deny, "rejected", "reject", false)
		s.cascade(req.SessionID, requestID, Deny, "rejected", "reject")
	case "always":
		s.resolve(requestID, Allow, "always", "always", false)
		s.autoAllow(req)
	}
	return nil
}

// Pending lists this session's parked requests.
func (s *Service) Pending(sessionID string) ([]Request, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []Request{}
	for _, e := range s.pending {
		if e.req.SessionID == sessionID {
			out = append(out, e.req)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RequestID < out[j].RequestID })
	return out, nil
}

// autoAllow resolves same-session pendings whose permission matches and
// whose resources are all covered by the just-persisted always patterns.
func (s *Service) autoAllow(req Request) {
	others := s.sessionPending(req.SessionID, req.RequestID)
	for _, e := range others {
		r := e.req
		if r.Permission != req.Permission {
			continue
		}
		if !covered(r.Resources, req.Always) {
			continue
		}
		// stored as "once": only explicit "always" replies mint always rules
		s.resolve(r.RequestID, Allow, "once", "always", true)
	}
}

// cascade applies the verdict to every other parked request in the session
// (used by "reject").
func (s *Service) cascade(sessionID, skipID string, d Decision, stored, wire string) {
	for _, e := range s.sessionPending(sessionID, skipID) {
		s.resolve(e.req.RequestID, d, stored, wire, true)
	}
}

func (s *Service) sessionPending(sessionID, skipID string) []*pendingEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*pendingEntry
	for id, e := range s.pending {
		if e.req.SessionID != sessionID || id == skipID {
			continue
		}
		out = append(out, e)
	}
	return out
}

// resolve records the response, emits permission.replied, and delivers the
// decision to the waiting asker. Reply persistence is best-effort here: the
// decision is already in memory, so deliver and publish happen regardless of
// the DB result — a failed write must not strand the blocked Ask.
func (s *Service) resolve(requestID string, d Decision, stored, wire string, auto bool) {
	s.mu.Lock()
	e, ok := s.pending[requestID]
	delete(s.pending, requestID)
	var sessionID string
	if ok {
		sessionID = e.req.SessionID
	}
	s.mu.Unlock()

	if err := s.db.ReplyPermission(requestID, stored); err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			s.lg.Errorf("permission: persist reply %s: %v", requestID, err)
		}
	}
	props := protocol.PermissionRepliedProps{RequestID: requestID, Reply: wire, Auto: auto}
	if sessionID != "" {
		props.SessionID = sessionID
	}
	if ev, err := protocol.MakeEvent(protocol.EventTypePermissionReplied, props); err != nil {
		s.lg.Errorf("permission: marshal %s: %v", protocol.EventTypePermissionReplied, err)
	} else {
		s.bus.Publish(ev)
	}
	if ok {
		s.deliver(e, d)
	}
}

func (s *Service) deliver(e *pendingEntry, d Decision) {
	select {
	case e.ch <- d:
	default:
	}
}

// persist stores (or re-stores) the request row. response "" -> NULL (pending).
func (s *Service) persist(req Request, response string) error {
	var alwaysJSON string
	if len(req.Always) > 0 {
		if b, err := json.Marshal(req.Always); err == nil {
			alwaysJSON = string(b)
		}
	}
	return s.db.SavePermission(storage.PermissionRow{
		RequestID:   req.RequestID,
		SessionID:   req.SessionID,
		Action:      req.Permission,
		Resource:    strings.Join(req.Resources, ","),
		Response:    response,
		AlwaysJSON:  alwaysJSON,
		TimeCreated: req.CreatedAt,
	})
}

func (s *Service) publishAsked(req Request) {
	meta := map[string]any{}
	for k, v := range req.Meta {
		meta[k] = v
	}
	if req.Tool != "" {
		meta["tool"] = req.Tool
	}
	if req.Agent != "" {
		meta["agent"] = req.Agent
	}
	props := protocol.PermissionAskedProps{
		ID:         req.RequestID,
		SessionID:  req.SessionID,
		Permission: req.Permission,
		Patterns:   req.Resources,
		Always:     req.Always,
		Metadata:   meta,
	}
	if req.Tool != "" {
		if req.CallID != "" {
			props.Tool = &protocol.PermissionToolRef{MessageID: req.MessageID, CallID: req.CallID}
		} else {
			props.Tool = &protocol.PermissionToolRef{CallID: req.RequestID}
		}
	}
	if ev, err := protocol.MakeEvent(protocol.EventTypePermissionAsked, props); err != nil {
		s.lg.Errorf("permission: marshal %s: %v", protocol.EventTypePermissionAsked, err)
	} else {
		s.bus.Publish(ev)
	}
}

func covered(resources, patterns []string) bool {
	for _, res := range resources {
		ok := false
		for _, p := range patterns {
			if glob.Match(p, res) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

func storedResponse(d Decision) string {
	if d == Allow {
		return "once"
	}
	return "rejected"
}

func now() int64 {
	return time.Now().UnixMilli()
}
