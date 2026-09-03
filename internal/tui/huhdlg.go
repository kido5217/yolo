package tui

import (
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/tui/theme"
)

// huhFormDlg is the huh-form payload of a modal field dialog (S2.3): the
// form is a child tea.Model — the stack forwards every non-esc key, watches
// f.State, and fires onConfirm on completion (esc/ctrl+c close via the
// stack, firing the dialog's onClose).
type huhFormDlg struct {
	form      *huh.Form
	onConfirm func(*App, *huh.Form)
}

// updateMsg forwards one message to the form and drives the completion
// callbacks on the form's State transitions (upstream: submit returns the
// form's value). Note: huh's Model is a v1-style interface — Form.Update
// returns (Model, tea.Cmd), and *Form.Update always returns itself; assert
// the concrete type back onto f.form.
func (f *huhFormDlg) updateMsg(a *App, msg tea.Msg) []tea.Cmd {
	m, cmd := f.form.Update(msg)
	if fm, ok := m.(*huh.Form); ok {
		f.form = fm
	}
	switch f.form.State {
	case huh.StateCompleted:
		done := f.onConfirm
		f.onConfirm = nil
		a.closeTopModal()
		if done != nil {
			done(a, f.form)
		}
		return nil
	case huh.StateAborted:
		a.closeTopModal()
		return nil
	}
	if cmd != nil {
		return []tea.Cmd{cmd}
	}
	return nil
}

// handleKey forwards a keypress to the form.
func (f *huhFormDlg) handleKey(a *App, k tea.KeyPressMsg) []tea.Cmd {
	return f.updateMsg(a, k)
}

// forwardMsg re-feeds huh's internal form-progress messages (unexported
// group/field msg types — nextFieldMsg, nextGroupMsg, …) and huh's submit
// command back into the form: App.updateMsg has no case for them, so the
// stack routes them here instead of dropping them.
func (f *huhFormDlg) forwardMsg(a *App, msg tea.Msg) []tea.Cmd {
	if _, isCmd := msg.(tea.Cmd); isCmd {
		return nil
	}
	return f.updateMsg(a, msg)
}

// openFormModal pushes a huh-form modal: the form sizes itself on the last
// terminal size, its Init cmd seeds the fields, onConfirm fires on submit
// and onClose on esc/ctrl+c.
func (a *App) openFormModal(form *huh.Form, size dlgSize, onConfirm func(*App, *huh.Form), onClose func(*App)) []tea.Cmd {
	f := &huhFormDlg{form: form, onConfirm: onConfirm}
	a.pushModal(dialog{kind: dlgForm, form: f}, size, onClose)
	cmds := a.emit(form.Init())
	if a.size.Width > 0 {
		if m, cc := f.form.Update(tea.WindowSizeMsg{Width: a.size.Width, Height: a.size.Height}); cc != nil {
			if fm, ok := m.(*huh.Form); ok {
				f.form = fm
			}
			cmds = append(cmds, cc)
		}
	}
	return cmds
}

// buildAlertForm is the upstream dialog-alert (a single ok pill).
func buildAlertForm(th theme.Theme, title, description string) *huh.Form {
	v := true
	return huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Key("confirm").
			Title(title).Description(description).
			Affirmative("ok").Negative("").Value(&v),
	)).WithTheme(themeDialog(th)).WithShowHelp(false)
}

// buildConfirmForm is the upstream dialog-confirm (confirm/cancel pills,
// starting on confirm).
func buildConfirmForm(th theme.Theme, title, description string) *huh.Form {
	v := true
	return huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Key("confirm").
			Title(title).Description(description).
			Affirmative("Confirm").Negative("Cancel").Value(&v),
	)).WithTheme(themeDialog(th)).WithShowHelp(false)
}

// buildInputForm is the upstream dialog-prompt (a single text input with
// placeholder + initial value; return submits, esc cancels).
func buildInputForm(th theme.Theme, title, description, placeholder, initial string) *huh.Form {
	v := initial
	return huh.NewForm(huh.NewGroup(
		huh.NewInput().Key("value").
			Title(title).Description(description).
			Placeholder(placeholder).Value(&v),
	)).WithTheme(themeDialog(th)).WithShowHelp(false)
}

// themeDialog maps the resolved theme to huh's Styles (deviation 170 —
// borderless field boxes, the upstream dialog look; tokens from the
// resolved theme; a zero Theme degrades to huh's default palette).
func themeDialog(th theme.Theme) huh.Theme {
	return huh.ThemeFunc(func(isDark bool) *huh.Styles {
		s := huh.ThemeBase(isDark)
		s.Focused.Base = lipgloss.NewStyle() // strip the default thick-left border
		s.Blurred.Base = lipgloss.NewStyle()
		s.FieldSeparator = lipgloss.NewStyle().SetString("\n")
		if c, ok := th.Color("text"); ok {
			fg := lipgloss.Color(c.Hex()[:7])
			s.Focused.Title = lipgloss.NewStyle().Foreground(fg).Bold(true)
			s.Blurred.Title = lipgloss.NewStyle().Foreground(fg)
		}
		if c, ok := th.Color("textMuted"); ok {
			muted := lipgloss.Color(c.Hex()[:7])
			s.Focused.Description = lipgloss.NewStyle().Foreground(muted)
			s.Blurred.Description = lipgloss.NewStyle().Foreground(muted)
			s.Group.Description = s.Blurred.Description
		}
		if c, ok := th.Color("primary"); ok {
			bg := lipgloss.Color(c.Hex()[:7])
			sel := th.SelectedForeground(c)
			fg := lipgloss.Color(sel.Hex()[:7])
			s.Focused.FocusedButton = lipgloss.NewStyle().
				Padding(0, 2).MarginRight(1).
				Foreground(fg).Background(bg).Bold(true)
		}
		if c, ok := th.Color("textMuted"); ok {
			s.Focused.BlurredButton = lipgloss.NewStyle().
				Padding(0, 2).MarginRight(1).
				Foreground(lipgloss.Color(c.Hex()[:7]))
		}
		return s
	})
}
