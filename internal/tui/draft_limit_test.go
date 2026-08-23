//go:build !race

package tui

import "time"

// Wall-clock threshold for TestDraftSoftEnterAmortized without the race
// detector.
const draftAmortizedLimit = 5 * time.Second
