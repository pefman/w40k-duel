package stats

import (
	"sync"
	"time"
)

// UserStats stores statistics for each user (in-memory for demo)
var (
    statsMu   sync.Mutex
    userStats = make(map[string]map[string]interface{})
    // Global daily max-attack (by date string YYYY-MM-DD UTC)
    dailyMax  = make(map[string]map[string]interface{})
)

func SaveUserStats(username string, stats map[string]interface{}) {
    statsMu.Lock()
    defer statsMu.Unlock()
    userStats[username] = stats
}

func GetUserStats(username string) map[string]interface{} {
    statsMu.Lock()
    defer statsMu.Unlock()
    if s, ok := userStats[username]; ok {
        return s
    }
    return map[string]interface{}{}
}

// SaveGlobalMaxAttack updates the per-day global max attack if the provided attack is larger
// Attack map keys: username, unit, weapon, wounds(int), damage(int), at(optional time)
func SaveGlobalMaxAttack(attack map[string]interface{}) {
    if attack == nil { return }
    // date key in UTC
    dateKey := time.Now().UTC().Format("2006-01-02")
    getInt := func(m map[string]interface{}, key string) int {
        if vv, ok := m[key]; ok {
            switch t := vv.(type) {
            case float64:
                return int(t)
            case int:
                return t
            case int64:
                return int(t)
            }
        }
        return 0
    }
    statsMu.Lock()
    defer statsMu.Unlock()
    cur := dailyMax[dateKey]
    if cur == nil {
        dailyMax[dateKey] = attack
        return
    }
    cd, cw := getInt(cur, "damage"), getInt(cur, "wounds")
    nd, nw := getInt(attack, "damage"), getInt(attack, "wounds")
    if nd > cd || (nd == cd && nw > cw) {
        dailyMax[dateKey] = attack
    }
}

func GetGlobalMaxAttackToday() map[string]interface{} {
    dateKey := time.Now().UTC().Format("2006-01-02")
    statsMu.Lock()
    defer statsMu.Unlock()
    if m, ok := dailyMax[dateKey]; ok && m != nil {
        return m
    }
    return map[string]interface{}{}
}
