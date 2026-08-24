package config_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/kido5217/yolo/internal/config"
)

// seedProfile creates <root>/<id> with the given file contents.
func seedProfile(t *testing.T, root, id string, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func writeMarker(t *testing.T, root, id string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "active"), []byte(id+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readMarker(t *testing.T, root string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, "active"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func TestEnsureActive(t *testing.T) {
	t.Run("fresh root creates default and marker", func(t *testing.T) {
		root := t.TempDir()
		id, err := config.EnsureActive(filepath.Join(root, "yolo"))
		if err != nil {
			t.Fatalf("EnsureActive: %v", err)
		}
		if id != "default" {
			t.Fatalf("active = %q, want default", id)
		}
		if _, err := os.Stat(filepath.Join(root, "yolo", "default")); err != nil {
			t.Fatalf("default dir missing: %v", err)
		}
		if got := readMarker(t, filepath.Join(root, "yolo")); got != "default" {
			t.Fatalf("marker = %q, want default", got)
		}
	})

	t.Run("existing root with valid marker is untouched", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "yolo")
		seedProfile(t, root, "abcd1234", nil)
		writeMarker(t, root, "abcd1234")
		id, err := config.EnsureActive(root)
		if err != nil {
			t.Fatalf("EnsureActive: %v", err)
		}
		if id != "abcd1234" {
			t.Fatalf("active = %q, want abcd1234", id)
		}
		if got := readMarker(t, root); got != "abcd1234" {
			t.Fatalf("marker = %q, want abcd1234 (must not be rewritten)", got)
		}
	})

	t.Run("missing marker recovers default", func(t *testing.T) {
		root := t.TempDir()
		seedProfile(t, root, "abcd1234", nil)
		id, err := config.EnsureActive(root)
		if err != nil {
			t.Fatalf("EnsureActive: %v", err)
		}
		if id != "default" || readMarker(t, root) != "default" {
			t.Fatalf("active = %q marker = %q, want default", id, readMarker(t, root))
		}
		if _, err := os.Stat(filepath.Join(root, "default")); err != nil {
			t.Fatalf("default dir missing: %v", err)
		}
	})

	t.Run("stale marker recovers default without clobbering existing default", func(t *testing.T) {
		root := t.TempDir()
		seedProfile(t, root, "default", map[string]string{"yolo.jsonc": `{"model":"keep"}`})
		writeMarker(t, root, "gone0000")
		id, err := config.EnsureActive(root)
		if err != nil {
			t.Fatalf("EnsureActive: %v", err)
		}
		if id != "default" || readMarker(t, root) != "default" {
			t.Fatalf("active = %q, want default", id)
		}
		b, err := os.ReadFile(filepath.Join(root, "default", "yolo.jsonc"))
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != `{"model":"keep"}` {
			t.Fatalf("existing default config clobbered: %s", b)
		}
	})
}

func TestGenerateID(t *testing.T) {
	hex8 := regexp.MustCompile(`^[0-9a-f]{8}$`)
	seen := map[string]bool{}
	for i := 0; i < 256; i++ {
		id, err := config.GenerateID()
		if err != nil {
			t.Fatalf("GenerateID: %v", err)
		}
		if !hex8.MatchString(id) {
			t.Fatalf("GenerateID = %q, want 8 lowercase hex chars", id)
		}
		if seen[id] {
			t.Fatalf("GenerateID produced duplicate %q in 256 draws", id)
		}
		seen[id] = true
	}
}

func TestSetActive(t *testing.T) {
	root := t.TempDir()
	seedProfile(t, root, "abcd1234", nil)
	if err := config.SetActive(root, "abcd1234"); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	if got := readMarker(t, root); got != "abcd1234" {
		t.Fatalf("marker = %q, want abcd1234", got)
	}
	if err := config.SetActive(root, "nope0000"); !errors.Is(err, config.ErrNotFound) {
		t.Fatalf("SetActive(missing) err = %v, want ErrNotFound", err)
	}
}

func TestActiveID(t *testing.T) {
	t.Run("missing marker is empty, not an error", func(t *testing.T) {
		id, err := config.ActiveID(t.TempDir())
		if err != nil {
			t.Fatalf("ActiveID: %v", err)
		}
		if id != "" {
			t.Fatalf("ActiveID = %q, want empty", id)
		}
	})
	t.Run("reads marker", func(t *testing.T) {
		root := t.TempDir()
		writeMarker(t, root, "abcd1234")
		id, err := config.ActiveID(root)
		if err != nil {
			t.Fatalf("ActiveID: %v", err)
		}
		if id != "abcd1234" {
			t.Fatalf("ActiveID = %q, want abcd1234", id)
		}
	})
}

func TestList(t *testing.T) {
	root := t.TempDir()
	seedProfile(t, root, "11111111", map[string]string{
		"yolo.jsonc": `{"profile":{"name":"work","description":"work laptop"}}`,
	})
	seedProfile(t, root, "22222222", map[string]string{
		"config.json": `{"profile":{"name":"personal"}}`,
		"yolo.jsonc":  `{"model":"kido/q"}`,
	})
	seedProfile(t, root, "33333333", nil) // no metadata: name falls back to id
	writeMarker(t, root, "11111111")      // the marker file must not list as a profile

	profiles, err := config.List(root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(profiles) != 3 {
		t.Fatalf("List = %d profiles, want 3: %+v", len(profiles), profiles)
	}
	got := map[string]config.Profile{}
	for _, p := range profiles {
		got[p.ID] = p
	}
	if p := got["11111111"]; p.Name != "work" || p.Description != "work laptop" {
		t.Fatalf("profile 11111111 = %+v, want name work + description", p)
	}
	if p := got["22222222"]; p.Name != "personal" {
		t.Fatalf("profile 22222222 = %+v, want name personal (config.json layer)", p)
	}
	if p := got["33333333"]; p.Name != "33333333" {
		t.Fatalf("profile 33333333 = %+v, want name fall back to id", p)
	}
	// sorted by name, then id
	names := make([]string, len(profiles))
	for i, p := range profiles {
		names[i] = p.Name
	}
	if !sort.StringsAreSorted(names) {
		t.Fatalf("List not sorted by name: %v", names)
	}
}

// TestListToleratesCorruptConfig pins that a profile whose global config
// fails to parse still lists (id fallback, blank description) instead of
// failing the whole listing.
func TestListToleratesCorruptConfig(t *testing.T) {
	root := t.TempDir()
	seedProfile(t, root, "11111111", map[string]string{
		"yolo.jsonc": `{"profile":{"name":"work","description":"work laptop"}}`,
	})
	seedProfile(t, root, "22222222", map[string]string{
		"yolo.jsonc": `{"profile":{"name":`, // corrupt: truncated JSONC
	})

	profiles, err := config.List(root)
	if err != nil {
		t.Fatalf("List: %v, want nil (a corrupt config must not fail the listing)", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("List = %d profiles, want 2: %+v", len(profiles), profiles)
	}
	got := map[string]config.Profile{}
	for _, p := range profiles {
		got[p.ID] = p
	}
	if p := got["22222222"]; p.Name != "22222222" || p.Description != "" {
		t.Fatalf("corrupt profile 22222222 = %+v, want id fallback + blank description", p)
	}
	if p := got["11111111"]; p.Name != "work" || p.Description != "work laptop" {
		t.Fatalf("healthy profile 11111111 = %+v, want name work + description", p)
	}
}

func TestResolve(t *testing.T) {
	root := t.TempDir()
	seedProfile(t, root, "11111111", map[string]string{
		"yolo.jsonc": `{"profile":{"name":"work"}}`,
	})
	seedProfile(t, root, "22222222", nil)

	tests := []struct {
		ref     string
		wantID  string
		wantErr error
	}{
		{ref: "11111111", wantID: "11111111"}, // exact id
		{ref: "22222222", wantID: "22222222"}, // id, no name
		{ref: "work", wantID: "11111111"},     // effective name
		{ref: "nope", wantErr: config.ErrNotFound},
	}
	for _, tc := range tests {
		t.Run("ref "+tc.ref, func(t *testing.T) {
			id, err := config.Resolve(root, tc.ref)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("Resolve err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if id != tc.wantID {
				t.Fatalf("Resolve = %q, want %q", id, tc.wantID)
			}
		})
	}

	t.Run("id wins over name when both match", func(t *testing.T) {
		r2 := t.TempDir()
		seedProfile(t, r2, "work", nil) // id literally "work"
		seedProfile(t, r2, "44444444", map[string]string{
			"yolo.jsonc": `{"profile":{"name":"work"}}`,
		})
		id, err := config.Resolve(r2, "work")
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if id != "work" {
			t.Fatalf("Resolve = %q, want id match to win", id)
		}
	})

	t.Run("ambiguous name is an error", func(t *testing.T) {
		r2 := t.TempDir()
		seedProfile(t, r2, "55555555", map[string]string{
			"yolo.jsonc": `{"profile":{"name":"dup"}}`,
		})
		seedProfile(t, r2, "66666666", map[string]string{
			"yolo.jsonc": `{"profile":{"name":"dup"}}`,
		})
		if _, err := config.Resolve(r2, "dup"); !errors.Is(err, config.ErrAmbiguous) {
			t.Fatalf("Resolve err = %v, want ErrAmbiguous", err)
		}
	})

	t.Run("missing root is not found", func(t *testing.T) {
		if _, err := config.Resolve(filepath.Join(t.TempDir(), "nope"), "x"); !errors.Is(err, config.ErrNotFound) {
			t.Fatalf("Resolve err = %v, want ErrNotFound", err)
		}
	})

	t.Run("corrupt sibling does not block name resolution", func(t *testing.T) {
		r2 := t.TempDir()
		seedProfile(t, r2, "11111111", map[string]string{
			"yolo.jsonc": `{"profile":{"name":"work"}}`,
		})
		seedProfile(t, r2, "22222222", map[string]string{
			"yolo.jsonc": `{"profile":{"name":`, // corrupt sibling
		})
		id, err := config.Resolve(r2, "work")
		if err != nil {
			t.Fatalf("Resolve(work) = %v, want nil (corrupt sibling must not block)", err)
		}
		if id != "11111111" {
			t.Fatalf("Resolve(work) = %q, want 11111111", id)
		}
	})
}

func TestAdd(t *testing.T) {
	t.Run("with name and description writes profile element", func(t *testing.T) {
		root := t.TempDir()
		p, err := config.Add(root, "work", "work laptop")
		if err != nil {
			t.Fatalf("Add: %v", err)
		}
		if p.Name != "work" || p.Description != "work laptop" {
			t.Fatalf("Add = %+v, want name work + description", p)
		}
		if !regexp.MustCompile(`^[0-9a-f]{8}$`).MatchString(p.ID) {
			t.Fatalf("Add id = %q, want auto-generated 8 hex", p.ID)
		}
		b, err := os.ReadFile(filepath.Join(root, p.ID, "yolo.jsonc"))
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatalf("yolo.jsonc: %v: %s", err, b)
		}
		prof, _ := m["profile"].(map[string]any)
		if prof == nil || prof["name"] != "work" || prof["description"] != "work laptop" {
			t.Fatalf("profile element = %v, want name+description", m["profile"])
		}
	})

	t.Run("without name or description creates an empty dir", func(t *testing.T) {
		root := t.TempDir()
		p, err := config.Add(root, "", "")
		if err != nil {
			t.Fatalf("Add: %v", err)
		}
		if p.Name != p.ID {
			t.Fatalf("name = %q, want fallback to id %q", p.Name, p.ID)
		}
		entries, err := os.ReadDir(filepath.Join(root, p.ID))
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Fatalf("profile dir has %d entries, want empty", len(entries))
		}
	})

	t.Run("duplicate name is rejected", func(t *testing.T) {
		root := t.TempDir()
		if _, err := config.Add(root, "work", ""); err != nil {
			t.Fatalf("Add: %v", err)
		}
		if _, err := config.Add(root, "work", ""); !errors.Is(err, config.ErrNameTaken) {
			t.Fatalf("Add duplicate err = %v, want ErrNameTaken", err)
		}
		if _, err := config.Add(root, "personal", ""); err != nil {
			t.Fatalf("Add distinct name: %v", err)
		}
	})

	t.Run("whitespace-only name is treated as no name", func(t *testing.T) {
		root := t.TempDir()
		p, err := config.Add(root, "   ", "")
		if err != nil {
			t.Fatalf("Add: %v", err)
		}
		if p.Name != p.ID {
			t.Fatalf("name = %q, want fallback to id", p.Name)
		}
	})

	t.Run("corrupt sibling does not block add", func(t *testing.T) {
		root := t.TempDir()
		seedProfile(t, root, "22222222", map[string]string{
			"yolo.jsonc": `{"profile":{"name":`, // corrupt sibling
		})
		p, err := config.Add(root, "home", "")
		if err != nil {
			t.Fatalf("Add: %v, want nil (name check must tolerate a corrupt sibling)", err)
		}
		if p.Name != "home" {
			t.Fatalf("Add = %+v, want name home", p)
		}
	})
}

func TestRemove(t *testing.T) {
	t.Run("removes the profile dir", func(t *testing.T) {
		root := t.TempDir()
		seedProfile(t, root, "11111111", nil)
		if err := config.Remove(root, "11111111"); err != nil {
			t.Fatalf("Remove: %v", err)
		}
		if _, err := os.Stat(filepath.Join(root, "11111111")); !os.IsNotExist(err) {
			t.Fatalf("profile dir still exists")
		}
	})

	t.Run("missing profile is an error", func(t *testing.T) {
		if err := config.Remove(t.TempDir(), "11111111"); !errors.Is(err, config.ErrNotFound) {
			t.Fatalf("Remove err = %v, want ErrNotFound", err)
		}
	})

	t.Run("removing active falls back to first remaining by name", func(t *testing.T) {
		root := t.TempDir()
		seedProfile(t, root, "11111111", map[string]string{"yolo.jsonc": `{"profile":{"name":"alpha"}}`})
		seedProfile(t, root, "22222222", map[string]string{"yolo.jsonc": `{"profile":{"name":"beta"}}`})
		writeMarker(t, root, "22222222")
		if err := config.Remove(root, "22222222"); err != nil {
			t.Fatalf("Remove: %v", err)
		}
		if got := readMarker(t, root); got != "11111111" {
			t.Fatalf("marker = %q, want 11111111 (first remaining by name)", got)
		}
	})

	t.Run("removing the only profile recreates default", func(t *testing.T) {
		root := t.TempDir()
		seedProfile(t, root, "11111111", nil)
		writeMarker(t, root, "11111111")
		if err := config.Remove(root, "11111111"); err != nil {
			t.Fatalf("Remove: %v", err)
		}
		if got := readMarker(t, root); got != "default" {
			t.Fatalf("marker = %q, want default", got)
		}
		if _, err := os.Stat(filepath.Join(root, "default")); err != nil {
			t.Fatalf("default dir missing after fallback: %v", err)
		}
	})

	t.Run("removing a non-active profile leaves the marker", func(t *testing.T) {
		root := t.TempDir()
		seedProfile(t, root, "11111111", nil)
		seedProfile(t, root, "22222222", nil)
		writeMarker(t, root, "22222222")
		if err := config.Remove(root, "11111111"); err != nil {
			t.Fatalf("Remove: %v", err)
		}
		if got := readMarker(t, root); got != "22222222" {
			t.Fatalf("marker = %q, want 22222222 (untouched)", got)
		}
	})

	t.Run("removing active with corrupt sibling succeeds and falls back", func(t *testing.T) {
		root := t.TempDir()
		seedProfile(t, root, "11111111", map[string]string{"yolo.jsonc": `{"profile":{"name":"alpha"}}`})
		seedProfile(t, root, "22222222", map[string]string{"yolo.jsonc": `{"profile":{"name":`}) // corrupt
		writeMarker(t, root, "11111111")
		if err := config.Remove(root, "11111111"); err != nil {
			t.Fatalf("Remove: %v, want nil (corrupt sibling must not block the fallback list)", err)
		}
		if got := readMarker(t, root); got != "22222222" {
			t.Fatalf("marker = %q, want 22222222 (first remaining by name)", got)
		}
	})
}

func TestCopy(t *testing.T) {
	t.Run("copies config and applies new name and description", func(t *testing.T) {
		root := t.TempDir()
		seedProfile(t, root, "11111111", map[string]string{
			"config.json": `{"profile":{"name":"work","description":"work laptop"},"permission":{"bash":"allow"}}`,
			"yolo.jsonc":  `{"model":"kido/q"}`,
		})
		p, err := config.Copy(root, "11111111", "work-home", "home copy")
		if err != nil {
			t.Fatalf("Copy: %v", err)
		}
		if p.ID == "11111111" {
			t.Fatal("Copy reused the source id")
		}
		if p.Name != "work-home" || p.Description != "home copy" {
			t.Fatalf("Copy = %+v, want name work-home + description home copy", p)
		}
		// effective config of the copy must preserve the merged source config
		cfg, err := config.LoadGlobal(filepath.Join(root, p.ID))
		if err != nil {
			t.Fatalf("LoadGlobal(copy): %v", err)
		}
		if cfg.Model != "kido/q" {
			t.Fatalf("copy model = %q, want kido/q", cfg.Model)
		}
		if cfg.Permission["bash"] != "allow" {
			t.Fatalf("copy permission = %v, want bash allow", cfg.Permission)
		}
		if cfg.Profile == nil || cfg.Profile.Name != "work-home" || cfg.Profile.Description != "home copy" {
			t.Fatalf("copy profile element = %+v, want updated name+description", cfg.Profile)
		}
	})

	t.Run("without description the source description is kept", func(t *testing.T) {
		root := t.TempDir()
		seedProfile(t, root, "11111111", map[string]string{
			"yolo.jsonc": `{"profile":{"name":"work","description":"kept"}}`,
		})
		p, err := config.Copy(root, "11111111", "work-2", "")
		if err != nil {
			t.Fatalf("Copy: %v", err)
		}
		if p.Description != "kept" {
			t.Fatalf("description = %q, want kept (copied from source)", p.Description)
		}
	})

	t.Run("name colliding with the source is rejected", func(t *testing.T) {
		root := t.TempDir()
		seedProfile(t, root, "11111111", map[string]string{
			"yolo.jsonc": `{"profile":{"name":"work"}}`,
		})
		if _, err := config.Copy(root, "11111111", "work", ""); !errors.Is(err, config.ErrNameTaken) {
			t.Fatalf("Copy err = %v, want ErrNameTaken", err)
		}
	})

	t.Run("missing source is an error", func(t *testing.T) {
		if _, err := config.Copy(t.TempDir(), "11111111", "x", ""); !errors.Is(err, config.ErrNotFound) {
			t.Fatalf("Copy err = %v, want ErrNotFound", err)
		}
	})

	t.Run("empty name is rejected", func(t *testing.T) {
		root := t.TempDir()
		seedProfile(t, root, "11111111", nil)
		if _, err := config.Copy(root, "11111111", "  ", ""); err == nil {
			t.Fatal("Copy with empty name: want error, got nil")
		}
	})
}

func TestProfileEdit(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		seed     bool
		files    map[string]string
		other    map[string]map[string]string
		nameArg  string
		descArg  string
		hasName  bool
		hasDesc  bool
		wantName string
		wantDesc string
		wantErr  error
	}{
		{
			name:     "set both name and description",
			id:       "11111111",
			seed:     true,
			files:    map[string]string{"yolo.jsonc": `{"profile":{"name":"old","description":"old desc"},"model":"kido/q"}`},
			nameArg:  "new",
			descArg:  "new desc",
			hasName:  true,
			hasDesc:  true,
			wantName: "new",
			wantDesc: "new desc",
		},
		{
			name:     "set name only keeps the description",
			id:       "22222222",
			seed:     true,
			files:    map[string]string{"yolo.jsonc": `{"profile":{"name":"old","description":"kept"}}`},
			nameArg:  "renamed",
			hasName:  true,
			wantName: "renamed",
			wantDesc: "kept",
		},
		{
			name:     "set description only on a profile without a profile element",
			id:       "33333333",
			seed:     true,
			descArg:  "just desc",
			hasDesc:  true,
			wantName: "33333333",
			wantDesc: "just desc",
		},
		{
			name:     "rename to self is a successful no-op",
			id:       "44444444",
			seed:     true,
			files:    map[string]string{"yolo.jsonc": `{"profile":{"name":"same","description":"d"}}`},
			nameArg:  "same",
			hasName:  true,
			wantName: "same",
			wantDesc: "d",
		},
		{
			name:    "name taken by another profile",
			id:      "55555555",
			seed:    true,
			files:   map[string]string{"yolo.jsonc": `{"profile":{"name":"mine"}}`},
			other:   map[string]map[string]string{"66666666": {"yolo.jsonc": `{"profile":{"name":"taken"}}`}},
			nameArg: "taken",
			hasName: true,
			wantErr: config.ErrNameTaken,
		},
		{
			name:     "clearing the name falls back to the id",
			id:       "77777777",
			seed:     true,
			files:    map[string]string{"yolo.jsonc": `{"profile":{"name":"gone","description":"kept"}}`},
			nameArg:  "",
			hasName:  true,
			wantName: "77777777",
			wantDesc: "kept",
		},
		{
			name:     "clearing the description",
			id:       "88888888",
			seed:     true,
			files:    map[string]string{"yolo.jsonc": `{"profile":{"name":"keep","description":"bye"}}`},
			descArg:  "",
			hasDesc:  true,
			wantName: "keep",
			wantDesc: "",
		},
		{
			name:     "clearing both",
			id:       "99999999",
			seed:     true,
			files:    map[string]string{"yolo.jsonc": `{"profile":{"name":"gone","description":"bye"}}`},
			nameArg:  "",
			descArg:  "",
			hasName:  true,
			hasDesc:  true,
			wantName: "99999999",
			wantDesc: "",
		},
		{
			name:    "nonexistent id",
			id:      "00000000",
			nameArg: "x",
			hasName: true,
			wantErr: config.ErrNotFound,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.seed {
				seedProfile(t, root, tc.id, tc.files)
			}
			for oid, ofiles := range tc.other {
				seedProfile(t, root, oid, ofiles)
			}
			p, err := config.Edit(root, tc.id, tc.nameArg, tc.descArg, tc.hasName, tc.hasDesc)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("Edit err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Edit: %v", err)
			}
			if p.ID != tc.id {
				t.Fatalf("Edit id = %q, want %q", p.ID, tc.id)
			}
			if p.Name != tc.wantName || p.Description != tc.wantDesc {
				t.Fatalf("Edit = %+v, want name %q description %q", p, tc.wantName, tc.wantDesc)
			}
		})
	}

	t.Run("clearing both drops the profile element from yolo.jsonc", func(t *testing.T) {
		root := t.TempDir()
		seedProfile(t, root, "11111111", map[string]string{
			"yolo.jsonc": `{"profile":{"name":"gone","description":"bye"},"model":"kido/q"}`,
		})
		if _, err := config.Edit(root, "11111111", "", "", true, true); err != nil {
			t.Fatalf("Edit: %v", err)
		}
		b, err := os.ReadFile(filepath.Join(root, "11111111", "yolo.jsonc"))
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatalf("yolo.jsonc: %v: %s", err, b)
		}
		if _, ok := m["profile"]; ok {
			t.Fatalf("profile element still present: %s", b)
		}
		if m["model"] != "kido/q" {
			t.Fatalf("model = %v, want kido/q (unrelated fields survive)", m["model"])
		}
		profiles, err := config.List(root)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(profiles) != 1 {
			t.Fatalf("List = %d profiles, want 1: %+v", len(profiles), profiles)
		}
		if p := profiles[0]; p.ID != "11111111" || p.Name != "11111111" || p.Description != "" {
			t.Fatalf("List profile = %+v, want id fallback + empty description", p)
		}
	})

	t.Run("setting both keeps unrelated config fields", func(t *testing.T) {
		root := t.TempDir()
		seedProfile(t, root, "11111111", map[string]string{
			"yolo.jsonc": `{"profile":{"name":"old","description":"old"},"model":"kido/q"}`,
		})
		if _, err := config.Edit(root, "11111111", "new", "new desc", true, true); err != nil {
			t.Fatalf("Edit: %v", err)
		}
		cfg, err := config.LoadGlobal(filepath.Join(root, "11111111"))
		if err != nil {
			t.Fatalf("LoadGlobal: %v", err)
		}
		if cfg.Model != "kido/q" {
			t.Fatalf("model = %q, want kido/q (survives the edit)", cfg.Model)
		}
		if cfg.Profile == nil || cfg.Profile.Name != "new" || cfg.Profile.Description != "new desc" {
			t.Fatalf("profile element = %+v, want name new + description new desc", cfg.Profile)
		}
	})

	t.Run("id directory name is unchanged by a name change", func(t *testing.T) {
		root := t.TempDir()
		seedProfile(t, root, "11111111", map[string]string{
			"yolo.jsonc": `{"profile":{"name":"old"}}`,
		})
		if _, err := config.Edit(root, "11111111", "new", "", true, false); err != nil {
			t.Fatalf("Edit: %v", err)
		}
		if _, err := os.Stat(filepath.Join(root, "11111111")); err != nil {
			t.Fatalf("original dir 11111111 gone: %v", err)
		}
		profiles, err := config.List(root)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(profiles) != 1 || profiles[0].ID != "11111111" {
			t.Fatalf("List = %+v, want the same id", profiles)
		}
	})

	t.Run("editing the active profile leaves the marker", func(t *testing.T) {
		root := t.TempDir()
		seedProfile(t, root, "11111111", map[string]string{
			"yolo.jsonc": `{"profile":{"name":"old"}}`,
		})
		writeMarker(t, root, "11111111")
		if _, err := config.Edit(root, "11111111", "new", "", true, false); err != nil {
			t.Fatalf("Edit: %v", err)
		}
		if got := readMarker(t, root); got != "11111111" {
			t.Fatalf("marker = %q, want 11111111 (untouched)", got)
		}
	})
}
