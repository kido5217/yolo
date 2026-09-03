package tui

import "time"

// S6.1 startup loading spinner (deviation 237): the ported upstream
// startup-loading.tsx timing + text. The 500 ms show delay avoids the flash
// when hydration finishes fast; the min hold keeps the shown span >= 3 s;
// the two-state text marks the ready (finishing) phase.
const (
	startupShowDelay   = 500 * time.Millisecond
	startupMinHold     = 3 * time.Second
	startupTextLoading = "Loading sessions..."
	startupTextReady   = "Finishing startup..."
)

// loadShowMsg fires after the 500 ms show delay: it arms the shown state +
// the shown stamp (the min-hold origin).
type loadShowMsg struct{}

// loadDoneMsg fires at the end of the min-hold (or immediately when the hold
// is already expired): it hides the loading line.
type loadDoneMsg struct{}
