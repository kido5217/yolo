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
	if rules := protocol.ParsePerms(cfg.Permission); len(rules) != 1 || rules[0].Action != "ask" {
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
