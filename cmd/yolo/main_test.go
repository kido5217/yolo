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

var (
	// profileIDRe matches a bare profile id (8 lowercase hex chars).
	profileIDRe = regexp.MustCompile(`^[0-9a-f]{8}$`)
	// profileAddOutRe matches the `yolo profile add` success line
	// "<id>  work".
	profileAddOutRe = regexp.MustCompile(`^[0-9a-f]{8}  work$`)
)

// buildBinary compiles the yolo binary for subprocess tests.
func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "yolo")
	cmd := exec.Command(
		"go",
		"build",
		"-o",
		bin,
		".",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

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
		{name: "serve -h exits 0", args: []string{"serve", "-h"}, wantCode: 0, wantPart: "Usage:"},
		{name: "-h after a flag exits 0", args: []string{"--dir", "/nonexistent", "-h"}, wantCode: 0, wantPart: "Usage:"},
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
	if err := os.WriteFile(script, []byte(fakeScriptOK), 0o644); err != nil {
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
// idle), closes the listener gracefully, and finishes well inside the drain
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

// TestServeSigtermDrainsAndExitsZero pins the process-level contract
// SIGINT/SIGTERM -> exit 0: a serving `yolo serve` gets SIGTERM, exits 0,
// and does so within the drain budget.
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
	perm := permission.New(
		db,
		b,
		logger,
		root,
	)
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
	for engine.Status(sid) != protocol.SessionStatusBusy && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if engine.Status(sid) != protocol.SessionStatusBusy {
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
				missing := i < 0 || j < 0
				if missing || i > j {
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

	// resetStore drops the store file so each subtest starts from a clean
	// slate (independent of what the previous subtest left behind).
	resetStore := func(t *testing.T) {
		t.Helper()
		if p, err := auth.Path(); err == nil {
			_ = os.Remove(p)
		}
	}

	t.Run("closed stdin: read error, nothing persisted", func(t *testing.T) {
		resetStore(t)
		var code int
		_, stderr := withStdio(
			t,
			"",
			true,
			func() { code = authCmd([]string{"add", "acme"}) },
		)
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
		resetStore(t)
		var code int
		_, stderr := withStdio(
			t,
			"\n",
			false,
			func() { code = authCmd([]string{"add", "acme"}) },
		)
		if code != 1 || !strings.Contains(stderr, "auth add:") {
			t.Fatalf("exit = %d stderr = %q, want 1 + auth add: line", code, stderr)
		}
		if got := readStore(); len(got) != 0 {
			t.Fatalf("store = %v, want empty", got)
		}
	})
	t.Run("whitespace-only argument rejected", func(t *testing.T) {
		resetStore(t)
		var code int
		_, _ = withStdio(
			t,
			"",
			false,
			func() { code = authCmd([]string{"add", "acme", "   "}) },
		)
		if code != 1 {
			t.Fatalf("exit = %d, want 1", code)
		}
		if got := readStore(); len(got) != 0 {
			t.Fatalf("store = %v, want empty", got)
		}
	})
	t.Run("valid stdin key still persists", func(t *testing.T) {
		resetStore(t)
		var code int
		_, _ = withStdio(
			t,
			"sk-abc\n",
			false,
			func() { code = authCmd([]string{"add", "acme"}) },
		)
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		if got := readStore(); got["acme"] != "sk-abc" {
			t.Fatalf("store = %v, want acme=sk-abc", got)
		}
	})
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
	_ = os.WriteFile(script, []byte(fakeScriptOK), 0o644)
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

// runProfile runs profileCmd with the process stdout/stderr swapped for
// pipes and returns (exit code, stdout, stderr).
func runProfile(t *testing.T, args ...string) (int, string, string) {
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

// withConfigHome points XDG_CONFIG_HOME at a fresh temp dir for the calling
// (sub)test and returns the config home (the yolo config root is
// <configHome>/yolo).
func withConfigHome(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "config")
	t.Setenv("XDG_CONFIG_HOME", home)
	return home
}

// testProfile is one seeded profile for the profile-command tests: id is
// the profile dir name, name/desc/model are optional config fields, and
// isActive writes the id to the active marker.
type testProfile struct {
	id       string
	name     string
	desc     string
	model    string
	isActive bool
}

// seedProfiles writes each profile dir <configHome>/yolo/<id>/yolo.jsonc
// and, for every active profile, the active marker (last one wins).
func seedProfiles(t *testing.T, configHome string, profiles ...testProfile) {
	t.Helper()
	for _, p := range profiles {
		m := map[string]any{}
		if p.name != "" || p.desc != "" {
			e := map[string]any{}
			if p.name != "" {
				e["name"] = p.name
			}
			if p.desc != "" {
				e["description"] = p.desc
			}
			m["profile"] = e
		}
		if p.model != "" {
			m["model"] = p.model
		}
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(configHome, "yolo", p.id, "yolo.jsonc")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		if p.isActive {
			if err := os.WriteFile(filepath.Join(configHome, "yolo", "active"), []byte(p.id+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// TestProfileUsage pins the yolo profile usage contract: a missing or
// unknown subcommand prints the usage to stderr and exits 2.
func TestProfileUsage(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "no subcommand"},
		{name: "unknown subcommand", args: []string{"bogus"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			withConfigHome(t)
			code, _, errOut := runProfile(t, tc.args...)
			if code != 2 || !strings.Contains(errOut, "Usage:") {
				t.Fatalf("code = %d stderr = %q, want usage + exit 2", code, errOut)
			}
		})
	}
}

// TestProfileList pins `yolo profile list` on a fresh root: the first run
// creates the default profile and marks it active.
func TestProfileList(t *testing.T) {
	t.Run("fresh root creates and shows default", func(t *testing.T) {
		withConfigHome(t)
		code, out, errOut := runProfile(t, "list")
		if code != 0 {
			t.Fatalf("code = %d stderr = %q", code, errOut)
		}
		if !strings.Contains(out, "* default  default") {
			t.Fatalf("list = %q, want * default  default", out)
		}
	})
}

// TestProfileAdd pins `yolo profile add`: the success output (id + name),
// the list output (name + description), the name-taken error, and the
// dangling -d flag rejection.
func TestProfileAdd(t *testing.T) {
	t.Run("add with name and description", func(t *testing.T) {
		withConfigHome(t)
		code, out, errOut := runProfile(
			t,
			"add",
			"work",
			"-d",
			"work laptop",
		)
		if code != 0 {
			t.Fatalf("code = %d stderr = %q", code, errOut)
		}
		if !profileAddOutRe.MatchString(strings.TrimSpace(out)) {
			t.Fatalf("add output = %q, want '<id>  work'", out)
		}
		_, listOut, _ := runProfile(t, "list")
		if !strings.Contains(listOut, "work  work laptop") {
			t.Fatalf("list missing name + description:\n%s", listOut)
		}
	})
	t.Run("add duplicate name: exit 1", func(t *testing.T) {
		ch := withConfigHome(t)
		seedProfiles(t, ch, testProfile{id: "aaaa1111", name: "work", isActive: true})
		code, _, errOut := runProfile(t, "add", "work")
		if code != 1 || !strings.Contains(errOut, "already in use") {
			t.Fatalf("code = %d stderr = %q, want name-taken error", code, errOut)
		}
	})
	t.Run("dangling -d flag: usage, exit 2", func(t *testing.T) {
		withConfigHome(t)
		code, _, errOut := runProfile(
			t,
			"add",
			"work",
			"-d",
		)
		if code != 2 || !strings.Contains(errOut, "Usage:") {
			t.Fatalf("code = %d stderr = %q, want usage + exit 2", code, errOut)
		}
	})
}

// TestProfileUse pins `yolo profile use`: name resolution, the active
// marker switch (visible in list), and the not-found error with the
// available-profiles hint.
func TestProfileUse(t *testing.T) {
	t.Run("use by name switches the active profile", func(t *testing.T) {
		ch := withConfigHome(t)
		seedProfiles(t, ch, testProfile{id: "aaaa1111", name: "work"})
		code, out, errOut := runProfile(t, "use", "work")
		if code != 0 {
			t.Fatalf("code = %d stderr = %q", code, errOut)
		}
		if !profileIDRe.MatchString(strings.TrimSpace(out)) {
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
		withConfigHome(t)
		code, _, errOut := runProfile(t, "use", "nope")
		if code != 1 || !strings.Contains(errOut, "not found") {
			t.Fatalf("code = %d stderr = %q", code, errOut)
		}
		if !strings.Contains(errOut, "available") {
			t.Fatalf("stderr missing available-profiles hint:\n%s", errOut)
		}
	})
}

// TestProfileCopy pins `yolo profile copy`: a new profile carries the
// source model and gets the given name; a colliding name and a missing
// source are errors.
func TestProfileCopy(t *testing.T) {
	t.Run("copy creates a new profile carrying the source model", func(t *testing.T) {
		ch := withConfigHome(t)
		seedProfiles(t, ch, testProfile{id: "aaaa1111", name: "work", model: "m-work"})
		code, out, errOut := runProfile(
			t,
			"copy",
			"work",
			"work-home",
			"-d",
			"home copy",
		)
		if code != 0 {
			t.Fatalf("code = %d stderr = %q", code, errOut)
		}
		if !strings.Contains(strings.TrimSpace(out), "work-home") {
			t.Fatalf("copy output = %q, want the new name", out)
		}
		profRoot := filepath.Join(ch, "yolo")
		entries, err := os.ReadDir(profRoot)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			b, err := os.ReadFile(filepath.Join(profRoot, e.Name(), "yolo.jsonc"))
			if err != nil {
				continue
			}
			if strings.Contains(string(b), `"work-home"`) && strings.Contains(string(b), `"m-work"`) {
				found = true
			}
		}
		if !found {
			t.Fatal("no profile dir holds the copy (name work-home + source model m-work)")
		}
	})
	t.Run("copy name colliding with source: exit 1", func(t *testing.T) {
		ch := withConfigHome(t)
		seedProfiles(t, ch, testProfile{id: "aaaa1111", name: "work"})
		code, _, errOut := runProfile(
			t,
			"copy",
			"work",
			"work",
		)
		if code != 1 || !strings.Contains(errOut, "already in use") {
			t.Fatalf("code = %d stderr = %q, want name-taken error", code, errOut)
		}
	})
	t.Run("copy with missing source: exit 1", func(t *testing.T) {
		withConfigHome(t)
		code, _, errOut := runProfile(
			t,
			"copy",
			"nope",
			"x",
		)
		if code != 1 || !strings.Contains(errOut, "not found") {
			t.Fatalf("code = %d stderr = %q", code, errOut)
		}
	})
}

// TestProfileRemove pins `yolo profile remove`: removing the active
// profile moves the marker to a remaining profile; a missing profile is
// an error.
func TestProfileRemove(t *testing.T) {
	t.Run("remove the active profile falls back to the remaining one", func(t *testing.T) {
		ch := withConfigHome(t)
		seedProfiles(
			t,
			ch,
			testProfile{id: "aaaa1111", name: "work", isActive: true},
			testProfile{id: "bbbb2222", name: "personal"},
		)
		code, _, errOut := runProfile(t, "remove", "work")
		if code != 0 {
			t.Fatalf("code = %d stderr = %q", code, errOut)
		}
		_, listOut, _ := runProfile(t, "list")
		if !strings.Contains(listOut, "* bbbb2222  personal") {
			t.Fatalf("active marker not on a remaining profile:\n%s", listOut)
		}
		if strings.Contains(listOut, "aaaa1111") {
			t.Fatalf("work profile still listed:\n%s", listOut)
		}
	})
	t.Run("remove missing profile: exit 1", func(t *testing.T) {
		withConfigHome(t)
		code, _, errOut := runProfile(t, "remove", "nope")
		if code != 1 || !strings.Contains(errOut, "not found") {
			t.Fatalf("code = %d stderr = %q", code, errOut)
		}
	})
}

// TestProfileEdit pins the yolo profile edit surface: -n/-d flag presence
// semantics (absent != empty), id and name references, the
// usage/not-found/name-taken/dangling-flag exits, and the list output after
// an edit. Each subtest seeds its own state, so the subtests run
// independently.
func TestProfileEdit(t *testing.T) {
	t.Run("edit by id sets name and description", func(t *testing.T) {
		ch := withConfigHome(t)
		seedProfiles(t, ch, testProfile{id: "aaaa1111", name: "work", desc: "work laptop"})
		code, out, errOut := runProfile(
			t,
			"edit",
			"aaaa1111",
			"-n",
			"work2",
			"-d",
			"renamed desc",
		)
		if code != 0 {
			t.Fatalf("code = %d stderr = %q", code, errOut)
		}
		if strings.TrimSpace(out) != "aaaa1111  work2" {
			t.Fatalf("edit output = %q, want \"aaaa1111  work2\"", out)
		}
		_, listOut, _ := runProfile(t, "list")
		if !strings.Contains(listOut, "work2  renamed desc") {
			t.Fatalf("list missing name + description:\n%s", listOut)
		}
	})
	t.Run("edit by name reference", func(t *testing.T) {
		ch := withConfigHome(t)
		seedProfiles(t, ch, testProfile{id: "aaaa1111", name: "work", desc: "work laptop"})
		code, out, errOut := runProfile(
			t,
			"edit",
			"work",
			"-n",
			"work3",
		)
		if code != 0 {
			t.Fatalf("code = %d stderr = %q", code, errOut)
		}
		if strings.TrimSpace(out) != "aaaa1111  work3" {
			t.Fatalf("edit output = %q, want \"aaaa1111  work3\"", out)
		}
		_, listOut, _ := runProfile(t, "list")
		if !strings.Contains(listOut, "work3  work laptop") {
			t.Fatalf("list missing name with kept description:\n%s", listOut)
		}
	})
	t.Run("edit with no flags: usage, exit 2", func(t *testing.T) {
		ch := withConfigHome(t)
		seedProfiles(t, ch, testProfile{id: "aaaa1111", name: "work"})
		code, _, errOut := runProfile(t, "edit", "work")
		if code != 2 || !strings.Contains(errOut, "Usage:") {
			t.Fatalf("code = %d stderr = %q, want usage + exit 2", code, errOut)
		}
	})
	t.Run("edit nonexistent: exit 1 with not-found hint", func(t *testing.T) {
		withConfigHome(t)
		code, _, errOut := runProfile(
			t,
			"edit",
			"nope",
			"-n",
			"x",
		)
		if code != 1 || !strings.Contains(errOut, "not found") {
			t.Fatalf("code = %d stderr = %q", code, errOut)
		}
		if !strings.Contains(errOut, "available") {
			t.Fatalf("stderr missing available-profiles hint:\n%s", errOut)
		}
	})
	t.Run("edit name taken by another profile: exit 1", func(t *testing.T) {
		ch := withConfigHome(t)
		seedProfiles(
			t,
			ch,
			testProfile{id: "aaaa1111", name: "work"},
			testProfile{id: "bbbb2222", name: "other"},
		)
		code, _, errOut := runProfile(
			t,
			"edit",
			"work",
			"-n",
			"other",
		)
		if code != 1 || !strings.Contains(errOut, "already in use") {
			t.Fatalf("code = %d stderr = %q, want name-taken error", code, errOut)
		}
	})
	t.Run("equals-forms --name=X and --description=Y", func(t *testing.T) {
		ch := withConfigHome(t)
		seedProfiles(t, ch, testProfile{id: "aaaa1111", name: "work", desc: "work laptop"})
		code, out, errOut := runProfile(
			t,
			"edit",
			"work",
			"--name=final",
			"--description=final desc",
		)
		if code != 0 {
			t.Fatalf("code = %d stderr = %q", code, errOut)
		}
		if strings.TrimSpace(out) != "aaaa1111  final" {
			t.Fatalf("edit output = %q, want \"aaaa1111  final\"", out)
		}
		_, listOut, _ := runProfile(t, "list")
		if !strings.Contains(listOut, "final  final desc") {
			t.Fatalf("list missing name + description:\n%s", listOut)
		}
	})
	t.Run("extra positional: usage, exit 2", func(t *testing.T) {
		ch := withConfigHome(t)
		seedProfiles(t, ch, testProfile{id: "aaaa1111", name: "work"})
		code, _, errOut := runProfile(
			t,
			"edit",
			"work",
			"extra",
		)
		if code != 2 || !strings.Contains(errOut, "Usage:") {
			t.Fatalf("code = %d stderr = %q, want usage + exit 2", code, errOut)
		}
	})
	t.Run("dangling -n flag: usage, exit 2", func(t *testing.T) {
		ch := withConfigHome(t)
		seedProfiles(t, ch, testProfile{id: "aaaa1111", name: "work"})
		code, _, errOut := runProfile(t, "edit", "work", "-n")
		if code != 2 || !strings.Contains(errOut, "Usage:") {
			t.Fatalf("code = %d stderr = %q, want usage + exit 2", code, errOut)
		}
	})
}
