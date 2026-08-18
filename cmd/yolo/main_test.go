package main

import (
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kido5217/yolo/internal/server"
)

func TestDispatchServeFlag(t *testing.T) {
	bin := buildBinary(t)
	out, err := exec.Command(bin, "help").CombinedOutput()
	if err != nil {
		t.Fatalf("help exit err: %v\n%s", err, out)
	}
	for _, want := range []string{"serve", "auth"} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("help output missing %q:\n%s", want, out)
		}
	}
}

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := t.TempDir() + "/yolo"
	cmd := exec.Command("go", "build", "-o", bin, ".")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

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
	deps, closeDB, err := buildDeps(workDir)
	if err != nil {
		t.Fatalf("buildDeps: %v", err)
	}
	return deps, closeDB
}
