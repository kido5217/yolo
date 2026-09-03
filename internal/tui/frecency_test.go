package tui

import (
	"fmt"
	"math"
	"testing"
)

func TestFrecencyScore(t *testing.T) {
	now := int64(1_000_000_000_000)
	t.Run("score = frequency / (1 + age-days)", func(t *testing.T) {
		e := frecencyEntry{Frequency: 10, LastOpen: now - dayMs} // one day old
		if got := frecencyScore(&e, now); math.Abs(got-5.0) > 1e-9 {
			t.Fatalf("one day old = %v, want 5", got)
		}
		e2 := frecencyEntry{Frequency: 5, LastOpen: now} // just now
		if got := frecencyScore(&e2, now); math.Abs(got-5.0) > 1e-9 {
			t.Fatalf("just now = %v, want 5", got)
		}
		if got := frecencyScore(nil, now); got != 0 {
			t.Fatalf("absent = %v, want 0", got)
		}
	})
}

func TestUpdateFrecency(t *testing.T) {
	entries := []frecencyEntry{{Path: "a", Frequency: 1, LastOpen: 1}}
	got := updateFrecency(entries, "a", 100)
	if len(got) != 1 || got[0].Frequency != 2 || got[0].LastOpen != 100 {
		t.Fatalf("refresh a: %v", got)
	}
	got = updateFrecency(got, "b", 200)
	if len(got) != 2 {
		t.Fatalf("append b: %v", got)
	}
	if got[0].Path != "b" {
		t.Fatalf("sort lastOpen desc: %v", got)
	}
}

func TestParseFrecency(t *testing.T) {
	t.Run("dedupe by path (last wins)", func(t *testing.T) {
		entries := []frecencyEntry{{Path: "a", Frequency: 1, LastOpen: 1}, {Path: "a", Frequency: 9, LastOpen: 2}}
		got := parseFrecency(entries)
		if len(got) != 1 || got[0].Frequency != 9 {
			t.Fatalf("dedupe last-wins: %v", got)
		}
	})
	t.Run("cap at 1000", func(t *testing.T) {
		big := make([]frecencyEntry, 1200)
		for i := range big {
			big[i] = frecencyEntry{Path: fmt.Sprintf("p%d", i), Frequency: 1, LastOpen: int64(i)}
		}
		if got := parseFrecency(big); len(got) != maxFrecencyEntries {
			t.Fatalf("cap: %d, want %d", len(got), maxFrecencyEntries)
		}
	})
}
