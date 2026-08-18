// Package fake is a scripted llm.Driver for engine tests and the
// YOLO_LLM=fake wiring.
package fake

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kido5217/yolo/internal/llm"
)

// titleMarker matches the first line of the title-generation prompt
// (prompt/title.txt): requests whose first system message starts with it
// consume TitleTurns instead of Turns.
const titleMarker = "You are a title generator"

// Turn is one scripted Stream reply: parts emitted in order, the last MUST
// carry Finish. For Kind:"tool" parts, Text holds the tool arguments JSON
// (LOCKED convention — the driver-level Part.Args is the canonical carrier,
// Text mirrors it for script readability).
type Turn struct {
	Parts []llm.Part
	// Err, if set: Stream returns (zero stream, Err).
	Err error
	// Auto marks the synthesizing placeholder (AutoText): when no scripted
	// turn remains, a text part "ok-<n>" is emitted instead of an empty
	// stream, where <n> is the 1-based request number.
	Auto bool
	// Delay holds the reply open for d before any part is emitted
	// (slow-turn tests, script delay_ms).
	Delay time.Duration
}

// AutoText is the synthesizing placeholder turn for harnesses that do not
// care about reply content (server tests, in-process smoke).
func AutoText() Turn { return Turn{Auto: true} }

// Driver serves scripted turns. Every Stream call is recorded in ReqLog.
// Title requests (first system message starts with titleMarker) draw from
// TitleTurns; all other requests draw from Turns. A missing turn yields a
// synthesized reply when AutoText() was passed to New, else an empty stream
// (immediate EOF).
type Driver struct {
	Turns      []Turn
	TitleTurns []Turn
	ReqLog     []llm.Request

	mu        sync.Mutex
	auto      bool
	baseDelay time.Duration
}

func New(turns ...Turn) *Driver {
	d := &Driver{}
	for _, tr := range turns {
		if tr.Auto {
			d.auto = true
			continue
		}
		d.Turns = append(d.Turns, tr)
	}
	return d
}

// Requests returns a copy of the recorded requests (ReqLog is appended to by
// concurrent Stream calls).
func (d *Driver) Requests() []llm.Request {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]llm.Request, len(d.ReqLog))
	copy(out, d.ReqLog)
	return out
}

// SetDelay makes subsequently synthesized (Auto) replies hold open for d
// before emitting; scripted turns use their own Turn.Delay.
func (d *Driver) SetDelay(dur time.Duration) {
	d.mu.Lock()
	d.baseDelay = dur
	d.mu.Unlock()
}

// Stream implements llm.Driver.
func (d *Driver) Stream(ctx context.Context, req llm.Request) (llm.PartStream, error) {
	d.mu.Lock()
	d.ReqLog = append(d.ReqLog, req)
	n := len(d.ReqLog) // 1-based request number for synthesized "ok-n" text
	isTitle := len(req.Messages) > 0 && req.Messages[0].Role == llm.RoleSystem &&
		strings.HasPrefix(req.Messages[0].Content, titleMarker)
	var turn Turn
	if isTitle {
		if len(d.TitleTurns) > 0 {
			turn, d.TitleTurns = d.TitleTurns[0], d.TitleTurns[1:]
		}
	} else {
		if len(d.Turns) > 0 {
			turn, d.Turns = d.Turns[0], d.Turns[1:]
		}
	}
	synth := d.auto && turn.Err == nil && len(turn.Parts) == 0
	delay := turn.Delay
	if synth {
		delay = d.baseDelay
	}
	d.mu.Unlock()

	if turn.Err != nil {
		return llm.PartStream{}, turn.Err
	}
	parts := turn.Parts
	if synth {
		parts = []llm.Part{{
			Kind:   "text",
			Text:   "ok-" + strconv.Itoa(n),
			Usage:  &llm.Usage{Input: 1, Output: 1},
			Finish: "stop",
		}}
	}
	ch := make(chan llm.Part, len(parts)+1)
	go func() {
		defer close(ch)
		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				ch <- llm.Part{Finish: "error", Err: ctx.Err()}
				return
			}
		}
		for _, p := range parts {
			select {
			case <-ctx.Done():
				ch <- llm.Part{Finish: "error", Err: ctx.Err()}
				return
			case ch <- p:
			}
		}
	}()
	return llm.PartStream{Parts: ch}, nil
}

type scriptPart struct {
	Kind   string          `json:"kind"`
	Name   string          `json:"name,omitempty"`
	CallID string          `json:"call_id,omitempty"`
	Args   json.RawMessage `json:"args,omitempty"`
	Text   string          `json:"text"`
	Usage  *llm.Usage      `json:"usage,omitempty"`
	Finish string          `json:"finish,omitempty"`
}

type scriptTurn struct {
	Parts   []scriptPart `json:"parts"`
	DelayMS int64        `json:"delay_ms"`
}

// FromScript loads a driver from a JSON script file (M5 format):
// [{"parts":[{"kind":"text","text":"hi","finish":"stop","usage":{"input":1,"output":1}}],"delay_ms":0}]
// delay_ms is optional per turn (0 = emit immediately).
func FromScript(path string) (*Driver, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw []scriptTurn
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("fake: parse script: %w", err)
	}
	d := New()
	for _, tr := range raw {
		parts := make([]llm.Part, 0, len(tr.Parts))
		for _, p := range tr.Parts {
			parts = append(parts, llm.Part{
				Kind:   p.Kind,
				Name:   p.Name,
				CallID: p.CallID,
				Args:   p.Args,
				Text:   p.Text,
				Usage:  p.Usage,
				Finish: p.Finish,
			})
		}
		d.Turns = append(d.Turns, Turn{Parts: parts, Delay: time.Duration(tr.DelayMS) * time.Millisecond})
	}
	return d, nil
}
