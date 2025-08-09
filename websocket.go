package main

import (
	"log"
	"net/http"
)

// WebSocket handler
func (gs *GameServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := gs.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	playerID := generatePlayerID()
	player := &Player{
		ID:     playerID,
		Name:   generatePlayerName(),
		Conn:   conn,
		Status: "connected",
	}

	gs.mutex.Lock()
	gs.players[playerID] = player
	gs.mutex.Unlock()

	log.Printf("Player %s (%s) connected", player.Name, player.ID)

	// Send initial player info
	gs.sendToPlayer(player, map[string]interface{}{
		"type":      "player_info",
		"player_id": player.ID,
		"name":      player.Name,
	})

	// Broadcast updated online players list
	gs.broadcastOnlinePlayersList()

	// Handle messages
	for {
		var msg map[string]interface{}
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Printf("Player %s disconnected: %v", player.ID, err)
			gs.handleDisconnect(player)
			break
		}
		gs.handleMessage(player, msg)
	}
}

func (gs *GameServer) handleMessage(player *Player, msg map[string]interface{}) {
	msgType, ok := msg["type"].(string)
	if !ok {
		return
	}

	switch msgType {
	case "set_name":
		name, _ := msg["name"].(string)
		gs.setPlayerName(player, name)
	case "join_matchmaking":
		gs.joinMatchmaking(player)
	case "play_vs_ai":
		difficulty, _ := msg["difficulty"].(string)
		if difficulty == "" {
			difficulty = "medium"
		}
		gs.createAIMatch(player, difficulty)
	case "select_faction":
		faction, _ := msg["faction"].(string)
		gs.selectFaction(player, faction)
	case "get_unit_weapons":
		unitName, _ := msg["unit_name"].(string)
		gs.getUnitWeapons(player, unitName)
	case "select_army":
		armyData, _ := msg["army"].([]interface{})
		gs.selectArmy(player, armyData)
	case "roll_dice":
		dice, _ := msg["dice"].(float64)
		gs.rollDice(player, int(dice))
	case "ready_to_fight":
		gs.readyToFight(player)
	case "select_attacking_unit":
		unitName, _ := msg["unit_name"].(string)
		weaponName, _ := msg["weapon_name"].(string)
		gs.selectAttackingUnit(player, unitName, weaponName)
	case "submit_hit_rolls":
		rolls, _ := msg["rolls"].([]interface{})
		gs.submitHitRolls(player, rolls)
	case "submit_wound_rolls":
		rolls, _ := msg["rolls"].([]interface{})
		gs.submitWoundRolls(player, rolls)
	case "submit_save_rolls":
		rolls, _ := msg["rolls"].([]interface{})
		gs.submitSaveRolls(player, rolls)
	case "start_attack":
		log.Printf("Received start_attack message from player %s", player.Name)
		gs.startAttack(player)
	case "roll_hit", "roll_wound", "roll_save":
		gs.handleManualDiceRoll(player, msgType)
	}
}

func (gs *GameServer) readyToFight(player *Player) {
	// Implementation for when player confirms ready to fight
	player.Status = "ready"
}

func (gs *GameServer) broadcastOnlinePlayersList() {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	// Create list of online players (excluding AI players and disconnected players)
	playerList := make([]map[string]interface{}, 0)
	for _, p := range gs.players {
		if p.Status != "disconnected" && !p.IsAI {
			playerList = append(playerList, map[string]interface{}{
				"name": p.Name,
				"id":   p.ID,
			})
		}
	}

	// Broadcast to all connected human players (not AI players)
	for _, player := range gs.players {
		if player.Status != "disconnected" && !player.IsAI && player.Conn != nil {
			gs.sendToPlayer(player, map[string]interface{}{
				"type":    "players_online",
				"players": playerList,
			})
		}
	}
}

func (gs *GameServer) selectAttackingUnit(player *Player, unitName, weaponName string) {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	// Find the player's match
	var match *Match
	for _, m := range gs.matches {
		if m.Player1 == player || m.Player2 == player {
			match = m
			break
		}
	}

	if match == nil || match.State != "fighting" {
		return
	}

	// Find the unit and weapon from the player's army
	var attackingUnit UnitSelection
	var weapon Weapon
	found := false

	for _, unit := range player.Army {
		if unit.UnitName == unitName {
			attackingUnit = unit
			// Find the weapon in this unit
			unitData := findUnitOptimized(player.Faction, unitName)
			if unitData != nil {
				for _, w := range unitData.Weapons {
					if w.Name == weaponName {
						weapon = w
						found = true
						break
					}
				}
			}
			break
		}
	}

	if !found {
		return
	}

	// Determine defender
	var defender *Player
	if match.Player1 == player {
		defender = match.Player2
	} else {
		defender = match.Player1
	}

	// Start attack sequence using the battle.go method
	gs.startAttackSequence(match, player, defender, attackingUnit, weapon)

	// Notify both players of the combat state
	gs.sendToPlayer(match.Player1, map[string]interface{}{
		"type":   "combat_state",
		"combat": match.CurrentCombat,
	})
	gs.sendToPlayer(match.Player2, map[string]interface{}{
		"type":   "combat_state",
		"combat": match.CurrentCombat,
	})
}

func (gs *GameServer) submitHitRolls(player *Player, rollsInterface []interface{}) {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	// Convert interface{} slice to int slice
	rolls := make([]int, len(rollsInterface))
	for i, r := range rollsInterface {
		if val, ok := r.(float64); ok {
			rolls[i] = int(val)
		}
	}

	// Find the player's match
	var match *Match
	for _, m := range gs.matches {
		if m.Player1 == player || m.Player2 == player {
			match = m
			break
		}
	}

	if match == nil || match.CurrentCombat == nil || match.CurrentCombat.Phase != "hit_rolls" {
		return
	}

	// Process hit rolls
	gs.processHitRolls(match, player, rolls)

	// Notify both players of updated combat state
	gs.sendToPlayer(match.Player1, map[string]interface{}{
		"type":   "combat_state",
		"combat": match.CurrentCombat,
	})
	gs.sendToPlayer(match.Player2, map[string]interface{}{
		"type":   "combat_state",
		"combat": match.CurrentCombat,
	})
}

func (gs *GameServer) submitWoundRolls(player *Player, rollsInterface []interface{}) {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	// Convert interface{} slice to int slice
	rolls := make([]int, len(rollsInterface))
	for i, r := range rollsInterface {
		if val, ok := r.(float64); ok {
			rolls[i] = int(val)
		}
	}

	// Find the player's match
	var match *Match
	for _, m := range gs.matches {
		if m.Player1 == player || m.Player2 == player {
			match = m
			break
		}
	}

	if match == nil || match.CurrentCombat == nil || match.CurrentCombat.Phase != "wound_rolls" {
		return
	}

	// Process wound rolls
	gs.processWoundRolls(match, player, rolls)

	// Notify both players of updated combat state
	gs.sendToPlayer(match.Player1, map[string]interface{}{
		"type":   "combat_state",
		"combat": match.CurrentCombat,
	})
	gs.sendToPlayer(match.Player2, map[string]interface{}{
		"type":   "combat_state",
		"combat": match.CurrentCombat,
	})
}

func (gs *GameServer) submitSaveRolls(player *Player, rollsInterface []interface{}) {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	// Convert interface{} slice to int slice
	rolls := make([]int, len(rollsInterface))
	for i, r := range rollsInterface {
		if val, ok := r.(float64); ok {
			rolls[i] = int(val)
		}
	}

	// Find the player's match
	var match *Match
	for _, m := range gs.matches {
		if m.Player1 == player || m.Player2 == player {
			match = m
			break
		}
	}

	if match == nil || match.CurrentCombat == nil || match.CurrentCombat.Phase != "save_rolls" {
		return
	}

	// Process save rolls and complete the attack sequence
	gs.processSaveRolls(match, player, rolls)

	// Check if the match is over (one army destroyed)
	if match.State == "finished" {
		gs.sendToPlayer(match.Player1, map[string]interface{}{
			"type":   "match_finished",
			"winner": match.Winner,
		})
		gs.sendToPlayer(match.Player2, map[string]interface{}{
			"type":   "match_finished",
			"winner": match.Winner,
		})
		return
	}

	// End the attack sequence and switch turns
	woundsInflicted := 0
	if match.CurrentCombat != nil {
		woundsInflicted = match.CurrentCombat.WoundsInflicted
	}
	gs.endAttackSequence(match, player, woundsInflicted)

	// Notify both players of updated combat state
	gs.sendToPlayer(match.Player1, map[string]interface{}{
		"type":   "combat_state",
		"combat": match.CurrentCombat,
	})
	gs.sendToPlayer(match.Player2, map[string]interface{}{
		"type":   "combat_state",
		"combat": match.CurrentCombat,
	})
}
