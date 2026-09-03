package theme

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestKVGetSetAndNilDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.json")
	kv, err := OpenKV(path)
	if err != nil {
		t.Fatalf("OpenKV: %v", err)
	}
	defer kv.Close()

	// `??` semantics: missing key → default.
	if got := kv.Get("absent", "dflt"); got != "dflt" {
		t.Errorf("Get(absent) = %v, want dflt", got)
	}
	kv.Set("theme", "kanagawa")
	if got := kv.Get("theme", "dflt"); got != "kanagawa" {
		t.Errorf("Get(theme) = %v, want kanagawa", got)
	}
	// Falsy JSON values are preserved (`??`, not `||`): false, "", 0.
	kv.Set("flag", false)
	if got := kv.Get("flag", "dflt"); got != false {
		t.Errorf("Get(flag) = %v, want false (falsy preserved)", got)
	}
	kv.Set("empty", "")
	if got := kv.Get("empty", "dflt"); got != "" {
		t.Errorf("Get(empty) = %v, want \"\" (falsy preserved)", got)
	}
	kv.Set("zero", 0)
	if got := kv.Get("zero", "dflt"); got != 0 {
		t.Errorf("Get(zero) = %v, want 0 (falsy preserved)", got)
	}
	// A nil value deletes the key (upstream setStore(key, undefined)).
	kv.Set("theme", nil)
	if got := kv.Get("theme", "dflt"); got != "dflt" {
		t.Errorf("Get(theme) after nil-set = %v, want dflt (nil deletes)", got)
	}
}

func TestKVMissingFileIsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.json")
	kv, err := OpenKV(path)
	if err != nil {
		t.Fatalf("OpenKV on missing file must not error: %v", err)
	}
	defer kv.Close()
	if got := kv.Get("theme", "opencode"); got != "opencode" {
		t.Errorf("Get = %v, want opencode (missing file = empty store)", got)
	}
}

func TestKVCorruptFileIsLoggedAndEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	kv, err := OpenKV(path)
	if err != nil {
		t.Fatalf("corrupt file must not error (upstream catch → continue): %v", err)
	}
	defer kv.Close()
	if got := kv.Get("theme", "opencode"); got != "opencode" {
		t.Errorf("Get = %v, want opencode (corrupt file = empty store)", got)
	}
	kv.Set("theme", "nord")
	if got := kv.Get("theme", "dflt"); got != "nord" {
		t.Errorf("Get = %v, want nord (store usable after corrupt load)", got)
	}
}

func TestKVRapidSetsFlushOrderedOnClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deep", "nested", "kv.json")
	kv, err := OpenKV(path)
	if err != nil {
		t.Fatalf("OpenKV (must MkdirAll the parent): %v", err)
	}
	for i := 0; i < 50; i++ {
		kv.Set(fmt.Sprintf("key%02d", i), i)
	}
	if err := kv.Close(); err != nil {
		t.Fatalf("Close (drain + flush): %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read KV file: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("KV file must be valid JSON: %v\n%s", err, data)
	}
	if len(m) != 50 {
		t.Fatalf("keys = %d, want 50", len(m))
	}
	for i := 0; i < 50; i++ {
		if got := m[fmt.Sprintf("key%02d", i)]; got != float64(i) {
			t.Errorf("key%02d = %v, want %d (ordered writes)", i, got, i)
		}
	}
}

func TestKVReloadPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.json")
	kv, err := OpenKV(path)
	if err != nil {
		t.Fatal(err)
	}
	kv.Set("theme", "kanagawa")
	if err := kv.Close(); err != nil {
		t.Fatal(err)
	}
	kv2, err := OpenKV(path)
	if err != nil {
		t.Fatal(err)
	}
	defer kv2.Close()
	if got := kv2.Get("theme", "dflt"); got != "kanagawa" {
		t.Errorf("reloaded Get = %v, want kanagawa", got)
	}
}
