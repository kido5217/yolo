// parity_fixture_test.go — the S8.2 pin guards (root principle 3): the
// upstream pty-capture fixtures + the shared parity constants are the
// S8.3 sweep's contract — every surface has a fixture whose sha256
// matches MANIFEST.json, the catalog/canned pins match their manifest
// entries, and the yolo-side Go canned constants (parity_test.go, S8.3)
// agree with the shared canned.json the S8.1 mock is pinned to.
package tui

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const paritySurfaceCount = 17

// paritySurfaceNames is the frozen D2 list (it must equal the capture's
// SURFACES names and the MANIFEST surface names).
var paritySurfaceNames = []string{
	"home", "session-text", "session-tool", "palette", "help", "model",
	"agent", "theme", "session-list", "session-rename", "session-delete",
	"status", "which-key", "sidebar", "prompt-slash", "prompt-mention",
	"epilogue",
}

func sha256hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestParityFixturesPinned(t *testing.T) {
	man, err := os.ReadFile(filepath.Join("testdata", "parity", "upstream", "MANIFEST.json"))
	if err != nil {
		t.Fatalf("read MANIFEST.json: %v (run the S8.2 capture first: just parity-capture)", err)
	}
	var m struct {
		NPMVersion    string `json:"npm_version"`
		CatalogSHA256 string `json:"catalog_sha256"`
		CannedSHA256  string `json:"canned_sha256"`
		Surfaces      []struct {
			Name   string `json:"name"`
			Cols   int    `json:"cols"`
			Rows   int    `json:"rows"`
			SHA256 string `json:"sha256"`
		} `json:"surfaces"`
	}
	if err := json.Unmarshal(man, &m); err != nil {
		t.Fatalf("MANIFEST.json: %v", err)
	}
	if m.NPMVersion != "1.18.18" {
		t.Fatalf("npm_version = %q, want 1.18.18", m.NPMVersion)
	}
	if len(m.Surfaces) != paritySurfaceCount {
		t.Fatalf("surface count = %d, want %d", len(m.Surfaces), paritySurfaceCount)
	}
	seen := map[string]bool{}
	for _, s := range m.Surfaces {
		seen[s.Name] = true
		raw, err := os.ReadFile(filepath.Join("testdata", "parity", "upstream", s.Name+".screen.json"))
		if err != nil {
			t.Fatalf("fixture %s: %v", s.Name, err)
		}
		if got := sha256hex(raw); got != s.SHA256 {
			t.Fatalf("fixture %s sha256 = %s, manifest says %s (re-baseline in the same commit as the change)", s.Name, got, s.SHA256)
		}
		var scr map[string]any
		if err := json.Unmarshal(raw, &scr); err != nil {
			t.Fatalf("fixture %s: not screen JSON: %v", s.Name, err)
		}
	}
	for _, name := range paritySurfaceNames {
		if !seen[name] {
			t.Fatalf("the manifest is missing the surface %q", name)
		}
	}
	catalog, err := os.ReadFile(filepath.Join("testdata", "parity", "catalog-pin.json"))
	if err != nil {
		t.Fatalf("catalog-pin.json: %v (the capture creates it)", err)
	}
	if got := sha256hex(catalog); got != m.CatalogSHA256 {
		t.Fatalf("catalog pin sha256 = %s, manifest says %s", got, m.CatalogSHA256)
	}
	canned, err := os.ReadFile(filepath.Join("testdata", "parity", "canned.json"))
	if err != nil {
		t.Fatalf("canned.json: %v", err)
	}
	if got := sha256hex(canned); got != m.CannedSHA256 {
		t.Fatalf("canned pin sha256 = %s, manifest says %s", got, m.CannedSHA256)
	}
}

// TestParityCannedConsistent pins the yolo-side Go canned constants
// (parity_test.go, S8.3) against the shared canned.json (the S8.1
// mock's source) — a drift would surface as a false parity gap (D1).
func TestParityCannedConsistent(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "parity", "canned.json"))
	if err != nil {
		t.Fatalf("canned.json: %v", err)
	}
	type ctool struct {
		Name string `json:"name"`
		Args string `json:"args"`
	}
	type ccanned struct {
		Prompt    string `json:"prompt"`
		Reply     string `json:"reply"`
		Tool      *ctool `json:"tool"`
		ToolReply string `json:"tool_reply"`
		Usage     struct {
			Input  int `json:"input"`
			Output int `json:"output"`
		} `json:"usage"`
	}
	var b struct {
		Text ccanned `json:"text"`
		Tool ccanned `json:"tool"`
		Todo ccanned `json:"todo"`
	}
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if b.Text.Prompt != parityPromptText || b.Text.Reply != parityReplyText ||
		b.Text.Usage.Input != 12 || b.Text.Usage.Output != 40 {
		t.Fatalf("text turn drifted: %+v", b.Text)
	}
	if b.Tool.Prompt != parityPromptTool || b.Tool.Tool == nil ||
		b.Tool.Tool.Name != "bash" || b.Tool.Tool.Args != parityArgsTool ||
		b.Tool.ToolReply != parityReplyTool {
		t.Fatalf("tool turn drifted: %+v", b.Tool)
	}
	if b.Todo.Prompt != parityPromptTodo || b.Todo.Tool == nil ||
		b.Todo.Tool.Name != "todowrite" || b.Todo.Tool.Args != parityArgsTodo ||
		b.Todo.ToolReply != parityReplyTodo {
		t.Fatalf("todo turn drifted: %+v", b.Todo)
	}
}

// The yolo-side canned constants (D1 — pinned against canned.json by
// TestParityCannedConsistent; the S8.3 fake driver scripts from them).
const (
	parityPromptText = "say hi to the mock"
	parityReplyText  = "## Heading\n\nSome **bold** and `inline code` text.\n\n- one\n- two\n\n> a quote\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n[link](https://example.com)\n\n```js\nconst x = 1;\n```\n\n你好 world\n\nDone."

	parityPromptTool = "run the parity check"
	parityArgsTool   = `{"command":"echo parity-ok"}`
	parityReplyTool  = "The check printed parity-ok.\n"

	parityPromptTodo = "add two todos"
	parityArgsTodo   = `{"todos":[{"content":"first item","status":"in_progress"},{"content":"second item","status":"pending"}]}`
	parityReplyTodo  = "Todos updated.\n"
)
