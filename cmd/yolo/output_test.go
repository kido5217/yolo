package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/kido5217/yolo/internal/auth"
)

// runOutput runs the CLI in-process with the process stdout/stderr swapped
// for pipes and returns (exit code, stdout, stderr).
func runOutput(t *testing.T, args ...string) (int, string, string) {
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
	code := run(args)
	_ = outW.Close()
	_ = errW.Close()
	outB, _ := io.ReadAll(outR)
	errB, _ := io.ReadAll(errR)
	return code, string(outB), string(errB)
}

// withXDG points all XDG homes at a fresh temp dir for the calling (sub)test.
func withXDG(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	return root
}

// TestOutputUnknownFormat pins D2 (yolo-o75.8): an unknown --output value is
// a usage error with the root prefix, the usage on stderr, exit 2, and an
// empty stdout. json is the only accepted value (no "human" alias).
func TestOutputUnknownFormat(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantMsg string
	}{
		{name: "root", args: []string{"--output", "yaml"}, wantMsg: `yolo: unknown output format "yaml"`},
		{name: "auth list", args: []string{"auth", "list", "--output", "yaml"}, wantMsg: `yolo: unknown output format "yaml"`},
		{name: "no human alias", args: []string{"version", "--output=human"}, wantMsg: `yolo: unknown output format "human"`},
		{name: "single-dash long form", args: []string{"auth", "list", "-output", "json5"}, wantMsg: `yolo: unknown output format "json5"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			withXDG(t)
			code, out, errOut := runOutput(t, tc.args...)
			if code != 2 {
				t.Fatalf("exit = %d, want 2\nstdout: %s\nstderr: %s", code, out, errOut)
			}
			if out != "" {
				t.Fatalf("stdout = %q, want empty", out)
			}
			if !strings.Contains(errOut, tc.wantMsg) {
				t.Fatalf("stderr missing %q:\n%s", tc.wantMsg, errOut)
			}
			if !strings.Contains(errOut, "Usage:") {
				t.Fatalf("stderr missing the usage:\n%s", errOut)
			}
		})
	}
}

// TestOutputUnsupported pins D2: --output json on any non-reporting command
// (serve, the TUI root, completion, mutating leaves, bare parents) is a
// usage error naming the command, before any side effect.
func TestOutputUnsupported(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantMsg string
	}{
		{name: "serve", args: []string{"serve", "--output", "json"}, wantMsg: "yolo serve: --output is not supported by serve"},
		{name: "serve -v", args: []string{"serve", "-v", "--output", "json"}, wantMsg: "yolo serve: --output is not supported by serve"},
		{name: "tui root", args: []string{"--output", "json"}, wantMsg: "yolo: --output is not supported by yolo"},
		// `completion bash` is the runnable leaf; bare `completion` (no Run)
		// shows help and exits 0, as pre-cobra-x1.
		{name: "completion bash", args: []string{"completion", "bash", "--output", "json"}, wantMsg: "yolo completion bash: --output is not supported by completion bash"},
		{name: "bare auth", args: []string{"auth", "--output", "json"}, wantMsg: "yolo auth: --output is not supported by auth"},
		{name: "auth add", args: []string{"auth", "add", "kido", "sk-x", "--output", "json"}, wantMsg: "yolo auth add: --output is not supported by auth add"},
		{name: "profile use", args: []string{"profile", "use", "nope", "--output", "json"}, wantMsg: "yolo profile use: --output is not supported by profile use"},
		{name: "profile copy", args: []string{"profile", "copy", "a", "b", "--output", "json"}, wantMsg: "yolo profile copy: --output is not supported by profile copy"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			withXDG(t)
			code, out, errOut := runOutput(t, tc.args...)
			if code != 2 {
				t.Fatalf("exit = %d, want 2\nstdout: %s\nstderr: %s", code, out, errOut)
			}
			if out != "" {
				t.Fatalf("stdout = %q, want empty", out)
			}
			// The message is the exact first stderr line (before the usage):
			// a byte pin, so a doubled prefix (v0.6.0's "yolo yolo:", the
			// spec-literal reading fixed in v0.6.1, yolo-sti) fails here.
			first := errOut
			if i := strings.IndexByte(errOut, '\n'); i >= 0 {
				first = errOut[:i]
			}
			if first != tc.wantMsg {
				t.Fatalf("stderr first line = %q, want %q:\n%s", first, tc.wantMsg, errOut)
			}
			if !strings.Contains(errOut, "Usage:") {
				t.Fatalf("stderr missing the usage:\n%s", errOut)
			}
		})
	}
}

// TestOutputUnsupportedNoSideEffects pins the "checked before any side
// effect" half of D2: the failing commands never reach their RunE body.
func TestOutputUnsupportedNoSideEffects(t *testing.T) {
	t.Run("profile use does not switch the active marker", func(t *testing.T) {
		ch := withConfigHome(t)
		seedProfiles(
			t,
			ch,
			testProfile{id: "aaaa1111", name: "work"},
			testProfile{id: "bbbb2222", name: "personal"},
		)
		code, _, _ := runOutput(t, "profile", "use", "work", "--output", "json")
		if code != 2 {
			t.Fatalf("exit = %d, want 2", code)
		}
		// The pre-run error precedes profileRoot(): no active marker and no
		// auto-created default profile.
		if _, err := os.Stat(filepath.Join(ch, "yolo", "active")); !os.IsNotExist(err) {
			t.Fatalf("active marker exists (RunE ran), want none: %v", err)
		}
		if _, err := os.Stat(filepath.Join(ch, "yolo", "default")); !os.IsNotExist(err) {
			t.Fatalf("default profile dir exists (RunE ran), want none: %v", err)
		}
	})
	t.Run("auth add persists nothing", func(t *testing.T) {
		withXDG(t)
		code, _, _ := runOutput(t, "auth", "add", "kido", "sk-x", "--output", "json")
		if code != 2 {
			t.Fatalf("exit = %d, want 2", code)
		}
		p, err := auth.Path()
		if err != nil {
			t.Fatalf("auth.Path: %v", err)
		}
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("store file exists (RunE ran), want none: %v", err)
		}
	})
}

// TestAuthListJSON pins the D2 shape for `auth list --output json`: a bare
// top-level array of {id, type}, sorted by id, 2-space-indented, with an
// empty store rendering as [] (not null).
func TestAuthListJSON(t *testing.T) {
	t.Run("empty store prints bare []", func(t *testing.T) {
		withXDG(t)
		code, out, errOut := runOutput(t, "auth", "list", "--output", "json")
		if code != 0 {
			t.Fatalf("exit = %d stderr = %q", code, errOut)
		}
		if out != "[]\n" {
			t.Fatalf("stdout = %q, want %q", out, "[]\n")
		}
	})
	t.Run("entries: exact 2-space JSON sorted by id", func(t *testing.T) {
		withXDG(t)
		p, err := auth.Path()
		if err != nil {
			t.Fatalf("auth.Path: %v", err)
		}
		if err := auth.SaveTo(auth.Store{
			"zeta": {Type: "api", Key: "sk-zeta"},
			"kido": {Type: "api", Key: "sk-kido"},
		}, p); err != nil {
			t.Fatalf("seed store: %v", err)
		}
		code, out, errOut := runOutput(t, "auth", "list", "--output", "json")
		if code != 0 {
			t.Fatalf("exit = %d stderr = %q", code, errOut)
		}
		want := `[
  {
    "id": "kido",
    "type": "api"
  },
  {
    "id": "zeta",
    "type": "api"
  }
]
`
		if out != want {
			t.Fatalf("stdout = %q, want the exact pinned JSON\n%s", out, want)
		}
	})
}

// TestProfileListJSON pins the D2 shape for `profile list --output json`: a
// bare array of {id, name, description (omitempty), active}, in the
// config.List order (by name, then id).
func TestProfileListJSON(t *testing.T) {
	t.Run("fresh root lists the default profile", func(t *testing.T) {
		withConfigHome(t)
		code, out, errOut := runOutput(t, "profile", "list", "--output", "json")
		if code != 0 {
			t.Fatalf("exit = %d stderr = %q", code, errOut)
		}
		want := `[
  {
    "id": "default",
    "name": "default",
    "active": true
  }
]
`
		if out != want {
			t.Fatalf("stdout = %q, want the exact pinned JSON\n%s", out, want)
		}
	})
	t.Run("name, description and active; empty description omitted", func(t *testing.T) {
		ch := withConfigHome(t)
		seedProfiles(
			t,
			ch,
			testProfile{id: "aaaa1111", name: "work", desc: "work laptop", isActive: true},
			testProfile{id: "bbbb2222", name: "personal"},
		)
		code, out, errOut := runOutput(t, "profile", "list", "--output", "json")
		if code != 0 {
			t.Fatalf("exit = %d stderr = %q", code, errOut)
		}
		want := `[
  {
    "id": "bbbb2222",
    "name": "personal",
    "active": false
  },
  {
    "id": "aaaa1111",
    "name": "work",
    "description": "work laptop",
    "active": true
  }
]
`
		if out != want {
			t.Fatalf("stdout = %q, want the exact pinned JSON\n%s", out, want)
		}
	})
}

var commitSHARe = regexp.MustCompile(`^[0-9a-f]{40}$`)

// TestVersionJSON pins the D2 shape for `version --output json`: a bare
// object {name, version, commit (omitempty), built (omitempty)} with no
// envelope and 2-space indent. The commit is the full vcs.revision (the
// human line truncates to 8 chars for display only).
func TestVersionJSON(t *testing.T) {
	code, out, errOut := runOutput(t, "version", "--output", "json")
	if code != 0 {
		t.Fatalf("exit = %d stderr = %q", code, errOut)
	}
	// commit/built are omitempty (absent when the binary carries no VCS
	// stamping), so pin the head without the trailing comma.
	if !strings.HasPrefix(out, "{\n  \"name\": \"yolo\",\n  \"version\": \"0.0.0-dev\"") {
		t.Fatalf("stdout = %q, want the pinned 2-space JSON head", out)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("stdout is not a bare JSON object: %v\n%s", err, out)
	}
	for k := range m {
		switch k {
		case "name", "version", "commit", "built":
		default:
			t.Fatalf("unexpected key %q in the version JSON", k)
		}
	}
	if c, ok := m["commit"]; ok {
		if s, _ := c.(string); !commitSHARe.MatchString(s) {
			t.Fatalf("commit = %q, want a 40-hex vcs.revision", s)
		}
	}
	if b, ok := m["built"]; ok {
		if s, _ := b.(string); s == "" {
			t.Fatal("built present but empty")
		}
	}
}

// TestOutputJSONOnlyOnSuccess pins D2: stdout carries JSON only on success;
// on failure it stays empty while stderr keeps the human wording.
func TestOutputJSONOnlyOnSuccess(t *testing.T) {
	t.Run("auth list load error: exit 1, empty stdout", func(t *testing.T) {
		withXDG(t)
		p, err := auth.Path()
		if err != nil {
			t.Fatalf("auth.Path: %v", err)
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("{not json"), 0o600); err != nil {
			t.Fatalf("write corrupt store: %v", err)
		}
		code, out, errOut := runOutput(t, "auth", "list", "--output", "json")
		if code != 1 {
			t.Fatalf("exit = %d, want 1", code)
		}
		if out != "" {
			t.Fatalf("stdout = %q, want empty", out)
		}
		if !strings.Contains(errOut, "auth list:") {
			t.Fatalf("stderr missing the human error:\n%s", errOut)
		}
	})
	t.Run("profile list root error: exit 1, empty stdout", func(t *testing.T) {
		ch := withConfigHome(t)
		// A regular file where the profile root dir should be: EnsureActive
		// fails, so the supported leaf exits 1 with the human error and no
		// JSON on stdout.
		if err := os.MkdirAll(ch, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ch, "yolo"), []byte("not a dir"), 0o644); err != nil {
			t.Fatal(err)
		}
		code, out, errOut := runOutput(t, "profile", "list", "--output", "json")
		if code != 1 {
			t.Fatalf("exit = %d, want 1", code)
		}
		if out != "" {
			t.Fatalf("stdout = %q, want empty", out)
		}
		if !strings.Contains(errOut, "yolo profile list:") {
			t.Fatalf("stderr missing the human error:\n%s", errOut)
		}
	})
}

// TestOutputHumanDefault pins the byte-for-byte human default: without the
// flag the current output is unchanged.
func TestOutputHumanDefault(t *testing.T) {
	t.Run("version human unchanged", func(t *testing.T) {
		code, out, _ := runOutput(t, "version")
		if code != 0 {
			t.Fatalf("exit = %d", code)
		}
		if !strings.HasPrefix(out, "yolo 0.0.0-dev\n") {
			t.Fatalf("stdout = %q, want the human version block", out)
		}
	})
	t.Run("auth list human unchanged", func(t *testing.T) {
		withXDG(t)
		code, out, errOut := runOutput(t, "auth", "list")
		if code != 0 {
			t.Fatalf("exit = %d stderr = %q", code, errOut)
		}
		if out != "no credentials\n" {
			t.Fatalf("stdout = %q, want the human empty-store line", out)
		}
	})
}

// TestOutputFlagInHelp pins the flag surface: the root help lists --output
// as a global (persistent) flag.
func TestOutputFlagInHelp(t *testing.T) {
	code, out, errOut := runOutput(t, "help")
	if code != 0 {
		t.Fatalf("exit = %d stderr = %q", code, errOut)
	}
	if !strings.Contains(out, "--output") {
		t.Fatalf("help missing --output:\n%s", out)
	}
}

// TestCompleteWithOutputFlag pins that shell-completion requests (__complete,
// invoked by the shells with the user's in-progress flag words) never see
// the --output check.
func TestCompleteWithOutputFlag(t *testing.T) {
	code, out, errOut := runOutput(t, "__complete", "auth", "list", "--output", "json", "")
	if code != 0 {
		t.Fatalf("__complete exit = %d stderr = %q", code, errOut)
	}
	if strings.Contains(errOut, "not supported") {
		t.Fatalf("completion saw the --output check:\n%s", errOut)
	}
	if out == "" {
		t.Fatal("__complete printed nothing")
	}
}
