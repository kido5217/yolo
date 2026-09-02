package tui

import "sort"

// frecencyEntry is one file-path frecency record (the port of upstream
// FrecencyEntry, prompt/frecency.tsx). The key is a scope-relative path
// (deviation 224 — upstream cwd-resolves to an absolute path); the JSON
// keys match the upstream camelCase shape so the KV round-trip (the
// S5.3 persistence, deviation 223) reloads into the same keys.
type frecencyEntry struct {
	Path      string `json:"path"`
	Frequency int    `json:"frequency"`
	LastOpen  int64  `json:"lastOpen"`
}

const (
	// maxFrecencyEntries caps the frecency list (upstream
	// MAX_FRECENCY_ENTRIES).
	maxFrecencyEntries = 1000
	// dayMs is a day in milliseconds (the upstream age divisor).
	dayMs = 86_400_000
)

// frecencyScore ranks an entry (the ported upstream calculateFrecency:
// frequency / (1 + age-days); absent or zero-frequency → 0; a negative
// age clamps to 0).
func frecencyScore(e *frecencyEntry, now int64) float64 {
	if e == nil || e.Frequency == 0 {
		return 0
	}
	ageMs := now - e.LastOpen
	if ageMs < 0 {
		ageMs = 0
	}
	return float64(e.Frequency) / (1 + float64(ageMs)/dayMs)
}

// parseFrecency normalizes the list: dedupe by path (last wins), sort
// lastOpen desc, cap maxFrecencyEntries (the ported upstream
// parseFrecency). Ties keep first-appearance order (upstream's stable
// sort over the dedupe order).
func parseFrecency(entries []frecencyEntry) []frecencyEntry {
	byPath := make(map[string]frecencyEntry, len(entries))
	order := make([]string, 0, len(entries))
	for _, e := range entries {
		if _, ok := byPath[e.Path]; !ok {
			order = append(order, e.Path)
		}
		byPath[e.Path] = e
	}
	out := make([]frecencyEntry, 0, len(byPath))
	for _, p := range order {
		out = append(out, byPath[p])
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].LastOpen > out[j].LastOpen
	})
	if len(out) > maxFrecencyEntries {
		out = out[:maxFrecencyEntries]
	}
	return out
}

// updateFrecency refreshes the entry for relPath (frequency+1,
// lastOpen=now) or appends a fresh one, then parses (the ported upstream
// updateFrecency; the key is scope-relative, deviation 224).
func updateFrecency(entries []frecencyEntry, relPath string, now int64) []frecencyEntry {
	for i := range entries {
		if entries[i].Path == relPath {
			entries[i].Frequency++
			entries[i].LastOpen = now
			return parseFrecency(entries)
		}
	}
	fresh := frecencyEntry{Path: relPath, Frequency: 1, LastOpen: now}
	return parseFrecency(append(entries, fresh))
}
