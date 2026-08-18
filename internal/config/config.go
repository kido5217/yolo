// Package config implements yolo.jsonc / yolo.json discovery, JSONC parsing,
// deterministic deep merge, and {env:NAME} substitution.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"

	"github.com/tidwall/jsonc"

	"github.com/kido5217/yolo/internal/protocol"
)

// XDG roots.
func Home() string {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return v
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".config")
}

func Data() string {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return v
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".local", "share")
}

func Cache() string {
	if v := os.Getenv("XDG_CACHE_HOME"); v != "" {
		return v
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".cache")
}

func GlobalYoloDir() string { return filepath.Join(Home(), "yolo") }
func DataYoloDir() string   { return filepath.Join(Data(), "yolo") }
func CacheYoloDir() string  { return filepath.Join(Cache(), "yolo") }

// Dirs carries the three XDG roots; zero value = real XDG. Server/test injection.
type Dirs struct{ Home, Data, Cache string }

var envPat = regexp.MustCompile(`\{env:([A-Za-z_][A-Za-z0-9_]*)\}`)

// Loader owns an env view (nil => os.Environ) and resolves the effective config.
type Loader struct{ Env map[string]string }

// EnvVal reports whether name is set in the loader's env view.
func (l Loader) EnvVal(name string) (string, bool) {
	if l.Env != nil {
		v, ok := l.Env[name]
		return v, ok
	}
	return os.LookupEnv(name)
}

// Load wraps LoadAt with the real global dir and workDir as startDir.
func (l Loader) Load(workDir string) (*protocol.Config, error) {
	return l.LoadAt(GlobalYoloDir(), workDir)
}

const (
	globalFiles   = "config.json"
	yoloFilesJSON = "yolo.json"
	yoloFilesC    = "yolo.jsonc"
)

// LoadGlobal reads the global config layer in globalDir: config.json, then
// yolo.json, then yolo.jsonc (later wins); missing files are skipped (M5
// injectable path). No env substitution; Loader.LoadAt applies it for the
// effective config.
func LoadGlobal(globalDir string) (*protocol.Config, error) {
	l := Loader{}
	merged := map[string]any{}
	for _, f := range []string{globalFiles, yoloFilesJSON, yoloFilesC} {
		m, err := l.readFile(filepath.Join(globalDir, f))
		if err != nil {
			return nil, err
		}
		merged = Merge(merged, m)
	}
	cfg, err := cfgFromMap(merged)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// SaveGlobal merges cfg over the existing global config in globalDir and
// rewrites <globalDir>/yolo.jsonc (highest-precedence global file; created
// if absent). JSONC comments are NOT preserved (parse -> merge ->
// MarshalIndent 2-space; flagged deviation, TUI never relies on comments).
func SaveGlobal(globalDir string, cfg *protocol.Config) error {
	cur, err := LoadGlobal(globalDir)
	if err != nil {
		return err
	}
	curMap, err := mapFromCfg(cur)
	if err != nil {
		return err
	}
	cfgMap, err := mapFromCfg(cfg)
	if err != nil {
		return err
	}
	merged := Merge(curMap, cfgMap)
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(globalDir, yoloFilesC), b, 0o644)
}

func cfgFromMap(merged map[string]any) (*protocol.Config, error) {
	b, err := json.Marshal(merged)
	if err != nil {
		return nil, err
	}
	var cfg protocol.Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func mapFromCfg(cfg *protocol.Config) (map[string]any, error) {
	b, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// LoadAt deterministically merges global (globalDir) and project (startDir,
// walked up to filesystem root, innermost last, plus <startDir>/.yolo).
func (l Loader) LoadAt(globalDir, startDir string) (*protocol.Config, error) {
	merged := map[string]any{}

	// Global precedence: config.json -> yolo.json -> yolo.jsonc.
	for _, f := range []string{globalFiles, yoloFilesJSON, yoloFilesC} {
		m, err := l.readFile(filepath.Join(globalDir, f))
		if err != nil {
			return nil, err
		}
		merged = Merge(merged, m)
	}

	// Project chain: startDir up to root, processed outermost first.
	chain := []string{}
	dir := startDir
	for {
		chain = append(chain, dir)
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	for i := len(chain) - 1; i >= 0; i-- {
		for _, f := range []string{yoloFilesJSON, yoloFilesC} {
			m, err := l.readFile(filepath.Join(chain[i], f))
			if err != nil {
				return nil, err
			}
			merged = Merge(merged, m)
		}
	}

	// startDir/.yolo, innermost override of the project directory.
	for _, f := range []string{yoloFilesJSON, yoloFilesC} {
		m, err := l.readFile(filepath.Join(startDir, ".yolo", f))
		if err != nil {
			return nil, err
		}
		merged = Merge(merged, m)
	}

	merged = Substitute(merged, l.EnvVal).(map[string]any)

	b, err := json.Marshal(merged)
	if err != nil {
		return nil, err
	}
	var cfg protocol.Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (l Loader) readFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	return UnmarshalJSONC(data)
}

// UnmarshalJSONC parses JSON with comments into a map.
func UnmarshalJSONC(data []byte) (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal(jsonc.ToJSON(data), &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}

// Merge deep-merges src over dst: maps recurse, the "instructions" key
// concatenates (dedupe, first occurrence wins), other arrays/leaves: src wins.
func Merge(dst, src map[string]any) map[string]any {
	out := make(map[string]any, len(dst)+len(src))
	for k, v := range dst {
		out[k] = v
	}
	for k, v := range src {
		dv, ok := out[k]
		if ok {
			if dm, dok := dv.(map[string]any); dok {
				if sm, sok := v.(map[string]any); sok {
					out[k] = Merge(dm, sm)
					continue
				}
			}
			if k == "instructions" {
				if da, dok := dv.([]any); dok {
					if sa, sok := v.([]any); sok {
						out[k] = concatDedupe(da, sa)
						continue
					}
				}
			}
		}
		out[k] = v
	}
	return out
}

func concatDedupe(dst, src []any) []any {
	out := make([]any, 0, len(dst)+len(src))
	seen := map[string]bool{}
	for _, e := range append(append([]any{}, dst...), src...) {
		if s, ok := e.(string); ok {
			if seen[s] {
				continue
			}
			seen[s] = true
		}
		out = append(out, e)
	}
	return out
}

// Substitute walks v, replacing {env:NAME} occurrences and whole-string env
// names using env (returns ("", false) when unset).
func Substitute(v any, env func(string) (string, bool)) any {
	switch t := v.(type) {
	case string:
		if envPat.MatchString(t) {
			return envPat.ReplaceAllStringFunc(t, func(m string) string {
				name := envPat.FindStringSubmatch(m)[1]
				if val, ok := env(name); ok {
					return val
				}
				return m
			})
		}
		if val, ok := env(t); ok {
			return val
		}
		return t
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = Substitute(val, env)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = Substitute(val, env)
		}
		return out
	default:
		return v
	}
}
