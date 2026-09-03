package tui

import (
	"strings"
	"testing"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/store"
)

// TestNumber pins the ported Locale.number (locale.ts:46-54): the
// ≥1e6 → "1.2M" / ≥1e3 → "1.2K" compact form, the plain string below
// (the identity under 1000 — the existing ↑123 ↓45 pins stay).
func TestNumber(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0"},
		{42, "42"},
		{999, "999"},
		{1000, "1.0K"},
		{1234, "1.2K"},
		{12345, "12.3K"},
		{1000000, "1.0M"},
		{1234567, "1.2M"},
	}
	for _, tt := range tests {
		if got := number(tt.n); got != tt.want {
			t.Fatalf("number(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

// TestFooterTokensCompact pins the restyled tokens segment: the K/M
// compact form over the ↑in ↓out arrows (the frozen segment set is kept,
// the numbers get the upstream format).
func TestFooterTokensCompact(t *testing.T) {
	a := footerApp(store.State{
		Live: true,
		Current: &protocol.Session{ID: "ses_1", Agent: "build",
			Model:  refModel("kido", "q"),
			Tokens: protocol.Tokens{Input: 12345, Output: 678}},
	})
	a.route = routeSession
	if got := stripANSI(a.footerView()); !strings.Contains(got, "↑12.3K ↓678") {
		t.Fatalf("footer = %q, want the compact ↑12.3K ↓678 tokens segment", got)
	}
}

// TestFooterContextPct pins the (pct%) context segment: shown only when
// the session model resolves (over store.Providers — the lazy catalog
// referent, deviation 241) to a Limit.Context > 0; pct = round(100 *
// total / context), total = the session aggregate
// input+output+reasoning+cache.read+cache.write.
func TestFooterContextPct(t *testing.T) {
	mk := func(provs []protocol.Provider) string {
		st := store.State{
			Live: true,
			Current: &protocol.Session{ID: "ses_1", Agent: "build",
				Model: refModel("kido", "q"),
				Tokens: protocol.Tokens{Input: 100, Output: 50, Reasoning: 25,
					Cache: protocol.CacheTokens{Read: 10, Write: 15}}},
		}
		st.Providers = provs
		a := footerApp(st)
		a.route = routeSession
		return stripANSI(a.footerView())
	}
	// total = 100+50+25+10+15 = 200 → 200/200 = 100%.
	if got := mk([]protocol.Provider{{ID: "kido", Models: map[string]protocol.Model{
		"q": {ID: "q", Limit: protocol.ModelLimit{Context: 200}},
	}}}); !strings.Contains(got, "(100%)") {
		t.Fatalf("footer = %q, want the (100%%) context segment", got)
	}
	// 200/1000 → 20%.
	if got := mk([]protocol.Provider{{ID: "kido", Models: map[string]protocol.Model{
		"q": {ID: "q", Limit: protocol.ModelLimit{Context: 1000}},
	}}}); !strings.Contains(got, "(20%)") {
		t.Fatalf("footer = %q, want the (20%%) context segment", got)
	}
	// no catalog: the segment is absent (the lazy referent); a zero context
	// limit: absent too.
	if got := mk(nil); strings.Contains(got, "%") {
		t.Fatalf("no-catalog footer = %q, want no context segment", got)
	}
	if got := mk([]protocol.Provider{{ID: "kido", Models: map[string]protocol.Model{
		"q": {ID: "q", Limit: protocol.ModelLimit{Context: 0}},
	}}}); strings.Contains(got, "%") {
		t.Fatalf("zero-limit footer = %q, want no context segment", got)
	}
}

// TestFooterCostOmitted pins the upstream cost convention (deviation 249):
// the Intl en-US USD shape "$%.2f", the segment omitted when cost == 0.
func TestFooterCostOmitted(t *testing.T) {
	a := footerApp(store.State{
		Live: true,
		Current: &protocol.Session{ID: "ses_1", Agent: "build",
			Model: refModel("kido", "q")},
	})
	a.route = routeSession
	if got := stripANSI(a.footerView()); strings.Contains(got, "$") {
		t.Fatalf("zero-cost footer = %q, want the cost segment omitted", got)
	}
	b := footerApp(store.State{
		Live: true,
		Current: &protocol.Session{ID: "ses_1", Agent: "build",
			Model: refModel("kido", "q"), Cost: 1.2346},
	})
	b.route = routeSession
	if got := stripANSI(b.footerView()); !strings.Contains(got, "$1.23") {
		t.Fatalf("footer = %q, want the $1.23 cost segment (2 decimals)", got)
	}
}
