package auth_test

import (
	"os"
	"testing"

	"github.com/kido5217/yolo/internal/auth"
	"github.com/kido5217/yolo/internal/protocol"
)

func TestEnvName(t *testing.T) {
	cases := []struct {
		provider, want string
	}{
		{"opencode", "OPENCODE_API_KEY"},
		{"kido", "KIDO_API_KEY"},
		{"myprov", "MYPROV_API_KEY"},
		{"my-prov", "MY_PROV_API_KEY"}, // non-alphanumerics map to _
		{"a.b", "A_B_API_KEY"},         // dots map to _ too
	}
	for _, tc := range cases {
		t.Run(tc.provider, func(t *testing.T) {
			if got := auth.EnvName(tc.provider); got != tc.want {
				t.Fatalf("EnvName(%q) = %q, want %q", tc.provider, got, tc.want)
			}
		})
	}
}

func TestResolutionOrder(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	env := map[string]string{}
	lookup := func(k string) (string, bool) { v, ok := env[k]; return v, ok }

	cfg := &protocol.Config{}
	// 1) env wins over all
	env["OPENCODE_API_KEY"] = "from-env"
	if k, _ := auth.ResolveKey("opencode", cfg, lookup); k != "from-env" {
		t.Fatalf("env should win: %q", k)
	}
	delete(env, "OPENCODE_API_KEY")

	// 2) auth.json wins over config
	s := auth.Store{}
	s.Set("opencode", "from-file")
	if err := auth.Save(s); err != nil {
		t.Fatal(err)
	}
	cfg.Provider = map[string]protocol.ProviderConfig{"opencode": {APIKey: "from-config"}}
	if k, _ := auth.ResolveKey("opencode", cfg, lookup); k != "from-file" {
		t.Fatalf("auth.json should beat config: %q", k)
	}

	// 3) config apiKey last
	s2, err := auth.Load()
	if err != nil {
		t.Fatal(err)
	}
	delete(s2, "opencode")
	if err := auth.Save(s2); err != nil {
		t.Fatal(err)
	}
	if k, _ := auth.ResolveKey("opencode", cfg, lookup); k != "from-config" {
		t.Fatalf("config should be last resort: %q", k)
	}
	if k, ok := auth.ResolveKey("kido", cfg, lookup); ok || k != "" {
		t.Fatalf("kido key-less: k=%q ok=%v", k, ok)
	}
}

func TestSaveIs0600(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	s := auth.Store{}
	s.Set("kido", "x")
	if err := auth.Save(s); err != nil {
		t.Fatal(err)
	}
	path, err := auth.Path()
	if err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 600", fi.Mode().Perm())
	}
}

func TestRoundTrip(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	s := auth.Store{}
	s.Set("opencode", "abc")
	if err := auth.Save(s); err != nil {
		t.Fatal(err)
	}
	got, err := auth.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got["opencode"].Key != "abc" || got["opencode"].Type != "api" {
		t.Fatalf("round trip: %+v", got)
	}
	s.Delete("opencode")
	if err := auth.Save(s); err != nil {
		t.Fatal(err)
	}
	got, err = auth.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := got["opencode"]; exists {
		t.Fatal("delete did not persist")
	}
}
