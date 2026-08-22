package tool

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tmpFile(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func runRead(t *testing.T, p string, offset, limit int) (Output, error) {
	t.Helper()
	m := map[string]any{"filePath": p}
	if offset > 0 {
		m["offset"] = offset
	}
	if limit > 0 {
		m["limit"] = limit
	}
	raw, _ := json.Marshal(m)
	env := Env{Dir: filepath.Dir(p), Limits: Limits{2000, 50 * 1024}}
	return Registry()["read"].Run(context.Background(), raw, &env)
}

func TestReadHugeLimitNoPanic(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	fp := filepath.Join(d, "big.log")
	if err := os.WriteFile(fp, []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{
		"filePath": fp,
		"limit":    9000000000000000000.0,
	})
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}}
	out, err := Registry()["read"].Run(context.Background(), raw, env)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Text, "1: a") {
		t.Fatalf("content = %q", out.Text)
	}
}

func TestReadDirListingOverflowSafe(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	for _, n := range []string{"a", "b", "c"} {
		if err := os.WriteFile(filepath.Join(d, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, err := readDirListing(d, d, 2, math.MaxInt64)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Text, "b\nc") {
		t.Fatalf("listing = %q", out.Text)
	}
	if strings.Contains(out.Text, "a\nb") {
		t.Fatalf("offset not applied: %q", out.Text)
	}
}

func TestReadFileExactFormat(t *testing.T) {
	t.Parallel()
	p := tmpFile(t, "a.txt", "l1\nl2\nl3\n")
	out, err := runRead(t, p, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := "<path>" + p + "</path>\n<type>file</type>\n<content>\n1: l1\n2: l2\n3: l3\n\n(End of file - total 3 lines)\n</content>"
	if out.Text != want {
		t.Fatalf("text mismatch:\n%q\nwant:\n%q", out.Text, want)
	}
	if out.Title != "a.txt" {
		t.Fatalf("title = %q", out.Title)
	}
}

func TestReadFileOffsetLimit(t *testing.T) {
	t.Parallel()
	p := tmpFile(t, "a.txt", strings.Repeat("x\n", 10))
	out, err := runRead(t, p, 3, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := "<path>" + p + "</path>\n<type>file</type>\n<content>\n3: x\n4: x\n\n(Showing lines 3-4 of 10. Use offset=5 to continue.)\n</content>"
	if out.Text != want {
		t.Fatalf("text = %q", out.Text)
	}
}

func TestReadFileOffsetOutOfRange(t *testing.T) {
	t.Parallel()
	p := tmpFile(t, "a.txt", "a\nb\n")
	_, err := runRead(t, p, 99, 0)
	if err == nil || !strings.Contains(err.Error(), "offset 99 is out of range for this file") {
		t.Fatalf("err = %v", err)
	}
}

func TestReadDirListing(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	if err := os.MkdirAll(filepath.Join(d, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "b.txt"), []byte("z"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "A.txt"), []byte("z"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runRead(t, d, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	// case-insensitive sort: "A.txt" < "b.txt" < "sub/" (dir suffix pinned)
	want := "<path>" + d + "</path>\n<type>directory</type>\n<entries>\nA.txt\nb.txt\nsub/\n\n(3 entries)\n</entries>"
	if out.Text != want {
		t.Fatalf("listing mismatch:\n%q\nwant:\n%q", out.Text, want)
	}
}

// Plan deviation: the plan's pair (sibling "app.go", missing "ap.go") matches
// nothing under the pinned case-insensitive-substring-either-direction
// algorithm; the "myapp.go"/"app.go" pair exercises it.
func TestReadMissingFileSuggests(t *testing.T) {
	t.Parallel()
	p := tmpFile(t, "src/myapp.go", "x")
	_, err := runRead(t, strings.Replace(p, "myapp.go", "app.go", 1), 0, 0)
	if err == nil || !strings.Contains(err.Error(), "Did you mean one of these?") {
		t.Fatalf("err = %v", err)
	}
}

func TestReadBinaryRefused(t *testing.T) {
	t.Parallel()
	p := tmpFile(t, "bin.dat", "\x00\x01\x02"+strings.Repeat("a", 100))
	_, err := runRead(t, p, 0, 0)
	if err == nil || !strings.HasPrefix(err.Error(), "cannot read binary file:") {
		t.Fatalf("err = %v", err)
	}
}

// Plan deviation: the plan's 3000x9-byte input (27000 bytes) can never trip
// the 50KB cap under any accounting; 3000 lines x 40 bytes (120000 bytes)
// cuts around line 1305, before the 2000-line limit, as the test promises.
func TestReadByteCap(t *testing.T) {
	t.Parallel()
	line := strings.Repeat("a", 39) + "\n"
	p := tmpFile(t, "big.txt", strings.Repeat(line, 3000))
	out, err := runRead(t, p, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Text, "(Output capped at 50KB. Showing lines") || !strings.Contains(out.Text, "Use offset=") {
		t.Fatalf("no cap trailer: %q…", out.Text[:200])
	}
}

func TestTruncateTail(t *testing.T) {
	t.Parallel()
	lines := make([]string, 3000)
	for i := range lines {
		lines[i] = "line"
	}
	out, cut := Truncate(strings.Join(lines, "\n"), Limits{100, 50 * 1024})
	if !cut {
		t.Fatal("want cut")
	}
	got := strings.Split(out, "\n")
	if len(got) != 100 {
		t.Fatalf("lines = %d", len(got))
	}
}

func TestTruncateSingleLineUTF8Cut(t *testing.T) {
	t.Parallel()
	// upstream tail(): a single over-long line keeps its LAST MaxBytes
	// bytes, advanced to a UTF-8 boundary. One 80000-byte line (40000 x
	// U+00E9, 2 bytes each) cuts at byte 28800 (already a boundary) and
	// keeps 51200 bytes = 25600 runes.
	out, cut := Truncate(strings.Repeat("\u00e9", 40000), Limits{100, 50 * 1024})
	if !cut {
		t.Fatal("want cut")
	}
	if r := len([]rune(out)); r != 25600 {
		t.Fatalf("runes = %d, want 25600", r)
	}
	if b := len(out); b != 51200 {
		t.Fatalf("bytes = %d, want 51200", b)
	}
	// mid-rune cut: 51300-byte line of U+65E5 (3 bytes each) cuts at byte
	// 100, inside a rune; the boundary advance skips to byte 102, keeping
	// 51198 bytes = 17066 runes.
	out, cut = Truncate(strings.Repeat("\u65e5", 17100), Limits{100, 50 * 1024})
	if !cut {
		t.Fatal("want cut")
	}
	if r := len([]rune(out)); r != 17066 {
		t.Fatalf("runes = %d, want 17066", r)
	}
	if b := len(out); b != 51198 {
		t.Fatalf("bytes = %d, want 51198", b)
	}
}

func TestReadSchema(t *testing.T) {
	t.Parallel()
	s := SchemaFor(Registry()["read"])
	fn := s["function"].(map[string]any)
	if fn["name"] != "read" {
		t.Fatalf("name = %v", fn["name"])
	}
	params := fn["parameters"].(map[string]any)["properties"].(map[string]any)
	for _, k := range []string{"filePath", "offset", "limit"} {
		if _, ok := params[k]; !ok {
			t.Fatalf("missing param %s", k)
		}
	}
}

func TestDescPinned(t *testing.T) {
	t.Parallel()
	// sha256 of upstream v1.18.18 packages/opencode/src/tool/read.txt
	if !sha256Ok(t, "desc/read.txt", []byte(readDesc), "98ee843341c2dab2227add0019e48d4b2f0f00f9b042b853d1ee52bb34e6363d") {
		t.Fatal("desc drifted")
	}
}

func sha256Ok(t *testing.T, label string, content []byte, want string) bool {
	t.Helper()
	sum := sha256.Sum256(content)
	got := hex.EncodeToString(sum[:])
	if got != want {
		t.Logf("%s sha256 = %s, want %s", label, got, want)
		return false
	}
	return true
}
