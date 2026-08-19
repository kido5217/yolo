package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"

	"github.com/kido5217/yolo/internal/protocol"
)

// promptModel is the always-focused single-line input with the slash command
// menu (T25). When the value starts with "/" the filtered command menu is
// open and arrows/enter/esc drive it; "\"+enter accumulates soft-entered
// lines into draft until the final enter sends them.
type promptModel struct {
	input textinput.Model
	sel   int
	draft string
}

// busyToast is the locked message for a send attempted while the session is
// busy, whether by the store-side pre-check or a server 409 (client.ErrBusy).
const busyToast = "abort or wait (esc aborts)"

var promptEnter = key.NewBinding(key.WithKeys("enter"))

// slashActive reports whether the slash menu is open.
func (pm *promptModel) slashActive() bool {
	v := pm.input.Value()
	return v != "" && strings.HasPrefix(v, "/")
}

// commandAliases maps canonical command names to accepted aliases. Aliases
// are input forms only: the menu surfaces the canonical name.
var commandAliases = map[string][]string{"/quit": {"/exit"}}

// menuItems filters the known commands by the typed "/prefix", matching both
// canonical names and their aliases. It returns nil when the menu is closed,
// else the filtered (possibly empty) list in server order.
func (pm *promptModel) menuItems(cmds []protocol.Command) []protocol.Command {
	if !pm.slashActive() {
		return nil
	}
	prefix := pm.input.Value()[1:]
	out := []protocol.Command{}
	for _, c := range cmds {
		match := strings.HasPrefix(c.Name[1:], prefix)
		if !match {
			for _, alias := range commandAliases[c.Name] {
				if strings.HasPrefix(alias[1:], prefix) {
					match = true
					break
				}
			}
		}
		if match {
			out = append(out, c)
		}
	}
	return out
}

// menuLines is the menu block's height in lines (0 closed, 1 open-but-empty).
func (pm *promptModel) menuLines(cmds []protocol.Command) int {
	items := pm.menuItems(cmds)
	if items == nil {
		return 0
	}
	if len(items) == 0 {
		return 1
	}
	return len(items)
}

func (pm *promptModel) menuView(cmds []protocol.Command) string {
	items := pm.menuItems(cmds)
	if items == nil {
		return ""
	}
	if len(items) == 0 {
		return dim.Render("  no match")
	}
	var b strings.Builder
	for i, c := range items {
		line := "  " + c.Name + "  " + c.Description
		if i == pm.sel {
			line = cursor.Render(line)
		} else {
			line = dim.Render(line)
		}
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(line)
	}
	return b.String()
}

// view renders the prompt line (the textinput carries the "> " prompt).
func (pm *promptModel) view() string { return pm.input.View() }

// moveMenuSel moves the selection by d with wraparound (n items).
func (pm *promptModel) moveMenuSel(n, d int) {
	if n == 0 {
		pm.sel = 0
		return
	}
	pm.sel = ((pm.sel+d)%n + n) % n
}
