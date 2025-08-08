package main

import (
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type GameServer struct {
	players      map[string]*Player
	matches      map[string]*Match
	waitingQueue []*Player
	upgrader     websocket.Upgrader
	mutex        sync.RWMutex
}

func NewGameServer() *GameServer {
	return &GameServer{
		players:      make(map[string]*Player),
		matches:      make(map[string]*Match),
		waitingQueue: make([]*Player, 0),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

func (gs *GameServer) sendToPlayer(player *Player, message map[string]interface{}) {
	if player.Conn != nil {
		err := player.Conn.WriteJSON(message)
		if err != nil {
			log.Printf("Error sending message to player %s: %v", player.ID, err)
		}
	}
}

func (gs *GameServer) handleDisconnect(player *Player) {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	log.Printf("Player %s disconnected: %s", player.Name, player.ID)
	player.Status = "disconnected"

	// Check if player was in a match and clean up accordingly
	for matchID, match := range gs.matches {
		if match.Player1.ID == player.ID || match.Player2.ID == player.ID {
			// If one of the players is AI, remove the AI player from the players map
			if match.Player1.IsAI {
				log.Printf("Cleaning up AI player %s after human disconnection", match.Player1.Name)
				delete(gs.players, match.Player1.ID)
			}
			if match.Player2.IsAI {
				log.Printf("Cleaning up AI player %s after human disconnection", match.Player2.Name)
				delete(gs.players, match.Player2.ID)
			}

			// Clean up the match
			log.Printf("Removing match %s due to player disconnection", matchID)
			delete(gs.matches, matchID)
			break
		}
	}

	// Clean up player from players map after a delay (to handle reconnections)
	go func(playerID string) {
		time.Sleep(30 * time.Second) // Give 30 seconds for potential reconnection
		gs.mutex.Lock()
		defer gs.mutex.Unlock()
		if p, exists := gs.players[playerID]; exists && p.Status == "disconnected" {
			log.Printf("Removing disconnected player %s from memory", playerID)
			delete(gs.players, playerID)
		}
	}(player.ID)

	// Broadcast updated player list
	go gs.broadcastOnlinePlayersList()
}

func (gs *GameServer) setPlayerName(player *Player, name string) {
	// Only allow setting the name if it's a valid saved name and not empty
	trimmedName := strings.TrimSpace(name)
	if len(trimmedName) == 0 || len(trimmedName) > 50 {
		// Invalid name, keep the current generated name
		return
	}

	// Update player name to the saved one
	gs.mutex.Lock()
	player.Name = trimmedName
	gs.mutex.Unlock()

	log.Printf("Player %s using saved name: %s", player.ID, trimmedName)

	// Send updated player info
	gs.sendToPlayer(player, map[string]interface{}{
		"type":      "player_info",
		"player_id": player.ID,
		"name":      player.Name,
	})
}
