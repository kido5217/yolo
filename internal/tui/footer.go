package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/protocol"
)

// footerFrames is the locked 5-frame spinner (the first five braille frames).
var footerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼"}

// spinMsg advances the footer spinner one frame.
type spinMsg struct{}

// spinTick arms the next spinner frame.
func (a *App) spinTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg { return spinMsg{} })
}

func (a *App) spinFrame() string {
	return footerFrames[a.spinIdx%len(footerFrames)]
}

// statusSeg renders the footer status segment: spinner + "busy", or
// "retry n: msg" while retrying; empty when idle.
func (a *App) statusSeg() string {
	switch a.store.Status.Type {
	case protocol.SessionStatusBusy:
		return a.spinFrame() + " busy"
	case protocol.SessionStatusRetry:
		return a.spinFrame() + fmt.Sprintf(" retry %d: %s", a.store.Status.Attempt, a.store.Status.Message)
	}
	return ""
}

// footerView renders the locked status footer (visible on both routes):
// model · agent · ↑in ↓out · $cost · conn · status.
func (a *App) footerView() string {
	var (
		model  string
		agent  string
		tokens protocol.Tokens
		cost   float64
	)
	switch a.route {
	case routeSession:
		if a.store.Current != nil {
			if mr := a.store.Current.Model; mr != nil {
				model = mr.ProviderID + "/" + mr.ID
			} else {
				model = "no model"
			}
			agent = a.store.Current.Agent
			tokens = a.store.Current.Tokens
			cost = a.store.Current.Cost
		} else {
			model, agent = "no model", "default"
		}
	default: // routeHome
		if m, ok := a.store.Config["model"].(string); ok && m != "" {
			model = m
		} else {
			model = "no model"
		}
		if ag, ok := a.store.Config["agent"].(string); ok && ag != "" {
			agent = ag
		} else {
			agent = "default"
		}
	}
	muted := a.theme.TextMuted()
	tokensSeg := "↑" + number(tokens.Input) + " ↓" + number(tokens.Output)
	if pct := a.contextPct(); pct >= 0 {
		tokensSeg += fmt.Sprintf(" (%d%%)", pct)
	}
	seg := model + " · " + agent + " · " + tokensSeg
	if cost > 0 {
		seg += " · " + fmt.Sprintf("$%.2f", cost)
	}
	parts := []string{muted.Render(seg)}
	switch {
	case a.resyncing:
		parts = append(parts, a.theme.Error().Render("◌ reconnecting"))
	case a.store.Live:
		parts = append(parts, a.theme.Success().Render("● live"))
	default:
		parts = append(parts, a.theme.Error().Render("○ off"))
	}
	if seg := a.statusSeg(); seg != "" {
		parts = append(parts, muted.Render(seg))
	}
	return strings.Join(parts, " · ")
}

// contextPct is the S7.4 restyle's context percentage (the upstream
// prompt-bar usage shape, prompt/index.tsx:264-282): the round(100 * total
// / context) when the session model resolves (over store.Providers — the
// lazy catalog referent, deviation 241) to a Limit.Context > 0; total =
// the session aggregate input+output+reasoning+cache.read+cache.write
// (the yolo referent for the upstream last-assistant-message total —
// deviation 249). -1 when unknown (the segment omitted).
func (a *App) contextPct() int64 {
	if a.route != routeSession || a.store.Current == nil {
		return -1
	}
	mr := a.store.Current.Model
	if mr == nil {
		return -1
	}
	for _, p := range a.store.Providers {
		m, ok := p.Models[mr.ID]
		if !ok || p.ID != mr.ProviderID {
			continue
		}
		if m.Limit.Context <= 0 {
			return -1
		}
		t := a.store.Current.Tokens
		total := t.Input + t.Output + t.Reasoning + t.Cache.Read + t.Cache.Write
		return int64(math.Round(100 * float64(total) / float64(m.Limit.Context)))
	}
	return -1
}
