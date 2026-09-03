package session_test

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain fails the run if the suite left goroutines behind: every harness
// cleans up its own (event collectors, reply watcher, shells, DB), so goleak
// must find nothing after the suite.
//
// The suite runs serially by choice: the harnesses spawn real shell
// processes and share a per-process permission service, so t.Parallel would
// couple unrelated tests through process and permission state; the suite
// wall time (~1.7 s) does not pay for the coupling.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
