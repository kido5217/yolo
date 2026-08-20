package tool

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadHugeLimitNoPanic(t *testing.T) {
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
