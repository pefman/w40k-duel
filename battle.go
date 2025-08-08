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
	// Simple combat simulation
	for round := 1; round <= 5; round++ {
		match.Log = append(match.Log, fmt.Sprintf("--- Round %d ---", round))

		// Player 1 attacks
		damage1 := gs.calculateDamage(match.Player1.Army)
		match.Log = append(match.Log, fmt.Sprintf("%s deals %d damage", match.Player1.Name, damage1))

		// Player 2 attacks
		damage2 := gs.calculateDamage(match.Player2.Army)
		match.Log = append(match.Log, fmt.Sprintf("%s deals %d damage", match.Player2.Name, damage2))

		// Send round results
		gs.sendToPlayer(match.Player1, map[string]interface{}{
			"type":            "combat_round",
			"round":           round,
			"damage_dealt":    damage1,
			"damage_received": damage2,
			"log":             match.Log,
		})
		gs.sendToPlayer(match.Player2, map[string]interface{}{
			"type":            "combat_round",
			"round":           round,
			"damage_dealt":    damage2,
			"damage_received": damage1,
			"log":             match.Log,
		})

		// Determine winner after round 5
		if round == 5 {
			if damage1 > damage2 {
				match.Winner = match.Player1.Name
			} else if damage2 > damage1 {
				match.Winner = match.Player2.Name
			} else {
				match.Winner = "Draw"
			}

			match.State = "finished"
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
			break
		}

		time.Sleep(2 * time.Second)
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
}
