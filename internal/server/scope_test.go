package server_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/kido5217/yolo/internal/server/testutil"
)

// TestScopeMatrix verifies directory scoping: absent header falls back to the
// server work dir, and every id-scoped route 404s for a session id belonging to
// a different directory.
func TestScopeMatrix(t *testing.T) {
	t.Parallel()
	s := testutil.Boot(t)
	wd := s.Dir

	t.Run("no_header_uses_workdir", func(t *testing.T) {
		resp, b := testutil.Req(t, s, "GET", "/path", "", "")
		var p map[string]string
		_ = json.Unmarshal(b, &p)
		if resp.StatusCode != 200 || p["directory"] != wd {
			t.Fatalf("no-header /path = %d %s, want dir %s", resp.StatusCode, b, wd)
		}
		mkSession(t, s, "", "Cwd") // created without a header -> work dir
		_, b = testutil.Req(t, s, "GET", "/session", "", "")
		var list []map[string]any
		_ = json.Unmarshal(b, &list)
		if len(list) != 1 {
			t.Fatalf("workdir session list = %d, want 1: %s", len(list), b)
		}
	})

	// id-scoped routes: each must 404 for a session id scoped to another dir.
	idScoped := []struct {
		method, path string
		body         string
	}{
		{"GET", "/session/%s", ""},
		{"PATCH", "/session/%s", `{"title":"x"}`},
		{"DELETE", "/session/%s", ""},
		{"GET", "/session/%s/message", ""},
		{"POST", "/session/%s/message", `{"text":"hi"}`},
		{"POST", "/session/%s/abort", ""},
		{"POST", "/session/%s/command", `{"command":"/new"}`},
	}
	dA := t.TempDir()
	dB := t.TempDir()
	id := mkSession(t, s, dA, "A")
	for _, r := range idScoped {
		resp, b := testutil.Req(t, s, r.method, fmt.Sprintf(r.path, id), dB, r.body)
		if resp.StatusCode != 404 {
			t.Fatalf("%s %s (other dir) = %d, want 404: %s", r.method, r.path, resp.StatusCode, b)
		}
	}
	// sanity: the owning dir resolves the id
	resp, b := testutil.Req(t, s, "GET", "/session/"+id, dA, "")
	if resp.StatusCode != 200 {
		t.Fatalf("GET /session/%s (owning dir) = %d, want 200: %s", id, resp.StatusCode, b)
	}
}
