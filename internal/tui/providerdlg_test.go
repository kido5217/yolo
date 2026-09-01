package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
)

// providerFixtureS3: a known "popular" id, an unknown yolo id, and the
// custom option. (model_test.go's providerFixture is the S2 shape — this
// fixture is the S3 sort-order check.)
func providerFixtureS3() []protocol.Provider {
	return []protocol.Provider{
		{ID: "kido", Name: "Kido", Auth: &protocol.ProviderAuth{Status: "loaded"}},
		{ID: "anthropic", Name: "Anthropic", Auth: &protocol.ProviderAuth{RequiresKey: true, Status: "missing"}},
		{ID: "openai", Name: "OpenAI", Auth: &protocol.ProviderAuth{Status: "not-required"}},
	}
}

func openProviderDlg(t *testing.T) *recApp {
	t.Helper()
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	a.openProviderDialog()
	a.applyCatalog(catalogMsg{provs: providerFixtureS3(), agents: agentFixture()})
	a.Cmds = nil
	return a
}

func TestNormalizeCustomProviderID(t *testing.T) {
	tests := []struct{ in, want string }{
		{"my-provider", "my-provider"},
		{"my_provider", "my_provider"},
		{"9lives", "9lives"},
		{"@ai-sdk/openai", "openai"},
		{"@ai-sdk/openrouter", "openrouter"},
		{"OpenAI", ""},   // uppercase
		{"-leading", ""}, // leading hyphen
		{"a b", ""},      // space
		{"", ""},
	}
	for _, tc := range tests {
		if got := normalizeCustomProviderID(tc.in); got != tc.want {
			t.Fatalf("normalize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestProviderDialogRender(t *testing.T) {
	t.Run("priority order, categories, Other tail, status footers", func(t *testing.T) {
		a := openProviderDlg(t)
		got := stripANSI(a.dlg.provider().view(80, 26))
		if !strings.Contains(got, "Connect a provider") {
			t.Fatalf("title missing:\n%s", got)
		}
		// openai (2) + anthropic (4) sort before the unknown kido (99);
		// the custom option is last.
		iA, iO, iK, iC := strings.Index(got, "Anthropic"), strings.Index(got, "OpenAI"), strings.Index(got, "Kido"), strings.Index(got, "Other")
		if iA < 0 || iO < 0 || iK < 0 || iC < 0 || !(iO < iA && iA < iK && iK < iC) {
			t.Fatalf("order wrong (openai < anthropic < kido < Other):\n%s", got)
		}
		for _, cat := range []string{"Popular", "Providers"} {
			if !strings.Contains(got, cat) {
				t.Fatalf("category %q missing:\n%s", cat, got)
			}
		}
		for _, tok := range []string{"● loaded", "○ missing", "· not-required"} {
			if !strings.Contains(got, tok) {
				t.Fatalf("status footer %q missing:\n%s", tok, got)
			}
		}
	})

	t.Run("empty catalog shows the loading hint", func(t *testing.T) {
		ts := testutil.Boot(t)
		c := client.New(ts.URL, ts.Dir)
		a := newRecApp(c, store.State{}, "")
		t.Cleanup(a.Close)
		a.openProviderDialog()
		a.Cmds = nil
		got := stripANSI(a.dlg.provider().view(80, 24))
		if !strings.Contains(got, "loading…") {
			t.Fatalf("loading hint missing:\n%s", got)
		}
	})
}

func TestProviderDialogFlow(t *testing.T) {
	t.Run("known provider: key form, auth success replaces with the model dialog", func(t *testing.T) {
		a := openProviderDlg(t)
		// select "OpenAI" (the second row) and enter
		a.handleKey(press(tea.KeyDown))
		a.handleKey(press(tea.KeyEnter))
		if a.dlg.form() == nil {
			t.Fatal("the API-key form must be on top")
		}
		got := stripANSI(a.dlg.form().form.View())
		if !strings.Contains(got, "API key") {
			t.Fatalf("the key form title missing:\n%s", got)
		}
		updateKey(a, press('k'))
		updateKey(a, press('3'))
		updateKey(a, enterKey)
		driveCmds(t, a) // the submit cascade + the auth cmd round-trip
		if a.dlg.model() == nil {
			top, _ := a.dlg.top()
			t.Fatalf("success must replace the dialog with the model dialog: top=%v", top.kind)
		}
	})

	t.Run("invalid custom id: the verbatim error toast, the id prompt re-opens", func(t *testing.T) {
		a := openProviderDlg(t)
		// navigate to the "Other" row (the last) and enter
		for i := 0; i < 3; i++ {
			a.handleKey(press(tea.KeyDown))
		}
		a.handleKey(press(tea.KeyEnter))
		if a.dlg.form() == nil {
			t.Fatal("the custom-id prompt must be on top")
		}
		updateKey(a, press('B')) // "B" -> invalid (uppercase)
		updateKey(a, enterKey)
		driveCmds(t, a)
		if len(a.toasts) != 1 || !strings.Contains(a.toasts[len(a.toasts)-1].msg,
			"Provider ids must start with a lowercase letter or number") {
			t.Fatalf("invalid-id toast wrong: %v", a.toasts)
		}
		// the id prompt re-opened (the upstream re-prompt)
		if a.dlg.form() == nil {
			t.Fatal("the id prompt must re-open after the invalid id")
		}
	})

	t.Run("custom provider: auth success toasts the saved-credential note and closes", func(t *testing.T) {
		a := openProviderDlg(t)
		for i := 0; i < 3; i++ {
			a.handleKey(press(tea.KeyDown))
		}
		a.handleKey(press(tea.KeyEnter))
		updateKey(a, press('m'))
		updateKey(a, press('y'))
		updateKey(a, enterKey)
		driveCmds(t, a) // the id prompt resolves -> the key form
		if a.dlg.form() == nil {
			t.Fatal("the key form must be on top after the valid id")
		}
		updateKey(a, press('k'))
		updateKey(a, enterKey)
		driveCmds(t, a)
		if !a.dlg.empty() {
			t.Fatalf("the custom flow must close: depth=%d", len(a.dlg.items))
		}
		if len(a.toasts) != 1 || !strings.Contains(a.toasts[0].msg,
			"Saved credential for my. Configure it in yolo.jsonc to use it.") {
			t.Fatalf("saved-credential toast wrong: %v", a.toasts)
		}
	})
}

// TestTUIProviderDialog is the teatest leg: /connect opens the dialog on the
// real stack (the provider list from the real catalog).
func TestTUIProviderDialog(t *testing.T) {
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))

	teatest.WaitFor(t, tm.Output(), hasLines("New session"), teatest.WithDuration(5*time.Second))
	suiteType(tm, "/connect")
	tm.Send(press(tea.KeyEnter))
	// ONE merged condition: the title + the custom tail (the status footers
	// depend on the real catalog — kept out of the condition).
	teatest.WaitFor(t, tm.Output(), hasLines("Connect a provider", "Other"), teatest.WithDuration(5*time.Second))

	tm.Send(press(tea.KeyEscape))
	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
