package main

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kido5217/yolo/internal/server"
)

// fakeScriptOK is the minimal one-turn fake-driver script: a single text
// part, no delay. Shared by the tests that boot the core stack without a
// meaningful LLM response.
const fakeScriptOK = `[{"parts":[{"kind":"text","text":"ok","finish":"stop","usage":{"input":1,"output":1}}]}]`

// TestServeHealthInProcess boots the same stack `yolo serve` builds
// (buildDeps via buildStack) on the fake driver and asserts the core API
// answers in-process — no child process in the unit suite.
func TestServeHealthInProcess(t *testing.T) {
	t.Setenv("YOLO_LLM", "fake")
	script := filepath.Join(t.TempDir(), "script.json")
	data := `[{"parts":[{"kind":"text","text":"ok","finish":"stop","usage":{"input":1,"output":1}}],"delay_ms":10}]`
	if err := os.WriteFile(script, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("YOLO_FAKE_SCRIPT", script)

	deps, cleanup := buildStack(t)
	defer cleanup()
	srv := server.NewServer(*deps)
	ln, err := srv.Start("127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer srv.Close()

	resp, err := http.Get("http://" + ln.String() + "/global/health")
	if err != nil {
		t.Fatalf("GET /global/health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET /global/health = %d: %s", resp.StatusCode, b)
	}
}

// buildStack boots the full core stack behind `yolo serve` and TUI mode
// (buildDeps) against temp XDG roots and the env-gated fake driver (the
// caller sets YOLO_LLM=fake + YOLO_FAKE_SCRIPT). It returns the server
// deps and a cleanup that closes the DB; the test starts its own
// in-process listener.
func buildStack(t *testing.T) (*server.Deps, func()) {
	t.Helper()
	root := t.TempDir()
	workDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	deps, closeDB, err := buildDeps(workDir, "")
	if err != nil {
		t.Fatalf("buildDeps: %v", err)
	}
	return deps, closeDB
}

// seedProfileRootForCmd builds <configHome>/yolo with one profile dir per
// model value plus the given active marker ("" = no marker).
func seedProfileRootForCmd(t *testing.T, configHome, marker string, models map[string]string) {
	t.Helper()
	for id, model := range models {
		p := filepath.Join(configHome, "yolo", id, "yolo.jsonc")
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(`{"model":"`+model+`"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if marker != "" {
		p := filepath.Join(configHome, "yolo", "active")
		if err := os.WriteFile(p, []byte(marker+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestBuildDepsProfileSelection pins the process profile precedence in
// buildDeps: the --profile flag beats the YOLO_PROFILE env, which beats
// the active marker; a first run (no root) creates the default profile.
func TestBuildDepsProfileSelection(t *testing.T) {
	root := t.TempDir()
	workDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	script := filepath.Join(root, "script.json")
	if err := os.WriteFile(script, []byte(fakeScriptOK), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("YOLO_LLM", "fake")
	t.Setenv("YOLO_FAKE_SCRIPT", script)
	seedProfileRootForCmd(t, filepath.Join(root, "config"), "work", map[string]string{
		"default": "m-default",
		"work":    "m-work",
	})

	selectProfile := func(t *testing.T, flag string) (string, error) {
		t.Helper()
		deps, closeDB, err := buildDeps(workDir, flag)
		if err != nil {
			return "", err
		}
		defer closeDB()
		return deps.Dirs.Profile, nil
	}

	t.Run("no flag or env: active marker", func(t *testing.T) {
		// Pin the env explicitly: an inherited YOLO_PROFILE must not leak
		// into this subtest (t.Setenv restores it when the subtest ends).
		t.Setenv("YOLO_PROFILE", "")
		got, err := selectProfile(t, "")
		if err != nil {
			t.Fatalf("buildDeps: %v", err)
		}
		if got != "work" {
			t.Fatalf("profile = %q, want work (marker)", got)
		}
	})
	t.Run("env overrides the marker", func(t *testing.T) {
		t.Setenv("YOLO_PROFILE", "default")
		got, err := selectProfile(t, "")
		if err != nil {
			t.Fatalf("buildDeps: %v", err)
		}
		if got != "default" {
			t.Fatalf("profile = %q, want default (env)", got)
		}
	})
	t.Run("flag beats env and marker", func(t *testing.T) {
		t.Setenv("YOLO_PROFILE", "default")
		got, err := selectProfile(t, "work")
		if err != nil {
			t.Fatalf("buildDeps: %v", err)
		}
		if got != "work" {
			t.Fatalf("profile = %q, want work (flag)", got)
		}
	})
	t.Run("name references resolve to ids", func(t *testing.T) {
		p := filepath.Join(
			root,
			"config",
			"yolo",
			"aaaa1111",
			"yolo.jsonc",
		)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(`{"profile":{"name":"home-office"},"model":"m-home"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := selectProfile(t, "home-office")
		if err != nil {
			t.Fatalf("buildDeps: %v", err)
		}
		if got != "aaaa1111" {
			t.Fatalf("profile = %q, want aaaa1111 (resolved by name)", got)
		}
	})
	t.Run("missing profile: error", func(t *testing.T) {
		if _, err := selectProfile(t, "nope"); err == nil {
			t.Fatal("buildDeps with missing profile: want error, got nil")
		}
	})
}

// TestBuildDepsSweepErrorLogged: the startup retention sweep
// (CleanOutputDir) is best effort — a sweep failure must not block boot,
// but it must not vanish silently: buildDeps logs it at WARN in yolo.log.
func TestBuildDepsSweepErrorLogged(t *testing.T) {
	root := t.TempDir()
	workDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	t.Setenv("YOLO_LOG_LEVEL", "INFO")
	script := filepath.Join(root, "script.json")
	if err := os.WriteFile(script, []byte(fakeScriptOK), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("YOLO_LLM", "fake")
	t.Setenv("YOLO_FAKE_SCRIPT", script)

	// A regular file at the sweep path makes ReadDir fail (ENOTDIR, not
	// os.ErrNotExist), so CleanOutputDir returns an error at startup.
	toolOutput := filepath.Join(root, "data", "yolo", "tool-output")
	if err := os.MkdirAll(filepath.Dir(toolOutput), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(toolOutput, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A sweep failure must not block boot.
	_, closeDB, err := buildDeps(workDir, "")
	if err != nil {
		t.Fatalf("buildDeps with a broken sweep dir: %v", err)
	}
	defer closeDB()

	data, err := os.ReadFile(filepath.Join(root, "data", "yolo", "log", "yolo.log"))
	if err != nil {
		t.Fatalf("read yolo.log: %v", err)
	}
	// The sweep message and the WARN level must sit on the SAME line:
	// an unrelated WARN plus the message at another level would not do.
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, "tool-output sweep failed") && strings.Contains(line, "level=WARN") {
			return
		}
	}
	t.Fatalf("sweep error not logged at WARN in yolo.log:\n%s", data)
}
