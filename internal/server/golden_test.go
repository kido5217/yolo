package server_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/kido5217/yolo/internal/server/testutil"
)

// updateGolden regenerates testdata/golden/*.json
// (run: go test ./internal/server/ -run Golden -update).
var updateGolden = flag.Bool("update", false, "regenerate server contract golden files")

const goldenDir = "testdata/golden"

// idRe matches the generated ID contract (prefix + '_' + body).
var idRe = regexp.MustCompile(`^(ses|msg|prt|prj|evt|perm|cmd|req|mod)_[0-9A-Za-z]+$`)

// normalizer rewrites a decoded JSON tree so a response can be compared
// byte-for-byte across runs: generated IDs -> <PREFIX><n> (deduped per
// concrete id), the test project dir + its basename -> <DIR>/<DIRNAME>, and
// epoch-millisecond integers (>= 1e11) -> <T>. Maps are re-emitted key-sorted.
type normalizer struct {
	dir  string
	base string
	seen map[string]string
	cnt  map[string]int
}

func newNormalizer(dir string) *normalizer {
	return &normalizer{dir: dir, base: filepath.Base(dir), seen: map[string]string{}, cnt: map[string]int{}}
}

func (n *normalizer) idPlaceholder(s string) (string, bool) {
	m := idRe.FindStringSubmatch(s)
	if m == nil {
		return s, false
	}
	if ph, ok := n.seen[s]; ok {
		return ph, true
	}
	n.cnt[m[1]]++
	ph := strings.ToUpper(m[1]) + strconv.Itoa(n.cnt[m[1]])
	n.seen[s] = ph
	return ph, true
}

func (n *normalizer) str(s string) any {
	if ph, ok := n.idPlaceholder(s); ok {
		return ph
	}
	switch {
	case s == n.dir:
		return "<DIR>"
	case s == n.base:
		return "<DIRNAME>"
	case strings.Contains(s, n.dir):
		return strings.ReplaceAll(s, n.dir, "<DIR>")
	}
	return s
}

func (n *normalizer) num(f float64) any {
	if f >= 1e11 && f == math.Trunc(f) {
		return "<T>"
	}
	return f
}

func (n *normalizer) walk(v any) any {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make(map[string]any, len(t))
		for _, k := range keys {
			nk, _ := n.str(k).(string)
			out[nk] = n.walk(t[k])
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = n.walk(e)
		}
		return out
	case string:
		return n.str(t)
	case float64:
		return n.num(t)
	default:
		return v
	}
}

// golden performs one canonical request, normalizes the JSON body, and compares
// (or, with -update, regenerates) it against testdata/golden/<name>.json.
func golden(t *testing.T, s *testutil.TestServer, name, method, path, dir, body string, want int) {
	t.Helper()
	resp, b := testutil.Req(t, s, method, path, dir, body)
	if resp.StatusCode != want {
		t.Fatalf("%s %s: status %d, want %d: %s", method, path, resp.StatusCode, want, b)
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("%s %s: decode: %v", method, path, err)
	}
	normDir := dir
	if normDir == "" {
		normDir = s.Dir
	}
	norm := newNormalizer(normDir).walk(v)
	data, err := json.MarshalIndent(norm, "", "  ")
	if err != nil {
		t.Fatalf("%s %s: encode: %v", method, path, err)
	}
	data = append(data, '\n')
	gp := filepath.Join(goldenDir, name+".json")
	if *updateGolden {
		if err := os.MkdirAll(goldenDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(gp, data, 0o644); err != nil {
			t.Fatalf("write golden %s: %v", gp, err)
		}
		return
	}
	wantData, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("no golden %s (run: go test ./internal/server/ -run Golden -update): %v", gp, err)
	}
	if !bytes.Equal(data, wantData) {
		t.Fatalf("golden %s mismatch:\n--- got ---\n%s\n--- want ---\n%s", gp, data, wantData)
	}
}

// mkSession creates a session in dir ("") and returns its id.
func mkSession(t *testing.T, s *testutil.TestServer, dir, title string) string {
	t.Helper()
	resp, b := testutil.Req(t, s, "POST", "/session", dir, `{"title":"`+title+`"}`)
	if resp.StatusCode != 201 {
		t.Fatalf("create session: %d %s", resp.StatusCode, b)
	}
	var ses struct{ ID string }
	_ = json.Unmarshal(b, &ses)
	if ses.ID == "" {
		t.Fatalf("create session: empty id: %s", b)
	}
	return ses.ID
}

func TestGoldenResponses(t *testing.T) {
	t.Run("health", func(t *testing.T) {
		t.Parallel()
		s := testutil.Boot(t)
		golden(t, s, "health", "GET", "/global/health", "", "", 200)
	})
	t.Run("path", func(t *testing.T) {
		t.Parallel()
		s := testutil.Boot(t)
		d := t.TempDir()
		golden(t, s, "path", "GET", "/path", d, "", 200)
	})
	t.Run("project", func(t *testing.T) {
		t.Parallel()
		s := testutil.Boot(t)
		d := t.TempDir()
		golden(t, s, "project", "GET", "/project/current", d, "", 200)
	})
	t.Run("session_list", func(t *testing.T) {
		t.Parallel()
		s := testutil.Boot(t)
		d := t.TempDir()
		mkSession(t, s, d, "Golden")
		golden(t, s, "session_list", "GET", "/session", d, "", 200)
	})
	t.Run("session_create", func(t *testing.T) {
		t.Parallel()
		s := testutil.Boot(t)
		d := t.TempDir()
		golden(t, s, "session_create", "POST", "/session", d, `{"title":"Golden"}`, 201)
	})
	t.Run("session_get", func(t *testing.T) {
		t.Parallel()
		s := testutil.Boot(t)
		d := t.TempDir()
		id := mkSession(t, s, d, "Golden")
		golden(t, s, "session_get", "GET", "/session/"+id, d, "", 200)
	})
	t.Run("session_patch", func(t *testing.T) {
		t.Parallel()
		s := testutil.Boot(t)
		d := t.TempDir()
		id := mkSession(t, s, d, "Golden")
		golden(t, s, "session_patch", "PATCH", "/session/"+id, d,
			`{"title":"Patched","agent":"yolo","model":"opencode/gpt-5-nano"}`, 200)
	})
	t.Run("message_list", func(t *testing.T) {
		t.Parallel()
		s := testutil.Boot(t)
		d := t.TempDir()
		id := mkSession(t, s, d, "Golden")
		resp, b := testutil.Req(t, s, "POST", "/session/"+id+"/message", d, `{"text":"hello"}`)
		if resp.StatusCode != 202 {
			t.Fatalf("send: %d %s", resp.StatusCode, b)
		}
		s.WaitIdle(t, d, id)
		golden(t, s, "message_list", "GET", "/session/"+id+"/message", d, "", 200)
	})
	t.Run("provider", func(t *testing.T) {
		t.Parallel()
		s := testutil.Boot(t)
		d := t.TempDir()
		golden(t, s, "provider", "GET", "/provider", d, "", 200)
	})
	t.Run("config", func(t *testing.T) {
		t.Parallel()
		s := testutil.Boot(t)
		d := t.TempDir()
		testutil.WriteCfg(t, d, `{"model":"kido/q","permission":{"edit":"ask"}}`)
		golden(t, s, "config", "GET", "/config", d, "", 200)
	})
	t.Run("agent", func(t *testing.T) {
		t.Parallel()
		s := testutil.Boot(t)
		golden(t, s, "agent", "GET", "/agent", "", "", 200)
	})
	t.Run("command", func(t *testing.T) {
		t.Parallel()
		s := testutil.Boot(t)
		golden(t, s, "command", "GET", "/command", "", "", 200)
	})
	t.Run("permission_empty", func(t *testing.T) {
		t.Parallel()
		s := testutil.Boot(t)
		d := t.TempDir()
		golden(t, s, "permission_empty", "GET", "/permission", d, "", 200)
	})
	t.Run("status", func(t *testing.T) {
		t.Parallel()
		s := testutil.Boot(t)
		d := t.TempDir()
		mkSession(t, s, d, "Golden")
		golden(t, s, "status", "GET", "/session/status", d, "", 200)
	})
}
