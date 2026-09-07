package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/kido5217/yolo/internal/protocol"
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
// --agent -> session -> send -> event loop. (The boot/send/loop body
// lands in the next task; this slice ends at the message check.)
func runRunE(cmd *cobra.Command, args []string) error {
	formatStr, _ := cmd.Flags().GetString("format")
	switch formatStr {
	case "default":
	case "json":
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
	// files is consumed by the send leg (the next task); held here so the
	// slice compiles while the boot/send body is not yet written.
	_ = files
	return nil
}
