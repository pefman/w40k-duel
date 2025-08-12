package game

import (
	"fmt"
	"strconv"
	"strings"
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

    // Normalize ability flags
    has := func(key string) bool {
        key = strings.ToLower(strings.TrimSpace(key))
        for _, a := range w.Abilities {
            if strings.Contains(strings.ToLower(a), key) { return true }
        }
        // also infer from weapon name/desc tokens if needed (not available here)
        return false
    }
    // Record abilities summary upfront
    if len(w.Abilities) > 0 {
        logs = append(logs, fmt.Sprintf("Weapon Abilities: [%s]", strings.Join(w.Abilities, ", ")))
    }

    torrent := has("torrent") // auto-hits
    sustainedHits := 0 // e.g., Sustained Hits X (X can be 1..6)
    for _, a := range w.Abilities {
        al := strings.ToLower(strings.TrimSpace(a))
        if strings.HasPrefix(al, "sustained hits") {
            // Extract number after label
            parts := strings.Fields(al)
            n := 0
            for _, p := range parts {
                if v, err := strconv.Atoi(strings.Trim(p, "+")); err == nil { n = v; break }
            }
            if n <= 0 { n = 1 }
            if n > 6 { n = 6 }
            sustainedHits = n
        }
    }
    lethalHits := has("lethal hits") // crit 6s to wound become auto-wounds? In 10th, crit 6s to hit auto-wound. We'll apply on hits.
    twinLinked := has("twin-linked") // re-roll wounds
    devastating := has("devastating wounds") // 6s to wound spill mortals (we'll treat as max damage)

    if torrent { logs = append(logs, "Torrent active: attacks automatically hit") }
    if sustainedHits > 0 { logs = append(logs, fmt.Sprintf("Sustained Hits %d active: each critical hit adds +%d hit(s)", sustainedHits, sustainedHits)) }
    if lethalHits { logs = append(logs, "Lethal Hits active: critical hit (6) converts to auto-wound") }
    if twinLinked { logs = append(logs, "Twin-linked active: re-roll failed wound rolls once") }
    if devastating { logs = append(logs, "Devastating Wounds active: critical wound (6) converts to maximum damage") }

    // Attacks
    attacks := rollExpr(rng, w.Attacks)
    sp.Attacks.Count = attacks
    logs = append(logs, fmt.Sprintf("Attacks A=%s -> %d", strings.TrimSpace(w.Attacks), attacks))

    // Hits
    sp.Hits.Target = w.Skill
    logs = append(logs, fmt.Sprintf("To Hit: needs %d+", w.Skill))
    hits := 0
    critAutoWounds := 0 // from lethal hits (6s to hit)
    for i := 0; i < attacks; i++ {
        var roll int
        if torrent {
            roll = 6 // treat as auto-hit; log as such
            sp.Hits.Rolls = append(sp.Hits.Rolls, roll)
            hits++
            logs = append(logs, fmt.Sprintf("Hit (Torrent) %d: auto-hit", i+1))
        } else {
            roll = 1 + rng.Intn(6)
            sp.Hits.Rolls = append(sp.Hits.Rolls, roll)
        if roll >= w.Skill && roll != 1 {
                hits++
                logs = append(logs, fmt.Sprintf("Hit roll %d: %d -> HIT (needs %d+)", i+1, roll, w.Skill))
                if lethalHits && roll == 6 {
                    critAutoWounds++
            logs = append(logs, "Lethal Hits: critical hit converts to auto-wound")
                }
                if sustainedHits > 0 && roll == 6 {
                    hits += sustainedHits // add extra hits
            logs = append(logs, fmt.Sprintf("Sustained Hits: +%d additional hit(s)", sustainedHits))
                }
            } else {
                logs = append(logs, fmt.Sprintf("Hit roll %d: %d -> MISS (needs %d+)", i+1, roll, w.Skill))
            }
        }
    }
    sp.Hits.Success = hits
    logs = append(logs, fmt.Sprintf("Hits total: %d", hits))

    // Wounds
    woundTN := woundTarget(w.Strength, def.T)
    logs = append(logs, fmt.Sprintf("To Wound base: S %d vs T %d -> needs %d+", w.Strength, def.T, woundTN))
    // Anti- keywords override wound threshold when matching defender keywords
    antiTN := 0
    antiKW := ""
    antiMatchedDefKW := ""
    for _, a := range w.Abilities {
        al := strings.ToLower(a)
        if strings.HasPrefix(al, "anti-") {
            // Parse e.g., "Anti-Infantry 4+"
            // Extract keyword and TN
            kw := ""
            tn := 0
            parts := strings.SplitN(strings.TrimPrefix(al, "anti-"), " ", 2)
            if len(parts) == 2 {
                kw = strings.TrimSpace(parts[0])
                s := strings.TrimSpace(parts[1])
                // s like "4+" or "5+"
                if len(s) >= 2 && s[len(s)-1] == '+' {
                    if n, err := strconv.Atoi(strings.TrimSpace(s[:len(s)-1])); err == nil { tn = n }
                }
            }
            if kw != "" && tn >= 2 && tn <= 6 {
                // if defender has matching keyword (case-insensitive substring match)
                for _, dk := range def.Keywords {
                    if strings.Contains(strings.ToLower(dk), kw) {
                        if antiTN == 0 || tn < antiTN {
                            antiTN = tn
                            antiKW = kw
                            antiMatchedDefKW = dk
                        }
                        break
                    }
                }
            }
        }
    }
    if antiTN > 0 && antiTN < woundTN {
        logs = append(logs, fmt.Sprintf("Anti-%s %d+ applies (defender has '%s'): override wound target to %d+", antiKW, antiTN, antiMatchedDefKW, antiTN))
        woundTN = antiTN
    }
    sp.Wounds.Target = woundTN
    wounds := 0
    attempts := hits
    // auto-wounds from lethal hits add without rolling
    if critAutoWounds > 0 {
        wounds += critAutoWounds
        attempts -= critAutoWounds
        logs = append(logs, fmt.Sprintf("Lethal Hits auto-wounds added: +%d", critAutoWounds))
    }
    for i := 0; i < attempts; i++ {
        roll := 1 + rng.Intn(6)
        var passes bool
        if roll >= woundTN && roll != 1 { passes = true }
        if !passes && twinLinked {
            // twin-linked: re-roll failed wound once
            r2 := 1 + rng.Intn(6)
            logs = append(logs, fmt.Sprintf("Twin-linked re-roll: %d -> %d (needs %d+)", roll, r2, woundTN))
            roll = r2
            if roll >= woundTN && roll != 1 { passes = true }
        }
        sp.Wounds.Rolls = append(sp.Wounds.Rolls, roll)
        if passes {
            wounds++
            logs = append(logs, fmt.Sprintf("Wound roll %d: %d -> WOUND (needs %d+)", i+1, roll, woundTN))
        } else {
            logs = append(logs, fmt.Sprintf("Wound roll %d: %d -> FAIL (needs %d+)", i+1, roll, woundTN))
        }
    }
    sp.Wounds.Success = wounds
    logs = append(logs, fmt.Sprintf("Wounds total: %d", wounds))

    // Saves
    // Compute save threshold with explanation
    effSave := def.Sv - w.AP
    if effSave < 2 { effSave = 2 }
    if effSave > 6 { effSave = 7 }
    usedInv := false
    saveTN := effSave
    if def.InvSv > 0 && def.InvSv < effSave { saveTN = def.InvSv; usedInv = true }
    sp.Saves.Target = saveTN
    saved := 0
    unsaved := 0
    effSaveStr := ""
    if effSave == 7 { effSaveStr = "no save" } else { effSaveStr = fmt.Sprintf("%d+", effSave) }
    if usedInv {
        logs = append(logs, fmt.Sprintf("Saves: AP %d modifies Sv to %s, Invulnerable %d+ is better -> using Invulnerable", w.AP, effSaveStr, def.InvSv))
    } else {
        logs = append(logs, fmt.Sprintf("Saves: AP %d modifies Sv to %s", w.AP, effSaveStr))
    }
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
        var dmg int
        if devastating && i < len(sp.Wounds.Rolls) && sp.Wounds.Rolls[i] == 6 {
            // Model devastating wounds as max damage on crit wounds
            // Try to infer max from dice expr (e.g., D6 -> 6, D3 -> 3). Fallback: roll.
            expr := strings.TrimSpace(w.Damage)
            if strings.HasPrefix(strings.ToUpper(expr), "D6") { dmg = 6 } else if strings.HasPrefix(strings.ToUpper(expr), "D3") { dmg = 3 }
            if dmg == 0 { dmg = rollExpr(rng, w.Damage) }
            logs = append(logs, fmt.Sprintf("Devastating Wounds: critical wound -> max damage from %s = %d", strings.TrimSpace(w.Damage), dmg))
        } else {
            dmg = rollExpr(rng, w.Damage)
        }
        sp.Damage.Rolls = append(sp.Damage.Rolls, dmg)
        totalDmg += dmg
        logs = append(logs, fmt.Sprintf("Damage roll %d: %s -> %d", i+1, strings.TrimSpace(w.Damage), dmg))
    }
    // Feel No Pain: parse from defender abilities ("Feel No Pain X+" or "FNP X+") and roll once per damage to ignore
    fnpTN := 0
    fnpSrc := ""
    for _, a := range def.Abilities {
        al := strings.ToLower(strings.TrimSpace(a))
        if strings.HasPrefix(al, "feel no pain") || strings.HasPrefix(al, "fnp") {
            // find an X+ token
            fields := strings.Fields(al)
            for _, f := range fields {
                f = strings.TrimSpace(f)
                if len(f) >= 2 && f[len(f)-1] == '+' {
                    if n, err := strconv.Atoi(strings.Trim(f[:len(f)-1], "+ ")) ; err == nil {
                        if n >= 2 && n <= 6 {
                            if fnpTN == 0 || n < fnpTN { fnpTN = n; fnpSrc = a }
                        }
                    }
                }
            }
        }
    }
    // Apply FNP if present
    if fnpTN > 0 && totalDmg > 0 {
        rolls := make([]int, 0, totalDmg)
        ignored := 0
        for i := 0; i < totalDmg; i++ {
            r := 1 + rng.Intn(6)
            rolls = append(rolls, r)
            if r >= fnpTN && r != 1 { ignored++ }
        }
        logs = append(logs, fmt.Sprintf("Feel No Pain %d+ (%s): rolls %v -> ignored %d damage", fnpTN, fnpSrc, rolls, ignored))
        if ignored > 0 { totalDmg -= ignored }
        if totalDmg < 0 { totalDmg = 0 }
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
