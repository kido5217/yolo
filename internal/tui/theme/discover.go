package theme

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

// ThemeDirs is the port of the upstream discover directory list
// (theme.tsx:38-44): the injected global config dir FIRST, then <dir>/.yolo
// for every dir from cwd up to and including the filesystem root (upstream:
// <dir>/.opencode under the flat ~/.config/opencode root — yolo: the flat
// ~/.config/yolo, spec §3). No dedupe: Discover's later-dir-wins override
// follows exactly this order (upstream object-assignment order).
func ThemeDirs(globalYoloDir, cwd string) []string {
	dirs := []string{globalYoloDir}
	for current := cwd; ; current = filepath.Dir(current) {
		dirs = append(dirs, filepath.Join(current, ".yolo"))
		if filepath.Dir(current) == current {
			break
		}
	}
	return dirs
}

// Discover is the port of upstream discoverThemes (theme.tsx:52-61): for
// each dir in order, scan <dir>/themes/ for *.json entries — dotfile names
// included, symlink entries followed (upstream Glob.scan dot:true,
// symlink:true); name = base name minus ".json"; later dirs override
// earlier names. A missing themes dir is skipped (upstream Glob.scan yields
// nothing); an unreadable or unparseable file is a hard error (upstream
// JSON.parse throws → the whole discover fails; the caller's catch sets
// active to "yolo" — S0.7). Values are returned RAW: the IsTheme filter
// is the caller's job (theme.tsx:137-140, S0.7), not Discover's.
func Discover(dirs []string) (map[string]any, error) {
	result := map[string]any{}
	for _, dir := range dirs {
		entries, err := os.ReadDir(filepath.Join(dir, "themes"))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("theme discovery %s: %w", dir, err)
		}
		for _, e := range entries {
			name := e.Name()
			if !strings.HasSuffix(name, ".json") {
				continue
			}
			full := filepath.Join(dir, "themes", name)
			info, err := os.Stat(full) // follows symlinks (upstream symlink:true)
			if err != nil {
				return nil, fmt.Errorf("theme discovery %s: %w", full, err)
			}
			if !info.Mode().IsRegular() {
				continue // a directory named *.json is not a file match
			}
			data, err := os.ReadFile(full)
			if err != nil {
				return nil, fmt.Errorf("theme discovery %s: %w", full, err)
			}
			var v any
			if err := json.Unmarshal(data, &v); err != nil {
				return nil, fmt.Errorf("theme discovery %s: %w", full, err)
			}
			result[strings.TrimSuffix(name, ".json")] = v
		}
	}
	return result, nil
}

// WatchThemeSignals is the port of upstream subscribeRefresh
// (theme.tsx:46-49): SIGUSR2 → refresh, kept per spec §3. The 250/1000 ms
// debounce (theme.tsx:82, 235-244) lives in the engine (S0.7), not here —
// every signal is forwarded. stop() stops the forwarding goroutine and
// restores the default SIGUSR2 disposition.
func WatchThemeSignals(refresh func()) (stop func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGUSR2)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-ch:
				refresh()
			case <-done:
				return
			}
		}
	}()
	return func() {
		close(done)
		signal.Stop(ch)
		signal.Reset(syscall.SIGUSR2)
	}
}
