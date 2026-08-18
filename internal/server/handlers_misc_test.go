package server_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/server/testutil"
)

func TestProviderListAndAuth(t *testing.T) {
	s := testutil.Boot(t)
	resp, b := testutil.Req(t, s, "GET", "/provider", "", "")
	if resp.StatusCode != 200 {
		t.Fatalf("%d %s", resp.StatusCode, b)
	}
	var ps []protocol.Provider
	if err := json.Unmarshal(b, &ps); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, b)
	}
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
	testutil.WriteCfg(t, s.Dir, `{"provider": {"myprov": {"base_url": "http://x", "models": {"m1": {"name": "M1"}}}}}`)
	_, b = testutil.Req(t, s, "GET", "/provider", s.Dir, "")
	if err := json.Unmarshal(b, &ps); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, b)
	}
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
	s := testutil.Boot(t)
	resp, b := testutil.Req(t, s, "GET", "/provider/auth", "", "")
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
	s := testutil.Boot(t)
	d := s.Dir
	resp, b := testutil.Req(t, s, "GET", "/config", d, "")
	if resp.StatusCode != 200 {
		t.Fatalf("%d %s", resp.StatusCode, b)
	}
	resp, b = testutil.Req(t, s, "PATCH", "/config", d, `{"model": "opencode/gpt-5-nano", "provider": {"kido": {"options": {"foo": true}}}}`)
	if resp.StatusCode != 200 {
		t.Fatalf("%d %s", resp.StatusCode, b)
	}
	var cfg map[string]any
	if err := json.Unmarshal(b, &cfg); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, b)
	}
	if cfg["model"] != "opencode/gpt-5-nano" {
		t.Fatalf("merged = %v", cfg["model"])
	}
	// file written with 2-space indent
	raw, _ := os.ReadFile(filepath.Join(d, "yolo.jsonc"))
	if !bytes.Contains(raw, []byte("  \"model\"")) {
		t.Fatalf("file = %s", raw)
	}
	// patch again — deep merge keeps provider.kido.options.foo
	_, b = testutil.Req(t, s, "PATCH", "/config", d, `{"provider": {"kido": {"options": {"bar": 1}}}}`)
	if err := json.Unmarshal(b, &cfg); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, b)
	}
	k := cfg["provider"].(map[string]any)["kido"].(map[string]any)["options"].(map[string]any)
	if k["foo"] != true || k["bar"] != float64(1) {
		t.Fatalf("deep merge lost keys: %v", k)
	}
}

func TestGlobalConfig(t *testing.T) {
	s := testutil.Boot(t)
	resp, b := testutil.Req(t, s, "PATCH", "/global/config", "", `{"model": "kido/m"}`)
	if resp.StatusCode != 200 {
		t.Fatalf("%d %s", resp.StatusCode, b)
	}
	f := filepath.Join(s.Home, "yolo", "yolo.jsonc")
	raw, err := os.ReadFile(f)
	if err != nil {
		t.Fatalf("global file: %v", err)
	}
	if !bytes.Contains(raw, []byte(`"kido/m"`)) {
		t.Fatalf("global = %s", raw)
	}
	// project overrides global in GET /config
	if pr, _ := testutil.Req(t, s, "PATCH", "/config", s.Dir, `{"model": "kido/other"}`); pr.StatusCode != 200 {
		t.Fatalf("patch status = %d", pr.StatusCode)
	}
	_, b = testutil.Req(t, s, "GET", "/config", s.Dir, "")
	var cfg map[string]any
	if err := json.Unmarshal(b, &cfg); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, b)
	}
	if cfg["model"] != "kido/other" {
		t.Fatalf("precedence broken: %v", cfg["model"])
	}
}

func TestAuthPutDelete(t *testing.T) {
	s := testutil.Boot(t)
	resp, _ := testutil.Req(t, s, "PUT", "/auth/opencode", "", `{"key": "sk-test"}`)
	if resp.StatusCode != 204 {
		t.Fatalf("put: %d", resp.StatusCode)
	}
	_, b := testutil.Req(t, s, "GET", "/provider", "", "")
	var ps []protocol.Provider
	if err := json.Unmarshal(b, &ps); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, b)
	}
	for _, p := range ps {
		if p.ID == "opencode" {
			if p.Auth == nil || p.Auth.Status != "loaded" {
				t.Fatalf("zen auth after put = %+v", p.Auth)
			}
		}
	}
	resp, _ = testutil.Req(t, s, "DELETE", "/auth/opencode", "", "")
	if resp.StatusCode != 204 {
		t.Fatalf("delete: %d", resp.StatusCode)
	}
	_, b = testutil.Req(t, s, "GET", "/provider", "", "")
	if err := json.Unmarshal(b, &ps); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, b)
	}
	for _, p := range ps {
		if p.ID == "opencode" && p.Auth != nil && p.Auth.Status == "loaded" {
			t.Fatalf("key still loaded after delete")
		}
	}
}

func TestPermissionListAndReply(t *testing.T) {
	s := testutil.Boot(t)
	d := t.TempDir()
	_, b := testutil.Req(t, s, "POST", "/session", d, `{}`)
	var ses struct{ ID string }
	if err := json.Unmarshal(b, &ses); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, b)
	}
	// park a pending ask (action with no rules → ask) via the permission
	// service directly (harness seam permSvc.Ask in a goroutine):
	s.ParkAsk(ses.ID, "custom", "res1")
	resp, b := testutil.Req(t, s, "GET", "/permission", d, "")
	if resp.StatusCode != 200 {
		t.Fatalf("%d %s", resp.StatusCode, b)
	}
	var pend []protocol.PermissionAskedProps
	if err := json.Unmarshal(b, &pend); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, b)
	}
	if len(pend) != 1 || pend[0].Permission != "custom" {
		t.Fatalf("pending = %+v", pend)
	}
	resp, _ = testutil.Req(t, s, "POST", "/permission/"+pend[0].ID+"/reply", d, `{"response":"once"}`)
	if resp.StatusCode != 204 {
		t.Fatalf("reply: %d", resp.StatusCode)
	}
	_, b = testutil.Req(t, s, "GET", "/permission", d, "")
	if err := json.Unmarshal(b, &pend); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, b)
	}
	if len(pend) != 0 {
		t.Fatalf("still pending: %+v", pend)
	}
	resp, _ = testutil.Req(t, s, "POST", "/permission/per_missing/reply", d, `{"response":"once"}`)
	if resp.StatusCode != 404 {
		t.Fatalf("unknown reply: %d", resp.StatusCode)
	}
	resp, _ = testutil.Req(t, s, "POST", "/permission/per_missing/reply", d, `{"response":"bogus"}`)
	if resp.StatusCode == 404 { // 404 wins over 400? LOCKED: validate body first → 400
		t.Fatalf("bad response should be 400")
	}
}

func TestAgentAndCommand(t *testing.T) {
	s := testutil.Boot(t)
	_, b := testutil.Req(t, s, "GET", "/agent", "", "")
	var agents []protocol.Agent
	if err := json.Unmarshal(b, &agents); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, b)
	}
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
	_, b = testutil.Req(t, s, "GET", "/command", "", "")
	var cmds []protocol.Command
	if err := json.Unmarshal(b, &cmds); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, b)
	}
	if len(cmds) != 5 {
		t.Fatalf("commands = %s", b)
	}
}

func TestUnknownRoutes404(t *testing.T) {
	s := testutil.Boot(t)
	for _, p := range []string{"/", "/api/v2/sessions", "/mcp/x", "/skill/s", "/nope"} {
		resp, _ := testutil.Req(t, s, "GET", p, "", "")
		if resp.StatusCode != 404 {
			t.Fatalf("%s → %d, want 404", p, resp.StatusCode)
		}
	}
}
