package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Faction API response structure
type FactionResponse struct {
	Name string `json:"name"`
	File string `json:"file"`
}

// API Unit structure from the W40K API
type APIUnit struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Type     string       `json:"type"`
	Profiles []APIProfile `json:"profiles"`
	Entries  []APIEntry   `json:"entries"`
	Costs    []APICost    `json:"costs"`
}

type APIProfile struct {
	Name  string    `json:"name"`
	Type  string    `json:"type"`
	Lines []APILine `json:"lines"`
}

type APILine struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type APIEntry struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Type     string       `json:"type"`
	Profiles []APIProfile `json:"profiles"`
}

type APICost struct {
	Name   string `json:"name"`
	TypeID string `json:"typeid"`
	Value  string `json:"value"`
}

// Enhanced types for optimization (based on w40k-api repository)
type OptimizedFactionData struct {
	FactionName string           `json:"factionname"`
	Units       []Unit           `json:"units"`
	UnitIndex   map[string]*Unit `json:"-"` // O(1) lookup index - don't serialize
}

// OptimizedCacheEntry stores cached faction data with timestamp
type OptimizedCacheEntry struct {
	Data      *OptimizedFactionData
	LoadedAt  time.Time
	FileStats os.FileInfo
}

// OptimizedCache stores all faction data in memory for fast access
type OptimizedCache struct {
	factions map[string]*OptimizedCacheEntry
	mutex    sync.RWMutex
	ttl      time.Duration

	// Statistics tracking
	cacheHits       int64
	cacheMisses     int64
	totalRequests   int64
	startTime       time.Time
	lastCacheUpdate time.Time
	statsMutex      sync.RWMutex
}

// Global optimized cache instance
var (
	optimizedCache     *OptimizedCache
	optimizedCacheOnce sync.Once
)

// API functions
func fetchFactions() ([]string, error) {
	// Read faction names from local JSON files
	files, err := filepath.Glob("static/json/*.json")
	if err != nil {
		log.Printf("Error reading local faction files: %v", err)
		// Fallback to external API
		return fetchFactionsFromAPI()
	}

	if len(files) == 0 {
		log.Println("No local faction files found, using external API")
		return fetchFactionsFromAPI()
	}

	// Extract faction names from filenames
	factionNames := make([]string, 0, len(files))
	for _, file := range files {
		filename := filepath.Base(file)
		factionName := strings.TrimSuffix(filename, ".json")
		// Convert filename back to display name format
		displayName := strings.ReplaceAll(factionName, "-", " ")
		displayName = strings.Title(displayName)
		factionNames = append(factionNames, displayName)
	}

	log.Printf("✅ Found %d local factions", len(factionNames))
	return factionNames, nil
}

// Fallback function for external API
func fetchFactionsFromAPI() ([]string, error) {
	resp, err := http.Get("https://w40k-api-eu-85079828466.europe-west1.run.app/factions")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var factionResponses []FactionResponse
	err = json.NewDecoder(resp.Body).Decode(&factionResponses)
	if err != nil {
		return nil, err
	}

	// Extract just the names for the frontend
	factionNames := make([]string, len(factionResponses))
	for i, faction := range factionResponses {
		factionNames[i] = faction.Name
	}

	return factionNames, nil
}

func fetchFactionUnits(faction string) ([]Unit, error) {
	// Try local JSON files first
	units, err := fetchFactionUnitsFromLocal(faction)
	if err == nil {
		log.Printf("✅ Retrieved %d units from local files for faction %s", len(units), faction)
		return units, nil
	}
	log.Printf("⚠️  Local file miss, falling back to external API for faction %s: %v", faction, err)

	// Fallback to external API
	return fetchFactionUnitsFromAPI(faction)
}

// Cache-aware version for game usage
func fetchFactionUnitsCached(faction string) ([]Unit, error) {
	// Try optimized cache first for better performance
	if optimizedCache != nil {
		factionData, err := getCachedFactionOptimized(faction)
		if err == nil {
			log.Printf("✅ Retrieved %d units from optimized cache for faction %s", len(factionData.Units), faction)
			return factionData.Units, nil
		}
		log.Printf("⚠️  Cache miss, loading faction %s", faction)
	}

	// If cache fails, use standard fetch (which handles local + API)
	return fetchFactionUnits(faction)
}

func fetchFactionUnitsFromLocal(faction string) ([]Unit, error) {
	// Convert display name to filename format
	factionFile := strings.ToLower(strings.ReplaceAll(faction, " ", "-"))
	filePath := filepath.Join("static/json", factionFile+".json")

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("local faction file not found: %s", filePath)
	}

	// Read the JSON file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read faction file: %v", err)
	}

	// Parse the JSON using the same structure as parser.go
	var factionData struct {
		FactionName string `json:"factionname"`
		Units       []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Type     string `json:"type"`
			Profiles []struct {
				Name  string `json:"name"`
				Type  string `json:"type"`
				Lines []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:"lines"`
			} `json:"profiles"`
			Costs []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"costs"`
			Weapons []struct {
				Name  string `json:"name"`
				Type  string `json:"type"`
				Lines []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:"lines"`
			} `json:"weapons"`
		} `json:"units"`
	}

	if err := json.Unmarshal(data, &factionData); err != nil {
		return nil, fmt.Errorf("failed to parse faction data: %v", err)
	}

	// Extract all faction weapons first
	factionWeapons := extractAllFactionWeapons(factionFile)
	log.Printf("📦 Loaded %d weapons for faction %s", len(factionWeapons), faction)

	// Convert to our Unit structure with filtering
	units := make([]Unit, 0)
	unitCount := 0
	maxUnits := 52
	processedUnits := make(map[string]bool)

	for _, localUnit := range factionData.Units {
		if unitCount >= maxUnits {
			break
		}

		// Filter similar to the API version
		if localUnit.Type != "unit" && localUnit.Type != "model" {
			continue
		}

		// Check for duplicates
		unitKey := strings.ToLower(strings.TrimSpace(localUnit.Name))
		if processedUnits[unitKey] {
			log.Printf("🔄 Skipping duplicate unit: %s", localUnit.Name)
			continue
		}

		unit := Unit{
			Name:      localUnit.Name,
			Wounds:    "1",
			Attacks:   "1",
			Strength:  "4",
			Toughness: "4",
			Weapons:   convertWeaponProfilesToWeapons(localUnit.Weapons), // Use unit's stored weapons
		}

		// Extract unit stats from profiles
		for _, profile := range localUnit.Profiles {
			if profile.Type == "Unit" {
				for _, line := range profile.Lines {
					switch line.Name {
					case "W":
						unit.Wounds = line.Value
					case "A", "Attacks":
						unit.Attacks = line.Value
					case "S", "Strength":
						unit.Strength = line.Value
					case "T", "Toughness":
						unit.Toughness = line.Value
					case "WS":
						unit.WS = line.Value
					case "BS":
						unit.BS = line.Value
					case "M", "Movement":
						unit.Movement = line.Value
					case "SV", "Save":
						unit.Save = line.Value
					case "LD", "Leadership":
						unit.Leadership = line.Value
					}
				}
			}
		}

		units = append(units, unit)
		processedUnits[unitKey] = true
		unitCount++
	}

	return units, nil
}

func fetchFactionUnitsFromAPI(faction string) ([]Unit, error) {
	// Original implementation as fallback
	resp, err := http.Get(fmt.Sprintf("https://w40k-api-eu-85079828466.europe-west1.run.app/faction/%s", faction))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Define the structure for the API response
	var apiResponse struct {
		FactionName string    `json:"factionname"`
		Units       []APIUnit `json:"units"`
	}

	err = json.NewDecoder(resp.Body).Decode(&apiResponse)
	if err != nil {
		log.Printf("Error decoding API response for faction %s: %v", faction, err)
		// If API parsing fails, return basic units
		basicUnits := []Unit{
			{Name: "Tactical Squad", Wounds: "1", Attacks: "1", Strength: "4", Toughness: "4"},
			{Name: "Assault Marines", Wounds: "1", Attacks: "2", Strength: "4", Toughness: "4"},
			{Name: "Devastator Squad", Wounds: "1", Attacks: "1", Strength: "4", Toughness: "4"},
		}
		return basicUnits, nil
	}

	// Convert API units to our Unit structure with filtering
	units := make([]Unit, 0)
	unitCount := 0
	maxUnits := 52                          // Increase slightly to accommodate the exact Necron list
	processedUnits := make(map[string]bool) // Track processed units to avoid duplicates

	for _, apiUnit := range apiResponse.Units {
		// Skip if we've reached the unit limit
		if unitCount >= maxUnits {
			break
		}

		// Filter out unwanted unit types (pass faction info)
		if shouldSkipUnit(apiUnit, faction) {
			continue
		}

		// Check for duplicate units
		unitKey := strings.ToLower(strings.TrimSpace(apiUnit.Name))
		if processedUnits[unitKey] {
			log.Printf("🔄 Skipping duplicate unit: %s", apiUnit.Name)
			continue
		}

		if apiUnit.Type == "unit" || apiUnit.Type == "model" {
			// Extract weapons specific to this unit
			unitWeapons := extractWeaponsFromUnit(apiUnit)

			// Log units with no weapons for debugging
			if len(unitWeapons) == 0 {
				log.Printf("⚠️  Unit %s has no weapons found", apiUnit.Name)
			}

			unit := Unit{
				Name:      apiUnit.Name,
				Wounds:    "1",
				Attacks:   "1",
				Strength:  "4",
				Toughness: "4",
				Weapons:   unitWeapons, // Assign only this unit's specific weapons
			}

			// Extract stats from profiles
			for _, profile := range apiUnit.Profiles {
				if profile.Type == "Unit" {
					for _, line := range profile.Lines {
						switch line.Name {
						case "W":
							unit.Wounds = line.Value
						case "A", "Attacks":
							unit.Attacks = line.Value
						case "S", "Strength":
							unit.Strength = line.Value
						case "T", "Toughness":
							unit.Toughness = line.Value
						case "WS":
							unit.WS = line.Value
						case "BS":
							unit.BS = line.Value
						case "M", "Movement":
							unit.Movement = line.Value
						case "SV", "Save":
							unit.Save = line.Value
						case "LD", "Leadership":
							unit.Leadership = line.Value
						}
					}
				}
			}

			units = append(units, unit)
			processedUnits[unitKey] = true // Mark this unit as processed
			unitCount++
		}
	}

	log.Printf("Fetched %d units for faction %s (filtered from %d total)", len(units), faction, len(apiResponse.Units))
	return units, nil
}

// Initialize optimized cache system (based on w40k-api optimizations)
func initOptimizedCache() {
	optimizedCache = &OptimizedCache{
		factions:        make(map[string]*OptimizedCacheEntry),
		ttl:             5 * time.Minute, // 5-minute TTL
		startTime:       time.Now(),
		lastCacheUpdate: time.Now(),
	}
	log.Printf("🚀 Optimized cache system initialized")
}

// Get faction data from cache or load it
func getCachedFactionOptimized(faction string) (*OptimizedFactionData, error) {
	optimizedCacheOnce.Do(initOptimizedCache)

	optimizedCache.statsMutex.Lock()
	optimizedCache.totalRequests++
	optimizedCache.statsMutex.Unlock()

	optimizedCache.mutex.RLock()
	entry, exists := optimizedCache.factions[faction]
	optimizedCache.mutex.RUnlock()

	// Check if we have valid cached data
	if exists && time.Since(entry.LoadedAt) < optimizedCache.ttl {
		optimizedCache.statsMutex.Lock()
		optimizedCache.cacheHits++
		optimizedCache.statsMutex.Unlock()
		return entry.Data, nil
	}

	// Cache miss - load from API
	optimizedCache.statsMutex.Lock()
	optimizedCache.cacheMisses++
	optimizedCache.statsMutex.Unlock()

	log.Printf("💾 Loading faction %s into cache", faction)

	// Load units using the standard fetch function (which handles local files + API fallback)
	units, err := fetchFactionUnits(faction)
	if err != nil {
		return nil, err
	}

	// Create optimized faction data with unit index
	factionData := &OptimizedFactionData{
		FactionName: faction,
		Units:       units,
		UnitIndex:   make(map[string]*Unit),
	}

	// Build unit index for O(1) lookups
	buildUnitIndexOptimized(factionData)

	// Store in cache
	cacheEntry := &OptimizedCacheEntry{
		Data:     factionData,
		LoadedAt: time.Now(),
	}

	optimizedCache.mutex.Lock()
	optimizedCache.factions[faction] = cacheEntry
	optimizedCache.lastCacheUpdate = time.Now()
	optimizedCache.mutex.Unlock()

	log.Printf("✅ Cached faction %s with %d units (%d indexed)", faction, len(units), len(factionData.UnitIndex))
	return factionData, nil
}

// Build unit lookup index for fast O(1) searches
func buildUnitIndexOptimized(factionData *OptimizedFactionData) {
	count := 0

	indexUnits := func(units []Unit) {
		for i := range units {
			unit := &units[i]
			count++

			// Index by exact name
			factionData.UnitIndex[unit.Name] = unit

			// Index by normalized name (lowercase, spaces to dashes)
			normalizedName := strings.ToLower(strings.ReplaceAll(unit.Name, " ", "-"))
			factionData.UnitIndex[normalizedName] = unit

			// Index by lowercase name for case-insensitive lookup
			lowerName := strings.ToLower(unit.Name)
			factionData.UnitIndex[lowerName] = unit

			// Index nested units if they exist (from current structure)
			// Note: Current Unit structure doesn't have nested entries like the API,
			// but we'll keep this for future compatibility
		}
	}

	indexUnits(factionData.Units)
	log.Printf("🔍 Indexed %d units for faction %s", count, factionData.FactionName)
}

// Fast unit lookup using optimized index
func findUnitInIndexOptimized(factionData *OptimizedFactionData, unitName string) *Unit {
	// Try exact match first
	if unit, exists := factionData.UnitIndex[unitName]; exists {
		return unit
	}

	// Try normalized name (spaces to dashes)
	normalizedName := strings.ToLower(strings.ReplaceAll(unitName, " ", "-"))
	if unit, exists := factionData.UnitIndex[normalizedName]; exists {
		return unit
	}

	// Try case-insensitive search
	lowerUnitName := strings.ToLower(unitName)
	if unit, exists := factionData.UnitIndex[lowerUnitName]; exists {
		return unit
	}

	return nil
}

// Optimized faction units retrieval using cache
func fetchFactionUnitsOptimized(faction string) ([]Unit, error) {
	factionData, err := getCachedFactionOptimized(faction)
	if err != nil {
		return nil, err
	}
	return factionData.Units, nil
}

// Get cache statistics for monitoring
func getCacheStatsOptimized() map[string]interface{} {
	if optimizedCache == nil {
		return map[string]interface{}{
			"status": "not_initialized",
		}
	}

	optimizedCache.statsMutex.RLock()
	optimizedCache.mutex.RLock()

	stats := map[string]interface{}{
		"status":          "healthy",
		"cached_factions": len(optimizedCache.factions),
		"total_requests":  optimizedCache.totalRequests,
		"cache_hits":      optimizedCache.cacheHits,
		"cache_misses":    optimizedCache.cacheMisses,
		"hit_rate":        float64(optimizedCache.cacheHits) / float64(optimizedCache.totalRequests) * 100,
		"uptime":          time.Since(optimizedCache.startTime).String(),
		"last_update":     optimizedCache.lastCacheUpdate.Format(time.RFC3339),
		"cache_ttl":       optimizedCache.ttl.String(),
	}

	optimizedCache.mutex.RUnlock()
	optimizedCache.statsMutex.RUnlock()

	return stats
}

// Optimized weapon collection with deduplication (from w40k-api)
func collectWeaponsByTypeOptimized(unit Unit, weaponType string) []Weapon {
	weapons := make([]Weapon, 0, 10) // Pre-allocate reasonable capacity
	seen := make(map[string]bool)    // Deduplicate weapons

	// Collect from unit's direct weapons
	for _, weapon := range unit.Weapons {
		if weapon.Type == weaponType {
			key := weapon.Name + "|" + weapon.Type
			if !seen[key] {
				weapons = append(weapons, weapon)
				seen[key] = true
			}
		}
	}

	return weapons
}

// Enhanced unit lookup with caching
func findUnitOptimized(faction, unitName string) *Unit {
	factionData, err := getCachedFactionOptimized(faction)
	if err != nil {
		log.Printf("❌ Error loading faction %s: %v", faction, err)
		return nil
	}

	unit := findUnitInIndexOptimized(factionData, unitName)
	if unit == nil {
		log.Printf("❌ Unit %s not found in faction %s", unitName, faction)
	}
	return unit
}

// Get ranged weapons using optimized system
func getRangedWeaponsOptimized(faction, unitName string) []Weapon {
	unit := findUnitOptimized(faction, unitName)
	if unit == nil {
		return []Weapon{}
	}
	return collectWeaponsByTypeOptimized(*unit, "Ranged")
}

// Get melee weapons using optimized system
func getMeleeWeaponsOptimized(faction, unitName string) []Weapon {
	unit := findUnitOptimized(faction, unitName)
	if unit == nil {
		return []Weapon{}
	}
	return collectWeaponsByTypeOptimized(*unit, "Melee")
}

// Helper function to determine if a unit should be skipped based on filtering rules
func shouldSkipUnit(apiUnit APIUnit, faction string) bool {
	unitName := strings.ToLower(strings.TrimSpace(apiUnit.Name))

	// Skip upgrade/weapon/enhancement units
	if apiUnit.Type == "upgrade" || apiUnit.Type == "weapon" || apiUnit.Type == "enhancement" {
		return true
	}

	// Skip units containing "legends" (case insensitive)
	if strings.Contains(unitName, "legends") {
		return true
	}

	// For Necrons specifically, use the whitelist
	if strings.Contains(strings.ToLower(faction), "necron") {
		return !isValidNecronUnit(unitName)
	}

	// For other factions, apply general filtering rules
	skipKeywords := []string{"upgrade", "weapon", "enhancement", "relic", "warlord trait"}
	for _, keyword := range skipKeywords {
		if strings.Contains(unitName, keyword) {
			return true
		}
	}

	return false
}

// isValidNecronUnit checks if a unit name is in the official Necron unit list
func isValidNecronUnit(unitName string) bool {
	validNecronUnits := map[string]bool{
		"C'tan Shard Of The Deceiver":        true,
		"C'tan Shard Of The Nightbringer":    true,
		"C'tan Shard Of The Void Dragon":     true,
		"Catacomb Command Barge":             true,
		"Chronomancer":                       true,
		"Hexmark Destroyer":                  true,
		"Illuminor Szeras":                   true,
		"Imotekh The Stormlord":              true,
		"Lokhust Lord":                       true,
		"Orikan The Diviner":                 true,
		"Overlord":                           true,
		"Overlord with Translocation Shroud": true,
		"Plasmancer":                         true,
		"Psychomancer":                       true,
		"Royal Warden":                       true,
		"Skorpekh Lord":                      true,
		"Technomancer":                       true,
		"The Silent King":                    true,
		"Transcendent C'tan":                 true,
		"Trazyn The Infinite":                true,
		"Anrakyr The Traveller":              true,
		"Lord":                               true,
		"Nemesor Zahndrekh":                  true,
		"Vargard Obyron":                     true,
		"Immortals":                          true,
		"Necron Warriors":                    true,
		"Ghost Ark":                          true,
		"Convergence Of Dominion":            true,
		"Cryptothralls":                      true,
		"Deathmarks":                         true,
		"Flayed Ones":                        true,
		"Lychguard":                          true,
		"Ophydian Destroyers":                true,
		"Skorpekh Destroyers":                true,
		"Triarch Praetorians":                true,
		"Triarch Stalker":                    true,
		"Annihilation Barge":                 true,
		"Canoptek Doomstalker":               true,
		"Canoptek Reanimator":                true,
		"Canoptek Scarab Swarms":             true,
		"Canoptek Spyders":                   true,
		"Canoptek Wraiths":                   true,
		"Doom Scythe":                        true,
		"Doomsday Ark":                       true,
		"Lokhust Destroyers":                 true,
		"Lokhust Heavy Destroyers":           true,
		"Monolith":                           true,
		"Night Scythe":                       true,
		"Obelisk":                            true,
		"Tesseract Vault":                    true,
		"Tomb Blades":                        true,
		"Seraptek Heavy Construct":           true,
	}

	// First check exact match
	if validNecronUnits[unitName] {
		log.Printf("✅ Necron unit ACCEPTED (exact): %s", unitName)
		return true
	}

	// Also try case-insensitive match
	unitNameLower := strings.ToLower(unitName)
	for validUnit := range validNecronUnits {
		if strings.ToLower(validUnit) == unitNameLower {
			log.Printf("✅ Necron unit ACCEPTED (case-insensitive): %s", unitName)
			return true
		}
	}

	log.Printf("❌ Necron unit REJECTED: %s", unitName)
	return false
}

// containsIgnoreCase checks if string contains substring (case insensitive)
func containsIgnoreCase(str, substr string) bool {
	return strings.Contains(strings.ToLower(str), strings.ToLower(substr))
}

// extractFactionWeapons extracts all weapons from upgrade-type units in a faction
func extractFactionWeapons(apiUnits []APIUnit) []Weapon {
	weapons := make([]Weapon, 0)

	log.Printf("extractFactionWeapons called with %d units", len(apiUnits))

	for _, apiUnit := range apiUnits {
		if apiUnit.Type == "upgrade" {
			unitWeapons := extractWeaponsFromUnit(apiUnit)
			weapons = append(weapons, unitWeapons...)
		}
	}

	log.Printf("extractFactionWeapons returning %d weapons", len(weapons))
	return weapons
}

func extractWeaponsFromUnit(apiUnit APIUnit) []Weapon {
	weaponMap := make(map[string]Weapon) // Use map for deduplication

	log.Printf("Extracting weapons from unit: %s (type: %s)", apiUnit.Name, apiUnit.Type)

	// Check profiles for weapon profiles
	for _, profile := range apiUnit.Profiles {
		log.Printf("  Profile: %s (type: %s)", profile.Name, profile.Type)
		if profile.Type == "Ranged Weapons" || profile.Type == "Melee Weapons" {
			weapon := Weapon{
				Name: profile.Name,
				Type: getWeaponType(profile.Type),
			}

			for _, line := range profile.Lines {
				switch line.Name {
				case "A", "Attacks":
					weapon.Attacks = line.Value
				case "BS", "WS":
					weapon.Skill = line.Value
				case "S", "Strength":
					weapon.Strength = line.Value
				case "AP":
					weapon.AP = line.Value
				case "D", "Damage":
					weapon.Damage = line.Value
				case "Range":
					weapon.Range = line.Value
				}
			}

			if weapon.Name != "" {
				key := weapon.Name + "|" + weapon.Type
				weaponMap[key] = weapon
				log.Printf("    Found weapon: %s (%s)", weapon.Name, weapon.Type)
			}
		}
	}

	// Check entries for weapon entries
	for _, entry := range apiUnit.Entries {
		log.Printf("  Entry: %s", entry.Name)
		for _, profile := range entry.Profiles {
			log.Printf("    Entry Profile: %s (type: %s)", profile.Name, profile.Type)
			if profile.Type == "Ranged Weapons" || profile.Type == "Melee Weapons" {
				weapon := Weapon{
					Name: profile.Name,
					Type: getWeaponType(profile.Type),
				}

				for _, line := range profile.Lines {
					switch line.Name {
					case "A", "Attacks":
						weapon.Attacks = line.Value
					case "BS", "WS":
						weapon.Skill = line.Value
					case "S", "Strength":
						weapon.Strength = line.Value
					case "AP":
						weapon.AP = line.Value
					case "D", "Damage":
						weapon.Damage = line.Value
					case "Range":
						weapon.Range = line.Value
					}
				}

				if weapon.Name != "" {
					key := weapon.Name + "|" + weapon.Type
					weaponMap[key] = weapon
					log.Printf("      Found weapon: %s (%s)", weapon.Name, weapon.Type)
				}
			}
		}
	}

	// Convert map to slice
	weapons := make([]Weapon, 0, len(weaponMap))
	for _, weapon := range weaponMap {
		weapons = append(weapons, weapon)
	}

	log.Printf("Total unique weapons found for %s: %d", apiUnit.Name, len(weapons))
	return weapons
}

func getWeaponType(profileType string) string {
	if profileType == "Ranged Weapons" {
		return "Ranged"
	} else if profileType == "Melee Weapons" {
		return "Melee"
	}
	return "Unknown"
}

func extractAllFactionWeapons(faction string) []Weapon {
	fileName := fmt.Sprintf("static/json/%s.json", faction)
	file, err := os.Open(fileName)
	if err != nil {
		log.Printf("❌ Could not open faction file %s: %v", fileName, err)
		return []Weapon{}
	}
	defer file.Close()

	var data struct {
		FactionName string    `json:"factionname"`
		Units       []APIUnit `json:"units"`
	}

	if err := json.NewDecoder(file).Decode(&data); err != nil {
		log.Printf("❌ Could not decode faction file %s: %v", fileName, err)
		return []Weapon{}
	}

	weaponMap := make(map[string]Weapon) // Use map to deduplicate by name

	// Extract weapons from all units in the faction
	for _, apiUnit := range data.Units {
		// Look for weapon entries (type upgrade with weapon profiles)
		if apiUnit.Type == "upgrade" {
			for _, profile := range apiUnit.Profiles {
				if profile.Type == "Ranged Weapons" || profile.Type == "Melee Weapons" {
					weapon := Weapon{
						Name: profile.Name,
						Type: getWeaponType(profile.Type),
					}

					for _, line := range profile.Lines {
						switch line.Name {
						case "A", "Attacks":
							weapon.Attacks = line.Value
						case "BS", "WS":
							weapon.Skill = line.Value
						case "S", "Strength":
							weapon.Strength = line.Value
						case "AP":
							weapon.AP = line.Value
						case "D", "Damage":
							weapon.Damage = line.Value
						case "Range":
							weapon.Range = line.Value
						}
					}

					if weapon.Name != "" {
						// Use weapon name as key to deduplicate
						weaponMap[weapon.Name] = weapon
					}
				}
			}
		}

		// Also extract from unit profiles and entries (but still deduplicate)
		unitWeapons := extractWeaponsFromUnit(apiUnit)
		for _, weapon := range unitWeapons {
			if weapon.Name != "" {
				weaponMap[weapon.Name] = weapon
			}
		}
	}

	// Convert map back to slice
	weapons := make([]Weapon, 0, len(weaponMap))
	for _, weapon := range weaponMap {
		weapons = append(weapons, weapon)
	}

	log.Printf("📦 Extracted %d unique weapons from faction %s", len(weapons), faction)
	return weapons
}

func getFactionWeaponsForUnit(factionWeapons []Weapon, unitName string) []Weapon {
	// Create a map to ensure uniqueness by weapon name and type combination
	uniqueWeapons := make(map[string]Weapon)

	for _, weapon := range factionWeapons {
		if weapon.Name != "" {
			// Use name + type as key to ensure complete uniqueness
			key := weapon.Name + "|" + weapon.Type
			uniqueWeapons[key] = weapon
		}
	}

	// Convert back to slice
	result := make([]Weapon, 0, len(uniqueWeapons))
	for _, weapon := range uniqueWeapons {
		result = append(result, weapon)
	}

	log.Printf("🔧 Filtered %d unique weapons for unit %s", len(result), unitName)
	return result
}

func getUnitWeaponsByType(unit Unit, weaponType string) []Weapon {
	var filteredWeapons []Weapon
	for _, weapon := range unit.Weapons {
		if weapon.Type == weaponType {
			filteredWeapons = append(filteredWeapons, weapon)
		}
	}
	return filteredWeapons
}

func getUnitWeaponTypes(unit Unit) []string {
	typeMap := make(map[string]bool)
	for _, weapon := range unit.Weapons {
		if weapon.Type != "" && weapon.Type != "Unknown" {
			typeMap[weapon.Type] = true
		}
	}

	types := make([]string, 0, len(typeMap))
	for weaponType := range typeMap {
		types = append(types, weaponType)
	}
	return types
}

func getUnitWeaponNames(unit Unit) []string {
	nameMap := make(map[string]bool)
	for _, weapon := range unit.Weapons {
		if weapon.Name != "" {
			nameMap[weapon.Name] = true
		}
	}

	names := make([]string, 0, len(nameMap))
	for weaponName := range nameMap {
		names = append(names, weaponName)
	}
	return names
}

func getUnitWeaponsCategorized(unit Unit) map[string][]Weapon {
	meleeWeapons := make(map[string]Weapon)
	rangedWeapons := make(map[string]Weapon)

	for _, weapon := range unit.Weapons {
		if weapon.Name != "" {
			if weapon.Type == "Melee" {
				meleeWeapons[weapon.Name] = weapon
			} else if weapon.Type == "Ranged" {
				rangedWeapons[weapon.Name] = weapon
			}
		}
	}

	melee := make([]Weapon, 0, len(meleeWeapons))
	for _, weapon := range meleeWeapons {
		melee = append(melee, weapon)
	}

	ranged := make([]Weapon, 0, len(rangedWeapons))
	for _, weapon := range rangedWeapons {
		ranged = append(ranged, weapon)
	}

	return map[string][]Weapon{
		"melee":  melee,
		"ranged": ranged,
	}
}

// convertWeaponProfilesToWeapons converts weapon profiles from JSON to Weapon structs
func convertWeaponProfilesToWeapons(weaponProfiles []struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Lines []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"lines"`
}) []Weapon {
	weapons := make([]Weapon, 0, len(weaponProfiles))

	for _, profile := range weaponProfiles {
		weapon := Weapon{
			Name: profile.Name,
			Type: getWeaponType(profile.Type),
		}

		for _, line := range profile.Lines {
			switch line.Name {
			case "A", "Attacks":
				weapon.Attacks = line.Value
			case "BS", "WS":
				weapon.Skill = line.Value
			case "S", "Strength":
				weapon.Strength = line.Value
			case "AP":
				weapon.AP = line.Value
			case "D", "Damage":
				weapon.Damage = line.Value
			case "Range":
				weapon.Range = line.Value
			}
		}

		if weapon.Name != "" {
			weapons = append(weapons, weapon)
		}
	}

	return weapons
}
