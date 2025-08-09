package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ========================= Config (env-configurable) =========================
// Defaults can be overridden via environment variables:
//   GAME_PORT       (default: 8081)
//   DATA_API_BASE   (default: http://localhost:8080)

var (
	gameListenAddr string
	dataAPIBase    string
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
	dataAPIBase = getenv("DATA_API_BASE", "http://localhost:8080")
}

// ========================= Domain Models =========================
// Minimal shapes for gameplay. API responses are mapped into this.

type Weapon struct {
	Name    string `json:"name"`
	Range   string `json:"range"`
	Attacks int    `json:"attacks"`
	BS      int    `json:"bs"`
	S       int    `json:"s"`
	AP      int    `json:"ap"`
	D       int    `json:"d"`
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
}

type Loadout struct {
	Faction string   `json:"faction"`
	Unit    string   `json:"unit"`
	Weapons []string `json:"weapons,omitempty"`
	Weapon  string   `json:"weapon,omitempty"`
}

// ========================= API Client =========================

var httpClient = &http.Client{Timeout: 8 * time.Second}

func apiGet[T any](path string, out *T) error {
	base := strings.TrimRight(dataAPIBase, "/")
	url := base + path
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	req.Header.Set("Accept", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("api status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

type apiFaction struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type apiUnit struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type apiWeapon struct {
	Name     string `json:"name"`
	Range    string `json:"range"`
	Attacks  string `json:"attacks"`
	BSOrWS   string `json:"bs_ws"`
	Strength string `json:"strength"`
	AP       string `json:"ap"`
	Damage   string `json:"damage"`
}

type apiModel struct {
	Name string `json:"name"`
	T    string `json:"T"`
	Sv   string `json:"Sv"`
	W    string `json:"W"`
}

func FetchFactions() ([]apiFaction, error) {
	var res []apiFaction
	if err := apiGet("/api/factions", &res); err != nil {
		return nil, err
	}
	return res, nil
}

func toSlug(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "’", "")
	s = strings.ReplaceAll(s, "'", "")
	s = strings.ReplaceAll(s, "&", "and")
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "--", "-")
	return s
}

// FetchUnits builds gameplay-ready units for a faction name by calling the data API.
func FetchUnits(factionName string) ([]Unit, error) {
	slug := toSlug(factionName)
	// 1) List units for the faction
	var list []apiUnit
	if err := apiGet("/api/"+slug+"/units", &list); err != nil {
		return nil, err
	}
	// 2) For each unit, fetch models and weapons to extract basic stats
	out := make([]Unit, 0, len(list))
	for _, u := range list {
		var models []apiModel
		if err := apiGet("/api/"+slug+"/"+u.ID+"/models", &models); err != nil {
			// Skip units that fail to load
			continue
		}
		// Pick first model for simplicity
		W, T, Sv := 10, 4, 4
		if len(models) > 0 {
			W = mustAtoi(models[0].W, 10)
			T = mustAtoi(models[0].T, 4)
			Sv = parseSave(models[0].Sv)
		}
		var apiW []apiWeapon
		if err := apiGet("/api/"+slug+"/"+u.ID+"/weapons", &apiW); err != nil {
			apiW = nil
		}
		// Options (valid wargear text lines)
		var apiOpts []struct {
			Line        int    `json:"line"`
			Bullet      string `json:"bullet"`
			Description string `json:"description"`
		}
		_ = apiGet("/api/"+slug+"/"+u.ID+"/options", &apiOpts)
		opts := make([]string, 0, len(apiOpts))
		for _, o := range apiOpts {
			opts = append(opts, strings.TrimSpace(o.Description))
		}
		// Costs
		var costs []struct {
			Line        int    `json:"line"`
			Description string `json:"description"`
			Cost        string `json:"cost"`
		}
		_ = apiGet("/api/"+slug+"/"+u.ID+"/costs", &costs)
		pts := 0
		if len(costs) > 0 {
			// pick first cost number
			for _, c := range costs {
				if n := mustAtoi(c.Cost, 0); n > 0 {
					pts = n
					break
				}
			}
		}
		weps := make([]Weapon, 0, len(apiW))
		for _, w := range apiW {
			weps = append(weps, Weapon{
				Name:    w.Name,
				Range:   w.Range,
				Attacks: parseAttacks(w.Attacks),
				BS:      parseSave(w.BSOrWS),
				S:       mustAtoi(w.Strength, 4),
				AP:      parseAP(w.AP),
				D:       mustAtoi(w.Damage, 1),
			})
		}
		// If no weapons found, add a generic one
		if len(weps) == 0 {
			weps = []Weapon{{Name: "Generic", Range: "24", Attacks: 2, BS: 4, S: T, AP: 0, D: 1}}
		}
		out = append(out, Unit{Faction: factionName, Name: u.Name, W: W, T: T, Sv: Sv, Weapons: weps, DefaultW: weps[0].Name, Options: opts, Points: pts})
	}
	// Stable order by name
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	if len(out) == 0 {
		return nil, errors.New("no units found for faction")
	}
	return out, nil
}

func mustAtoi(s string, def int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	// Strip trailing '+' or '-' and non-digits except first minus
	s2 := strings.TrimSuffix(s, "+")
	// Some fields could be like "-1" for AP
	if n, err := strconv.Atoi(s2); err == nil {
		return n
	}
	// Try parse first integer in string
	num := ""
	for i, r := range s {
		if (r == '-' && i == 0) || (r >= '0' && r <= '9') {
			num += string(r)
		} else if num != "" {
			break
		}
	}
	if n, err := strconv.Atoi(num); err == nil {
		return n
	}
	return def
}

func parseSave(s string) int { // e.g., "3+" -> 3
	return clamp(2, 6, mustAtoi(s, 4))
}
func parseAP(s string) int { // e.g., "-1" -> 1 added to save in our simple model
	if s == "" {
		return 0
	}
	// AP is stored as "-X"; we convert to positive modifier to save roll
	ap := mustAtoi(s, 0)
	if ap < 0 {
		return -ap
	}
	return ap
}
func parseAttacks(s string) int {
	// Handle numeric or dice like D6 by picking a reasonable default
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" {
		return 2
	}
	if s == "D6" {
		return 4
	}
	if s == "D3" {
		return 2
	}
	return mustAtoi(s, 2)
}

// ========================= Matchmaking & Rooms =========================

type Player struct {
	ID      string
	Conn    *websocket.Conn
	Name    string
	IsAI    bool
	Ready   bool
	Loadout Loadout
	Wounds  int
	Unit    Unit
}

type Room struct {
	ID       string
	P1, P2   *Player
	Turn     string
	Started  bool
	Finished bool
	Winner   string
	Mu       sync.Mutex
	Phase    string // "attack" or "save"
	// Pending save step (set after attacker computes wounds; cleared after defender rolls)
	PendingSaves int
	PendingNeed  int
	PendingDmg   int
	PendingBy    string // attacker ID for this pending step
	// Multi-weapon sequence tracking
	AttackQueue   []string
	AttackIndex   int
	CurrentWeapon string
}

var (
	matchQueue   = make(chan *Player, 32)
	roomsMu      sync.Mutex
	rooms        = map[string]*Room{}
	playersIndex sync.Map // id -> roomID
)

// ========================= Web =========================

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

func main() {
	http.HandleFunc("/", serveIndex)
	http.HandleFunc("/ws", handleWS)
	http.HandleFunc("/api/factions", handleFactions)
	http.HandleFunc("/api/units", handleUnits)
	http.HandleFunc("/debug/rooms", handleDebugRooms)

	go matchmaker()

	log.Printf("go40k duel game listening on %s (DATA_API_BASE=%s)", gameListenAddr, dataAPIBase)
	log.Fatal(http.ListenAndServe(gameListenAddr, nil))
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, indexHTML)
}

func handleFactions(w http.ResponseWriter, r *http.Request) {
	list, err := FetchFactions()
	if err != nil || len(list) == 0 {
		// Fallback sample names
		list = []apiFaction{{Name: "Orks"}, {Name: "Necrons"}, {Name: "Adeptus Custodes"}}
	}
	sort.Slice(list, func(i, j int) bool { return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name) })
	// Return only names for dropdown simplicity
	type outFac struct {
		Name        string `json:"name"`
		Factionname string `json:"factionname"`
	}
	out := make([]outFac, 0, len(list))
	for _, f := range list {
		out = append(out, outFac{Name: f.Name, Factionname: f.Name})
	}
	_ = json.NewEncoder(w).Encode(out)
}

func handleUnits(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("faction")
	if name == "" {
		http.Error(w, "missing faction", 400)
		return
	}
	units, err := FetchUnits(name)
	if err != nil || len(units) == 0 {
		http.Error(w, "failed to fetch units", http.StatusBadGateway)
		return
	}
	_ = json.NewEncoder(w).Encode(units)
}

// Debug: Inspect rooms and queue
func handleDebugRooms(w http.ResponseWriter, r *http.Request) {
	type pInfo struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		AI    bool   `json:"ai"`
		Ready bool   `json:"ready"`
	}
	type roomInfo struct {
		ID       string `json:"id"`
		Started  bool   `json:"started"`
		Finished bool   `json:"finished"`
		Turn     string `json:"turn"`
		P1       pInfo  `json:"p1"`
		P2       pInfo  `json:"p2"`
	}
	roomsMu.Lock()
	list := make([]roomInfo, 0, len(rooms))
	for _, rr := range rooms {
		list = append(list, roomInfo{
			ID:       rr.ID,
			Started:  rr.Started,
			Finished: rr.Finished,
			Turn:     rr.Turn,
			P1:       pInfo{ID: rr.P1.ID, Name: rr.P1.Name, AI: rr.P1.IsAI, Ready: rr.P1.Ready},
			P2:       pInfo{ID: rr.P2.ID, Name: rr.P2.Name, AI: rr.P2.IsAI, Ready: rr.P2.Ready},
		})
	}
	roomsMu.Unlock()
	out := map[string]any{
		"queueLen": len(matchQueue),
		"rooms":    list,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// ----------------- Matchmaker -----------------

func matchmaker() {
	for {
		p1 := <-matchQueue
		log.Printf("matchmaker: got p1 id=%s name=%q ai=%v (queueLen=%d)", p1.ID, p1.Name, p1.IsAI, len(matchQueue))
		select {
		case p2 := <-matchQueue:
			log.Printf("matchmaker: pairing p1=%s (%s) with p2=%s (%s)", p1.ID, p1.Name, p2.ID, p2.Name)
			createRoom(p1, p2)
		case <-time.After(1200 * time.Millisecond):
			log.Printf("matchmaker: timeout waiting for p2 (p1.ai=%v)", p1.IsAI)
			if p1.IsAI {
				log.Printf("matchmaker: p1 requested AI opponent — creating AI")
				createRoom(p1, makeAIPlayer())
			} else {
				log.Printf("matchmaker: waiting for second player to join for p1=%s", p1.ID)
				p2 := <-matchQueue
				log.Printf("matchmaker: second player arrived: p2=%s (%s)", p2.ID, p2.Name)
				createRoom(p1, p2)
			}
		}
	}
}

func createRoom(p1, p2 *Player) {
	id := fmt.Sprintf("room_%d", time.Now().UnixNano())
	room := &Room{ID: id, P1: p1, P2: p2}
	roomsMu.Lock()
	rooms[id] = room
	roomsMu.Unlock()
	playersIndex.Store(p1.ID, id)
	playersIndex.Store(p2.ID, id)
	log.Printf("room: created id=%s p1=%s(%s, ai=%v) p2=%s(%s, ai=%v)", id, p1.ID, p1.Name, p1.IsAI, p2.ID, p2.Name, p2.IsAI)
	go roomLoop(room)
}

// Create a simple AI opponent with a random (or fallback) unit
func makeAIPlayer() *Player {
	ai := &Player{ID: fmt.Sprintf("ai_%d", time.Now().UnixNano()), Name: "AI Opponent", IsAI: true}
	// Try fetch a random faction and unit
	facs, _ := FetchFactions()
	fName := "Necrons"
	if len(facs) > 0 {
		fName = facs[rand.Intn(len(facs))].Name
	}
	us, err := FetchUnits(fName)
	var u Unit
	if err == nil && len(us) > 0 {
		u = us[rand.Intn(len(us))]
	} else {
		u = Unit{Name: "Generic Squad", W: 10, T: 4, Sv: 4, Weapons: []Weapon{{Name: "Bolter", Range: "24", Attacks: 2, BS: 4, S: 4, AP: 0, D: 1}}}
	}
	w := u.DefaultW
	if w == "" && len(u.Weapons) > 0 {
		w = u.Weapons[0].Name
	}
	ai.Loadout = Loadout{Faction: fName, Unit: u.Name, Weapons: []string{w}, Weapon: w}
	ai.Unit = u
	ai.Wounds = u.W
	ai.Ready = true
	return ai
}

// ----------------- Room Loop & Combat -----------------

type wsMsg struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

func roomLoop(r *Room) {
	broadcast := func(m wsMsg) { sendTo(r.P1, m); sendTo(r.P2, m) }
	broadcast(wsMsg{Type: "status", Data: map[string]any{"room": r.ID, "message": "Match found. Choose faction, unit, weapon."}})
	log.Printf("room %s: started, waiting for players to be ready", r.ID)

	// Wait for both players ready
	waitStart := time.Now()
	tick := time.NewTicker(1 * time.Second)
	defer tick.Stop()
	for {
		time.Sleep(50 * time.Millisecond)
		if r.P1.Ready && r.P2.Ready {
			break
		}
		select {
		case <-tick.C:
			log.Printf("room %s: still waiting ready — p1.ready=%v p2.ready=%v (elapsed=%s)", r.ID, r.P1.Ready, r.P2.Ready, time.Since(waitStart).Truncate(time.Second))
		default:
		}
	}

	// Initialize
	for _, p := range []*Player{r.P1, r.P2} {
		if p.Unit.Name == "" {
			p.Unit = findUnit(p.Loadout.Faction, p.Loadout.Unit)
		}
		if p.Wounds == 0 {
			p.Wounds = p.Unit.W
		}
	}

	// Roll-off
	roll1, roll2 := d6(), d6()
	for roll1 == roll2 {
		roll1, roll2 = d6(), d6()
	}
	first := r.P1
	if roll2 > roll1 {
		first = r.P2
	}
	r.Turn = first.ID
	broadcast(wsMsg{Type: "log", Data: fmt.Sprintf("Roll-off: %s vs %s → %s first", r.P1.Name, r.P2.Name, first.Name)})
	broadcastGameState(r)
}

func d6() int { return rand.Intn(6) + 1 }

func findUnit(faction, unit string) Unit {
	us, err := FetchUnits(faction)
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
	return Unit{Faction: faction, Name: unit, W: 10, T: 4, Sv: 4, Weapons: []Weapon{{Name: "Bolter", Range: "24", Attacks: 2, BS: 4, S: 4, AP: 0, D: 1}}}
}

func sendTo(p *Player, m wsMsg) {
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
		"phase":         r.Phase,
		"currentWeapon": r.CurrentWeapon,
		"p1":            summarizePlayer(r.P1),
		"p2":            summarizePlayer(r.P2),
		"winner":        r.Winner,
	}
	if r.Phase == "save" && r.PendingSaves > 0 {
		state["pendingSaves"] = map[string]int{"count": r.PendingSaves, "need": r.PendingNeed, "dmg": r.PendingDmg}
	}
	sendTo(r.P1, wsMsg{Type: "state", Data: state})
	sendTo(r.P2, wsMsg{Type: "state", Data: state})
}

func summarizePlayer(p *Player) map[string]any {
	return map[string]any{
		"id":     p.ID,
		"name":   p.Name,
		"ai":     p.IsAI,
		"ready":  p.Ready,
		"wounds": p.Wounds,
		"maxW":   p.Unit.W,
		"unit": map[string]any{
			"faction": p.Loadout.Faction,
			"name":    p.Unit.Name,
			"W":       p.Unit.W,
			"T":       p.Unit.T,
			"Sv":      p.Unit.Sv,
			"Points":  p.Unit.Points,
			"weapons": p.Unit.Weapons,
		},
		"loadout": p.Loadout,
	}
}

// ----------------- WebSocket per player -----------------

func handleWS(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		name = randomFunnyName()
	}
	wantAI := r.URL.Query().Get("ai") == "1"
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	player := &Player{ID: fmt.Sprintf("p_%d", time.Now().UnixNano()), Conn: conn, Name: name, IsAI: wantAI}
	log.Printf("ws: connect id=%s name=%q ai=%v from=%s", player.ID, name, wantAI, r.RemoteAddr)
	// Tell the client its own player ID
	_ = player.Conn.WriteJSON(wsMsg{Type: "you", Data: map[string]string{"id": player.ID}})
	go wsReader(player)
	matchQueue <- player
	log.Printf("ws: enqueued player id=%s (queueLen=%d)", player.ID, len(matchQueue))
}

// randomFunnyName builds a nickname from playful adjectives and a serious WH-style surname
func randomFunnyName() string {
	adjs := []string{
		"Sexy", "Cringe", "Spicy", "Sassy", "Cheeky", "Awkward", "Sneaky", "Grim", "Grimdark", "Heretical", "Pious", "Stoic", "Stalwart", "Brutal", "Warped", "Shiny", "Rusty", "Lucky", "Unlucky",
	}
	surnames := []string{
		// Warhammer-flavored surnames (characters and common names)
		"Gaunt", "Creed", "Yarrick", "Cain", "Eisenhorn", "Ravenor", "Calgar", "Guilliman", "Sicarius", "Telion", "Trazyn", "Cawl", "Mephiston", "Solar", "Ventris", "Varren", "Severus", "Drake",
	}
	a := adjs[rand.Intn(len(adjs))]
	// 30% chance to chain two adjectives for extra fun
	if rand.Intn(100) < 30 {
		b := adjs[rand.Intn(len(adjs))]
		if b != a {
			a = a + " " + b
		} // avoid exact dup
	}
	s := surnames[rand.Intn(len(surnames))]
	return a + " " + s
}

type clientIn struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

func wsReader(p *Player) {
	defer func() {
		if p.Conn != nil {
			_ = p.Conn.Close()
		}
		log.Printf("ws: closed id=%s name=%q", p.ID, p.Name)
	}()
	for {
		var in clientIn
		if err := p.Conn.ReadJSON(&in); err != nil {
			log.Printf("ws: read error id=%s: %v", p.ID, err)
			return
		}
		log.Printf("ws: recv id=%s type=%s", p.ID, in.Type)
		roomIDAny, ok := playersIndex.Load(p.ID)
		if !ok {
			log.Printf("ws: no room yet for player id=%s", p.ID)
			continue
		}
		r := getRoom(roomIDAny.(string))
		if r == nil {
			log.Printf("ws: room not found for id=%s room=%v", p.ID, roomIDAny)
			continue
		}
		switch in.Type {
		case "choose":
			var sel Loadout
			_ = json.Unmarshal(in.Data, &sel)
			// Normalize weapons selection
			if len(sel.Weapons) == 0 && sel.Weapon != "" {
				sel.Weapons = []string{sel.Weapon}
			}
			if sel.Weapon == "" && len(sel.Weapons) > 0 {
				sel.Weapon = sel.Weapons[0]
			}
			p.Loadout = sel
			p.Unit = findUnit(sel.Faction, sel.Unit)
			p.Wounds = p.Unit.W
			log.Printf("room %s: %s chose faction=%q unit=%q weapons=%v (active=%q)", r.ID, p.ID, sel.Faction, sel.Unit, sel.Weapons, sel.Weapon)
			sendTo(p, wsMsg{Type: "log", Data: fmt.Sprintf("Selected %s – %s (%s)", sel.Faction, sel.Unit, sel.Weapon)})
			broadcastGameState(r)
		case "set_weapon":
			var body struct {
				Weapon string `json:"weapon"`
			}
			_ = json.Unmarshal(in.Data, &body)
			if body.Weapon != "" {
				p.Loadout.Weapon = body.Weapon
				log.Printf("room %s: %s set active weapon=%q", r.ID, p.ID, body.Weapon)
				sendTo(p, wsMsg{Type: "log", Data: fmt.Sprintf("Active weapon: %s", body.Weapon)})
				broadcastGameState(r)
			}
		case "ready":
			p.Ready = true
			log.Printf("room %s: %s is READY", r.ID, p.ID)
			sendTo(p, wsMsg{Type: "log", Data: "Ready! Waiting for opponent..."})
		case "attack":
			log.Printf("room %s: attack requested by %s", r.ID, p.ID)
			roomStartAttackSequence(r, p)
		case "save_rolls":
			// Defender submitted manual save dice results
			var body struct {
				Rolls []int `json:"rolls"`
			}
			_ = json.Unmarshal(in.Data, &body)
			r.Mu.Lock()
			if r.Finished || r.Phase != "save" || r.PendingSaves <= 0 {
				r.Mu.Unlock()
				break
			}
			// Determine defender
			defender := r.P1
			if defender.ID == r.Turn {
				defender = r.P2
			}
			if defender.ID != p.ID {
				r.Mu.Unlock()
				break
			}
			need := r.PendingNeed
			dmgPer := max(1, r.PendingDmg)
			rolls := body.Rolls
			if len(rolls) == 0 { // fallback if client sent nothing
				for i := 0; i < r.PendingSaves; i++ {
					rolls = append(rolls, d6())
				}
			}
			// compute outcome
			unsaved := 0
			logLines := []string{fmt.Sprintf("%s rolls %d saves (need %d+):", defender.Name, len(rolls), need)}
			for i, rv := range rolls {
				res := "SAVED"
				if rv < need {
					res = "FAILED"
					unsaved++
				}
				logLines = append(logLines, fmt.Sprintf("Save %d: rolled %d → %s", i+1, rv, res))
			}
			totalDmg := unsaved * dmgPer
			before := defender.Wounds
			defender.Wounds = max(0, defender.Wounds-totalDmg)
			logLines = append(logLines, fmt.Sprintf("%d unsaved → %d damage. %s Wounds: %d → %d", unsaved, totalDmg, defender.Name, before, defender.Wounds))
			if defender.Wounds <= 0 {
				r.Finished = true
				r.Winner = r.Turn
				logLines = append(logLines, fmt.Sprintf("%s destroyed!", defender.Unit.Name))
			}
			// clear pending and advance turn
			r.PendingSaves, r.PendingNeed, r.PendingDmg, r.PendingBy = 0, 0, 0, ""
			r.Phase = ""
			// Continue next weapon if sequence remains
			attacker := r.P1
			if attacker.ID != r.Turn {
				attacker = r.P2
			}
			var nextWeapon string
			if !r.Finished && r.AttackQueue != nil && r.AttackIndex+1 < len(r.AttackQueue) {
				r.AttackIndex++
				nextWeapon = r.AttackQueue[r.AttackIndex]
				r.CurrentWeapon = nextWeapon
			} else if !r.Finished {
				// end of sequence → flip turn
				if r.Turn == r.P1.ID {
					r.Turn = r.P2.ID
				} else {
					r.Turn = r.P1.ID
				}
				r.AttackQueue, r.AttackIndex, r.CurrentWeapon = nil, 0, ""
			}
			r.Mu.Unlock()
			sendTo(r.P1, wsMsg{Type: "log_multi", Data: logLines})
			sendTo(r.P2, wsMsg{Type: "log_multi", Data: logLines})
			if nextWeapon != "" {
				// proceed to next weapon in sequence
				resolveWeaponStep(attacker, r, nextWeapon)
			} else {
				broadcastGameState(r)
			}
		}
	}
}

func getRoom(id string) *Room { roomsMu.Lock(); defer roomsMu.Unlock(); return rooms[id] }

// Start a multi-weapon attack sequence for attacker using selected weapons
func roomStartAttackSequence(r *Room, attacker *Player) {
	r.Mu.Lock()
	if r.Finished || r.Turn != attacker.ID {
		r.Mu.Unlock()
		return
	}
	r.Phase = "attack"
	// Build queue from selected weapons; fallback to single active weapon
	queue := attacker.Loadout.Weapons
	if len(queue) == 0 && attacker.Loadout.Weapon != "" {
		queue = []string{attacker.Loadout.Weapon}
	}
	if len(queue) == 0 {
		r.Mu.Unlock()
		sendTo(attacker, wsMsg{Type: "log", Data: "No weapons selected."})
		return
	}
	r.AttackQueue = append([]string(nil), queue...)
	r.AttackIndex = 0
	r.CurrentWeapon = r.AttackQueue[0]
	cur := r.CurrentWeapon
	r.Mu.Unlock()
	resolveWeaponStep(attacker, r, cur)
}

// Resolve one weapon step: compute hits/wounds; if wounds>0, set save phase; else continue to next weapon or end turn
func resolveWeaponStep(attacker *Player, r *Room, weaponName string) {
	r.Mu.Lock()
	if r.Finished || r.Turn != attacker.ID {
		r.Mu.Unlock()
		return
	}
	defender := r.P1
	if defender.ID == attacker.ID {
		defender = r.P2
	}
	wep := weaponByName(attacker.Unit, weaponName)
	if wep == nil && len(attacker.Unit.Weapons) > 0 {
		wep = &attacker.Unit.Weapons[0]
	}
	if wep == nil {
		r.Mu.Unlock()
		return
	}
	logLines := []string{fmt.Sprintf("%s attacks with %s", attacker.Name, wep.Name)}
	// Hits: gather rolls first, then animate to clients, then log
	hitNeed := clamp(2, 6, wep.BS)
	attacks := max(1, wep.Attacks)
	hitRolls := make([]int, 0, attacks)
	hits := 0
	for i := 0; i < attacks; i++ {
		roll := d6()
		hitRolls = append(hitRolls, roll)
		if roll >= hitNeed {
			hits++
		}
	}
	r.Mu.Unlock()
	// animate hits
	sendTo(r.P1, wsMsg{Type: "rolls", Data: map[string]any{"phase": "hit", "need": hitNeed, "rolls": hitRolls, "weapon": wep.Name, "attacker": attacker.Name}})
	sendTo(r.P2, wsMsg{Type: "rolls", Data: map[string]any{"phase": "hit", "need": hitNeed, "rolls": hitRolls, "weapon": wep.Name, "attacker": attacker.Name}})
	time.Sleep(600 * time.Millisecond)
	r.Mu.Lock()
	for i, roll := range hitRolls {
		logLines = append(logLines, fmt.Sprintf("Hit %d: rolled %d vs %d+ → %s", i+1, roll, hitNeed, tern(roll >= hitNeed, "HIT", "MISS")))
	}
	// Wounds
	woundTarget := woundNeeded(wep.S, defender.Unit.T)
	woundRolls := make([]int, 0, hits)
	wounds := 0
	for i := 0; i < hits; i++ {
		roll := d6()
		woundRolls = append(woundRolls, roll)
		if roll >= woundTarget {
			wounds++
		}
	}
	r.Mu.Unlock()
	// animate wounds
	sendTo(r.P1, wsMsg{Type: "rolls", Data: map[string]any{"phase": "wound", "need": woundTarget, "rolls": woundRolls, "weapon": wep.Name, "attacker": attacker.Name}})
	sendTo(r.P2, wsMsg{Type: "rolls", Data: map[string]any{"phase": "wound", "need": woundTarget, "rolls": woundRolls, "weapon": wep.Name, "attacker": attacker.Name}})
	time.Sleep(600 * time.Millisecond)
	r.Mu.Lock()
	for i, roll := range woundRolls {
		logLines = append(logLines, fmt.Sprintf("Wound %d: need %d+, rolled %d → %s", i+1, woundTarget, roll, tern(roll >= woundTarget, "WOUND", "FAIL")))
	}
	if wounds <= 0 {
		// No saves; continue to next weapon or end turn
		sendTo(r.P1, wsMsg{Type: "log_multi", Data: logLines})
		sendTo(r.P2, wsMsg{Type: "log_multi", Data: logLines})
		var nextWeapon string
		if r.AttackQueue != nil && r.AttackIndex+1 < len(r.AttackQueue) {
			r.AttackIndex++
			nextWeapon = r.AttackQueue[r.AttackIndex]
			r.CurrentWeapon = nextWeapon
		} else {
			// end turn
			if r.Turn == r.P1.ID {
				r.Turn = r.P2.ID
			} else {
				r.Turn = r.P1.ID
			}
			r.AttackQueue, r.AttackIndex, r.CurrentWeapon = nil, 0, ""
		}
		r.Mu.Unlock()
		if nextWeapon != "" {
			resolveWeaponStep(attacker, r, nextWeapon)
		} else {
			broadcastGameState(r)
		}
		return
	}
	// Saves become manual: set pending
	saveNeed := clamp(2, 6, defender.Unit.Sv+wep.AP)
	r.Phase = "save"
	r.PendingSaves = wounds
	r.PendingNeed = saveNeed
	r.PendingDmg = max(1, wep.D)
	r.PendingBy = attacker.ID
	r.Mu.Unlock()
	logLines = append(logLines, fmt.Sprintf("%d potential wounds → defender to roll %d saves (need %d+)", wounds, wounds, saveNeed))
	sendTo(r.P1, wsMsg{Type: "log_multi", Data: logLines})
	sendTo(r.P2, wsMsg{Type: "log_multi", Data: logLines})
	broadcastGameState(r)
	// If defender is AI, roll saves automatically after a short delay with animation
	if defender.IsAI && wounds > 0 {
		go func() {
			time.Sleep(400 * time.Millisecond)
			r.Mu.Lock()
			need := r.PendingNeed
			count := r.PendingSaves
			r.Mu.Unlock()
			rolls := make([]int, 0, count)
			for i := 0; i < count; i++ {
				rolls = append(rolls, d6())
			}
			// animate save rolls
			sendTo(r.P1, wsMsg{Type: "rolls", Data: map[string]any{"phase": "save", "need": need, "rolls": rolls}})
			sendTo(r.P2, wsMsg{Type: "rolls", Data: map[string]any{"phase": "save", "need": need, "rolls": rolls}})
			time.Sleep(600 * time.Millisecond)
			b, _ := json.Marshal(map[string]any{"rolls": rolls})
			wsReaderHandleSave(defender, b)
		}()
	}
}

// helper to process save rolls path for AI without duplicating logic
func wsReaderHandleSave(p *Player, data []byte) {
	roomIDAny, ok := playersIndex.Load(p.ID)
	if !ok {
		return
	}
	r := getRoom(roomIDAny.(string))
	if r == nil {
		return
	}
	var body struct {
		Rolls []int `json:"rolls"`
	}
	_ = json.Unmarshal(data, &body)
	r.Mu.Lock()
	if r.Finished || r.Phase != "save" || r.PendingSaves <= 0 {
		r.Mu.Unlock()
		return
	}
	defender := r.P1
	if defender.ID == r.Turn {
		defender = r.P2
	}
	if defender.ID != p.ID {
		r.Mu.Unlock()
		return
	}
	need := r.PendingNeed
	dmgPer := max(1, r.PendingDmg)
	rolls := body.Rolls
	if len(rolls) == 0 {
		for i := 0; i < r.PendingSaves; i++ {
			rolls = append(rolls, d6())
		}
	}
	unsaved := 0
	logLines := []string{fmt.Sprintf("%s rolls %d saves (need %d+):", defender.Name, len(rolls), need)}
	for i, rv := range rolls {
		res := "SAVED"
		if rv < need {
			res = "FAILED"
			unsaved++
		}
		logLines = append(logLines, fmt.Sprintf("Save %d: rolled %d → %s", i+1, rv, res))
	}
	totalDmg := unsaved * dmgPer
	before := defender.Wounds
	defender.Wounds = max(0, defender.Wounds-totalDmg)
	logLines = append(logLines, fmt.Sprintf("%d unsaved → %d damage. %s Wounds: %d → %d", unsaved, totalDmg, defender.Name, before, defender.Wounds))
	if defender.Wounds <= 0 {
		r.Finished = true
		r.Winner = r.Turn
		logLines = append(logLines, fmt.Sprintf("%s destroyed!", defender.Unit.Name))
	}
	r.PendingSaves, r.PendingNeed, r.PendingDmg, r.PendingBy = 0, 0, 0, ""
	r.Phase = ""
	if !r.Finished {
		if r.Turn == r.P1.ID {
			r.Turn = r.P2.ID
		} else {
			r.Turn = r.P1.ID
		}
	}
	r.Mu.Unlock()
	sendTo(r.P1, wsMsg{Type: "log_multi", Data: logLines})
	sendTo(r.P2, wsMsg{Type: "log_multi", Data: logLines})
	broadcastGameState(r)
}

func weaponByName(u Unit, name string) *Weapon {
	for i := range u.Weapons {
		if strings.EqualFold(u.Weapons[i].Name, name) {
			return &u.Weapons[i]
		}
	}
	return nil
}

func woundNeeded(S, T int) int {
	if S >= 2*T {
		return 2
	}
	if S > T {
		return 3
	}
	if S == T {
		return 4
	}
	if S*2 <= T {
		return 6
	}
	return 5
}
func tern[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func clamp(lo, hi, v int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ========================= Frontend (embedded) =========================

const indexHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>go40k – Online Duel (MVP)</title>
  <style>
    @import url('https://fonts.googleapis.com/css2?family=Cinzel:wght@500;700&family=Montserrat:wght@400;600&display=swap');
    :root{ --bg:#0a0c10; --panel:#0f131a; --card:#0b1016; --panel-edge:#131924; --text:#e5e7eb; --muted:#9aa4b2; --gold:#c9a753; --gold-soft:#e5d5a5; --accent:#3a5a9e; --red:#b91c1c; --shadow:0 6px 20px rgba(0,0,0,.45);} *{box-sizing:border-box} html,body{height:100%}
    body{ margin:0; color:var(--text); background: radial-gradient(1200px 600px at 10% -10%, rgba(30,41,59,.35), transparent 60%), radial-gradient(900px 400px at 110% 10%, rgba(30,41,59,.2), transparent 60%), linear-gradient(180deg,#0a0c10 0%,#07090d 100%); font-family:'Montserrat', ui-sans-serif, system-ui, -apple-system, Segoe UI, Roboto, Arial; }
    header.site-header{ display:grid; grid-template-columns:auto 1fr auto; align-items:center; gap:16px; padding:14px 20px; background:linear-gradient(180deg,#0f131a,#0b0f15); border-bottom:2px solid var(--gold); box-shadow:var(--shadow); position:sticky; top:0; z-index:10; }
    .brand{display:flex; align-items:center; gap:10px} .brand .eagle{font-size:20px; filter:drop-shadow(0 0 6px rgba(201,167,83,.35))} .brand .wordmark{font-family:'Cinzel', serif; font-weight:700; letter-spacing:.12em; font-size:18px}
    .nav{display:flex; gap:18px} .nav a{font-weight:600; text-decoration:none; color:var(--muted); position:relative} .nav a:hover{color:var(--gold)} .nav a::after{content:""; position:absolute; left:0; right:0; bottom:-8px; height:2px; background:linear-gradient(90deg,transparent,var(--gold),transparent); opacity:0; transition:opacity .2s} .nav a:hover::after{opacity:1}
    .tray{display:flex; gap:8px} .pill{display:inline-block; padding:4px 10px; border-radius:999px; border:1px solid rgba(201,167,83,.5); background:rgba(201,167,83,.08); color:var(--gold);} 
		main{display:grid; grid-template-columns:360px 1fr 360px; gap:16px; padding:18px; max-width:1300px; margin:0 auto}
    .card{background:linear-gradient(180deg, rgba(255,255,255,.02), rgba(0,0,0,.28)); border:1px solid var(--panel-edge); border-radius:14px; padding:14px; box-shadow:var(--shadow);} h2{font-family:'Cinzel', serif; font-size:18px; margin:0 0 10px; color:var(--gold-soft); letter-spacing:.06em}
    select, input[type=text]{width:100%; padding:12px 12px; border-radius:10px; border:1px solid #243042; background:#0a0f16; color:var(--text); outline:none} select:focus, input[type=text]:focus{border-color:var(--gold); box-shadow:0 0 0 2px rgba(201,167,83,.25)}
    .grid{display:grid; grid-template-columns:1fr 1fr; gap:10px} .row{display:flex; align-items:center; justify-content:space-between; padding:6px 0; color:#cbd5e1}
    button{cursor:pointer; padding:11px 16px; border-radius:12px; border:1px solid rgba(201,167,83,.45); background:linear-gradient(180deg,#1a2330,#0e141e); color:#f3f4f6; font-weight:700; letter-spacing:.04em; box-shadow: inset 0 1px 0 rgba(255,255,255,.08), 0 6px 16px rgba(0,0,0,.35); transition: transform .05s ease, box-shadow .15s ease, filter .15s ease;} button:hover{filter:brightness(1.05)} button:active{transform:translateY(1px)} button[disabled]{opacity:.5; cursor:not-allowed}
	#btn-attack{background:linear-gradient(180deg, #c9a753, #9b7e37); color:#0a0c10; border-color:#e8d6a6; text-transform:uppercase}
	/* Weapon selection highlight */
	#weaponList label{ transition: background .15s ease, border-color .15s ease }
	#weaponList label.active{ background: rgba(201,167,83,.08); border-color: var(--gold) }
	/* Dice animation */
	.dice{ width:34px; height:34px; border-radius:6px; background:#111827; border:1px solid #374151; display:inline-flex; align-items:center; justify-content:center; margin:4px; font-weight:800; box-shadow: inset 0 1px 0 rgba(255,255,255,.05); animation: roll .6s ease }
	.dice.good{ border-color:#22c55e; box-shadow:0 0 0 2px rgba(34,197,94,.2) inset }
	.dice.bad{ border-color:#ef4444; box-shadow:0 0 0 2px rgba(239,68,68,.2) inset }
	@keyframes roll { 0%{ transform: rotate(0) scale(.9) } 50%{ transform: rotate(20deg) scale(1.05) } 100%{ transform: rotate(0) scale(1) } }
    .hpbar{height:14px; background:#131922; border-radius:10px; overflow:hidden; border:1px solid #2a3546} .hpfill{height:100%; background:linear-gradient(90deg,#c9a753,#7ed3ee)}
    .log{height:380px; overflow:auto; white-space:pre-wrap; background:#0a0f16; border-radius:12px; padding:12px; border:1px solid #232c3a; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size:13px}
    #p1,#p2{background:linear-gradient(180deg, rgba(255,255,255,.02), rgba(0,0,0,.18)); border:1px solid #263145; border-radius:12px}
	.winner{color:#22c55e; font-weight:800}
	/* Rolls tray */
	#rollsTray{ display:none; margin:8px 0 10px; padding:10px; border:1px solid #243042; border-radius:10px; background:#0b0f16 }
	#rollsHeader{ font-weight:700; color:#e5e7eb; margin-bottom:6px }

		/* --- Mobile tweaks --- */
		@media (max-width: 1100px){
			main{ grid-template-columns: 1fr; padding: 12px; gap: 12px; }
			header.site-header{ grid-template-columns: 1fr auto; }
			.nav{ display: none; }
		}
		@media (max-width: 720px){
			body{ font-size: 15px; }
			h2{ font-size: 16px; }
			.card{ padding: 12px; border-radius: 12px; }
			.grid{ grid-template-columns: 1fr; }
			.log{ height: 220px; }
			button{ width: 100%; }
			#btn-attack{ position: sticky; bottom: 8px; width: 100%; box-shadow: 0 8px 18px rgba(0,0,0,.35); }
			.pill{ font-size: 12px; padding: 3px 8px; }
			#weaponList label{ padding: 8px 10px; border: 1px solid #243042; border-radius: 10px; margin-bottom: 8px; display:flex; align-items:center; gap:8px; }
			#weaponList input[type=checkbox]{ transform: scale(1.2); }
		}
		/* improve tap targets & wrapping */
		* { -webkit-tap-highlight-color: rgba(0,0,0,0); }
		.row span{ word-break: break-word; }
		main{ padding-bottom: env(safe-area-inset-bottom); }
  </style>
</head>
<body>
  <header class="site-header">
    <div class="brand"><span class="eagle">⚔️</span><span class="wordmark">GO40K DUEL</span></div>
    <nav class="nav"><a href="#">Battle</a><a href="#">Armoury</a><a href="#">Lore</a></nav>
    <div class="tray"><span id="status" class="pill">Ready</span></div>
  </header>
  <main>
			<section id="setup" class="card">
				<h2 style="margin-top:0">1) Your Army</h2>
			<div class="grid">
				<div><label>Faction</label><select id="faction"></select></div>
								<div><label>Unit</label><select id="unit"></select><div id="unitStats" class="muted" style="margin-top:6px; font-size:12px; color:#9aa4b2"></div></div>
			</div>
			<div style="margin-top:8px">
				<label>Weapons (pick same type)</label>
				<div id="weaponList"></div>
				<div id="optionsHint" class="muted" style="margin-top:6px; font-size:12px; color:#9aa4b2"></div>
				<div class="muted" style="margin-top:6px; font-size:12px; color:#9aa4b2">Tip: if you pick a melee weapon, all selections must be melee; same for ranged.</div>
			</div>
				<h2 style="margin-top:12px">2) Choose Opponent Type</h2>
				<div class="grid"><button id="btn-ai">Play vs AI</button><button id="btn-pvp">Play vs Player</button></div>
      <div class="cta" style="margin-top:12px"><button id="btn-ready" disabled>Ready Up</button></div>
    </section>
    <section class="card center">
	<h2>Battlefield</h2>
	<div class="row"><span>Turn:</span> <span id="turn">—</span></div>
	<div id="rollsTray"><div id="rollsHeader"></div><div id="rollsDice"></div></div>
	<div id="versus" class="row"><span>Waiting for match…</span></div>
	<button id="btn-attack" disabled>Attack</button>
	<div id="saveUI" style="display:none; margin-top:10px">
		<div class="row"><span>Saves to roll:</span><span id="saveNeed">—</span></div>
		<div id="diceTray" style="margin:8px 0"></div>
		<div class="grid"><button id="btn-roll-saves">Roll Saves</button></div>
	</div>
	<div id="postgame" style="display:none; margin-top:12px" class="grid"><button id="btn-rematch">Rematch</button><button id="btn-back">Back to Setup</button></div>
      <div class="log" id="log"></div>
    </section>
    <section class="card">
      <h2>Combatants</h2>
			<div id="p1" class="card" style="padding:10px">
				<div class="row"><strong id="p1name"></strong><span class="pill" id="p1ai"></span></div>
				<div class="row"><span id="p1unit"></span><span id="p1cfg" class="pill"></span></div>
				<div class="hpbar"><div id="p1hp" class="hpfill" style="width:0%"></div></div>
				<div class="row"><span>Wounds Left:</span><span id="p1wounds">0</span></div>
			</div>
			<div id="p2" class="card" style="padding:10px; margin-top:8px">
				<div class="row"><strong id="p2name"></strong><span class="pill" id="p2ai"></span></div>
				<div class="row"><span id="p2unit"></span><span id="p2cfg" class="pill"></span></div>
				<div class="hpbar"><div id="p2hp" class="hpfill" style="width:0%"></div></div>
				<div class="row"><span>Wounds Left:</span><span id="p2wounds">0</span></div>
			</div>
      <div class="row" style="margin-top:10px"><span>Winner:</span> <span id="winner" class="winner">—</span></div>
    </section>
  </main>
  <script>
    const $ = (id)=>document.getElementById(id);
		let ws; let state={}; let me=null; let chosenWeapons=[]; let weaponType=null; // 'melee' or 'ranged'
		async function loadFactions(){ const res=await fetch('/api/factions'); const data=await res.json(); const fac=$('faction'); fac.innerHTML=''; data.forEach(f=>{ const o=document.createElement('option'); o.value=f.name||f.factionname; o.textContent=o.value; fac.appendChild(o); }); await loadUnits(); }
	async function loadUnits(){ const f=$('faction').value; if(!f) return; const res=await fetch('/api/units?faction='+encodeURIComponent(f)); const units=await res.json(); const uSel=$('unit'); uSel.innerHTML=''; const unitStats=$('unitStats'); unitStats.textContent=''; units.forEach(u=>{ const o=document.createElement('option'); o.value=u.Name||u.name; const pts=(u.Points||u.points||0); const label=(u.Name||u.name)+ (u.W? (' — W:'+u.W+' T:'+u.T) : '') + (pts? (' — '+pts+'pts') : ''); o.textContent=label; uSel.appendChild(o); }); const first=units[0]; if(first){ unitStats.textContent='W: '+(first.W||first.wounds||'?')+'  T: '+(first.T||'?') + (first.Points? ('  •  '+first.Points+' pts') : ''); } loadWeaponsList(first); uSel.onchange=()=>{ const picked=units.find(x=>(x.Name||x.name)=== (uSel.value.split(' — ')[0]) ); unitStats.textContent='W: '+(picked.W||picked.wounds||'?')+'  T: '+(picked.T||'?') + (picked.Points? ('  •  '+picked.Points+' pts') : ''); loadWeaponsList(picked); }; }
			function loadWeaponsList(unit){
				const box=$('weaponList'); box.innerHTML=''; chosenWeapons=[]; weaponType=null; const optHint=$('optionsHint');
				const list=(unit.Weapons||unit.weapons||[]);
				// Build a set of allowed weapon names from unit.Options text lines (simple heuristic: match by inclusion of weapon name case-insensitive)
				const allowed=new Set();
				const opts=(unit.Options||unit.options||[]);
				const names=list.map(w=> (w.name||w.Name) );
				opts.forEach(line=>{ const s=line.toLowerCase(); names.forEach(n=>{ if(n && s.includes(n.toLowerCase())) allowed.add(n); }); });
				optHint.textContent = opts && opts.length? ('Options: '+opts.slice(0,3).join(' | ')+(opts.length>3?' …':'')) : '';
				list.forEach(w=>{
					const name=w.name||w.Name;
					const line=document.createElement('label'); line.style.display='block';
					const cb=document.createElement('input'); cb.type='checkbox'; cb.value=name; cb.onchange=()=>{ onWeaponToggle(w, cb.checked); highlightActiveWeapon(); send('set_weapon', {weapon: chosenWeapons[0]||''}); };
					if(allowed.size>0 && !allowed.has(name)) { cb.disabled=true; line.style.opacity=.6; }
					const S = (w.s ?? w.S ?? '?');
					const AP = (w.ap ?? w.AP ?? 0);
					const A = (w.attacks ?? w.Attacks ?? '?');
					const BSWS = (w.bs ?? w.BS ?? '?');
					const D = (w.d ?? w.D ?? '?');
					const details = ' (S:'+S+' AP:'+AP+' A:'+A+' BS/WS:'+BSWS+' D:'+D+')';
					line.appendChild(cb);
					const txt=document.createElement('span'); txt.textContent = name + details; line.appendChild(txt);
					box.appendChild(line);
				});
				$('btn-ready').disabled=false;
		highlightActiveWeapon();
			}
	function highlightActiveWeapon(){ const inputs=[...document.querySelectorAll('#weaponList label')]; inputs.forEach(l=>l.classList.remove('active')); const first=chosenWeapons[0]; if(!first) return; [...document.querySelectorAll('#weaponList input[type=checkbox]')].forEach((i)=>{ if(i.value===first){ i.parentElement.classList.add('active'); }}); }
		function onWeaponToggle(w, checked){ const name=w.name||w.Name; const rng=(w.range??w.Range??'').toString().toLowerCase(); const isMelee = rng==='melee' || rng==='' || rng==='0' || rng==='0"' || rng==='0”'; const type=isMelee?'melee':'ranged'; if(weaponType && type!==weaponType){ // enforce same type
				// undo toggle
				const inputs=[...document.querySelectorAll('#weaponList input[type=checkbox]')]; const me=inputs.find(i=>i.value===name); if(me){ me.checked=!checked; }
				alert('Please select weapons of the same type ('+weaponType+').'); return; }
			if(checked){ if(!chosenWeapons.includes(name)) chosenWeapons.push(name); weaponType=weaponType||type; } else { chosenWeapons=chosenWeapons.filter(x=>x!==name); if(chosenWeapons.length===0) weaponType=null; }
		}
	function connect(ai=false){ const proto=(location.protocol==='https:'?'wss':'ws'); ws=new WebSocket(proto+'://'+location.host+'/ws?ai='+(ai?1:0)); ws.onopen=()=>setStatus('Matchmaking…'); ws.onmessage=(ev)=>{ const msg=JSON.parse(ev.data); if(msg.type==='you'){ me=msg.data.id; } if(msg.type==='state') onState(msg.data); if(msg.type==='rolls') onRolls(msg.data); if(msg.type==='status') logLine(msg.data.message); if(msg.type==='log') logLine(msg.data); if(msg.type==='log_multi') msg.data.forEach(line=>logLine(line)); }; ws.onclose=()=>{ setStatus('Disconnected'); me=null; }; }
		function send(type, data){ ws && ws.readyState===1 && ws.send(JSON.stringify({type, data})); }
		function choose(){ const payload={ faction:$('faction').value, unit:$('unit').value, weapons:chosenWeapons, weapon:chosenWeapons[0]||'' }; send('choose', payload); }
    function ready(){ send('ready', {}); }
	function attack(){ send('attack', {}); }
	function onState(s){ state=s; updateUI(); }
	function onRolls(ev){ const {phase, need, rolls, weapon, attacker} = ev; const tray=$('rollsTray'), hdr=$('rollsHeader'), dice=$('rollsDice'); tray.style.display='block'; const title = (phase==='hit'?'Hit rolls':phase==='wound'?'Wound rolls':'Save rolls'); hdr.textContent = (weapon? (title+' for '+weapon) : title) + (need? (' — need '+need+'+') : ''); dice.innerHTML=''; (rolls||[]).forEach(v=>{ const d=document.createElement('div'); d.className='dice'; d.textContent=v; if(need) d.classList.add(v>=need?'good':'bad'); dice.appendChild(d); }); setTimeout(()=>{ tray.style.display='none'; dice.innerHTML=''; }, 1400); }
		function updateUI(){ if(!state.p1||!state.p2) return; const p1=state.p1, p2=state.p2; $('p1name').textContent=p1.name||'—'; $('p2name').textContent=p2.name||'—'; $('p1ai').textContent=p1.ai?'AI':'Player'; $('p2ai').textContent=p2.ai?'AI':'Player'; $('p1unit').textContent=p1.unit.faction+' — '+p1.unit.name; $('p2unit').textContent=p2.unit.faction+' — '+p2.unit.name; $('p1cfg').textContent=(p1.loadout.weapons&&p1.loadout.weapons.length?p1.loadout.weapons.join(', '):(p1.loadout.weapon||'—')); $('p2cfg').textContent=(p2.loadout.weapons&&p2.loadout.weapons.length?p2.loadout.weapons.join(', '):(p2.loadout.weapon||'—')); const p1pct=p1.unit.W?Math.max(0,Math.round(100*p1.wounds/p1.unit.W)):0; const p2pct=p2.unit.W?Math.max(0,Math.round(100*p2.wounds/p2.unit.W)):0; $('p1hp').style.width=p1pct+'%'; $('p2hp').style.width=p2pct+'%'; $('p1wounds').textContent = (p1.wounds||0)+' / '+(p1.unit.W||0); $('p2wounds').textContent = (p2.wounds||0)+' / '+(p2.unit.W||0); $('turn').textContent = state.turn===p1.id? p1.name : (state.turn===p2.id? p2.name : '—'); $('winner').textContent = state.winner? ((state.winner===p1.id? p1.name : p2.name) + ' 🎉') : '—'; const inGame=!!state.turn && !state.winner; $('btn-attack').disabled = !inGame; // Set dynamic label
	    const myTurn = state.turn===me; const phase = state.phase||'attack'; $('btn-attack').textContent = phase==='save' && !myTurn ? 'Save' : 'Attack';
	    // Manual save UI
	    const pending = state.pendingSaves; const isDefender = (state.turn!==me);
	    if(phase==='save' && pending && isDefender){ $('saveUI').style.display='block'; const need=pending.need||0; const cnt=pending.count||0; $('saveNeed').textContent = cnt+' × '+need+'+'; renderDice(cnt, need); } else { $('saveUI').style.display='none'; $('diceTray').innerHTML=''; }
			// Hide setup during game; show postgame actions
			$('setup').style.display = inGame? 'none' : 'block';
			$('postgame').style.display = state.winner? 'grid' : 'none';
			setStatus(state.winner? 'Game over' : (inGame? 'In game' : 'Ready'));
		}
    function renderDice(n, need){ const tray=$('diceTray'); tray.innerHTML=''; for(let i=0;i<n;i++){ const d=document.createElement('div'); d.className='dice'; d.dataset.need=need; d.textContent='?'; tray.appendChild(d);} }
	$('btn-roll-saves').onclick=()=>{ const btn=$('btn-roll-saves'); if(btn.disabled) return; const ds=[...document.querySelectorAll('#diceTray .dice')]; ds.forEach(d=>{ const v=1+Math.floor(Math.random()*6); d.textContent=v; const need=+d.dataset.need; d.classList.remove('good','bad'); d.classList.add(v>=need?'good':'bad'); }); const vals=ds.map(d=>parseInt(d.textContent,10)||d.dataset.need); send('save_rolls', {rolls: vals}); btn.disabled=true; $('saveUI').style.display='none'; };
    function logLine(t){ const el=$('log'); const atBottom=el.scrollTop+el.clientHeight>=el.scrollHeight-4; const ts=new Date().toLocaleTimeString(); el.textContent += '['+ts+'] '+t+'\n'; if(atBottom) el.scrollTop=el.scrollHeight; }
    function setStatus(t){ $('status').textContent=t; }
		$('btn-ai').onclick=()=>{ connect(true); }; $('btn-pvp').onclick=()=>{ connect(false); }; $('faction').onchange=loadUnits; $('btn-ready').onclick=()=>{ choose(); ready(); }; $('btn-attack').onclick=attack; $('btn-rematch').onclick=()=>{ location.reload(); }; $('btn-back').onclick=()=>{ location.reload(); }; loadFactions();
  </script>
</body>
</html>`
