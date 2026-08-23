package session

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/kido5217/yolo/internal/llm"
	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/provider"
	"github.com/kido5217/yolo/internal/storage"
)

// round is one model round's assistant message state: the row id, its
// creation time, and the message identity re-published on finalize.
type round struct {
	id  string
	now int64
	msg protocol.Message
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
	if err := e.db.CreateMessage(ctx, storage.MessageRow{
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
		// Finalization must land even when the turn ctx is cancelled
		// (abort): a cancelled ctx would drop the terminal part write and
		// leave the part "running" in the store.
		if err := e.db.UpsertPart(context.WithoutCancel(ctx), row); err != nil {
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
			e.finishRound(ctx, t, r, usage, finish)
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
				e.finishRound(ctx, t, r, usage, finish)
				return false, ctx.Err()
			}
			if isOverflowError(p.Err) {
				e.lg.Info("overflow detected", "session_id", t.sessionID, "model", t.model.ID, "reason", "api_error")
				e.saveSynthetic(ctx, t, r, overflowNote(t.model, 0, p.Err))
				e.finishRound(ctx, t, r, usage, finish)
				return false, nil
			}
			// Mid-stream failure after content: keep the partial text, note
			// the error on a synthetic part (excluded from history replay —
			// the model never sees it) and fail the turn (no retry).
			e.saveSynthetic(ctx, t, r, p.Err.Error())
			e.finishRound(ctx, t, r, usage, finish)
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
				e.finishRound(ctx, t, r, usage, finish)
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
		e.saveSynthetic(ctx, t, r, overflowNote(t.model, usage.Input, nil))
		e.finishRound(ctx, t, r, usage, finish)
		return false, nil
	}
	e.finishRound(ctx, t, r, usage, finish)

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
			e.finishRound(ctx, t, r, nil, "")
			return llm.PartStream{}, ctx.Err()
		}
		if !llm.IsTransient(sErr) {
			if isOverflowError(sErr) {
				e.lg.Info("overflow detected", "session_id", t.sessionID, "model", t.model.ID, "reason", "api_error")
				e.saveSynthetic(ctx, t, r, overflowNote(t.model, 0, sErr))
				e.finishRound(ctx, t, r, nil, "")
				// The round is already finalized: the caller ends the
				// turn idle without reading a stream.
				return llm.PartStream{}, errRoundEnded
			}
			// Pre-stream failure: keep the decoded provider text on a
			// synthetic note (excluded from history replay) and fail the
			// turn (mid-stream parity).
			e.saveSynthetic(ctx, t, r, sErr.Error())
			e.finishRound(ctx, t, r, nil, "")
			return llm.PartStream{}, sErr
		}
		if attempt >= maxRetryAttempts {
			e.finishRound(ctx, t, r, nil, "")
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
				Type: protocol.SessionStatusRetry, Attempt: attempt,
				Message: sErr.Error(), Next: delay.Milliseconds(),
			},
		})
		retry.Reset(delay)
		select {
		case <-retry.C:
		case <-ctx.Done():
			e.finishRound(ctx, t, r, nil, "")
			return llm.PartStream{}, ctx.Err()
		}
	}
}

// finishRound completes the assistant message row and re-publishes
// message.updated with the final state, deriving cost/tokens from the round's
// usage (nil-safe). It is called on every round-exit path (success, failure,
// abort, retry exhaustion, overflow, max-steps).
func (e *Engine) finishRound(ctx context.Context, t *turn, r *round, usage *llm.Usage, finish string) {
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
	// Finalization must land even when the turn ctx is cancelled (abort):
	// a cancelled ctx would drop the terminal message update (cost/tokens/
	// time_completed) and the round's snapshot read.
	if err := e.db.UpdateMessage(context.WithoutCancel(ctx), storage.MessageRow{
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
	if mw, aerr := e.roundAsMessage(context.WithoutCancel(ctx), t, r); aerr == nil {
		t.hist = append(t.hist, mw)
	}
}

// roundAsMessage builds the snapshot entry for a completed assistant round:
// its final message info + non-synthetic parts (⑪).
func (e *Engine) roundAsMessage(ctx context.Context, t *turn, r *round) (protocol.MessageWithParts, error) {
	prs, err := e.db.ListParts(ctx, r.id)
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
func (e *Engine) saveSynthetic(ctx context.Context, t *turn, r *round, text string) {
	syn := true
	start := e.clock()
	p := protocol.Part{
		ID: protocol.NewID("prt"), SessionID: t.sessionID, MessageID: r.id,
		Type: "text", Text: text, IsSynthetic: &syn,
		Time: protocol.PartTime{Start: start, End: e.clock()},
	}
	row, perr := storage.ProtocolToPart(p)
	if perr != nil {
		e.lg.Error("persist part marshal failed", "part_id", p.ID, "session_id", t.sessionID, "error", perr)
		return
	}
	if err := e.db.UpsertPart(ctx, row); err != nil {
		e.lg.Error("persist part failed", "part_id", p.ID, "session_id", t.sessionID, "error", err)
	}
	e.publish(protocol.EventTypeMessagePartUpdated, protocol.MessagePartUpdatedProps{
		SessionID: t.sessionID, Part: p, Time: e.clock(),
	})
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
