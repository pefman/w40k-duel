package main

import (
	"time"

	"github.com/gorilla/websocket"
)

// Game structures
type Player struct {
	ID      string          `json:"id"`
	Name    string          `json:"name"`
	Conn    *websocket.Conn `json:"-"`
	Status  string          `json:"status"` // "waiting", "matched", "selecting", "ready", "fighting"
	Faction string          `json:"faction"`
	Army    []UnitSelection `json:"army"`
	MatchID string          `json:"match_id"`
	IsAI    bool            `json:"is_ai"`
}

type AIPlayer struct {
	*Player
	Difficulty string `json:"difficulty"` // "easy", "medium", "hard"
}

type UnitSelection struct {
	UnitName string   `json:"unit_name"`
	Quantity int      `json:"quantity"`
	Weapons  []Weapon `json:"weapons"`
}

type Weapon struct {
	Name     string `json:"name"`
	Type     string `json:"type"`  // "Ranged" or "Melee"
	Range    string `json:"range"` // For ranged weapons
	Attacks  string `json:"attacks"`
	Skill    string `json:"skill"` // BS for ranged, WS for melee
	Strength string `json:"strength"`
	AP       string `json:"ap"`
	Damage   string `json:"damage"`
	Keywords string `json:"keywords"` // Special abilities/keywords
}

type Unit struct {
	Name       string            `json:"name"`
	Movement   string            `json:"movement"`
	WS         string            `json:"ws"`
	BS         string            `json:"bs"`
	Strength   string            `json:"strength"`
	Toughness  string            `json:"toughness"`
	Wounds     string            `json:"wounds"`
	Attacks    string            `json:"attacks"`
	Leadership string            `json:"leadership"`
	Save       string            `json:"save"`
	Weapons    []Weapon          `json:"weapons"`
	Abilities  map[string]string `json:"abilities"`
}

type Faction struct {
	Name  string `json:"name"`
	Units []Unit `json:"units"`
}

type Match struct {
	ID                   string    `json:"id"`
	Player1              *Player   `json:"player1"`
	Player2              *Player   `json:"player2"`
	State                string    `json:"state"` // "selecting", "initiative", "fighting", "finished"
	Turn                 int       `json:"turn"`
	Log                  []string  `json:"log"`
	Winner               string    `json:"winner"`
	Created              time.Time `json:"created"`
	Player1Initiative    int       `json:"player1_initiative"`
	Player2Initiative    int       `json:"player2_initiative"`
	Player1InitiativeSet bool      `json:"player1_initiative_set"`
	Player2InitiativeSet bool      `json:"player2_initiative_set"`
	CurrentPlayer        *Player   `json:"current_player"`
}

type DiceRoll struct {
	PlayerID string `json:"player_id"`
	Dice     int    `json:"dice"`
	Result   int    `json:"result"`
}
