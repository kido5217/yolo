package session

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/provider"
)

// sha256 constants from `sha256sum internal/session/prompt/*.txt` (ALL 14).
var promptPins = map[string]string{
	"prompt/anthropic.txt":     "8324e4cf58eb45d4d9d6fd120f5e8da59e0548de48e7e6aefcdfbf2923f40b4e",
	"prompt/beast.txt":         "a384d7b485829c1fe43bd6deaae10466db2c16b8cba045764538974f737958ba",
	"prompt/build-switch.txt":  "5e3db616a685a3dfaaf95fb86ae6e2acfbdf520bda60f7b27f727d2a88ba8a25",
	"prompt/codex.txt":         "c30bca40693a47965e25ceac3f02d3709712af7abeab1278bba53a9efcffa928",
	"prompt/copilot-gpt-5.txt": "0ef5261daf7a4ae72b3e874cabc7b06cf34d991376d1dcfa1874734b13031828",
	"prompt/default.txt":       "962fbf3cb3ec659c9a5244425ee2e7bb141ad4428f489a630a7738566880dc6a",
	"prompt/gemini.txt":        "921750803b0314b88b8adc996e2afcf1a61fd7d9dd6dfcf812baeadac1468cf3",
	"prompt/gpt.txt":           "83a66a46a5febbc21454161d5f053638b22d25d95e09d77b8f6da33debc848ad",
	"prompt/kimi.txt":          "ade9199b00df5aa3b51bb02b8e8c711f3e0de224345aef7df9f31d3ea08a5bc7",
	"prompt/meta.txt":          "9068607ce8bbb3f9b09531d8114fc16e1724de96cd1e364565c9f6f6b2b61df3",
	"prompt/plan-mode.txt":     "473381e8f20d054fa24ed3631a3b741a4fd432dadb8a0f0925f73d94a6e2866c",
	"prompt/plan.txt":          "455db97e0d21e8097c2afb539d167b4b2483e99b585dbc4fff23cafd4a3029b8",
	"prompt/title.txt":         "e7a6848eba328f28c7e870874cf0591e4edbaf90d7602ad8fdfe90601c6e656f",
	"prompt/trinity.txt":       "0019dc1d018d08c1b5a065d10896e22d2615d33580088f44aefc3a042a46ebe2",
}

func sha256hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func mustReadBytes(t *testing.T, path string) []byte {
	t.Helper()
	b, err := prompts.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return b
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	return string(mustReadBytes(t, path))
}

// firstLine returns the file's first line, used as a distinctive selection
// fingerprint (each prompt file has a unique opening line).
func firstLine(t *testing.T, file string) string {
	t.Helper()
	b, err := prompts.ReadFile("prompt/" + file)
	if err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	return strings.SplitN(string(b), "\n", 2)[0]
}

func TestPromptFilesPinned(t *testing.T) {
	for name, want := range promptPins {
		if got := sha256hex(mustReadBytes(t, name)); got != want {
			t.Fatalf("%s sha = %s, want %s", name, got, want)
		}
	}
}

func TestFamilySelection(t *testing.T) {
	cases := []struct {
		api, prov, want string
	}{
		{"Qwen3.8-27B", "kido", "default.txt"},
		{"claude-opus-4-7", "opencode", "anthropic.txt"},
		{"gpt-5-nano", "opencode", "gpt.txt"},
		{"gpt-codex-1", "openai", "codex.txt"},
		{"gpt-4.1", "opencode", "beast.txt"},
		{"o3-mini", "opencode", "beast.txt"},
		{"gemini-3-flash", "opencode", "gemini.txt"},
		{"kimi-k2", "moonshotai", "kimi.txt"},
		{"some-model", "kimi-for-coding", "kimi.txt"},
		{"trinity-x", "x", "trinity.txt"},
		{"muse-glimmer-9b", "openrouter", "meta.txt"},
	}
	for _, c := range cases {
		name, text, err := FamilyPrompt(c.api, c.prov)
		if err != nil {
			t.Fatalf("%s/%s: %v", c.api, c.prov, err)
		}
		want := "prompt/" + c.want
		if name != want {
			t.Fatalf("%s/%s family = %s, want %s", c.api, c.prov, name, want)
		}
		if c.want == "meta.txt" {
			if !strings.Contains(text, "Muse Glimmer") {
				t.Fatalf("muse-glimmer substitution missing: %q", text)
			}
			continue
		}
		if !strings.Contains(text, firstLine(t, c.want)) {
			t.Fatalf("%s/%s did not select %s", c.api, c.prov, c.want)
		}
	}
	// FamilyName is the name-only accessor.
	if got := FamilyName("o3-mini", "opencode"); got != "prompt/beast.txt" {
		t.Fatalf("FamilyName = %s", got)
	}
}

func TestEnvBlock(t *testing.T) {
	d := t.TempDir() // not a git repo → "no"
	got := EnvBlock(d, "Qwen3.8-27B", "kido")
	wantPrefix := "You are powered by the model named Qwen3.8-27B. The exact model ID is kido/Qwen3.8-27B\n" +
		"Here is some useful information about the environment you are running in:\n" +
		"<env>\n" +
		"  Working directory: " + d + "\n" +
		"  Workspace root folder: " + d + "\n" +
		"  Is directory a git repo: no\n" +
		"  Platform: linux\n" +
		"  Today's date: "
	if !strings.HasPrefix(got, wantPrefix) {
		t.Fatalf("env block = %q", got)
	}
	if !strings.HasSuffix(got, "</env>") {
		t.Fatalf("env block missing closing </env>: %q", got)
	}
}

func TestBuildSystemPromptInstructions(t *testing.T) {
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "AGENTS.md"), []byte("PROJECT RULES"), 0o644); err != nil {
		t.Fatal(err)
	}
	sys, err := buildSystemPromptForTest(d, provider.Model{ID: "Qwen3.8-27B", Name: "q"}, "kido", []string{})
	if err != nil {
		t.Fatal(err)
	}
	if len(sys) != 3 { // family, env, AGENTS.md
		t.Fatalf("len = %d (%#v)", len(sys), sys)
	}
	if !strings.Contains(sys[2], "PROJECT RULES") {
		t.Fatalf("instructions = %q", sys[2])
	}
	// nearest AGENTS.md wins on walk-up (v1 pin)
	sub := filepath.Join(d, "deep")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "AGENTS.md"), []byte("NEARER RULES"), 0o644); err != nil {
		t.Fatal(err)
	}
	sys2, err := buildSystemPromptForTest(sub, provider.Model{ID: "m", Name: "m"}, "prov", []string{filepath.Join(d, "missing-instructions.md")})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sys2[2], "NEARER RULES") {
		t.Fatal("nearest AGENTS.md must win")
	}
}

func TestPlanReminders(t *testing.T) {
	planText := mustRead(t, "prompt/plan.txt")
	switchText := mustRead(t, "prompt/build-switch.txt")
	mkp := func(role, agent string) protocol.MessageWithParts {
		return protocol.MessageWithParts{Info: protocol.Message{Role: role, Agent: agent}}
	}
	// plan agent → plan reminder
	got := PlanReminders([]protocol.MessageWithParts{mkp("user", "plan")}, "plan")
	if len(got) != 1 || got[0] != planText {
		t.Fatalf("plan reminders = %q", got)
	}
	// build→plan switch (last assistant was plan, current build) → build-switch only
	got2 := PlanReminders([]protocol.MessageWithParts{
		mkp("user", "build"),
		{Info: protocol.Message{Role: "assistant", Agent: "plan"}},
		{Info: protocol.Message{Role: "user", Agent: "build"}},
	}, "build")
	if len(got2) != 1 || got2[0] != switchText {
		t.Fatalf("switch reminders = %q", got2)
	}
	// plan→plan continues: plan reminder only
	got3 := PlanReminders([]protocol.MessageWithParts{
		{Info: protocol.Message{Role: "assistant", Agent: "plan"}},
		{Info: protocol.Message{Role: "user", Agent: "plan"}},
	}, "plan")
	if len(got3) != 1 || got3[0] != planText {
		t.Fatalf("continued plan = %q", got3)
	}
	// build→build: none
	if got4 := PlanReminders([]protocol.MessageWithParts{mkp("user", "build")}, "build"); len(got4) != 0 {
		t.Fatalf("build reminders = %q", got4)
	}
}
