package provider_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/provider"
)

func dirs(t *testing.T) provider.Dirs {
	t.Helper()
	d := t.TempDir()
	return provider.Dirs{
		KidoBase:   "http://127.0.0.1:0", // replaced per test
		ZenBase:    "https://opencode.ai/zen/v1",
		ZenCatalog: "http://127.0.0.1:0", // replaced per test
		ZenCache:   filepath.Join(d, "models.json"),
		Home:       d,
	}
}

func TestKidoParsesLlamacpp(t *testing.T) {
	t.Parallel()
	raw, _ := os.ReadFile("testdata/kido-models.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = w.Write(raw)
	}))
	defer srv.Close()
	m := provider.FetchKido(context.Background(), srv.URL+"/v1", 5, false, nil)
	if len(m) != 1 {
		t.Fatalf("models = %d", len(m))
	}
	q := m[0]
	if q.ID != "Qwen3.8-27B" || q.Context != 262144 || !q.ToolCall || !q.Reasoning || q.Adapter != "openai" {
		t.Fatalf("model = %+v", q)
	}
}

func TestKidoFallsBackStaticOnNetworkError(t *testing.T) {
	t.Parallel()
	// Network failure falls back to the static list without an error
	// (FetchKido never reports probe problems).
	m := provider.FetchKido(context.Background(), "http://127.0.0.1:1", 200, false, nil)
	if len(m) != 1 || m[0].ID != "Qwen3.8-27B" || m[0].Context != 262144 {
		t.Fatalf("fallback model = %+v", m)
	}
}

func TestKidoSkipsInvalidEntries(t *testing.T) {
	t.Parallel()
	t.Run("skips invalid entries", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"data":[
				{"id":"","meta":{"n_ctx":4096}},
				{"id":"broken","meta":{"n_ctx":0}},
				{"id":"good","meta":{"n_ctx":8192}}
			]}`))
		}))
		defer srv.Close()
		m := provider.FetchKido(context.Background(), srv.URL, 5000, false, nil)
		if len(m) != 1 || m[0].ID != "good" || m[0].Context != 8192 || m[0].Output != 1024 {
			t.Fatalf("models = %+v", m)
		}
	})
	t.Run("all-invalid falls back to static", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"data":[{"id":"","meta":{"n_ctx":0}}]}`))
		}))
		defer srv.Close()
		m := provider.FetchKido(context.Background(), srv.URL, 5000, false, nil)
		if len(m) != 1 || m[0].ID != "Qwen3.8-27B" {
			t.Fatalf("fallback = %+v", m)
		}
	})
}

func TestZenFiltersAndAdapterMap(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("testdata/zen-opencode.json")
	if err != nil {
		t.Fatal(err)
	}
	var cat map[string]any
	if err := json.Unmarshal(raw, &cat); err != nil {
		t.Fatal(err)
	}
	// counts in the fixture (frozen 2026-08-17): 91 models, 64 paid, 57 kept, 7 google
	models := cat["opencode"].(map[string]any)["models"].(map[string]any)
	if len(models) != 91 {
		t.Fatalf("fixture models = %d, want 91", len(models))
	}
	kept, err := provider.ParseZenCatalog(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != 57 {
		t.Fatalf("kept = %d, want 57", len(kept))
	}
	openaiN, anthropicN := 0, 0
	for _, m := range kept {
		switch m.Adapter {
		case "openai":
			openaiN++
		case "anthropic":
			anthropicN++
		default:
			t.Fatalf("bad adapter %q for %s", m.Adapter, m.ID)
		}
	}
	if openaiN != 42 || anthropicN != 15 {
		t.Fatalf("openai=%d anthropic=%d, want 42/15", openaiN, anthropicN)
	}
	// spot checks
	byID := map[string]provider.Model{}
	for _, m := range kept {
		byID[m.ID] = m
	}
	if byID["claude-opus-4-7"].Adapter != "anthropic" || byID["claude-opus-4-7"].Context != 1000000 {
		t.Fatalf("claude = %+v", byID["claude-opus-4-7"])
	}
	if byID["gpt-5-nano"].Adapter != "openai" || byID["gpt-5-nano"].Context != 400000 {
		t.Fatalf("gpt = %+v", byID["gpt-5-nano"])
	}
	if _, exists := byID["gemini-3-flash"]; exists {
		t.Fatal("google model not excluded")
	}
}

func TestZenCacheTTLAndAtomicRewrite(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	cache := filepath.Join(d, "models.json")
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		raw, _ := os.ReadFile("testdata/zen-opencode.json")
		_, _ = w.Write(raw)
	}))
	defer srv.Close()
	pol := provider.NewCatalogPolicy(cache, 5, srv.URL, nil)
	if _, err := pol.Load(context.Background()); err != nil { // miss -> fetch+write
		t.Fatal(err)
	}
	if hits != 1 {
		t.Fatalf("hits = %d", hits)
	}
	fi, err := os.Stat(cache)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() == 0 {
		t.Fatal("empty cache file")
	}
	if _, err := pol.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if hits != 1 {
		t.Fatalf("fresh cache re-fetched: hits = %d", hits)
	}
	// force stale (mtime -10 min)
	stale := fi.ModTime().Add(-10 * 60 * 1e9)
	_ = os.Chtimes(cache, stale, stale)
	if _, err := pol.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if hits != 2 {
		t.Fatalf("stale cache not refetched: hits = %d", hits)
	}
}

func TestRegistryListAndResolve(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir()) // isolate auth.json
	t.Setenv("OPENCODE_API_KEY", "")       // unset for key-resolution determinism
	kidoSrv := kidoServer(t)
	zenSrv := zenServer(t)
	d := dirs(t)
	d.KidoBase = kidoSrv.URL
	d.ZenCatalog = zenSrv.URL
	cfg := &protocol.Config{}
	odirs, err := provider.OverridableDirs(d, true) // true = use injected URLs
	if err != nil {
		t.Fatal(err)
	}
	reg, err := provider.New(context.Background(), cfg, http.DefaultClient, odirs)
	if err != nil {
		t.Fatal(err)
	}
	ps := reg.List()
	byID := map[string]protocol.Provider{}
	for _, p := range ps {
		byID[p.ID] = p
	}
	if _, ok := byID["kido"]; !ok {
		t.Fatal("kido provider missing")
	}
	z := byID["opencode"]
	if len(z.Models) != 57 {
		t.Fatalf("zen models = %d", len(z.Models))
	}
	if z.Auth == nil || z.Auth.KeyRequired != true || z.Auth.Status != "missing" {
		t.Fatalf("zen auth = %+v", z.Auth)
	}
	k := byID["kido"]
	if k.Auth == nil || k.Auth.KeyRequired != false || k.Auth.Status != "not-required" {
		t.Fatalf("kido auth = %+v", k.Auth)
	}
	info, model, err := reg.Resolve("kido/Qwen3.8-27B")
	if err != nil || model.ID != "Qwen3.8-27B" || info.ID != "kido" {
		t.Fatalf("resolve = %+v %+v %v", info, model, err)
	}
	if p, m, err := reg.Resolve(""); err != nil || p.ID != "kido" || m.ID != "Qwen3.8-27B" {
		t.Fatalf("default resolve = %s/%s %v", p.ID, m.ID, err)
	}
	if _, _, err := reg.Resolve("nope/nope"); err == nil {
		t.Fatal("want error for unknown provider")
	}
}

func kidoServer(t *testing.T) *httptest.Server {
	t.Helper()
	raw, _ := os.ReadFile("testdata/kido-models.json")
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(raw)
	}))
}
func zenServer(t *testing.T) *httptest.Server {
	t.Helper()
	raw, _ := os.ReadFile("testdata/zen-opencode.json")
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(raw)
	}))
}
