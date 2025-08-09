package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"
)

func (gs *GameServer) startBattle(match *Match) {
	match.State = "initiative"
	match.Player1.Status = "rolling_initiative"
	match.Player2.Status = "rolling_initiative"

	log.Printf("Battle started: %s vs %s - Rolling for initiative", match.Player1.Name, match.Player2.Name)

	// Both players need to roll D6 for initiative
	gs.sendToPlayer(match.Player1, map[string]interface{}{
		"type":     "battle_started",
		"opponent": match.Player2.Name,
		"phase":    "initiative",
		"message":  "Battle begins! Roll D6 for initiative - highest goes first!",
	})
	gs.sendToPlayer(match.Player2, map[string]interface{}{
		"type":     "battle_started",
		"opponent": match.Player1.Name,
		"phase":    "initiative",
		"message":  "Battle begins! Roll D6 for initiative - highest goes first!",
	})

	// If either player is AI, handle their initiative roll automatically
	if match.Player1.IsAI {
		go gs.handleAIInitiative(match.Player1)
	}
	if match.Player2.IsAI {
		go gs.handleAIInitiative(match.Player2)
	}
}

func (gs *GameServer) simulateCombat(match *Match) {
	// Initialize army health tracking
	match.Player1.RemainingWounds = gs.calculateTotalWounds(match.Player1.Army, match.Player1.Faction)
	match.Player2.RemainingWounds = gs.calculateTotalWounds(match.Player2.Army, match.Player2.Faction)

	round := 1

	// Turn-based combat loop
	for {
		match.Log = append(match.Log, fmt.Sprintf("--- Round %d ---", round))

		// Player 1's turn to attack
		if match.CurrentPlayer.ID == match.Player1.ID {
			gs.startCombatPhase(match, match.Player1, match.Player2)
			return // Wait for player input
		} else {
			gs.startCombatPhase(match, match.Player2, match.Player1)
			return // Wait for player input
		}
	}
}

func (gs *GameServer) calculateDamage(army []UnitSelection) int {
	totalDamage := 0
	for _, unit := range army {
		for _, weapon := range unit.Weapons {
			attacks := parseStatValue(weapon.Attacks)
			damage := parseStatValue(weapon.Damage)
			totalDamage += attacks * damage * unit.Quantity
		}
	}
	return totalDamage
}

func (gs *GameServer) rollDice(player *Player, dice int) {
	result := rand.Intn(dice) + 1

	// Check if we're in initiative phase
	if match, exists := gs.matches[player.MatchID]; exists && match.State == "initiative" && dice == 6 {
		gs.handleInitiativeRoll(player, result, match)
		return
	}

	// Regular dice roll
	gs.sendToPlayer(player, map[string]interface{}{
		"type":   "dice_result",
		"dice":   dice,
		"result": result,
	})

	// Notify opponent
	if match, exists := gs.matches[player.MatchID]; exists {
		opponent := match.Player1
		if player.ID == match.Player1.ID {
			opponent = match.Player2
		}
		gs.sendToPlayer(opponent, map[string]interface{}{
			"type":        "opponent_dice_roll",
			"player_name": player.Name,
			"dice":        dice,
			"result":      result,
		})
	}
}

func (gs *GameServer) handleInitiativeRoll(player *Player, result int, match *Match) {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	// Record the initiative roll
	if player.ID == match.Player1.ID {
		match.Player1Initiative = result
		match.Player1InitiativeSet = true
	} else {
		match.Player2Initiative = result
		match.Player2InitiativeSet = true
	}

	// Send roll result to both players
	gs.sendToPlayer(match.Player1, map[string]interface{}{
		"type":        "initiative_roll",
		"player_name": player.Name,
		"result":      result,
	})
	gs.sendToPlayer(match.Player2, map[string]interface{}{
		"type":        "initiative_roll",
		"player_name": player.Name,
		"result":      result,
	})

	// Check if both players have rolled
	if match.Player1InitiativeSet && match.Player2InitiativeSet {
		gs.resolveInitiative(match)
	}
}

func (gs *GameServer) resolveInitiative(match *Match) {
	var winner *Player
	var message string

	if match.Player1Initiative > match.Player2Initiative {
		winner = match.Player1
		message = fmt.Sprintf("%s wins initiative (%d vs %d) and goes first!",
			match.Player1.Name, match.Player1Initiative, match.Player2Initiative)
	} else if match.Player2Initiative > match.Player1Initiative {
		winner = match.Player2
		message = fmt.Sprintf("%s wins initiative (%d vs %d) and goes first!",
			match.Player2.Name, match.Player2Initiative, match.Player1Initiative)
	} else {
		// Tie - re-roll
		match.Player1InitiativeSet = false
		match.Player2InitiativeSet = false
		match.Player1.Status = "rolling_initiative"
		match.Player2.Status = "rolling_initiative"
		message = fmt.Sprintf("Tied initiative (%d vs %d)! Both players roll again.",
			match.Player1Initiative, match.Player2Initiative)

		gs.sendToPlayer(match.Player1, map[string]interface{}{
			"type":    "initiative_tie",
			"phase":   "initiative",
			"message": message,
		})
		gs.sendToPlayer(match.Player2, map[string]interface{}{
			"type":    "initiative_tie",
			"phase":   "initiative",
			"message": message,
		})

		// If either player is AI, handle their initiative roll automatically after a short delay
		if match.Player1.IsAI {
			go func() {
				time.Sleep(1 * time.Second)
				gs.handleAIInitiative(match.Player1)
			}()
		}
		if match.Player2.IsAI {
			go func() {
				time.Sleep(1 * time.Second)
				gs.handleAIInitiative(match.Player2)
			}()
		}
		return
	}

	// Set current player and start fighting phase
	match.CurrentPlayer = winner
	match.State = "fighting"
	match.Player1.Status = "fighting"
	match.Player2.Status = "fighting"

	// Determine the defender (loser)
	var defender *Player
	if winner.ID == match.Player1.ID {
		defender = match.Player2
	} else {
		defender = match.Player1
	}

	gs.sendToPlayer(match.Player1, map[string]interface{}{
		"type":           "initiative_resolved",
		"message":        message,
		"current_player": winner.Name,
		"your_turn":      winner.ID == match.Player1.ID,
	})
	gs.sendToPlayer(match.Player2, map[string]interface{}{
		"type":           "initiative_resolved",
		"message":        message,
		"current_player": winner.Name,
		"your_turn":      winner.ID == match.Player2.ID,
	})

	log.Printf("Initiative resolved: %s goes first", winner.Name)

	// Start the combat phase after a short delay
	go func() {
		time.Sleep(2 * time.Second) // Give players time to read the initiative result
		gs.startCombatPhase(match, winner, defender)
	}()
}

// calculateTotalWounds calculates the total wounds for an army
func (gs *GameServer) calculateTotalWounds(army []UnitSelection, faction string) int {
	totalWounds := 0
	for _, unit := range army {
		// Get unit stats from the cache/API
		unitData := findUnitOptimized(faction, unit.UnitName)
		if unitData != nil {
			wounds := parseStatValue(unitData.Wounds)
			totalWounds += wounds * unit.Quantity
		}
	}
	return totalWounds
}

// startCombatPhase initiates a combat phase for the attacking player
func (gs *GameServer) startCombatPhase(match *Match, attacker *Player, defender *Player) {
	match.State = "combat_phase"
	match.CurrentPlayer = attacker
	
	log.Printf("COMBAT PHASE START: %s attacking %s (Attacker: %d wounds, Defender: %d wounds)", 
		attacker.Name, defender.Name, attacker.RemainingWounds, defender.RemainingWounds)
	
	match.Log = append(match.Log, fmt.Sprintf("⚔️ %s's turn to attack! (%d wounds vs %d wounds)", 
		attacker.Name, attacker.RemainingWounds, defender.RemainingWounds))

	// Check if attacker has weapons
	hasWeapons := false
	for _, unit := range attacker.Army {
		if len(unit.Weapons) > 0 {
			hasWeapons = true
			break
		}
	}

	if !hasWeapons {
		gs.sendToPlayer(attacker, map[string]interface{}{
			"type":    "no_weapons_selected",
			"message": "You need to select weapons for your units before attacking! Please go to the army building phase.",
		})

		gs.sendToPlayer(defender, map[string]interface{}{
			"type":    "opponent_no_weapons",
			"message": fmt.Sprintf("%s has no weapons selected. Switching turns.", attacker.Name),
		})

		// Switch turns immediately
		go func() {
			time.Sleep(2 * time.Second)
			gs.nextTurn(match)
		}()
		return
	}

	// Start the automatic combat sequence
	gs.sendCombatStartMessage(match, attacker, defender)
}

// sendCombatStartMessage sends the initial combat start message without auto-executing
func (gs *GameServer) sendCombatStartMessage(match *Match, attacker *Player, defender *Player) {
	gs.sendToPlayer(attacker, map[string]interface{}{
		"type":    "combat_start",
		"phase":   "attacking",
		"message": "YOUR TURN - Attack!",
		"combat_info": map[string]interface{}{
			"sequence": []string{
				"1. Choose attacks → 2. Roll to hit → 3. Roll to wound → 4. Opponent rolls saves",
			},
		},
	})

	gs.sendToPlayer(defender, map[string]interface{}{
		"type":    "combat_start",
		"phase":   "defending",
		"message": fmt.Sprintf("%s is attacking! Prepare to roll saves.", attacker.Name),
		"combat_info": map[string]interface{}{
			"sequence": []string{
				"1. Choose attacks → 2. Roll to hit → 3. Roll to wound → 4. Opponent rolls saves",
			},
		},
	})

	// If attacker is AI, automatically execute the attack
	if attacker.IsAI {
		go gs.handleAIAttack(attacker)
	}
}

// startAttack handles when a player clicks the "Begin Attack!" button
func (gs *GameServer) startAttack(player *Player) {
	// Early exit if player is nil
	if player == nil {
		log.Printf("DEBUG: startAttack called with nil player, exiting")
		return
	}

	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	log.Printf("DEBUG: startAttack called for player %s (%s), IsAI: %v", player.Name, player.ID, player.IsAI)

	// Find the player's match - first try by ID, then by name
	var match *Match
	for _, m := range gs.matches {
		// Add nil checks to prevent crash
		if m.Player1 == nil || m.Player2 == nil || m.CurrentPlayer == nil {
			log.Printf("DEBUG: Skipping match with nil players")
			continue
		}

		log.Printf("DEBUG: Checking match - Player1: %s (%s), Player2: %s (%s), CurrentPlayer: %s (%s), State: %s",
			m.Player1.Name, m.Player1.ID, m.Player2.Name, m.Player2.ID, m.CurrentPlayer.Name, m.CurrentPlayer.ID, m.State)

		// Check if player is in this match and it's in a valid state
		if (m.Player1.ID == player.ID || m.Player2.ID == player.ID) && (m.State == "battle" || m.State == "manual_dice_rolling" || m.State == "combat_phase") {
			// Check if it's this player's turn
			if m.CurrentPlayer.ID == player.ID {
				match = m
				log.Printf("DEBUG: Found match by ID for player %s", player.Name)
				break
			} else {
				log.Printf("DEBUG: Player %s is in match but it's not their turn (current: %s)", player.Name, m.CurrentPlayer.Name)
			}
		}
	}

	// If no match found by ID, try by name (fallback for reconnected players)
	if match == nil {
		log.Printf("DEBUG: No match found by ID, trying by name fallback...")
		for _, m := range gs.matches {
			// Add nil checks for name fallback too
			if m.Player1 == nil || m.Player2 == nil || m.CurrentPlayer == nil {
				log.Printf("DEBUG: Skipping match with nil players in name fallback")
				continue
			}

			if (m.Player1.Name == player.Name || m.Player2.Name == player.Name) && (m.State == "battle" || m.State == "manual_dice_rolling" || m.State == "combat_phase") {
				// Check if it's this player's turn by name
				if m.CurrentPlayer.Name == player.Name {
					match = m

					// Update the match to use the current player object
					if m.Player1.Name == player.Name {
						m.Player1 = player
					} else {
						m.Player2 = player
					}
					m.CurrentPlayer = player

					// Update player's match ID
					player.MatchID = m.ID

					log.Printf("Found match by name for player %s (%s), updated match references", player.Name, player.ID)
					break
				} else {
					log.Printf("DEBUG: Player %s found in match by name but it's not their turn (current: %s)", player.Name, m.CurrentPlayer.Name)
				}
			}
		}
	}

	if match == nil {
		log.Printf("No valid match found for player %s (%s) to start attack", player.Name, player.ID)
		return
	}

	// If there's already an ongoing attack sequence, don't start a new one
	if match.State == "manual_dice_rolling" && len(match.AttackSequence) > 0 {
		log.Printf("Player %s attempted to start attack but attack sequence already in progress", player.ID)
		gs.sendToPlayer(player, map[string]interface{}{
			"type":    "error",
			"message": "Attack already in progress! Complete the current attack sequence first.",
		})
		return
	}

	// Find the defender
	var defender *Player
	if match.Player1.ID == player.ID || match.Player1.Name == player.Name {
		defender = match.Player2
	} else {
		defender = match.Player1
	}

	// Execute the new manual attack phase
	gs.executeAttackPhase(match, player, defender)
}

// executeCombatSequence runs the step-by-step Warhammer 40K combat sequence
func (gs *GameServer) executeCombatSequence(match *Match, attacker *Player, defender *Player) {
	// Send initial combat start message
	gs.sendToPlayer(attacker, map[string]interface{}{
		"type":    "combat_start",
		"phase":   "attacking",
		"message": "YOUR TURN - Attack!",
		"combat_info": map[string]interface{}{
			"sequence": []string{
				"1. Choose attacks → 2. Roll to hit → 3. Roll to wound → 4. Opponent rolls saves",
			},
		},
	})

	gs.sendToPlayer(defender, map[string]interface{}{
		"type":    "combat_start",
		"phase":   "defending",
		"message": fmt.Sprintf("%s is attacking! Prepare to roll saves.", attacker.Name),
		"combat_info": map[string]interface{}{
			"sequence": []string{
				"1. Choose attacks → 2. Roll to hit → 3. Roll to wound → 4. Opponent rolls saves",
			},
		},
	})
}

// executeAttackPhase prepares the manual dice rolling attack sequence
func (gs *GameServer) executeAttackPhase(match *Match, attacker *Player, defender *Player) {
	log.Printf("Executing attack phase for player %s", attacker.Name)

	// Debug: Log player army composition
	log.Printf("DEBUG: Player %s army composition:", attacker.Name)
	for i, unit := range attacker.Army {
		log.Printf("  Unit %d: %s (qty: %d), weapons: %d", i, unit.UnitName, unit.Quantity, len(unit.Weapons))
		for j, weapon := range unit.Weapons {
			log.Printf("    Weapon %d: %s (%s)", j, weapon.Name, weapon.Type)
		}
	}

	// Count total weapons to attack with and build attack sequence
	attackSequence := []map[string]interface{}{}
	totalWeapons := 0

	// Build complete attack summary
	for _, unit := range attacker.Army {
		if unit.Quantity <= 0 {
			continue
		}

		unitData := findUnitOptimized(attacker.Faction, unit.UnitName)
		if unitData == nil {
			continue
		}

		// Deduplicate weapons by name for this unit
		weaponMap := make(map[string]Weapon)
		for _, weapon := range unit.Weapons {
			weaponMap[weapon.Name] = weapon
		}

		// Process deduplicated weapons
		for _, weapon := range weaponMap {
			attacks := parseStatValue(weapon.Attacks) * unit.Quantity
			if attacks <= 0 {
				continue
			}

			totalWeapons++

			// Get defender toughness (simplified)
			defenderToughness := 4
			strengthValue := parseStatValue(weapon.Strength)
			if strengthValue == 0 {
				strengthValue = parseStatValue(unitData.Strength)
			}

			weaponAttack := map[string]interface{}{
				"weapon_id":    fmt.Sprintf("%s_%s_%d", unit.UnitName, weapon.Name, totalWeapons),
				"weapon_name":  weapon.Name,
				"unit_name":    unit.UnitName,
				"attacks":      attacks,
				"skill":        parseStatValue(weapon.Skill),
				"strength":     strengthValue,
				"ap":           parseStatValue(weapon.AP),
				"damage":       parseStatValue(weapon.Damage),
				"toughness":    defenderToughness,
				"armor_save":   4, // Default, should get from defender
				"wound_target": gs.calculateWoundThreshold(strengthValue, defenderToughness),
				"save_target":  4 + parseStatValue(weapon.AP), // armor + AP
				"completed":    false,
				"hit_rolls":    []int{},
				"wound_rolls":  []int{},
				"save_rolls":   []int{},
				"hits":         0,
				"wounds":       0,
				"saves":        0,
				"damage_dealt": 0,
			}
			attackSequence = append(attackSequence, weaponAttack)
		}
	}

	if totalWeapons == 0 {
		gs.sendToPlayer(attacker, map[string]interface{}{
			"type":    "no_weapons_selected",
			"message": "No weapons equipped! Turn skipped.",
		})
		gs.sendToPlayer(defender, map[string]interface{}{
			"type":    "no_weapons_selected",
			"message": fmt.Sprintf("%s has no weapons equipped - turn skipped.", attacker.Name),
		})

		// Skip to next turn since no weapons are available
		gs.nextTurn(match)
		return
	}

	// Store attack sequence in match for manual rolling
	match.AttackSequence = attackSequence
	match.CurrentWeaponIndex = 0
	match.CurrentPhase = "hit"
	match.State = "manual_dice_rolling"
	match.AttackHistory = []map[string]interface{}{}

	// Send complete attack summary to both players
	gs.sendToPlayer(attacker, map[string]interface{}{
		"type":                "attack_summary",
		"phase":               "manual_rolling",
		"weapons":             attackSequence,
		"total_weapons":       totalWeapons,
		"current_weapon":      0,
		"current_phase":       "hit",
		"message":             fmt.Sprintf("🎯 Attack Phase: %d weapons ready. Click 'Roll Hits' to start!", totalWeapons),
		"show_hit_button":     true,
		"show_action_buttons": true,
	})

	gs.sendToPlayer(defender, map[string]interface{}{
		"type":                "attack_summary",
		"phase":               "manual_rolling",
		"weapons":             attackSequence,
		"total_weapons":       totalWeapons,
		"current_weapon":      0,
		"current_phase":       "hit",
		"message":             fmt.Sprintf("⚔️ %s is attacking with %d weapons! Prepare for combat!", attacker.Name, totalWeapons),
		"show_hit_button":     false,
		"show_action_buttons": false,
	})
}

// sendCombatResults sends final combat results to both players
func (gs *GameServer) sendCombatResults(match *Match, attacker *Player, defender *Player, totalDamage int) {
	// Apply damage to defender
	defender.RemainingWounds = max(0, defender.RemainingWounds-totalDamage)

	// Send to attacker
	gs.sendToPlayer(attacker, map[string]interface{}{
		"type":         "combat_results",
		"phase":        "results",
		"total_damage": totalDamage,
		"enemy_wounds": defender.RemainingWounds,
		"message":      fmt.Sprintf("Attack complete! You dealt %d total damage. %s has %d wounds remaining.", totalDamage, defender.Name, defender.RemainingWounds),
	})

	// Send to defender
	gs.sendToPlayer(defender, map[string]interface{}{
		"type":         "combat_results",
		"phase":        "results",
		"total_damage": totalDamage,
		"your_wounds":  defender.RemainingWounds,
		"message":      fmt.Sprintf("%s's attack complete! You took %d damage and have %d wounds remaining.", attacker.Name, totalDamage, defender.RemainingWounds),
	})

	// Check if defender is defeated
	if defender.RemainingWounds <= 0 {
		gs.endBattle(match, attacker, defender)
		return
	}
}

// calculateWoundThreshold determines the dice roll needed to wound based on Strength vs Toughness
func (gs *GameServer) calculateWoundThreshold(strength, toughness int) int {
	if strength >= toughness*2 {
		return 2 // 2+ to wound
	} else if strength > toughness {
		return 3 // 3+ to wound
	} else if strength == toughness {
		return 4 // 4+ to wound
	} else if strength*2 <= toughness {
		return 6 // 6+ to wound
	} else {
		return 5 // 5+ to wound
	}
}

// Helper function to get max of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// endBattle ends the match when one player is defeated
func (gs *GameServer) endBattle(match *Match, winner *Player, loser *Player) {
	match.State = "finished"
	match.Log = append(match.Log, fmt.Sprintf("%s wins! %s has been defeated!", winner.Name, loser.Name))

	// Send victory message to both players
	gs.sendToPlayer(winner, map[string]interface{}{
		"type":    "battle_ended",
		"result":  "victory",
		"message": fmt.Sprintf("Victory! You defeated %s!", loser.Name),
	})

	gs.sendToPlayer(loser, map[string]interface{}{
		"type":    "battle_ended",
		"result":  "defeat",
		"message": fmt.Sprintf("Defeat! You were defeated by %s!", winner.Name),
	})

	// Clean up match
	delete(gs.matches, match.ID)
}

// nextTurn switches to the next player's turn
func (gs *GameServer) nextTurn(match *Match) {
	// Switch current player
	oldPlayer := match.CurrentPlayer.Name
	if match.CurrentPlayer == match.Player1 {
		match.CurrentPlayer = match.Player2
	} else {
		match.CurrentPlayer = match.Player1
	}

	log.Printf("TURN CHANGE: %s → %s (P1: %d wounds, P2: %d wounds)", 
		oldPlayer, match.CurrentPlayer.Name, match.Player1.RemainingWounds, match.Player2.RemainingWounds)
	
	// Add turn change to battle log
	match.Log = append(match.Log, fmt.Sprintf("🔄 Turn switches to %s", match.CurrentPlayer.Name))

	// Start next combat phase
	defender := match.Player1
	if match.CurrentPlayer == match.Player1 {
		defender = match.Player2
	}

	log.Printf("STARTING TURN: %s (attacker) vs %s (defender)", match.CurrentPlayer.Name, defender.Name)
	
	gs.startCombatPhase(match, match.CurrentPlayer, defender)
}

// startAttackSequence begins the hit/wound/save sequence
func (gs *GameServer) startAttackSequence(match *Match, attacker *Player, defender *Player, attackingUnit UnitSelection, weapon Weapon) {
	// Get unit stats for calculations
	attackerUnitData := findUnitOptimized(attacker.Faction, attackingUnit.UnitName)
	if attackerUnitData == nil {
		log.Printf("❌ Could not find attacker unit data for %s", attackingUnit.UnitName)
		return
	}

	// Calculate number of attacks
	attacks := parseStatValue(weapon.Attacks) * attackingUnit.Quantity

	// Store combat state
	combat := &CombatAttack{
		AttackerUnit:   attackingUnit,
		AttackerWeapon: weapon,
		Attacks:        attacks,
		Phase:          "hit_rolls",
	}

	match.CurrentCombat = combat
	match.State = "rolling_hits"

	// Send hit rolling phase to attacker
	gs.sendToPlayer(attacker, map[string]interface{}{
		"type":    "hit_phase",
		"message": fmt.Sprintf("Roll %d dice to hit with %s", attacks, weapon.Name),
		"attacks": attacks,
		"hit_on":  weapon.Skill,
		"weapon":  weapon,
		"phase":   "hit",
	})

	// Notify defender
	gs.sendToPlayer(defender, map[string]interface{}{
		"type":    "combat_waiting",
		"message": fmt.Sprintf("%s is rolling to hit with %s", attacker.Name, weapon.Name),
		"phase":   "enemy_hitting",
	})
}

// processHitRolls handles the hit roll results
func (gs *GameServer) processHitRolls(match *Match, player *Player, rolls []int) {
	combat := match.CurrentCombat
	if combat == nil {
		return
	}

	combat.HitRolls = rolls
	hitTarget := parseStatValue(combat.AttackerWeapon.Skill)
	hits := 0

	for _, roll := range rolls {
		if roll >= hitTarget {
			hits++
		}
	}

	if hits == 0 {
		// No hits, end attack
		gs.endAttackSequence(match, player, 0)
		return
	}

	// Move to wound phase
	match.State = "rolling_wounds"
	combat.Phase = "wound_rolls"

	gs.sendToPlayer(player, map[string]interface{}{
		"type":    "wound_phase",
		"message": fmt.Sprintf("%d hits! Roll %d dice to wound", hits, hits),
		"hits":    hits,
		"weapon":  combat.AttackerWeapon,
		"phase":   "wound",
	})
}

// processWoundRolls handles the wound roll results
func (gs *GameServer) processWoundRolls(match *Match, player *Player, rolls []int) {
	combat := match.CurrentCombat
	if combat == nil {
		return
	}

	combat.WoundRolls = rolls

	// Get defender unit stats for toughness
	defender := match.Player1
	if player.ID == match.Player1.ID {
		defender = match.Player2
	}

	// For simplicity, use a representative unit from defender's army
	// In a full implementation, player would choose which unit takes wounds
	defenderUnitData := findUnitOptimized(defender.Faction, defender.Army[0].UnitName)
	if defenderUnitData == nil {
		log.Printf("❌ Could not find defender unit data")
		return
	}

	// Calculate wound target number using S vs T table
	strength := parseStatValue(combat.AttackerWeapon.Strength)
	toughness := parseStatValue(defenderUnitData.Toughness)
	woundTarget := gs.calculateWoundTarget(strength, toughness)

	wounds := 0
	for _, roll := range rolls {
		if roll >= woundTarget {
			wounds++
		}
	}

	if wounds == 0 {
		// No wounds, end attack
		gs.endAttackSequence(match, player, 0)
		return
	}

	// Move to save phase - defender rolls saves
	match.State = "rolling_saves"
	combat.Phase = "save_rolls"
	combat.DefenderUnit = defenderUnitData

	// Calculate save target (armor save + AP modifier)
	armorSave := parseStatValue(defenderUnitData.Save)
	armorPenetration := parseStatValue(combat.AttackerWeapon.AP)
	saveTarget := armorSave - armorPenetration

	gs.sendToPlayer(defender, map[string]interface{}{
		"type":        "save_phase",
		"message":     fmt.Sprintf("%d wounds allocated! Roll %d armor saves", wounds, wounds),
		"wounds":      wounds,
		"save_target": saveTarget,
		"phase":       "save",
	})

	// Notify attacker
	gs.sendToPlayer(player, map[string]interface{}{
		"type":    "combat_waiting",
		"message": fmt.Sprintf("Defender rolling %d saves", wounds),
		"phase":   "enemy_saving",
	})
}

// processSaveRolls handles the save roll results
func (gs *GameServer) processSaveRolls(match *Match, player *Player, rolls []int) {
	combat := match.CurrentCombat
	if combat == nil {
		return
	}

	combat.SaveRolls = rolls

	// Calculate save target
	armorSave := parseStatValue(combat.DefenderUnit.Save)
	armorPenetration := parseStatValue(combat.AttackerWeapon.AP)
	saveTarget := armorSave - armorPenetration

	failedSaves := 0
	for _, roll := range rolls {
		if roll < saveTarget {
			failedSaves++
		}
	}

	// Calculate final wounds inflicted
	damage := parseStatValue(combat.AttackerWeapon.Damage)
	woundsInflicted := failedSaves * damage

	// Update combat state
	combat.Phase = "complete"
	combat.WoundsInflicted = woundsInflicted

	gs.endAttackSequence(match, player, woundsInflicted)
}

// endAttackSequence concludes the attack and applies damage
func (gs *GameServer) endAttackSequence(match *Match, player *Player, woundsInflicted int) {
	// Apply wounds to defender
	defender := match.Player1
	if player.ID == match.Player1.ID {
		defender = match.Player2
	}

	log.Printf("DEBUG: endAttackSequence - Before damage: %s has %d wounds, applying %d wounds", defender.Name, defender.RemainingWounds, woundsInflicted)

	defender.RemainingWounds -= woundsInflicted

	log.Printf("DEBUG: endAttackSequence - After damage: %s has %d wounds remaining", defender.Name, defender.RemainingWounds)

	// Log the attack result
	attacker := player
	if player.ID == defender.ID {
		// If player is defender, find attacker
		if match.Player1.ID == defender.ID {
			attacker = match.Player2
		} else {
			attacker = match.Player1
		}
	}

	combat := match.CurrentCombat
	logMessage := fmt.Sprintf("%s attacks with %s: %d wounds inflicted",
		attacker.Name, combat.AttackerWeapon.Name, woundsInflicted)
	match.Log = append(match.Log, logMessage)

	// Check for army destruction
	if defender.RemainingWounds <= 0 {
		// Battle over - attacker wins
		match.State = "finished"
		match.Winner = attacker.Name

		gs.sendToPlayer(match.Player1, map[string]interface{}{
			"type":   "battle_finished",
			"winner": match.Winner,
			"log":    match.Log,
		})
		gs.sendToPlayer(match.Player2, map[string]interface{}{
			"type":   "battle_finished",
			"winner": match.Winner,
			"log":    match.Log,
		})
		return
	}

	// Switch turns
	if match.CurrentPlayer.ID == match.Player1.ID {
		match.CurrentPlayer = match.Player2
	} else {
		match.CurrentPlayer = match.Player1
	}

	// Start next turn
	newAttacker := match.CurrentPlayer
	newDefender := match.Player1
	if newAttacker.ID == match.Player1.ID {
		newDefender = match.Player2
	}

	gs.startCombatPhase(match, newAttacker, newDefender)
}

// calculateWoundTarget returns the target number needed to wound based on S vs T
func (gs *GameServer) calculateWoundTarget(strength int, toughness int) int {
	if strength >= toughness*2 {
		return 2
	} else if strength > toughness {
		return 3
	} else if strength == toughness {
		return 4
	} else if strength*2 <= toughness {
		return 6
	} else {
		return 5
	}
}

// handleManualDiceRoll processes manual dice rolling for the attack sequence
func (gs *GameServer) handleManualDiceRoll(player *Player, rollType string) {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	log.Printf("DEBUG: handleManualDiceRoll called for player %s with rollType %s", player.Name, rollType)

	// Find the player's match
	var match *Match
	for _, m := range gs.matches {
		log.Printf("DEBUG: Checking match - Player1: %s, Player2: %s, State: %s, CurrentPlayer: %s",
			m.Player1.Name, m.Player2.Name, m.State, m.CurrentPlayer.Name)
		if (m.Player1 == player || m.Player2 == player) && m.State == "manual_dice_rolling" {
			match = m
			break
		}
	}

	if match == nil {
		log.Printf("No match found for manual dice rolling for player %s", player.ID)
		return
	}

	// Determine who should be rolling based on the phase
	var allowedPlayer *Player
	if match.CurrentPhase == "save" {
		// For saves, the defending player (opponent) should roll
		if match.CurrentPlayer == match.Player1 {
			allowedPlayer = match.Player2
		} else {
			allowedPlayer = match.Player1
		}
	} else {
		// For hits and wounds, the attacking player should roll
		allowedPlayer = match.CurrentPlayer
	}

	if allowedPlayer != player {
		log.Printf("Player %s tried to roll dice but it's not their turn (phase: %s)", player.ID, match.CurrentPhase)
		return
	}

	// Get the current weapon being processed
	if match.CurrentWeaponIndex >= len(match.AttackSequence) {
		log.Printf("All weapons processed, finishing attack")
		gs.finishManualAttackPhase(match)
		return
	}

	currentWeapon := match.AttackSequence[match.CurrentWeaponIndex]

	// Process the dice roll based on the current phase
	log.Printf("DEBUG: Processing rollType %s, CurrentPhase is '%s'", rollType, match.CurrentPhase)
	switch rollType {
	case "roll_hit":
		if match.CurrentPhase != "hit" {
			log.Printf("DEBUG: CurrentPhase mismatch - expected 'hit', got '%s'", match.CurrentPhase)
			return
		}
		gs.processHitPhase(match, currentWeapon)
	case "roll_wound":
		if match.CurrentPhase != "wound" {
			return
		}
		gs.processWoundPhase(match, currentWeapon)
	case "roll_save":
		if match.CurrentPhase != "save" {
			return
		}
		gs.processSavePhase(match, currentWeapon)
	}
}

// processHitPhase handles the hit dice rolling
func (gs *GameServer) processHitPhase(match *Match, weapon map[string]interface{}) {
	attacks := weapon["attacks"].(int)
	hitOn := weapon["skill"].(int)

	// Roll hit dice
	hitRolls := make([]int, attacks)
	hits := 0
	for i := 0; i < attacks; i++ {
		roll := rand.Intn(6) + 1
		hitRolls[i] = roll
		if roll >= hitOn {
			hits++
		}
	}

	// Update weapon data
	weapon["hit_rolls"] = hitRolls
	weapon["hits"] = hits

	// Add to attack history
	historyEntry := map[string]interface{}{
		"weapon_name": weapon["weapon_name"],
		"unit_name":   weapon["unit_name"],
		"phase":       "hit",
		"rolls":       hitRolls,
		"target":      hitOn,
		"successes":   hits,
		"message":     fmt.Sprintf("Hit rolls (need %d+): %v → %d hits", hitOn, hitRolls, hits),
	}
	match.AttackHistory = append(match.AttackHistory, historyEntry)

	log.Printf("COMBAT HIT: %s's %s rolled %v (need %d+) → %d hits", 
		match.CurrentPlayer.Name, weapon["weapon_name"], hitRolls, hitOn, hits)
	
	// Add to battle log
	match.Log = append(match.Log, fmt.Sprintf("🎯 %s's %s: %d hits from %d attacks", 
		match.CurrentPlayer.Name, weapon["weapon_name"], hits, len(hitRolls)))

	// Send results to both players
	attacker := match.CurrentPlayer
	var defender *Player
	if match.Player1 == attacker {
		defender = match.Player2
	} else {
		defender = match.Player1
	}

	gs.sendToPlayer(attacker, map[string]interface{}{
		"type":              "hit_results",
		"weapon":            weapon,
		"attack_history":    match.AttackHistory,
		"current_weapon":    match.CurrentWeaponIndex,
		"total_weapons":     len(match.AttackSequence),
		"message":           historyEntry["message"],
		"show_wound_button": hits > 0,
		"show_next_weapon":  hits == 0,
	})

	gs.sendToPlayer(defender, map[string]interface{}{
		"type":           "hit_results",
		"weapon":         weapon,
		"attack_history": match.AttackHistory,
		"current_weapon": match.CurrentWeaponIndex,
		"total_weapons":  len(match.AttackSequence),
		"message":        fmt.Sprintf("Opponent %s", historyEntry["message"]),
	})

	// Move to next phase or weapon
	if hits > 0 {
		match.CurrentPhase = "wound"
	} else {
		gs.nextWeapon(match)
	}
}

// processWoundPhase handles the wound dice rolling
func (gs *GameServer) processWoundPhase(match *Match, weapon map[string]interface{}) {
	hits := weapon["hits"].(int)
	woundOn := weapon["wound_target"].(int)

	// Roll wound dice
	woundRolls := make([]int, hits)
	wounds := 0
	for i := 0; i < hits; i++ {
		roll := rand.Intn(6) + 1
		woundRolls[i] = roll
		if roll >= woundOn {
			wounds++
		}
	}

	// Update weapon data
	weapon["wound_rolls"] = woundRolls
	weapon["wounds"] = wounds

	// Add to attack history
	historyEntry := map[string]interface{}{
		"weapon_name": weapon["weapon_name"],
		"unit_name":   weapon["unit_name"],
		"phase":       "wound",
		"rolls":       woundRolls,
		"target":      woundOn,
		"successes":   wounds,
		"message":     fmt.Sprintf("Wound rolls (S%d vs T%d, need %d+): %v → %d wounds", weapon["strength"], weapon["toughness"], woundOn, woundRolls, wounds),
	}
	match.AttackHistory = append(match.AttackHistory, historyEntry)

	log.Printf("COMBAT WOUND: %s's %s rolled %v (need %d+) → %d wounds", 
		match.CurrentPlayer.Name, weapon["weapon_name"], woundRolls, woundOn, wounds)
	
	// Add to battle log
	if wounds > 0 {
		match.Log = append(match.Log, fmt.Sprintf("💥 %s's %s: %d wounds caused", 
			match.CurrentPlayer.Name, weapon["weapon_name"], wounds))
	} else {
		match.Log = append(match.Log, fmt.Sprintf("🛡️ %s's %s: No wounds caused", 
			match.CurrentPlayer.Name, weapon["weapon_name"]))
	}

	// Send results to both players
	attacker := match.CurrentPlayer
	var defender *Player
	if match.Player1 == attacker {
		defender = match.Player2
	} else {
		defender = match.Player1
	}

	gs.sendToPlayer(attacker, map[string]interface{}{
		"type":             "wound_results",
		"weapon":           weapon,
		"attack_history":   match.AttackHistory,
		"current_weapon":   match.CurrentWeaponIndex,
		"total_weapons":    len(match.AttackSequence),
		"message":          historyEntry["message"],
		"show_save_button": false, // Attacker never rolls saves
		"show_next_weapon": wounds == 0,
	})

	gs.sendToPlayer(defender, map[string]interface{}{
		"type":             "wound_results",
		"weapon":           weapon,
		"attack_history":   match.AttackHistory,
		"current_weapon":   match.CurrentWeaponIndex,
		"total_weapons":    len(match.AttackSequence),
		"message":          fmt.Sprintf("Opponent %s", historyEntry["message"]),
		"show_save_button": wounds > 0 && !defender.IsAI, // Only show save button to human defenders
	})

	log.Printf("DEBUG: Wound phase complete - wounds: %d, defender.IsAI: %v, show_save_button: %v", wounds, defender.IsAI, wounds > 0 && !defender.IsAI)

	// Move to next phase or weapon
	if wounds > 0 {
		match.CurrentPhase = "save"
		log.Printf("DEBUG: Transitioning to save phase for defender %s (IsAI: %v)", defender.Name, defender.IsAI)

		// If the defending player is AI, automatically handle their saves
		if defender.IsAI {
			go func() {
				time.Sleep(1 * time.Second) // Brief delay for realism
				gs.processSavePhase(match, weapon)
			}()
		} else {
			log.Printf("DEBUG: Human defender %s needs to roll saves manually", defender.Name)
		}
	} else {
		gs.nextWeapon(match)
	}
}

// processSavePhase handles the save dice rolling
func (gs *GameServer) processSavePhase(match *Match, weapon map[string]interface{}) {
	wounds := weapon["wounds"].(int)
	saveOn := weapon["save_target"].(int)

	// Roll save dice
	saveRolls := make([]int, wounds)
	saves := 0
	for i := 0; i < wounds; i++ {
		roll := rand.Intn(6) + 1
		saveRolls[i] = roll
		if roll >= saveOn {
			saves++
		}
	}

	unsavedWounds := wounds - saves
	damage := weapon["damage"].(int)
	totalDamage := unsavedWounds * damage

	// Update weapon data
	weapon["save_rolls"] = saveRolls
	weapon["saves"] = saves
	weapon["damage_dealt"] = totalDamage
	weapon["completed"] = true

	// Add to attack history
	historyEntry := map[string]interface{}{
		"weapon_name":    weapon["weapon_name"],
		"unit_name":      weapon["unit_name"],
		"phase":          "save",
		"rolls":          saveRolls,
		"target":         saveOn,
		"successes":      saves,
		"unsaved_wounds": unsavedWounds,
		"damage":         totalDamage,
		"message":        fmt.Sprintf("Armor saves (need %d+, AP-%d): %v → %d saves, %d unsaved wounds, %d damage", saveOn, weapon["ap"], saveRolls, saves, unsavedWounds, totalDamage),
	}
	match.AttackHistory = append(match.AttackHistory, historyEntry)

	// Add detailed battle log entry
	defender := match.Player2
	if match.Player1 != match.CurrentPlayer {
		defender = match.Player1
	}
	
	log.Printf("COMBAT: %s's %s dealing %d damage to %s (was %d wounds, will be %d wounds)", 
		match.CurrentPlayer.Name, weapon["weapon_name"], totalDamage, defender.Name, 
		defender.RemainingWounds, max(0, defender.RemainingWounds-totalDamage))
	
	match.Log = append(match.Log, fmt.Sprintf("🎯 %s's %s: %d unsaved wounds → %d damage dealt", 
		match.CurrentPlayer.Name, weapon["weapon_name"], unsavedWounds, totalDamage))

	// Send results to both players
	attacker := match.CurrentPlayer
	if match.Player1 == attacker {
		defender = match.Player2
	} else {
		defender = match.Player1
	}

	gs.sendToPlayer(attacker, map[string]interface{}{
		"type":             "save_results",
		"weapon":           weapon,
		"attack_history":   match.AttackHistory,
		"current_weapon":   match.CurrentWeaponIndex,
		"total_weapons":    len(match.AttackSequence),
		"message":          fmt.Sprintf("Opponent %s", historyEntry["message"]),
		"show_next_weapon": true,
	})

	gs.sendToPlayer(defender, map[string]interface{}{
		"type":           "save_results",
		"weapon":         weapon,
		"attack_history": match.AttackHistory,
		"current_weapon": match.CurrentWeaponIndex,
		"total_weapons":  len(match.AttackSequence),
		"message":        historyEntry["message"],
	})

	// Always move to next weapon after saves
	gs.nextWeapon(match)
	
	// Check if current player is AI and continue their attack sequence
	if match.CurrentPlayer.IsAI && match.State == "manual_dice_rolling" {
		log.Printf("DEBUG: Resuming AI attack sequence after save phase")
		go func() {
			time.Sleep(500 * time.Millisecond) // Brief delay
			gs.handleAIDiceRolling(match.CurrentPlayer)
		}()
	}
}

// nextWeapon moves to the next weapon in the attack sequence
func (gs *GameServer) nextWeapon(match *Match) {
	match.CurrentWeaponIndex++
	match.CurrentPhase = "hit"

	// Check if we have more weapons
	if match.CurrentWeaponIndex >= len(match.AttackSequence) {
		gs.finishManualAttackPhase(match)
		return
	}

	// Send next weapon ready message
	nextWeapon := match.AttackSequence[match.CurrentWeaponIndex]
	attacker := match.CurrentPlayer
	var defender *Player
	if match.Player1 == attacker {
		defender = match.Player2
	} else {
		defender = match.Player1
	}

	gs.sendToPlayer(attacker, map[string]interface{}{
		"type":            "next_weapon_ready",
		"weapon":          nextWeapon,
		"attack_history":  match.AttackHistory,
		"current_weapon":  match.CurrentWeaponIndex,
		"total_weapons":   len(match.AttackSequence),
		"message":         fmt.Sprintf("Weapon %d/%d: %s (%s) - %d attacks", match.CurrentWeaponIndex+1, len(match.AttackSequence), nextWeapon["weapon_name"], nextWeapon["unit_name"], nextWeapon["attacks"]),
		"show_hit_button": true,
	})

	gs.sendToPlayer(defender, map[string]interface{}{
		"type":           "next_weapon_ready",
		"weapon":         nextWeapon,
		"attack_history": match.AttackHistory,
		"current_weapon": match.CurrentWeaponIndex,
		"total_weapons":  len(match.AttackSequence),
		"message":        fmt.Sprintf("Next weapon: %s (%s) - %d attacks", nextWeapon["weapon_name"], nextWeapon["unit_name"], nextWeapon["attacks"]),
	})
}

// finishManualAttackPhase completes the attack phase and moves to next turn
func (gs *GameServer) finishManualAttackPhase(match *Match) {
	// Calculate total damage
	totalDamage := 0
	for _, weapon := range match.AttackSequence {
		if damage, ok := weapon["damage_dealt"].(int); ok {
			totalDamage += damage
		}
	}

	// Get players
	attacker := match.CurrentPlayer
	var defender *Player
	if match.Player1 == attacker {
		defender = match.Player2
	} else {
		defender = match.Player1
	}

	// Apply damage to defender's wounds
	log.Printf("DAMAGE APPLICATION: %s applying %d total damage to %s (current wounds: %d)", 
		attacker.Name, totalDamage, defender.Name, defender.RemainingWounds)
	
	// Apply the damage
	oldWounds := defender.RemainingWounds
	defender.RemainingWounds = max(0, defender.RemainingWounds-totalDamage)
	
	// Log the wound change
	woundsLost := oldWounds - defender.RemainingWounds
	log.Printf("WOUNDS UPDATE: %s lost %d wounds (%d → %d)", defender.Name, woundsLost, oldWounds, defender.RemainingWounds)
	
	// Add comprehensive battle log entry
	match.Log = append(match.Log, fmt.Sprintf("💥 %s's assault complete: %d total damage dealt", attacker.Name, totalDamage))
	if woundsLost > 0 {
		match.Log = append(match.Log, fmt.Sprintf("🩸 %s suffers %d wounds (%d remaining)", defender.Name, woundsLost, defender.RemainingWounds))
	} else {
		match.Log = append(match.Log, fmt.Sprintf("🛡️ %s takes no damage (%d wounds remaining)", defender.Name, defender.RemainingWounds))
	}

	// Send final results with wound information
	gs.sendToPlayer(attacker, map[string]interface{}{
		"type":           "attack_phase_complete",
		"attack_history": match.AttackHistory,
		"total_damage":   totalDamage,
		"enemy_wounds":   defender.RemainingWounds,
		"your_wounds":    attacker.RemainingWounds,
		"message":        fmt.Sprintf("🎯 Attack complete! %d damage dealt. %s has %d wounds left", totalDamage, defender.Name, defender.RemainingWounds),
	})

	gs.sendToPlayer(defender, map[string]interface{}{
		"type":           "attack_phase_complete",
		"attack_history": match.AttackHistory,
		"total_damage":   totalDamage,
		"your_wounds":    defender.RemainingWounds,
		"enemy_wounds":   attacker.RemainingWounds,
		"message":        fmt.Sprintf("⚔️ %s's attack complete! %d damage taken. You have %d wounds left", attacker.Name, totalDamage, defender.RemainingWounds),
	})

	// Check if battle is over due to army destruction
	if defender.RemainingWounds <= 0 {
		log.Printf("BATTLE END: %s destroyed! %s wins", defender.Name, attacker.Name)
		match.State = "finished"
		match.Winner = attacker.Name
		match.Log = append(match.Log, fmt.Sprintf("🏆 %s wins! %s's army has been destroyed!", attacker.Name, defender.Name))

		// Send battle finished messages
		gs.sendToPlayer(attacker, map[string]interface{}{
			"type":    "battle_finished",
			"winner":  attacker.Name,
			"message": fmt.Sprintf("🏆 Victory! You have destroyed %s's army!", defender.Name),
			"log":     match.Log,
		})
		gs.sendToPlayer(defender, map[string]interface{}{
			"type":    "battle_finished",
			"winner":  attacker.Name,
			"message": fmt.Sprintf("💀 Defeat! Your army has been destroyed by %s!", attacker.Name),
			"log":     match.Log,
		})
		return
	}

	// Reset attack state and move to next turn
	match.AttackSequence = []map[string]interface{}{}
	match.CurrentWeaponIndex = 0
	match.CurrentPhase = ""
	match.AttackHistory = []map[string]interface{}{}
	match.State = "battle"

	gs.nextTurn(match)
}
