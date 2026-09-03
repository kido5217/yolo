package session

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/kido5217/yolo/internal/llm"
	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/storage"
)

// titleCancel carries one title stream's cancel as a pointer so a stale
// title's exit can check identity against the tracked entry (func values
// are incomparable; only == nil is allowed on them).
type titleCancel struct{ cancel context.CancelFunc }

// maybeScheduleTitle fires the one-shot title generation for the session's
// first user message when the title is still the default.
func (e *Engine) maybeScheduleTitle(ctx context.Context, t *turn, userText string) {
	if t.row.Title != "" && t.row.Title != "New session" {
		return
	}
	msgs, err := e.db.ListMessages(ctx, t.sessionID)
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
	// Best-effort: a config load failure degrades to env-only key
	// resolution (nil cfg); the title stays the default either way.
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
	if err := e.db.UpdateSession(ctx, t.sessionID, storage.SessionRow{Title: title, TimeUpdated: e.clock()}); err != nil {
		return
	}
	updated, err := e.db.GetSession(ctx, t.sessionID)
	if err != nil {
		return
	}
	msgs, err := e.db.ListMessages(ctx, t.sessionID)
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
