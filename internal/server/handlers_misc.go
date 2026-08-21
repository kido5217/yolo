package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"

	"github.com/kido5217/yolo/internal/auth"
	"github.com/kido5217/yolo/internal/config"
	"github.com/kido5217/yolo/internal/permission"
	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/provider"
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

// providerEntries resolves the provider list for dir: registry providers plus
// config-defined ones, with per-request auth state and env. (The plan scopes
// /provider here so config providers resolve against the request directory.)
func (s *Server) providerEntries(dir string) ([]protocol.Provider, error) {
	gdir, err := s.globalDir()
	if err != nil {
		return nil, err
	}
	cfg, err := s.Config.LoadAt(gdir, dir)
	if err != nil {
		return nil, err
	}
	store := s.authSnapshot()
	seen := map[string]bool{}
	out := make([]protocol.Provider, 0, len(cfg.Provider)+2)
	for _, p := range s.Prov.List() {
		seen[p.ID] = true
		st, _ := s.authState(p.ID, p.Auth.KeyRequired, store, cfg)
		p.Auth = &protocol.ProviderAuth{Type: "api", Status: st, KeyRequired: p.Auth.KeyRequired}
		if len(p.Env) == 0 {
			p.Env = []string{auth.EnvName(p.ID)}
		}
		out = append(out, p)
	}
	ids := make([]string, 0, len(cfg.Provider))
	for id := range cfg.Provider {
		if !seen[id] {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		p := provider.FromConfig(id, cfg.Provider[id])
		st, _ := s.authState(id, p.Auth.KeyRequired, store, cfg)
		p.Auth = &protocol.ProviderAuth{Type: "api", Status: st, KeyRequired: p.Auth.KeyRequired}
		if len(p.Env) == 0 {
			p.Env = []string{auth.EnvName(id)}
		}
		out = append(out, p)
	}
	return out, nil
}

func (s *Server) authSnapshot() auth.Store {
	s.authMu.Lock()
	defer s.authMu.Unlock()
	return s.snapshotStoreLocked()
}

// snapshotStoreLocked deep-copies the auth store so callers can read it
// without authMu while handleAuthPut/Delete mutate the live map. The caller
// must hold authMu.
func (s *Server) snapshotStoreLocked() auth.Store {
	out := make(auth.Store, len(s.authStore))
	for k, v := range s.authStore {
		out[k] = v
	}
	return out
}

// authState recomputes a provider's auth state in spec order (env ->
// auth.json -> config apiKey), mirroring auth.ResolveKey.
func (s *Server) authState(id string, keyRequired bool, store auth.Store, cfg *protocol.Config) (status, source string) {
	if !keyRequired {
		return "not-required", "none"
	}
	if v, ok := s.Config.EnvVal(auth.EnvName(id)); ok && v != "" {
		return "loaded", "env"
	}
	if e, ok := store[id]; ok && e.Key != "" {
		return "loaded", "auth.json"
	}
	if cfg != nil {
		if pc, ok := cfg.Provider[id]; ok && pc.APIKey != "" {
			return "loaded", "config"
		}
	}
	return "missing", "none"
}

func (s *Server) handleProvider(w http.ResponseWriter, r *http.Request) {
	dir, ok := s.scoped(w, r)
	if !ok {
		return
	}
	entries, err := s.providerEntries(dir)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, "list providers", err)
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

// handleProviderAuth is the key source view: {key_required, env, status,
// source} per provider (the two pinned keys plus the loaded source).
func (s *Server) handleProviderAuth(w http.ResponseWriter, _ *http.Request) {
	dir := s.WorkDir
	entries, err := s.providerEntries(dir)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, "list providers", err)
		return
	}
	gdir, gerr := s.globalDir()
	if gerr != nil {
		s.fail(w, http.StatusInternalServerError, "load config", gerr)
		return
	}
	cfg, err := s.Config.LoadAt(gdir, dir)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, "load config", err)
		return
	}
	store := s.authSnapshot()
	out := make(map[string]map[string]any, len(entries))
	for _, p := range entries {
		st, src := s.authState(p.ID, p.Auth.KeyRequired, store, cfg)
		out[p.ID] = map[string]any{
			"key_required": p.Auth.KeyRequired,
			"env":          p.Env,
			"status":       st,
			"source":       src,
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleConfigGet(w http.ResponseWriter, r *http.Request) {
	dir, ok := s.scoped(w, r)
	if !ok {
		return
	}
	gdir, gerr := s.globalDir()
	if gerr != nil {
		s.fail(w, http.StatusInternalServerError, "load config", gerr)
		return
	}
	cfg, err := s.Config.LoadAt(gdir, dir)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, "load config", err)
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// handleConfigPatch deep-merges the partial into the project yolo.jsonc
// (created if absent) and returns the parsed+merged config. JSONC comments
// are not preserved (flagged deviation).
func (s *Server) handleConfigPatch(w http.ResponseWriter, r *http.Request) {
	dir, ok := s.scoped(w, r)
	if !ok {
		return
	}
	var partial map[string]any
	if err := decode(r, &partial); err != nil {
		envelope(w, http.StatusBadRequest, "invalid config JSON", nil)
		return
	}
	if _, err := mergeWriteConfig(filepath.Join(dir, "yolo.jsonc"), partial, false); err != nil {
		s.fail(w, http.StatusInternalServerError, "write project config", err)
		return
	}
	gdir, gerr := s.globalDir()
	if gerr != nil {
		s.fail(w, http.StatusInternalServerError, "load config", gerr)
		return
	}
	cfg, err := s.Config.LoadAt(gdir, dir)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, "load config", err)
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// mergeWriteConfig deep-merges partial on top of the existing JSONC layer at
// path, rewrites it as 2-space JSON (comments not preserved, flagged
// deviation), and returns the merged object. ensureDir creates path's parent
// directory before writing (used by the global route; project layer sits in
// the already-existing working directory).
func mergeWriteConfig(path string, partial map[string]any, ensureDir bool) (map[string]any, error) {
	existing := map[string]any{}
	if raw, err := os.ReadFile(path); err == nil {
		m, uerr := config.UnmarshalJSONC(raw)
		if uerr != nil {
			return nil, uerr
		}
		existing = m
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	merged := config.Merge(existing, partial)
	if ensureDir {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
	}
	b, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return nil, err
	}
	return merged, nil
}

func (s *Server) handleGlobalConfigGet(w http.ResponseWriter, _ *http.Request) {
	gdir, err := s.globalDir()
	if err != nil {
		s.fail(w, http.StatusInternalServerError, "load global config", err)
		return
	}
	cfg, err := config.LoadGlobal(gdir)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, "load global config", err)
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// handleGlobalConfigPatch merge-writes <globalDir>/yolo.jsonc (highest-
// precedence global file, per the M5 lock; deviation 37) and returns the
// merged object.
func (s *Server) handleGlobalConfigPatch(w http.ResponseWriter, r *http.Request) {
	var partial map[string]any
	if err := decode(r, &partial); err != nil {
		envelope(w, http.StatusBadRequest, "invalid config JSON", nil)
		return
	}
	dir, err := s.globalDir()
	if err != nil {
		s.fail(w, http.StatusInternalServerError, "load global config", err)
		return
	}
	merged, err := mergeWriteConfig(filepath.Join(dir, "yolo.jsonc"), partial, true)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, "write global config", err)
		return
	}
	writeJSON(w, http.StatusOK, merged)
}

// handleAuthPut upserts the key in the boot-lifetime store and persists it.
func (s *Server) handleAuthPut(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Key string `json:"key"`
	}
	if err := decode(r, &body); err != nil || body.Key == "" {
		envelope(w, http.StatusBadRequest, "invalid key", nil)
		return
	}
	if s.authErr != nil {
		s.fail(w, http.StatusInternalServerError, "auth store unavailable", s.authErr)
		return
	}
	s.authMu.Lock()
	s.authStore.Set(r.PathValue("providerID"), body.Key)
	snap := s.snapshotStoreLocked()
	s.authMu.Unlock()
	if err := auth.SaveTo(snap, s.authPath); err != nil {
		s.fail(w, http.StatusInternalServerError, "save auth", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAuthDelete(w http.ResponseWriter, r *http.Request) {
	if s.authErr != nil {
		s.fail(w, http.StatusInternalServerError, "auth store unavailable", s.authErr)
		return
	}
	s.authMu.Lock()
	s.authStore.Delete(r.PathValue("providerID"))
	snap := s.snapshotStoreLocked()
	s.authMu.Unlock()
	if err := auth.SaveTo(snap, s.authPath); err != nil {
		s.fail(w, http.StatusInternalServerError, "save auth", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
			sort.Strings(ids)
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

// handlePermissionList returns pending asks for every session in the request
// directory (M5: /permission is scoped).
func (s *Server) handlePermissionList(w http.ResponseWriter, r *http.Request) {
	dir, ok := s.scoped(w, r)
	if !ok {
		return
	}
	rows, err := s.DB.ListSessions(dir, 0)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, "list sessions", err)
		return
	}
	out := make([]protocol.PermissionAskedProps, 0)
	for _, row := range rows {
		reqs, err := s.Perm.Pending(row.ID)
		if err != nil {
			s.fail(w, http.StatusInternalServerError, "list permissions", err)
			return
		}
		for _, q := range reqs {
			out = append(out, askedProps(q))
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func askedProps(q permission.Request) protocol.PermissionAskedProps {
	meta := make(map[string]any, len(q.Meta)+2)
	for k, v := range q.Meta {
		meta[k] = v
	}
	if q.Tool != "" {
		meta["tool"] = q.Tool
	}
	if q.Agent != "" {
		meta["agent"] = q.Agent
	}
	return protocol.PermissionAskedProps{
		ID:         q.RequestID,
		SessionID:  q.SessionID,
		Permission: q.Permission,
		Patterns:   q.Resources,
		Always:     q.Always,
		Metadata:   meta,
	}
}

// handlePermissionReply answers a parked ask; the body is validated before
// the unknown-id lookup (LOCKED: bad response is 400, not 404).
func (s *Server) handlePermissionReply(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Response string `json:"response"`
	}
	if err := decode(r, &body); err != nil {
		envelope(w, http.StatusBadRequest, "invalid reply", nil)
		return
	}
	switch body.Response {
	case "once", "always", "reject":
	default:
		envelope(w, http.StatusBadRequest, "invalid response", nil)
		return
	}
	id := r.PathValue("requestID")
	if err := s.Perm.Reply(id, body.Response); err != nil {
		if errors.Is(err, permission.ErrNoPending) {
			envelope(w, http.StatusNotFound, "no pending permission request", nil)
			return
		}
		s.fail(w, http.StatusInternalServerError, "reply permission", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
