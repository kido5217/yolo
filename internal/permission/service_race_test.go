package permission

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/kido5217/yolo/internal/bus"
	"github.com/kido5217/yolo/internal/storage"
)

// TestConcurrentSessionsNoRace pins the concurrency-1 P0 finding: two+
// concurrent sessions sharing one Service (the real wiring — one service per
// process) must race-free on decisionFor/Ask/Reply. The original finding
// (unlocked s.dataDir read vs SetDataDir write) was eliminated when
// SetDataDir was removed (39a196e): dataDir is a constructor constant. This
// test guards the invariant. Run with -race.
func TestConcurrentSessionsNoRace(t *testing.T) {
	t.Parallel()
	db, err := storage.Open(filepath.Join(t.TempDir(), "yolo.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	b := bus.New()
	svc := New(db, b, nil, t.TempDir())

	// Two "projects": different always-rule histories per session, so
	// decisionFor exercises the db + rules paths in both.
	sessions := []string{"ses_a", "ses_b"}
	for _, sid := range sessions {
		if err := db.CreateSession(t.Context(), storage.SessionRow{ID: sid, ProjectDir: "/w", Agent: "build", Model: "k"}); err != nil {
			t.Fatal(err)
		}
		// Seed a per-session always-rule (response='always' + always_json
		// patterns) so decisionFor exercises the db AlwaysRules path.
		if err := db.SavePermission(t.Context(), storage.PermissionRow{
			RequestID: "seed-" + sid, SessionID: sid, Action: "read",
			Resource: "/x/*", Response: "always", AlwaysJSON: `["/x/*"]`, TimeCreated: 1,
		}); err != nil {
			t.Fatal(err)
		}
	}

	var wg sync.WaitGroup
	ctx := context.Background()
	for i := range 32 {
		wg.Go(func() {
			sid := sessions[i%2]
			req := Request{
				// PreDecision empty -> decisionFor runs (builtins + db
				// always-rules), exercising the shared service concurrently.
				RequestID:  fmt.Sprintf("r-%02d", i),
				SessionID:  sid,
				Agent:      "build",
				Permission: "read",
				Resources:  []string{"/x/file.txt"},
			}
			if _, err := svc.Ask(ctx, req); err != nil {
				t.Errorf("Ask: %v", err)
			}
		})
	}
	wg.Wait()
}
