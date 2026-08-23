package tui

import (
	"fmt"
	"strconv"
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
	segs := []string{
		model,
		agent,
		"↑" + strconv.FormatInt(tokens.Input, 10) + " ↓" + strconv.FormatInt(tokens.Output, 10),
		fmt.Sprintf("$%.4f", cost),
	}
	switch {
	case a.resyncing:
		segs = append(segs, errRed.Render("◌ reconnecting"))
	case a.store.Live:
		segs = append(segs, okGreen.Render("● live"))
	default:
		segs = append(segs, errRed.Render("○ off"))
	}
	if seg := a.statusSeg(); seg != "" {
		segs = append(segs, seg)
	}
	return strings.Join(segs, " · ")
}
