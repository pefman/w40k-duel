package models

// ========================= Domain Models =========================
// Minimal shapes for gameplay. API responses are mapped into this.

type Weapon struct {
	Name    string `json:"name"`
	Range   string `json:"range"`
	Attacks int    `json:"attacks"`
	// Original attacks expression from the data source (e.g., "4D6", "D3+3").
	// When present, combat uses this expression to roll the number of attacks each time.
	AttacksExpr string `json:"attacks_expr,omitempty"`
	BS          int    `json:"bs"`
	S           int    `json:"s"`
	AP          int    `json:"ap"`
	D           int    `json:"d"`
	// Derived rules/keywords
	LethalHits        bool     `json:"lethal_hits,omitempty"`
	TwinLinked        bool     `json:"twin_linked,omitempty"`
	Torrent           bool     `json:"torrent,omitempty"`
	DevastatingWounds bool     `json:"devastating_wounds,omitempty"`
	SustainedHits     int      `json:"sustained_hits,omitempty"`
	AntiTag           string   `json:"anti_tag,omitempty"`
	AntiValue         int      `json:"anti_value,omitempty"`
	Tags              []string `json:"tags,omitempty"`
}

type Unit struct {
	Faction  string   `json:"Faction,omitempty"`
	Name     string   `json:"Name"`
	W        int      `json:"W"`
	T        int      `json:"T"`
	Sv       int      `json:"Sv"`
	Weapons  []Weapon `json:"Weapons"`
	DefaultW string   `json:"default_weapon,omitempty"`
	Options  []string `json:"Options,omitempty"`
	Points   int      `json:"Points,omitempty"`
	// Extras
	InvSv      int      `json:"InvSv,omitempty"`
	InvSvDescr string   `json:"InvSvDescr,omitempty"`
	Keywords   []string `json:"Keywords,omitempty"`
	FNP        int      `json:"FNP,omitempty"` // 0 if none, else threshold (e.g., 5 means 5+)
	DamageRed  int      `json:"DR,omitempty"`  // per-attack damage reduction
}

type Loadout struct {
	Faction string   `json:"faction"`
	Unit    string   `json:"unit"`
	Weapons []string `json:"weapons,omitempty"`
}

// Game state types
type Player struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Conn     interface{} `json:"-"` // Will be *websocket.Conn in actual use
	Loadout  Loadout     `json:"loadout,omitempty"`
	Wounds   int         `json:"wounds"`
	IsAI     bool        `json:"is_ai,omitempty"`
	Category string      `json:"category,omitempty"` // "ranged" or "melee" preference for AI
}

type Room struct {
	ID            string  `json:"id"`
	Player1       *Player `json:"player1"`
	Player2       *Player `json:"player2"`
	Turn          int     `json:"turn"`
	CurrentPlayer int     `json:"current_player"` // 1 or 2
	Winner        string  `json:"winner,omitempty"`
	// Combat state
	CurrentWeapon string       `json:"current_weapon,omitempty"`
	PendingSave   *PendingSave `json:"pending_save,omitempty"`
	LastUpdated   int64        `json:"last_updated"`
}

// Pending save step (set after attacker computes wounds; cleared after defender rolls)
type PendingSave struct {
	Count      int    `json:"count"`       // Number of wounds to save
	AP         int    `json:"ap"`          // Armor penetration value
	DamageEach int    `json:"damage_each"` // Damage per unsaved wound
	Attacker   string `json:"attacker"`    // For logging
	Weapon     string `json:"weapon"`      // For logging
}

type LobbyEntry struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Faction  string `json:"faction,omitempty"`
	Unit     string `json:"unit,omitempty"`
	Points   int    `json:"points,omitempty"`
	Category string `json:"category,omitempty"` // "ranged" or "melee"
	Weapons  int    `json:"weapons,omitempty"`  // number of selected weapons
	Since    int64  `json:"since"`              // timestamp
}

// WebSocket message structure
type WsMsg struct {
	Type string      `json:"type"`
	Data interface{} `json:"data,omitempty"`
}
