package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/kido5217/yolo/internal/protocol"
)

type format int

const (
	formatDefault format = iota
	formatJSON
)

// renderer is the run output contract (spec §6): a pure event-loop
// state machine over protocol.Event, writing to injected sinks (no ANSI
// bytes ever; the NDJSON timestamp comes from now so tests pin it).
type renderer struct {
	f         format
	thinking  bool
	sessionID string
	header    string // pre-built "> agent · provider/model" ("" prints nothing)
	now       func() int64
	out, errw io.Writer

	partText      map[string]string // partID -> accumulated delta text
	seen          map[string]bool   // assistant messageIDs seen (step_start)
	headerDone    bool
	sawAssistant  bool
	lastAssistant *protocol.Message
	turnErr       bool
}

func newRenderer(f format, thinking bool, sessionID, header string, now func() int64, out, errw io.Writer) *renderer {
	return &renderer{
		f: f, thinking: thinking, sessionID: sessionID, header: header, now: now,
		out: out, errw: errw,
		partText: map[string]string{}, seen: map[string]bool{},
	}
}

func (r *renderer) apply(ev protocol.Event) {
	switch ev.Type {
	case protocol.EventTypeMessageUpdated:
		var p protocol.MessageUpdatedProps
		if json.Unmarshal(ev.Properties, &p) != nil || p.SessionID != r.sessionID ||
			p.Info.Role != "assistant" {
			return
		}
		if !r.seen[p.Info.ID] {
			r.seen[p.Info.ID] = true
			if r.f == formatJSON {
				r.line("step_start", &stepStartPart{Type: "step-start", MessageID: p.Info.ID})
			}
		}
		if r.f == formatDefault && !r.headerDone && r.header != "" {
			fmt.Fprintln(r.errw, r.header)
			r.headerDone = true
		}
		r.lastAssistant = &p.Info
		r.sawAssistant = true
		if p.Info.Error != nil {
			r.turnErr = true
			r.emitError(p.Info.Error)
		}
	case protocol.EventTypeMessagePartUpdated:
		var p protocol.MessagePartUpdatedProps
		if json.Unmarshal(ev.Properties, &p) != nil || p.SessionID != r.sessionID {
			return
		}
		pt := p.Part
		switch pt.Type {
		case protocol.PartTypeText:
			if pt.Time.End == 0 {
				return // not finalized
			}
			if r.f == formatJSON {
				r.line("text", pt)
				return
			}
			acc := r.partText[pt.ID]
			if acc == "" {
				acc = pt.Text // finalized without streaming deltas
			}
			if acc != "" && !strings.HasSuffix(acc, "\n") {
				fmt.Fprint(r.out, "\n")
			}
		case protocol.PartTypeReasoning:
			if pt.Time.End == 0 {
				return // not finalized
			}
			if !r.thinking {
				return
			}
			if r.f == formatJSON {
				r.line("reasoning", pt)
				return
			}
			// verbatim: multi-line reasoning spans physical lines, first
			// prefixed (spec §6.1-5)
			fmt.Fprintf(r.errw, "thinking: %s\n", pt.Text)
		case protocol.PartTypeTool:
			// A tool part is "done" when its state reaches completed/error
			// (its own completion signal, independent of the part time).
			if pt.State == nil || (pt.State.Status != "completed" && pt.State.Status != "error") {
				return
			}
			if r.f == formatJSON {
				r.line("tool_use", pt)
				return
			}
			l := "tool: " + pt.Tool
			if pt.State.Title != "" {
				l += " " + pt.State.Title
			}
			if pt.State.Status == "error" && pt.State.Error != "" {
				l += " (error: " + pt.State.Error + ")"
			}
			fmt.Fprintln(r.errw, l)
		}
	case protocol.EventTypeMessagePartDelta:
		var p protocol.MessagePartDeltaProps
		if json.Unmarshal(ev.Properties, &p) != nil || p.SessionID != r.sessionID {
			return
		}
		r.partText[p.PartID] += p.Delta
		if r.f == formatDefault && p.Field == "text" {
			fmt.Fprint(r.out, p.Delta)
		}
	}
}

// finish synthesizes the step_finish NDJSON line at turn end (spec
// §6.2): only when at least one assistant message.updated was seen; the
// field mapping from the LAST assistant info omits absent keys (reason
// when "", tokens when nil, cost when 0 — the omission rule IS the pin).
func (r *renderer) finish() {
	if r.f != formatJSON || !r.sawAssistant || r.lastAssistant == nil {
		return
	}
	r.line("step_finish", &stepFinishPart{
		Type:   "step-finish",
		Reason: r.lastAssistant.Finish,
		Tokens: r.lastAssistant.Tokens,
		Cost:   r.lastAssistant.Cost,
	})
}

// emitError writes the error surface: the NDJSON line (json mode) and
// the stderr line (BOTH modes — spec §6.2 json-mode stderr).
func (r *renderer) emitError(me *protocol.MessageError) {
	if r.f == formatJSON {
		r.errLine(me)
	}
	fmt.Fprintf(r.errw, "error: %s\n", me.Message)
}

func (r *renderer) line(typ string, part any) {
	r.writeLine(ndjsonLine{Type: typ, Timestamp: r.now(), SessionID: r.sessionID, Part: part})
}

func (r *renderer) errLine(me *protocol.MessageError) {
	r.writeLine(ndjsonLine{Type: "error", Timestamp: r.now(), SessionID: r.sessionID, Error: me})
}

func (r *renderer) writeLine(l ndjsonLine) {
	b, err := json.Marshal(l)
	if err != nil {
		return // a marshal failure drops the line; the settle read still
		// decides the exit code
	}
	fmt.Fprintln(r.out, string(b))
}

// ndjsonLine is the §6.2 envelope: field order IS the byte order
// (type, timestamp, sessionID, then the data — part XOR error).
type ndjsonLine struct {
	Type      string                 `json:"type"`
	Timestamp int64                  `json:"timestamp"`
	SessionID string                 `json:"sessionID"`
	Part      any                    `json:"part,omitempty"`
	Error     *protocol.MessageError `json:"error,omitempty"`
}

type stepStartPart struct {
	Type      string `json:"type"`
	MessageID string `json:"messageID"`
}

type stepFinishPart struct {
	Type   string           `json:"type"`
	Reason string           `json:"reason,omitempty"`
	Tokens *protocol.Tokens `json:"tokens,omitempty"`
	Cost   float64          `json:"cost,omitempty"`
}

// runHeader builds the default-format header line ONCE from the resolved
// session (spec §6.1-1): "> agent · providerID/modelID"; a nil Model
// drops the separator.
func runHeader(ses protocol.Session) string {
	if ses.Model == nil {
		return "> " + ses.Agent
	}
	return fmt.Sprintf("> %s · %s/%s", ses.Agent, ses.Model.ProviderID, ses.Model.ID)
}
