// Command yolo runs the core HTTP server (REST + SSE) in-process and, by
// default, the bubbletea TUI which talks to it only through the wire
// contract (internal/protocol via internal/tui/client). `yolo serve` runs
// the server alone; `yolo auth` manages credentials.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/kido5217/yolo/internal/auth"
	"github.com/kido5217/yolo/internal/config"
	"github.com/kido5217/yolo/internal/log"
	"github.com/kido5217/yolo/internal/server"
	"github.com/kido5217/yolo/internal/tui"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// version is injected at build time: -ldflags "-X main.version=..." (just build).
var version = "0.0.0-dev"

// printVersion renders the version block: line 1 is always the ldflags
// version; lines 2-3 come from Go's automatic VCS stamping and are omitted
// when absent (e.g. GOFLAGS=-buildvcs=false).
func printVersion() {
	fmt.Printf("yolo %s\n", version)
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				if len(s.Value) > 8 {
					s.Value = s.Value[:8]
				}
				fmt.Printf("commit %s\n", s.Value)
			case "vcs.time":
				if s.Value != "" {
					fmt.Printf("built  %s\n", s.Value)
				}
			}
		}
	}
}

func main() {
	os.Exit(run(os.Args[1:]))
}

// errUsage and errRuntime are the RunE sentinels for the two failure
// classes (Unix convention, matching the pre-cobra dispatch): a usage
// error (bad flag/argument) exits 2, a runtime failure exits 1. The
// failing command has already printed its own "yolo <path>: %v" line (plus
// usage, for usage errors) before returning, so cobra prints nothing extra
// (SilenceErrors/SilenceUsage on the whole tree).
var (
	errUsage   = errors.New("usage error")
	errRuntime = errors.New("runtime error")
)

// run dispatches the CLI on the cobra command tree (v0.6.0 D1, yolo-o75.2):
// root = TUI, serve/auth/profile/version subcommands plus cobra's default
// completion command (static only). A fresh tree is built per call so the
// in-process tests can re-invoke run() without flag state leaking between
// invocations. The root -v/--version stays a first-arg pre-scan (NOT a
// persistent flag: `yolo --dir x -v` is a usage error today and must stay
// so).
// singleDashShorthands are the single-char shorthands the CLI defines (h =
// cobra's help flag on every command; v/d/n on serve/profile).
var singleDashShorthands = map[byte]bool{'h': true, 'v': true, 'd': true, 'n': true}

// normalizeSingleDash converts stdlib-flag-style single-dash long flags
// (-addr, -dir, -profile, even -description) to pflag's double-dash form.
// pflag reads a single dash as a shorthand cluster, but the stdlib flag
// package (the pre-cobra dispatcher) accepted single-dash long names, and
// the surface keeps them (the pinned serve test uses `-addr`). Only a
// single known shorthand char stays single-dash.
func normalizeSingleDash(args []string) []string {
	out := make([]string, len(args))
	copy(out, args)
	for i, a := range out {
		if len(a) < 2 || a[0] != '-' || a[1] == '-' {
			continue
		}
		name := a[1:]
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			name = name[:eq]
		}
		if len(name) == 1 && singleDashShorthands[name[0]] {
			continue
		}
		out[i] = "-" + a
	}
	return out
}

func run(args []string) int {
	if len(args) > 0 && (args[0] == "-v" || args[0] == "--version") {
		printVersion()
		return 0
	}
	root := newRootCmd()
	root.SetArgs(normalizeSingleDash(args))
	cmd, err := root.ExecuteC()
	if err == nil || errors.Is(err, flag.ErrHelp) {
		return 0
	}
	switch {
	case errors.Is(err, errUsage):
		return 2
	case errors.Is(err, errRuntime):
		return 1
	default:
		// A flag-parse error reached here (it precedes RunE): cobra
		// printed nothing (SilenceErrors), so print the app-style line +
		// the failing command's usage and exit 2, as the pre-cobra
		// per-command parseFlags path did.
		fmt.Fprintf(os.Stderr, "%s: %v\n", cmd.CommandPath(), err)
		cmd.Usage()
		return 2
	}
}

// profileFlagUsage is the --profile flag help text, shared by the root TUI
// command and serve.
const profileFlagUsage = "config profile to use (id or name; default: YOLO_PROFILE env, then the active profile)"

// newRootCmd builds the cobra command tree fresh (the D1 tree, yolo-o75.2).
// The root command runs the TUI; a positional argument is a sessionID for
// the root, not an error (pre-cobra behavior) — ArbitraryArgs opts out of
// cobra's legacyArgs rule that a root with subcommands rejects positionals.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "yolo [sessionID]",
		Short: "Go TUI + core-server harness",
		Args:  cobra.ArbitraryArgs,
		Long: "yolo runs the core HTTP server (REST + SSE) in-process and, by " +
			"default, the bubbletea TUI, which talks to it only through the " +
			"wire contract. `yolo [<sessionID>]` starts the TUI (optionally " +
			"resume a session).\n\n" +
			"--profile selects the config profile by id or name (default: " +
			"YOLO_PROFILE env, then the active profile set with yolo profile " +
			"use).",
		RunE: tuiRunE,
	}
	// --output (D2, yolo-o75.8): a root persistent flag validated in the
	// pre-run before any command side effect (json is the only value).
	root.PersistentFlags().String("output", "", "output format (json; default: human)")
	root.PersistentPreRunE = checkOutputFormat
	root.Flags().String("dir", "", "project directory (default CWD)")
	root.Flags().String("profile", "", profileFlagUsage)
	root.AddCommand(newServeCmd(), newAuthCmd(), newProfileCmd(), newVersionCmd())
	silenceAll(root)
	return root
}

// silenceAll sets SilenceUsage/SilenceErrors on the whole tree: the commands
// print their own "yolo <path>: %v" lines (plus usage where the pre-cobra
// commands did), so cobra must not print anything of its own.
func silenceAll(c *cobra.Command) {
	c.SilenceUsage = true
	c.SilenceErrors = true
	for _, sub := range c.Commands() {
		silenceAll(sub)
	}
}

func newServeCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "serve",
		Short: "run the core server only (default http://127.0.0.1:4096)",
		RunE:  serveRunE,
	}
	c.Flags().String("addr", "127.0.0.1:4096", "listen address")
	c.Flags().String("profile", "", profileFlagUsage)
	// -v and --version (the pre-cobra flag set defined the two separately);
	// one BoolP covers both spellings.
	c.Flags().BoolP("version", "v", false, "print version and exit")
	return c
}

// parentUsageErr is the RunE of the auth/profile parent commands: a bare
// `yolo auth` or an unknown subcommand prints the parent's usage to stderr
// and exits 2 (cobra's bare-parent default would drift to help/exit 0).
func parentUsageErr(cmd *cobra.Command, args []string) error {
	cmd.Usage()
	return errUsage
}

// unexpectedArg reports a stray positional argument on a subcommand leaf:
// "<command path>: unexpected argument %q" + the parent's usage, exit 2.
func unexpectedArg(cmd *cobra.Command, args []string, index int) error {
	fmt.Fprintf(os.Stderr, "%s: unexpected argument %q\n", cmd.CommandPath(), args[index])
	cmd.Parent().Usage()
	return errUsage
}

// profileRootErr loads the profile root, printing "<command path>: <err>"
// and flagging exit 1 on failure.
func profileRootErr(cmd *cobra.Command) (string, error) {
	root, err := profileRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", cmd.CommandPath(), err)
		return "", errRuntime
	}
	return root, nil
}

func newAuthCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "auth",
		Short: "manage credentials (list | add <provider> [key] | remove <provider>)",
		RunE:  parentUsageErr,
	}
	c.AddCommand(authListCmd(), authAddCmd(), authRemoveCmd())
	return c
}

func authListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "list credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return unexpectedArg(cmd, args, 0)
			}
			s, err := auth.Load()
			if err != nil {
				fmt.Fprintln(os.Stderr, "auth list:", err)
				return errRuntime
			}
			ids := make([]string, 0, len(s))
			for id := range s {
				ids = append(ids, id)
			}
			sort.Strings(ids)
			if isJSONOutput(cmd) {
				entries := make([]jsonAuthEntry, 0, len(ids))
				for _, id := range ids {
					entries = append(entries, jsonAuthEntry{ID: id, Type: s[id].Type})
				}
				b, err := renderJSON(entries)
				if err != nil {
					fmt.Fprintln(os.Stderr, "auth list:", err)
					return errRuntime
				}
				fmt.Println(string(b))
				return nil
			}
			if len(s) == 0 {
				fmt.Println("no credentials")
				return nil
			}
			for _, id := range ids {
				fmt.Printf("%s  %s  (set)\n", id, s[id].Type)
			}
			return nil
		},
	}
}

func authAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <provider> [key]",
		Short: "add a credential (key omitted = prompt on stdin)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 2 {
				return unexpectedArg(cmd, args, 2)
			}
			if len(args) < 1 {
				cmd.Parent().Usage()
				return errUsage
			}
			provider := args[0]
			var key string
			if len(args) >= 2 {
				key = strings.TrimSpace(args[1])
			} else {
				// no new dep: plain stdin prompt, echo NOT disabled (documented limitation)
				fmt.Fprint(os.Stderr, "API key: ")
				line, rerr := bufio.NewReader(os.Stdin).ReadString('\n')
				if rerr != nil {
					fmt.Fprintf(os.Stderr, "auth add: reading key: %v\n", rerr)
					return errRuntime
				}
				key = strings.TrimSpace(line)
			}
			if key == "" {
				fmt.Fprintln(os.Stderr, "auth add: key must not be empty")
				return errRuntime
			}
			s, err := auth.Load()
			if err != nil {
				fmt.Fprintln(os.Stderr, "auth add:", err)
				return errRuntime
			}
			s.Set(provider, key)
			if err := auth.Save(s); err != nil {
				fmt.Fprintln(os.Stderr, "auth add:", err)
				return errRuntime
			}
			return nil
		},
	}
}

func authRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <provider>",
		Short: "remove a credential",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return unexpectedArg(cmd, args, 1)
			}
			if len(args) < 1 {
				cmd.Parent().Usage()
				return errUsage
			}
			s, err := auth.Load()
			if err != nil {
				fmt.Fprintln(os.Stderr, "auth remove:", err)
				return errRuntime
			}
			s.Delete(args[0])
			if err := auth.Save(s); err != nil {
				fmt.Fprintln(os.Stderr, "auth remove:", err)
				return errRuntime
			}
			return nil
		},
	}
}

func newProfileCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "profile",
		Short: "manage config profiles (list | add [name] [-d DESC] | use ID | edit ID [-n NAME] [-d DESC] | remove ID | copy SRC NAME [-d DESC])",
		RunE:  parentUsageErr,
	}
	c.AddCommand(
		profileListCmd(),
		profileAddCmd(),
		profileUseCmd(),
		profileEditCmd(),
		profileRemoveCmd(),
		profileCopyCmd(),
	)
	return c
}

func profileListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "list profiles (* = active)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return unexpectedArg(cmd, args, 0)
			}
			root, err := profileRootErr(cmd)
			if err != nil {
				return err
			}
			profiles, err := config.List(root)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", cmd.CommandPath(), err)
				return errRuntime
			}
			active, err := config.ActiveID(root)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", cmd.CommandPath(), err)
				return errRuntime
			}
			if isJSONOutput(cmd) {
				entries := make([]jsonProfileEntry, 0, len(profiles))
				for _, p := range profiles {
					entries = append(entries, jsonProfileEntry{
						ID: p.ID, Name: p.Name, Description: p.Description, Active: p.ID == active,
					})
				}
				b, err := renderJSON(entries)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s: %v\n", cmd.CommandPath(), err)
					return errRuntime
				}
				fmt.Println(string(b))
				return nil
			}
			if len(profiles) == 0 {
				fmt.Println("no profiles")
				return nil
			}
			for _, p := range profiles {
				mark := "  "
				if p.ID == active {
					mark = "* "
				}
				line := mark + p.ID + "  " + p.Name
				if p.Description != "" {
					line += "  " + p.Description
				}
				fmt.Println(line)
			}
			return nil
		},
	}
}

func profileAddCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "add [name]",
		Short: "create a profile (auto id; name + optional description)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return unexpectedArg(cmd, args, 1)
			}
			desc, _ := cmd.Flags().GetString("description")
			root, err := profileRootErr(cmd)
			if err != nil {
				return err
			}
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			p, err := config.Add(root, name, desc)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", cmd.CommandPath(), err)
				return errRuntime
			}
			fmt.Printf("%s  %s\n", p.ID, p.Name)
			return nil
		},
	}
	c.Flags().StringP("description", "d", "", "profile description")
	return c
}

func profileUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <id_or_name>",
		Short: "set the active profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return unexpectedArg(cmd, args, 1)
			}
			if len(args) < 1 {
				cmd.Parent().Usage()
				return errUsage
			}
			root, err := profileRootErr(cmd)
			if err != nil {
				return err
			}
			id, ok := resolveProfile(cmd.Name(), root, args[0])
			if !ok {
				return errRuntime
			}
			if err := config.SetActive(root, id); err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", cmd.CommandPath(), err)
				return errRuntime
			}
			fmt.Println(id)
			return nil
		},
	}
}

func profileEditCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "edit <id_or_name>",
		Short: "change name and/or description (-n, -d)",
		RunE: func(cmd *cobra.Command, args []string) error {
			f := cmd.Flags()
			// A profile reference AND at least one field flag are both
			// required (pre-cobra: hasProfile && hasFieldFlag).
			if len(args) > 1 {
				return unexpectedArg(cmd, args, 1)
			}
			if len(args) != 1 || (!f.Changed("name") && !f.Changed("description")) {
				cmd.Parent().Usage()
				return errUsage
			}
			root, err := profileRootErr(cmd)
			if err != nil {
				return err
			}
			id, ok := resolveProfile(cmd.Name(), root, args[0])
			if !ok {
				return errRuntime
			}
			name, _ := f.GetString("name")
			desc, _ := f.GetString("description")
			p, err := config.Edit(
				root,
				id,
				name,
				desc,
				f.Changed("name"),
				f.Changed("description"),
			)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", cmd.CommandPath(), err)
				return errRuntime
			}
			fmt.Printf("%s  %s\n", p.ID, p.Name)
			return nil
		},
	}
	c.Flags().StringP("name", "n", "", "new profile name")
	c.Flags().StringP("description", "d", "", "new profile description")
	return c
}

func profileRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <id_or_name>",
		Short: "delete a profile (active falls back to the next one)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return unexpectedArg(cmd, args, 1)
			}
			if len(args) < 1 {
				cmd.Parent().Usage()
				return errUsage
			}
			root, err := profileRootErr(cmd)
			if err != nil {
				return err
			}
			id, ok := resolveProfile(cmd.Name(), root, args[0])
			if !ok {
				return errRuntime
			}
			if err := config.Remove(root, id); err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", cmd.CommandPath(), err)
				return errRuntime
			}
			fmt.Println(id)
			return nil
		},
	}
}

func profileCopyCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "copy <src> <name>",
		Short: "duplicate a profile under a new id",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 2 {
				return unexpectedArg(cmd, args, 2)
			}
			if len(args) < 2 {
				cmd.Parent().Usage()
				return errUsage
			}
			desc, _ := cmd.Flags().GetString("description")
			root, err := profileRootErr(cmd)
			if err != nil {
				return err
			}
			srcID, ok := resolveProfile(cmd.Name(), root, args[0])
			if !ok {
				return errRuntime
			}
			p, err := config.Copy(root, srcID, args[1], desc)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", cmd.CommandPath(), err)
				return errRuntime
			}
			fmt.Printf("%s  %s\n", p.ID, p.Name)
			return nil
		},
	}
	c.Flags().StringP("description", "d", "", "profile description")
	return c
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "print version (same as: yolo -v)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if isJSONOutput(cmd) {
				b, err := renderJSON(versionJSON())
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s: %v\n", cmd.CommandPath(), err)
					return errRuntime
				}
				fmt.Println(string(b))
				return nil
			}
			printVersion()
			return nil
		},
	}
}

// workDir resolves --dir to an absolute directory that must exist.
func workDir(flagDir string) (string, error) {
	d := flagDir
	if d == "" {
		var err error
		if d, err = os.Getwd(); err != nil {
			return "", err
		}
	}
	abs, err := filepath.Abs(d)
	if err != nil {
		return "", err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !st.IsDir() {
		return "", fmt.Errorf("not a directory: %s", abs)
	}
	return abs, nil
}

// tuiRunE runs `yolo [<sessionID>] [--dir DIR] [--profile ID]`: in-process
// server on an ephemeral port + the TUI.
func tuiRunE(cmd *cobra.Command, args []string) error {
	if len(args) > 1 {
		cmd.Root().Usage()
		return errUsage
	}
	var sessionID string
	if len(args) == 1 {
		sessionID = args[0]
	}
	dir, _ := cmd.Flags().GetString("dir")
	profile, _ := cmd.Flags().GetString("profile")
	wd, err := workDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yolo: %v\n", err)
		return errUsage
	}
	deps, closeDB, err := buildDeps(wd, profile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yolo: %v\n", err)
		return errRuntime
	}
	defer closeDB()

	deps.Log.Info("yolo starting", "mode", "tui", "workdir", wd, "version", version)

	srv := server.NewServer(*deps)
	ln, err := srv.Start("127.0.0.1:0")
	if err != nil {
		deps.Log.Error("listen failed", "error", err)
		fmt.Fprintf(os.Stderr, "yolo: %v\n", err)
		drain(deps, srv)
		return errRuntime
	}
	deps.Log.Info("serving on", "addr", ln.String(), "workdir", wd)

	// Swallow signals outside the TUI run so the drain below can finish;
	// during Run bubbletea's own handler ends the program (the clean-exit
	// mapping is tuiExit).
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(stop)
	go func() {
		sig := <-stop
		if sig != nil {
			deps.Log.Info("received signal, shutting down", "signal", sig.String())
		}
	}()

	cl := client.New("http://"+ln.String(), wd)
	if sessionID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := cl.GetSession(ctx, sessionID)
		cancel()
		if err != nil {
			if errors.Is(err, client.ErrNotFound) {
				fmt.Fprintf(os.Stderr, "session not found: %s\n", sessionID)
			} else {
				fmt.Fprintf(os.Stderr, "yolo: %v\n", err)
			}
			drain(deps, srv)
			return errUsage
		}
	}

	// Theme engine (S0.7): the config > KV > default selection chain
	// over the TUI-local KV file. The config is loaded via the same
	// profile-pinned loader buildDeps used (buildDeps consumes its
	// config internally and does not return it).
	globalDir, err := config.GlobalYoloDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yolo: %v\n", err)
		drain(deps, srv)
		return errRuntime
	}
	loader := config.Loader{Profile: deps.Dirs.Profile}
	cfg, err := loader.LoadAt(filepath.Join(globalDir, deps.Dirs.Profile), wd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yolo: %v\n", err)
		drain(deps, srv)
		return errRuntime
	}
	engine, err := theme.New(theme.EngineOptions{
		KVPath:        filepath.Join(deps.Dirs.Data, "tui", "kv.json"),
		GlobalYoloDir: globalDir,
		CWD:           wd,
		ConfigTheme:   cfg.Theme,
		Palette:       theme.DetectStd,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "yolo: %v\n", err)
		drain(deps, srv)
		return errRuntime
	}
	defer engine.Close()
	if err := engine.Resolve(context.Background()); err != nil {
		deps.Log.Error("theme resolve failed", "error", err)
	}

	app := tui.NewApp(
		cl,
		store.State{},
		sessionID,
		engine,
	)
	// the keybinds config (S4.3): apply the yolo.jsonc keybinds overrides to
	// the keymap registry (an unknown keybind is a config error — fail the
	// start, matching the other config-load failures above).
	if err := app.SetKeybinds(cfg.Keybinds); err != nil {
		fmt.Fprintf(os.Stderr, "yolo: %v\n", err)
		drain(deps, srv)
		return errRuntime
	}
	deps.Log.Info("tui start", "workdir", wd)
	program := tea.NewProgram(app)
	// The theme watcher (S0.6) sends ThemeRefreshMsg into the running
	// program; armed just before Run (a SIGUSR2 in the arm→Run gap
	// reaches the program at its first flush — the first refresh leg
	// runs one tick late at worst).
	stopTheme := theme.WatchThemeSignals(func() {
		program.Send(tui.ThemeRefreshMsg{})
	})
	defer stopTheme()
	_, runErr := program.Run()
	if runErr != nil {
		// One line to stderr (no stack): a TUI start failure must be
		// visible in the dead terminal, not only in the log (row 12).
		fmt.Fprintf(os.Stderr, "yolo: %v\n", runErr)
	}
	deps.Log.Info("tui end", "exit_code", tuiExit(runErr))
	app.Close()
	drain(deps, srv)
	if tuiExit(runErr) != 0 {
		return errRuntime
	}
	return nil
}

// tuiExit maps a tea.Run result to the process exit code: a program killed
// by a signal (bubbletea's built-in SIGINT/SIGTERM handler) is a clean
// exit, any other error is a failure.
func tuiExit(err error) int {
	if err == nil || errors.Is(err, tea.ErrProgramKilled) {
		return 0
	}
	return 1
}

// drainBudget is the graceful-shutdown budget: active turns and the
// listener are stopped within it; a second signal cancels it early (see
// armForceKill).
const drainBudget = 5 * time.Second

// drainCtx stops active turns and the listener within the caller's ctx
// budget, then closes the logger.
func drainCtx(deps *server.Deps, srv *server.Server, ctx context.Context) {
	deps.Engine.Shutdown(ctx)
	srv.Shutdown(ctx)
	deps.Log.Close()
}

// drain stops active turns and the listener within one drain budget, then
// closes the logger (process-exit path for serve and TUI mode).
func drain(deps *server.Deps, srv *server.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), drainBudget)
	defer cancel()
	drainCtx(deps, srv, ctx)
}

// armForceKill blocks on the next signal and cancels the drain ctx
// immediately (a second signal force-kills instead of waiting out the
// 5 s budget, concurrency-5).
func armForceKill(lg *log.Logger, stop <-chan os.Signal, cancel context.CancelFunc) {
	go func() {
		sig2 := <-stop
		lg.Info("second signal, force-killing", "signal", sig2.String())
		cancel()
	}()
}

// serveRunE runs `yolo serve [--addr ADDR] [--profile ID]`.
func serveRunE(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return unexpectedArg(cmd, args, 0)
	}
	showVersion, _ := cmd.Flags().GetBool("version")
	if showVersion {
		printVersion()
		return nil
	}
	addr, _ := cmd.Flags().GetString("addr")
	profile, _ := cmd.Flags().GetString("profile")
	wd, err := workDir("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "yolo serve: %v\n", err)
		return errRuntime
	}
	deps, closeDB, err := buildDeps(wd, profile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yolo serve: %v\n", err)
		return errRuntime
	}
	defer closeDB()
	deps.Log.Info("yolo starting", "mode", "serve", "workdir", wd, "version", version)

	srv := server.NewServer(*deps)
	ln, err := srv.Start(addr)
	if err != nil {
		deps.Log.Error("listen failed", "error", err)
		fmt.Fprintf(os.Stderr, "yolo serve: listen: %v\n", err)
		drain(deps, srv)
		return errRuntime
	}
	fmt.Printf("yolo serving on http://%s (dir %s)\n", ln.String(), wd)
	deps.Log.Info("serving on", "addr", ln.String(), "workdir", wd)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	sig := <-stop
	deps.Log.Info("received signal, shutting down", "signal", sig.String())
	ctx, cancel := context.WithTimeout(context.Background(), drainBudget)
	defer cancel()
	armForceKill(deps.Log, stop, cancel)
	drainCtx(deps, srv, ctx)
	return nil
}

// profileRoot returns the global profile root (<XDG config>/yolo) and
// ensures the active profile exists (first run creates the default).
func profileRoot() (string, error) {
	root, err := config.GlobalYoloDir()
	if err != nil {
		return "", err
	}
	if _, err := config.EnsureActive(root); err != nil {
		return "", err
	}
	return root, nil
}

// resolveProfile maps a CLI id_or_name reference to a profile id, printing
// a not-found hint (with the available profiles) on failure. A failing
// available-profiles listing degrades the hint (the not-found message
// stands on its own).
func resolveProfile(cmd, root, ref string) (string, bool) {
	id, err := config.Resolve(root, ref)
	if err == nil {
		return id, true
	}
	if errors.Is(err, config.ErrNotFound) {
		hint := ""
		if avail, lerr := config.List(root); lerr == nil {
			names := make([]string, 0, len(avail))
			for _, p := range avail {
				names = append(names, p.ID+" ("+p.Name+")")
			}
			hint = " (available: " + strings.Join(names, ", ") + ")"
		}
		fmt.Fprintf(os.Stderr, "yolo profile %s: profile %q not found%s\n", cmd, ref, hint)
	} else {
		fmt.Fprintf(os.Stderr, "yolo profile %s: %v\n", cmd, err)
	}
	return "", false
}
