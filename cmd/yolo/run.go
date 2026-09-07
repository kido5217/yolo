package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/server"
	"github.com/kido5217/yolo/internal/tui/client"
)

// newRunCmd is the `yolo run` leaf (0.9.0): runs a prompt headlessly
// (spec docs/superpowers/specs/2026-09-07-yolo-run-design.md).
func newRunCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "run [message...]",
		Short: "run a prompt headlessly and print the result",
		Args:  cobra.ArbitraryArgs,
		RunE:  runRunE,
	}
	c.Flags().String("model", "", "seed the minted session's model (provider/model; ignored with --session/--continue)")
	c.Flags().String("agent", "", "seed the minted session's agent (ignored with --session/--continue)")
	c.Flags().String("title", "", "title of the minted session (blank: server default)")
	c.Flags().String("dir", "", "project directory (default CWD)")
	c.Flags().String("session", "", "run in an existing session (takes precedence over --continue)")
	c.Flags().Bool("continue", false, "run in the most recently updated session in the directory")
	c.Flags().String("attach", "", "attach to a running yolo serve URL instead of booting in-process")
	c.Flags().StringArrayP("file", "f", nil, "attach a local file (repeatable)")
	c.Flags().String("format", "default", "output format: default | json (NDJSON)")
	c.Flags().Bool("auto", false, "answer permission asks with once instead of reject")
	c.Flags().Bool("thinking", false, "print reasoning output (off by default)")
	return c
}

// resolveFiles validates and reads the --file entries (flag order),
// producing data URLs (spec §3). base is the resolved workdir. The error
// text is the pinned line after the `yolo run: ` prefix.
func resolveFiles(base string, paths []string) ([]protocol.FileRef, error) {
	out := make([]protocol.FileRef, 0, len(paths))
	for _, p := range paths {
		abs := p
		if !filepath.IsAbs(p) {
			abs = filepath.Join(base, p)
		}
		st, err := os.Stat(abs)
		if err != nil {
			return nil, fmt.Errorf("File not found: %s", p)
		}
		if !st.Mode().IsRegular() || st.Size() > protocol.AttachFileMaxBytes {
			return nil, fmt.Errorf("Cannot attach local file larger than 10 MiB or a special file: %s", p)
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return nil, fmt.Errorf("File not found: %s", p)
		}
		mime := "text/plain"
		if !utf8.Valid(data) {
			mime = "application/octet-stream"
		}
		out = append(out, protocol.FileRef{
			MIME:     mime,
			Filename: filepath.Base(abs),
			URL:      "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data),
		})
	}
	return out, nil
}

// composeRunMessage joins the positionals with a single space and appends
// non-TTY stdin (the upstream resolveRunInput referent, spec §2):
// positionals non-empty -> joined + "\n" + stdin; positionals empty ->
// stdin alone.
func composeRunMessage(args []string) string {
	msg := strings.Join(args, " ")
	f, _ := os.Stdin.Stat()
	if f == nil || f.Mode()&os.ModeCharDevice == 0 {
		b, _ := io.ReadAll(os.Stdin)
		if s := strings.TrimSpace(string(b)); s != "" {
			if msg != "" {
				msg += "\n" + s
			} else {
				msg = s
			}
		}
	}
	return msg
}

// runRunE executes `yolo run`. The pre-flight order is pinned (spec §2):
// flag parse (cobra) -> --format -> --dir -> files -> message -> boot ->
// --agent -> session -> send -> event loop.
func runRunE(cmd *cobra.Command, args []string) error {
	formatStr, _ := cmd.Flags().GetString("format")
	var format format
	switch formatStr {
	case "default":
		format = formatDefault
	case "json":
		format = formatJSON
	default:
		fmt.Fprintf(os.Stderr, "yolo run: unknown format value %q\n", formatStr)
		cmd.Usage()
		return errUsage
	}
	dir, _ := cmd.Flags().GetString("dir")
	wd, err := workDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yolo run: %v\n", err)
		return errUsage
	}
	fileFlags, _ := cmd.Flags().GetStringArray("file")
	files, err := resolveFiles(wd, fileFlags)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yolo run: %v\n", err)
		return errRuntime
	}
	msg := composeRunMessage(args)
	if strings.TrimSpace(msg) == "" {
		fmt.Fprintf(os.Stderr, "yolo run: message required\n")
		cmd.Usage()
		return errUsage
	}

	// Boot (spec §2 step 6): the tuiRunE in-process pattern, or --attach.
	attach, _ := cmd.Flags().GetString("attach")
	ctx := context.Background()
	var cl *client.Service
	var closeDB func()
	var srv *server.Server
	if attach != "" {
		cl = client.New(attach, wd)
	} else {
		deps, cdb, err := buildDeps(wd, "")
		if err != nil {
			fmt.Fprintf(os.Stderr, "yolo run: %v\n", err)
			return errRuntime
		}
		closeDB = cdb
		defer closeDB()
		s := server.NewServer(*deps)
		ln, err := s.Start("127.0.0.1:0")
		if err != nil {
			fmt.Fprintf(os.Stderr, "yolo run: %v\n", err)
			return errRuntime
		}
		srv = s
		defer srv.Close()
		cl = client.New("http://"+ln.String(), wd)
	}

	// --agent validation (spec §2 step 7): MINT PATH ONLY (skipped with
	// --session/--continue, where the agent is ignored). A list failure
	// exits 1; an unknown name warns and falls back (never an exit).
	agent, _ := cmd.Flags().GetString("agent")
	sessionFlag, _ := cmd.Flags().GetString("session")
	cont, _ := cmd.Flags().GetBool("continue")
	if agent != "" && sessionFlag == "" && !cont {
		agents, err := cl.ListAgents(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "yolo run: %v\n", err)
			return errRuntime
		}
		found := false
		for _, a := range agents {
			if a.Name == agent {
				found = true
				break
			}
		}
		if !found {
			fmt.Fprintf(os.Stderr, "agent %q not found. Falling back to default agent\n", agent)
			agent = ""
		}
	}

	// Session resolution (spec §2 step 8 / §7.1): --session (404 -> exit
	// 1) > --continue (first row; empty list mints silently) > mint.
	var ses protocol.Session
	switch {
	case sessionFlag != "":
		s, err := cl.GetSession(ctx, sessionFlag)
		if err != nil {
			if errors.Is(err, client.ErrNotFound) {
				fmt.Fprintf(os.Stderr, "yolo run: Session not found: %s\n", sessionFlag)
			} else {
				fmt.Fprintf(os.Stderr, "yolo run: %v\n", err)
			}
			return errRuntime
		}
		ses = s
	case cont:
		list, err := cl.ListSessions(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "yolo run: %v\n", err)
			return errRuntime
		}
		if len(list) > 0 {
			ses = list[0] // ORDER BY time_updated DESC (the pinned rule)
		}
	}
	if ses.ID == "" {
		title, _ := cmd.Flags().GetString("title")
		model, _ := cmd.Flags().GetString("model")
		s, err := cl.CreateSessionWith(ctx, title, agent, model)
		if err != nil {
			fmt.Fprintf(os.Stderr, "yolo run: %v\n", err)
			return errRuntime
		}
		ses = s
	}

	// Signals (spec §7.4): armed after boot, BEFORE the send. First SIGINT
	// aborts the in-flight turn (10 s cap) then exits 130; the second
	// SIGINT force-kills immediately.
	var started atomic.Bool
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT)
	defer signal.Stop(sigCh)
	go func() {
		first := true
		for range sigCh {
			if first {
				first = false
				os.Exit(firstSigint(cl, ses.ID, started.Load()))
			}
			os.Exit(130)
		}
	}()

	// Send (spec §2 step 9).
	if _, err := cl.SendMessage(ctx, ses.ID, protocol.SendMessageRequest{Text: msg, Files: files}); err != nil {
		if errors.Is(err, client.ErrBusy) {
			fmt.Fprintf(os.Stderr, "yolo run: session busy: %s\n", ses.ID)
		} else {
			fmt.Fprintf(os.Stderr, "yolo run: %v\n", err)
		}
		return errRuntime
	}
	started.Store(true)

	// Event loop until idle (spec §2 step 10 / §6 / §6.3).
	thinking, _ := cmd.Flags().GetBool("thinking")
	auto, _ := cmd.Flags().GetBool("auto")
	header := ""
	if format == formatDefault {
		header = runHeader(ses)
	}
	r := newRenderer(format, thinking, ses.ID, header, time.Now().UnixMilli, os.Stdout, os.Stderr)
	turnErr := runEvents(ctx, cl, ses.ID, r, auto)
	r.finish()
	// Settle read: the persisted rows are the source of truth — this
	// catches a turn-error event lost to an SSE drop (spec §6.3).
	if msgs, err := cl.ListMessages(ctx, ses.ID); err == nil {
		turnErr = settleTurnError(msgs, turnErr)
	}
	if turnErr {
		return errRuntime
	}
	return nil
}

// runEvents consumes the run's event stream until the session reports
// idle (or a resync status check finds it idle — spec §6.3), answering
// the run's permission asks per the policy (reject by default, once with
// --auto). It returns the turn-error flag observed on events; the caller
// settles it with the final ListMessages read.
func runEvents(ctx context.Context, cl *client.Service, sessionID string, r *renderer, auto bool) bool {
	events, resync := cl.Events(ctx)
	turnErr := false
	for {
		select {
		case <-ctx.Done():
			return turnErr
		case ev, ok := <-events:
			if !ok {
				return turnErr
			}
			if ev.Type == protocol.EventTypePermissionAsked {
				var p protocol.PermissionAskedProps
				if json.Unmarshal(ev.Properties, &p) == nil && p.SessionID == sessionID {
					reply := "reject"
					if auto {
						reply = "once"
					}
					// A reply failure (the ask already resolved/expired)
					// is swallowed: no stderr, no exit-code change
					// (spec §7.2).
					_ = cl.ReplyPermission(ctx, p.ID, reply)
					if !auto {
						permissionNote(os.Stderr, &p)
					}
				}
				continue
			}
			r.apply(ev)
			if ev.Type == protocol.EventTypeMessageUpdated {
				var p protocol.MessageUpdatedProps
				if json.Unmarshal(ev.Properties, &p) == nil && p.SessionID == sessionID && p.Info.Error != nil {
					turnErr = true
				}
			}
			if ev.Type == protocol.EventTypeSessionStatus {
				var p protocol.SessionStatusProps
				if json.Unmarshal(ev.Properties, &p) == nil && p.SessionID == sessionID &&
					p.Status.Type == protocol.SessionStatusIdle {
					return turnErr
				}
			}
		case <-resync:
			// One Status() check per drop (spec §6.3): idle -> the turn
			// is over (the settle read still runs after the break);
			// absent or non-idle -> keep consuming the reconnected stream.
			if st, err := cl.Status(ctx); err == nil {
				if st[sessionID] == protocol.SessionStatusIdle {
					return turnErr
				}
			}
		}
	}
}

// permissionNote is the §6.1-3 stderr line, byte-exactly:
// "permission requested: <permission>" + (when len(Patterns) >= 1:
// " (<patterns joined \", \")>") + "; auto-rejecting".
func permissionNote(w io.Writer, p *protocol.PermissionAskedProps) {
	l := "permission requested: " + p.Permission
	if len(p.Patterns) >= 1 {
		l += " (" + strings.Join(p.Patterns, ", ") + ")"
	}
	l += "; auto-rejecting"
	fmt.Fprintln(w, l)
}

// settleTurnError folds the persisted assistant error flag into the
// exit-code flag (the settle read, spec §6.3).
func settleTurnError(msgs []protocol.MessageWithParts, turnErr bool) bool {
	for i := range msgs {
		if msgs[i].Info.Role == "assistant" && msgs[i].Info.Error != nil {
			turnErr = true
		}
	}
	return turnErr
}

// firstSigint is the first-SIGINT handler (spec §7.4): when the turn has
// started it aborts (fresh ctx, 10 s cap — the server endpoint waits
// <=2 s for the aborted turn to settle idle), then returns the exit code.
func firstSigint(cl *client.Service, sessionID string, started bool) int {
	if started {
		ac, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, _ = cl.Abort(ac, sessionID)
		cancel()
	}
	return 130
}
