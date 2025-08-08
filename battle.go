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
		message = fmt.Sprintf("Tied initiative (%d vs %d)! Both players roll again.",
			match.Player1Initiative, match.Player2Initiative)

		gs.sendToPlayer(match.Player1, map[string]interface{}{
			"type":    "initiative_tie",
			"message": message,
		})
		gs.sendToPlayer(match.Player2, map[string]interface{}{
			"type":    "initiative_tie",
			"message": message,
		})
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
	match.Log = append(match.Log, fmt.Sprintf("%s's turn to attack!", attacker.Name))

	// Send combat phase start to both players
	gs.sendToPlayer(attacker, map[string]interface{}{
		"type":         "combat_phase_start",
		"phase":        "select_unit",
		"message":      "Select a unit to attack with",
		"your_turn":    true,
		"army":         attacker.Army,
		"enemy_wounds": defender.RemainingWounds,
	})

	gs.sendToPlayer(defender, map[string]interface{}{
		"type":        "combat_phase_start",
		"phase":       "enemy_turn",
		"message":     fmt.Sprintf("%s is selecting units to attack with", attacker.Name),
		"your_turn":   false,
		"your_wounds": defender.RemainingWounds,
	})
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

	defender.RemainingWounds -= woundsInflicted

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
