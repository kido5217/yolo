package server

import (
	"testing"

	"github.com/kido5217/yolo/internal/config"
)

// TestAuthSnapshotIsDecoupledFromLiveStore pins that handlers that read the
// auth store lock-free after authSnapshot() never observe later in-place
// mutations (handleAuthPut/Delete mutate s.authStore under authMu); the
// snapshot must be a copy, not an alias of the live map.
func TestAuthSnapshotIsDecoupledFromLiveStore(t *testing.T) {
	// Construct the way build() does (NewServer returns a lifecycle-only
	// wrapper whose auth state lives on the handler-bound instance).
	s := &Server{Deps: Deps{WorkDir: t.TempDir(), Dirs: config.Dirs{Data: t.TempDir()}}}
	if err := s.initAuth(); err != nil {
		t.Fatal(err)
	}
	s.authMu.Lock()
	s.authStore.Set("openrouter", "old-key")
	s.authMu.Unlock()
	snap := s.authSnapshot()
	s.authMu.Lock()
	s.authStore.Set("openrouter", "new-key")
	s.authMu.Unlock()
	e, ok := snap["openrouter"]
	if !ok || e.Key != "old-key" {
		t.Fatalf("snapshot aliases the live store: key = %+v, want %q", e, "old-key")
	}
}
