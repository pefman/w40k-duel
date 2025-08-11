package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var httpClient = &http.Client{Timeout: 8 * time.Second}

// Simple cache for faction list to reduce redundant API calls
var (
	factionCache      []apiFaction
	factionCacheTime  time.Time
	factionCacheTTL   = 5 * time.Minute
	factionCacheMutex sync.RWMutex
)

// Config holds API configuration
type Config struct {
	BaseURL string
}

type Client struct {
	config Config
}

func NewClient(baseURL string) *Client {
	return &Client{
		config: Config{BaseURL: baseURL},
	}
}

func (c *Client) apiGet(path string, out interface{}) error {
	base := strings.TrimRight(c.config.BaseURL, "/")
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

// Domain models for API responses
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

// API response types
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

func (c *Client) FetchFactions() ([]apiFaction, error) {
	// Check cache first
	factionCacheMutex.RLock()
	if time.Since(factionCacheTime) < factionCacheTTL && len(factionCache) > 0 {
		result := make([]apiFaction, len(factionCache))
		copy(result, factionCache)
		factionCacheMutex.RUnlock()
		return result, nil
	}
	factionCacheMutex.RUnlock()

	// Fetch from API
	var res []apiFaction
	if err := c.apiGet("/api/factions", &res); err != nil {
		return nil, err
	}

	// Update cache
	factionCacheMutex.Lock()
	factionCache = make([]apiFaction, len(res))
	copy(factionCache, res)
	factionCacheTime = time.Now()
	factionCacheMutex.Unlock()

	return res, nil
}

func toSlug(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "'", "")
	s = strings.ReplaceAll(s, "'", "")
	s = strings.ReplaceAll(s, "&", "and")
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "--", "-")
	return s
}

// FetchUnits builds gameplay-ready units for a faction name by calling the data API.
func (c *Client) FetchUnits(factionName string) ([]Unit, error) {
	slug := toSlug(factionName)
	// 1) List units for the faction
	var list []apiUnit
	if err := c.apiGet("/api/"+slug+"/units", &list); err != nil {
		return nil, err
	}
	// 2) For each unit, fetch models and weapons to extract basic stats
	out := make([]Unit, 0, len(list))
	for _, u := range list {
		var models []apiModel
		if err := c.apiGet("/api/"+slug+"/"+u.ID+"/models", &models); err != nil {
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
		if err := c.apiGet("/api/"+slug+"/"+u.ID+"/weapons", &apiW); err != nil {
			apiW = nil
		}
		// keywords and abilities
		var apiK []apiKeyword
		_ = c.apiGet("/api/"+slug+"/"+u.ID+"/keywords", &apiK)
		var apiA []apiAbility
		_ = c.apiGet("/api/"+slug+"/"+u.ID+"/abilities", &apiA)
		// Options (valid wargear text lines)
		var apiOpts []struct {
			Line        int    `json:"line"`
			Bullet      string `json:"bullet"`
			Description string `json:"description"`
		}
		_ = c.apiGet("/api/"+slug+"/"+u.ID+"/options", &apiOpts)
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
		_ = c.apiGet("/api/"+slug+"/"+u.ID+"/costs", &costs)
		pts := 0
		if len(costs) > 0 {
			// pick first cost number
			for _, cost := range costs {
				if n := mustAtoi(cost.Cost, 0); n > 0 {
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
		out = append(out, Unit{
			Faction:    factionName,
			Name:       u.Name,
			W:          W,
			T:          T,
			Sv:         Sv,
			InvSv:      inv,
			InvSvDescr: invDescr,
			Keywords:   keywords,
			FNP:        fnp,
			DamageRed:  dr,
			Weapons:    weps,
			DefaultW:   weps[0].Name,
			Options:    opts,
			Points:     pts,
		})
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

func clamp(min, max, v int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
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

// deriveWeaponRules parses weapon rules from API data
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
