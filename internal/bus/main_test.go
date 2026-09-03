package bus_test

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain fails the run if the suite left goroutines behind: the tests stop
// every pump they start, so goleak must find nothing after the suite.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
