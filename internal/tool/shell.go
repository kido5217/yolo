package tool

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// shellOutputCap is the in-memory guard per command invocation (plan pin:
// 10MB). Display truncation (Truncate) is applied by the bash tool to the
// returned output.
const shellOutputCap = 10 * 1024 * 1024

// errShellTimeout is returned by Shell.Exec when the per-command timeout
// fires; the bash tool rewrites it to the pinned upstream message.
var errShellTimeout = errors.New("shell timeout")

// errShellAborted is returned by Shell.Exec when ctx is cancelled; the bash
// tool rewrites it to "command aborted".
var errShellAborted = errors.New("shell aborted")

// Shell is the per-session persistent shell (plan Task 14 pinned protocol).
// It keeps one `bash --norc --noprofile` process alive across Exec calls so
// that `cd`, environment assignments, and other shell state carry over.
// Each command is written as its lines followed by a marker line
// `echo __YOLO_END_{n}_:$?_$(pwd | base64 -w0)`; output (stderr wired to the
// same pipe) is read until a line matches `^__YOLO_END_{n}_(\d+)_(\S*)$` —
// the exit code and the new cwd come from the marker. On timeout/abort the
// process group is SIGKILL'd and the shell is marked dead; the next Exec
// respawns from the last known cwd (Dir if it no longer exists).
type Shell struct {
	Executable string // default "bash"; test override
	Dir        string
	limits     Limits
	mu         sync.Mutex
	cwd        string
	st         *shellProc
	nextMarker int
}

// NewShell creates a shell rooted at dir. The process is spawned lazily on
// the first Exec.
func NewShell(dir string, limits Limits) *Shell {
	if dir == "" {
		dir = "."
	}
	return &Shell{Executable: "bash", Dir: dir, limits: limits.def(), cwd: dir}
}

// Cwd returns the shell's current working directory.
func (s *Shell) Cwd() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cwd
}

// Close kills the shell process group (if any) and reaps it.
func (s *Shell) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.st != nil {
		st := s.st
		s.st = nil
		reapProc(st, true)
	}
	return nil
}

// Exec runs one command in the persistent shell and returns its exit code
// and the combined stdout+stderr output (empty output stays empty; the
// caller formats it). timeoutMS <= 0 disables the timer. onLine, if
// non-nil, is invoked for every non-marker line as it is read. A shell
// killed by timeout/abort (or dead for any other reason) is respawned on
// the next call.
func (s *Shell) Exec(ctx context.Context, command string, timeoutMS int, onLine func(line string)) (exitCode int, out string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.st == nil {
		if err := s.spawnLocked(); err != nil {
			return 0, "", err
		}
	}
	st := s.st
	n := s.nextMarker
	s.nextMarker++
	// Pinned protocol: the marker line echoes the previous command's exit
	// code and the new cwd (base64, unwrapped). The reader matches
	// `^__YOLO_END_{n}_(\d+)_(\S*)$` on the emitted line; the emitted form
	// therefore has no separator before $? and the regex captures the code
	// then the base64 cwd. (Plan pin line 2739.)
	marker := fmt.Sprintf("echo __YOLO_END_%d_$?_$(pwd | base64 -w0)", n)
	re := regexp.MustCompile(fmt.Sprintf(`^__YOLO_END_%d_(\d+)_([^\s]*)$`, n))

	if _, err := io.WriteString(st.stdin, command+"\n"+marker+"\n"); err != nil {
		s.st = nil
		reapProc(st, true)
		return 0, "", fmt.Errorf("shell: command process died: %w", err)
	}

	lines := st.lines
	var buf strings.Builder
	var timer <-chan time.Time
	var timerStop func() bool
	if timeoutMS > 0 {
		t := time.NewTimer(time.Duration(timeoutMS) * time.Millisecond)
		timer = t.C
		timerStop = t.Stop
	}
	defer func() {
		if timerStop != nil {
			timerStop()
		}
	}()

	for {
		select {
		case ev, ok := <-lines:
			if !ok || ev.last {
				st2 := s.st
				s.st = nil
				code := reapProc(st2, false)
				return code, buf.String(), nil
			}
			if m := re.FindStringSubmatch(ev.line); m != nil {
				code := 0
				if c, aerr := strconv.Atoi(m[1]); aerr == nil {
					code = c
				}
				if b64 := m[2]; b64 != "" {
					if p, derr := base64.StdEncoding.DecodeString(b64); derr == nil {
						if dir := string(p); dir != "" {
							s.cwd = dir
						}
					}
				}
				return code, buf.String(), nil
			}
			if room := shellOutputCap - buf.Len(); room > 0 {
				if len(ev.raw) > room {
					buf.WriteString(ev.raw[:cutAtRuneBoundary(ev.raw, room)])
				} else {
					buf.WriteString(ev.raw)
				}
			}
			if onLine != nil {
				onLine(ev.line)
			}
		case <-timer:
			s.st = nil
			reapProc(st, true)
			return 0, buf.String(), errShellTimeout
		case <-ctx.Done():
			s.st = nil
			reapProc(st, true)
			return 0, buf.String(), errShellAborted
		}
	}
}

// cutAtRuneBoundary returns the largest cut ≤ n such that s[:cut] ends at a
// UTF-8 rune boundary: when the cap lands mid-rune (s[n] is a continuation
// byte) it backs off to that rune's start, so output truncated at
// shellOutputCap never carries a dangling continuation byte.
func cutAtRuneBoundary(s string, n int) int {
	for n > 0 && s[n]&0xC0 == 0x80 {
		n--
	}
	return n
}

// shellProc is one live shell process plus its output pump.
type shellProc struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *os.File // read end of the output pipe; child stdout+stderr both hit it
	readr  *bufio.Reader
	lines  chan shellEvt
	stop   chan struct{}
}

// shellEvt is one pumped line. raw keeps the trailing newline for the
// output buffer; line is the trimmed form for onLine/marker matching.
type shellEvt struct {
	raw  string
	line string
	last bool
}

// spawnLocked starts the shell process and its output pump. Callers hold
// s.mu and s.st is nil.
func (s *Shell) spawnLocked() error {
	exe := s.Executable
	if exe == "" {
		exe = "bash"
	}
	dir := s.cwd
	if fi, fterr := os.Stat(dir); fterr != nil || !fi.IsDir() {
		dir = s.Dir
	}
	args := []string{}
	if filepath.Base(exe) == "bash" {
		args = append(args, "--norc", "--noprofile")
	}
	cmd := exec.Command(exe, args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	// One output pipe: the child's stdout AND stderr both write to its write
	// end, so the combined stream lands in a single buffer for the reader.
	// (Pointing cmd.Stderr at the read end would fail the child's writes —
	// the read end is O_RDONLY.)
	outR, outW, err := os.Pipe()
	if err != nil {
		stdin.Close()
		return err
	}
	cmd.Stdout = outW
	cmd.Stderr = outW
	if err := cmd.Start(); err != nil {
		stdin.Close()
		outW.Close()
		outR.Close()
		return err
	}
	// Parent drops its copy of the write end so the reader observes EOF once
	// the child (and its dups of the write end) are gone.
	outW.Close()
	st := &shellProc{
		cmd:    cmd,
		stdin:  stdin,
		stdout: outR,
		readr:  bufio.NewReaderSize(outR, 64*1024),
		lines:  make(chan shellEvt, 512),
		stop:   make(chan struct{}),
	}
	s.st = st
	go s.readLoop(st)
	return nil
}

// readLoop pumps combined stdout+stderr into st.lines until the stream
// closes (process death or the teardown closing st.stop). It outlives
// individual Exec calls — the reader is per-process, the marker matching is
// per-Exec in Shell.Exec.
func (s *Shell) readLoop(st *shellProc) {
	defer close(st.lines)
	for {
		data, rerr := st.readr.ReadString('\n')
		if data != "" {
			raw := data
			if strings.HasSuffix(raw, "\n") {
				raw = strings.TrimSuffix(raw, "\n")
				raw = strings.TrimSuffix(raw, "\r")
				raw += "\n"
			}
			ev := shellEvt{raw: raw, line: strings.TrimRight(raw, "\n")}
			select {
			case st.lines <- ev:
			case <-st.stop:
				return
			}
		}
		if rerr != nil {
			select {
			case st.lines <- shellEvt{last: true}:
			case <-st.stop:
			}
			return
		}
	}
}

// killGroup SIGKILLs the shell's process group (Setpgid makes the shell the
// group leader, so children share the group). Signals to an already-dead
// process are ignored.
func killGroup(st *shellProc) {
	p := st.cmd.Process
	if p == nil {
		return
	}
	pgid, err := syscall.Getpgid(p.Pid)
	if err != nil {
		_ = syscall.Kill(p.Pid, syscall.SIGKILL)
		return
	}
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
}

// reapProc detaches a proc: optionally SIGKILLs its group, releases the
// output pump (st.stop has exactly one closer — the detaching side), and
// reaps the process, returning its exit code (-1 when the code is
// undetermined, e.g. killed by signal).
func reapProc(st *shellProc, kill bool) int {
	if kill {
		killGroup(st)
	}
	close(st.stop)
	st.stdin.Close()
	if st.stdout != nil {
		// Unblocks the reader (if still waiting) and releases the fd. Safe to
		// close while the reader is blocked on it; the reader then exits via
		// st.stop.
		st.stdout.Close()
	}
	err := st.cmd.Wait()
	if err == nil {
		return 0
	}
	ee, ok := err.(*exec.ExitError)
	if !ok {
		return -1
	}
	return ee.ExitCode()
}
