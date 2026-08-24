package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/auth"
	"github.com/kido5217/yolo/internal/bus"
	"github.com/kido5217/yolo/internal/config"
	"github.com/kido5217/yolo/internal/llm"
	"github.com/kido5217/yolo/internal/log"
	"github.com/kido5217/yolo/internal/permission"
	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/provider"
	"github.com/kido5217/yolo/internal/server"
	"github.com/kido5217/yolo/internal/session"
	"github.com/kido5217/yolo/internal/storage"
	"github.com/kido5217/yolo/internal/tool"
	"github.com/kido5217/yolo/internal/tui/client"
)

func TestHelpListsSubcommands(t *testing.T) {
	bin := buildBinary(t)
	out, err := exec.Command(bin, "help").CombinedOutput()
	if err != nil {
		t.Fatalf("help exit err: %v\n%s", err, out)
	}
	for _, want := range []string{"serve", "auth", "profile", "version", "help"} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("help output missing %q:\n%s", want, out)
		}
	}
	if !strings.Contains(string(out), "edit ID") {
		t.Fatalf("help output missing the profile edit subcommand:\n%s", out)
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
	if err := os.WriteFile(script,
		[]byte(`[{"parts":[{"kind":"text","text":"ok","finish":"stop","usage":{"input":1,"output":1}}]}]`), 0o644); err != nil {
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
		p := filepath.Join(root, "config", "yolo", "aaaa1111", "yolo.jsonc")
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
	for deps.Engine.Status(ses.ID) != protocol.SessionStatusBusy {
		if time.Now().After(deadline) {
			t.Fatal("turn never became busy")
		}
		time.Sleep(20 * time.Millisecond)
	}

	start := time.Now()
	drain(deps, srv)
	elapsed := time.Since(start)

	if got := deps.Engine.Status(ses.ID); got != protocol.SessionStatusIdle {
		t.Fatalf("status after drain = %q, want %q", got, protocol.SessionStatusIdle)
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

// TestAuthCmd pins the yolo auth CLI surface: subcommand dispatch, usage
// and exit code on missing args, key handling (explicit arg and stdin
// prompt), and store load/save errors. State is isolated to a temp XDG
// data dir; os.Stdin/Stdout/Stderr are swapped per case.
func TestAuthCmd(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))

	authPath := func() string {
		p, err := auth.Path()
		if err != nil {
			t.Fatalf("auth.Path: %v", err)
		}
		return p
	}
	readStore := func() auth.Store {
		s, err := auth.LoadFrom(authPath())
		if err != nil {
			t.Fatalf("load store: %v", err)
		}
		return s
	}
	// runAuth runs authCmd with the process stdio swapped for pipes and
	// returns (exit code, stdout, stderr).
	runAuth := func(t *testing.T, stdin string, args ...string) (int, string, string) {
		t.Helper()
		oldIn, oldOut, oldErr := os.Stdin, os.Stdout, os.Stderr
		t.Cleanup(func() { os.Stdin, os.Stdout, os.Stderr = oldIn, oldOut, oldErr })
		if stdin != "" {
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			_, _ = w.WriteString(stdin)
			_ = w.Close()
			os.Stdin = r
		}
		outR, outW, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		errR, errW, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		os.Stdout, os.Stderr = outW, errW
		code := authCmd(args)
		_ = outW.Close()
		_ = errW.Close()
		outB, _ := io.ReadAll(outR)
		errB, _ := io.ReadAll(errR)
		return code, string(outB), string(errB)
	}

	tests := []struct {
		name        string
		seed        map[string]auth.Entry
		corrupt     bool
		stdin       string
		args        []string
		wantCode    int
		wantOutPart string
		wantErrPart string
		wantOrder   [2]string
		wantStore   map[string]string
	}{
		{
			name:        "no subcommand: usage, exit 2",
			args:        nil,
			wantCode:    2,
			wantErrPart: "Usage:",
		},
		{
			name:        "unknown subcommand: usage, exit 2",
			args:        []string{"bogus"},
			wantCode:    2,
			wantErrPart: "Usage:",
		},
		{
			name:        "list without credentials",
			args:        []string{"list"},
			wantCode:    0,
			wantOutPart: "no credentials",
		},
		{
			name:      "add with explicit key persists store",
			args:      []string{"add", "kido", "sk-test"},
			wantCode:  0,
			wantStore: map[string]string{"kido": "sk-test"},
		},
		{
			name:        "add missing provider: usage, exit 2",
			args:        []string{"add"},
			wantCode:    2,
			wantErrPart: "Usage:",
		},
		{
			name:      "remove existing provider",
			seed:      map[string]auth.Entry{"kido": {Type: "api", Key: "sk-old"}},
			args:      []string{"remove", "kido"},
			wantCode:  0,
			wantStore: map[string]string{},
		},
		{
			name:      "remove missing provider is a no-op",
			args:      []string{"remove", "nope"},
			wantCode:  0,
			wantStore: map[string]string{},
		},
		{
			name: "list shows sorted entries",
			seed: map[string]auth.Entry{
				"zeta": {Type: "api", Key: "sk-zeta"},
				"kido": {Type: "api", Key: "sk-kido"},
			},
			args:        []string{"list"},
			wantCode:    0,
			wantOutPart: "api  (set)",
			wantOrder:   [2]string{"kido", "zeta"},
		},
		{
			name:        "add via stdin prompt",
			stdin:       "sk-stdin\n",
			args:        []string{"add", "kido"},
			wantCode:    0,
			wantErrPart: "API key: ",
			wantStore:   map[string]string{"kido": "sk-stdin"},
		},
		{
			name:        "corrupt store: load error, exit 1",
			corrupt:     true,
			args:        []string{"list"},
			wantCode:    1,
			wantErrPart: "auth list:",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_ = os.Remove(authPath())
			if tc.corrupt {
				if err := os.WriteFile(authPath(), []byte("{not json"), 0o600); err != nil {
					t.Fatalf("write corrupt store: %v", err)
				}
			} else if tc.seed != nil {
				if err := auth.SaveTo(auth.Store(tc.seed), authPath()); err != nil {
					t.Fatalf("seed store: %v", err)
				}
			}

			code, out, errOut := runAuth(t, tc.stdin, tc.args...)
			if code != tc.wantCode {
				t.Fatalf("authCmd exit = %d, want %d\nstdout: %s\nstderr: %s", code, tc.wantCode, out, errOut)
			}
			if tc.wantOutPart != "" && !strings.Contains(out, tc.wantOutPart) {
				t.Fatalf("stdout missing %q:\n%s", tc.wantOutPart, out)
			}
			if tc.wantErrPart != "" && !strings.Contains(errOut, tc.wantErrPart) {
				t.Fatalf("stderr missing %q:\n%s", tc.wantErrPart, errOut)
			}
			if tc.wantOrder[0] != "" {
				i, j := strings.Index(out, tc.wantOrder[0]), strings.Index(out, tc.wantOrder[1])
				if i < 0 || j < 0 || i > j {
					t.Fatalf("stdout order %q before %q violated:\n%s", tc.wantOrder[0], tc.wantOrder[1], out)
				}
			}
			if tc.wantStore != nil {
				s := readStore()
				if len(s) != len(tc.wantStore) {
					t.Fatalf("store = %v, want %v", s, tc.wantStore)
				}
				for p, k := range tc.wantStore {
					e, ok := s[p]
					if !ok || e.Key != k || e.Type != "api" {
						t.Fatalf("store[%q] = %+v, want key %q type api", p, e, k)
					}
				}
			}
		})
	}
}

// TestDispatchExitCodes pins run()'s dispatch contract that has no
// in-process seam: version prints the version and exits 0, the explicit
// help flag prints usage and exits 0, unknown flags and too many
// positionals fail with usage and exit 2.
func TestDispatchExitCodes(t *testing.T) {
	bin := buildBinary(t)
	tests := []struct {
		name          string
		args          []string
		wantCode      int
		wantPart      string
		wantFirstLine string
	}{
		{name: "version", args: []string{"version"}, wantCode: 0, wantFirstLine: "yolo 0.0.0-dev"},
		{name: "v flag", args: []string{"-v"}, wantCode: 0, wantFirstLine: "yolo 0.0.0-dev"},
		{name: "version long flag", args: []string{"--version"}, wantCode: 0, wantFirstLine: "yolo 0.0.0-dev"},
		{name: "explicit help flag", args: []string{"--help"}, wantCode: 0, wantPart: "Usage:"},
		{name: "unknown flag exits 2", args: []string{"--bogus"}, wantCode: 2},
		{name: "too many positionals exit 2", args: []string{"a", "b"}, wantCode: 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := exec.Command(bin, tc.args...).CombinedOutput()
			if tc.wantCode == 0 {
				if err != nil {
					t.Fatalf("exit = %v, want 0\n%s", err, out)
				}
			} else {
				var ee *exec.ExitError
				if !errors.As(err, &ee) || ee.ExitCode() != tc.wantCode {
					t.Fatalf("exit = %v, want %d\n%s", err, tc.wantCode, out)
				}
			}
			if tc.wantPart != "" && !strings.Contains(string(out), tc.wantPart) {
				t.Fatalf("output missing %q:\n%s", tc.wantPart, out)
			}
			if tc.wantFirstLine != "" {
				lines := strings.Split(strings.TrimSpace(string(out)), "\n")
				if lines[0] != tc.wantFirstLine {
					t.Fatalf("first line = %q, want %q:\n%s", lines[0], tc.wantFirstLine, out)
				}
			}
		})
	}
}

// TestServeVersionFlag pins -v/--version inside the serve flag set: it prints
// the version block, exits 0, and never starts listening.
func TestServeVersionFlag(t *testing.T) {
	bin := buildBinary(t)
	for _, flag := range []string{"-v", "--version"} {
		out, err := exec.Command(bin, "serve", flag).CombinedOutput()
		if err != nil {
			t.Fatalf("serve %s exit = %v\n%s", flag, err, out)
		}
		if !strings.Contains(string(out), "yolo 0.0.0-dev") {
			t.Fatalf("serve %s missing version:\n%s", flag, out)
		}
		if strings.Contains(string(out), "yolo serving on") {
			t.Fatalf("serve %s started the server:\n%s", flag, out)
		}
	}
}

// TestJustfileVersionRecipe pins the justfile entry point: it parses and the
// version variable resolves to a non-empty git-derived string (skipped when
// `just` is not installed — the artifact still ships).
func TestJustfileVersionRecipe(t *testing.T) {
	if _, err := exec.LookPath("just"); err != nil {
		t.Skip("just not installed")
	}
	out, err := exec.Command("just", "--evaluate", "version").CombinedOutput()
	if err != nil {
		t.Fatalf("just --evaluate version: %v\n%s", err, out)
	}
	if v := strings.TrimSpace(string(out)); v == "" || v == "0.0.0-dev" {
		t.Fatalf("version variable = %q, want a git-derived value", v)
	}
	list, err := exec.Command("just", "--list").CombinedOutput()
	if err != nil {
		t.Fatalf("just --list: %v\n%s", err, list)
	}
	for _, want := range []string{"build", "e2e-live"} {
		if !strings.Contains(string(list), want) {
			t.Fatalf("just --list missing %q:\n%s", want, list)
		}
	}
}

// TestTuiRunErrorPrintsToStderr pins W (row 12): a TUI start failure prints
// one line to stderr in addition to the log + exit code. The child runs in
// its own session (no controlling terminal), so bubbletea's Run fails fast
// with a TTY-open error.
func TestTuiRunErrorPrintsToStderr(t *testing.T) {
	bin := buildBinary(t)
	root := t.TempDir()
	wd := t.TempDir()
	script := filepath.Join(root, "script.json")
	if err := os.WriteFile(script,
		[]byte(`[{"parts":[{"kind":"text","text":"ok","finish":"stop","usage":{"input":1,"output":1}}]}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "--dir", wd)
	env := make([]string, 0, len(os.Environ())+5)
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "XDG_") {
			continue
		}
		env = append(env, kv)
	}
	env = append(env,
		"YOLO_LLM=fake", "YOLO_FAKE_SCRIPT="+script,
		"XDG_CONFIG_HOME="+filepath.Join(root, "config"),
		"XDG_DATA_HOME="+filepath.Join(root, "data"),
		"XDG_CACHE_HOME="+filepath.Join(root, "cache"),
	)
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} // no controlling TTY
	stdin, err := os.Open("/dev/null")
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	cmd.Stdin = stdin
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err = cmd.Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("exit = %v, want exit code 1 (a TUI start failure)", err)
	}
	lines := strings.Split(strings.TrimSpace(stderr.String()), "\n")
	found := false
	for _, l := range lines {
		if strings.HasPrefix(l, "yolo: ") && strings.Contains(l, "TTY") {
			found = true
		}
	}
	if !found {
		t.Fatalf("stderr has no one-line TUI failure:\n%s", stderr.String())
	}
}

// TestServeDrainForceKill pins X (concurrency-5): a turn hung on a
// provider call that ignores cancellation outlasts the drain budget, so
// only the second signal's force-kill (immediate ctx cancel) ends the
// drain — measured against the real drainCtx + armForceKill wiring.
func TestServeDrainForceKill(t *testing.T) {
	root := t.TempDir()
	wd := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))

	db, err := openDB(filepath.Join(root, "yolo.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	logger := log.New(root) // closed by drainCtx; not cleaned up separately
	b := bus.New()
	perm := permission.New(db, b, logger, root)
	prov := provider.NewStaticForTest()

	gate := make(chan struct{}) // never closed: the hang lives past cancellation
	engine, err := session.New(session.Deps{
		DB: db, Bus: b, Prov: prov, Perm: perm,
		Tools: tool.Registry(), DataDir: root, Log: logger,
		Cfg:     func(string) (*protocol.Config, error) { return &protocol.Config{}, nil },
		Drivers: map[string]llm.Driver{"kido": hangDriver{gate: gate}},
	})
	if err != nil {
		t.Fatal(err)
	}
	deps := &server.Deps{
		DB: db, Bus: b, Perm: perm, Log: logger, Prov: prov,
		Dirs:    config.Dirs{Home: root, Data: root, Cache: root},
		WorkDir: wd, Engine: engine,
	}
	srv := server.NewServer(*deps)
	if _, err := srv.Start("127.0.0.1:0"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	now := time.Now().UnixMilli()
	sid := protocol.NewID("ses")
	if err := db.CreateSession(context.Background(), storage.SessionRow{
		ID: sid, ProjectDir: wd, Title: "t", Model: "kido/q",
		TimeCreated: now, TimeUpdated: now,
	}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := engine.Send(ctx, sid, "hang", func(error) {}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for engine.Status(sid) != "busy" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if engine.Status(sid) != "busy" {
		t.Fatal("turn never went busy")
	}

	// First signal: serveCmd's main body reads it, the drain starts with
	// its 5 s budget, and the force-kill arm blocks on the next signal.
	stop := make(chan os.Signal, 2)
	dctx, dcancel := context.WithTimeout(context.Background(), 5*time.Second)
	armForceKill(logger, stop, dcancel)
	stop <- syscall.SIGTERM
	<-stop // the first-signal read
	done := make(chan struct{})
	go func() {
		drainCtx(deps, srv, dctx)
		close(done)
	}()
	time.Sleep(300 * time.Millisecond) // the drain is now waiting on the hung turn

	// Second signal: force-kill cancels the drain ctx immediately.
	stop <- syscall.SIGTERM
	at := time.Now()
	select {
	case <-done:
		if el := time.Since(at); el > 2*time.Second {
			t.Fatalf("drain took %v after the force-kill, want < 2s (pre-fix: full 5s budget)", el)
		}
	case <-time.After(7 * time.Second):
		t.Fatal("drain did not end after the force-kill cancel")
	}
}

// hangDriver parks a turn in Stream without honoring ctx: a provider call
// that hangs past cancellation (the live scenario force-kill protects).
type hangDriver struct{ gate chan struct{} }

func (h hangDriver) Stream(ctx context.Context, req llm.Request) (llm.PartStream, error) {
	<-h.gate
	return llm.PartStream{}, nil
}

// TestAuthAddRejectsUnreadableOrEmptyKey pins Y (error-6/cli-3): a
// ReadString failure or an empty-after-trim key is an error + exit 1 with
// nothing persisted; a valid key still persists.
func TestAuthAddRejectsUnreadableOrEmptyKey(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))

	withStdio := func(t *testing.T, stdin string, stdinClosed bool, fn func()) (string, string) {
		t.Helper()
		oldIn, oldOut, oldErr := os.Stdin, os.Stdout, os.Stderr
		t.Cleanup(func() { os.Stdin, os.Stdout, os.Stderr = oldIn, oldOut, oldErr })
		if stdin != "" || stdinClosed {
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			if stdin != "" {
				_, _ = w.WriteString(stdin)
			}
			_ = w.Close()
			os.Stdin = r
		}
		outR, outW, _ := os.Pipe()
		errR, errW, _ := os.Pipe()
		os.Stdout, os.Stderr = outW, errW
		fn()
		_ = outW.Close()
		_ = errW.Close()
		outB, _ := io.ReadAll(outR)
		errB, _ := io.ReadAll(errR)
		return string(outB), string(errB)
	}

	readStore := func() map[string]string {
		p, err := auth.Path()
		if err != nil {
			t.Fatalf("auth.Path: %v", err)
		}
		s, err := auth.LoadFrom(p)
		if err != nil {
			t.Fatalf("load store: %v", err)
		}
		out := map[string]string{}
		for id, e := range s {
			out[id] = e.Key
		}
		return out
	}

	t.Run("closed stdin: read error, nothing persisted", func(t *testing.T) {
		var code int
		_, stderr := withStdio(t, "", true, func() { code = authCmd([]string{"add", "acme"}) })
		if code != 1 {
			t.Fatalf("exit = %d, want 1", code)
		}
		if !strings.Contains(stderr, "auth add:") {
			t.Fatalf("stderr = %q, want an auth add: error line", stderr)
		}
		if got := readStore(); len(got) != 0 {
			t.Fatalf("store = %v, want empty (nothing persisted)", got)
		}
	})
	t.Run("empty line: trimmed empty, nothing persisted", func(t *testing.T) {
		var code int
		_, stderr := withStdio(t, "\n", false, func() { code = authCmd([]string{"add", "acme"}) })
		if code != 1 || !strings.Contains(stderr, "auth add:") {
			t.Fatalf("exit = %d stderr = %q, want 1 + auth add: line", code, stderr)
		}
		if got := readStore(); len(got) != 0 {
			t.Fatalf("store = %v, want empty", got)
		}
	})
	t.Run("whitespace-only argument rejected", func(t *testing.T) {
		var code int
		_, _ = withStdio(t, "", false, func() { code = authCmd([]string{"add", "acme", "   "}) })
		if code != 1 {
			t.Fatalf("exit = %d, want 1", code)
		}
		if got := readStore(); len(got) != 0 {
			t.Fatalf("store = %v, want empty", got)
		}
	})
	t.Run("valid stdin key still persists", func(t *testing.T) {
		var code int
		_, _ = withStdio(t, "sk-abc\n", false, func() { code = authCmd([]string{"add", "acme"}) })
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		if got := readStore(); got["acme"] != "sk-abc" {
			t.Fatalf("store = %v, want acme=sk-abc", got)
		}
	})
}

// TestHelpToStdout pins Z (cli-6): yolo help prints the usage to stdout,
// so pipes and capture scripts see it; stderr stays empty.
func TestHelpToStdout(t *testing.T) {
	bin := buildBinary(t)
	for _, arg := range []string{"help", "-h", "--help"} {
		t.Run(arg, func(t *testing.T) {
			cmd := exec.Command(bin, arg)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("%s exited: %v\nstdout: %s\nstderr: %s", arg, err, stdout.String(), stderr.String())
			}
			if !strings.Contains(stdout.String(), "Usage:") {
				t.Fatalf("%s: stdout missing the usage:\n%s", arg, stdout.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("%s: stderr must be empty, got %q", arg, stderr.String())
			}
		})
	}
}

// TestRejectUnexpectedPositionals pins AA (cli-7): serve and auth reject
// unexpected positional args with usage + exit 2, matching tuiCmd.
// Assertions are on exit codes (the usage text is covered by TestAuthCmd).
func TestRejectUnexpectedPositionals(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))

	if code := run([]string{"auth", "list", "x"}); code != 2 {
		t.Fatalf("auth list x exit = %d, want 2", code)
	}
	if code := run([]string{"auth", "remove", "a", "b"}); code != 2 {
		t.Fatalf("auth remove a b exit = %d, want 2", code)
	}
	if code := run([]string{"auth", "add", "a", "b", "c"}); code != 2 {
		t.Fatalf("auth add a b c exit = %d, want 2", code)
	}
	if code := run([]string{"auth", "list"}); code != 0 {
		t.Fatalf("auth list (valid) exit = %d, want 0", code)
	}

	// serve: subprocess — pre-fix it would start serving and block on the
	// signal channel, so the 10 s watchdog proves the rejection path.
	bin := buildBinary(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv := exec.CommandContext(ctx, bin, "serve", "junk")
	env := make([]string, 0, len(os.Environ())+5)
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "XDG_") {
			continue
		}
		env = append(env, kv)
	}
	script := filepath.Join(root, "script.json")
	_ = os.WriteFile(script, []byte(`[{"parts":[{"kind":"text","text":"ok","finish":"stop","usage":{"input":1,"output":1}}]}`), 0o644)
	env = append(env,
		"YOLO_LLM=fake", "YOLO_FAKE_SCRIPT="+script,
		"XDG_CONFIG_HOME="+filepath.Join(root, "config"),
		"XDG_DATA_HOME="+filepath.Join(root, "data"),
		"XDG_CACHE_HOME="+filepath.Join(root, "cache"),
	)
	srv.Env = env
	err := srv.Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 2 {
		t.Fatalf("serve junk exit = %v, want exit code 2 (pre-fix: it starts serving and is killed by the watchdog)", err)
	}
}

// TestProfileCmd pins the yolo profile CLI surface: subcommand dispatch,
// usage and exit codes, add/use/remove/copy semantics, and the list output
// (active marker, id + name + description columns). State is isolated to a
// temp XDG config dir; stdout/stderr are swapped per case.
func TestProfileCmd(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))

	runProfile := func(t *testing.T, args ...string) (int, string, string) {
		t.Helper()
		oldOut, oldErr := os.Stdout, os.Stderr
		t.Cleanup(func() { os.Stdout, os.Stderr = oldOut, oldErr })
		outR, outW, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		errR, errW, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		os.Stdout, os.Stderr = outW, errW
		code := profileCmd(args)
		_ = outW.Close()
		_ = errW.Close()
		outB, _ := io.ReadAll(outR)
		errB, _ := io.ReadAll(errR)
		return code, string(outB), string(errB)
	}

	t.Run("no subcommand: usage, exit 2", func(t *testing.T) {
		code, _, errOut := runProfile(t)
		if code != 2 || !strings.Contains(errOut, "Usage:") {
			t.Fatalf("code = %d stderr = %q, want usage + exit 2", code, errOut)
		}
	})

	t.Run("unknown subcommand: usage, exit 2", func(t *testing.T) {
		code, _, errOut := runProfile(t, "bogus")
		if code != 2 || !strings.Contains(errOut, "Usage:") {
			t.Fatalf("code = %d stderr = %q, want usage + exit 2", code, errOut)
		}
	})

	t.Run("list on fresh root creates and shows default", func(t *testing.T) {
		code, out, _ := runProfile(t, "list")
		if code != 0 {
			t.Fatalf("code = %d", code)
		}
		if !strings.Contains(out, "* default  default") {
			t.Fatalf("list = %q, want * default  default", out)
		}
	})

	t.Run("add with name and description", func(t *testing.T) {
		code, out, errOut := runProfile(t, "add", "work", "-d", "work laptop")
		if code != 0 {
			t.Fatalf("code = %d stderr = %q", code, errOut)
		}
		if !regexp.MustCompile(`^[0-9a-f]{8}  work$`).MatchString(strings.TrimSpace(out)) {
			t.Fatalf("add output = %q, want '<id>  work'", out)
		}
		_, listOut, _ := runProfile(t, "list")
		if !strings.Contains(listOut, "work  work laptop") {
			t.Fatalf("list missing name + description:\n%s", listOut)
		}
	})

	t.Run("add duplicate name: exit 1", func(t *testing.T) {
		code, _, errOut := runProfile(t, "add", "work")
		if code != 1 || !strings.Contains(errOut, "already in use") {
			t.Fatalf("code = %d stderr = %q, want name-taken error", code, errOut)
		}
	})

	t.Run("use by name switches the active profile", func(t *testing.T) {
		code, out, errOut := runProfile(t, "use", "work")
		if code != 0 {
			t.Fatalf("code = %d stderr = %q", code, errOut)
		}
		if !regexp.MustCompile(`^[0-9a-f]{8}$`).MatchString(strings.TrimSpace(out)) {
			t.Fatalf("use output = %q, want the resolved id", out)
		}
		_, listOut, _ := runProfile(t, "list")
		for _, line := range strings.Split(listOut, "\n") {
			if strings.Contains(line, "work") && !strings.HasPrefix(line, "* ") {
				t.Fatalf("work profile not marked active:\n%s", listOut)
			}
		}
		if !strings.Contains(listOut, "* ") {
			t.Fatalf("no active marker in list:\n%s", listOut)
		}
	})

	t.Run("use missing profile: exit 1 with available list", func(t *testing.T) {
		code, _, errOut := runProfile(t, "use", "nope")
		if code != 1 || !strings.Contains(errOut, "not found") {
			t.Fatalf("code = %d stderr = %q", code, errOut)
		}
		if !strings.Contains(errOut, "available") {
			t.Fatalf("stderr missing available-profiles hint:\n%s", errOut)
		}
	})

	t.Run("copy creates a new profile with the given name", func(t *testing.T) {
		code, out, errOut := runProfile(t, "copy", "work", "work-home", "-d", "home copy")
		if code != 0 {
			t.Fatalf("code = %d stderr = %q", code, errOut)
		}
		if !strings.Contains(strings.TrimSpace(out), "work-home") {
			t.Fatalf("copy output = %q, want the new name", out)
		}
		// seed a model into the source profile, then verify the copy
		// carries it over
		profRoot := filepath.Join(root, "config", "yolo")
		entries, err := os.ReadDir(profRoot)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			p := filepath.Join(profRoot, e.Name(), "yolo.jsonc")
			b, err := os.ReadFile(p)
			if err != nil || !strings.Contains(string(b), `"work"`) {
				continue
			}
			var m map[string]any
			if err := json.Unmarshal(b, &m); err != nil {
				t.Fatal(err)
			}
			m["model"] = "m-work"
			b, _ = json.Marshal(m)
			if err := os.WriteFile(p, b, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		_, _, _ = runProfile(t, "copy", "work", "work-2")
		found := false
		entries, err = os.ReadDir(profRoot)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			b, err := os.ReadFile(filepath.Join(profRoot, e.Name(), "yolo.jsonc"))
			if err != nil {
				continue
			}
			if strings.Contains(string(b), `"work-2"`) && strings.Contains(string(b), `"m-work"`) {
				found = true
			}
		}
		if !found {
			t.Fatal("no profile dir holds the copy (name work-2 + source model m-work)")
		}
	})

	t.Run("copy name colliding with source: exit 1", func(t *testing.T) {
		code, _, errOut := runProfile(t, "copy", "work", "work")
		if code != 1 || !strings.Contains(errOut, "already in use") {
			t.Fatalf("code = %d stderr = %q, want name-taken error", code, errOut)
		}
	})

	t.Run("copy with missing source: exit 1", func(t *testing.T) {
		code, _, errOut := runProfile(t, "copy", "nope", "x")
		if code != 1 || !strings.Contains(errOut, "not found") {
			t.Fatalf("code = %d stderr = %q", code, errOut)
		}
	})

	t.Run("remove the active profile falls back to the remaining one", func(t *testing.T) {
		_, _, _ = runProfile(t, "add", "personal")
		code, _, errOut := runProfile(t, "remove", "work")
		if code != 0 {
			t.Fatalf("code = %d stderr = %q", code, errOut)
		}
		_, listOut, _ := runProfile(t, "list")
		if !strings.Contains(listOut, "* default  default") && !strings.Contains(listOut, "* personal  personal") {
			t.Fatalf("active marker not on a remaining profile:\n%s", listOut)
		}
		if strings.Contains(listOut, "  work  ") {
			t.Fatalf("work profile still listed:\n%s", listOut)
		}
	})

	t.Run("remove missing profile: exit 1", func(t *testing.T) {
		code, _, errOut := runProfile(t, "remove", "nope")
		if code != 1 || !strings.Contains(errOut, "not found") {
			t.Fatalf("code = %d stderr = %q", code, errOut)
		}
	})
}

// TestProfileEditCmd pins the yolo profile edit surface: -n/-d flag
// presence semantics (absent != empty: an empty value clears the field),
// id and name references, the usage/not-found/name-taken exits, and the
// list output after an edit. State is isolated to a temp XDG config dir;
// stdout/stderr are swapped per case.
func TestProfileEditCmd(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))

	runEdit := func(t *testing.T, args ...string) (int, string, string) {
		t.Helper()
		oldOut, oldErr := os.Stdout, os.Stderr
		t.Cleanup(func() { os.Stdout, os.Stderr = oldOut, oldErr })
		outR, outW, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		errR, errW, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		os.Stdout, os.Stderr = outW, errW
		code := profileCmd(args)
		_ = outW.Close()
		_ = errW.Close()
		outB, _ := io.ReadAll(outR)
		errB, _ := io.ReadAll(errR)
		return code, string(outB), string(errB)
	}

	var id string

	t.Run("seed: add work with description", func(t *testing.T) {
		code, out, errOut := runEdit(t, "add", "work", "-d", "work laptop")
		if code != 0 {
			t.Fatalf("add code = %d stderr = %q", code, errOut)
		}
		m := regexp.MustCompile(`^([0-9a-f]{8})  work$`).FindStringSubmatch(strings.TrimSpace(out))
		if m == nil {
			t.Fatalf("add output = %q, want '<id>  work'", out)
		}
		id = m[1]
	})

	t.Run("edit by id sets name and description", func(t *testing.T) {
		code, out, errOut := runEdit(t, "edit", id, "-n", "work2", "-d", "renamed desc")
		if code != 0 {
			t.Fatalf("code = %d stderr = %q", code, errOut)
		}
		if strings.TrimSpace(out) != id+"  work2" {
			t.Fatalf("edit output = %q, want %q", out, id+"  work2")
		}
		_, listOut, _ := runEdit(t, "list")
		if !strings.Contains(listOut, "work2  renamed desc") {
			t.Fatalf("list missing name + description:\n%s", listOut)
		}
	})

	t.Run("edit by name reference", func(t *testing.T) {
		code, out, errOut := runEdit(t, "edit", "work2", "-n", "work3")
		if code != 0 {
			t.Fatalf("code = %d stderr = %q", code, errOut)
		}
		if strings.TrimSpace(out) != id+"  work3" {
			t.Fatalf("edit output = %q, want %q", out, id+"  work3")
		}
		_, listOut, _ := runEdit(t, "list")
		if !strings.Contains(listOut, "work3  renamed desc") {
			t.Fatalf("list missing name with kept description:\n%s", listOut)
		}
	})

	t.Run("edit with no flags: usage, exit 2", func(t *testing.T) {
		code, _, errOut := runEdit(t, "edit", "work3")
		if code != 2 || !strings.Contains(errOut, "Usage:") {
			t.Fatalf("code = %d stderr = %q, want usage + exit 2", code, errOut)
		}
	})

	t.Run("edit nonexistent: exit 1 with not-found hint", func(t *testing.T) {
		code, _, errOut := runEdit(t, "edit", "nope", "-n", "x")
		if code != 1 || !strings.Contains(errOut, "not found") {
			t.Fatalf("code = %d stderr = %q", code, errOut)
		}
		if !strings.Contains(errOut, "available") {
			t.Fatalf("stderr missing available-profiles hint:\n%s", errOut)
		}
	})

	t.Run("edit name taken by another profile: exit 1", func(t *testing.T) {
		code, _, errOut := runEdit(t, "add", "other")
		if code != 0 {
			t.Fatalf("seed add code = %d stderr = %q", code, errOut)
		}
		code, _, errOut = runEdit(t, "edit", "work3", "-n", "other")
		if code != 1 || !strings.Contains(errOut, "already in use") {
			t.Fatalf("code = %d stderr = %q, want name-taken error", code, errOut)
		}
	})

	t.Run("equals-forms --name=X and --description=Y", func(t *testing.T) {
		code, out, errOut := runEdit(t, "edit", "work3", "--name=final", "--description=final desc")
		if code != 0 {
			t.Fatalf("code = %d stderr = %q", code, errOut)
		}
		if strings.TrimSpace(out) != id+"  final" {
			t.Fatalf("edit output = %q, want %q", out, id+"  final")
		}
		_, listOut, _ := runEdit(t, "list")
		if !strings.Contains(listOut, "final  final desc") {
			t.Fatalf("list missing name + description:\n%s", listOut)
		}
	})

	t.Run("extra positional: usage, exit 2", func(t *testing.T) {
		code, _, errOut := runEdit(t, "edit", "final", "extra")
		if code != 2 || !strings.Contains(errOut, "Usage:") {
			t.Fatalf("code = %d stderr = %q, want usage + exit 2", code, errOut)
		}
	})
}
