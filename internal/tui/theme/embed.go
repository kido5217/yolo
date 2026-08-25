package theme

import (
	"embed"
)

// The 33 upstream theme assets, verbatim (strict-copy bar, spec §1).
//
//go:embed assets/*.json
var assetsFS embed.FS
