package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/kido5217/yolo/internal/config"
)

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
// configWriteMu serializes the read->merge->write of any yolo.jsonc (project
// or global) so concurrent PATCHes to the same file cannot lose an update
// (③). Writes are rare; a single process-wide lock suffices (no per-path map).
var configWriteMu sync.Mutex

func mergeWriteConfig(path string, partial map[string]any, ensureDir bool) (map[string]any, error) {
	configWriteMu.Lock()
	defer configWriteMu.Unlock()
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
