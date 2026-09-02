package tui

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

// wantTipsPinnedSHA256 pins the ported tips set (root principle 3: the
// pin records the current intended content — the PORTED set, deviation
// 234; an intentional change re-baselines the pin in the same commit).
// Canonical form: noModelsTip first, then the 37 tips in order, each line
// followed by "\n". The constant is computed at Step 3 (the test prints
// the live hash) and re-baselined in the same commit.
const wantTipsPinnedSHA256 = "68cbcf2600c2f9c5c94f61b2a3d3a5f38162ee5e62e84141095646924c7f71db"

func tipsPinnedText() string {
	var b strings.Builder
	b.WriteString(noModelsTip)
	b.WriteByte('\n')
	for _, t := range tips {
		b.WriteString(t)
		b.WriteByte('\n')
	}
	return b.String()
}

func TestTipsPinned(t *testing.T) {
	sum := sha256.Sum256([]byte(tipsPinnedText()))
	if got := hex.EncodeToString(sum[:]); got != wantTipsPinnedSHA256 {
		t.Fatalf("tips sha256 = %s, want %s — re-baseline the pin in the same commit", got, wantTipsPinnedSHA256)
	}
}

// TestTipsShape pins the ported-set size (a silent drop/insert would be a
// data regression the pin catches too — the count is the cheap leg).
func TestTipsShape(t *testing.T) {
	if len(tips) != 37 {
		t.Fatalf("tips = %d entries, want 37", len(tips))
	}
	if noModelsTip == "" {
		t.Fatal("noModelsTip must be set")
	}
}

// TestTipsTokenIntegrity pins that every <binding> token in the tips
// resolves to a keymap binding (a dangling token would render "none"
// mid-sentence) and that every tipBindings entry is actually used.
func TestTipsTokenIntegrity(t *testing.T) {
	all := noModelsTip + "\n" + strings.Join(tips, "\n")
	for _, m := range tipTokenRe.FindAllStringSubmatch(all, -1) {
		if _, ok := Definitions[m[1]]; !ok {
			t.Fatalf("tip token <%s> has no keymap binding", m[1])
		}
	}
	for _, b := range tipBindings {
		if !strings.Contains(all, "<"+b+">") {
			t.Fatalf("tipBindings %s unused in the tips", b)
		}
	}
	if !strings.Contains(all, "{theme_count}") {
		t.Fatal("a tip must use the {theme_count} token")
	}
}

// TestParseTip pins the {highlight} markup port (upstream parse()).
func TestParseTip(t *testing.T) {
	parts := parseTip("Run {highlight}/connect{/highlight} to add an AI provider and start coding")
	if len(parts) != 3 || parts[0].hi || !parts[1].hi || parts[2].hi ||
		parts[0].text != "Run " || parts[1].text != "/connect" || parts[2].text != " to add an AI provider and start coding" {
		t.Fatalf("parse = %+v", parts)
	}
	parts = parseTip("plain text")
	if len(parts) != 1 || parts[0].hi || parts[0].text != "plain text" {
		t.Fatalf("plain parse = %+v", parts)
	}
	parts = parseTip("{highlight}a{/highlight} mid {highlight}b{/highlight}")
	if len(parts) != 3 {
		t.Fatalf("mixed parse = %+v", parts)
	}
}
