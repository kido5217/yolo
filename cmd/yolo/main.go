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
	"github.com/kido5217/yolo/internal/tui"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
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
	case "version":
		printVersion()
		return 0
	default:
		return tuiCmd(args)
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `yolo — Go port of opencode (v1.18.18 wire contract)

Usage:
  yolo [<sessionID>] [--dir DIR]   start the TUI (optionally resume a session)
  yolo serve [--addr ADDR]         run the core server only (default http://127.0.0.1:4096)
  yolo auth <subcommand>           manage credentials (list | add <provider> [key] | remove <provider>)
  yolo [-v|--version]              print version (same as: yolo version)
  yolo help                        this help
`)
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

// buildDeps assembles the full core stack for workDir: config loader (real
// XDG env), storage DB, bus, permission service, provider registry, tools
// and the session engine. YOLO_LLM=fake (with YOLO_FAKE_SCRIPT) selects the
// scripted fake driver + static catalog so the suite never hits the
// network; any other env runs the live registry.
func buildDeps(workDir string) (*server.Deps, func(), error) {
	loader := config.Loader{} // nil Env view = real process environment
	homeDir, err := config.Home()
	if err != nil {
		return nil, nil, err
	}
	dataDir, err := config.DataYoloDir()
	if err != nil {
		return nil, nil, err
	}
	cacheDir, err := config.CacheYoloDir()
	if err != nil {
		return nil, nil, err
	}
	logger := log.New(dataDir)

	// Retention sweep for full outputs of truncated bash runs (upstream
	// runs it hourly; v1 runs it once at startup). Best effort: the
	// hygiene pass must not block boot.
	_ = tool.CleanOutputDir(filepath.Join(dataDir, "tool-output"))

	fail := func(err error) (*server.Deps, func(), error) {
		logger.Error("startup failed", "error", err)
		logger.Close()
		return nil, nil, err
	}

	db, err := openDB(filepath.Join(dataDir, "storage", "yolo.db"))
	if err != nil {
		return fail(err)
	}
	if v, verr := db.SchemaVersion(); verr == nil {
		logger.Info("storage open", "path", filepath.Join(dataDir, "storage", "yolo.db"), "schema_version", v)
	}
	closeDB := func() {
		_ = db.Close()
		logger.Close()
	}

	b := bus.New()
	deps := &server.Deps{
		DB:     db,
		Bus:    b,
		Perm:   permission.New(db, b, logger, dataDir),
		Config: loader,
		Log:    logger,
		// Dirs are resolved above: the server never re-resolves XDG itself
		// (a broken home is a buildDeps error, not a per-request 500).
		Dirs:    config.Dirs{Home: homeDir, Data: dataDir, Cache: cacheDir},
		WorkDir: workDir,
	}

	globalDir, err := config.GlobalYoloDir()
	if err != nil {
		closeDB()
		return fail(err)
	}
	cfg, err := loader.LoadAt(globalDir, workDir)
	if err != nil {
		closeDB()
		return fail(err)
	}

	fake, err := server.FakeFromEnv(envMap())
	if err != nil {
		closeDB()
		return fail(err)
	}
	var drivers map[string]llm.Driver
	if fake != nil {
		deps.Prov = provider.NewStaticForTest()
		drivers = map[string]llm.Driver{"kido": fake}
	} else {
		prov, err := provider.New(context.Background(), cfg, nil, provider.Dirs{})
		if err != nil {
			closeDB()
			return fail(err)
		}
		deps.Prov = prov
	}
	engine, err := session.New(session.Deps{
		DB: db, Bus: deps.Bus, Prov: deps.Prov, Perm: deps.Perm,
		Tools: tool.Registry(), DataDir: dataDir, Log: logger,
		Cfg: func(dir string) (*protocol.Config, error) {
			return loader.LoadAt(globalDir, dir)
		},
		Drivers: drivers,
	})
	if err != nil {
		closeDB()
		return fail(err)
	}
	deps.Engine = engine
	return deps, closeDB, nil
}

func openDB(path string) (*storage.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return storage.Open(path)
}

func envMap() map[string]string {
	env := map[string]string{}
	for _, kv := range os.Environ() {
		if i := strings.IndexByte(kv, '='); i > 0 {
			env[kv[:i]] = kv[i+1:]
		}
	}
	return env
}

// tuiCmd runs `yolo [<sessionID>] [--dir DIR]`: in-process server on an
// ephemeral port + the TUI.
func tuiCmd(args []string) int {
	fs := flag.NewFlagSet("yolo", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dir := fs.String("dir", "", "project directory (default CWD)")
	if err := fs.Parse(args); err != nil {
		usage(os.Stderr)
		return 2
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
	deps, closeDB, err := buildDeps(wd)
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

	app := tui.NewApp(cl, store.State{}, sessionID)
	deps.Log.Info("tui start", "workdir", wd)
	_, runErr := tea.NewProgram(app).Run()
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

// drainCtx stops active turns and the listener within the caller's ctx
// budget, then closes the logger.
func drainCtx(deps *server.Deps, srv *server.Server, ctx context.Context) {
	deps.Engine.Shutdown(ctx)
	srv.Shutdown(ctx)
	deps.Log.Close()
}

// drain stops active turns and the listener within one 5 s budget, then
// closes the logger (process-exit path for serve and TUI mode).
func drain(deps *server.Deps, srv *server.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:4096", "listen address")
	showVer := fs.Bool("v", false, "print version and exit")
	showVerLong := fs.Bool("version", false, "print version and exit")
	// ExitOnError: Parse prints and os.Exit's on bad flags, never returns
	// a non-nil error.
	_ = fs.Parse(args)
	if *showVer || *showVerLong {
		printVersion()
		return 0
	}
	wd, err := workDir("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "yolo serve: %v\n", err)
		return 1
	}
	deps, closeDB, err := buildDeps(wd)
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
		if len(rest) < 1 {
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
		if len(rest) < 1 {
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
