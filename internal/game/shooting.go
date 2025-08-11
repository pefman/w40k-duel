package game

import (
	"fmt"
)

func woundTarget(S, T int) int {
    // Returns target roll (2-6) needed to wound
    switch {
    case S >= 2*T:
        return 2
    case S > T:
        return 3
    case S == T:
        return 4
    case S*2 <= T:
        return 6
    default:
        return 5
    }
}

func bestSaveThreshold(sv, inv int, ap int) int {
    // sv is 2..6 where 2 means 2+, inv 0 if none; ap negative makes saves worse
    eff := sv - ap // ap e.g., -1 -> 3+ becomes 2 worse? We assume AP is negative numbers in data, so subtracting yields harder save
    if eff < 2 { eff = 2 }
    if eff > 6 { eff = 7 } // 7 means no save
    if inv > 0 && inv < eff {
        eff = inv
    }
    return eff
}

// ResolveShooting executes a single weapon volley from attacker to defender and logs steps
func ResolveShooting(att UnitSnapshot, def UnitSnapshot, w WeaponSnapshot) ShootingResult {
    logs := []string{}
    rng := newRNG()
    sp := &ShootingSubphases{}

    // Attacks
    attacks := rollExpr(rng, w.Attacks)
    sp.Attacks.Count = attacks
    logs = append(logs, fmt.Sprintf("Attacks rolled: %d", attacks))

    // Hits
    sp.Hits.Target = w.Skill
    hits := 0
    for i := 0; i < attacks; i++ {
        roll := 1 + rng.Intn(6)
        sp.Hits.Rolls = append(sp.Hits.Rolls, roll)
        if roll >= w.Skill && roll != 1 {
            hits++
            logs = append(logs, fmt.Sprintf("Hit roll %d: %d -> HIT (needs %d+)", i+1, roll, w.Skill))
        } else {
            logs = append(logs, fmt.Sprintf("Hit roll %d: %d -> MISS (needs %d+)", i+1, roll, w.Skill))
        }
    }
    sp.Hits.Success = hits
    logs = append(logs, fmt.Sprintf("Hits total: %d", hits))

    // Wounds
    woundTN := woundTarget(w.Strength, def.T)
    sp.Wounds.Target = woundTN
    wounds := 0
    for i := 0; i < hits; i++ {
        roll := 1 + rng.Intn(6)
        sp.Wounds.Rolls = append(sp.Wounds.Rolls, roll)
        if roll >= woundTN && roll != 1 {
            wounds++
            logs = append(logs, fmt.Sprintf("Wound roll %d: %d -> WOUND (needs %d+)", i+1, roll, woundTN))
        } else {
            logs = append(logs, fmt.Sprintf("Wound roll %d: %d -> FAIL (needs %d+)", i+1, roll, woundTN))
        }
    }
    sp.Wounds.Success = wounds
    logs = append(logs, fmt.Sprintf("Wounds total: %d", wounds))

    // Saves
    saveTN := bestSaveThreshold(def.Sv, def.InvSv, w.AP)
    sp.Saves.Target = saveTN
    saved := 0
    unsaved := 0
    for i := 0; i < wounds; i++ {
        roll := 1 + rng.Intn(6)
        sp.Saves.Rolls = append(sp.Saves.Rolls, roll)
        if roll >= saveTN && roll != 1 {
            saved++
            logs = append(logs, fmt.Sprintf("Save roll %d: %d -> SAVED (needs %d+)", i+1, roll, saveTN))
        } else {
            unsaved++
            logs = append(logs, fmt.Sprintf("Save roll %d: %d -> FAILED (needs %d+)", i+1, roll, saveTN))
        }
    }
    sp.Saves.Success = saved
    sp.Saves.Failed = unsaved
    logs = append(logs, fmt.Sprintf("Saves total: %d, Unsaved total: %d (TN %d+)", saved, unsaved, saveTN))

    // Damage
    totalDmg := 0
    for i := 0; i < unsaved; i++ {
        dmg := rollExpr(rng, w.Damage)
        sp.Damage.Rolls = append(sp.Damage.Rolls, dmg)
        totalDmg += dmg
        logs = append(logs, fmt.Sprintf("Damage roll %d: %d", i+1, dmg))
    }
    sp.Damage.Total = totalDmg
    remain := def.W - totalDmg
    if remain < 0 { remain = 0 }
    logs = append(logs, fmt.Sprintf("Total Damage: %d, Defender Wounds left: %d", totalDmg, remain))

    return ShootingResult{
        Logs:           logs,
        Attacks:        attacks,
        Hits:           hits,
        Wounds:         wounds,
        Saved:          saved,
        Unsaved:        unsaved,
        DamageTotal:    totalDmg,
        DefenderWounds: remain,
        Subphases:      sp,
    }
}
