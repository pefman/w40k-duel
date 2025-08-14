package stats

// This file contains helpers around daily stats. It complements stats.go.

// ResetDaily clears the in-memory global daily max map.
// Intended for tests and dev convenience.
func ResetDaily() {
	statsMu.Lock()
	defer statsMu.Unlock()
	for k := range dailyMax {
		delete(dailyMax, k)
	}
}
