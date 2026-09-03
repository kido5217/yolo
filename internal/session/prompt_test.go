package session

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/provider"
)

// sha256 constants from `sha256sum internal/session/prompt/*.txt` (ALL 14).
var promptPins = map[string]string{
	"prompt/anthropic.txt":     "d4087ed2105a5f61a2f631d80744edb37a0496e9f2e1e9a412675f72583af99d",
	"prompt/beast.txt":         "8869178bf7996c25f57a6f8829e9efb62ab9db2bd542dbc2ab53c7213fccec64",
	"prompt/build-switch.txt":  "5e3db616a685a3dfaaf95fb86ae6e2acfbdf520bda60f7b27f727d2a88ba8a25",
	"prompt/codex.txt":         "b1ad2ffeb2dda8941bb707e27b35acf00b02a3dd1e8d38cdfabd40dddfb37acb",
	"prompt/copilot-gpt-5.txt": "df6814d4e9630c26a86968ce75ecd850546cef4abfc8b41046c4cb6c17142c53",
	"prompt/default.txt":       "6e207840afe7f2b905d2e3e6de1956260f35c407b663d13dca54116eff6e74e3",
	"prompt/gemini.txt":        "ba6fde8ccdd770e27f6cf56dfc925f660187c459f24315825fce23661f09c587",
	"prompt/gpt.txt":           "549aef0b99b93ea42a4c986f41d282ecb89489ead260a8238710998511be63ba",
	"prompt/kimi.txt":          "419b7fa4ed588da1e702910d5f3276a55bffd0357f1cff087704ae8c5a2cc3d8",
	"prompt/meta.txt":          "17e1197af4a338e83da5a8794d99eb904a0e1916f4b709a33e510607d87678c6",
	"prompt/plan-mode.txt":     "473381e8f20d054fa24ed3631a3b741a4fd432dadb8a0f0925f73d94a6e2866c",
	"prompt/plan.txt":          "455db97e0d21e8097c2afb539d167b4b2483e99b585dbc4fff23cafd4a3029b8",
	"prompt/title.txt":         "e7a6848eba328f28c7e870874cf0591e4edbaf90d7602ad8fdfe90601c6e656f",
	"prompt/trinity.txt":       "8b29dcd766ccd125223be9319d10e1920e26b20feef159d7397cbe8c5866d2d5",
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
		t.Run(c.api+"/"+c.prov, func(t *testing.T) {
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
				return
			}
			if !strings.Contains(text, firstLine(t, c.want)) {
				t.Fatalf("%s/%s did not select %s", c.api, c.prov, c.want)
			}
		})
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

// buildSystemPromptForTest is the test seam over buildCore: instructions are
// given explicitly and no config is involved (the engine appends the config
// instructions itself, see messagesFor).
func buildSystemPromptForTest(
	dir string, model provider.Model, providerID string, instructionPaths []string,
) ([]string, error) {
	return buildCore(dir, model.ID, providerID, instructionPaths)
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
	model := provider.Model{ID: "m", Name: "m"}
	paths := []string{filepath.Join(d, "missing-instructions.md")}
	sys2, err := buildSystemPromptForTest(sub, model, "prov", paths)
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

// TestGitRepoCacheExpires: a dir that becomes a git repo mid-process stops
// being permanently "no" once the TTL passes (pre-fix the first "no" is
// cached forever).
func TestGitRepoCacheExpires(t *testing.T) {
	dir := t.TempDir()
	oldTTL := gitCacheTTL
	gitCacheTTL = 30 * time.Millisecond
	t.Cleanup(func() { gitCacheTTL = oldTTL })
	if gitRepo(dir) {
		t.Fatal("temp dir reported as a git repo before init")
	}
	if err := exec.Command("git", "init", "-q", dir).Run(); err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for !gitRepo(dir) {
		if time.Now().After(deadline) {
			t.Fatal("cache did not expire: dir is a git repo but gitRepo still false")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
