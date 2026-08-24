package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kido5217/yolo/internal/config"
	"github.com/kido5217/yolo/internal/protocol"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGlobalProjectYoloDiscoveryAndMerge(t *testing.T) {
	global := t.TempDir()
	work := t.TempDir()

	// fake global config dir
	write(t, filepath.Join(global, "config.json"), `{"model":"opencode/gpt-5-nano","provider":{"opencode":{"apiKey":"{env:MY_KEY}"}}}`)
	write(t, filepath.Join(global, "yolo.jsonc"), `// comment
{"instructions":["/docs/A.md"], "theme":{"dark":true}}`)

	root := filepath.Join(work, "repo")
	mid := filepath.Join(root, "mid")
	write(t, filepath.Join(root, "yolo.jsonc"), `{"model":"kido/Qwen3.8-27B","instructions":["/docs/A.md","/docs/B.md"],"permission":{"bash":"ask"}}`)
	write(t, filepath.Join(mid, "yolo.json"), `{"agent":"plan"}`)
	write(t, filepath.Join(mid, ".yolo", "yolo.jsonc"), `{"instructions":["/docs/C.md"],"tool_output":{"max_lines":500}}`)

	l := config.Loader{Env: map[string]string{"MY_KEY": "sekret"}}
	cfg, err := l.LoadAt(global, mid)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Model != "kido/Qwen3.8-27B" {
		t.Fatalf("project model should win: %s", cfg.Model)
	}
	if cfg.Agent != "plan" {
		t.Fatalf("innermost project wins agent: %s", cfg.Agent)
	}
	// deep merge: provider.opencode.apiKey kept from global (env-substituted)
	if got := cfg.Provider["opencode"].APIKey; got != "sekret" {
		t.Fatalf("env substitution + deep merge failed: %q", got)
	}
	// instructions concatenate + dedupe: global[A] < project[A,B] < .yolo[C]
	wantInst := []string{"/docs/A.md", "/docs/B.md", "/docs/C.md"}
	seen := map[string]bool{}
	for _, s := range cfg.Instructions {
		if seen[s] {
			t.Fatalf("dup instruction %s in %v", s, cfg.Instructions)
		}
		seen[s] = true
	}
	for i, w := range wantInst {
		if i >= len(cfg.Instructions) || cfg.Instructions[i] != w {
			t.Fatalf("instructions = %v, want %v", cfg.Instructions, wantInst)
		}
	}
	if cfg.Permission["bash"] == nil {
		t.Fatal("permission map lost in merge")
	}
	if cfg.ToolOutput == nil || cfg.ToolOutput.MaxLines != 500 {
		t.Fatalf("tool_output not merged: %+v", cfg.ToolOutput)
	}
	if cfg.Theme == nil {
		t.Fatal("theme lost")
	}
	rules, err := protocol.ParsePerms(cfg.Permission)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 || rules[0].Action != "ask" {
		t.Fatalf("perms: %+v", rules)
	}
}

func TestJSONCCommentsAndUnknownFieldsIgnored(t *testing.T) {
	work := t.TempDir()
	write(t, filepath.Join(work, "yolo.jsonc"), `
{
  // leading comment
  "model": "kido/Qwen3.8-27B", /* inline */
  "futureField": {"x": 1},
  "instructions": ["a.md"] // trailing
}
`)
	cfg, err := config.Loader{Env: nil}.LoadAt(t.TempDir(), work)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Model != "kido/Qwen3.8-27B" {
		t.Fatalf("model = %q", cfg.Model)
	}
}

func TestNoConfigFilesIsValid(t *testing.T) {
	cfg, err := config.Loader{Env: nil}.LoadAt(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil || cfg.Model != "" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestMalformedJSONIsError(t *testing.T) {
	work := t.TempDir()
	write(t, filepath.Join(work, "yolo.json"), `{broken`)
	_, err := config.Loader{Env: nil}.LoadAt(t.TempDir(), work)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

// seedProfileRoot builds a <configHome>/yolo profile root: one dir per
// model value plus an optional legacy flat file, with the given marker.
func seedProfileRoot(t *testing.T, configHome, marker string, profiles map[string]string) {
	t.Helper()
	for id, model := range profiles {
		write(t, filepath.Join(configHome, "yolo", id, "yolo.jsonc"), `{"model":"`+model+`"}`)
	}
	if marker != "" {
		write(t, filepath.Join(configHome, "yolo", "active"), marker+"\n")
	}
}

func TestLoaderLoadProfileResolution(t *testing.T) {
	work := t.TempDir()

	t.Run("active marker selects the profile dir, legacy flat files are ignored", func(t *testing.T) {
		home := t.TempDir()
		seedProfileRoot(t, home, "work", map[string]string{
			"default": "m-default",
			"work":    "m-work",
		})
		write(t, filepath.Join(home, "yolo", "yolo.jsonc"), `{"model":"m-legacy"}`)
		t.Setenv("XDG_CONFIG_HOME", home)
		cfg, err := config.Loader{}.Load(work)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Model != "m-work" {
			t.Fatalf("model = %q, want m-work (marker profile; legacy must be ignored)", cfg.Model)
		}
	})

	t.Run("YOLO_PROFILE env overrides the marker", func(t *testing.T) {
		home := t.TempDir()
		seedProfileRoot(t, home, "work", map[string]string{
			"work":  "m-work",
			"other": "m-other",
		})
		t.Setenv("XDG_CONFIG_HOME", home)
		cfg, err := config.Loader{Env: map[string]string{"YOLO_PROFILE": "other"}}.Load(work)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Model != "m-other" {
			t.Fatalf("model = %q, want m-other (env override)", cfg.Model)
		}
	})

	t.Run("Loader.Profile override beats env and marker", func(t *testing.T) {
		home := t.TempDir()
		seedProfileRoot(t, home, "work", map[string]string{
			"work":  "m-work",
			"third": "m-third",
		})
		t.Setenv("XDG_CONFIG_HOME", home)
		l := config.Loader{Profile: "third", Env: map[string]string{"YOLO_PROFILE": "work"}}
		cfg, err := l.Load(work)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Model != "m-third" {
			t.Fatalf("model = %q, want m-third (flag override)", cfg.Model)
		}
	})

	t.Run("first run creates the default profile and loads it", func(t *testing.T) {
		home := t.TempDir() // no yolo dir yet
		t.Setenv("XDG_CONFIG_HOME", home)
		cfg, err := config.Loader{}.Load(work)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Model != "" {
			t.Fatalf("model = %q, want empty (fresh default profile)", cfg.Model)
		}
		if _, err := os.Stat(filepath.Join(home, "yolo", "default")); err != nil {
			t.Fatalf("first run must create the default profile: %v", err)
		}
	})

	t.Run("override naming a missing profile is an error", func(t *testing.T) {
		home := t.TempDir()
		seedProfileRoot(t, home, "default", map[string]string{"default": "m-default"})
		t.Setenv("XDG_CONFIG_HOME", home)
		l := config.Loader{Profile: "nope"}
		if _, err := l.Load(work); err == nil {
			t.Fatal("Load with missing override: want error, got nil")
		}
		l = config.Loader{Env: map[string]string{"YOLO_PROFILE": "nope"}}
		if _, err := l.Load(work); err == nil {
			t.Fatal("Load with missing env profile: want error, got nil")
		}
	})
}
