package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
)

// recApp is App with the emitted-cmd capture sink installed (the production
// App deliberately has no test hook).
type recApp struct {
	*App
	Cmds []tea.Cmd
}

func newRecApp(c *client.Service, s store.Store, startSessionID string) *recApp {
	ra := &recApp{App: NewApp(c, s, startSessionID)}
	ra.emitSink = func(cmds ...tea.Cmd) { ra.Cmds = append(ra.Cmds, cmds...) }
	return ra
}
