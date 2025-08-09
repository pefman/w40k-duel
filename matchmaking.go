package main

import (
	"log"
	"time"
)

func (gs *GameServer) joinMatchmaking(player *Player) {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	// Check if already in queue
	for _, p := range gs.waitingQueue {
		if p.ID == player.ID {
			return
		}
	}

	player.Status = "waiting"
	gs.waitingQueue = append(gs.waitingQueue, player)

	gs.sendToPlayer(player, map[string]interface{}{
		"type":    "matchmaking_status",
		"status":  "waiting",
		"message": "Looking for opponent...",
	})

	// Try to match players
	if len(gs.waitingQueue) >= 2 {
		player1 := gs.waitingQueue[0]
		player2 := gs.waitingQueue[1]
		gs.waitingQueue = gs.waitingQueue[2:]

		gs.createMatch(player1, player2)
	}
}

func (gs *GameServer) createMatch(player1, player2 *Player) {
	matchID := generateMatchID()
	battleID := generateBattleID()
	match := &Match{
		ID:       matchID,
		BattleID: battleID,
		Player1:  player1,
		Player2:  player2,
		State:    "selecting",
		Turn:     1,
		Log:      make([]string, 0),
		Created:  time.Now(),
	}

	gs.matches[matchID] = match
	player1.MatchID = matchID
	player2.MatchID = matchID
	player1.Status = "matched"
	player2.Status = "matched"

	log.Printf("Match created: %s vs %s (Match ID: %s, Battle ID: %s)", player1.Name, player2.Name, matchID, battleID)

	// Notify both players
	gs.sendToPlayer(player1, map[string]interface{}{
		"type":     "match_found",
		"match_id": matchID,
		"opponent": player2.Name,
	})
	gs.sendToPlayer(player2, map[string]interface{}{
		"type":     "match_found",
		"match_id": matchID,
		"opponent": player1.Name,
	})

	// Send available factions
	gs.sendFactions(player1)
	gs.sendFactions(player2)
}
