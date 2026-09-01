// providerdlg.go — the /connect picker (S3.4): the select over
// store.Providers (priority-sorted, "Popular"|"Providers" categories, the
// trailing "Other" custom option) + the API-key flow. Deviation 192: no
// oauth wire — known providers go straight to the API-key prompt; custom ids
// through the ported regex + @ai-sdk/ strip.

package tui

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/store"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// customProviderIDValue is the "Other" option's value (the upstream
// __opencode_custom_provider__ ported).
const customProviderIDValue = "__yolo_custom_provider__"

// customProviderIDErr is the invalid-id toast (upstream message verbatim).
const customProviderIDErr = "Provider ids must start with a lowercase letter or number and only use lowercase letters, numbers, hyphens, and underscores"

// customProviderIDRe is the ported upstream id validation regex.
var customProviderIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-_]*$`)

// normalizeCustomProviderID strips the leading @ai-sdk/ prefix and validates
// the rest; it returns "" when the id is invalid.
func normalizeCustomProviderID(s string) string {
	s = strings.TrimPrefix(s, "@ai-sdk/")
	if !customProviderIDRe.MatchString(s) {
		return ""
	}
	return s
}

// providerPriority is the ported PROVIDER_PRIORITY (unknown → 99).
func providerPriority(id string) int {
	switch id {
	case "opencode":
		return 0
	case "opencode-go":
		return 1
	case "openai":
		return 2
	case "github-copilot":
		return 3
	case "anthropic":
		return 4
	case "google":
		return 5
	default:
		return 99
	}
}

// providerCategory is the option category from the priority (< 99 → the
// ported "Popular" bucket).
func providerCategory(priority int) string {
	if priority < 99 {
		return "Popular"
	}
	return "Providers"
}

// providerDescription is the ported known-id description map (the yolo ids
// are mostly unknown → ""); unknown → "".
func providerDescription(id string) string {
	switch id {
	case "opencode":
		return "OpenCode's hosted models"
	case "opencode-go":
		return "OpenCode Go models"
	case "openai":
		return "OpenAI models"
	case "github-copilot":
		return "GitHub Copilot"
	case "anthropic":
		return "Anthropic models"
	case "google":
		return "Google models"
	default:
		return ""
	}
}

// providerOptions is the provider select's option list: a copy of
// store.Providers sorted by (priority, name lc, ID) — the ported
// PROVIDER_PRIORITY + the "Popular"|"Providers" categories + the known-id
// description map + the provider status footer — with the trailing "Other"
// custom option. No isCurrent (the upstream provider select has none).
func providerOptions(st *store.State) []selectOption {
	provs := make([]protocol.Provider, len(st.Providers))
	copy(provs, st.Providers)
	sort.SliceStable(provs, func(i, j int) bool {
		pi, pj := providerPriority(provs[i].ID), providerPriority(provs[j].ID)
		if pi != pj {
			return pi < pj
		}
		ni, nj := strings.ToLower(provs[i].Name), strings.ToLower(provs[j].Name)
		if ni != nj {
			return ni < nj
		}
		return provs[i].ID < provs[j].ID
	})
	opts := make([]selectOption, 0, len(provs)+1)
	for _, p := range provs {
		opts = append(opts, selectOption{
			title:       p.Name,
			description: providerDescription(p.ID),
			footer:      providerStatusText(p.Auth),
			category:    providerCategory(providerPriority(p.ID)),
			value:       p.ID,
		})
	}
	opts = append(opts, selectOption{
		title:       "Other",
		description: "Custom provider id",
		value:       customProviderIDValue,
	})
	return opts
}

// providerDlg is the /connect picker payload: the select + the API-key /
// custom-id flow. th is the theme at open (the pinned view takes no theme
// arg — the sessionsDlg convention, deviation 200).
type providerDlg struct {
	sel *selectModel
	th  theme.Theme
}

// handleKey drives the select (the modal stack consumes esc/ctrl+c first).
func (d *providerDlg) handleKey(a *App, k tea.KeyPressMsg) []tea.Cmd {
	if d.sel == nil {
		return nil
	}
	return d.sel.handleKey(a, k)
}

// view renders the select (the modal stack draws the panel chrome); a nil
// sel — the pre-catalog open — renders the loading hint.
func (d *providerDlg) view(w, h int) string {
	if d.sel == nil {
		return title.Render("Connect a provider") + "\n" + d.th.TextMuted().Render("  loading…")
	}
	return d.sel.view(w, h, d.th)
}

// openProviderDialog pushes the /connect picker (dlgMedium — the upstream
// DialogSelect default) and fetches the catalog. A nil sel = the pre-catalog
// loading hint; syncProviderSel builds the select once the catalog is
// hydrated (open-time or on arrival).
func (a *App) openProviderDialog() []tea.Cmd {
	d := &providerDlg{th: a.theme}
	a.pushModal(dialog{kind: dlgProvider, provider: d}, dlgMedium, nil)
	a.syncProviderSel()
	return a.emit(a.fetchCatalogCmd())
}

// syncProviderSel (re)builds the open provider select on the catalog (the
// syncModelSel mirror): it builds the select when the catalog first
// hydrates (nil sel = loading) and re-seeds the options on arrival. No
// selection re-anchor (the provider select has no isCurrent).
func (a *App) syncProviderSel() {
	d := a.dlg.provider()
	if d == nil {
		return
	}
	if d.sel == nil {
		if len(a.store.Providers) == 0 {
			return
		}
		d.sel = selectNew("Connect a provider", "Search", providerOptions(&a.store),
			nil, func(app *App, o selectOption) { app.providerSelectPick(o) }, nil)
	}
	d.sel.options = providerOptions(&a.store)
}

// providerSelectPick is the select's onSelect: the custom option opens the
// id prompt, a known provider the API-key form (deviation 192: the upstream
// auth-method select is not ported — the API-key form opens directly).
func (a *App) providerSelectPick(o selectOption) {
	v, _ := o.value.(string)
	if v == customProviderIDValue {
		a.openProviderIDForm()
		return
	}
	if v == "" {
		return
	}
	a.openKeyForm(v, false)
}

// openProviderIDForm pushes the custom-id prompt (the upstream DialogPrompt
// "Other"). Invalid ids toast the verbatim message and re-open the prompt
// (the upstream re-prompt; the cascade already closed the submitted form,
// so the re-open is a push — the provider dialog stays on top).
func (a *App) openProviderIDForm() []tea.Cmd {
	form := buildInputForm(a.theme, "Other", "", "Provider id", "")
	return a.openFormModal(form, dlgMedium, func(app *App, f *huh.Form) {
		id := normalizeCustomProviderID(f.GetString("value"))
		if id == "" {
			app.toast(customProviderIDErr)
			app.openProviderIDForm()
			return
		}
		app.openKeyForm(id, true)
	}, nil)
}

// openKeyForm pushes the API-key prompt: the known-provider description is
// "API key for <id>"; the custom-id description is the saved-credential note
// (the upstream message adapted to yolo.jsonc). An empty value re-opens the
// prompt (the upstream `if (!value) return` guard — the cascade already
// closed the form, so the re-open honors the upstream "stay").
func (a *App) openKeyForm(id string, custom bool) []tea.Cmd {
	desc := "API key for " + id
	if custom {
		desc = "This only stores a credential. Configure the provider in yolo.jsonc to use it."
	}
	form := buildInputForm(a.theme, "API key", desc, "API key", "")
	return a.openFormModal(form, dlgMedium, func(app *App, f *huh.Form) {
		v := f.GetString("value")
		if v == "" {
			app.openKeyForm(id, custom)
			return
		}
		app.emit(app.authCmd(id, v))
	}, nil)
}

// authCmd stores the API key for the provider (PUT /auth/{id}). The custom
// flag is derived from the catalog at exec time (the yolo wire is
// API-key-only — deviation 192 — and the custom id is never in the
// catalog).
func (a *App) authCmd(providerID, key string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := a.Auth(ctx, providerID, key, false)
		return authMsg{providerID: providerID, custom: !a.providerInCatalog(providerID), err: err}
	}
}

// authMsg reports the API-key save result (custom = the id is not in the
// catalog).
type authMsg struct {
	providerID string
	custom     bool
	err        error
}

// providerInCatalog reports whether the id is a catalog provider.
func (a *App) providerInCatalog(id string) bool {
	for _, p := range a.store.Providers {
		if p.ID == id {
			return true
		}
	}
	return false
}

// applyAuth lands the API-key save: an error toasts the error string and
// the dialog stays (the upstream stays on the api-key failure); success on
// a catalog provider closes and opens the model dialog (the upstream
// dialog.replace(DialogModel)); success on a custom id toasts the
// saved-credential note and closes.
func (a *App) applyAuth(m authMsg) tea.Cmd {
	if m.err != nil {
		a.toast(m.err.Error())
		return nil
	}
	if m.custom {
		a.toast("Saved credential for " + m.providerID + ". Configure it in yolo.jsonc to use it.")
		a.closeTopModal()
		return nil
	}
	a.closeTopModal()
	return a.openModelDialog()[0]
}
