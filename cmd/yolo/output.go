package main

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

// --output (D2, yolo-o75.8, v0.6.0): machine-readable output for the
// data-reporting leaves only (auth list, profile list, version). `json` is
// the sole accepted value; the default (empty) keeps the human output
// byte-for-byte. The JSON is a bare top-level value (no envelope),
// 2-space-indented, on stdout on success only; stderr stays human and the
// exit codes stay 0/1/2.

// jsonOutputFormat is the sole accepted --output value.
const jsonOutputFormat = "json"

// jsonAuthEntry is one entry of the `auth list --output json` array.
type jsonAuthEntry struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// jsonProfileEntry is one entry of the `profile list --output json` array.
type jsonProfileEntry struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Active      bool   `json:"active"`
}

// jsonVersion is the `version --output json` object.
type jsonVersion struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Commit  string `json:"commit,omitempty"`
	Built   string `json:"built,omitempty"`
}

// outputSupported reports whether the command accepts --output: only the
// three data-reporting leaves (D2).
func outputSupported(cmd *cobra.Command) bool {
	switch cmd.CommandPath() {
	case "yolo auth list", "yolo profile list", "yolo version":
		return true
	}
	return false
}

// checkOutputFormat is the root PersistentPreRunE: it validates --output
// before any command side effect (serve must not bind, the TUI root must not
// start its in-process server). An unknown value is a usage error with the
// root prefix; json on an unsupported command names the command.
func checkOutputFormat(cmd *cobra.Command, _ []string) error {
	// Shell-completion requests run __complete with the user's in-progress
	// flag words; they must never see the check.
	if cmd.Name() == "__complete" || cmd.Name() == "__completeNoDesc" {
		return nil
	}
	val, _ := cmd.Flags().GetString("output")
	if val == "" {
		return nil
	}
	if val != jsonOutputFormat {
		fmt.Fprintf(os.Stderr, "yolo: unknown output format %q\n", val)
		cmd.Root().Usage()
		return errUsage
	}
	if !outputSupported(cmd) {
		// The message names the command CLI-style: "yolo" for the root (a
		// single prefix, per the v0.6.1 ruling, yolo-sti) and "yolo <path>"
		// for subcommands.
		var prefix, name string
		if cmd == cmd.Root() {
			prefix, name = "yolo", "yolo"
		} else {
			prefix = cmd.CommandPath()
			name = strings.TrimPrefix(prefix, "yolo ")
		}
		fmt.Fprintf(os.Stderr, "%s: --output is not supported by %s\n", prefix, name)
		cmd.Usage()
		return errUsage
	}
	return nil
}

// isJSONOutput reports whether the command ran with --output json (the
// pre-run has already validated the value: empty or json only).
func isJSONOutput(cmd *cobra.Command) bool {
	out, _ := cmd.Flags().GetString("output")
	return out == jsonOutputFormat
}

// renderJSON marshals v as a bare 2-space-indented JSON value (no envelope);
// the leaf prints it to stdout on success only.
func renderJSON(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

// versionJSON is the --output json counterpart of printVersion: the full
// vcs.revision (no display truncation) and the raw vcs.time, both omitted
// when absent.
func versionJSON() jsonVersion {
	v := jsonVersion{Name: "yolo", Version: version}
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				v.Commit = s.Value
			case "vcs.time":
				v.Built = s.Value
			}
		}
	}
	return v
}
