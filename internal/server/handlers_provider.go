package server

import (
	"net/http"
	"slices"

	"github.com/kido5217/yolo/internal/auth"
	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/provider"
)

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
		st, _ := s.authState(p.ID, p.Auth.RequiresKey, store, cfg)
		p.Auth = &protocol.ProviderAuth{Type: "api", Status: st, RequiresKey: p.Auth.RequiresKey}
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
	slices.Sort(ids)
	for _, id := range ids {
		p := provider.FromConfig(id, cfg.Provider[id])
		st, _ := s.authState(id, p.Auth.RequiresKey, store, cfg)
		p.Auth = &protocol.ProviderAuth{Type: "api", Status: st, RequiresKey: p.Auth.RequiresKey}
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
		if pc, ok := cfg.Provider[id]; ok {
			if pc.APIKey != "" {
				return "loaded", "config"
			}
			if k, ok := pc.Options["apiKey"].(string); ok && k != "" {
				return "loaded", "config"
			}
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
func (s *Server) handleProviderAuth(w http.ResponseWriter, r *http.Request) {
	dir, ok := s.scoped(w, r)
	if !ok {
		return
	}
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
		st, src := s.authState(p.ID, p.Auth.RequiresKey, store, cfg)
		out[p.ID] = map[string]any{
			"key_required": p.Auth.RequiresKey,
			"env":          p.Env,
			"status":       st,
			"source":       src,
		}
	}
	writeJSON(w, http.StatusOK, out)
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
