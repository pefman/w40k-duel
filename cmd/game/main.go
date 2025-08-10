package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	dataAPIBase = getenv("DATA_API_BASE", "http://localhost:8080")
}

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
	Type     string `json:"type"`
	Desc     string `json:"description"`
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
	Inv  string `json:"inv_sv"`
	InvD string `json:"inv_sv_descr"`
	W    string `json:"W"`
}

type apiKeyword struct {
	Keyword string `json:"keyword"`
	Model   string `json:"model"`
}

type apiAbility struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
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
		inv, invDescr := 0, ""
		if len(models) > 0 {
			W = mustAtoi(models[0].W, 10)
			T = mustAtoi(models[0].T, 4)
			Sv = parseSave(models[0].Sv)
			inv = parseSave(models[0].Inv)
			invDescr = strings.TrimSpace(models[0].InvD)
		}
		var apiW []apiWeapon
		if err := apiGet("/api/"+slug+"/"+u.ID+"/weapons", &apiW); err != nil {
			apiW = nil
		}
		// keywords and abilities
		var apiK []apiKeyword
		_ = apiGet("/api/"+slug+"/"+u.ID+"/keywords", &apiK)
		var apiA []apiAbility
		_ = apiGet("/api/"+slug+"/"+u.ID+"/abilities", &apiA)
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
		// derive unit keywords list (non-empty)
		keywords := make([]string, 0, len(apiK))
		for _, k := range apiK {
			s := strings.TrimSpace(k.Keyword)
			if s != "" {
				keywords = append(keywords, s)
			}
		}
		fnp, dr := parseFNPAndDR(apiA)
		weps := make([]Weapon, 0, len(apiW))
		for _, w := range apiW {
			weps = append(weps, deriveWeaponRules(w))
		}
		// If no weapons found, add a generic one
		if len(weps) == 0 {
			weps = []Weapon{{Name: "Generic", Range: "24", Attacks: 2, BS: 4, S: T, AP: 0, D: 1}}
		}
		out = append(out, Unit{Faction: factionName, Name: u.Name, W: W, T: T, Sv: Sv, InvSv: inv, InvSvDescr: invDescr, Keywords: keywords, FNP: fnp, DamageRed: dr, Weapons: weps, DefaultW: weps[0].Name, Options: opts, Points: pts})
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

// ---------- Parsing helpers for rules ----------

func deriveWeaponRules(w apiWeapon) Weapon {
	base := Weapon{
		Name:        w.Name,
		Range:       w.Range,
		Attacks:     parseAttacks(w.Attacks),
		AttacksExpr: strings.TrimSpace(w.Attacks),
		BS:          parseSave(w.BSOrWS),
		S:           mustAtoi(w.Strength, 4),
		AP:          parseAP(w.AP),
		D:           mustAtoi(w.Damage, 1),
	}
	blob := strings.ToLower(w.Type + " " + w.Desc)
	tags := []string{}
	if strings.Contains(blob, "lethal hits") {
		base.LethalHits = true
		tags = append(tags, "Lethal Hits")
	}
	if strings.Contains(blob, "twin-linked") {
		base.TwinLinked = true
		tags = append(tags, "Twin-linked")
	}
	if strings.Contains(blob, "torrent") {
		base.Torrent = true
		tags = append(tags, "Torrent")
	}
	if strings.Contains(blob, "devastating wounds") {
		base.DevastatingWounds = true
		tags = append(tags, "Devastating Wounds")
	}
	// Sustained Hits X
	if idx := strings.Index(blob, "sustained hits"); idx >= 0 {
		sub := strings.TrimSpace(blob[idx+len("sustained hits"):])
		n := mustAtoi(sub, 0)
		if n <= 0 { // try format like "sustained hits 1"
			// look ahead for first digit
			for _, r := range sub {
				if r >= '0' && r <= '9' {
					n = int(r - '0')
					break
				}
			}
		}
		if n > 0 {
			base.SustainedHits = n
			tags = append(tags, fmt.Sprintf("Sustained Hits %d", n))
		}
	}
	// Anti-[X] (n+)
	if idx := strings.Index(blob, "anti-"); idx >= 0 {
		sub := blob[idx+len("anti-"):]
		// capture tag until space or '(' or end
		tag := strings.TrimSpace(sub)
		// trim at '('
		if p := strings.IndexAny(tag, " (\n\t,"); p >= 0 {
			tag = strings.TrimSpace(tag[:p])
		}
		// find threshold like (4+) or 4+
		n := 0
		if p := strings.Index(sub, "("); p >= 0 {
			inside := sub[p+1:]
			n = mustAtoi(inside, 0)
		} else {
			n = mustAtoi(sub, 0)
		}
		if tag != "" && n >= 2 && n <= 6 {
			base.AntiTag = strings.ToLower(tag)
			base.AntiValue = n
			tags = append(tags, fmt.Sprintf("Anti-%s (%d+)", tag, n))
		}
	}
	base.Tags = tags
	return base
}

func parseFNPAndDR(abs []apiAbility) (fnp int, dr int) {
	fnp, dr = 0, 0
	for _, a := range abs {
		text := strings.ToLower(a.Description + " " + a.Name)
		// FNP
		if strings.Contains(text, "feel no pain") || strings.Contains(text, "fnp") {
			// find number before '+'
			n := 0
			// scan runes
			for i := 0; i < len(text); i++ {
				if text[i] >= '2' && text[i] <= '6' {
					// lookahead for '+'
					if i+1 < len(text) && text[i+1] == '+' {
						n = int(text[i] - '0')
						break
					}
				}
			}
			if n >= 2 && n <= 6 {
				if fnp == 0 || n < fnp {
					fnp = n
				}
			}
		}
		// Damage Reduction patterns
		if strings.Contains(text, "reduce damage by") || strings.Contains(text, "damage reduction") || strings.Contains(text, "-1 damage") {
			// try to parse a number near
			n := 1
			if idx := strings.Index(text, "reduce damage by"); idx >= 0 {
				sub := text[idx+len("reduce damage by"):]
				n = mustAtoi(sub, 1)
			} else if idx := strings.Index(text, "damage reduction"); idx >= 0 {
				sub := text[idx+len("damage reduction"):]
				n = mustAtoi(sub, 1)
			} else if strings.Contains(text, "-1 damage") {
				n = 1
			}
			if n < 0 {
				n = -n
			}
			if n > dr {
				dr = n
			}
		}
	}
	if dr < 0 {
		dr = 0
	}
	return
}

// (reserved) helper for future keyword checks

// ========================= Matchmaking & Rooms =========================

type Player struct {
	ID      string
	Conn    *websocket.Conn
	Name    string
	IsAI    bool // true only for bot players we create server-side
	WantsAI bool // true if this human asked to play vs AI
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
	// Lobby of waiting players (not yet matched). Entries removed on match or disconnect.
	lobbyMu sync.Mutex
	lobby   = map[string]LobbyEntry{}
)

// Lightweight lobby entry exposed via /lobby
type LobbyEntry struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Faction string   `json:"faction"`
	Unit    string   `json:"unit"`
	Weapons []string `json:"weapons,omitempty"`
	Points  int      `json:"points,omitempty"`
	Locked  bool     `json:"locked"`
	WantsAI bool     `json:"wantsAI"`
	Queued  bool     `json:"queued"`
	Since   int64    `json:"since"` // unix seconds
}

// ========================= Web =========================

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

// ========================= Daily Stats (in-memory) =========================

type DailyTopDamage struct {
	Damage          int    `json:"damage"`
	Attacker        string `json:"attacker"`
	AttackerFaction string `json:"attacker_faction,omitempty"`
	AttackerUnit    string `json:"attacker_unit,omitempty"`
	Defender        string `json:"defender,omitempty"`
	Weapon          string `json:"weapon,omitempty"`
	Time            int64  `json:"time"`
}

type DailyWorstSave struct {
	Roll            int    `json:"roll"`
	Need            int    `json:"need"`
	Defender        string `json:"defender"`
	DefenderFaction string `json:"defender_faction,omitempty"`
	DefenderUnit    string `json:"defender_unit,omitempty"`
	Count           int    `json:"count"`
	Time            int64  `json:"time"`
}

type DailyStats struct {
	Date      string         `json:"date"`
	TopDamage DailyTopDamage `json:"top_damage"`
	WorstSave DailyWorstSave `json:"worst_save"`
}

var (
	dailyMu    sync.Mutex
	dailyState = DailyStats{Date: time.Now().Format("2006-01-02"), TopDamage: DailyTopDamage{Damage: 0}, WorstSave: DailyWorstSave{Roll: 7}}
)

func dailyStatsGet() DailyStats {
	dailyMu.Lock()
	defer dailyMu.Unlock()
	today := time.Now().Format("2006-01-02")
	if dailyState.Date != today {
		dailyState = DailyStats{Date: today, TopDamage: DailyTopDamage{Damage: 0}, WorstSave: DailyWorstSave{Roll: 7}}
	}
	return dailyState
}

func dailyStatsMaybeTopDamage(dmg int, attacker, aFac, aUnit, defender, weapon string) {
	if dmg <= 0 {
		return
	}
	dailyMu.Lock()
	defer dailyMu.Unlock()
	today := time.Now().Format("2006-01-02")
	if dailyState.Date != today {
		dailyState = DailyStats{Date: today, TopDamage: DailyTopDamage{Damage: 0}, WorstSave: DailyWorstSave{Roll: 7}}
	}
	if dmg > dailyState.TopDamage.Damage {
		dailyState.TopDamage = DailyTopDamage{Damage: dmg, Attacker: attacker, AttackerFaction: aFac, AttackerUnit: aUnit, Defender: defender, Weapon: weapon, Time: time.Now().Unix()}
	}
}

func dailyStatsMaybeWorstSave(minRoll int, need int, defender, dFac, dUnit string, count int) {
	if minRoll <= 0 || need <= 0 {
		return
	}
	dailyMu.Lock()
	defer dailyMu.Unlock()
	today := time.Now().Format("2006-01-02")
	if dailyState.Date != today {
		dailyState = DailyStats{Date: today, TopDamage: DailyTopDamage{Damage: 0}, WorstSave: DailyWorstSave{Roll: 7}}
	}
	if minRoll < dailyState.WorstSave.Roll {
		dailyState.WorstSave = DailyWorstSave{Roll: minRoll, Need: need, Defender: defender, DefenderFaction: dFac, DefenderUnit: dUnit, Count: count, Time: time.Now().Unix()}
	}
}

func main() {
	http.HandleFunc("/", serveIndex)
	http.HandleFunc("/ws", handleWS)
	http.HandleFunc("/api/factions", handleFactions)
	http.HandleFunc("/api/units", handleUnits)
	http.HandleFunc("/lobby", handleLobby)
	http.HandleFunc("/leaderboard", handleLeaderboard)
	http.HandleFunc("/leaderboard/daily", handleLeaderboardDaily)
	http.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"version": buildVersion,
			"time":    buildTime,
		})
	})
	http.HandleFunc("/debug", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(io.LimitReader(r.Body, 4096))
		msg := strings.TrimSpace(string(b))
		if msg == "" {
			msg = "(empty client debug)"
		}
		log.Printf("client-debug: %s", msg)
		w.WriteHeader(http.StatusNoContent)
	})
	http.HandleFunc("/debug/rooms", handleDebugRooms)

	go matchmaker()

	log.Printf("go40k duel game listening on %s (DATA_API_BASE=%s)", gameListenAddr, dataAPIBase)
	log.Fatal(http.ListenAndServe(gameListenAddr, nil))
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	html := strings.ReplaceAll(indexHTML, "{{BUILD_VERSION}}", buildVersion)
	fmt.Fprint(w, html)
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

// Public lobby endpoint: list currently waiting players and their locked selections
func handleLobby(w http.ResponseWriter, r *http.Request) {
	// Unified lobby: include waiting players and players in rooms (humans only)
	type outEntry struct {
		ID       string   `json:"id"`
		Name     string   `json:"name"`
		Faction  string   `json:"faction,omitempty"`
		Unit     string   `json:"unit,omitempty"`
		Weapons  []string `json:"weapons,omitempty"`
		Points   int      `json:"points,omitempty"`
		Locked   bool     `json:"locked,omitempty"`
		WantsAI  bool     `json:"wantsAI,omitempty"`
		Since    int64    `json:"since,omitempty"`
		Status   string   `json:"status"`             // lobby | in-game | finished
		Phase    string   `json:"phase,omitempty"`    // Lobby | Attack | Save | Waiting | Finished
		Opponent string   `json:"opponent,omitempty"` // opponent name if in game
	}
	out := make([]outEntry, 0, 32)
	// 1) Waiting lobby entries
	lobbyMu.Lock()
	for _, v := range lobby {
		phase := "Lobby"
		if v.Queued {
			phase = "Looking for match..."
		} else if !v.Locked {
			phase = "Idling"
		}
		out = append(out, outEntry{
			ID: v.ID, Name: v.Name, Faction: v.Faction, Unit: v.Unit, Weapons: v.Weapons,
			Points: v.Points, Locked: v.Locked, WantsAI: v.WantsAI, Since: v.Since,
			Status: "lobby", Phase: phase,
		})
	}
	lobbyMu.Unlock()
	// 2) In-game players from rooms (humans only)
	roomsMu.Lock()
	for _, r := range rooms {
		if r == nil {
			continue
		}
		// helper
		add := func(p *Player, opp *Player) {
			if p == nil || p.IsAI {
				return
			}
			status := "in-game"
			phase := "Waiting"
			if r.Finished {
				status = "finished"
				phase = "Finished"
			}
			if !r.Finished {
				if r.Phase == "save" {
					// defender is non-turn player
					def := r.P1
					if def != nil && def.ID == r.Turn {
						def = r.P2
					}
					if def != nil && p.ID == def.ID {
						phase = "Save"
					} else {
						phase = "Awaiting Saves"
					}
				} else if p.ID == r.Turn {
					phase = "Attack"
				} else {
					phase = "Waiting"
				}
			}
			unitName := p.Unit.Name
			if unitName == "" {
				unitName = p.Loadout.Unit
			}
			pts := p.Unit.Points
			oppName := ""
			if opp != nil {
				if opp.IsAI {
					oppName = "AI"
				} else if opp.Name != "" {
					oppName = opp.Name
				} else {
					oppName = "Anon"
				}
			}
			out = append(out, outEntry{
				ID: p.ID, Name: p.Name, Faction: p.Loadout.Faction, Unit: unitName,
				Points: pts, Status: status, Phase: phase, Opponent: oppName,
			})
		}
		add(r.P1, r.P2)
		add(r.P2, r.P1)
	}
	roomsMu.Unlock()
	sort.Slice(out, func(i, j int) bool {
		rank := func(s string) int {
			if s == "in-game" {
				return 0
			} else if s == "lobby" {
				return 1
			} else {
				return 2
			}
		}
		ri, rj := rank(out[i].Status), rank(out[j].Status)
		if ri != rj {
			return ri < rj
		}
		if out[i].Since != out[j].Since {
			return out[i].Since < out[j].Since
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"players": out, "count": len(out)})
}

// Public leaderboard endpoint: list online humans (in lobby or in-game) with unit, points, and phase
func handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	type lbEntry struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Status  string `json:"status"` // lobby | in-game | finished
		Phase   string `json:"phase"`  // Lobby | Attack | Save | Waiting | Finished
		Faction string `json:"faction,omitempty"`
		Unit    string `json:"unit,omitempty"`
		Points  int    `json:"points,omitempty"`
		Since   int64  `json:"since,omitempty"`
	}
	out := make([]lbEntry, 0, 16)
	// 1) Lobby entries (humans waiting)
	lobbyMu.Lock()
	for _, v := range lobby {
		phase := "Lobby"
		if v.Queued {
			phase = "Looking for match..."
		} else if !v.Locked {
			phase = "Idling"
		}
		out = append(out, lbEntry{
			ID: v.ID, Name: v.Name, Status: "lobby", Phase: phase,
			Faction: v.Faction, Unit: v.Unit, Points: v.Points, Since: v.Since,
		})
	}
	lobbyMu.Unlock()
	// 2) Rooms (both players if human)
	roomsMu.Lock()
	for _, r := range rooms {
		// Helper to add a player if human
		addP := func(p *Player) {
			if p == nil || p.IsAI {
				return
			}
			phase := "Waiting"
			status := "in-game"
			if r.Finished {
				phase = "Finished"
				status = "finished"
			} else if r.Phase == "save" {
				// Defender is the non-turn player
				defender := r.P1
				if defender.ID == r.Turn {
					defender = r.P2
				}
				if defender != nil && p.ID == defender.ID {
					phase = "Save"
				} else {
					phase = "Awaiting Saves"
				}
			} else {
				if p.ID == r.Turn {
					phase = "Attack"
				} else {
					phase = "Waiting"
				}
			}
			unitName := p.Unit.Name
			if unitName == "" {
				unitName = p.Loadout.Unit
			}
			pts := p.Unit.Points
			out = append(out, lbEntry{
				ID: p.ID, Name: p.Name, Status: status, Phase: phase,
				Faction: p.Loadout.Faction, Unit: unitName, Points: pts,
			})
		}
		addP(r.P1)
		addP(r.P2)
	}
	roomsMu.Unlock()
	// Sort by status (in-game first), then since/name
	sort.Slice(out, func(i, j int) bool {
		rank := func(s string) int {
			if s == "in-game" {
				return 0
			} else if s == "lobby" {
				return 1
			} else {
				return 2
			}
		}
		ri, rj := rank(out[i].Status), rank(out[j].Status)
		if ri != rj {
			return ri < rj
		}
		if out[i].Since != out[j].Since {
			return out[i].Since < out[j].Since
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"players": out, "count": len(out)})
}

// Daily leaderboard: top damage in one successful attack and worst single save roll (today)
func handleLeaderboardDaily(w http.ResponseWriter, r *http.Request) {
	s := dailyStatsGet()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s)
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
		log.Printf("matchmaker: got p1 id=%s name=%q wantsAI=%v (queueLen=%d)", p1.ID, p1.Name, p1.WantsAI, len(matchQueue))
		select {
		case p2 := <-matchQueue:
			log.Printf("matchmaker: pairing p1=%s (%s) with p2=%s (%s)", p1.ID, p1.Name, p2.ID, p2.Name)
			createRoom(p1, p2)
		case <-time.After(1200 * time.Millisecond):
			log.Printf("matchmaker: timeout waiting for p2 (p1.wantsAI=%v)", p1.WantsAI)
			if p1.WantsAI {
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
	// Remove both from lobby now that they are matched
	lobbyDelete(p1.ID)
	lobbyDelete(p2.ID)
	go roomLoop(room)
}

// Create a simple AI opponent with a random (or fallback) unit
func makeAIPlayer() *Player {
	ai := &Player{ID: fmt.Sprintf("ai_%d", time.Now().UnixNano()), Name: "AI Opponent", IsAI: true}
	facs, _ := FetchFactions()
	fName := "Necrons"
	if len(facs) > 0 {
		fName = facs[rand.Intn(len(facs))].Name
	}
	// initial pick
	us, err := FetchUnits(fName)
	var u Unit
	if err == nil && len(us) > 0 {
		u = us[rand.Intn(len(us))]
	} else {
		u = Unit{Name: "Generic Squad", W: 10, T: 4, Sv: 4, Weapons: []Weapon{{Name: "Bolter", Range: "24", Attacks: 2, BS: 4, S: 4, AP: 0, D: 1}}}
	}
	// If there is a current waiting player with a locked loadout, try to match points
	// Heuristic: scan the queue head if exists and try same faction else any faction closest points
	targetPts := u.Points
	// Note: matchQueue is a channel; we won't peek. Instead, we try to get closest in current faction list.
	if err == nil && len(us) > 0 {
		best := u
		bestDiff := abs(best.Points - targetPts)
		for _, cand := range us {
			d := abs(cand.Points - targetPts)
			if d < bestDiff {
				best, bestDiff = cand, d
			}
		}
		u = best
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
			// Broadcast current state so clients can see readiness
			broadcastGameState(r)
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
	broadcast(wsMsg{Type: "log", Data: fmt.Sprintf("It is now %s's turn", first.Name)})
	broadcastGameState(r)
	// If AI goes first, schedule its opening attack automatically
	scheduleAIAttack(r, 1500)
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
			"InvSv":   p.Unit.InvSv,
			"FNP":     p.Unit.FNP,
			"DR":      p.Unit.DamageRed,
			"Points":  p.Unit.Points,
			"weapons": p.Unit.Weapons,
		},
		"loadout": p.Loadout,
	}
}

// ----- Lobby helpers -----
func lobbySet(p *Player, locked bool) {
	if p == nil {
		return
	}
	// Preserve existing queued flag and since, if present
	lobbyMu.Lock()
	prev, hasPrev := lobby[p.ID]
	lobbyMu.Unlock()
	e := LobbyEntry{
		ID:      p.ID,
		Name:    p.Name,
		Faction: p.Loadout.Faction,
		Unit:    p.Loadout.Unit,
		Weapons: append([]string(nil), p.Loadout.Weapons...),
		Points:  p.Unit.Points,
		Locked:  locked,
		WantsAI: p.WantsAI,
		Queued:  prev.Queued,
		Since: func() int64 {
			if hasPrev && prev.Since != 0 {
				return prev.Since
			}
			return time.Now().Unix()
		}(),
	}
	lobbyMu.Lock()
	lobby[p.ID] = e
	lobbyMu.Unlock()
}

// Mark a lobby entry as queued or not, preserving other fields
func lobbyMarkQueued(playerID string, queued bool) {
	lobbyMu.Lock()
	e, ok := lobby[playerID]
	if ok {
		e.Queued = queued
		lobby[playerID] = e
	}
	lobbyMu.Unlock()
}

func lobbyDelete(playerID string) {
	lobbyMu.Lock()
	delete(lobby, playerID)
	lobbyMu.Unlock()
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
	player := &Player{ID: fmt.Sprintf("p_%d", time.Now().UnixNano()), Conn: conn, Name: name, IsAI: false, WantsAI: wantAI}
	log.Printf("ws: connect id=%s name=%q ai=%v from=%s", player.ID, name, wantAI, r.RemoteAddr)
	// Tell the client its own player ID
	_ = player.Conn.WriteJSON(wsMsg{Type: "you", Data: map[string]string{"id": player.ID}})
	// Seed an initial lobby entry
	lobbySet(player, false)
	go wsReader(player)
	// Do not enqueue immediately; wait for an explicit 'queue' request from the client.
	log.Printf("ws: presence only (not queued) id=%s", player.ID)
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
			p.Conn = nil
		}
		log.Printf("ws: closed id=%s name=%q", p.ID, p.Name)
		// On disconnect, clear queued flag and remove from lobby
		lobbyMarkQueued(p.ID, false)
		lobbyDelete(p.ID)
	}()
	for {
		var in clientIn
		if err := p.Conn.ReadJSON(&in); err != nil {
			log.Printf("ws: read error id=%s: %v", p.ID, err)
			return
		}
		log.Printf("ws: recv id=%s type=%s", p.ID, in.Type)
		roomIDAny, ok := playersIndex.Load(p.ID)
		r := (*Room)(nil)
		if ok {
			r = getRoom(roomIDAny.(string))
			if r == nil {
				log.Printf("ws: room not found for id=%s room=%v", p.ID, roomIDAny)
			}
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
			log.Printf("choose: %s chose faction=%q unit=%q weapons=%v (active=%q)", p.ID, sel.Faction, sel.Unit, sel.Weapons, sel.Weapon)
			// Update lobby immediately
			lobbySet(p, p.Ready)
			if r != nil {
				sendTo(p, wsMsg{Type: "log", Data: fmt.Sprintf("Selected %s / %s (%s)", sel.Faction, sel.Unit, sel.Weapon)})
				broadcastGameState(r)
			}
		case "set_weapon":
			var body struct {
				Weapon string `json:"weapon"`
			}
			_ = json.Unmarshal(in.Data, &body)
			if body.Weapon != "" {
				p.Loadout.Weapon = body.Weapon
				log.Printf("set_weapon: %s active=%q", p.ID, body.Weapon)
				lobbySet(p, p.Ready)
				if r != nil {
					sendTo(p, wsMsg{Type: "log", Data: fmt.Sprintf("Active weapon: %s", body.Weapon)})
					broadcastGameState(r)
				}
			}
		case "ready":
			p.Ready = true
			log.Printf("ready: %s is READY", p.ID)
			lobbySet(p, true)
			// Always confirm to the client
			sendTo(p, wsMsg{Type: "log", Data: "Ready! Waiting for opponent..."})
		case "queue":
			// Client explicitly requests matchmaking; optional payload: { ai: bool }
			var body struct {
				AI bool `json:"ai"`
			}
			_ = json.Unmarshal(in.Data, &body)
			p.WantsAI = body.AI
			log.Printf("queue: %s wantsAI=%v", p.ID, p.WantsAI)
			// Update lobby (locked status remains as previously set by 'ready')
			lobbySet(p, p.Ready)
			lobbyMarkQueued(p.ID, true)
			// Enqueue now
			matchQueue <- p
			log.Printf("ws: enqueued player id=%s (queueLen=%d)", p.ID, len(matchQueue))
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
			totalDmg := unsaved * max(1, dmgPer-defender.Unit.DamageRed)
			if defender.Unit.FNP >= 2 && totalDmg > 0 {
				ignored := 0
				for i := 0; i < totalDmg; i++ {
					if d6() >= defender.Unit.FNP {
						ignored++
					}
				}
				if ignored > 0 {
					logLines = append(logLines, fmt.Sprintf("Feel No Pain %d+: ignored %d damage", defender.Unit.FNP, ignored))
					totalDmg = max(0, totalDmg-ignored)
				}
			}
			// Daily stats: worst save roll and top damage
			minRoll := 7
			for _, rv := range rolls {
				if rv < minRoll {
					minRoll = rv
				}
			}
			dailyStatsMaybeWorstSave(minRoll, need, defender.Name, defender.Loadout.Faction, defender.Unit.Name, len(rolls))
			attP := r.P1
			if attP.ID != r.Turn {
				attP = r.P2
			}
			dailyStatsMaybeTopDamage(totalDmg, attP.Name, attP.Loadout.Faction, attP.Unit.Name, defender.Name, r.CurrentWeapon)
			before := defender.Wounds
			defender.Wounds = max(0, defender.Wounds-totalDmg)
			logLines = append(logLines, fmt.Sprintf("%d unsaved → %d damage. %s Wounds: %d → %d", unsaved, totalDmg, defender.Name, before, defender.Wounds))
			if defender.Wounds <= 0 {
				r.Finished = true
				r.Winner = r.Turn
				logLines = append(logLines, fmt.Sprintf("%s / %s destroyed!", defender.Loadout.Faction, defender.Unit.Name))
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
				// If now AI's turn, schedule its attack
				scheduleAIAttack(r, 1500)
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
	// Prevent starting a new sequence while saves are pending or an existing sequence is mid-flight
	if r.Phase == "save" || (r.AttackQueue != nil && r.AttackIndex < len(r.AttackQueue)) {
		log.Printf("room %s: attack ignored — phase=%s, hasQueue=%v idx=%d len=%d", r.ID, r.Phase, r.AttackQueue != nil, r.AttackIndex, len(r.AttackQueue))
		r.Mu.Unlock()
		sendTo(attacker, wsMsg{Type: "log", Data: "Attack already in progress — wait for saves to resolve."})
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
	// Diagnostic: log full planned sequence
	log.Printf("room %s: start sequence by %s — %d weapon(s): %v", r.ID, attacker.Name, len(queue), queue)
	r.AttackQueue = append([]string(nil), queue...)
	r.AttackIndex = 0
	r.CurrentWeapon = r.AttackQueue[0]
	cur := r.CurrentWeapon
	r.Mu.Unlock()
	// Inform players about the sequence length (brief)
	sendTo(r.P1, wsMsg{Type: "log", Data: fmt.Sprintf("%s begins attack sequence (%d weapons)", attacker.Name, len(queue))})
	sendTo(r.P2, wsMsg{Type: "log", Data: fmt.Sprintf("%s begins attack sequence (%d weapons)", attacker.Name, len(queue))})
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
	stepInfo := fmt.Sprintf("(step %d/%d)", r.AttackIndex+1, max(1, len(r.AttackQueue)))
	log.Printf("room %s: %s %s using %q", r.ID, attacker.Name, stepInfo, wep.Name)
	logLines := []string{fmt.Sprintf("%s attacks with %s", attacker.Name, wep.Name)}
	// Hits: apply Torrent auto-hits and Sustained Hits
	hitNeed := clamp(2, 6, wep.BS)
	// Determine number of attacks: prefer dice expression if available (e.g., 4D6)
	attacks := 0
	if wep.AttacksExpr != "" && (strings.Contains(wep.AttacksExpr, "D") || strings.Contains(wep.AttacksExpr, "d")) {
		// Roll detailed dice and broadcast an 'attacks' phase before hit/wound/save
		dice, bonus, total := rollDiceExprDetails(wep.AttacksExpr)
		if len(dice) > 0 || bonus != 0 {
			// animate attacks dice
			sendTo(r.P1, wsMsg{Type: "rolls", Data: map[string]any{"phase": "attacks", "expr": strings.ToUpper(strings.TrimSpace(wep.AttacksExpr)), "rolls": dice, "bonus": bonus, "weapon": wep.Name, "attacker": attacker.Name}})
			sendTo(r.P2, wsMsg{Type: "rolls", Data: map[string]any{"phase": "attacks", "expr": strings.ToUpper(strings.TrimSpace(wep.AttacksExpr)), "rolls": dice, "bonus": bonus, "weapon": wep.Name, "attacker": attacker.Name}})
			time.Sleep(1600 * time.Millisecond)
		}
		attacks = max(1, total)
		// Detailed log: show rolled dice, bonus, and total
		expr := strings.ToUpper(strings.TrimSpace(wep.AttacksExpr))
		detail := ""
		if len(dice) == 1 {
			detail = fmt.Sprintf("roll %d", dice[0])
		} else if len(dice) > 1 {
			detail = fmt.Sprintf("rolls %v", dice)
		}
		if bonus != 0 {
			if detail == "" {
				detail = fmt.Sprintf("+ %d", bonus)
			} else {
				detail = fmt.Sprintf("%s + %d", detail, bonus)
			}
		}
		if detail == "" {
			logLines = append(logLines, fmt.Sprintf("Attacks: %s = %d", expr, attacks))
		} else {
			logLines = append(logLines, fmt.Sprintf("Attacks: %s — %s = %d", expr, detail, attacks))
		}
	} else {
		attacks = max(1, wep.Attacks)
	}
	hitRolls := make([]int, 0, attacks)
	hits := 0
	if wep.Torrent {
		hits = attacks
		// fabricate 6s for animation to look good
		for i := 0; i < attacks; i++ {
			hitRolls = append(hitRolls, 6)
		}
	} else {
		for i := 0; i < attacks; i++ {
			roll := d6()
			hitRolls = append(hitRolls, roll)
			if roll >= hitNeed {
				hits++
				// Sustained Hits X: crits (6s) generate extra hits
				if wep.SustainedHits > 0 && roll == 6 {
					hits += wep.SustainedHits
				}
			}
		}
	}
	r.Mu.Unlock()
	// animate hits
	sendTo(r.P1, wsMsg{Type: "rolls", Data: map[string]any{"phase": "hit", "need": hitNeed, "rolls": hitRolls, "weapon": wep.Name, "attacker": attacker.Name}})
	sendTo(r.P2, wsMsg{Type: "rolls", Data: map[string]any{"phase": "hit", "need": hitNeed, "rolls": hitRolls, "weapon": wep.Name, "attacker": attacker.Name}})
	time.Sleep(2800 * time.Millisecond)
	r.Mu.Lock()
	for i, roll := range hitRolls {
		logLines = append(logLines, fmt.Sprintf("Hit %d: rolled %d vs %d+ → %s", i+1, roll, hitNeed, tern(roll >= hitNeed, "HIT", "MISS")))
	}
	// Wounds: Lethal Hits (crits to auto-wound), Anti-[X] n+, Twin-linked (re-roll failed wounds)
	woundTarget := woundNeeded(wep.S, defender.Unit.T)
	// apply Anti threshold if any (we don't check tag matching yet; treat as generic anti for simplicity)
	if wep.AntiValue >= 2 {
		woundTarget = min(woundTarget, wep.AntiValue)
	}
	woundRolls := make([]int, 0, hits)
	wounds := 0
	autoWounds := 0
	mortals := 0
	mortalTriggers := 0
	for i := 0; i < hits; i++ {
		roll := d6()
		if wep.LethalHits && roll == 6 {
			autoWounds++
			woundRolls = append(woundRolls, roll)
			continue
		}
		// attempt wound roll, with twin-linked re-roll if failed
		pass := roll >= woundTarget
		if !pass && wep.TwinLinked {
			roll2 := d6()
			// record both? keep the final for animation clarity
			roll = roll2
			pass = roll2 >= woundTarget
		}
		woundRolls = append(woundRolls, roll)
		if pass {
			if wep.DevastatingWounds && roll == 6 {
				// convert to mortal damage equal to D; bypass saves
				mortals += max(1, wep.D)
				mortalTriggers++
			} else {
				wounds++
			}
		}
	}
	wounds += autoWounds
	r.Mu.Unlock()
	// animate wounds
	sendTo(r.P1, wsMsg{Type: "rolls", Data: map[string]any{"phase": "wound", "need": woundTarget, "rolls": woundRolls, "weapon": wep.Name, "attacker": attacker.Name}})
	sendTo(r.P2, wsMsg{Type: "rolls", Data: map[string]any{"phase": "wound", "need": woundTarget, "rolls": woundRolls, "weapon": wep.Name, "attacker": attacker.Name}})
	time.Sleep(2800 * time.Millisecond)
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
			log.Printf("room %s: continue sequence → next weapon %q (idx=%d/%d)", r.ID, nextWeapon, r.AttackIndex+1, len(r.AttackQueue))
			resolveWeaponStep(attacker, r, nextWeapon)
		} else {
			broadcastGameState(r)
			// If next turn is AI, schedule its attack
			scheduleAIAttack(r, 1500)
		}
		return
	}
	// Saves: choose best of armour vs invuln; AP increases armour save number only
	armourNeed := clamp(2, 6, defender.Unit.Sv+wep.AP)
	saveNeed := armourNeed
	if defender.Unit.InvSv > 0 {
		saveNeed = min(saveNeed, defender.Unit.InvSv)
	}
	// Apply mortal damage immediately (bypass saves); Feel No Pain may apply
	if mortals > 0 {
		before := defender.Wounds
		dmg := mortals
		// optional: do not apply Damage Reduction to mortals; only FNP
		if defender.Unit.FNP >= 2 {
			ignored := 0
			for i := 0; i < dmg; i++ {
				if d6() >= defender.Unit.FNP {
					ignored++
				}
			}
			if ignored > 0 {
				logLines = append(logLines, fmt.Sprintf("Feel No Pain %d+: ignored %d mortal damage", defender.Unit.FNP, ignored))
				dmg = max(0, dmg-ignored)
			}
		}
		defender.Wounds = max(0, defender.Wounds-dmg)
		logLines = append(logLines, fmt.Sprintf("Devastating Wounds: %d mortal damage inflicted (no saves). %s Wounds: %d → %d", mortals, defender.Name, before, defender.Wounds))
		if defender.Wounds <= 0 {
			r.Finished = true
			r.Winner = attacker.ID
		}
	}
	r.Phase = "save"
	r.PendingSaves = wounds
	r.PendingNeed = saveNeed
	r.PendingDmg = max(1, wep.D)
	r.PendingBy = attacker.ID
	r.Mu.Unlock()
	// Inform about pending saves
	logLines = append(logLines, fmt.Sprintf("%d potential wounds → defender to roll %d saves (need %d+)", wounds, wounds, saveNeed))
	sendTo(r.P1, wsMsg{Type: "log_multi", Data: logLines})
	sendTo(r.P2, wsMsg{Type: "log_multi", Data: logLines})
	broadcastGameState(r)
	// If defender is AI, roll saves automatically after a short delay with animation
	if defender.IsAI && wounds > 0 {
		go func() {
			time.Sleep(1200 * time.Millisecond)
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
			time.Sleep(2800 * time.Millisecond)
			b, _ := json.Marshal(map[string]any{"rolls": rolls})
			wsReaderHandleSave(defender, b)
		}()
	}
}

// helper to process save rolls path for AI without duplicating logic
// wsReaderHandleSave removed; saves are handled via the regular ws 'save_rolls' path only.

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
func min(a, b int) int {
	if a < b {
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

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// rollDiceCount parses expressions like "4D6", "D6", "D3+3", "2D6+1" and returns a rolled total.
// Supported forms: [N]D[M][+K] where N, M, K are integers; N defaults to 1 when omitted (e.g., "D6").
// If the expression is invalid, falls back to parseAttacks' numeric/average behavior.
// (rollDiceCount removed; use rollDiceExprDetails for full info)

// rollDiceExprDetails returns individual dice rolls, bonus, and total for an NdM(+K) expression.
// Supports D6, D3, generic M, and +K bonus. If invalid, returns nil,0,0.
func rollDiceExprDetails(expr string) ([]int, int, int) {
	s := strings.TrimSpace(strings.ToUpper(expr))
	if s == "" {
		return nil, 0, 0
	}
	parts := strings.SplitN(s, "D", 2)
	if len(parts) != 2 {
		return nil, 0, 0
	}
	nStr := strings.TrimSpace(parts[0])
	rest := strings.TrimSpace(parts[1])
	bonus := 0
	if plus := strings.Index(rest, "+"); plus >= 0 {
		bonus = mustAtoi(rest[plus+1:], 0)
		rest = strings.TrimSpace(rest[:plus])
	}
	m := mustAtoi(rest, 6)
	n := 1
	if nStr != "" {
		n = mustAtoi(nStr, 1)
	}
	if n <= 0 || m <= 0 {
		return nil, 0, 0
	}
	dice := make([]int, 0, n)
	total := 0
	for i := 0; i < n; i++ {
		var v int
		switch m {
		case 3:
			v = (d6() + 1) / 2
		case 6:
			v = d6()
		default:
			v = rand.Intn(m) + 1
		}
		dice = append(dice, v)
		total += v
	}
	total += bonus
	if total < 1 {
		total = 1
	}
	return dice, bonus, total
}

// ========================= AI Helpers =========================

// scheduleAIAttack triggers an AI attack after delayMS if it's the AI's turn and not in save phase.
func scheduleAIAttack(r *Room, delayMS int) {
	go func() {
		time.Sleep(time.Duration(delayMS) * time.Millisecond)
		r.Mu.Lock()
		if r == nil || r.Finished || r.Phase == "save" || r.Turn == "" {
			r.Mu.Unlock()
			return
		}
		var attacker *Player
		if r.P1 != nil && r.P1.ID == r.Turn {
			attacker = r.P1
		} else if r.P2 != nil && r.P2.ID == r.Turn {
			attacker = r.P2
		}
		if attacker == nil || !attacker.IsAI {
			r.Mu.Unlock()
			return
		}
		log.Printf("room %s: scheduling AI attack for %s", r.ID, attacker.Name)
		r.Mu.Unlock()
		// Start sequence outside lock
		roomStartAttackSequence(r, attacker)
	}()
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
	totalDmg := unsaved * max(1, dmgPer-defender.Unit.DamageRed)
	// Feel No Pain step: roll once per damage suffered; each success ignores 1
	if defender.Unit.FNP >= 2 && totalDmg > 0 {
		ignored := 0
		for i := 0; i < totalDmg; i++ {
			if d6() >= defender.Unit.FNP {
				ignored++
			}
		}
		if ignored > 0 {
			logLines = append(logLines, fmt.Sprintf("Feel No Pain %d+: ignored %d damage", defender.Unit.FNP, ignored))
			totalDmg = max(0, totalDmg-ignored)
		}
	}
	before := defender.Wounds
	defender.Wounds = max(0, defender.Wounds-totalDmg)
	logLines = append(logLines, fmt.Sprintf("%d unsaved → %d damage", unsaved, totalDmg))
	if totalDmg != unsaved*max(1, dmgPer-defender.Unit.DamageRed) && defender.Unit.DamageRed > 0 {
		logLines = append(logLines, fmt.Sprintf("(Damage reduced by armor: -%d per wound)", defender.Unit.DamageRed))
	}
	logLines = append(logLines, fmt.Sprintf("%s Wounds: %d → %d", defender.Name, before, defender.Wounds))
	// Daily stats: record worst save roll and top damage
	minRoll := 7
	for _, rv := range rolls {
		if rv < minRoll {
			minRoll = rv
		}
	}
	dailyStatsMaybeWorstSave(minRoll, need, defender.Name, defender.Loadout.Faction, defender.Unit.Name, len(rolls))
	attP := r.P1
	if attP.ID != r.Turn {
		attP = r.P2
	}
	dailyStatsMaybeTopDamage(totalDmg, attP.Name, attP.Loadout.Faction, attP.Unit.Name, defender.Name, r.CurrentWeapon)
	if defender.Wounds <= 0 {
		r.Finished = true
		r.Winner = r.Turn
		logLines = append(logLines, fmt.Sprintf("%s / %s destroyed!", defender.Loadout.Faction, defender.Unit.Name))
	}
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
		log.Printf("room %s: continue sequence → next weapon %q (idx=%d/%d)", r.ID, nextWeapon, r.AttackIndex+1, len(r.AttackQueue))
		resolveWeaponStep(attacker, r, nextWeapon)
	} else {
		broadcastGameState(r)
		// Schedule AI attack if it's now their turn
		scheduleAIAttack(r, 1500)
	}
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
	.card{background:linear-gradient(180deg, rgba(255,255,255,.02), rgba(0,0,0,.28)); border:1px solid var(--panel-edge); border-radius:14px; padding:16px; box-shadow:var(--shadow);} h2{font-family:'Cinzel', serif; font-size:18px; margin:0 0 10px; color:var(--gold-soft); letter-spacing:.06em}
	#setup{ position: relative; z-index: 99; pointer-events: auto; }
	select, input[type=text]{width:100%; padding:12px 12px; border-radius:10px; border:1px solid #243042; background:#0a0f16; color:var(--text); outline:none; pointer-events:auto}
	select:focus, input[type=text]:focus{border-color:var(--gold); box-shadow:0 0 0 2px rgba(201,167,83,.25)}
	.grid{display:grid; grid-template-columns:1fr 1fr; gap:10px} .row{display:flex; align-items:center; justify-content:space-between; padding:6px 8px; color:#cbd5e1}
    button{cursor:pointer; padding:11px 16px; border-radius:12px; border:1px solid rgba(201,167,83,.45); background:linear-gradient(180deg,#1a2330,#0e141e); color:#f3f4f6; font-weight:700; letter-spacing:.04em; box-shadow: inset 0 1px 0 rgba(255,255,255,.08), 0 6px 16px rgba(0,0,0,.35); transition: transform .05s ease, box-shadow .15s ease, filter .15s ease;} button:hover{filter:brightness(1.05)} button:active{transform:translateY(1px)} button[disabled]{opacity:.5; cursor:not-allowed}
	#btn-attack{background:linear-gradient(180deg, #c9a753, #9b7e37); color:#0a0c10; border-color:#e8d6a6; text-transform:uppercase}
	/* Make Attack/Save buttons prominent and centered */
	#btn-attack, #btn-roll-saves{ display:block; margin:16px auto; padding:16px 22px; font-size:18px; min-width:260px; width:min(520px, 80%); }
	#btn-roll-saves{ background:linear-gradient(180deg, #c9a753, #9b7e37); color:#0a0c10; border-color:#e8d6a6; text-transform:uppercase }
	/* Weapon selection layout + highlight */
	#weaponList label{ display:flex; align-items:center; gap:8px; transition: background .15s ease, border-color .15s ease; padding:8px 10px; border:1px solid #243042; border-radius:10px; margin-bottom:8px; }
	#weaponList label.active{ background: rgba(201,167,83,.08); border-color: var(--gold) }
	#weaponList input[type=checkbox]{ margin-right:8px; }
	/* Dice animation */
	.dice{ width:34px; height:34px; border-radius:6px; background:#111827; border:1px solid #374151; display:inline-flex; align-items:center; justify-content:center; margin:4px; font-weight:800; box-shadow: inset 0 1px 0 rgba(255,255,255,.05); animation: roll .6s ease }
	.dice.good{ border-color:#22c55e; box-shadow:0 0 0 2px rgba(34,197,94,.2) inset }
	.dice.bad{ border-color:#ef4444; box-shadow:0 0 0 2px rgba(239,68,68,.2) inset; color:#ef4444 }
	@keyframes roll { 0%{ transform: rotate(0) scale(.9) } 50%{ transform: rotate(20deg) scale(1.05) } 100%{ transform: rotate(0) scale(1) } }
	.hpbar{height:14px; background:#131922; border-radius:10px; overflow:hidden; border:1px solid #2a3546} .hpfill{height:100%; background:linear-gradient(90deg,#c9a753,#7ed3ee)}
    .log{height:380px; overflow:auto; white-space:pre-wrap; background:#0a0f16; border-radius:12px; padding:12px; border:1px solid #232c3a; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size:13px}
    #p1,#p2{background:linear-gradient(180deg, rgba(255,255,255,.02), rgba(0,0,0,.18)); border:1px solid #263145; border-radius:12px}
	.winner{color:#22c55e; font-weight:800}
	/* Rolls tray */
	#rollsTray{ display:none; margin:8px 0 10px; padding:10px; border:1px solid #243042; border-radius:10px; background:#0b0f16 }
	#rollsHeader{ font-weight:700; color:#e5e7eb; margin-bottom:6px }

	/* Current weapon panel (non-overlay) */
	#currentWeaponPanel{ border:2px solid var(--gold); border-radius:12px; background:linear-gradient(135deg, rgba(201,167,83,.12), rgba(201,167,83,.04)); padding:12px; box-shadow: 0 0 16px rgba(201,167,83,.25); }
	#currentWeaponPanel.att-left{ border-color: rgba(201,167,83,.9); box-shadow: inset 8px 0 0 0 rgba(201,167,83,.4), 0 0 20px rgba(201,167,83,.3); }
	#currentWeaponPanel.att-right{ border-color: rgba(201,167,83,.9); box-shadow: inset -8px 0 0 0 rgba(201,167,83,.4), 0 0 20px rgba(201,167,83,.3); }
	#cwDir{ text-align:center; font-weight:800; letter-spacing:.04em; color:var(--gold-soft); text-shadow: 0 0 8px rgba(201,167,83,.5); }
	.pill.tiny{ font-size:10px; padding:2px 6px; border-radius:999px; }

	/* Layout ordering so player/opponent flank the battlefield */
	#p1Panel{ order: 1; }
	#battlefield{ order: 2; }
	#p2Panel{ order: 3; }

	/* Persistent dice row */
	#dicePersistent{ display:flex; flex-wrap:wrap; gap:6px; justify-content:center; margin:8px 0 0; }
	.dice.removed{ opacity:.25; filter:grayscale(1); }
	.dice.fade{ transition: opacity .25s ease, transform .25s ease; opacity:0; transform: scale(.9); }

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
		<div class="tray"><span id="status" class="pill">Ready</span><span class="pill" title="Build version">v{{BUILD_VERSION}}</span></div>
  </header>
  <main>
		<!-- Sentinel: head script to confirm HTML up to here is parsed -->
		<script>try{ fetch('/debug', {method:'POST', headers:{'Content-Type':'text/plain'}, body: (new Date().toISOString()+" sentinel: pre-main-script")}); }catch(_){}</script>
			<section id="setup" class="card" style="grid-column: 1 / -1;">
				<h2 style="margin-top:0">1) Choose your units</h2>
			<div class="grid">
				<div><label for="faction">Faction</label><select id="faction" aria-describedby="factionHint"></select><div id="factionHint" class="muted" style="margin-top:6px; font-size:12px; color:#9aa4b2"></div></div>
								<div><label for="unit">Unit</label><select id="unit" aria-describedby="unitStats"></select><div id="unitStats" class="muted" style="margin-top:6px; font-size:12px; color:#9aa4b2"></div></div>
			</div>
			<div style="margin-top:8px">
				<fieldset style="border:none; padding:0; margin:0">
					<legend>Weapons (pick same type)</legend>
					<div id="weaponList"></div>
					<div id="optionsHint" class="muted" style="margin-top:6px; font-size:12px; color:#9aa4b2"></div>
					<div class="muted" style="margin-top:6px; font-size:12px; color:#9aa4b2">Tip: if you pick a melee weapon, all selections must be melee; same for ranged.</div>
				</fieldset>
			</div>
						<h2 style="margin-top:12px">2) Ready to rumble?</h2>
						<div class="cta" style="margin-top:12px"><button id="btn-ready" disabled>Lock In</button></div>
							<h2 style="margin-top:12px">3) Matchmake</h2>
							<div class="grid"><button id="btn-ai" disabled>Play vs AI</button><button id="btn-pvp" disabled>Play vs Player</button></div>
    </section>
	<section id="p1Panel" class="card" style="padding:10px; display:none">
	  <h2>Player</h2>
		<div id="p1" style="padding:8px 0">
			<div class="row"><strong id="p1name"></strong><span class="pill" id="p1ai"></span></div>
			<div class="row"><span id="p1unit"></span><span id="p1cfg" class="pill"></span></div>
			<div id="p1meta" style="font-size:12px; color:#9aa4b2; margin:2px 0 6px"> </div>
			<div class="hpbar"><div id="p1hp" class="hpfill" style="width:0%"></div></div>
			<div class="row"><span>Wounds Left:</span><span id="p1wounds">0</span></div>
			<div class="row" style="font-size:11px; color:#9aa4b2"><span>Active Weapon:</span><span id="p1weapon">—</span></div>
		</div>
	</section>

	<section id="battlefield" class="card center" style="display:none">
	<h2>Battlefield</h2>
	<div class="row"><span>Turn:</span> <span id="turn">—</span></div>
	<div id="currentWeaponPanel" style="display:none; margin:8px 0;">
		<div class="row"><strong id="cwTitle">—</strong><span class="pill" id="cwPhase">—</span></div>
		<div class="row"><span id="cwStats">—</span><span class="pill tiny" id="cwSaveStatus" style="display:none"></span></div>
		<div id="cwDir">⇦ ⇦ ⇦</div>
		<div id="cwTags" style="display:flex; gap:6px; flex-wrap:wrap; margin-top:4px"></div>
		<div id="phaseProgress" style="display:flex; gap:4px; margin-top:8px; justify-content:center; font-size:11px;">
			<span class="pill tiny" id="attacksPhase" style="opacity:0.4">Attacks</span>
			<span style="color:#666;">→</span>
			<span class="pill tiny" id="hitPhase" style="opacity:0.4">Hit</span>
			<span style="color:#666;">→</span>
			<span class="pill tiny" id="woundPhase" style="opacity:0.4">Wound</span>
			<span style="color:#666;">→</span>
			<span class="pill tiny" id="savePhase" style="opacity:0.4">Save</span>
			<span style="color:#666;">→</span>
			<span class="pill tiny" id="damagePhase" style="opacity:0.4">Damage</span>
		</div>
	</div>
	<div id="rollsTray"><div id="rollsHeader"></div><div id="rollsDice"></div></div>
	<div id="dicePersistent" style="display:none"></div>
	<div id="versus" class="row"><span>Waiting for opponent...</span></div>
	<button id="btn-attack" disabled>Attack</button>
	<div id="saveUI" style="display:none; margin-top:10px">
		<div class="row"><span>Saves to roll:</span><span id="saveNeed">—</span></div>
		<div id="diceTray" style="margin:8px 0"></div>
		<div class="grid"><button id="btn-roll-saves">Roll Saves</button></div>
	</div>
	<div id="postgame" style="display:none; margin-top:12px" class="grid"><button id="btn-rematch">Rematch</button><button id="btn-back">Back to Setup</button></div>
	<div class="row" style="justify-content:flex-end; margin:6px 0"><button id="btn-clear-log" class="tiny">Clear Log</button></div>
      <div class="log" id="log"></div>
    </section>
	<section id="p2Panel" class="card" style="padding:10px; display:none">
	  <h2>Opponent</h2>
		<div id="p2" style="padding:8px 0">
			<div class="row"><strong id="p2name"></strong><span class="pill" id="p2ai"></span></div>
			<div class="row"><span id="p2unit"></span><span id="p2cfg" class="pill"></span></div>
			<div id="p2meta" style="font-size:12px; color:#9aa4b2; margin:2px 0 6px"> </div>
			<div class="hpbar"><div id="p2hp" class="hpfill" style="width:0%"></div></div>
			<div class="row"><span>Wounds Left:</span><span id="p2wounds">0</span></div>
			<div class="row" style="font-size:11px; color:#9aa4b2"><span>Active Weapon:</span><span id="p2weapon">—</span></div>
		</div>
		<div class="row" style="margin-top:10px"><span>Winner:</span> <span id="winner" class="winner">—</span></div>
	</section>
  </main>
	<!-- Bottom info panels (always visible) -->
	<section id="bottomInfo" style="padding: 0 18px 18px; max-width:1300px; margin:0 auto;">
		<div id="lobbyPanel" class="card" style="margin-top:12px">
			<h2 style="margin:12px 0 6px">Lobby</h2>
			<div id="lobbyList" class="card" style="padding:10px">
				<div class="row"><span>Fetching lobby…</span><span></span></div>
			</div>
		</div>
		<div id="leaderboardPanel" class="card" style="margin-top:12px">
			<h2 style="margin:12px 0 6px">Leaderboard</h2>
			<div id="leaderboard" class="card" style="padding:10px">
				<div class="row"><span>Fetching leaderboard…</span><span></span></div>
			</div>
			<h2 style="margin:12px 0 6px">Daily Records</h2>
			<div id="leaderboardDaily" class="card" style="padding:10px">
				<div class="row"><span>Fetching daily records…</span><span></span></div>
			</div>
		</div>
	</section>
  <script>
		const $ = (id)=>document.getElementById(id);
		let ws; let state={}; let me=null; let chosenWeapons=[]; let weaponType=null; // 'melee' or 'ranged'
		let pendingQueueAI = null; // null => don't queue; true => AI; false => PvP
		let locked=false; let lockedLoadout=null;
	let lobbyTimer=null;
	let lbTimer=null; // leaderboard poll timer (declare early to avoid TDZ)
	let dailyTimer=null; // daily records poll timer
	// Lightweight client->server debug logger
	function dbg(msg){
		try{ fetch('/debug', {method:'POST', headers:{'Content-Type':'text/plain'}, body: (new Date().toISOString()+" "+(msg||''))}); }catch(e){}
	}
	// Global error capture to help trace unexpected syntax/runtime errors
	window.addEventListener('error', (e)=>{
		try{ fetch('/debug', {method:'POST', headers:{'Content-Type':'text/plain'}, body: (new Date().toISOString()+" js-error: "+e.message+" @ "+e.filename+":"+e.lineno+":"+e.colno)}); }catch(_){}
	});
	window.addEventListener('unhandledrejection', (e)=>{
		try{ const msg = (e && e.reason && (e.reason.stack||e.reason.message||String(e.reason))) || 'unhandledrejection'; fetch('/debug', {method:'POST', headers:{'Content-Type':'text/plain'}, body: (new Date().toISOString()+" js-unhandled: "+msg)}); }catch(_){}
	});
		async function loadFactions(){
			try{
				setStatus('Loading factions…');
				dbg('loadFactions: start');
				const fac=$('faction'); 
				if(!fac) { 
					console.error('Faction dropdown not found!'); 
					dbg('loadFactions: faction select not found');
					return; 
				}
				const res=await fetch('/api/factions');
				if(!res.ok) throw new Error('HTTP '+res.status);
				const data=await res.json();
				console.log('Loaded factions:', data?.length || 0, 'factions');
				dbg('loadFactions: fetched '+(data?.length||0)+' factions');
				fac.innerHTML=''; fac.disabled=false; fac.style.pointerEvents='auto';
				// Add a default option
				const defaultOpt = document.createElement('option');
				defaultOpt.value = '';
				defaultOpt.textContent = 'Select a faction...';
				fac.appendChild(defaultOpt);
				// Add faction options
				(data||[]).forEach(f=>{ const o=document.createElement('option'); o.value=f.name||f.factionname; o.textContent=o.value; fac.appendChild(o); });
				console.log('Faction dropdown populated with', fac.children.length, 'options');
				dbg('loadFactions: populated dropdown with '+fac.children.length+' options');
				// Auto-select first faction to unblock flow if user doesn't pick
				if(fac.children.length>1 && !fac.value){ fac.selectedIndex=1; dbg('loadFactions: auto-selected '+fac.value); loadUnits(); }
				setStatus('Ready');
			}catch(err){
				setStatus('Failed to load factions');
				logLine('Error loading factions: '+(err&&err.message?err.message:err));
				console.error('Faction loading error:', err);
				dbg('loadFactions: error '+(err&&err.message?err.message:err));
			}
		}
	async function loadUnits(){
		const readyBtn=$('btn-ready'); if(readyBtn) readyBtn.disabled=true;
		try{
			const f=$('faction').value; 
			dbg('loadUnits: for faction '+JSON.stringify(f));
			if(!f){ 
				setStatus('Pick a faction'); 
				$('unit').innerHTML='<option value="">Select a faction first...</option>';
				$('unitStats').textContent='';
				$('weaponList').innerHTML='';
				return; 
			}
			setStatus('Loading units…');
			const res=await fetch('/api/units?faction='+encodeURIComponent(f));
			if(!res.ok) throw new Error('HTTP '+res.status);
			const units=await res.json();
			console.log('Loaded units for', f+':', units?.length || 0, 'units');
			dbg('loadUnits: got '+(units?.length||0)+' units for '+f);
			const uSel=$('unit'); uSel.innerHTML=''; const unitStats=$('unitStats'); unitStats.textContent='';
			// Add default option
			const defaultOpt = document.createElement('option');
			defaultOpt.value = '';
			defaultOpt.textContent = 'Select a unit...';
			uSel.appendChild(defaultOpt);
			(units||[]).forEach(u=>{ const o=document.createElement('option'); o.value=u.Name||u.name; const pts=(u.Points||u.points||0); const label=(u.Name||u.name)+ (u.W? (' — W:'+u.W+' T:'+u.T) : '') + (pts? (' — '+pts+'pts') : ''); o.textContent=label; uSel.appendChild(o); });
			const first=(units&&units.length?units[0]:null);
			if(first){ unitStats.textContent='W: '+(first.W||first.wounds||'?')+'  T: '+(first.T||'?') + (first.InvSv? ('  Inv: '+first.InvSv+'+') : '') + (first.FNP? ('  FNP: '+first.FNP+'+') : '') + (first.Points? ('  •  '+first.Points+' pts') : ''); }
			$('weaponList').innerHTML=''; // Clear weapons until unit is selected
			uSel.onchange=()=>{ dbg('unit: change to '+uSel.value); const picked=(units||[]).find(x=>(x.Name||x.name)=== (uSel.value.split(' — ')[0]) ); if(!picked) return; unitStats.textContent='W: '+(picked.W||picked.wounds||'?')+'  T: '+(picked.T||'?') + (picked.InvSv? ('  Inv: '+picked.InvSv+'+') : '') + (picked.FNP? ('  FNP: '+picked.FNP+'+') : '') + (picked.Points? ('  •  '+picked.Points+' pts') : ''); loadWeaponsList(picked); };
			if(readyBtn) readyBtn.disabled=false; setStatus('Ready');
		}catch(err){
			setStatus('Failed to load units');
			logLine('Error loading units for faction: '+$('faction').value+' — '+(err&&err.message?err.message:err));
			console.error('Unit loading error:', err);
			dbg('loadUnits: error '+(err&&err.message?err.message:err));
		}
	}
			function loadWeaponsList(unit){
				const box=$('weaponList'); box.innerHTML=''; chosenWeapons=[]; weaponType=null; const optHint=$('optionsHint');
				const list=(unit.Weapons||unit.weapons||[]);
				// Precompute melee/ranged classification
				const isMeleeFn=(w)=>{ const rng=(w.range??w.Range??'').toString().toLowerCase(); return rng==='melee' || rng==='' || rng==='0' || rng==='0"' || rng==='0”'; };
				// Build a set of allowed weapon names from unit.Options, but only enforce disabling when options clearly constrain choices
				const allowed=new Set();
				const opts=(unit.Options||unit.options||[]);
				const norm=(s)=> (s||'').toString().toLowerCase().replace(/[^a-z0-9]/g,' ').replace(/\s+/g,' ').trim();
				const baseOf=(ns)=>{ const i=ns.search(/\s[-–—:]/); if(i>0) return ns.slice(0,i).trim(); return ns; };
				const names=list.map(w=> (w.name||w.Name) );
				const nameNorms=new Map(); names.forEach(n=>{ if(n){ nameNorms.set(n, norm(n)); } });
				let hasRestrictive=false; const restrRe=/(^|\b)(replace|swap|either|or|choose|select|may take|may equip|can be equipped)(\b|\s)/i;
				opts.forEach(line=>{ const sN=norm(line); if(restrRe.test(line)) hasRestrictive=true; nameNorms.forEach((nn, orig)=>{ const b=baseOf(nn); if((nn && sN.includes(nn)) || (b && sN.includes(b))) allowed.add(orig); }); });
				const restrictive = false; // do not disable; options are informative only to avoid hiding valid defaults
				optHint.textContent = opts && opts.length? ('Options: '+opts.slice(0,3).join(' | ')+(opts.length>3?' …':'')) : '';
				list.forEach(w=>{
					const name=w.name||w.Name;
					const line=document.createElement('label'); line.style.display='flex'; line.style.alignItems='center'; line.style.gap='8px'; line.style.position='relative';
					const cb=document.createElement('input'); cb.type='checkbox'; cb.value=name; cb.onchange=()=>{ onWeaponToggle(w, cb.checked); highlightActiveWeapon(); send('set_weapon', {weapon: chosenWeapons[0]||''}); };
					const S = (w.s ?? w.S ?? '?');
					const AP = (w.ap ?? w.AP ?? 0);
					const A = (w.attacks_expr ?? w.AttacksExpr ?? w.attacks ?? w.Attacks ?? '?');
					const BSWS = (w.bs ?? w.BS ?? '?');
					const D = (w.d ?? w.D ?? '?');
					const details = ' (A:'+A+' BS/WS:'+BSWS+'+ S:'+S+' AP:'+AP+' D:'+D+')';
					line.appendChild(cb);
					const wrap=document.createElement('div'); wrap.style.display='flex'; wrap.style.flexDirection='column'; wrap.style.flex='1 1 auto';
					const topRow=document.createElement('div'); topRow.style.display='flex'; topRow.style.justifyContent='space-between'; topRow.style.alignItems='center';
					const nameDiv=document.createElement('div'); nameDiv.style.fontWeight='600';
					// Name + inline ability chips
					nameDiv.appendChild(document.createTextNode(name));
					const tagsInline=(w.tags||w.Tags||[]);
					if(tagsInline && tagsInline.length){
						tagsInline.forEach(t=>{ const chip=document.createElement('span'); chip.className='pill'; chip.style.marginLeft='6px'; chip.style.fontSize='10px'; chip.style.padding='2px 6px'; chip.textContent=t; nameDiv.appendChild(chip); });
					}
					const statsDiv=document.createElement('div'); statsDiv.textContent = details; statsDiv.style.fontSize='12px'; statsDiv.style.color='#9aa4b2';
					topRow.appendChild(nameDiv); topRow.appendChild(statsDiv); wrap.appendChild(topRow);
					line.appendChild(wrap);
					box.appendChild(line);
				});
				// Quick-select buttons
				const qs=document.createElement('div'); qs.style.marginTop='8px'; qs.style.display='flex'; qs.style.gap='8px';
				const bM=document.createElement('button'); bM.textContent='All Melee';
				bM.onclick=()=>{
					const inputs=[...document.querySelectorAll('#weaponList input[type=checkbox]')];
					chosenWeapons=[]; weaponType='melee';
					inputs.forEach((i,idx)=>{ const w=list[idx]; const isM=isMeleeFn(w); i.checked=isM; if(isM){ const nm=w.name||w.Name; if(!chosenWeapons.includes(nm)) chosenWeapons.push(nm); } });
					highlightActiveWeapon(); send('set_weapon', {weapon: chosenWeapons[0]||''});
				};
				const bR=document.createElement('button'); bR.textContent='All Ranged';
				bR.onclick=()=>{
					const inputs=[...document.querySelectorAll('#weaponList input[type=checkbox]')];
					chosenWeapons=[]; weaponType='ranged';
					inputs.forEach((i,idx)=>{ const w=list[idx]; const isM=isMeleeFn(w); i.checked=!isM; if(!isM){ const nm=w.name||w.Name; if(!chosenWeapons.includes(nm)) chosenWeapons.push(nm); } });
					highlightActiveWeapon(); send('set_weapon', {weapon: chosenWeapons[0]||''});
				};
				qs.appendChild(bM); qs.appendChild(bR);
				box.appendChild(qs);
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
		function connect(ai=false){
			if(!locked){ alert('Lock in your unit and weapons first.'); return; }
			const proto=(location.protocol==='https:'?'wss':'ws');
			pendingQueueAI = ai;
			ws=new WebSocket(proto+'://'+location.host+'/ws?ai='+(ai?1:0));
			ws.onopen=()=>{
				setStatus('Connected');
				// Start lobby/leaderboard polling right away so this user shows as idle for others
				startLobby(); startLeaderboard();
				if(lockedLoadout){ send('choose', lockedLoadout); send('ready', {}); }
				// Now that the socket is open, send the queue request reliably
				if(pendingQueueAI!==null){ send('queue', {ai: !!pendingQueueAI}); setStatus('Looking for match...'); }
			};
			ws.onmessage=(ev)=>{ const msg=JSON.parse(ev.data); if(msg.type==='you'){ me=msg.data.id; } if(msg.type==='state') onState(msg.data); if(msg.type==='rolls') onRolls(msg.data); if(msg.type==='status') logLine(msg.data.message); if(msg.type==='log') logLine(msg.data); if(msg.type==='log_multi') msg.data.forEach(line=>logLine(line)); };
			ws.onclose=()=>{ setStatus('Disconnected'); me=null; };
		}
		function send(type, data){ ws && ws.readyState===1 && ws.send(JSON.stringify({type, data})); }
			function choose(){ const payload={ faction:$('faction').value, unit:($('unit').value||'').split(' — ')[0], weapons:chosenWeapons, weapon:chosenWeapons[0]||'' }; return payload; }
			function ready(){ send('ready', {}); }
			function lockIn(){ lockedLoadout = choose(); locked=true; setStatus('Locked in'); // disable selectors and weapon toggles
				$('faction').disabled=true; $('unit').disabled=true; [...document.querySelectorAll('#weaponList input[type=checkbox]')].forEach(i=>i.disabled=true);
				// enable matchmaking buttons
				$('btn-ai').disabled=false; $('btn-pvp').disabled=false; $('btn-ready').disabled=true;
			}
	function attack(){
		// If previous dice are lingering, clear immediately when starting a new attack
		if(window.clearDiceTimer){ clearTimeout(window.clearDiceTimer); window.clearDiceTimer=null; }
		resetPersistentDice(); updatePhaseProgress('reset');
		send('attack', {});
	}
	function onState(s){ state=s; updateUI(); }
		let persistentDice = [];
		// Track and clear dice after a short delay when a sequence ends
		window.clearDiceTimer = window.clearDiceTimer || null;
		let lastDiceActive = false;
		function ensurePersistentDice(n){
			const cont = $('dicePersistent');
			if(!cont) return;
			// Grow to n if needed
			while(persistentDice.length < n){
				const d=document.createElement('div'); d.className='dice'; d.textContent='?'; cont.appendChild(d); persistentDice.push({el:d, alive:true});
			}
		}

		function resetPersistentDice(){ const cont=$('dicePersistent'); if(!cont) return; cont.innerHTML=''; persistentDice=[]; }

		function onRolls(ev){
			const {phase, need, rolls, weapon} = ev;
			const tray=$('rollsTray'), hdr=$('rollsHeader'), dice=$('rollsDice');
			const title = (phase==='attacks'?'Attacks dice':phase==='hit'?'Hit rolls':phase==='wound'?'Wound rolls':'Save rolls');
			if(hdr){ hdr.textContent = (weapon? (title+' for '+weapon) : title) + (need? (' — need '+need+'+') : ''); }
			// Hide ephemeral tray dice to avoid duplicates; rely on persistent dice row
			if(tray){ tray.style.display='none'; }
			if(dice){ dice.innerHTML=''; }
			// Update phase progress
			if(phase==='attacks'){
				// Show the raw attacks dice (e.g., 4D6) briefly in the persistent row
				resetPersistentDice(); ensurePersistentDice((rolls||[]).length || 1);
				const cont=$('dicePersistent'); if(cont){ cont.style.display='flex'; }
				(rolls||[]).forEach((v,i)=>{ const slot=persistentDice[i]; if(!slot) return; slot.el.textContent=v; slot.el.classList.remove('bad','good','removed'); slot.alive=true; });
				updatePhaseProgress('attacks');
			} else if(phase==='hit'){
				// reset and create persistent dice equal to initial attack pool
				resetPersistentDice(); ensurePersistentDice((rolls||[]).length); const cont=$('dicePersistent'); if(cont){ cont.style.display='flex'; }
				(rolls||[]).forEach((v,i)=>{
					const slot = persistentDice[i]; if(!slot) return; slot.el.textContent=v; slot.el.classList.remove('bad','good','removed');
					if(need){ if(v>=need){ slot.el.classList.add('good'); } else { slot.el.classList.add('bad','removed'); slot.alive=false; } }
				});
				updatePhaseProgress('hit');
			} else if(phase==='wound'){
				// carry forward only alive dice; update values and mark failures to disappear
				let j=0; (rolls||[]).forEach((v)=>{ while(j<persistentDice.length && !persistentDice[j].alive) j++; if(j<persistentDice.length){ const slot=persistentDice[j]; slot.el.textContent=v; slot.el.classList.remove('bad','good'); if(need){ if(v>=need){ slot.el.classList.add('good'); } else { slot.el.classList.add('bad','removed'); slot.alive=false; } } j++; }}); const cont=$('dicePersistent'); if(cont){ cont.style.display='flex'; }
				updatePhaseProgress('wound');
			} else if(phase==='save'){
				// Defender saves: iterate alive dice and grey out saved ones (no green): saved -> 'removed', failed -> 'bad'
				let j=0; (rolls||[]).forEach((v)=>{ while(j<persistentDice.length && !persistentDice[j].alive) j++; if(j<persistentDice.length){ const slot=persistentDice[j]; slot.el.textContent=v; slot.el.classList.remove('bad','good'); if(need){ if(v>=need){ slot.el.classList.add('removed'); slot.alive=false; } else { slot.el.classList.add('bad'); } } j++; }}); const cont=$('dicePersistent'); if(cont){ cont.style.display='flex'; }
				updatePhaseProgress('save');
				return;
			}
			setTimeout(()=>{
				if(tray) tray.style.display='none';
				if(dice) dice.innerHTML='';
				// keep persistent dice visible across phases; we'll clear when sequence ends in updateUI
			}, 4000);
		}

		function updatePhaseProgress(currentPhase) {
			const phases = ['attacks', 'hit', 'wound', 'save', 'damage'];
			const idx = phases.indexOf(currentPhase);
			phases.forEach((p,i) => {
				const el = $(p + 'Phase');
				if (!el) return;
				if (currentPhase === 'reset') {
					el.style.opacity = '0.4';
					el.style.backgroundColor = '';
					el.style.color = '';
					return;
				}
				if (i < idx) { // completed
					el.style.opacity = '1';
					el.style.backgroundColor = 'rgba(201,167,83,.35)';
					el.style.color = '#e5d5a5';
				} else if (i === idx) { // current
					el.style.opacity = '1';
					el.style.backgroundColor = 'var(--gold)';
					el.style.color = '#000';
				} else { // upcoming
					el.style.opacity = '0.5';
					el.style.backgroundColor = '';
					el.style.color = '';
				}
			});
		}

		// Reintroduce UI updater to avoid syntax/runtime errors in live build
			function updateUI(){
				const hasActiveSequence = ()=> !!state.currentWeapon;
				const inGame = !!(state && state.turn);
				const myTurn = state.turn===me; const phase = state.phase||'attack';
				const canAttack = inGame && myTurn && phase!=='save' && !hasActiveSequence();
				$('btn-attack').disabled = !canAttack;
				// When it's not your turn, show a clear state instead of 'Attack' (unless defender is saving; then hide the button entirely)
				$('btn-attack').textContent = (!myTurn) ? 'you are under attack' : 'Attack';
				// Hide the 'Waiting for opponent...' line when player can attack
				if($('versus')){ $('versus').style.display = canAttack? 'none' : (inGame? 'block' : 'none'); }
				if($("turn")) { $("turn").textContent = myTurn? 'Your turn' : (state.turn? 'Opponent turn' : '—'); }
				// Manual save UI
				const pending = state.pendingSaves; const isDefender = (state.turn!==me);
								if(phase==='save' && pending && isDefender){
									// Hide the Attack button while defender has the explicit 'Roll Saves' action
									const atkBtn=$('btn-attack'); if(atkBtn){ atkBtn.style.display='none'; }
													$('saveUI').style.display='block';
													const saveBtn=$('btn-roll-saves'); if(saveBtn) saveBtn.disabled=false;
													const need=pending.need||0; const cnt=pending.count||0;
													$('saveNeed').textContent = cnt+' × '+need+'+';
													// Ensure the persistent dice row is visible with the correct count (should already match wound successes)
													const cont=$('dicePersistent'); if(cont){ cont.style.display='flex'; }
									// Fully disable overlay during defender saves to allow clicking
									const fo=$('fightOverlay'); if(fo){ fo.style.display='none'; }
									document.body.classList.remove('fight-active');
											// Hide the phase pill that says 'SAVE' to avoid duplicate controls; bottom 'ROLL SAVES' is the single action
											const phaseChipEl=$('cwPhase'); if(phaseChipEl){ phaseChipEl.style.display='none'; }
								} else {
									$('saveUI').style.display='none'; $('diceTray').innerHTML='';
									// Restore Attack button visibility outside defender save UI
									const atkBtn=$('btn-attack'); if(atkBtn){ atkBtn.style.display='inline-block'; }
									const fo=$('fightOverlay'); if(fo){ fo.style.display = inGame? 'block':'none'; }
									document.body.classList.toggle('fight-active', !!inGame);
										// Ensure the phase pill is visible when not in defender save UI
										const phaseChipEl=$('cwPhase'); if(phaseChipEl){ phaseChipEl.style.display='inline-block'; }
								}
				// Player/opponent cards (existing)
				function fillCard(prefix, name, unitName, wounds, maxW, isAI){
					const hp = Math.max(0, Math.min(100, Math.round((wounds||0)/(maxW||1)*100)));
					const hpEl = $(prefix+'HP'); if(hpEl) hpEl.style.width = hp+'%';
					const wEl = $(prefix+'Wounds'); if(wEl) wEl.textContent = (wounds||0)+'/'+(maxW||0);
					const nEl = $(prefix+'Name'); if(nEl) nEl.textContent = name||'—';
					const aEl = $(prefix+'AI'); if(aEl) aEl.textContent = isAI? 'AI' : 'Player';
					const uEl = $(prefix+'Unit'); if(uEl) uEl.textContent = unitName||'—';
				}
				// Determine sides
				const p1 = state.p1||{}; const p2 = state.p2||{};
				const amP1 = (p1 && p1.id===me);
				const meP = amP1? p1 : p2;
				const opP = amP1? p2 : p1;
				// Update side panel (existing IDs)
				function formatFactionUnit(p){ const f=(p&&p.unit&&p.unit.faction)||(p&&p.loadout&&p.loadout.faction)||''; const u=(p&&p.unit&&p.unit.name)||''; return (f?f:'?')+' / '+(u?u:'—'); }
				if($("p1name")){ $("p1name").textContent = p1.name||'—'; $("p1ai").textContent = p1.ai? 'AI':'Player'; $("p1unit").textContent = formatFactionUnit(p1); const hp1=Math.max(0,Math.min(100, Math.round((p1.wounds||0)/(p1.maxW||1)*100))); $("p1hp").style.width = hp1+'%'; $("p1wounds").textContent = p1.wounds||0; $("p1weapon").textContent = (state.turn===p1.id && state.currentWeapon) ? state.currentWeapon : '—'; }
				if($("p2name")){ $("p2name").textContent = p2.name||'—'; $("p2ai").textContent = p2.ai? 'AI':'Player'; $("p2unit").textContent = formatFactionUnit(p2); const hp2=Math.max(0,Math.min(100, Math.round((p2.wounds||0)/(p2.maxW||1)*100))); $("p2hp").style.width = hp2+'%'; $("p2wounds").textContent = p2.wounds||0; $("p2weapon").textContent = (state.turn===p2.id && state.currentWeapon) ? state.currentWeapon : '—'; }
				// Update fight overlay cards
				fillCard('foMe', meP.name, formatFactionUnit(meP), meP.wounds, meP.maxW, meP.ai);
				fillCard('foOpp', opP.name, formatFactionUnit(opP), opP.wounds, opP.maxW, opP.ai);
				// Meta lines: Points and defenses
				function metaFor(p){ const u=p&&p.unit; if(!u) return ''; const parts=[]; if(u.points) parts.push((u.points)+' pts'); if(u.T) parts.push('T '+u.T); if(u.Sv) parts.push('Sv '+u.Sv+'+'); if(u.InvSv) parts.push('Inv '+u.InvSv+'+'); if(u.FNP) parts.push('FNP '+u.FNP+'+'); if(u.DR) parts.push('DR -'+u.DR); return parts.join(' · '); }
				const meMeta = metaFor(meP), oppMeta = metaFor(opP);
				if($("foMeMeta")) $("foMeMeta").textContent = meMeta || ' ';
				if($("foOppMeta")) $("foOppMeta").textContent = oppMeta || ' ';
				const p1Meta = metaFor(p1), p2Meta = metaFor(p2);
				if($("p1meta")) $("p1meta").textContent = p1Meta || ' ';
				if($("p2meta")) $("p2meta").textContent = p2Meta || ' ';
						// Weapon panel in the middle (current weapon + stats + tags)
						const weapName = state.currentWeapon || (meP && meP.loadout && meP.loadout.weapon) || '';
						// Resolve attacker and weapon object early so title chips can use it safely
						function findWeapon(player, name){ const ws=((player&&player.unit&&player.unit.weapons)||[]); return ws.find(w=> (w.name||w.Name)===name) || null; }
						const attackerIsMe = (state.turn===me);
						const attacker = attackerIsMe? meP : opP;
						const wobj = weapName? findWeapon(attacker, weapName) : null;
								const weapTitle = $('cwTitle'); if(weapTitle){
									weapTitle.textContent='';
									weapTitle.appendChild(document.createTextNode(weapName||'—'));
									// Inline ability chips next to weapon name
									const tagsInline=(wobj&&(wobj.tags||wobj.Tags))||[];
									if(tagsInline && tagsInline.length){
										tagsInline.forEach(t=>{ const chip=document.createElement('span'); chip.className='pill'; chip.style.marginLeft='8px'; chip.style.fontSize='10px'; chip.style.padding='2px 6px'; chip.textContent=t; weapTitle.appendChild(chip); });
									}
								}
								const phaseChip = $('cwPhase'); if(phaseChip) phaseChip.textContent = (state.phase||'attack').toUpperCase();
								const weapStats = $('cwStats');
								const weapTags = $('cwTags');
									const weapDir = $('cwDir');
									const saveChip = $('cwSaveStatus');
						if(weapStats||weapTags){
									if(weapStats){
										if(wobj){
											const S = (wobj.s ?? wobj.S);
											const AP = (wobj.ap ?? wobj.AP);
											const A = (wobj.attacks_expr ?? wobj.AttacksExpr ?? wobj.attacks ?? wobj.Attacks);
											const BS = (wobj.bs ?? wobj.BS);
											const D = (wobj.d ?? wobj.D);
											weapStats.textContent = 'A:'+A+' BS/WS:'+(BS ?? '?')+'+ S:'+(S ?? '?')+' AP:'+(AP ?? '?')+' D:'+(D ?? '?');
										} else { weapStats.textContent='—'; }
									}
							if(weapTags){ weapTags.innerHTML=''; /* tags now inline with name */ }
								if(weapDir){ const label = attackerIsMe? (meP.name||'You') : (opP.name||'Opponent'); weapDir.textContent = attackerIsMe? (label+'  ➔➔➔') : ('➔➔➔  '+label); }
							if(saveChip){ const ps=state.pendingSaves; if(ps&&ps.count){ const who = (state.turn===me)? (opP.name||'Opponent') : (meP.name||'You'); saveChip.style.display='inline-block'; saveChip.textContent = who+': save '+ps.count+' @ '+ps.need+'+  D'+ps.dmg; } else { saveChip.style.display='none'; saveChip.textContent=''; } }
								const weapPanel = $('currentWeaponPanel'); if(weapPanel){ weapPanel.style.display = inGame? 'block':'none'; weapPanel.classList.remove('att-left','att-right'); weapPanel.classList.add(attackerIsMe? 'att-right':'att-left'); }
						}
				// Toggle main panels based on game state
				$('setup').style.display = inGame? 'none' : 'block';
				$('p1Panel').style.display = inGame? 'block' : 'none';
				$('battlefield').style.display = inGame? 'block' : 'none';
				$('p2Panel').style.display = inGame? 'block' : 'none';
				$('postgame').style.display = (inGame && state.winner)? 'grid' : 'none';
				setStatus(state.winner? 'Game over' : (inGame? 'In game' : 'Ready'));
				// Dice strip: keep visible during sequence; after it ends, keep for 2s, then clear, or clear immediately on next Attack
				const dp=$('dicePersistent'); if(dp){
					const active = inGame && !!state.currentWeapon;
					if(lastDiceActive && !active){
						if(window.clearDiceTimer){ clearTimeout(window.clearDiceTimer); }
						window.clearDiceTimer = setTimeout(()=>{ resetPersistentDice(); updatePhaseProgress('reset'); dp.style.display='none'; window.clearDiceTimer=null; }, 2000);
					}
					dp.style.display = (active || window.clearDiceTimer)? 'flex' : 'none';
					lastDiceActive = active;
				}
				// Always keep the public panels updated, even during a game
				startLobby(); startLeaderboard(); startDaily();
			}
	function renderDice(n, need){ const tray=$('diceTray'); tray.innerHTML=''; for(let i=0;i<n;i++){ const d=document.createElement('div'); d.className='dice'; d.dataset.need=need; d.textContent='?'; tray.appendChild(d);} }
		$('btn-roll-saves').onclick=()=>{
			const btn=$('btn-roll-saves'); if(btn.disabled) return;
			const need=(state&&state.pendingSaves&&state.pendingSaves.need)||0;
			const vals=[]; let j=0;
			// Roll for each alive persistent die
			while(j<persistentDice.length){ if(!persistentDice[j].alive){ j++; continue; } const v=1+Math.floor(Math.random()*6); vals.push(v); const slot=persistentDice[j]; slot.el.textContent=v; slot.el.classList.remove('bad','good'); if(v>=need){ slot.el.classList.add('removed'); slot.alive=false; } else { slot.el.classList.add('bad'); } j++; }
			send('save_rolls', {rolls: vals}); btn.disabled=true; setTimeout(()=>{ $('saveUI').style.display='none'; }, 2200);
		};
	$('btn-clear-log').onclick=()=>{ const el=$('log'); if(el){ el.textContent=''; } };
		function logLine(t){
			const el=$('log');
			const atBottom=el.scrollTop+el.clientHeight>=el.scrollHeight-4;
			const now=new Date();
			// Format: YYYY-MM-DD HH:MM:SS (24-hour)
			const pad=(n)=> String(n).padStart(2,'0');
			const ts = now.getFullYear()+"-"+pad(now.getMonth()+1)+"-"+pad(now.getDate())+" "+pad(now.getHours())+":"+pad(now.getMinutes())+":"+pad(now.getSeconds());
			el.textContent += '['+ts+'] '+t+'\n';
			if(atBottom) el.scrollTop=el.scrollHeight;
		}
    function setStatus(t){ $('status').textContent=t; }
	$('btn-ai').onclick=()=>{ dbg('btn-ai clicked'); connect(true); };
	$('btn-pvp').onclick=()=>{ dbg('btn-pvp clicked'); connect(false); };
		$('faction').addEventListener('click', ()=>{ const fac=$('faction'); dbg('faction: click value='+JSON.stringify(fac.value)+' options='+(fac.children?fac.children.length:0)); });
		$('faction').onchange=()=>{ const fac=$('faction'); dbg('faction: change to '+JSON.stringify(fac.value)); locked=false; lockedLoadout=null; $('btn-ready').disabled=true; loadUnits(); };
		$('btn-ready').onclick=()=>{ dbg('btn-ready lock-in'); lockIn(); };
		$('btn-attack').onclick=attack;
		$('btn-rematch').onclick=()=>{ location.reload(); };
		$('btn-back').onclick=()=>{ location.reload(); };
		loadFactions(); startLobby(); startLeaderboard(); startDaily();

				async function fetchLobby(){
					try{
						const res=await fetch('/lobby');
						if(!res.ok) throw new Error('HTTP '+res.status);
						const data=await res.json();
						renderLobby((data&&data.players)||[]);
					}catch(err){
						// silent
					}
				}
		function renderLobby(list){
					const el=$('lobbyList'); if(!el) return; el.innerHTML='';
					if(!list || !list.length){ el.innerHTML='<div class="row"><span>No one online</span><span></span></div>'; return; }
					list.forEach(p=>{
						const row=document.createElement('div'); row.className='row';
						const left=document.createElement('span');
						let statusText = (p.status==='in-game' && p.opponent) ? ('duelling '+p.opponent) : (p.phase || p.status || '');
						if((p.status==='in-game' && p.opponent==='AI')){ statusText += ' (vs AI)'; }
						if(p.status==='lobby' && p.wantsAI){ statusText += ' (wants AI)'; }
						if((p.status||'')==='lobby' && p.locked===false){ statusText = 'Idling'; }
						left.textContent = (p.name||'Anon') + ' — ' + statusText;
						const right=document.createElement('span');
			const isLobby = (p.status||'')==='lobby';
			const weapons=(isLobby && p.weapons && p.weapons.length)? (' ['+p.weapons.join(', ')+']') : '';
			right.textContent=(p.faction||'?')+' / '+(p.unit||'?')+ (p.points? (' — '+p.points+'pts') : '') + weapons;
						row.appendChild(left); row.appendChild(right); el.appendChild(row);
					});
				}
				function startLobby(){ if(lobbyTimer) return; fetchLobby(); lobbyTimer=setInterval(fetchLobby, 4000); }
				function stopLobby(){ if(lobbyTimer){ clearInterval(lobbyTimer); lobbyTimer=null; } }

				// Leaderboard polling
				async function fetchLeaderboard(){
					try{
						const res=await fetch('/leaderboard');
						if(!res.ok) throw new Error('HTTP '+res.status);
						const data=await res.json();
						renderLeaderboard((data&&data.players)||[]);
					}catch(err){ /* silent */ }
				}
		function renderLeaderboard(list){
					const el=$('leaderboard'); if(!el) return; el.innerHTML='';
					if(!list || !list.length){ el.innerHTML='<div class="row"><span>No one online</span><span></span></div>'; return; }
					list.forEach(p=>{
						const row=document.createElement('div'); row.className='row';
						const left=document.createElement('span'); left.textContent=(p.name||'Anon')+' — '+(p.phase||p.status||'');
			const right=document.createElement('span'); right.textContent=(p.faction||'?')+' / '+(p.unit||'?')+ (p.points? (' — '+p.points+'pts') : '');
						row.appendChild(left); row.appendChild(right); el.appendChild(row);
					});
				}
				function startLeaderboard(){ if(lbTimer) return; fetchLeaderboard(); lbTimer=setInterval(fetchLeaderboard, 5000); }
				function stopLeaderboard(){ if(lbTimer){ clearInterval(lbTimer); lbTimer=null; } }

				// Daily records polling
				async function fetchDaily(){
					try{
						const res=await fetch('/leaderboard/daily');
						if(!res.ok) throw new Error('HTTP '+res.status);
						const data=await res.json();
						renderDaily(data||{});
					}catch(err){ /* silent */ }
				}
				function renderDaily(s){
					const el=$('leaderboardDaily'); if(!el) return; el.innerHTML='';
					const date=(s.date||'').toString();
					const top=(s.top_damage)||{}; const worst=(s.worst_save)||{};
					const noData = (!top || (top.damage||0)===0) && (!worst || (worst.roll||7)>=7);
					if(noData){ el.innerHTML='<div class="row"><span>No data yet</span><span></span></div>'; return; }
					if(top && (top.damage||0)>0){
						const row=document.createElement('div'); row.className='row';
						const left=document.createElement('span'); left.textContent='Top damage ('+date+'): '+top.damage+' by '+(top.attacker||'?')+' — '+(top.attacker_faction||'?')+' / '+(top.attacker_unit||'?')+' with '+(top.weapon||'?');
						const right=document.createElement('span'); right.textContent='vs '+(top.defender||'?');
						row.appendChild(left); row.appendChild(right); el.appendChild(row);
					}
					if(worst && (worst.roll||7)<7){
						const row=document.createElement('div'); row.className='row';
						const left=document.createElement('span'); left.textContent='Worst save ('+date+'): rolled '+worst.roll+' (need '+(worst.need||'?')+'+) on '+(worst.count||'?')+' saves by '+(worst.defender||'?');
						const right=document.createElement('span'); right.textContent=(worst.defender_faction||'?')+' / '+(worst.defender_unit||'?');
						row.appendChild(left); row.appendChild(right); el.appendChild(row);
					}
				}
				function startDaily(){ if(dailyTimer) return; fetchDaily(); dailyTimer=setInterval(fetchDaily, 10000); }
				function stopDaily(){ if(dailyTimer){ clearInterval(dailyTimer); dailyTimer=null; } }
  </script>
	<!-- Sentinel: tail script to confirm main script closed cleanly -->
	<script>try{ fetch('/debug', {method:'POST', headers:{'Content-Type':'text/plain'}, body: (new Date().toISOString()+" sentinel: post-main-script")}); }catch(_){}</script>
</body>
</html>`
