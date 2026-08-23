//go:build race

package tui

import "time"

// Wall-clock threshold for TestDraftSoftEnterAmortized under the race
// detector, which slows this string-heavy path several times over. The
// iteration count is unchanged, so a quadratic draft re-scan (minutes)
// still fails loudly against the relaxed bound (linear ~10 s here).
const draftAmortizedLimit = 40 * time.Second
