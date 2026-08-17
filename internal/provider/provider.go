// Package provider builds the model/provider catalog: kido (live probe with
// static fallback), the opencode zen catalog (cached, filtered), and
// user-defined config providers.
package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/kido5217/yolo/internal/auth"
	"github.com/kido5217/yolo/internal/config"
	"github.com/kido5217/yolo/internal/llm"
	"github.com/kido5217/yolo/internal/protocol"
)

// Model is one catalog entry.
type Model struct {
	ID, Name, Family, Adapter                      string // Adapter: "openai" | "anthropic"
	ToolCall, Reasoning                            bool
	Attachment                                     bool
	Context, Output                                int
	CostIn, CostOut, CostCacheRead, CostCacheWrite float64 // USD per 1M
}

// Info is one provider's catalog state (maps to protocol.Provider in List).
type Info struct {
	ID, Name, Source, BaseURL string
	KeyRequired, KeyLoaded    bool
	Env                       []string
	Models                    []Model
}

// Registry is the resolved provider catalog.
type Registry struct {
	info        []Info
	defProvider string
	defModel    string
	client      *http.Client
}

// Dirs carries catalog locations; zero fields fall back to production.
type Dirs struct {
	Home       string
	KidoBase   string // production https://ai.kido.ws/v1
	ZenBase    string // production https://opencode.ai/zen/v1
	ZenCatalog string // production https://models.opencode.ai/api.json
	ZenCache   string // production <config.CacheYoloDir>/models.json
}

func DirsDefaults() Dirs {
	return Dirs{
		Home:       config.Home(),
		KidoBase:   "https://ai.kido.ws/v1",
		ZenBase:    "https://opencode.ai/zen/v1",
		ZenCatalog: "https://models.opencode.ai/api.json",
		ZenCache:   config.CacheYoloDir() + "/models.json",
	}
}

// OverridableDirs fills empty fields with production defaults when fill is
// true; when false the production defaults are returned verbatim.
func OverridableDirs(d Dirs, fill bool) Dirs {
	prod := DirsDefaults()
	if !fill {
		return prod
	}
	if d.Home == "" {
		d.Home = prod.Home
	}
	if d.KidoBase == "" {
		d.KidoBase = prod.KidoBase
	}
	if d.ZenBase == "" {
		d.ZenBase = prod.ZenBase
	}
	if d.ZenCatalog == "" {
		d.ZenCatalog = prod.ZenCatalog
	}
	if d.ZenCache == "" {
		d.ZenCache = prod.ZenCache
	}
	return d
}

// New builds the registry: kido (live/fallback), zen (cached catalog), and
// config-defined providers.
func New(ctx context.Context, cfg *protocol.Config, httpc *http.Client, homeDirs Dirs) (*Registry, error) {
	if httpc == nil {
		httpc = http.DefaultClient
	}
	dirs := OverridableDirs(homeDirs, true)
	if cfg == nil {
		cfg = &protocol.Config{}
	}
	r := &Registry{client: httpc, defProvider: "kido", defModel: "Qwen3.8-27B"}

	kidoModels, err := FetchKido(ctx, dirs.KidoBase, 5000, false)
	if err != nil {
		return nil, err
	}
	kido := Info{ID: "kido", Name: "Kido", Source: "builtin", BaseURL: dirs.KidoBase,
		KeyRequired: false, KeyLoaded: true, Models: kidoModels}
	if oc, ok := cfg.Provider["kido"]; ok && oc.BaseURL != "" {
		kido.BaseURL = oc.BaseURL
	}
	r.info = append(r.info, kido)

	if raw, lerr := NewCatalogPolicy(dirs.ZenCache, 5, dirs.ZenCatalog).Load(ctx); lerr == nil {
		models, perr := ParseZenCatalog(raw)
		if perr == nil {
			meta := zenMeta(raw)
			zen := Info{ID: "opencode", Name: meta.Name, Source: "builtin",
				BaseURL: meta.API, KeyRequired: true, Env: meta.Env, Models: models}
			if oc, ok := cfg.Provider["opencode"]; ok && oc.BaseURL != "" {
				zen.BaseURL = oc.BaseURL
			}
			r.info = append(r.info, zen)
		}
	}

	if cfg.Provider != nil {
		ids := []string{}
		for id := range cfg.Provider {
			if id == "kido" || id == "opencode" {
				continue
			}
			if oc := cfg.Provider[id]; oc.BaseURL != "" && len(oc.Models) > 0 {
				ids = append(ids, id)
			}
		}
		sort.Strings(ids)
		for _, id := range ids {
			r.info = append(r.info, configProviderInfo(id, cfg.Provider[id]))
		}
	}

	for i := range r.info {
		if k, ok := auth.ResolveKey(r.info[i].ID, cfg, os.LookupEnv); ok && k != "" {
			r.info[i].KeyLoaded = true
		}
	}

	if cfg.Model != "" {
		if id, mid, ok := splitRef(cfg.Model); ok {
			r.defProvider, r.defModel = id, mid
		}
	}
	return r, nil
}

func configProviderInfo(id string, oc protocol.ProviderConfig) Info {
	info := Info{ID: id, Name: id, Source: "config", BaseURL: oc.BaseURL, KeyRequired: true}
	mids := make([]string, 0, len(oc.Models))
	for mid := range oc.Models {
		mids = append(mids, mid)
	}
	sort.Strings(mids)
	for _, mid := range mids {
		info.Models = append(info.Models, cfgModel(mid, oc.Models[mid]))
	}
	return info
}

func cfgModel(mid string, mv any) Model {
	m, ok := mv.(map[string]any)
	if !ok {
		m = map[string]any{}
	}
	out := Model{ID: mid, Name: mid, Context: 32768, Adapter: "openai"}
	if s, ok := m["name"].(string); ok && s != "" {
		out.Name = s
	}
	if a, ok := m["adapter"].(string); ok && (a == "openai" || a == "anthropic") {
		out.Adapter = a
	}
	if lim, ok := m["limit"].(map[string]any); ok {
		if c, ok := lim["context"].(float64); ok {
			out.Context = int(c)
		}
		if c, ok := lim["output"].(float64); ok {
			out.Output = int(c)
		}
	}
	if cst, ok := m["cost"].(map[string]any); ok {
		out.CostIn = mfloat(cst, "input")
		out.CostOut = mfloat(cst, "output")
		out.CostCacheRead = mfloat(cst, "cache_read")
		out.CostCacheWrite = mfloat(cst, "cache_write")
	}
	return out
}

func mfloat(m map[string]any, k string) float64 {
	v, _ := m[k].(float64)
	return v
}

// List maps the catalog to wire providers.
func (r *Registry) List() []protocol.Provider {
	out := make([]protocol.Provider, 0, len(r.info))
	for _, i := range r.info {
		p := protocol.Provider{
			ID: i.ID, Name: i.Name, Source: i.Source,
			Env: i.Env, Options: map[string]any{},
			Models: map[string]protocol.Model{},
		}
		if p.Env == nil {
			p.Env = []string{}
		}
		for _, m := range i.Models {
			p.Models[m.ID] = protocol.Model{
				ID: m.ID, ProviderID: i.ID, Name: m.Name, Family: m.Family,
				ToolCall: m.ToolCall, Reasoning: m.Reasoning, Attachment: m.Attachment,
				Limit:   protocol.ModelLimit{Context: m.Context, Output: m.Output},
				Cost:    protocol.ModelCost{Input: m.CostIn, Output: m.CostOut, CacheRead: m.CostCacheRead, CacheWrite: m.CostCacheWrite},
				Adapter: m.Adapter,
			}
		}
		p.Auth = &protocol.ProviderAuth{
			Type: "api", KeyRequired: i.KeyRequired,
			Status: authStatus(i),
		}
		out = append(out, p)
	}
	return out
}

func authStatus(i Info) string {
	switch {
	case !i.KeyRequired:
		return "not-required"
	case i.KeyLoaded:
		return "loaded"
	default:
		return "missing"
	}
}

// Resolve maps a "provider/model" reference; empty uses the defaults.
func (r *Registry) Resolve(ref string) (Info, Model, error) {
	if ref == "" {
		ref = r.defProvider + "/" + r.defModel
	}
	pid, mid, ok := splitRef(ref)
	if !ok {
		return Info{}, Model{}, fmt.Errorf("bad model ref %q", ref)
	}
	for _, i := range r.info {
		if i.ID != pid {
			continue
		}
		for _, m := range i.Models {
			if m.ID == mid {
				return i, m, nil
			}
		}
		return i, Model{}, fmt.Errorf("unknown model %q in provider %s", mid, pid)
	}
	return Info{}, Model{}, fmt.Errorf("unknown provider %q", pid)
}

// DriverFor picks the llm driver by the model's adapter.
func (r *Registry) DriverFor(m Model) llm.Driver {
	if m.Adapter == "anthropic" {
		return llm.NewAnthropic(r.client)
	}
	return llm.NewOpenAI(r.client)
}

// Default returns the default provider/model.
func (r *Registry) Default() (string, string) {
	return r.defProvider, r.defModel
}

func splitRef(ref string) (string, string, bool) {
	i := strings.IndexByte(ref, '/')
	if i <= 0 || i == len(ref)-1 {
		return "", "", false
	}
	return ref[:i], ref[i+1:], true
}
