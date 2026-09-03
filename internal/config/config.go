// Package config implements yolo.jsonc / yolo.json discovery, JSONC parsing,
// deterministic deep merge, and {env:NAME} substitution.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/tidwall/jsonc"

	"github.com/kido5217/yolo/internal/protocol"
)

// XDG roots; a broken home (no XDG override, $HOME unset and no passwd
// entry) is an error so config never silently degrades to CWD-relative
// paths.
func Home() (string, error) {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return v, nil
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("xdg config home: %w", err)
	}
	return filepath.Join(h, ".config"), nil
}

func Data() (string, error) {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return v, nil
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("xdg data home: %w", err)
	}
	return filepath.Join(h, ".local", "share"), nil
}

func Cache() (string, error) {
	if v := os.Getenv("XDG_CACHE_HOME"); v != "" {
		return v, nil
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("xdg cache home: %w", err)
	}
	return filepath.Join(h, ".cache"), nil
}

func GlobalYoloDir() (string, error) {
	h, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, "yolo"), nil
}

func DataYoloDir() (string, error) {
	d, err := Data()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "yolo"), nil
}

func CacheYoloDir() (string, error) {
	c, err := Cache()
	if err != nil {
		return "", err
	}
	return filepath.Join(c, "yolo"), nil
}

// Dirs carries the three XDG roots and the resolved profile id; zero
// value = real XDG + marker-resolved profile. Server/test injection.
type Dirs struct {
	Home, Data, Cache string
	// Profile is the process-selected profile id; the CLI pins it at
	// startup (after resolving --profile > YOLO_PROFILE > active marker).
	// Empty only in the testutil harness: the server then resolves the
	// active marker per request (creating the default profile on a first
	// run).
	Profile string
}

var envPat = regexp.MustCompile(`\{env:([A-Za-z_][A-Za-z0-9_]*)\}`)

// ProfileEnvVar selects the profile for this process, below the --profile
// flag and above the active marker.
const ProfileEnvVar = "YOLO_PROFILE"

// Loader owns an env view (nil => os.Environ) and resolves the effective
// config. Profile is the --profile override; empty selects via the
// YOLO_PROFILE env, then the active marker.
type Loader struct {
	Env     map[string]string
	Profile string
}

// EnvVal reports whether name is set in the loader's env view.
func (l Loader) EnvVal(name string) (string, bool) {
	if l.Env != nil {
		v, ok := l.Env[name]
		return v, ok
	}
	return os.LookupEnv(name)
}

// selectProfile resolves the profile id for root via ProcessProfile with
// the loader's own env view.
func (l Loader) selectProfile(root string) (string, error) {
	return ProcessProfile(root, l.Profile, l.Env)
}

// Load resolves the active profile in the real global dir, then merges
// <root>/<profile>/ with the project chain up from workDir.
func (l Loader) Load(workDir string) (*protocol.Config, error) {
	g, err := GlobalYoloDir()
	if err != nil {
		return nil, err
	}
	id, err := l.selectProfile(g)
	if err != nil {
		return nil, err
	}
	return l.LoadAt(filepath.Join(g, id), workDir)
}

const (
	globalFile    = "config.json"
	yoloFileJSON  = "yolo.json"
	yoloFileJSONC = "yolo.jsonc"
)

// LoadGlobal reads the global config layer in globalDir: config.json, then
// yolo.json, then yolo.jsonc (later wins); missing files are skipped (M5
// injectable path). No env substitution; Loader.LoadAt applies it for the
// effective config.
func LoadGlobal(globalDir string) (*protocol.Config, error) {
	l := Loader{}
	merged := map[string]any{}
	for _, f := range []string{globalFile, yoloFileJSON, yoloFileJSONC} {
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
// if absent, mode 0600 — the config may carry provider API keys). JSONC
// comments are NOT preserved (parse -> merge -> MarshalIndent 2-space;
// flagged deviation, TUI never relies on comments).
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
	p := filepath.Join(globalDir, yoloFileJSONC)
	if err := os.WriteFile(p, b, 0o600); err != nil {
		return err
	}
	// Chmod upgrades a pre-existing file written before the 0600 mode;
	// WriteFile only applies the mode at creation (same pattern as
	// auth.SaveTo).
	return os.Chmod(p, 0o600)
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
	for _, f := range []string{globalFile, yoloFileJSON, yoloFileJSONC} {
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
		for _, f := range []string{yoloFileJSON, yoloFileJSONC} {
			m, err := l.readFile(filepath.Join(chain[i], f))
			if err != nil {
				return nil, err
			}
			merged = Merge(merged, m)
		}
	}

	// startDir/.yolo, innermost override of the project directory.
	for _, f := range []string{yoloFileJSON, yoloFileJSONC} {
		m, err := l.readFile(filepath.Join(startDir, ".yolo", f))
		if err != nil {
			return nil, err
		}
		merged = Merge(merged, m)
	}

	// A map in is a map out; the ok check keeps the assertion safe.
	substituted := Substitute(merged, l.EnvVal)
	merged, ok := substituted.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("env substitution: unexpected result type %T", substituted)
	}

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
	m, err := UnmarshalJSONC(data)
	if err != nil {
		// LoadAt merges many candidate files; a bare parse error names
		// none of them.
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return m, nil
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
		if !ok {
			out[k] = v
			continue
		}
		if merged, mergedOK := mergePair(k, dv, v); mergedOK {
			out[k] = merged
		} else {
			out[k] = v
		}
	}
	return out
}

// mergePair applies Merge's per-key rule to a dst/src value pair: map over
// map recurses, "instructions" slice over slice concatenates; ok is false
// when no merge applies (src wins as-is).
func mergePair(key string, dst, src any) (any, bool) {
	if dm, ok := dst.(map[string]any); ok {
		if sm, ok := src.(map[string]any); ok {
			return Merge(dm, sm), true
		}
		return nil, false
	}
	if key == "instructions" {
		if da, ok := dst.([]any); ok {
			if sa, ok := src.([]any); ok {
				return concatDedupe(da, sa), true
			}
		}
	}
	return nil, false
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
