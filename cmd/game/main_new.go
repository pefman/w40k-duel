package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pefman/w40k-duel/internal/api"
	"github.com/pefman/w40k-duel/internal/stats"
)

// ========================= Config =========================
var (
	gameListenAddr string
	apiClient      *api.Client
)

// Build metadata injected via -ldflags at build time
var (
	buildVersion = "dev"
	buildTime    = ""
)

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func init() {
	p := os.Getenv("PORT")
	if p == "" {
		p = getenv("GAME_PORT", "8081")
	}
	gameListenAddr = ":" + p
	dataAPIBase := getenv("DATA_API_BASE", "http://localhost:8080")
	apiClient = api.NewClient(dataAPIBase)
}

// ========================= Game Types =========================
type Player struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Conn     *websocket.Conn `json:"-"`
	Loadout  Loadout         `json:"loadout,omitempty"`
	Wounds   int             `json:"wounds"`
	IsAI     bool            `json:"is_ai,omitempty"`
	Category string          `json:"category,omitempty"` // "ranged" or "melee" preference for AI
}

type Loadout struct {
	Faction string   `json:"faction"`
	Unit    string   `json:"unit"`
	Weapons []string `json:"weapons,omitempty"`
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

type WsMsg struct {
	Type string      `json:"type"`
	Data interface{} `json:"data,omitempty"`
}

// ========================= Global State =========================
var (
	rooms    = make(map[string]*Room)
	roomsMu  sync.RWMutex
	lobby    = make(map[string]*Player)
	lobbyMu  sync.RWMutex
	upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
)

// ========================= Main & HTTP Handlers =========================
func main() {
	http.HandleFunc("/", serveIndex)
	http.HandleFunc("/api/factions", handleFactions)
	http.HandleFunc("/api/units", handleUnits)
	http.HandleFunc("/api/lobby", handleLobby)
	http.HandleFunc("/api/leaderboard", handleLeaderboard)
	http.HandleFunc("/api/leaderboard/daily", handleLeaderboardDaily)
	http.HandleFunc("/api/debug/rooms", handleDebugRooms)
	http.HandleFunc("/ws", handleWebSocket)

	go matchmaker()

	log.Printf("w40k-duel-game %s starting on %s", buildVersion, gameListenAddr)
	if err := http.ListenAndServe(gameListenAddr, nil); err != nil {
		log.Fatal(err)
	}
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, indexHTML)
}

func handleFactions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	factions, err := apiClient.FetchFactions()
	if err != nil {
		log.Printf("factions error: %v", err)
		http.Error(w, "failed to fetch factions", 500)
		return
	}
	names := make([]string, len(factions))
	for i, f := range factions {
		names[i] = f.Name
	}
	json.NewEncoder(w).Encode(names)
}

func handleUnits(w http.ResponseWriter, r *http.Request) {
	faction := r.URL.Query().Get("faction")
	if faction == "" {
		http.Error(w, "faction parameter required", 400)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	units, err := apiClient.FetchUnits(faction)
	if err != nil {
		log.Printf("units error for %s: %v", faction, err)
		http.Error(w, "failed to fetch units", 500)
		return
	}
	json.NewEncoder(w).Encode(units)
}

func handleLobby(w http.ResponseWriter, r *http.Request) {
	lobbyMu.RLock()
	defer lobbyMu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	list := make([]LobbyEntry, 0, len(lobby))
	for _, p := range lobby {
		entry := LobbyEntry{
			ID:       p.ID,
			Name:     p.Name,
			Faction:  p.Loadout.Faction,
			Unit:     p.Loadout.Unit,
			Category: p.Category,
			Weapons:  len(p.Loadout.Weapons),
			Since:    time.Now().Unix() - 30, // placeholder
		}
		if u := findUnit(p.Loadout.Faction, p.Loadout.Unit); u.Name != "" {
			entry.Points = u.Points
		}
		list = append(list, entry)
	}
	json.NewEncoder(w).Encode(list)
}

func handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	// Placeholder leaderboard
	json.NewEncoder(w).Encode([]map[string]interface{}{})
}

func handleLeaderboardDaily(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats.Get())
}

func handleDebugRooms(w http.ResponseWriter, r *http.Request) {
	roomsMu.RLock()
	defer roomsMu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	list := make([]map[string]interface{}, 0, len(rooms))
	for _, room := range rooms {
		list = append(list, map[string]interface{}{
			"id":      room.ID,
			"p1":      room.Player1.Name,
			"p2":      room.Player2.Name,
			"turn":    room.Turn,
			"winner":  room.Winner,
			"updated": room.LastUpdated,
		})
	}
	json.NewEncoder(w).Encode(list)
}

// ========================= Core Game Logic =========================
func d6() int { return rand.Intn(6) + 1 }

func clamp(lo, hi, v int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func findUnit(faction, unit string) api.Unit {
	us, err := apiClient.FetchUnits(faction)
	if err == nil {
		for _, u := range us {
			if strings.EqualFold(u.Name, unit) {
				return u
			}
		}
		if len(us) > 0 {
			return us[0]
		}
	}
	return api.Unit{
		Faction: faction,
		Name:    unit,
		W:       10,
		T:       4,
		Sv:      4,
		Weapons: []api.Weapon{{Name: "Bolter", Range: "24", Attacks: 2, BS: 4, S: 4, AP: 0, D: 1}},
	}
}

// ========================= Game Flow =========================
func scheduleNextAttack(r *Room, delayMS int) {
	time.AfterFunc(time.Duration(delayMS)*time.Millisecond, func() {
		attack(r)
	})
}

func attack(r *Room) {
	roomsMu.Lock()
	defer roomsMu.Unlock()

	if r.Winner != "" {
		return // game over
	}

	var attacker *Player
	if r.CurrentPlayer == 1 {
		attacker = r.Player1
	} else {
		attacker = r.Player2
	}

	unit := findUnit(attacker.Loadout.Faction, attacker.Loadout.Unit)
	if len(unit.Weapons) == 0 {
		// Skip this player
		r.CurrentPlayer = 3 - r.CurrentPlayer
		r.Turn++
		broadcastGameState(r)
		scheduleNextAttack(r, 1000)
		return
	}

	// Pick first weapon for simplicity
	weapon := unit.Weapons[0]
	r.CurrentWeapon = weapon.Name
	resolveWeaponStep(attacker, r, weapon)
}

func resolveWeaponStep(attacker *Player, r *Room, weapon api.Weapon) {
	var defender *Player
	if attacker == r.Player1 {
		defender = r.Player2
	} else {
		defender = r.Player1
	}

	defenderUnit := findUnit(defender.Loadout.Faction, defender.Loadout.Unit)

	// Calculate attacks, hits, wounds, saves automatically
	attacks := weapon.Attacks
	if weapon.AttacksExpr != "" {
		// Simple dice expression handling
		expr := strings.ToUpper(weapon.AttacksExpr)
		if expr == "D6" {
			attacks = d6()
		} else if expr == "D3" {
			attacks = (d6() + 1) / 2
		}
	}

	// Roll to hit
	hits := 0
	for i := 0; i < attacks; i++ {
		roll := d6()
		need := weapon.BS
		if weapon.Torrent {
			hits++ // Torrent always hits
		} else if roll >= need || (weapon.LethalHits && roll == 6) {
			hits++
			// Sustained Hits
			if weapon.SustainedHits > 0 && roll == 6 {
				hits += weapon.SustainedHits
			}
		}
	}

	if hits == 0 {
		// No hits, end turn
		r.CurrentWeapon = ""
		r.CurrentPlayer = 3 - r.CurrentPlayer
		r.Turn++
		broadcastGameState(r)
		scheduleNextAttack(r, 1000)
		return
	}

	// Roll to wound
	wounds := 0
	needToWound := 4 // default
	if weapon.S >= defenderUnit.T*2 {
		needToWound = 2
	} else if weapon.S > defenderUnit.T {
		needToWound = 3
	} else if weapon.S == defenderUnit.T {
		needToWound = 4
	} else if weapon.S*2 <= defenderUnit.T {
		needToWound = 6
	} else {
		needToWound = 5
	}

	for i := 0; i < hits; i++ {
		roll := d6()
		if roll >= needToWound {
			wounds++
		}
	}

	if wounds == 0 {
		// No wounds, end turn
		r.CurrentWeapon = ""
		r.CurrentPlayer = 3 - r.CurrentPlayer
		r.Turn++
		broadcastGameState(r)
		scheduleNextAttack(r, 1000)
		return
	}

	// Auto-resolve saves
	save := defenderUnit.Sv + weapon.AP
	if defenderUnit.InvSv > 0 && defenderUnit.InvSv < save {
		save = defenderUnit.InvSv
	}
	save = clamp(2, 6, save)

	savedWounds := 0
	for i := 0; i < wounds; i++ {
		roll := d6()
		if roll >= save {
			savedWounds++
		}
	}

	unsavedWounds := wounds - savedWounds
	if unsavedWounds <= 0 {
		// All saved, end turn
		r.CurrentWeapon = ""
		r.CurrentPlayer = 3 - r.CurrentPlayer
		r.Turn++
		broadcastGameState(r)
		scheduleNextAttack(r, 1000)
		return
	}

	// Apply damage
	totalDamage := unsavedWounds * weapon.D
	if defenderUnit.DamageRed > 0 {
		totalDamage = clamp(0, totalDamage, totalDamage-defenderUnit.DamageRed*unsavedWounds)
	}

	// FNP rolls
	if defenderUnit.FNP > 0 && totalDamage > 0 {
		fnpSaves := 0
		for i := 0; i < totalDamage; i++ {
			if d6() >= defenderUnit.FNP {
				fnpSaves++
			}
		}
		totalDamage -= fnpSaves
	}

	if totalDamage > 0 {
		defender.Wounds -= totalDamage
		stats.MaybeTopDamage(totalDamage, attacker.Name, attacker.Loadout.Faction, attacker.Loadout.Unit, defender.Name, weapon.Name)
	}

	if save < 7 {
		stats.MaybeWorstSave(1, save, defender.Name, defender.Loadout.Faction, defender.Loadout.Unit, wounds)
	}

	// Check for victory
	if defender.Wounds <= 0 {
		r.Winner = attacker.Name
	}

	// End turn
	r.CurrentWeapon = ""
	r.CurrentPlayer = 3 - r.CurrentPlayer
	r.Turn++
	r.LastUpdated = time.Now().Unix()

	broadcastGameState(r)

	if r.Winner == "" {
		scheduleNextAttack(r, 1500)
	}
}

// ========================= Utilities =========================
func sendTo(p *Player, m WsMsg) {
	if p != nil && p.Conn != nil {
		if err := p.Conn.WriteJSON(m); err != nil {
			log.Printf("ws: write error to %s: %v", p.ID, err)
		}
	}
}

func broadcastGameState(r *Room) {
	state := map[string]any{
		"room":          r.ID,
		"turn":          r.Turn,
		"currentWeapon": r.CurrentWeapon,
		"p1":            summarizePlayer(r.Player1),
		"p2":            summarizePlayer(r.Player2),
		"winner":        r.Winner,
	}

	msg := WsMsg{Type: "state", Data: state}
	sendTo(r.Player1, msg)
	sendTo(r.Player2, msg)
}

func summarizePlayer(p *Player) map[string]any {
	return map[string]any{
		"id":       p.ID,
		"name":     p.Name,
		"wounds":   p.Wounds,
		"faction":  p.Loadout.Faction,
		"unit":     p.Loadout.Unit,
		"weapons":  len(p.Loadout.Weapons),
		"is_ai":    p.IsAI,
		"category": p.Category,
	}
}

// ========================= Placeholder implementations =========================
// These are simplified versions - full implementations would go in separate files

func matchmaker() {
	// Simplified matchmaker - full implementation would be in internal/game/matchmaker.go
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		// Basic matchmaking logic here
		lobbyMu.Lock()
		if len(lobby) >= 2 {
			var p1, p2 *Player
			for _, p := range lobby {
				if p1 == nil {
					p1 = p
				} else {
					p2 = p
					break
				}
			}
			if p1 != nil && p2 != nil {
				delete(lobby, p1.ID)
				delete(lobby, p2.ID)
				createRoom(p1, p2)
			}
		}
		lobbyMu.Unlock()
	}
}

func createRoom(p1, p2 *Player) {
	roomID := fmt.Sprintf("room_%d", time.Now().UnixNano())

	// Initialize wounds from unit stats
	u1 := findUnit(p1.Loadout.Faction, p1.Loadout.Unit)
	u2 := findUnit(p2.Loadout.Faction, p2.Loadout.Unit)
	p1.Wounds = u1.W
	p2.Wounds = u2.W

	room := &Room{
		ID:            roomID,
		Player1:       p1,
		Player2:       p2,
		Turn:          1,
		CurrentPlayer: 1,
		LastUpdated:   time.Now().Unix(),
	}

	roomsMu.Lock()
	rooms[roomID] = room
	roomsMu.Unlock()

	broadcastGameState(room)
	scheduleNextAttack(room, 1500)
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Simplified WebSocket handler - full implementation would be in internal/handlers/websocket.go
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade error: %v", err)
		return
	}
	defer conn.Close()

	playerID := fmt.Sprintf("player_%d", time.Now().UnixNano())
	player := &Player{
		ID:   playerID,
		Name: "Player",
		Conn: conn,
	}

	// Handle messages
	for {
		var msg WsMsg
		if err := conn.ReadJSON(&msg); err != nil {
			break
		}

		switch msg.Type {
		case "join":
			if data, ok := msg.Data.(map[string]interface{}); ok {
				if name, ok := data["name"].(string); ok {
					player.Name = name
				}
				if loadout, ok := data["loadout"].(map[string]interface{}); ok {
					if faction, ok := loadout["faction"].(string); ok {
						player.Loadout.Faction = faction
					}
					if unit, ok := loadout["unit"].(string); ok {
						player.Loadout.Unit = unit
					}
				}
			}
			lobbyMu.Lock()
			lobby[playerID] = player
			lobbyMu.Unlock()
		}
	}

	// Cleanup
	lobbyMu.Lock()
	delete(lobby, playerID)
	lobbyMu.Unlock()
}

// ========================= HTML Template =========================
const indexHTML = `<!DOCTYPE html>
<html>
<head>
  <title>W40K Duel</title>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <style>
    body { font-family: system-ui, sans-serif; background: #0a0c10; color: #f3f4f6; margin: 0; padding: 20px; }
    .container { max-width: 1200px; margin: 0 auto; }
    h1 { color: #c9a753; text-align: center; }
    button { 
      cursor: pointer; padding: 11px 16px; border-radius: 12px; 
      border: 1px solid rgba(201,167,83,.45); 
      background: linear-gradient(180deg,#1a2330,#0e141e); 
      color: #f3f4f6; font-weight: 700; letter-spacing: .04em; 
      box-shadow: inset 0 1px 0 rgba(255,255,255,.08), 0 6px 16px rgba(0,0,0,.35); 
      transition: transform .05s ease, box-shadow .15s ease, filter .15s ease;
    }
    button:hover { filter: brightness(1.05); }
    button:active { transform: translateY(1px); }
    button[disabled] { opacity: .5; cursor: not-allowed; }
    #btn-attack { 
      background: linear-gradient(180deg, #c9a753, #9b7e37); 
      color: #0a0c10; border-color: #e8d6a6; text-transform: uppercase;
    }
    .game-area { display: grid; grid-template-columns: 1fr 1fr; gap: 20px; margin: 20px 0; }
    .player-panel { 
      background: rgba(26,35,48,.6); border-radius: 12px; padding: 20px; 
      border: 1px solid rgba(201,167,83,.2);
    }
    .combat-log { 
      background: rgba(14,20,30,.8); border-radius: 8px; padding: 15px; 
      margin: 10px 0; max-height: 200px; overflow-y: auto;
    }
    .status { text-align: center; font-size: 18px; margin: 20px 0; }
    .lobby { display: none; }
    .postgame { display: none; }
  </style>
</head>
<body>
  <div class="container">
    <h1>Warhammer 40,000 Duel Arena</h1>
    
    <div class="status" id="status">Connecting...</div>
    
    <div class="lobby" id="lobby">
      <h2>Join Game</h2>
      <input type="text" id="playerName" placeholder="Your name" value="Player">
      <select id="factionSelect"><option>Loading...</option></select>
      <select id="unitSelect"><option>Select faction first</option></select>
      <button onclick="joinGame()">Join Lobby</button>
    </div>
    
    <div class="game-area" id="gameArea" style="display:none;">
      <div class="player-panel">
        <h3 id="p1-name">Player 1</h3>
        <div id="p1-info"></div>
        <button id="btn-attack" onclick="startCombat()" style="display:none;">Start Combat</button>
      </div>
      <div class="player-panel">
        <h3 id="p2-name">Player 2</h3>
        <div id="p2-info"></div>
      </div>
      <div class="combat-log" id="combatLog"></div>
    </div>
    
    <div class="postgame" id="postgame">
      <h2>Game Over!</h2>
      <div id="winner"></div>
      <button onclick="location.reload()">Play Again</button>
    </div>
  </div>

  <script>
    let ws = null;
    let gameState = null;

    function connect() {
      const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
      ws = new WebSocket(protocol + '//' + location.host + '/ws');
      
      ws.onopen = () => {
        document.getElementById('status').textContent = 'Connected - Loading factions...';
        loadFactions();
      };
      
      ws.onmessage = (event) => {
        const msg = JSON.parse(event.data);
        if (msg.type === 'state') {
          handleGameState(msg.data);
        }
      };
      
      ws.onclose = () => {
        document.getElementById('status').textContent = 'Disconnected';
        setTimeout(connect, 1000);
      };
    }

    async function loadFactions() {
      try {
        const response = await fetch('/api/factions');
        const factions = await response.json();
        const select = document.getElementById('factionSelect');
        select.innerHTML = '<option value="">Select Faction</option>';
        factions.forEach(faction => {
          select.innerHTML += '<option value="' + faction + '">' + faction + '</option>';
        });
        document.getElementById('lobby').style.display = 'block';
        document.getElementById('status').textContent = 'Ready to join game';
      } catch (err) {
        document.getElementById('status').textContent = 'Failed to load factions';
      }
    }

    async function loadUnits(faction) {
      if (!faction) return;
      try {
        const response = await fetch('/api/units?faction=' + encodeURIComponent(faction));
        const units = await response.json();
        const select = document.getElementById('unitSelect');
        select.innerHTML = '<option value="">Select Unit</option>';
        units.forEach(unit => {
          select.innerHTML += '<option value="' + unit.Name + '">' + unit.Name + '</option>';
        });
      } catch (err) {
        console.error('Failed to load units:', err);
      }
    }

    function joinGame() {
      const name = document.getElementById('playerName').value || 'Player';
      const faction = document.getElementById('factionSelect').value;
      const unit = document.getElementById('unitSelect').value;
      
      if (!faction || !unit) {
        alert('Please select faction and unit');
        return;
      }

      ws.send(JSON.stringify({
        type: 'join',
        data: {
          name: name,
          loadout: { faction: faction, unit: unit }
        }
      }));
      
      document.getElementById('lobby').style.display = 'none';
      document.getElementById('status').textContent = 'Waiting for opponent...';
    }

    function handleGameState(state) {
      gameState = state;
      
      // Show game area
      document.getElementById('gameArea').style.display = 'grid';
      document.getElementById('lobby').style.display = 'none';
      
      // Update player info
      if (state.p1) {
        document.getElementById('p1-name').textContent = state.p1.name;
        document.getElementById('p1-info').innerHTML = 
          state.p1.faction + ' - ' + state.p1.unit + '<br>Wounds: ' + state.p1.wounds;
      }
      
      if (state.p2) {
        document.getElementById('p2-name').textContent = state.p2.name;
        document.getElementById('p2-info').innerHTML = 
          state.p2.faction + ' - ' + state.p2.unit + '<br>Wounds: ' + state.p2.wounds;
      }
      
      // Update status
      if (state.winner) {
        document.getElementById('status').textContent = state.winner + ' wins!';
        document.getElementById('postgame').style.display = 'block';
        document.getElementById('winner').textContent = state.winner + ' is victorious!';
      } else if (state.currentWeapon) {
        document.getElementById('status').textContent = 'Combat in progress...';
        document.getElementById('btn-attack').style.display = 'none';
      } else {
        document.getElementById('status').textContent = 'Turn ' + state.turn;
        document.getElementById('btn-attack').style.display = 'block';
      }
    }

    function startCombat() {
      // Combat is now automatic, this button just triggers the start
      document.getElementById('btn-attack').style.display = 'none';
    }

    // Event listeners
    document.getElementById('factionSelect').addEventListener('change', (e) => {
      loadUnits(e.target.value);
    });

    // Initialize
    connect();
  </script>
</body>
</html>`
