package main

import (
	"encoding/json"
	"net/http"

	"github.com/pefman/w40k-duel/internal/stats"
)

// POST /api/stats/save
func SaveStatsHandler(w http.ResponseWriter, r *http.Request) {
	type SaveReq struct {
		Username string                 `json:"username"`
		Stats    map[string]interface{} `json:"stats"`
	}
	var req SaveReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", 400)
		return
	}
	if req.Username == "" {
		http.Error(w, "missing username", 400)
		return
	}
	// Merge with existing stats and keep the biggest maxAttack
	existing := stats.GetUserStats(req.Username)
	// Shallow copy existing into merged
	merged := map[string]interface{}{}
	for k, v := range existing { merged[k] = v }
	for k, v := range req.Stats {
		if k == "maxAttack" { continue }
		merged[k] = v
	}
	// Handle maxAttack specially
	if v, ok := req.Stats["maxAttack"]; ok && v != nil {
		newMA, _ := v.(map[string]interface{})
		if newMA != nil {
			// normalize numbers possibly as float64 from JSON
			getInt := func(m map[string]interface{}, key string) int {
				if vv, ok := m[key]; ok {
					switch t := vv.(type) {
					case float64:
						return int(t)
					case int:
						return t
					case int64:
						return int(t)
					case json.Number:
						if n, err := t.Int64(); err == nil { return int(n) }
					}
				}
				return 0
			}
			best := merged["maxAttack"]
			if bestMap, ok := best.(map[string]interface{}); ok && bestMap != nil {
				// keep the one with higher damage, tie-break by wounds
				bd, bw := getInt(bestMap, "damage"), getInt(bestMap, "wounds")
				nd, nw := getInt(newMA, "damage"), getInt(newMA, "wounds")
				if nd > bd || (nd == bd && nw > bw) {
					merged["maxAttack"] = newMA
				} // else keep existing
			} else {
				merged["maxAttack"] = newMA
			}
		}
	}
	stats.SaveUserStats(req.Username, merged)
	w.WriteHeader(204)
}

// GET /api/stats/get?username=...
func GetStatsHandler(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	if username == "" {
		http.Error(w, "missing username", 400)
		return
	}
	s := stats.GetUserStats(username)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s)
}

// GET /api/stats/max-attack?username=...
func GetMaxAttackHandler(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	if username == "" {
		http.Error(w, "missing username", 400)
		return
	}
	stats := stats.GetUserStats(username)
	var out interface{}
	if v, ok := stats["maxAttack"]; ok {
		out = v
	} else {
		out = map[string]interface{}{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// POST /api/stats/max-attack/today
// Body: { attack: { username, unit, weapon, wounds, damage } }
func PostGlobalMaxAttackToday(w http.ResponseWriter, r *http.Request) {
	type Req struct { Attack map[string]interface{} `json:"attack"` }
	var req Req
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", 400)
		return
	}
	if req.Attack == nil { w.WriteHeader(204); return }
	stats.SaveGlobalMaxAttack(req.Attack)
	w.WriteHeader(204)
}

// GET /api/stats/max-attack/today
func GetGlobalMaxAttackToday(w http.ResponseWriter, r *http.Request) {
	out := stats.GetGlobalMaxAttackToday()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}
