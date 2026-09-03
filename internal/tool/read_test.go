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

// TestDescPinned pins each tool desc file's sha256; the pins record current
// intended content (change gates, root principle 3), not an upstream lock.
func TestDescPinned(t *testing.T) {
	t.Parallel()
	cases := []struct {
		file string
		desc string
		want string
	}{
		{file: "desc/bash.txt", desc: bashDesc, want: "5f45f41bbd0ed41f5764d9e7eaf4716a26219e0fa81d2ae83b1aebcd8eb6cf88"},
		{file: "desc/edit.txt", desc: editDesc, want: "4426ccf60241fe41d01bbafc1e7450ea6538003f9fca863ab0210492a74647f8"},
		{file: "desc/glob.txt", desc: globDesc, want: "50b2d2c41d4b8d0286ab4542c6ec882421ac4ae5c0567ad213c3668ed973ed9a"},
		{file: "desc/grep.txt", desc: grepDesc, want: "97fa2a9929353d20d3418041aae53ffea3aaf63e9a6e2fdc8cff6db61c3f4c5e"},
		{file: "desc/read.txt", desc: readDesc, want: "98ee843341c2dab2227add0019e48d4b2f0f00f9b042b853d1ee52bb34e6363d"},
		{file: "desc/todowrite.txt", desc: todoWriteDesc, want: "f214ea20cd870a9837cb30dd993aefbe5abe6d9e3319b47672c529961ba0c3ad"},
		{file: "desc/write.txt", desc: writeDesc, want: "8b7197b6e3a8ec1d129eeb6b82608e4cab759bfcc60ba890ecf36322a6e45180"},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			t.Parallel()
			if !sha256Ok(t, tc.file, []byte(tc.desc), tc.want) {
				t.Fatal("desc drifted")
			}
		})
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
