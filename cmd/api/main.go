package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	game "github.com/pefman/w40k-duel/internal/engine"
)

type Faction struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Link string `json:"link"`
}

// Unit holds datasheet-level info we care about for API
// Datasheets.csv: id|name|faction_id|...|link
// We'll extend as needed.
type Unit struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	FactionID string `json:"faction_id"`
	Role      string `json:"role,omitempty"`
	Link      string `json:"link,omitempty"`
	// Basic stats (populated from first model row when available)
	T         string `json:"T,omitempty"`
	W         string `json:"W,omitempty"`
	Points    string `json:"points,omitempty"`
}

type Weapon struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Range       string `json:"range"`
	Type        string `json:"type"`
	Attacks     string `json:"attacks"`
	BSOrWS      string `json:"bs_ws"`
	Strength    string `json:"strength"`
	AP          string `json:"ap"`
	Damage      string `json:"damage"`
	// internal order from CSV line column for stable ordering in responses
	Order int `json:"-"`
}

type Store struct {
	FactionsByID   map[string]Faction
	FactionsBySlug map[string]Faction
	FactionsList   []Faction
	UnitsByID      map[string]Unit
	UnitsByFac     map[string][]Unit
	WeaponsByDS    map[string][]Weapon    // datasheet_id -> weapons
	ModelsByDS     map[string][]Model     // datasheet_id -> models
	KeywordsByDS   map[string][]Keyword   // datasheet_id -> keywords
	AbilitiesByDS  map[string][]Ability   // datasheet_id -> abilities
	OptionsByDS    map[string][]Option    // datasheet_id -> options
	CostsByDS      map[string][]ModelCost // datasheet_id -> model costs
}

func mustOpen(path string) *os.File {
	f, err := os.Open(path)
	if err != nil {
		log.Fatalf("open %s: %v", path, err)
	}
	return f
}

func readPipeCSV(path string) ([][]string, error) {
	f := mustOpen(path)
	defer f.Close()
	csvr := csv.NewReader(f)
	csvr.Comma = '|'
	// CSV files contain unescaped quotes (e.g., 6" movement), allow them.
	csvr.LazyQuotes = true
	csvr.FieldsPerRecord = -1
	return csvr.ReadAll()
}

func loadFactions(root string) ([]Faction, map[string]Faction, error) {
	rows, err := readPipeCSV(filepath.Join(root, "src", "Factions.csv"))
	if err != nil {
		return nil, nil, err
	}
	var list []Faction
	byID := map[string]Faction{}
	for i, r := range rows {
		if i == 0 {
			continue
		}
		if len(r) < 3 {
			continue
		}
		f := Faction{ID: r[0], Name: r[1], Link: r[2]}
		list = append(list, f)
		byID[f.ID] = f
	}
	// sort by name
	sort.Slice(list, func(i, j int) bool { return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name) })
	return list, byID, nil
}

func loadUnits(root string) (map[string]Unit, map[string][]Unit, error) {
	rows, err := readPipeCSV(filepath.Join(root, "src", "Datasheets.csv"))
	if err != nil {
		return nil, nil, err
	}
	byID := map[string]Unit{}
	byFac := map[string][]Unit{}
	for i, r := range rows {
		if i == 0 {
			continue
		}
		if len(r) < 6 {
			continue
		}
		u := Unit{ID: r[0], Name: r[1], FactionID: r[2]}
		// role is r[5], link is last column; safer to check length
		if len(r) > 5 {
			u.Role = r[5]
		}
		if len(r) > 13 {
			u.Link = r[13]
		}
		byID[u.ID] = u
		byFac[u.FactionID] = append(byFac[u.FactionID], u)
	}
	// sort units per faction by name
	for fid := range byFac {
		units := byFac[fid]
		sort.Slice(units, func(i, j int) bool { return strings.ToLower(units[i].Name) < strings.ToLower(units[j].Name) })
		byFac[fid] = units
	}
	return byID, byFac, nil
}

func loadWeapons(root string) (map[string][]Weapon, error) {
	rows, err := readPipeCSV(filepath.Join(root, "src", "Datasheets_wargear.csv"))
	if err != nil {
		return nil, err
	}
	byDS := map[string][]Weapon{}
	for i, r := range rows {
		if i == 0 {
			continue
		}
		if len(r) < 13 {
			continue
		}
		dsid := r[0]
		order := 0
		if len(r) > 1 {
			if n, err := strconv.Atoi(strings.TrimSpace(r[1])); err == nil {
				order = n
			}
		}
		w := Weapon{
			Name:        r[4],
			Description: htmlToText(r[5]),
			Range:       r[6],
			Type:        r[7],
			Attacks:     r[8],
			BSOrWS:      r[9],
			Strength:    r[10],
			AP:          r[11],
			Damage:      r[12],
			Order:       order,
		}
		byDS[dsid] = append(byDS[dsid], w)
	}
	// sort weapons by CSV line order for stable outputs
	for dsid := range byDS {
		ws := byDS[dsid]
		sort.Slice(ws, func(i, j int) bool { return ws[i].Order < ws[j].Order })
		byDS[dsid] = ws
	}
	return byDS, nil
}

// Model stats from Datasheets_models.csv
type Model struct {
	Line       int    `json:"line"`
	Name       string `json:"name"`
	M          string `json:"M"`
	T          string `json:"T"`
	Sv         string `json:"Sv"`
	InvSv      string `json:"inv_sv"`
	InvSvDescr string `json:"inv_sv_descr"`
	W          string `json:"W"`
	Ld         string `json:"Ld"`
	OC         string `json:"OC"`
	BaseSize   string `json:"base_size"`
	BaseDescr  string `json:"base_size_descr"`
}

func loadModels(root string) (map[string][]Model, error) {
	rows, err := readPipeCSV(filepath.Join(root, "src", "Datasheets_models.csv"))
	if err != nil {
		return nil, err
	}
	byDS := map[string][]Model{}
	for i, r := range rows {
		if i == 0 {
			continue
		}
		if len(r) < 13 {
			continue
		}
		dsid := r[0]
		line := 0
		if n, err := strconv.Atoi(strings.TrimSpace(r[1])); err == nil {
			line = n
		}
		m := Model{
			Line:       line,
			Name:       r[2],
			M:          r[3],
			T:          r[4],
			Sv:         r[5],
			InvSv:      r[6],
			InvSvDescr: r[7],
			W:          r[8],
			Ld:         r[9],
			OC:         r[10],
			BaseSize:   r[11],
			BaseDescr:  r[12],
		}
		byDS[dsid] = append(byDS[dsid], m)
	}
	for dsid := range byDS {
		ms := byDS[dsid]
		sort.Slice(ms, func(i, j int) bool { return ms[i].Line < ms[j].Line })
		byDS[dsid] = ms
	}
	return byDS, nil
}

// Points cost from Datasheets_models_cost.csv
type ModelCost struct {
	Line        int    `json:"line"`
	Description string `json:"description"`
	Cost        string `json:"cost"`
}

func loadModelCosts(root string) (map[string][]ModelCost, error) {
	rows, err := readPipeCSV(filepath.Join(root, "src", "Datasheets_models_cost.csv"))
	if err != nil {
		return nil, err
	}
	byDS := map[string][]ModelCost{}
	for i, r := range rows {
		if i == 0 {
			continue
		}
		if len(r) < 4 {
			continue
		}
		dsid := r[0]
		line := 0
		if n, err := strconv.Atoi(strings.TrimSpace(r[1])); err == nil {
			line = n
		}
		mc := ModelCost{Line: line, Description: strings.TrimSpace(r[2]), Cost: strings.TrimSpace(r[3])}
		byDS[dsid] = append(byDS[dsid], mc)
	}
	for dsid := range byDS {
		list := byDS[dsid]
		sort.Slice(list, func(i, j int) bool { return list[i].Line < list[j].Line })
		byDS[dsid] = list
	}
	return byDS, nil
}

func htmlToText(s string) string {
	// very simple scrub: strip tags
	out := []rune{}
	inTag := false
	for _, ch := range s {
		if ch == '<' {
			inTag = true
			continue
		}
		if ch == '>' {
			inTag = false
			continue
		}
		if !inTag {
			out = append(out, ch)
		}
	}
	return strings.TrimSpace(strings.ReplaceAll(string(out), "\n", " "))
}

// Keyword from Datasheets_keywords.csv
type Keyword struct {
	Keyword   string `json:"keyword"`
	Model     string `json:"model,omitempty"`
	IsFaction bool   `json:"is_faction_keyword"`
}

func loadKeywords(root string) (map[string][]Keyword, error) {
	rows, err := readPipeCSV(filepath.Join(root, "src", "Datasheets_keywords.csv"))
	if err != nil {
		return nil, err
	}
	byDS := map[string][]Keyword{}
	for i, r := range rows {
		if i == 0 {
			continue
		}
		if len(r) < 4 {
			continue
		}
		dsid := r[0]
		kw := Keyword{Keyword: r[1], Model: r[2]}
		kw.IsFaction = strings.EqualFold(strings.TrimSpace(r[3]), "true")
		byDS[dsid] = append(byDS[dsid], kw)
	}
	return byDS, nil
}

// Ability from Datasheets_abilities.csv
type Ability struct {
	Line        int    `json:"line"`
	AbilityID   string `json:"ability_id,omitempty"`
	Model       string `json:"model,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Parameter   string `json:"parameter,omitempty"`
}

func loadAbilities(root string) (map[string][]Ability, error) {
	rows, err := readPipeCSV(filepath.Join(root, "src", "Datasheets_abilities.csv"))
	if err != nil {
		return nil, err
	}
	byDS := map[string][]Ability{}
	for i, r := range rows {
		if i == 0 {
			continue
		}
		if len(r) < 8 {
			continue
		}
		dsid := r[0]
		line := 0
		if n, err := strconv.Atoi(strings.TrimSpace(r[1])); err == nil {
			line = n
		}
		ab := Ability{
			Line:        line,
			AbilityID:   r[2],
			Model:       r[3],
			Name:        r[4],
			Description: htmlToText(r[5]),
			Type:        r[6],
			Parameter:   r[7],
		}
		byDS[dsid] = append(byDS[dsid], ab)
	}
	for dsid := range byDS {
		list := byDS[dsid]
		sort.Slice(list, func(i, j int) bool { return list[i].Line < list[j].Line })
		byDS[dsid] = list
	}
	return byDS, nil
}

// Option from Datasheets_options.csv
type Option struct {
	Line        int    `json:"line"`
	Bullet      string `json:"bullet,omitempty"`
	Description string `json:"description"`
}

func loadOptions(root string) (map[string][]Option, error) {
	rows, err := readPipeCSV(filepath.Join(root, "src", "Datasheets_options.csv"))
	if err != nil {
		return nil, err
	}
	byDS := map[string][]Option{}
	for i, r := range rows {
		if i == 0 {
			continue
		}
		if len(r) < 4 {
			continue
		}
		dsid := r[0]
		line := 0
		if n, err := strconv.Atoi(strings.TrimSpace(r[1])); err == nil {
			line = n
		}
		opt := Option{Line: line}
		if len(r) > 2 {
			opt.Bullet = r[2]
		}
		if len(r) > 3 {
			opt.Description = htmlToText(r[3])
		}
		byDS[dsid] = append(byDS[dsid], opt)
	}
	for dsid := range byDS {
		list := byDS[dsid]
		sort.Slice(list, func(i, j int) bool { return list[i].Line < list[j].Line })
		byDS[dsid] = list
	}
	return byDS, nil
}

func newStore(root string) (*Store, error) {
	fList, fMap, err := loadFactions(root)
	if err != nil {
		return nil, err
	}
	uByID, uByFac, err := loadUnits(root)
	if err != nil {
		return nil, err
	}
	wByDS, err := loadWeapons(root)
	if err != nil {
		return nil, err
	}
	mByDS, err := loadModels(root)
	if err != nil {
		return nil, err
	}
	kByDS, err := loadKeywords(root)
	if err != nil {
		return nil, err
	}
	aByDS, err := loadAbilities(root)
	if err != nil {
		return nil, err
	}
	oByDS, err := loadOptions(root)
	if err != nil {
		return nil, err
	}
	cByDS, err := loadModelCosts(root)
	if err != nil {
		return nil, err
	}
	// build faction slug map (lowercased hyphenated name)
	bySlug := map[string]Faction{}
	for _, f := range fList {
		slug := toSlug(f.Name)
		bySlug[slug] = f
		bySlug[strings.ToLower(f.ID)] = f // also allow id lowercased as slug
		bySlug[f.ID] = f                  // and raw id
	}
	return &Store{
		FactionsByID:   fMap,
		FactionsBySlug: bySlug,
		FactionsList:   fList,
		UnitsByID:      uByID,
		UnitsByFac:     uByFac,
		WeaponsByDS:    wByDS,
		ModelsByDS:     mByDS,
		KeywordsByDS:   kByDS,
		AbilitiesByDS:  aByDS,
		OptionsByDS:    oByDS,
		CostsByDS:      cByDS,
	}, nil
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":   http.StatusText(code),
		"message": msg,
		"status":  code,
	})
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

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// simple CORS for GET/OPTIONS
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ================= Lobby (in-memory) =================
type LobbyEntry struct {
	Name    string `json:"name"`
	Phase   string `json:"phase"`
	Since   int64  `json:"since"`
	Updated int64  `json:"updated"`
}

type Lobby struct {
	mu     sync.Mutex
	byName map[string]*LobbyEntry // key: lowercased name
}

func newLobby() *Lobby { return &Lobby{byName: map[string]*LobbyEntry{}} }

func (l *Lobby) upsert(name, phase string) *LobbyEntry {
	if name == "" { return nil }
	now := time.Now().Unix()
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" { return nil }
	l.mu.Lock()
	defer l.mu.Unlock()
	if e, ok := l.byName[key]; ok {
		e.Phase = phase
		e.Updated = now
		return e
	}
	e := &LobbyEntry{Name: name, Phase: phase, Since: now, Updated: now}
	l.byName[key] = e
	return e
}

func (l *Lobby) setPhase(name, phase string) bool {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" { return false }
	l.mu.Lock()
	defer l.mu.Unlock()
	if e, ok := l.byName[key]; ok {
		e.Phase = phase
	e.Updated = time.Now().Unix()
		return true
	}
	return false
}

func (l *Lobby) list() []LobbyEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]LobbyEntry, 0, len(l.byName))
	for _, e := range l.byName {
		out = append(out, *e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Since == out[j].Since {
			return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
		}
		return out[i].Since < out[j].Since
	})
	return out
}

// end lobby types

// ================= Match log (in-memory) =================
type MatchEntry struct {
	Time     int64            `json:"time"`
	Actor    string           `json:"actor"`
	Round    int              `json:"round"`
	Step     int              `json:"step"`
	Attacker game.UnitSnapshot `json:"attacker"`
	Defender game.UnitSnapshot `json:"defender"`
	Weapon   game.WeaponSnapshot `json:"weapon"`
	Result   game.ShootingResult `json:"result"`
}

type MatchRecord struct {
	ID      string       `json:"id"`
	Created int64        `json:"created"`
	Updated int64        `json:"updated"`
	Entries []MatchEntry `json:"entries"`
}

type MatchLog struct {
	mu   sync.Mutex
	recs map[string]*MatchRecord
}

func newMatchLog() *MatchLog { return &MatchLog{recs: map[string]*MatchRecord{}} }

func (m *MatchLog) append(id string, e MatchEntry) *MatchRecord {
	if id == "" { return nil }
	now := time.Now().Unix()
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.recs[id]
	if !ok {
		rec = &MatchRecord{ID: id, Created: now, Updated: now}
		m.recs[id] = rec
	}
	rec.Entries = append(rec.Entries, e)
	rec.Updated = now
	return rec
}

func (m *MatchLog) get(id string) *MatchRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rec, ok := m.recs[id]; ok {
		return rec
	}
	return nil
}

func (m *MatchLog) put(rec *MatchRecord) {
	if rec == nil || strings.TrimSpace(rec.ID) == "" { return }
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recs[rec.ID] = rec
}
// end match log types

// ============ Optional local persistence for match logs (dev/debug) ============
// Controlled by env MATCH_LOG_DIR. When set, match records will be saved to disk
// after each append, and GET will attempt lazy load from disk if not in memory.

func getMatchPersistDir() string {
	dir := strings.TrimSpace(os.Getenv("MATCH_LOG_DIR"))
	if dir == "" { return "" }
	if !filepath.IsAbs(dir) {
		// make relative paths anchored to cwd
		abs, err := filepath.Abs(dir)
		if err == nil { dir = abs }
	}
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

func sanitizeIDForFile(id string) string {
	// keep alnum, dash, underscore; replace others with '-'
	b := make([]rune, 0, len(id))
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b = append(b, r)
		} else {
			b = append(b, '-')
		}
	}
	out := strings.Trim(strings.ReplaceAll(string(b), "--", "-"), "-")
	if out == "" { out = "match" }
	return out
}

func matchFilePath(dir, id string) string {
	return filepath.Join(dir, sanitizeIDForFile(id)+".json")
}

func saveMatchRecord(dir string, rec *MatchRecord) {
	if dir == "" || rec == nil { return }
	path := matchFilePath(dir, rec.ID)
	// write atomically
	tmp := path + ".tmp"
	data, _ := json.MarshalIndent(rec, "", "  ")
	_ = os.WriteFile(tmp, data, 0o644)
	_ = os.Rename(tmp, path)
}

func loadMatchRecord(dir, id string) *MatchRecord {
	if dir == "" || strings.TrimSpace(id) == "" { return nil }
	path := matchFilePath(dir, id)
	data, err := os.ReadFile(path)
	if err != nil { return nil }
	var rec MatchRecord
	if err := json.Unmarshal(data, &rec); err != nil { return nil }
	// basic sanity
	if strings.TrimSpace(rec.ID) == "" { rec.ID = id }
	return &rec
}

func main() {
	root := "."
	store, err := newStore(root)
	if err != nil {
		log.Fatalf("load store: %v", err)
	}
	lobby := newLobby()
	matches := newMatchLog()
	// Optional local persistence dir for dev/debug
	matchPersistDir := getMatchPersistDir()

	mux := http.NewServeMux()
	// Serve static mockup from ./public at root
	mux.Handle("/", http.FileServer(http.Dir("public")))
	// Statistics endpoints
	mux.HandleFunc("/api/stats/save", SaveStatsHandler)
	mux.HandleFunc("/api/stats/get", GetStatsHandler)
		mux.HandleFunc("/api/stats/max-attack", GetMaxAttackHandler)
		mux.HandleFunc("/api/stats/max-attack/today", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				GetGlobalMaxAttackToday(w, r)
				return
			}
			if r.Method == http.MethodPost {
				PostGlobalMaxAttackToday(w, r)
				return
			}
			writeError(w, http.StatusMethodNotAllowed, "GET or POST only")
		})

	// Health
	mux.HandleFunc("/api/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"status": "ok"})
	})

	// Lobby endpoints
	// GET /api/lobby -> list of users with phases
	mux.HandleFunc("/api/lobby", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "GET only")
			return
		}
		writeJSON(w, lobby.list())
	})

	// Simulation endpoints (shooting-only duel head-up)
	mux.HandleFunc("/api/sim/shoot", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "POST only")
			return
		}
		var req struct {
			Attacker struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				T     int    `json:"T"`
				W     int    `json:"W"`
				Sv    int    `json:"Sv"`
				InvSv int    `json:"InvSv"`
				Keywords []string `json:"keywords,omitempty"`
				Abilities []string `json:"abilities,omitempty"`
			} `json:"attacker"`
			Defender struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				T     int    `json:"T"`
				W     int    `json:"W"`
				Sv    int    `json:"Sv"`
				InvSv int    `json:"InvSv"`
				Keywords []string `json:"keywords,omitempty"`
				Abilities []string `json:"abilities,omitempty"`
			} `json:"defender"`
			Weapon struct {
				Name     string `json:"name"`
				Type     string `json:"type"`
				Attacks  string `json:"attacks"`
				Skill    int    `json:"skill"`
				Strength int    `json:"strength"`
				AP       int    `json:"ap"`
				Damage   string `json:"damage"`
				Abilities []string `json:"abilities,omitempty"`
			} `json:"weapon"`
			MatchID string `json:"match_id,omitempty"`
			Meta    struct {
				Actor string `json:"actor,omitempty"`
				Round int    `json:"round,omitempty"`
				Step  int    `json:"step,omitempty"`
			} `json:"meta,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		att := game.UnitSnapshot{ID: req.Attacker.ID, Name: req.Attacker.Name, T: req.Attacker.T, W: req.Attacker.W, Sv: req.Attacker.Sv, InvSv: req.Attacker.InvSv, Keywords: req.Attacker.Keywords, Abilities: req.Attacker.Abilities}
		def := game.UnitSnapshot{ID: req.Defender.ID, Name: req.Defender.Name, T: req.Defender.T, W: req.Defender.W, Sv: req.Defender.Sv, InvSv: req.Defender.InvSv, Keywords: req.Defender.Keywords, Abilities: req.Defender.Abilities}
		wep := game.WeaponSnapshot{Name: req.Weapon.Name, Type: req.Weapon.Type, Attacks: req.Weapon.Attacks, Skill: req.Weapon.Skill, Strength: req.Weapon.Strength, AP: req.Weapon.AP, Damage: req.Weapon.Damage, Abilities: req.Weapon.Abilities}
		res := game.ResolveShooting(att, def, wep)
		// Append to match log if provided
		if strings.TrimSpace(req.MatchID) != "" {
			entry := MatchEntry{
				Time:     time.Now().Unix(),
				Actor:    strings.TrimSpace(req.Meta.Actor),
				Round:    req.Meta.Round,
				Step:     req.Meta.Step,
				Attacker: att,
				Defender: def,
				Weapon:   wep,
				Result:   res,
			}
			rec := matches.append(strings.TrimSpace(req.MatchID), entry)
			// Persist locally if enabled
			if matchPersistDir != "" {
				saveMatchRecord(matchPersistDir, rec)
			}
		}
		writeJSON(w, res)
	})

	// GET /api/match/{id} -> full match log
	mux.HandleFunc("/api/match/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "GET only")
			return
		}
		id := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/match/"))
		if id == "" {
			writeError(w, http.StatusBadRequest, "missing match id")
			return
		}
		if rec := matches.get(id); rec != nil {
			writeJSON(w, rec)
			return
		}
		// Try lazy-load from disk if enabled
		if md := getMatchPersistDir(); md != "" {
			if rec := loadMatchRecord(md, id); rec != nil {
				matches.put(rec)
				writeJSON(w, rec)
				return
			}
		}
		writeError(w, http.StatusNotFound, "match not found")
	})
	// POST /api/lobby/join {name}
	mux.HandleFunc("/api/lobby/join", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "POST only")
			return
		}
		var body struct{ Name string `json:"name"` }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Name) == "" {
			writeError(w, http.StatusBadRequest, "invalid name")
			return
		}
		e := lobby.upsert(strings.TrimSpace(body.Name), "idle")
		if e == nil {
			writeError(w, http.StatusBadRequest, "invalid name")
			return
		}
		writeJSON(w, e)
	})
	// POST /api/lobby/phase {name, phase}
	mux.HandleFunc("/api/lobby/phase", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "POST only")
			return
		}
		var body struct{ Name, Phase string }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Name) == "" {
			writeError(w, http.StatusBadRequest, "invalid payload")
			return
		}
		if ok := lobby.setPhase(strings.TrimSpace(body.Name), strings.TrimSpace(body.Phase)); !ok {
			writeError(w, http.StatusNotFound, "user not in lobby")
			return
		}
		writeJSON(w, map[string]string{"status": "ok"})
	})

	// GET /api/factions
	mux.HandleFunc("/api/factions", func(w http.ResponseWriter, r *http.Request) {
		// optional ?sort=name|id
		out := make([]Faction, len(store.FactionsList))
		copy(out, store.FactionsList)
		sortParam := strings.ToLower(r.URL.Query().Get("sort"))
		switch sortParam {
		case "id":
			sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].ID) < strings.ToLower(out[j].ID) })
		default: // name
			sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
		}
		writeJSON(w, out)
	})

	// GET /api/{faction}/units  (faction is faction_id, e.g., AC, ORK)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/api/")
		parts := strings.Split(strings.Trim(p, "/"), "/")
		if len(parts) == 0 || parts[0] == "" {
			writeError(w, http.StatusNotFound, "missing faction segment")
			return
		}
		requested := parts[0]
		// resolve faction by id or slug
		factionRec, ok := store.FactionsBySlug[strings.ToLower(requested)]
		if !ok {
			writeError(w, http.StatusNotFound, "unknown faction: "+requested)
			return
		}
		faction := factionRec.ID
		if len(parts) == 1 {
			writeError(w, http.StatusNotFound, "missing units or unit_id")
			return
		}
		switch parts[1] {
		case "units":
			units := store.UnitsByFac[faction]
			q := r.URL.Query()
			// filter by role
			if role := q.Get("role"); role != "" {
				roleLow := strings.ToLower(role)
				filtered := units[:0]
				for _, u := range units {
					if strings.ToLower(u.Role) == roleLow {
						filtered = append(filtered, u)
					}
				}
				units = filtered
			}
			// search by name contains
			if s := q.Get("q"); s != "" {
				needle := strings.ToLower(s)
				filtered := units[:0]
				for _, u := range units {
					if strings.Contains(strings.ToLower(u.Name), needle) {
						filtered = append(filtered, u)
					}
				}
				units = filtered
			}
			// sort param: name|id (default name)
			switch strings.ToLower(q.Get("sort")) {
			case "id":
				sort.Slice(units, func(i, j int) bool { return units[i].ID < units[j].ID })
			default:
				sort.Slice(units, func(i, j int) bool { return strings.ToLower(units[i].Name) < strings.ToLower(units[j].Name) })
			}
			// Enrich with basic T and W from first model row if present
			enriched := make([]Unit, len(units))
			for i, u := range units {
				eu := u
				if models, ok := store.ModelsByDS[u.ID]; ok && len(models) > 0 {
					eu.T = strings.TrimSpace(models[0].T)
					eu.W = strings.TrimSpace(models[0].W)
				}
				// add points: choose the minimum cost entry if multiple
				if costs, ok := store.CostsByDS[u.ID]; ok && len(costs) > 0 {
					min := -1
					for _, c := range costs {
						n, err := strconv.Atoi(strings.TrimSpace(c.Cost))
						if err != nil { continue }
						if n <= 0 { continue }
						if min < 0 || n < min {
							min = n
						}
					}
					if min > 0 {
						eu.Points = strconv.Itoa(min)
					}
				}
				enriched[i] = eu
			}
			units = enriched

			// pagination: limit, offset
			limit, _ := strconv.Atoi(q.Get("limit"))
			offset, _ := strconv.Atoi(q.Get("offset"))
			if offset < 0 {
				offset = 0
			}
			if limit <= 0 || limit > 500 {
				limit = len(units)
			}
			end := offset + limit
			if offset > len(units) {
				offset = len(units)
			}
			if end > len(units) {
				end = len(units)
			}
			writeJSON(w, units[offset:end])
			return
		default:
			// Expect: /api/{faction}/{unit_id}/... endpoints
			if len(parts) >= 2 {
				unitID := parts[1]
				if len(parts) == 2 {
					// unit data by id
					if u, ok := store.UnitsByID[unitID]; ok {
						// enrich same as in list
						eu := u
						if models, ok := store.ModelsByDS[u.ID]; ok && len(models) > 0 {
							eu.T = strings.TrimSpace(models[0].T)
							eu.W = strings.TrimSpace(models[0].W)
						}
						if costs, ok := store.CostsByDS[u.ID]; ok && len(costs) > 0 {
							min := -1
							for _, c := range costs {
								n, err := strconv.Atoi(strings.TrimSpace(c.Cost))
								if err != nil { continue }
								if n <= 0 { continue }
								if min < 0 || n < min { min = n }
							}
							if min > 0 { eu.Points = strconv.Itoa(min) }
						}
						writeJSON(w, eu)
						return
					}
					writeError(w, http.StatusNotFound, "unit not found: "+unitID)
					return
				}
				if len(parts) == 3 {
					switch parts[2] {
					case "weapons":
						{
							list := store.WeaponsByDS[unitID]
							if list == nil { list = []Weapon{} }
							writeJSON(w, list)
						}
						return
					case "models":
						{
							list := store.ModelsByDS[unitID]
							if list == nil { list = []Model{} }
							writeJSON(w, list)
						}
						return
					case "keywords":
						{
							list := store.KeywordsByDS[unitID]
							if list == nil { list = []Keyword{} }
							writeJSON(w, list)
						}
						return
					case "abilities":
						{
							list := store.AbilitiesByDS[unitID]
							if list == nil { list = []Ability{} }
							writeJSON(w, list)
						}
						return
					case "options":
						{
							list := store.OptionsByDS[unitID]
							if list == nil { list = []Option{} }
							writeJSON(w, list)
						}
						return
					case "costs":
						{
							list := store.CostsByDS[unitID]
							if list == nil { list = []ModelCost{} }
							writeJSON(w, list)
						}
						return
					}
				}
			}
			writeError(w, http.StatusNotFound, "unsupported path")
		}
	})

	// Prefer Cloud Run's PORT env var when present
	port := os.Getenv("PORT")
	if port == "" {
		port = getenv("API_PORT", "8080")
	}
	addr := ":" + port
	fmt.Printf("W40K API listening on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, withCORS(mux)))
}
