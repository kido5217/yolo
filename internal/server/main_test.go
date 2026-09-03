package server_test

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain fails the run if the suite left goroutines behind: the harness
// cleans up per test (DB, server, request bodies, parked asks), so goleak
// must find nothing after the suite.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
