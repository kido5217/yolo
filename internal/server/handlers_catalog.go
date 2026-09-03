package server

import (
	"net/http"
	"slices"

	"github.com/kido5217/yolo/internal/protocol"
)

// baseAgents are the built-in agents (upstream agent.ts, locked text).
var baseAgents = []protocol.Agent{
	{Name: "build", Description: "The default agent. Executes tools based on configured permissions.", Mode: "primary", Options: map[string]any{}},
	{Name: "plan", Description: "Plan mode. Disallows all edit tools.", Mode: "primary", Options: map[string]any{}},
	{Name: "yolo", Description: "Yolo agent. Permits everything without prompts.", Mode: "primary", Options: map[string]any{}},
}

// commands is the locked minimal command set (T20).
var commands = []protocol.Command{
	{Name: "/help", Description: "show help", Hints: []string{}},
	{Name: "/new", Description: "new session", Hints: []string{}},
	{Name: "/model", Description: "pick model", Hints: []string{}},
	{Name: "/agents", Description: "pick agent", Hints: []string{}},
	{Name: "/quit", Description: "exit", Hints: []string{}},
}

// handleAgent lists build/plan/yolo plus config-defined ids (name only,
// "Custom agent." stub).
func (s *Server) handleAgent(w http.ResponseWriter, _ *http.Request) {
	out := make([]protocol.Agent, 0, 4)
	out = append(out, baseAgents...)
	// best-effort: a global-dir or config load failure just omits
	// config-defined agents.
	if gdir, gerr := s.globalDir(); gerr == nil {
		if cfg, err := s.Config.LoadAt(gdir, s.WorkDir); err == nil {
			known := map[string]bool{"build": true, "plan": true, "yolo": true}
			ids := make([]string, 0, len(cfg.Agents))
			for id := range cfg.Agents {
				if !known[id] {
					ids = append(ids, id)
				}
			}
			slices.Sort(ids)
			for _, id := range ids {
				out = append(out, protocol.Agent{Name: id, Description: "Custom agent.", Mode: "primary", Options: map[string]any{}})
			}
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCommandList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, commands)
}
