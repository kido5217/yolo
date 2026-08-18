package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type zenModelEntry struct {
	Name       string `json:"name"`
	Family     string `json:"family"`
	ToolCall   bool   `json:"tool_call"`
	Reasoning  bool   `json:"reasoning"`
	Attachment bool   `json:"attachment"`
	Limit      struct {
		Context int `json:"context"`
		Output  int `json:"output"`
	} `json:"limit"`
	Cost struct {
		Input      float64 `json:"input"`
		Output     float64 `json:"output"`
		CacheRead  float64 `json:"cache_read"`
		CacheWrite float64 `json:"cache_write"`
	} `json:"cost"`
	Provider struct {
		Npm string `json:"npm"`
	} `json:"provider"`
}

// ParseZenCatalog keeps only paid models (cost.input > 0), excludes
// "@ai-sdk/google" providers, and maps the adapter from provider.npm
// ("@ai-sdk/anthropic" -> anthropic, everything else -> openai).
func ParseZenCatalog(raw []byte) ([]Model, error) {
	var cat struct {
		Opencode struct {
			Models map[string]zenModelEntry `json:"models"`
		} `json:"opencode"`
	}
	if err := json.Unmarshal(raw, &cat); err != nil {
		return nil, fmt.Errorf("zen catalog: %w", err)
	}
	out := []Model{}
	for id, m := range cat.Opencode.Models {
		if m.Cost.Input <= 0 {
			continue
		}
		if strings.HasPrefix(m.Provider.Npm, "@ai-sdk/google") {
			continue
		}
		adapter := "openai"
		if m.Provider.Npm == "@ai-sdk/anthropic" {
			adapter = "anthropic"
		}
		out = append(out, Model{
			ID: id, Name: m.Name, Family: m.Family, Adapter: adapter,
			ToolCall: m.ToolCall, Reasoning: m.Reasoning, Attachment: m.Attachment,
			Context: m.Limit.Context, Output: m.Limit.Output,
			CostIn: m.Cost.Input, CostOut: m.Cost.Output,
			CostCacheRead: m.Cost.CacheRead, CostCacheWrite: m.Cost.CacheWrite,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// ZenMeta carries the catalog's top-level "opencode" metadata.
type ZenMeta struct {
	Name string
	API  string
	Env  []string
}

func zenMeta(raw []byte) ZenMeta {
	var cat struct {
		Opencode struct {
			Name string   `json:"name"`
			API  string   `json:"api"`
			Env  []string `json:"env"`
		} `json:"opencode"`
	}
	_ = json.Unmarshal(raw, &cat)
	return ZenMeta{Name: cat.Opencode.Name, API: cat.Opencode.API, Env: cat.Opencode.Env}
}

// CatalogPolicy serves the zen catalog with a TTL-bounded cache: a fresh
// cache file wins; otherwise it fetches live and rewrites the cache
// atomically; a failed fetch falls back to the stale cache.
type CatalogPolicy struct {
	cachePath string
	ttl       time.Duration
	liveURL   string
}

func NewCatalogPolicy(cachePath string, ttlMin int, liveURL string) *CatalogPolicy {
	return &CatalogPolicy{cachePath: cachePath, ttl: time.Duration(ttlMin) * time.Minute, liveURL: liveURL}
}

func (p *CatalogPolicy) Load(ctx context.Context) ([]byte, error) {
	if b, ok := p.freshCache(); ok {
		return b, nil
	}
	fctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	data, err := p.fetch(fctx)
	if err != nil {
		if stale, ok := p.readCache(); ok {
			return stale, nil
		}
		return nil, err
	}
	_ = p.writeAtomic(data)
	return data, nil
}

func (p *CatalogPolicy) readCache() ([]byte, bool) {
	b, err := os.ReadFile(p.cachePath)
	if err != nil || len(b) == 0 {
		return nil, false
	}
	return b, true
}

func (p *CatalogPolicy) freshCache() ([]byte, bool) {
	fi, err := os.Stat(p.cachePath)
	if err != nil {
		return nil, false
	}
	if time.Since(fi.ModTime()) > p.ttl {
		return nil, false
	}
	return p.readCache()
}

func (p *CatalogPolicy) fetch(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.liveURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) yolo")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("zen catalog: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (p *CatalogPolicy) writeAtomic(data []byte) error {
	if err := os.MkdirAll(filepath.Dir(p.cachePath), 0o755); err != nil {
		return err
	}
	tmp := p.cachePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p.cachePath)
}
