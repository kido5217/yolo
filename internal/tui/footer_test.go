package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/store"
)

// footerApp builds the root app with a fixed 80x24 window and a preloaded
// store (footer unit table; the client is a dead endpoint, no requests are
// made). The Current session and its Model ref are copied (the only field
// with a pointer) so subtest mutations stay local.
func footerApp(st store.Store) *recApp {
	a := testApp()
	if st.Current != nil {
		cp := *st.Current
		if st.Current.Model != nil {
			m := *st.Current.Model
			cp.Model = &m
		}
		st.Current = &cp
	}
	a.store = st
	a.size = tea.WindowSizeMsg{Width: 80, Height: 24}
	return a
}

func TestFooterRender(t *testing.T) {
	idle := store.Store{
		Conn:    true,
		Current: &protocol.Session{ID: "ses_1", Agent: "build", Model: refModel("kido", "q"), Cost: 0.0002, Tokens: protocol.Tokens{Input: 123, Output: 45}},
	}
	tests := []struct {
		name   string
		route  route
		cfg    map[string]any
		mutate func(*store.Store)
		want   string
	}{
		{
			name:  "session idle live",
			route: routeSession,
			want:  "kido/q · build · ↑123 ↓45 · $0.0002 · ● live",
		},
		{
			name:   "session sse off",
			route:  routeSession,
			mutate: func(s *store.Store) { s.Conn = false },
			want:   "kido/q · build · ↑123 ↓45 · $0.0002 · ○ off",
		},
		{
			name:   "session busy",
			route:  routeSession,
			mutate: func(s *store.Store) { s.Status = protocol.SessionStatus{Type: protocol.StatusBusy} },
			want:   "kido/q · build · ↑123 ↓45 · $0.0002 · ● live · ⠋ busy",
		},
		{
			name:  "session retry shows attempt and message",
			route: routeSession,
			mutate: func(s *store.Store) {
				s.Status = protocol.SessionStatus{Type: protocol.StatusRetry, Attempt: 2, Message: "rate limited"}
			},
			want: "kido/q · build · ↑123 ↓45 · $0.0002 · ● live · ⠋ retry 2: rate limited",
		},
		{
			name:   "session without model",
			route:  routeSession,
			mutate: func(s *store.Store) { s.Current.Model = nil },
			want:   "no model · build · ↑123 ↓45 · $0.0002 · ● live",
		},
		{
			name:   "cost rounds to four decimals",
			route:  routeSession,
			mutate: func(s *store.Store) { s.Current.Cost = 1.23456 },
			want:   "kido/q · build · ↑123 ↓45 · $1.2346 · ● live",
		},
		{
			name:  "home uses config defaults",
			route: routeHome,
			cfg:   map[string]any{"model": "kido/q", "agent": "build"},
			want:  "kido/q · build · ↑0 ↓0 · $0.0000 · ● live",
		},
		{
			name:  "home without config",
			route: routeHome,
			want:  "no model · default · ↑0 ↓0 · $0.0000 · ● live",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := footerApp(idle)
			a.route = tt.route
			if tt.route == routeSession {
				a.cur = "ses_1"
			}
			if tt.cfg != nil {
				a.store.Config = tt.cfg
			}
			if tt.mutate != nil {
				tt.mutate(&a.store)
			}
			got := stripANSI(a.footerView())
			if got != tt.want {
				t.Errorf("footerView = %q, want %q", got, tt.want)
			}
		})
	}
}
