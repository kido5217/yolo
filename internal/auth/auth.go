// Package auth persists per-provider API keys in auth.json and resolves an
// API key in spec order: env -> auth.json -> config.
package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/kido5217/yolo/internal/config"
	"github.com/kido5217/yolo/internal/protocol"
)

// Entry is one provider credential.
type Entry struct {
	Type     string         `json:"type"` // "api"
	Key      string         `json:"key"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Store maps providerID -> Entry.
type Store map[string]Entry

// Path is <DataYoloDir>/auth.json.
func Path() string {
	return filepath.Join(config.DataYoloDir(), "auth.json")
}

// LoadFrom reads a store at path; a missing file is an empty store (M5
// injectable path; Load delegates here).
func LoadFrom(path string) (Store, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Store{}, nil
		}
		return nil, err
	}
	var s Store
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	if s == nil {
		s = Store{}
	}
	return s, nil
}

// Load reads the store; a missing file is an empty store.
func Load() (Store, error) { return LoadFrom(Path()) }

// SaveTo writes the store at path: dir 0700, file 0600 (M5 injectable path;
// Save delegates here).
func SaveTo(s Store, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// Save writes the store dir 0700, file 0600.
func Save(s Store) error { return SaveTo(s, Path()) }

// Set upserts a provider's key.
func (s Store) Set(providerID, key string) {
	entry := s[providerID]
	if entry.Type == "" {
		entry.Type = "api"
	}
	entry.Key = key
	s[providerID] = entry
}

// Delete removes a provider entry.
func (s Store) Delete(providerID string) {
	delete(s, providerID)
}

// EnvName is the environment variable for a provider's API key.
func EnvName(providerID string) string {
	up := strings.ToUpper(providerID)
	var b strings.Builder
	for _, r := range up {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String() + "_API_KEY"
}

// ResolveKey finds a provider API key: env -> auth.json -> config
// provider.<id>.apiKey then provider.<id>.options.apiKey.
func ResolveKey(providerID string, cfg *protocol.Config, env func(string) (string, bool)) (string, bool) {
	if k, ok := env(EnvName(providerID)); ok && k != "" {
		return k, true
	}
	if s, err := Load(); err == nil {
		if e, ok := s[providerID]; ok && e.Key != "" {
			return e.Key, true
		}
	}
	if cfg != nil {
		if pc, ok := cfg.Provider[providerID]; ok {
			if pc.APIKey != "" {
				return pc.APIKey, true
			}
			if k, ok := pc.Options["apiKey"].(string); ok && k != "" {
				return k, true
			}
		}
	}
	return "", false
}
