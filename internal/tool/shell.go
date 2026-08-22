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

// endMarkerRe matches any end-marker line; Exec accepts a match only when
// the captured counter equals its own n (a marker with another counter —
// e.g. echoed by the command itself — falls through as normal output, as
// with the old per-n regex).
var endMarkerRe = regexp.MustCompile(`^__YOLO_END_(\d+)_([^\s]*)$`)

// Shell is the per-session persistent shell (plan Task 14 pinned protocol).
// It keeps one `bash --norc --noprofile` process alive across Exec calls so
// that `cd`, environment assignments, and other shell state carry over.
// Each command is written as its lines followed by a marker line
// `echo __YOLO_END_{n}_$?_$(pwd | base64 -w0)`; output (stderr wired to the
// same pipe) is read until the emitted line matches the shared endMarkerRe
// with counter n — the exit code and the new cwd come from the marker.
// On timeout/abort the process group is SIGKILL'd and the shell is marked
// dead; the next Exec respawns from the last known cwd (Dir if it no longer
// exists).
type Shell struct {
	Executable string // default "bash"; test override
	Dir        string
	limits     Limits
	mu         sync.Mutex
	cwd        string
	proc       *shellProc
	nextMarker int
}

// NewShell creates a shell rooted at dir. The process is spawned lazily on
// the first Exec.
func NewShell(dir string, limits Limits) *Shell {
	if dir == "" {
		dir = "."
	}
	return &Shell{Executable: "bash", Dir: dir, limits: limits.withDefaults(), cwd: dir}
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
	if s.proc != nil {
		proc := s.proc
		s.proc = nil
		reapProc(proc, true)
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

	if s.proc == nil {
		if err := s.spawnLocked(); err != nil {
			return 0, "", err
		}
	}
	proc := s.proc
	n := s.nextMarker
	s.nextMarker++
	// Pinned protocol: the marker line echoes the previous command's exit
	// code and the new cwd (base64, unwrapped). The reader accepts a line
	// matching endMarkerRe only when the captured counter is n; the emitted
	// form therefore has no separator before $? and the regex captures the
	// code then the base64 cwd. (Plan pin line 2739.)
	marker := fmt.Sprintf("echo __YOLO_END_%d_$?_$(pwd | base64 -w0)", n)
	markerN := strconv.Itoa(n)

	if _, err := io.WriteString(proc.stdin, command+"\n"+marker+"\n"); err != nil {
		s.proc = nil
		reapProc(proc, true)
		return 0, "", fmt.Errorf("shell: command process died: %w", err)
	}

	lines := proc.lines
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
			if !ok || ev.isLast {
				proc2 := s.proc
				s.proc = nil
				code := reapProc(proc2, false)
				return code, buf.String(), nil
			}
			if m := endMarkerRe.FindStringSubmatch(ev.line); m != nil && m[1] == markerN {
				code, dir := decodeMarker(m)
				if dir != "" {
					s.cwd = dir
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
			s.proc = nil
			reapProc(proc, true)
			return 0, buf.String(), errShellTimeout
		case <-ctx.Done():
			s.proc = nil
			reapProc(proc, true)
			return 0, buf.String(), errShellAborted
		}
	}
}

// decodeMarker decodes a matched end-marker into the previous command's
// exit code (0 when unparseable) and the new cwd ("" when absent or
// undecodable). The regex captures the marker counter (m[1]) and the rest
// "{code}_{b64cwd}" (m[2]); the underscore separates code from cwd (the
// standard base64 alphabet has no underscore).
func decodeMarker(m []string) (code int, dir string) {
	rest := m[2]
	i := strings.IndexByte(rest, '_')
	if i < 0 {
		return 0, ""
	}
	if c, aerr := strconv.Atoi(rest[:i]); aerr == nil {
		code = c
	}
	if b64 := rest[i+1:]; b64 != "" {
		if p, derr := base64.StdEncoding.DecodeString(b64); derr == nil {
			// pwd prints a trailing newline into the pipe; drop it so the
			// stored cwd is a real path (a raw "\n" suffix would make the
			// respawn's os.Stat fail and silently land in the root dir).
			dir = strings.TrimSuffix(string(p), "\n")
		}
	}
	return code, dir
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
	outR   *os.File // read end of the output pipe; child stdout+stderr both hit it
	reader *bufio.Reader
	lines  chan shellEvt
	stop   chan struct{}
}

// shellEvt is one pumped line. raw keeps the trailing newline for the
// output buffer; line is the trimmed form for onLine/marker matching.
type shellEvt struct {
	raw    string
	line   string
	isLast bool
}

// spawnLocked starts the shell process and its output pump. Callers hold
// s.mu and s.proc is nil.
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
	proc := &shellProc{
		cmd:    cmd,
		stdin:  stdin,
		outR:   outR,
		reader: bufio.NewReaderSize(outR, 64*1024),
		lines:  make(chan shellEvt, 512),
		stop:   make(chan struct{}),
	}
	s.proc = proc
	go s.readLoop(proc)
	return nil
}

// readLoop pumps combined stdout+stderr into proc.lines until the stream
// closes (process death or the teardown closing proc.stop). It outlives
// individual Exec calls — the reader is per-process, the marker matching is
// per-Exec in Shell.Exec.
func (s *Shell) readLoop(proc *shellProc) {
	defer close(proc.lines)
	for {
		data, rerr := proc.reader.ReadString('\n')
		if data != "" {
			raw := data
			if strings.HasSuffix(raw, "\n") {
				raw = strings.TrimSuffix(raw, "\n")
				raw = strings.TrimSuffix(raw, "\r")
				raw += "\n"
			}
			ev := shellEvt{raw: raw, line: strings.TrimRight(raw, "\n")}
			select {
			case proc.lines <- ev:
			case <-proc.stop:
				return
			}
		}
		if rerr != nil {
			select {
			case proc.lines <- shellEvt{isLast: true}:
			case <-proc.stop:
			}
			return
		}
	}
}

// killGroup SIGKILLs the shell's process group (Setpgid makes the shell the
// group leader, so children share the group). Signals to an already-dead
// process are ignored.
func killGroup(proc *shellProc) {
	p := proc.cmd.Process
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
// output pump (proc.stop has exactly one closer — the detaching side), and
// reaps the process, returning its exit code (-1 when the code is
// undetermined, e.g. killed by signal).
func reapProc(proc *shellProc, kill bool) int {
	if kill {
		killGroup(proc)
	}
	close(proc.stop)
	proc.stdin.Close()
	if proc.outR != nil {
		// Unblocks the reader (if still waiting) and releases the fd. Safe to
		// close while the reader is blocked on it; the reader then exits via
		// proc.stop.
		proc.outR.Close()
	}
	err := proc.cmd.Wait()
	if err == nil {
		return 0
	}
	ee, ok := err.(*exec.ExitError)
	if !ok {
		return -1
	}
	return ee.ExitCode()
}
