package storage

// agentOrDefault normalizes the per-message agent ("build" when unset).
func agentOrDefault(a string) string {
	if a == "" {
		return "build"
	}
	return a
}
