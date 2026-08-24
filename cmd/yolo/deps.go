package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"

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
)

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
