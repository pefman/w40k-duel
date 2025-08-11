package stats

import (
	"sync"
	"time"
)

type DailyTopDamage struct {
	Damage          int    `json:"damage"`
	Attacker        string `json:"attacker"`
	AttackerFaction string `json:"attacker_faction,omitempty"`
	AttackerUnit    string `json:"attacker_unit,omitempty"`
	Defender        string `json:"defender,omitempty"`
	Weapon          string `json:"weapon,omitempty"`
	Time            int64  `json:"time"`
}

type DailyWorstSave struct {
	Roll            int    `json:"roll"`
	Need            int    `json:"need"`
	Defender        string `json:"defender"`
	DefenderFaction string `json:"defender_faction,omitempty"`
	DefenderUnit    string `json:"defender_unit,omitempty"`
	Count           int    `json:"count"`
	Time            int64  `json:"time"`
}

type DailyStats struct {
	Date      string         `json:"date"`
	TopDamage DailyTopDamage `json:"top_damage"`
	WorstSave DailyWorstSave `json:"worst_save"`
}

var (
	dailyMu    sync.Mutex
	dailyState = DailyStats{Date: time.Now().Format("2006-01-02"), TopDamage: DailyTopDamage{Damage: 0}, WorstSave: DailyWorstSave{Roll: 7}}
)

func Get() DailyStats {
	dailyMu.Lock()
	defer dailyMu.Unlock()
	today := time.Now().Format("2006-01-02")
	if dailyState.Date != today {
		dailyState = DailyStats{Date: today, TopDamage: DailyTopDamage{Damage: 0}, WorstSave: DailyWorstSave{Roll: 7}}
	}
	return dailyState
}

func MaybeTopDamage(dmg int, attacker, aFac, aUnit, defender, weapon string) {
	if dmg <= 0 {
		return
	}
	dailyMu.Lock()
	defer dailyMu.Unlock()
	today := time.Now().Format("2006-01-02")
	if dailyState.Date != today {
		dailyState = DailyStats{Date: today, TopDamage: DailyTopDamage{Damage: 0}, WorstSave: DailyWorstSave{Roll: 7}}
	}
	if dmg > dailyState.TopDamage.Damage {
		dailyState.TopDamage = DailyTopDamage{Damage: dmg, Attacker: attacker, AttackerFaction: aFac, AttackerUnit: aUnit, Defender: defender, Weapon: weapon, Time: time.Now().Unix()}
	}
}

func MaybeWorstSave(minRoll int, need int, defender, dFac, dUnit string, count int) {
	if minRoll <= 0 || need <= 0 {
		return
	}
	dailyMu.Lock()
	defer dailyMu.Unlock()
	today := time.Now().Format("2006-01-02")
	if dailyState.Date != today {
		dailyState = DailyStats{Date: today, TopDamage: DailyTopDamage{Damage: 0}, WorstSave: DailyWorstSave{Roll: 7}}
	}
	if minRoll < dailyState.WorstSave.Roll {
		dailyState.WorstSave = DailyWorstSave{Roll: minRoll, Need: need, Defender: defender, DefenderFaction: dFac, DefenderUnit: dUnit, Count: count, Time: time.Now().Unix()}
	}
}
