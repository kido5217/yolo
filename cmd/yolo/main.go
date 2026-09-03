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
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"

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

// run dispatches the CLI. The subcommand/flag surface is hand-rolled on the
// stdlib flag package; the golang-cli skill default is cobra+viper (with
// free shell completions) — migrating needs a dep approval (escalation,
// review yolo-3r8.5).
func run(args []string) int {
	if len(args) == 0 {
		return tuiCmd(nil)
	}
	if args[0] == "-v" || args[0] == "--version" {
		printVersion()
		return 0
	}
	switch args[0] {
	case "help", "-h", "--help":
		usage(os.Stdout)
		return 0
	case "serve":
		return serveCmd(args[1:])
	case "auth":
		return authCmd(args[1:])
	case "profile":
		return profileCmd(args[1:])
	case "version":
		printVersion()
		return 0
	default:
		return tuiCmd(args)
	}
}

func usage(w io.Writer) {
	// Quoted-string concatenation (not a raw string) so the long profile
	// line breaks at source level without changing the output.
	fmt.Fprint(w,
		"yolo — Go TUI + core-server harness\n\nUsage:\n  "+
			"yolo [<sessionID>] [--dir DIR] [--profile ID]   start the TUI (optionally resume a session)\n  "+
			"yolo serve [--addr ADDR] [--profile ID]         run the core server only (default http://127.0.0.1:4096)\n  "+
			"yolo auth <subcommand>           manage credentials (list | add <provider> [key] | remove <provider>)\n  "+
			"yolo profile <subcommand>        manage config profiles (list | add [name] [-d DESC] | use ID | "+
			"edit ID [-n NAME] [-d DESC] | remove ID | copy SRC NAME [-d DESC])\n  "+
			"yolo [-v|--version]              print version (same as: yolo version)\n  "+
			"yolo help                        this help\n\n"+
			"--profile selects the config profile by id or name (default: YOLO_PROFILE\n"+
			"env, then the active profile set with yolo profile use).\n")
}

// profileFlagUsage is the --profile flag help text, shared by the root TUI
// command and serve.
const profileFlagUsage = "config profile to use (id or name; default: YOLO_PROFILE env, then the active profile)"

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

// parseFlags runs the flagset with the flag package's own output silenced
// (we print the app usage ourselves): -h/--help is a clean help (stdout,
// exit 0), any other parse error prints the flag error + app usage and is
// a usage error (exit 2). It reports (ok, err): ok=false + err=nil means
// help was requested.
func parseFlags(fs *flag.FlagSet, args []string) (bool, error) {
	fs.SetOutput(io.Discard)
	err := fs.Parse(args)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, flag.ErrHelp) {
		return false, nil
	}
	return false, err
}

// tuiCmd runs `yolo [<sessionID>] [--dir DIR]`: in-process server on an
// ephemeral port + the TUI.
func tuiCmd(args []string) int {
	fs := flag.NewFlagSet("yolo", flag.ContinueOnError)
	dir := fs.String("dir", "", "project directory (default CWD)")
	profile := fs.String("profile", "", profileFlagUsage)
	ok, err := parseFlags(fs, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yolo: %v\n", err)
		usage(os.Stderr)
		return 2
	}
	if !ok { // -h/--help
		usage(os.Stdout)
		return 0
	}
	var sessionID string
	if fs.NArg() > 1 {
		usage(os.Stderr)
		return 2
	}
	if fs.NArg() == 1 {
		sessionID = fs.Arg(0)
	}
	wd, err := workDir(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yolo: %v\n", err)
		return 2
	}
	deps, closeDB, err := buildDeps(wd, *profile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yolo: %v\n", err)
		return 1
	}
	defer closeDB()

	deps.Log.Info("yolo starting", "mode", "tui", "workdir", wd, "version", version)

	srv := server.NewServer(*deps)
	ln, err := srv.Start("127.0.0.1:0")
	if err != nil {
		deps.Log.Error("listen failed", "error", err)
		fmt.Fprintf(os.Stderr, "yolo: %v\n", err)
		drain(deps, srv)
		return 1
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
			return 2
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
		return 1
	}
	loader := config.Loader{Profile: deps.Dirs.Profile}
	cfg, err := loader.LoadAt(filepath.Join(globalDir, deps.Dirs.Profile), wd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yolo: %v\n", err)
		drain(deps, srv)
		return 1
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
		return 1
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
		return 1
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
	return tuiExit(runErr)
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

func serveCmd(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1:4096", "listen address")
	profile := fs.String("profile", "", profileFlagUsage)
	showVersion := fs.Bool("v", false, "print version and exit")
	showVersionLong := fs.Bool("version", false, "print version and exit")
	ok, err := parseFlags(fs, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yolo serve: %v\n", err)
		usage(os.Stderr)
		return 2
	}
	if !ok { // -h/--help
		usage(os.Stdout)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "yolo serve: unexpected argument %q\n", fs.Arg(0))
		usage(os.Stderr)
		return 2
	}
	if *showVersion || *showVersionLong {
		printVersion()
		return 0
	}
	wd, err := workDir("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "yolo serve: %v\n", err)
		return 1
	}
	deps, closeDB, err := buildDeps(wd, *profile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yolo serve: %v\n", err)
		return 1
	}
	defer closeDB()
	deps.Log.Info("yolo starting", "mode", "serve", "workdir", wd, "version", version)

	srv := server.NewServer(*deps)
	ln, err := srv.Start(*addr)
	if err != nil {
		deps.Log.Error("listen failed", "error", err)
		fmt.Fprintf(os.Stderr, "yolo serve: listen: %v\n", err)
		drain(deps, srv)
		return 1
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
	return 0
}

func authUsage() int {
	fmt.Fprintln(os.Stderr, "Usage:\n  yolo auth list\n  yolo auth add <provider> [key]\n  yolo auth remove <provider>")
	return 2
}

func authCmd(args []string) int {
	if len(args) == 0 {
		return authUsage()
	}
	sub, rest := args[0], args[1:]

	loadStore := auth.Load

	switch sub {
	case "list":
		if len(rest) != 0 {
			fmt.Fprintf(os.Stderr, "yolo auth list: unexpected argument %q\n", rest[0])
			return authUsage()
		}
		s, err := loadStore()
		if err != nil {
			fmt.Fprintln(os.Stderr, "auth list:", err)
			return 1
		}
		if len(s) == 0 {
			fmt.Println("no credentials")
			return 0
		}
		ids := make([]string, 0, len(s))
		for id := range s {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			fmt.Printf("%s  %s  (set)\n", id, s[id].Type)
		}
		return 0
	case "add":
		if len(rest) < 1 || len(rest) > 2 {
			if len(rest) > 2 {
				fmt.Fprintf(os.Stderr, "yolo auth add: unexpected argument %q\n", rest[2])
			}
			return authUsage()
		}
		provider := rest[0]
		var key string
		if len(rest) >= 2 {
			key = strings.TrimSpace(rest[1])
		} else {
			// no new dep: plain stdin prompt, echo NOT disabled (documented limitation)
			fmt.Fprint(os.Stderr, "API key: ")
			line, rerr := bufio.NewReader(os.Stdin).ReadString('\n')
			if rerr != nil {
				fmt.Fprintf(os.Stderr, "auth add: reading key: %v\n", rerr)
				return 1
			}
			key = strings.TrimSpace(line)
		}
		if key == "" {
			fmt.Fprintln(os.Stderr, "auth add: key must not be empty")
			return 1
		}
		s, err := loadStore()
		if err != nil {
			fmt.Fprintln(os.Stderr, "auth add:", err)
			return 1
		}
		s.Set(provider, key)
		if err := auth.Save(s); err != nil {
			fmt.Fprintln(os.Stderr, "auth add:", err)
			return 1
		}
		return 0
	case "remove":
		if len(rest) < 1 || len(rest) > 1 {
			if len(rest) > 1 {
				fmt.Fprintf(os.Stderr, "yolo auth remove: unexpected argument %q\n", rest[1])
			}
			return authUsage()
		}
		s, err := loadStore()
		if err != nil {
			fmt.Fprintln(os.Stderr, "auth remove:", err)
			return 1
		}
		s.Delete(rest[0])
		if err := auth.Save(s); err != nil {
			fmt.Fprintln(os.Stderr, "auth remove:", err)
			return 1
		}
		return 0
	default:
		return authUsage()
	}
}

func profileUsage() int {
	fmt.Fprintln(os.Stderr,
		"Usage:\n  yolo profile list\n  yolo profile add [name] [-d description]\n  "+
			"yolo profile use <id_or_name>\n  yolo profile edit <id_or_name> [-n name] [-d description]\n  "+
			"yolo profile remove <id_or_name>\n  yolo profile copy <src> <name> [-d description]")
	return 2
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

// descFlagArgs is the parse result of a -d/--description flag group: the
// positional args plus the description value.
type descFlagArgs struct {
	pos  []string
	desc string
}

// editFlagArgs is the parse result of the -n/--name + -d/--description flag
// group: the positional args plus the flag values with their presence:
// absent != empty (an empty value clears the field), which descFlagArgs
// cannot express.
type editFlagArgs struct {
	pos     []string
	name    string
	desc    string
	hasName bool
	hasDesc bool
}

// pullDescFlags extracts the -d/--description flag from args (any position:
// -d X, --description X, --description=X) and returns the positional args
// plus the description value; a -d without a value is an error. The stdlib
// flag package stops at the first positional, which would forbid `profile
// add work -d "..."`.
func pullDescFlags(args []string) (descFlagArgs, error) {
	var out descFlagArgs
	out.pos = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-d" || a == "--description":
			if i+1 >= len(args) {
				return descFlagArgs{}, errors.New("-d flag needs a value")
			}
			out.desc = args[i+1]
			i++
		case strings.HasPrefix(a, "--description="):
			out.desc = strings.TrimPrefix(a, "--description=")
		default:
			out.pos = append(out.pos, a)
		}
	}
	return out, nil
}

// pullEditFlags extracts the -n/--name and -d/--description flags from args
// (any position: -n X, --name X, --name=X, -d X, --description X,
// --description=X) and returns the positional args plus the flag values with
// their presence; a -n or -d without a value is an error.
func pullEditFlags(args []string) (editFlagArgs, error) {
	var out editFlagArgs
	out.pos = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-n" || a == "--name":
			if i+1 >= len(args) {
				return editFlagArgs{}, errors.New("-n flag needs a value")
			}
			out.name, out.hasName = args[i+1], true
			i++
		case strings.HasPrefix(a, "--name="):
			out.name, out.hasName = strings.TrimPrefix(a, "--name="), true
		case a == "-d" || a == "--description":
			if i+1 >= len(args) {
				return editFlagArgs{}, errors.New("-d flag needs a value")
			}
			out.desc, out.hasDesc = args[i+1], true
			i++
		case strings.HasPrefix(a, "--description="):
			out.desc, out.hasDesc = strings.TrimPrefix(a, "--description="), true
		default:
			out.pos = append(out.pos, a)
		}
	}
	return out, nil
}

func profileCmd(args []string) int {
	if len(args) == 0 {
		return profileUsage()
	}
	sub, rest := args[0], args[1:]
	root, err := profileRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yolo profile: %v\n", err)
		return 1
	}

	switch sub {
	case "list":
		if len(rest) != 0 {
			fmt.Fprintf(os.Stderr, "yolo profile list: unexpected argument %q\n", rest[0])
			return profileUsage()
		}
		profiles, err := config.List(root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "yolo profile list: %v\n", err)
			return 1
		}
		active, err := config.ActiveID(root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "yolo profile list: %v\n", err)
			return 1
		}
		if len(profiles) == 0 {
			fmt.Println("no profiles")
			return 0
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
		return 0
	case "add":
		flags, err := pullDescFlags(rest)
		if err != nil {
			fmt.Fprintf(os.Stderr, "yolo profile add: %v\n", err)
			return profileUsage()
		}
		if len(flags.pos) > 1 {
			fmt.Fprintf(os.Stderr, "yolo profile add: unexpected argument %q\n", flags.pos[1])
			return profileUsage()
		}
		name := ""
		if len(flags.pos) == 1 {
			name = flags.pos[0]
		}
		p, err := config.Add(root, name, flags.desc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "yolo profile add: %v\n", err)
			return 1
		}
		fmt.Printf("%s  %s\n", p.ID, p.Name)
		return 0
	case "use":
		if len(rest) < 1 || len(rest) > 1 {
			if len(rest) > 1 {
				fmt.Fprintf(os.Stderr, "yolo profile use: unexpected argument %q\n", rest[1])
			}
			return profileUsage()
		}
		id, ok := resolveProfile("use", root, rest[0])
		if !ok {
			return 1
		}
		if err := config.SetActive(root, id); err != nil {
			fmt.Fprintf(os.Stderr, "yolo profile use: %v\n", err)
			return 1
		}
		fmt.Println(id)
		return 0
	case "edit":
		flags, err := pullEditFlags(rest)
		if err != nil {
			fmt.Fprintf(os.Stderr, "yolo profile edit: %v\n", err)
			return profileUsage()
		}
		if len(flags.pos) > 1 {
			fmt.Fprintf(os.Stderr, "yolo profile edit: unexpected argument %q\n", flags.pos[1])
			return profileUsage()
		}
		hasProfile := len(flags.pos) == 1
		hasFieldFlag := flags.hasName || flags.hasDesc
		if !hasProfile || !hasFieldFlag {
			return profileUsage()
		}
		id, ok := resolveProfile("edit", root, flags.pos[0])
		if !ok {
			return 1
		}
		p, err := config.Edit(
			root,
			id,
			flags.name,
			flags.desc,
			flags.hasName,
			flags.hasDesc,
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "yolo profile edit: %v\n", err)
			return 1
		}
		fmt.Printf("%s  %s\n", p.ID, p.Name)
		return 0
	case "remove":
		if len(rest) < 1 || len(rest) > 1 {
			if len(rest) > 1 {
				fmt.Fprintf(os.Stderr, "yolo profile remove: unexpected argument %q\n", rest[1])
			}
			return profileUsage()
		}
		id, ok := resolveProfile("remove", root, rest[0])
		if !ok {
			return 1
		}
		if err := config.Remove(root, id); err != nil {
			fmt.Fprintf(os.Stderr, "yolo profile remove: %v\n", err)
			return 1
		}
		fmt.Println(id)
		return 0
	case "copy":
		flags, err := pullDescFlags(rest)
		if err != nil {
			fmt.Fprintf(os.Stderr, "yolo profile copy: %v\n", err)
			return profileUsage()
		}
		if len(flags.pos) < 2 {
			return profileUsage()
		}
		if len(flags.pos) > 2 {
			fmt.Fprintf(os.Stderr, "yolo profile copy: unexpected argument %q\n", flags.pos[2])
			return profileUsage()
		}
		srcID, ok := resolveProfile("copy", root, flags.pos[0])
		if !ok {
			return 1
		}
		p, err := config.Copy(root, srcID, flags.pos[1], flags.desc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "yolo profile copy: %v\n", err)
			return 1
		}
		fmt.Printf("%s  %s\n", p.ID, p.Name)
		return 0
	default:
		return profileUsage()
	}
}
