package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"charm.land/huh/v2"
)

var enterKey = tea.KeyPressMsg{Code: tea.KeyEnter}

// driveCmds replays the emitted cmds through the app's update loop until a
// fixed point (huh v2's submit cascade: the key emits NextField, the form's
// nextFieldMsg emits the nextGroup cmd, the group's nextGroupMsg emits the
// form's SubmitCmd — each leg is a msg the app must deliver). BatchMsgs fan
// out like the production bubbletea event loop does. Cmds the emit sink
// appends mid-round (a test capture of cmds emitted from inside the update,
// e.g. the form's onConfirm) are drained by the next round. The replay runs
// the INTERNAL updateMsg (not App.Update): the toast TTL tick that Update
// would drain is a UI-timing detail outside the functional cascade —
// completing it during the replay would expire a toast raised mid-cascade
// before the test can observe it (deviation 200).
// updateKey mirrors the production event loop for one message: the cmd
// Update returns is queued for execution (bubbletea runs it; the test
// replays it in driveCmds).
func updateKey(a *recApp, msg tea.Msg) {
	if _, cmd := a.Update(msg); cmd != nil {
		a.Cmds = append(a.Cmds, cmd)
	}
}

func driveCmds(t *testing.T, a *recApp) {
	t.Helper()
	for i := 0; i < 8; i++ {
		if len(a.Cmds) == 0 {
			return
		}
		cmds := a.Cmds
		a.Cmds = nil
		var next []tea.Cmd
		for _, c := range cmds {
			if c == nil {
				continue
			}
			switch m := c().(type) {
			case tea.BatchMsg:
				next = append(next, m...)
			case tea.Cmd:
				next = append(next, m)
			default:
				if rc := a.updateMsg(m); rc != nil {
					next = append(next, rc)
				}
			}
		}
		if len(next) == 0 {
			if len(a.Cmds) == 0 {
				return
			}
			continue // the emit sink appended cmds this round: drain them
		}
		a.Cmds = append(a.Cmds, next...)
	}
}

func TestConfirmFormSubmit(t *testing.T) {
	a := testApp()
	a.size = tea.WindowSizeMsg{Width: 80, Height: 24}
	confirmed := false
	a.openFormModal(buildConfirmForm(a.theme, "do it?", "the thing"), dlgMedium,
		func(*App, *huh.Form) { confirmed = true }, nil)
	updateKey(a, enterKey)
	driveCmds(t, a)
	if !confirmed || len(a.dlg.items) != 0 {
		t.Fatalf("confirmed=%v depth=%d, want true/0", confirmed, len(a.dlg.items))
	}
}

func TestConfirmFormEscCancels(t *testing.T) {
	a := testApp()
	a.size = tea.WindowSizeMsg{Width: 80, Height: 24}
	confirmed, cancelled := false, false
	a.openFormModal(buildConfirmForm(a.theme, "t", "d"), dlgMedium,
		func(*App, *huh.Form) { confirmed = true },
		func(*App) { cancelled = true })
	updateKey(a, press(tea.KeyEscape))
	driveCmds(t, a)
	if confirmed || !cancelled || len(a.dlg.items) != 0 {
		t.Fatalf("confirmed=%v cancelled=%v depth=%d, want false/true/0",
			confirmed, cancelled, len(a.dlg.items))
	}
}

func TestConfirmFormKeysToggle(t *testing.T) {
	a := testApp()
	a.size = tea.WindowSizeMsg{Width: 80, Height: 24}
	var got bool
	a.openFormModal(buildConfirmForm(a.theme, "t", "d"), dlgMedium,
		func(_ *App, f *huh.Form) { got = f.GetBool("confirm") }, nil)
	// the pills start on "confirm" (true); left toggles to "cancel"
	updateKey(a, tea.KeyPressMsg{Code: tea.KeyLeft})
	driveCmds(t, a)
	updateKey(a, enterKey)
	driveCmds(t, a)
	if got {
		t.Fatalf("left must move the selection to cancel (false)")
	}
}

func TestAlertFormSingleButton(t *testing.T) {
	// The form sizes itself on the last terminal size (openFormModal
	// feeds it a WindowSizeMsg); an unsized huh v2 form renders blank.
	a := testApp()
	a.size = tea.WindowSizeMsg{Width: 80, Height: 24}
	a.openFormModal(buildAlertForm(a.theme, "heads up", "something happened"),
		dlgMedium, nil, nil)
	v := strings.ToLower(a.dlg.form().form.View())
	if !strings.Contains(v, "ok") {
		t.Fatalf("alert view missing the ok button: %q", v)
	}
	if strings.Contains(v, "cancel") || strings.Contains(v, "confirm") {
		t.Fatalf("alert view leaked a second button: %q", v)
	}
}

func TestInputFormTypedSubmit(t *testing.T) {
	a := testApp()
	a.size = tea.WindowSizeMsg{Width: 80, Height: 24}
	var got string
	a.openFormModal(buildInputForm(a.theme, "rename", "the session", "session name", "old"), dlgMedium,
		func(_ *App, f *huh.Form) { got = f.GetString("value") }, nil)
	driveCmds(t, a) // focus the field (openFormModal only queued the Init cmds)
	updateKey(a, press('h'))
	driveCmds(t, a)
	updateKey(a, press('i'))
	driveCmds(t, a)
	updateKey(a, enterKey)
	driveCmds(t, a)
	if got != "oldhi" || len(a.dlg.items) != 0 {
		t.Fatalf("got=%q depth=%d, want oldhi/0", got, len(a.dlg.items))
	}
}

func TestInputFormEscCancels(t *testing.T) {
	a := testApp()
	a.size = tea.WindowSizeMsg{Width: 80, Height: 24}
	cancelled := false
	a.openFormModal(buildInputForm(a.theme, "t", "d", "ph", "x"), dlgMedium,
		func(*App, *huh.Form) { t.Fatalf("confirm must not fire on esc") },
		func(*App) { cancelled = true })
	a.handleKey(press(tea.KeyEscape))
	if !cancelled || len(a.dlg.items) != 0 {
		t.Fatalf("cancelled=%v depth=%d, want true/0", cancelled, len(a.dlg.items))
	}
}

func TestInputFormPlaceholder(t *testing.T) {
	a := testApp()
	a.size = tea.WindowSizeMsg{Width: 80, Height: 24}
	a.openFormModal(buildInputForm(a.theme, "t", "d", "session name", ""), dlgMedium, nil, nil)
	driveCmds(t, a) // placeholder only renders once the textinput is focused
	v := a.dlg.form().form.View()
	// The placeholder renders interleaved with SGR runs (the cursor char is
	// reversed, the placeholder is dim), so check the stripped copy (deviation
	// 172).
	if !strings.Contains(stripANSI(v), "session name") {
		t.Fatalf("placeholder missing from the view: %q", v)
	}
}
