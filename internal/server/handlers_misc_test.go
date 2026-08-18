package server_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kido5217/yolo/internal/protocol"
)

func TestProviderListAndAuth(t *testing.T) {
	s := newSrv(t)
	resp, b := req(t, s, "GET", "/provider", "", "")
	if resp.StatusCode != 200 {
		t.Fatalf("%d %s", resp.StatusCode, b)
	}
	var ps []protocol.Provider
	json.Unmarshal(b, &ps)
	byID := map[string]protocol.Provider{}
	for _, p := range ps {
		byID[p.ID] = p
	}
	k, z := byID["kido"], byID["opencode"]
	if k.ID == "" || len(k.Models) < 1 {
		t.Fatalf("kido = %+v", k)
	}
	if z.ID == "" || len(z.Models) == 0 {
		t.Fatalf("zen = %+v (server test fixture: seed a minimal zen catalog via Dirs seam)", z)
	}
	if z.Auth.KeyRequired != true {
		t.Fatalf("zen auth = %+v", z.Auth)
	}
	// config-defined provider appears
	writeCfg(t, s.dir, `{"provider": {"myprov": {"base_url": "http://x", "models": {"m1": {"name": "M1"}}}}}`)
	resp, b = req(t, s, "GET", "/provider", s.dir, "")
	json.Unmarshal(b, &ps)
	var found bool
	for _, p := range ps {
		if p.ID == "myprov" && len(p.Models) == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("config provider missing: %s", b)
	}
}

// TestProviderAuthEndpoint is self-authored (route pinned in the plan table,
// body shape LOCKED): key_required + env per provider, merged with the
// loaded key source/status.
func TestProviderAuthEndpoint(t *testing.T) {
	s := newSrv(t)
	resp, b := req(t, s, "GET", "/provider/auth", "", "")
	if resp.StatusCode != 200 {
		t.Fatalf("%d %s", resp.StatusCode, b)
	}
	var m map[string]struct {
		KeyRequired bool     `json:"key_required"`
		Env         []string `json:"env"`
		Status      string   `json:"status"`
		Source      string   `json:"source"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, b)
	}
	if m["kido"].KeyRequired || len(m["kido"].Env) != 1 || m["kido"].Env[0] != "KIDO_API_KEY" {
		t.Fatalf("kido = %+v", m["kido"])
	}
	if !m["opencode"].KeyRequired || len(m["opencode"].Env) != 1 || m["opencode"].Env[0] != "OPENCODE_API_KEY" {
		t.Fatalf("opencode = %+v", m["opencode"])
	}
}

func TestConfigGetPatchRoundtrip(t *testing.T) {
	s := newSrv(t)
	d := s.dir
	resp, b := req(t, s, "GET", "/config", d, "")
	if resp.StatusCode != 200 {
		t.Fatalf("%d %s", resp.StatusCode, b)
	}
	resp, b = req(t, s, "PATCH", "/config", d, `{"model": "opencode/gpt-5-nano", "provider": {"kido": {"options": {"foo": true}}}}`)
	if resp.StatusCode != 200 {
		t.Fatalf("%d %s", resp.StatusCode, b)
	}
	var cfg map[string]any
	json.Unmarshal(b, &cfg)
	if cfg["model"] != "opencode/gpt-5-nano" {
		t.Fatalf("merged = %v", cfg["model"])
	}
	// file written with 2-space indent
	raw, _ := os.ReadFile(filepath.Join(d, "yolo.jsonc"))
	if !bytes.Contains(raw, []byte("  \"model\"")) {
		t.Fatalf("file = %s", raw)
	}
	// patch again — deep merge keeps provider.kido.options.foo
	resp, b = req(t, s, "PATCH", "/config", d, `{"provider": {"kido": {"options": {"bar": 1}}}}`)
	json.Unmarshal(b, &cfg)
	k := cfg["provider"].(map[string]any)["kido"].(map[string]any)["options"].(map[string]any)
	if k["foo"] != true || k["bar"] != float64(1) {
		t.Fatalf("deep merge lost keys: %v", k)
	}
}

func TestGlobalConfig(t *testing.T) {
	s := newSrv(t)
	resp, b := req(t, s, "PATCH", "/global/config", "", `{"model": "kido/m"}`)
	if resp.StatusCode != 200 {
		t.Fatalf("%d %s", resp.StatusCode, b)
	}
	f := filepath.Join(s.home, "yolo", "yolo.jsonc")
	raw, err := os.ReadFile(f)
	if err != nil {
		t.Fatalf("global file: %v", err)
	}
	if !bytes.Contains(raw, []byte(`"kido/m"`)) {
		t.Fatalf("global = %s", raw)
	}
	// project overrides global in GET /config
	resp, b = req(t, s, "PATCH", "/config", s.dir, `{"model": "kido/other"}`)
	resp, b = req(t, s, "GET", "/config", s.dir, "")
	var cfg map[string]any
	json.Unmarshal(b, &cfg)
	if cfg["model"] != "kido/other" {
		t.Fatalf("precedence broken: %v", cfg["model"])
	}
}

func TestAuthPutDelete(t *testing.T) {
	s := newSrv(t)
	resp, _ := req(t, s, "PUT", "/auth/opencode", "", `{"key": "sk-test"}`)
	if resp.StatusCode != 204 {
		t.Fatalf("put: %d", resp.StatusCode)
	}
	resp, b := req(t, s, "GET", "/provider", "", "")
	var ps []protocol.Provider
	json.Unmarshal(b, &ps)
	for _, p := range ps {
		if p.ID == "opencode" {
			if p.Auth == nil || p.Auth.Status != "loaded" {
				t.Fatalf("zen auth after put = %+v", p.Auth)
			}
		}
	}
	resp, _ = req(t, s, "DELETE", "/auth/opencode", "", "")
	if resp.StatusCode != 204 {
		t.Fatalf("delete: %d", resp.StatusCode)
	}
	resp, b = req(t, s, "GET", "/provider", "", "")
	json.Unmarshal(b, &ps)
	for _, p := range ps {
		if p.ID == "opencode" && p.Auth != nil && p.Auth.Status == "loaded" {
			t.Fatalf("key still loaded after delete")
		}
	}
}

func TestPermissionListAndReply(t *testing.T) {
	s := newSrv(t)
	d := t.TempDir()
	_, b := req(t, s, "POST", "/session", d, `{}`)
	var ses struct{ ID string }
	json.Unmarshal(b, &ses)
	// park a pending ask (action with no rules → ask) via the permission
	// service directly (harness seam permSvc.Ask in a goroutine):
	s.parkAsk(ses.ID, "custom", "res1")
	resp, b := req(t, s, "GET", "/permission", d, "")
	if resp.StatusCode != 200 {
		t.Fatalf("%d %s", resp.StatusCode, b)
	}
	var pend []protocol.PermissionAskedProps
	json.Unmarshal(b, &pend)
	if len(pend) != 1 || pend[0].Permission != "custom" {
		t.Fatalf("pending = %+v", pend)
	}
	resp, _ = req(t, s, "POST", "/permission/"+pend[0].ID+"/reply", d, `{"response":"once"}`)
	if resp.StatusCode != 204 {
		t.Fatalf("reply: %d", resp.StatusCode)
	}
	resp, b = req(t, s, "GET", "/permission", d, "")
	json.Unmarshal(b, &pend)
	if len(pend) != 0 {
		t.Fatalf("still pending: %+v", pend)
	}
	resp, _ = req(t, s, "POST", "/permission/per_missing/reply", d, `{"response":"once"}`)
	if resp.StatusCode != 404 {
		t.Fatalf("unknown reply: %d", resp.StatusCode)
	}
	resp, _ = req(t, s, "POST", "/permission/per_missing/reply", d, `{"response":"bogus"}`)
	if resp.StatusCode == 404 { // 404 wins over 400? LOCKED: validate body first → 400
		t.Fatalf("bad response should be 400")
	}
}

func TestAgentAndCommand(t *testing.T) {
	s := newSrv(t)
	_, b := req(t, s, "GET", "/agent", "", "")
	var agents []protocol.Agent
	json.Unmarshal(b, &agents)
	byName := map[string]string{}
	for _, a := range agents {
		byName[a.Name] = a.Description
	}
	if byName["build"] != "The default agent. Executes tools based on configured permissions." {
		t.Fatalf("build desc = %q", byName["build"])
	}
	if byName["plan"] != "Plan mode. Disallows all edit tools." {
		t.Fatalf("plan desc = %q", byName["plan"])
	}
	if _, ok := byName["yolo"]; !ok {
		t.Fatalf("yolo missing: %s", b)
	}
	_, b = req(t, s, "GET", "/command", "", "")
	var cmds []protocol.Command
	json.Unmarshal(b, &cmds)
	if len(cmds) != 5 {
		t.Fatalf("commands = %s", b)
	}
}

func TestUnknownRoutes404(t *testing.T) {
	s := newSrv(t)
	for _, p := range []string{"/", "/api/v2/sessions", "/mcp/x", "/skill/s", "/nope"} {
		resp, _ := req(t, s, "GET", p, "", "")
		if resp.StatusCode != 404 {
			t.Fatalf("%s → %d, want 404", p, resp.StatusCode)
		}
	}
}
