// Package fake is a scripted llm.Driver for engine tests and the
// YOLO_LLM=fake wiring.
package fake

import (
	"context"
	"strings"
	"sync"

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
}

// Driver serves scripted turns. Every Stream call is recorded in ReqLog.
// Title requests (first system message starts with titleMarker) draw from
// TitleTurns; all other requests draw from Turns. A missing turn yields an
// empty stream (immediate EOF).
type Driver struct {
	Turns      []Turn
	TitleTurns []Turn
	ReqLog     []llm.Request

	mu sync.Mutex
}

func New(turns ...Turn) *Driver {
	return &Driver{Turns: turns}
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

// Stream implements llm.Driver.
func (d *Driver) Stream(ctx context.Context, req llm.Request) (llm.PartStream, error) {
	d.mu.Lock()
	d.ReqLog = append(d.ReqLog, req)
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
	d.mu.Unlock()

	if turn.Err != nil {
		return llm.PartStream{}, turn.Err
	}
	ch := make(chan llm.Part, len(turn.Parts)+1)
	go func() {
		defer close(ch)
		for _, p := range turn.Parts {
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
