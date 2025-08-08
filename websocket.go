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
