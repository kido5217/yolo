package main

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/server"
	"github.com/kido5217/yolo/internal/storage"
	"github.com/kido5217/yolo/internal/tui/client"
)

// captureRun runs run(args) with stdout/stderr swapped for pipes and
// stdin pointed at /dev/null (composeRunMessage must not block on the
// test process's inherited stdin), returning (exit code, stdout,
// stderr).
func captureRun(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	oldOut, oldErr, oldIn := os.Stdout, os.Stderr, os.Stdin
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.Stdout, os.Stderr, os.Stdin = oldOut, oldErr, oldIn
		_ = devNull.Close()
	})
	outR, outW, _ := os.Pipe()
	errR, errW, _ := os.Pipe()
	os.Stdout, os.Stderr, os.Stdin = outW, errW, devNull
	code := run(args)
	_ = outW.Close()
	_ = errW.Close()
	outB, _ := io.ReadAll(outR)
	errB, _ := io.ReadAll(errR)
	return code, string(outB), string(errB)
}

// runEnv points the XDG roots at a fresh temp dir and arms the
// fake-driver env (fakeScriptOK); it returns the data root and a fresh
// workdir. Integration legs set the env before captureRun.
func runEnv(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	wd := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	t.Setenv("YOLO_LLM", "fake")
	script := filepath.Join(root, "script.json")
	if err := os.WriteFile(script, []byte(fakeScriptOK), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("YOLO_FAKE_SCRIPT", script)
	return filepath.Join(root, "data"), wd
}

// TestResolveFiles pins the §3 file-validation table (leg a): resolve,
// existence, regularity+size, read-once, mime, filename, data-URL bytes.
func TestResolveFiles(t *testing.T) {
	base := t.TempDir()
	hello := []byte("hello")
	helloB64 := base64.StdEncoding.EncodeToString(hello)
	bin := []byte{0x01, 0x02, 0xff, 0xfe}
	binB64 := base64.StdEncoding.EncodeToString(bin)

	t.Run("missing file", func(t *testing.T) {
		_, err := resolveFiles(base, []string{"nope.txt"})
		if err == nil || err.Error() != "File not found: nope.txt" {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("directory is a special file", func(t *testing.T) {
		sub := filepath.Join(base, "sub")
		if err := os.Mkdir(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		_, err := resolveFiles(base, []string{"sub"})
		if err == nil || err.Error() != "Cannot attach local file larger than 10 MiB or a special file: sub" {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("oversized rejected, exact max accepted", func(t *testing.T) {
		big := filepath.Join(base, "big.bin")
		if err := os.WriteFile(big, make([]byte, 10<<20+1), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := resolveFiles(base, []string{"big.bin"})
		if err == nil || err.Error() != "Cannot attach local file larger than 10 MiB or a special file: big.bin" {
			t.Fatalf("err = %v", err)
		}
		max := filepath.Join(base, "max.bin")
		maxData := make([]byte, 10<<20)
		maxData[0] = 0xff // invalid UTF-8: the exact-max file is binary, not
		// a run of valid (null) bytes that utf8.Valid would call text/plain.
		if err := os.WriteFile(max, maxData, 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := resolveFiles(base, []string{"max.bin"})
		if err != nil {
			t.Fatalf("exact-max file: %v", err)
		}
		if got[0].MIME != "application/octet-stream" || got[0].Filename != "max.bin" {
			t.Fatalf("max file = %+v", got[0])
		}
	})
	t.Run("symlink to a regular file is accepted", func(t *testing.T) {
		target := filepath.Join(base, "target.txt")
		if err := os.WriteFile(target, hello, 0o644); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(base, "link.txt")
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		got, err := resolveFiles(base, []string{"link.txt"})
		if err != nil {
			t.Fatalf("symlink: %v", err)
		}
		// stat follows the link (the target's size is checked); the
		// filename is the Base of the RESOLVED (link) path, and the URL
		// carries the target's content.
		if got[0].Filename != "link.txt" || got[0].URL != "data:text/plain;base64,"+helloB64 {
			t.Fatalf("symlink ref = %+v", got[0])
		}
	})
	t.Run("utf8 content is text/plain, pinned data URL", func(t *testing.T) {
		p := filepath.Join(base, "notes.txt")
		if err := os.WriteFile(p, hello, 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := resolveFiles(base, []string{"notes.txt"})
		if err != nil {
			t.Fatal(err)
		}
		want := protocol.FileRef{MIME: "text/plain", Filename: "notes.txt", URL: "data:text/plain;base64," + helloB64}
		if got[0] != want {
			t.Fatalf("ref = %+v, want %+v", got[0], want)
		}
	})
	t.Run("invalid utf8 is application/octet-stream, pinned data URL", func(t *testing.T) {
		p := filepath.Join(base, "bin.dat")
		if err := os.WriteFile(p, bin, 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := resolveFiles(base, []string{"bin.dat"})
		if err != nil {
			t.Fatal(err)
		}
		want := protocol.FileRef{MIME: "application/octet-stream", Filename: "bin.dat", URL: "data:application/octet-stream;base64," + binB64}
		if got[0] != want {
			t.Fatalf("ref = %+v, want %+v", got[0], want)
		}
	})
	t.Run("absolute paths as-is, flag order preserved", func(t *testing.T) {
		a := filepath.Join(base, "a.txt")
		b := filepath.Join(base, "b.txt")
		if err := os.WriteFile(a, hello, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(b, hello, 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := resolveFiles(base, []string{b, "a.txt"})
		if err != nil {
			t.Fatal(err)
		}
		if got[0].Filename != "b.txt" || got[1].Filename != "a.txt" {
			t.Fatalf("order = %+v", got)
		}
	})
}

// TestRunPreflight pins the pre-boot legs (spec §2 order + §7.3 rows):
// the exit-2 usage legs and the exit-1 file legs (the file error sits
// beside yolo's usage=2 rule by design — deviation 3).
func TestRunPreflight(t *testing.T) {
	_, wd := runEnv(t)
	big := filepath.Join(wd, "big.bin")
	if err := os.WriteFile(big, make([]byte, 10<<20+1), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		args   []string
		want   int
		stderr string
	}{
		{"missing message", []string{"run"}, 2, "yolo run: message required"},
		{"unknown format", []string{"run", "--format", "bogus", "hi"}, 2, `yolo run: unknown format value "bogus"`},
		{"--output json", []string{"run", "--output", "json", "hi"}, 2, "yolo run: --output is not supported by run"},
		{"unknown flag", []string{"run", "--nope"}, 2, "yolo run: unknown flag"},
		{"bad dir", []string{"run", "--dir", filepath.Join(wd, "nope"), "hi"}, 2, "yolo run: not a directory: " + filepath.Join(wd, "nope")},
		{"file not found", []string{"run", "hi", "--dir", wd, "--file", "missing.txt"}, 1, "yolo run: File not found: missing.txt"},
		{"file too large", []string{"run", "hi", "--dir", wd, "--file", "big.bin"}, 1, "yolo run: Cannot attach local file larger than 10 MiB or a special file: big.bin"},
		{"file error precedes the message check", []string{"run", "--dir", wd, "--file", "missing.txt"}, 1, "yolo run: File not found: missing.txt"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, out, errOut := captureRun(t, c.args...)
			if code != c.want {
				t.Fatalf("exit = %d, want %d (stderr: %s)", code, c.want, errOut)
			}
			if !strings.Contains(errOut, c.stderr) {
				t.Fatalf("stderr = %q, want it to contain %q", errOut, c.stderr)
			}
			if out != "" {
				t.Fatalf("stdout = %q, want empty (pre-flight legs print nothing on stdout)", out)
			}
		})
	}
}

// TestRunCleanTurn pins the happy path end to end (in-process boot +
// fake driver): exit 0, stdout the assistant text + the finalize
// newline, stderr the header line.
func TestRunCleanTurn(t *testing.T) {
	dataRoot, wd := runEnv(t)
	_ = dataRoot
	code, out, errOut := captureRun(t, "run", "hi", "--dir", wd)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, errOut)
	}
	if want := "ok\n"; out != want {
		t.Fatalf("stdout = %q, want %q", out, want)
	}
	if !strings.Contains(errOut, "> build · kido/q") {
		t.Fatalf("stderr missing the header:\n%s", errOut)
	}
}

// TestRunAgentFallback pins the --agent warn-and-fallback (spec §2 row
// --agent): unknown name -> the warning line, the run proceeds on the
// server default, exit 0.
func TestRunAgentFallback(t *testing.T) {
	_, wd := runEnv(t)
	code, _, errOut := captureRun(t, "run", "hi", "--dir", wd, "--agent", "nope")
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, errOut)
	}
	if want := `agent "nope" not found. Falling back to default agent`; !strings.Contains(errOut, want) {
		t.Fatalf("stderr missing %q:\n%s", want, errOut)
	}
}

// TestRunSession404 pins the --session 404 leg (spec §7.1): exit 1 +
// the pinned line.
func TestRunSession404(t *testing.T) {
	_, wd := runEnv(t)
	code, _, errOut := captureRun(t, "run", "hi", "--dir", wd, "--session", "ses_nonexistent")
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (stderr: %s)", code, errOut)
	}
	if want := "yolo run: Session not found: ses_nonexistent"; !strings.Contains(errOut, want) {
		t.Fatalf("stderr missing %q:\n%s", want, errOut)
	}
}

// seedSessions inserts session rows directly into the run's store (the
// XDG data root's DB) so the --continue/--session legs have data; it
// closes the connection before returning (the run's stack opens its own).
func seedSessions(t *testing.T, dataRoot, wd string, rows ...storage.SessionRow) {
	t.Helper()
	dbPath := filepath.Join(dataRoot, "yolo", "storage", "yolo.db")
	// The run's stack (openDB) creates this dir on boot; seedSessions runs
	// before the boot, so create it here (mirrors cmd/yolo/deps.go openDB).
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := t.Context()
	for _, r := range rows {
		if err := db.CreateSession(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
}

// TestRunContinueSelectsMostRecentlyUpdated pins the --continue rule
// (spec §2, deviation 12): the first row of GET /session (ORDER BY
// time_updated DESC) is selected; an empty list mints silently.
func TestRunContinueSelectsMostRecentlyUpdated(t *testing.T) {
	dataRoot, wd := runEnv(t)
	a, b := protocol.NewID("ses"), protocol.NewID("ses")
	seedSessions(t, dataRoot, wd,
		storage.SessionRow{ID: a, ProjectDir: wd, TimeCreated: 1, TimeUpdated: 100},
		storage.SessionRow{ID: b, ProjectDir: wd, TimeCreated: 2, TimeUpdated: 200},
	)
	code, out, errOut := captureRun(t, "run", "hi", "--dir", wd, "--continue")
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, errOut)
	}
	if out != "ok\n" {
		t.Fatalf("stdout = %q, want %q", out, "ok\n")
	}
	// the turn landed in the MORE recently updated session (b)
	db, err := storage.Open(filepath.Join(dataRoot, "yolo", "storage", "yolo.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := t.Context()
	msgsA, err := db.ListMessages(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	msgsB, err := db.ListMessages(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	// the turn is a user + assistant message pair: b (the more recently
	// updated session) holds both, a holds none.
	if len(msgsA) != 0 || len(msgsB) != 2 {
		t.Fatalf("turn landed in the wrong session: a=%d msgs, b=%d msgs", len(msgsA), len(msgsB))
	}
}

// TestRunNDJSONIntegration pins the end-to-end json mode: --format json
// -> NDJSON on stdout (step_start ... step_finish), exit 0, no stderr
// decoration (the machine channel is stdout).
func TestRunNDJSONIntegration(t *testing.T) {
	_, wd := runEnv(t)
	code, out, errOut := captureRun(t, "run", "hi", "--dir", wd, "--format", "json")
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, errOut)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected >= 2 NDJSON lines, got %d:\n%s", len(lines), out)
	}
	var first, last struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil || first.Type != "step_start" {
		t.Fatalf("first line = %s (err %v), want step_start", lines[0], err)
	}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &last); err != nil || last.Type != "step_finish" {
		t.Fatalf("last line = %s (err %v), want step_finish", lines[len(lines)-1], err)
	}
	for _, l := range lines {
		var env struct {
			Type      string `json:"type"`
			Timestamp int64  `json:"timestamp"`
			SessionID string `json:"sessionID"`
		}
		if err := json.Unmarshal([]byte(l), &env); err != nil || env.Type == "" || env.Timestamp == 0 || env.SessionID == "" {
			t.Fatalf("line %q is not a well-formed envelope: %v", l, err)
		}
	}
	if errOut != "" {
		t.Fatalf("json-mode stderr = %q, want empty (clean turn)", errOut)
	}
}

// waitForStatus polls client.Status until the session reports want.
func waitForStatus(t *testing.T, cl *client.Service, sessionID, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		st, err := cl.Status(t.Context())
		if err == nil && st[sessionID] == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("status never reached %s (last: %v, %v)", want, st, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// TestRunFirstSigint pins leg (h): the first-SIGINT handler aborts the
// in-flight turn against a REAL in-process server + fake-driver busy
// turn (the wall-clock bound asserts < 10 s, the cap — the endpoint's
// <=2 s settle makes it typically < 3 s), and returns 130.
func TestRunFirstSigint(t *testing.T) {
	root := t.TempDir()
	wd := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	t.Setenv("YOLO_LLM", "fake")
	script := filepath.Join(root, "script.json")
	// a slow turn: the stream stays open long enough to abort in flight
	data := `[{"parts":[{"kind":"text","text":"slow","finish":"stop","usage":{"input":1,"output":1}}],"delay_ms":2000}]`
	if err := os.WriteFile(script, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("YOLO_FAKE_SCRIPT", script)

	deps, closeDB := buildStack(t)
	defer closeDB()
	srv := server.NewServer(*deps)
	ln, err := srv.Start("127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer srv.Close()
	cl := client.New("http://"+ln.String(), wd)
	ctx := t.Context()
	ses, err := cl.CreateSessionWith(ctx, "", "", "")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, err := cl.SendMessage(ctx, ses.ID, protocol.SendMessageRequest{Text: "go"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	waitForStatus(t, cl, ses.ID, protocol.SessionStatusBusy)

	start := time.Now()
	code := firstSigint(cl, ses.ID, true)
	if code != 130 {
		t.Fatalf("exit code = %d, want 130", code)
	}
	if elapsed := time.Since(start); elapsed >= 10*time.Second {
		t.Fatalf("first SIGINT took %s, want < 10 s (the cap)", elapsed)
	}
	waitForStatus(t, cl, ses.ID, protocol.SessionStatusIdle) // the <=2 s settle
}

// TestRunAutoPermission pins the §7.2 policy legs (leg: --auto). The fake
// turn calls read on foo.env — a permission ask by default via the builtins'
// {read,*.env,ask} rule (bash would NOT ask: the builtins' * catch-all
// auto-allows the core actions, so read-of-*.env is the tool that actually
// reaches the bus) — and the default policy auto-rejects (the stderr note)
// while --auto answers once (no note, the tool executes).
func TestRunAutoPermission(t *testing.T) {
	toolScript := `[{"parts":[{"kind":"tool","name":"read","call_id":"c1","args":{"filePath":"foo.env"},"finish":"tool_calls"},{"kind":"text","text":"done","finish":"stop","usage":{"input":1,"output":1}}]}]`
	t.Run("default policy auto-rejects", func(t *testing.T) {
		root := t.TempDir()
		wd := t.TempDir()
		_ = os.WriteFile(filepath.Join(wd, "foo.env"), []byte("SECRET=1"), 0o644)
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
		t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
		t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
		t.Setenv("YOLO_LLM", "fake")
		script := filepath.Join(root, "script.json")
		if err := os.WriteFile(script, []byte(toolScript), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("YOLO_FAKE_SCRIPT", script)
		code, _, errOut := captureRun(t, "run", "go", "--dir", wd)
		if code != 0 {
			t.Fatalf("exit = %d, want 0 (stderr: %s)", code, errOut)
		}
		if !strings.Contains(errOut, "permission requested: read") || !strings.Contains(errOut, "; auto-rejecting") {
			t.Fatalf("stderr missing the auto-reject note:\n%s", errOut)
		}
	})
	t.Run("--auto answers once, no note", func(t *testing.T) {
		root := t.TempDir()
		wd := t.TempDir()
		_ = os.WriteFile(filepath.Join(wd, "foo.env"), []byte("SECRET=1"), 0o644)
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
		t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
		t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
		t.Setenv("YOLO_LLM", "fake")
		script := filepath.Join(root, "script.json")
		if err := os.WriteFile(script, []byte(toolScript), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("YOLO_FAKE_SCRIPT", script)
		code, out, errOut := captureRun(t, "run", "go", "--dir", wd, "--auto")
		if code != 0 {
			t.Fatalf("exit = %d, want 0 (stderr: %s)", code, errOut)
		}
		if strings.Contains(errOut, "permission requested") {
			t.Fatalf("--auto must not print the note:\n%s", errOut)
		}
		if want := "done\n"; out != want {
			t.Fatalf("stdout = %q, want %q (the tool ran, the turn completed)", out, want)
		}
	})
}

// TestRunExitCodes pins the §7.3 rows (leg i): the exit-1 runtime legs
// not yet covered (a send-side 500 via an unknown model, and a busy
// session); the exit-2 legs are pinned in TestRunPreflight (Task 8) and
// the clean-turn exit 0 in TestRunCleanTurn (Task 9).
func TestRunExitCodes(t *testing.T) {
	t.Run("unknown model: send-side failure exits 1", func(t *testing.T) {
		_, wd := runEnv(t)
		code, _, errOut := captureRun(t, "run", "hi", "--dir", wd, "--model", "kido/doesnotexist")
		if code != 1 {
			t.Fatalf("exit = %d, want 1 (stderr: %s)", code, errOut)
		}
		if !strings.HasPrefix(errOut, "yolo run: ") {
			t.Fatalf("stderr = %q, want the yolo run: prefix", errOut)
		}
	})
	t.Run("busy session: 409 exits 1", func(t *testing.T) {
		root := t.TempDir()
		wd := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
		t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
		t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
		t.Setenv("YOLO_LLM", "fake")
		script := filepath.Join(root, "script.json")
		// a slow turn holds the session busy for the second send
		data := `[{"parts":[{"kind":"text","text":"slow","finish":"stop","usage":{"input":1,"output":1}}],"delay_ms":2000}]`
		if err := os.WriteFile(script, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("YOLO_FAKE_SCRIPT", script)
		deps, closeDB := buildStack(t)
		defer closeDB()
		srv := server.NewServer(*deps)
		ln, err := srv.Start("127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		defer srv.Close()
		cl := client.New("http://"+ln.String(), wd)
		ctx := t.Context()
		ses, err := cl.CreateSessionWith(ctx, "", "", "")
		if err != nil {
			t.Fatalf("create session: %v", err)
		}
		if _, err := cl.SendMessage(ctx, ses.ID, protocol.SendMessageRequest{Text: "first"}); err != nil {
			t.Fatalf("send: %v", err)
		}
		waitForStatus(t, cl, ses.ID, protocol.SessionStatusBusy)
		// the run attaches to the busy session -> 409 -> exit 1
		code, _, errOut := captureRun(t, "run", "second", "--dir", wd, "--attach", "http://"+ln.String(), "--session", ses.ID)
		if code != 1 {
			t.Fatalf("exit = %d, want 1 (stderr: %s)", code, errOut)
		}
		if want := "yolo run: session busy: " + ses.ID; !strings.Contains(errOut, want) {
			t.Fatalf("stderr missing %q:\n%s", want, errOut)
		}
	})
}

// TestRunAttach pins leg (g)'s --attach half: the run points at a
// SECOND in-process server (the serve target) — no in-process boot in
// the run process — and the turn completes THERE.
func TestRunAttach(t *testing.T) {
	root := t.TempDir()
	wd := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	t.Setenv("YOLO_LLM", "fake")
	script := filepath.Join(root, "script.json")
	if err := os.WriteFile(script, []byte(fakeScriptOK), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("YOLO_FAKE_SCRIPT", script)

	deps, closeDB := buildStack(t)
	defer closeDB()
	srv := server.NewServer(*deps)
	ln, err := srv.Start("127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer srv.Close()
	code, out, _ := captureRun(t, "run", "hi", "--attach", "http://"+ln.String(), "--dir", wd)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if want := "ok\n"; out != want {
		t.Fatalf("stdout = %q, want %q", out, want)
	}
}
