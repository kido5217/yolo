package main

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/server"
	"github.com/kido5217/yolo/internal/tui/client"
)

func TestDispatchServeFlag(t *testing.T) {
	bin := buildBinary(t)
	out, err := exec.Command(bin, "help").CombinedOutput()
	if err != nil {
		t.Fatalf("help exit err: %v\n%s", err, out)
	}
	for _, want := range []string{"serve", "auth", "version", "help"} {
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

// TestTuiExitMapping pins the TUI process-exit contract: a program killed by
// a signal (bubbletea's built-in SIGINT/SIGTERM handler) is a clean exit 0,
// any other tea.Run error is a failure.
func TestTuiExitMapping(t *testing.T) {
	if got := tuiExit(tea.ErrProgramKilled); got != 0 {
		t.Fatalf("tuiExit(ErrProgramKilled) = %d, want 0", got)
	}
	if got := tuiExit(nil); got != 0 {
		t.Fatalf("tuiExit(nil) = %d, want 0", got)
	}
	if got := tuiExit(errors.New("no tty")); got != 1 {
		t.Fatalf("tuiExit(err) = %d, want 1", got)
	}
}

// TestDrainCancelsBusyTurnAndClosesListener pins the drain contract that the
// signal paths rely on: drain cancels the in-flight turn (status returns to
// idle), closes the listener gracefully, and finishes well inside the 5 s
// budget — the 6 s scripted turn must be cancelled, not awaited.
func TestDrainCancelsBusyTurnAndClosesListener(t *testing.T) {
	t.Setenv("YOLO_LLM", "fake")
	script := filepath.Join(t.TempDir(), "script.json")
	data := `[{"parts":[{"kind":"text","text":"slow","finish":"stop","usage":{"input":1,"output":1}}],"delay_ms":6000}]`
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

	cl := client.New("http://"+ln.String(), deps.WorkDir)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ses, err := cl.CreateSession(ctx, "drain")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, err := cl.PatchSession(ctx, ses.ID, map[string]any{"agent": "build"}); err != nil {
		t.Fatalf("patch agent: %v", err)
	}
	if _, err := cl.SendMessage(ctx, ses.ID, "hello"); err != nil {
		t.Fatalf("send: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for deps.Engine.Status(ses.ID) != protocol.StatusBusy {
		if time.Now().After(deadline) {
			t.Fatal("turn never became busy")
		}
		time.Sleep(20 * time.Millisecond)
	}

	start := time.Now()
	drain(deps, srv)
	elapsed := time.Since(start)

	if got := deps.Engine.Status(ses.ID); got != protocol.StatusIdle {
		t.Fatalf("status after drain = %q, want %q", got, protocol.StatusIdle)
	}
	if conn, err := net.DialTimeout("tcp", ln.String(), 500*time.Millisecond); err == nil {
		_ = conn.Close()
		t.Fatal("listener still accepting after drain")
	}
	if elapsed > 3*time.Second {
		t.Fatalf("drain took %v, want < 3s (5s budget; turn must be cancelled)", elapsed)
	}
}

// TestServeSigtermDrainsAndExitsZero pins the process-level contract
// SIGINT/SIGTERM -> exit 0: a serving `yolo serve` gets SIGTERM, exits 0,
// and does so within the 5 s drain budget.
func TestServeSigtermDrainsAndExitsZero(t *testing.T) {
	bin := buildBinary(t)

	root := t.TempDir()
	wd := t.TempDir()
	cmd := exec.Command(bin, "serve", "-addr", "127.0.0.1:0")
	cmd.Dir = wd
	env := make([]string, 0, len(os.Environ())+3)
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "XDG_") {
			continue
		}
		env = append(env, kv)
	}
	env = append(env,
		"XDG_CONFIG_HOME="+filepath.Join(root, "config"),
		"XDG_DATA_HOME="+filepath.Join(root, "data"),
		"XDG_CACHE_HOME="+filepath.Join(root, "cache"),
	)
	cmd.Env = env

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start serve: %v", err)
	}
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	})

	scanner := bufio.NewScanner(stdout)
	addrCh := make(chan string, 1)
	scanErr := make(chan error, 1)
	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "yolo serving on http://") {
				url := strings.TrimPrefix(line, "yolo serving on http://")
				if i := strings.Index(url, " (dir "); i >= 0 {
					url = url[:i]
				}
				addrCh <- url
				return
			}
		}
		scanErr <- scanner.Err()
	}()

	select {
	case addr := <-addrCh:
		resp, err := http.Get("http://" + addr + "/global/health")
		if err != nil {
			t.Fatalf("health: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("health = %d", resp.StatusCode)
		}
		at := time.Now()
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			t.Fatalf("send SIGTERM: %v", err)
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("serve exit after SIGTERM: %v\nstderr: %s", err, stderr.String())
			}
			if el := time.Since(at); el > 5*time.Second {
				t.Fatalf("drain took %v, want <= 5s budget", el)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("serve did not exit within 10s of SIGTERM")
		}
	case err := <-scanErr:
		t.Fatalf("serve output: %v\nstderr: %s", err, stderr.String())
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not announce its address within 10s")
	}
}
