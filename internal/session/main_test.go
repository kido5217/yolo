package session_test

import (
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"
)

// TestMain fails the run if the suite left goroutines behind: every harness
// cleans up its own (event collectors, reply watcher, shells, DB), so the
// live count must return to the pre-suite baseline. Dependency-free census
// under the pinned-deps rule (goleak would need an explicit user call).
//
// The suite runs serially by choice: the harnesses spawn real shell
// processes and share a per-process permission service, so t.Parallel would
// couple unrelated tests through process and permission state; the suite
// wall time (~1.7 s) does not pay for the coupling.
func TestMain(m *testing.M) {
	base := runtime.NumGoroutine()
	code := m.Run()
	if code != 0 {
		os.Exit(code)
	}
	deadline := time.Now().Add(2 * time.Second)
	for n := runtime.NumGoroutine(); n > base; {
		if time.Now().After(deadline) {
			buf := make([]byte, 1<<20)
			nn := runtime.Stack(buf, true)
			fmt.Fprintf(os.Stderr, "goroutine leak: %d live after suite (baseline %d):\n%s\n", n, base, buf[:nn])
			os.Exit(1)
		}
		time.Sleep(20 * time.Millisecond)
		n = runtime.NumGoroutine()
	}
	os.Exit(0)
}
