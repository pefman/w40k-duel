package main

import (
	"log"
	"math/rand"
	"time"
)

// AI opponent logic
func (gs *GameServer) createAIPlayer(difficulty string) *Player {
	aiID := "ai_" + generatePlayerID()
	aiPlayer := &Player{
		ID:     aiID,
		Name:   generateAIName(difficulty),
		Conn:   nil, // AI doesn't have websocket connection
		Status: "connected",
		IsAI:   true,
	}

	// Store AI player
	gs.mutex.Lock()
	gs.players[aiID] = aiPlayer
	gs.mutex.Unlock()

	log.Printf("AI Player created: %s (difficulty: %s)", aiPlayer.Name, difficulty)
	return aiPlayer
}

func generateAIName(difficulty string) string {
	aiNames := map[string][]string{
		"easy": {
			"Recruit Bot", "Novice Alpha", "Training Servitor", "Basic Protocol",
			"Simple Simon", "Cadet Zero", "Initiate Bot", "Beginner Prime",
		},
		"medium": {
			"Tactical Unit", "Combat Protocol", "War Machine", "Battle Logic",
			"Strategic Mind", "Veteran Algorithm", "Combat Servitor", "War Engine",
		},
		"hard": {
			"Master Tactician", "War Lord Prime", "Ultimate Protocol", "Death Machine",
			"Apex Predator", "Supreme Commander", "Doom Engine", "Perfect Killer",
		},
	}

	names := aiNames[difficulty]
	if len(names) == 0 {
		names = aiNames["medium"] // fallback
	}

	return names[rand.Intn(len(names))]
}

func (gs *GameServer) createAIMatch(humanPlayer *Player, difficulty string) {
	aiPlayer := gs.createAIPlayer(difficulty)
	gs.createMatch(humanPlayer, aiPlayer)

	// Start AI behavior in a goroutine
	go gs.runAIBehavior(aiPlayer, difficulty)
}

func (gs *GameServer) runAIBehavior(aiPlayer *Player, difficulty string) {
	// Wait a bit to simulate "thinking"
	time.Sleep(1 * time.Second)

	// AI selects faction
	factions := gs.getAvailableFactions()
	if len(factions) > 0 {
		selectedFaction := gs.selectAIFaction(factions, difficulty)
		gs.selectFaction(aiPlayer, selectedFaction)

		// Wait a bit then select army
		time.Sleep(2 * time.Second)
		gs.selectAIArmy(aiPlayer, difficulty)
	}
}

func (gs *GameServer) selectAIFaction(factions []string, difficulty string) string {
	// Different difficulties prefer different faction types
	switch difficulty {
	case "easy":
		// Easy AI prefers simpler factions
		preferredFactions := []string{"imperium-astra-militarum", "imperium-adeptus-astartes-space-marines"}
		for _, faction := range preferredFactions {
			for _, available := range factions {
				if faction == available {
					return faction
				}
			}
		}
	case "hard":
		// Hard AI prefers complex/powerful factions
		preferredFactions := []string{"chaos-chaos-space-marines", "xenos-necrons", "imperium-adeptus-custodes"}
		for _, faction := range preferredFactions {
			for _, available := range factions {
				if faction == available {
					return faction
				}
			}
		}
	}

	// Default: random selection
	return factions[rand.Intn(len(factions))]
}

func (gs *GameServer) selectAIArmy(aiPlayer *Player, difficulty string) {
	units, err := fetchFactionUnits(aiPlayer.Faction)
	if err != nil {
		log.Printf("AI couldn't fetch units for faction %s: %v", aiPlayer.Faction, err)
		return
	}

	if len(units) == 0 {
		log.Printf("AI found no units for faction %s", aiPlayer.Faction)
		return
	}

	army := make([]UnitSelection, 0)

	// AI army composition based on difficulty
	maxUnits := gs.getMaxUnitsForDifficulty(difficulty)
	selectedUnits := gs.selectUnitsForAI(units, difficulty, maxUnits)

	for _, unit := range selectedUnits {
		quantity := gs.getUnitQuantityForAI(unit, difficulty)
		log.Printf("DEBUG: Processing unit: %s, available weapons: %d", unit.Name, len(unit.Weapons))

		// Select weapons intelligently based on AI preferences
		selectedWeapons := gs.selectWeaponsForAI(unit.Weapons, difficulty, aiPlayer)
		log.Printf("DEBUG: Selected %d weapons for unit %s: %v", len(selectedWeapons), unit.Name, selectedWeapons)

		unitSelection := UnitSelection{
			UnitName: unit.Name,
			Quantity: quantity,
			Weapons:  selectedWeapons,
		}

		army = append(army, unitSelection)
	}

	log.Printf("DEBUG: Final AI army before selectArmy call:")
	for i, unit := range army {
		log.Printf("  Unit %d: %s (qty: %d), available weapons: %d", i, unit.UnitName, unit.Quantity, len(unit.Weapons))
		for j, weapon := range unit.Weapons {
			log.Printf("    Weapon %d: %s (%s)", j, weapon.Name, weapon.Type)
		}
	}

	// Convert army data to interface{} slice
	armyInterface := make([]interface{}, len(army))
	for i, unit := range army {
		// Convert []Weapon to []interface{} for selectArmy compatibility
		weaponsInterface := make([]interface{}, len(unit.Weapons))
		for j, weapon := range unit.Weapons {
			weaponsInterface[j] = map[string]interface{}{
				"name":     weapon.Name,
				"type":     weapon.Type,
				"range":    weapon.Range,
				"attacks":  weapon.Attacks,
				"skill":    weapon.Skill,
				"strength": weapon.Strength,
				"ap":       weapon.AP,
				"damage":   weapon.Damage,
				"keywords": weapon.Keywords,
			}
		}

		armyInterface[i] = map[string]interface{}{
			"unit_name":        unit.UnitName,
			"quantity":         unit.Quantity,
			"selected_weapons": weaponsInterface,
		}
	}

	gs.selectArmy(aiPlayer, armyInterface)

	// AI is immediately ready to fight
	time.Sleep(1 * time.Second)
	gs.readyToFight(aiPlayer)
}

func (gs *GameServer) getMaxUnitsForDifficulty(difficulty string) int {
	switch difficulty {
	case "easy":
		return 2 // Easy AI uses fewer unit types
	case "medium":
		return 3
	case "hard":
		return 4 // Hard AI uses more diverse armies
	default:
		return 3
	}
}

func (gs *GameServer) selectUnitsForAI(units []Unit, difficulty string, maxUnits int) []Unit {
	// Filter out units with no weapons first
	unitsWithWeapons := make([]Unit, 0)
	for _, unit := range units {
		if len(unit.Weapons) > 0 {
			unitsWithWeapons = append(unitsWithWeapons, unit)
		}
	}

	log.Printf("DEBUG: Filtered units with weapons: %d out of %d total units", len(unitsWithWeapons), len(units))

	if len(unitsWithWeapons) == 0 {
		log.Printf("ERROR: No units with weapons found for AI selection")
		return []Unit{}
	}

	if len(unitsWithWeapons) <= maxUnits {
		return unitsWithWeapons
	}

	selected := make([]Unit, 0, maxUnits)

	switch difficulty {
	case "easy":
		// Easy AI prefers basic units (usually at the beginning of the list)
		for i := 0; i < maxUnits && i < len(unitsWithWeapons); i++ {
			selected = append(selected, unitsWithWeapons[i])
		}
	case "hard":
		// Hard AI prefers powerful units and good combinations
		// For now, select diverse units from different parts of the list
		step := len(unitsWithWeapons) / maxUnits
		for i := 0; i < maxUnits; i++ {
			idx := (i * step) % len(unitsWithWeapons)
			selected = append(selected, unitsWithWeapons[idx])
		}
	default: // medium
		// Medium AI selects randomly
		used := make(map[int]bool)
		for len(selected) < maxUnits {
			idx := rand.Intn(len(unitsWithWeapons))
			if !used[idx] {
				selected = append(selected, unitsWithWeapons[idx])
				used[idx] = true
			}
		}
	}

	return selected
}

func (gs *GameServer) getUnitQuantityForAI(unit Unit, difficulty string) int {
	switch difficulty {
	case "easy":
		return 1 // Easy AI uses single units
	case "medium":
		return rand.Intn(2) + 1 // 1-2 units
	case "hard":
		return rand.Intn(3) + 1 // 1-3 units
	default:
		return 1
	}
}

func (gs *GameServer) handleAIInitiative(aiPlayer *Player) {
	// AI automatically rolls for initiative
	time.Sleep(time.Duration(rand.Intn(2000)+1000) * time.Millisecond) // 1-3 seconds delay
	result := rand.Intn(6) + 1

	// Directly handle the initiative roll instead of calling rollDice
	if match, exists := gs.matches[aiPlayer.MatchID]; exists && match.State == "initiative" {
		gs.handleInitiativeRoll(aiPlayer, result, match)
	}
}

func (gs *GameServer) getAvailableFactions() []string {
	// Return list of available factions
	// This should match the factions available in your static/json directory
	return []string{
		"xenos-necrons",
		"imperium-adeptus-astartes-space-marines",
		"chaos-chaos-space-marines",
		"imperium-astra-militarum",
		"xenos-orks",
		"xenos-tyranids",
		"xenos-tau-empire",
		"imperium-adeptus-custodes",
		"chaos-death-guard",
		"xenos-aeldari",
	}
}

// selectWeaponsForAI intelligently selects weapons for AI based on strategy and difficulty
func (gs *GameServer) selectWeaponsForAI(availableWeapons []Weapon, difficulty string, aiPlayer *Player) []Weapon {
	if len(availableWeapons) == 0 {
		return availableWeapons
	}

	// Find the human player to analyze their weapon preferences
	humanPlayer := gs.getHumanPlayerInMatch(aiPlayer.MatchID)
	if humanPlayer == nil {
		// Fallback: use all weapons if no human player found
		return availableWeapons
	}

	// Analyze human player's weapon preferences
	humanMeleeCount, humanRangedCount := gs.analyzeHumanWeaponPreferences(humanPlayer)

	// AI strategy: match human player's combat style
	preferMelee := humanMeleeCount > humanRangedCount

	var selectedWeapons []Weapon
	meleeWeapons := make([]Weapon, 0)
	rangedWeapons := make([]Weapon, 0)

	// Categorize weapons
	for _, weapon := range availableWeapons {
		if weapon.Type == "Melee" {
			meleeWeapons = append(meleeWeapons, weapon)
		} else if weapon.Type == "Ranged" {
			rangedWeapons = append(rangedWeapons, weapon)
		}
	}

	// Select weapons based on difficulty and strategy
	switch difficulty {
	case "easy":
		// Easy AI: Simple selection, prefer 1-2 weapons per type
		if preferMelee && len(meleeWeapons) > 0 {
			selectedWeapons = append(selectedWeapons, meleeWeapons[0])
			if len(rangedWeapons) > 0 {
				selectedWeapons = append(selectedWeapons, rangedWeapons[0])
			}
		} else if len(rangedWeapons) > 0 {
			selectedWeapons = append(selectedWeapons, rangedWeapons[0])
			if len(meleeWeapons) > 0 {
				selectedWeapons = append(selectedWeapons, meleeWeapons[0])
			}
		}
	case "hard":
		// Hard AI: Use more weapons, better tactical selection
		if preferMelee {
			// Prioritize melee but include some ranged
			selectedWeapons = append(selectedWeapons, meleeWeapons...)
			if len(rangedWeapons) > 0 {
				maxRanged := len(rangedWeapons)/2 + 1
				for i := 0; i < maxRanged && i < len(rangedWeapons); i++ {
					selectedWeapons = append(selectedWeapons, rangedWeapons[i])
				}
			}
		} else {
			// Prioritize ranged but include some melee
			selectedWeapons = append(selectedWeapons, rangedWeapons...)
			if len(meleeWeapons) > 0 {
				maxMelee := len(meleeWeapons)/2 + 1
				for i := 0; i < maxMelee && i < len(meleeWeapons); i++ {
					selectedWeapons = append(selectedWeapons, meleeWeapons[i])
				}
			}
		}
	default: // medium
		// Medium AI: Balanced selection
		if preferMelee && len(meleeWeapons) > 0 {
			// Take half of available melee weapons
			maxMelee := (len(meleeWeapons) + 1) / 2
			for i := 0; i < maxMelee && i < len(meleeWeapons); i++ {
				selectedWeapons = append(selectedWeapons, meleeWeapons[i])
			}
		}
		if len(rangedWeapons) > 0 {
			// Take half of available ranged weapons
			maxRanged := (len(rangedWeapons) + 1) / 2
			for i := 0; i < maxRanged && i < len(rangedWeapons); i++ {
				selectedWeapons = append(selectedWeapons, rangedWeapons[i])
			}
		}
	}

	// Ensure we have at least one weapon
	if len(selectedWeapons) == 0 {
		selectedWeapons = append(selectedWeapons, availableWeapons[0])
	}

	// Deduplicate weapons by name (same logic as frontend)
	weaponMap := make(map[string]Weapon)
	for _, weapon := range selectedWeapons {
		weaponMap[weapon.Name] = weapon
	}

	// Convert map back to slice
	deduplicatedWeapons := make([]Weapon, 0, len(weaponMap))
	for _, weapon := range weaponMap {
		deduplicatedWeapons = append(deduplicatedWeapons, weapon)
	}

	log.Printf("AI %s selected %d weapons (deduplicated from %d, prefer melee: %t)", aiPlayer.Name, len(deduplicatedWeapons), len(selectedWeapons), preferMelee)
	return deduplicatedWeapons
}

// getHumanPlayerInMatch finds the human player in the same match as the AI
func (gs *GameServer) getHumanPlayerInMatch(matchID string) *Player {
	gs.mutex.RLock()
	defer gs.mutex.RUnlock()

	if match, exists := gs.matches[matchID]; exists {
		if !match.Player1.IsAI {
			return match.Player1
		}
		if !match.Player2.IsAI {
			return match.Player2
		}
	}
	return nil
}

// analyzeHumanWeaponPreferences counts melee vs ranged weapons in human player's army
func (gs *GameServer) analyzeHumanWeaponPreferences(humanPlayer *Player) (meleeCount, rangedCount int) {
	for _, unit := range humanPlayer.Army {
		for _, weapon := range unit.Weapons {
			if weapon.Type == "Melee" {
				meleeCount++
			} else if weapon.Type == "Ranged" {
				rangedCount++
			}
		}
	}
	return meleeCount, rangedCount
}

// handleAIAttack manages AI behavior during attack phase
func (gs *GameServer) handleAIAttack(aiPlayer *Player) {
	// Wait a bit to simulate thinking time
	time.Sleep(time.Duration(rand.Intn(2000)+1000) * time.Millisecond) // 1-3 seconds

	// AI automatically starts its attack
	gs.startAttack(aiPlayer)

	// After the attack is set up, automatically handle AI dice rolling
	go func() {
		time.Sleep(500 * time.Millisecond) // Brief pause after setup
		gs.handleAIDiceRolling(aiPlayer)
	}()
}

// handleAIDiceRolling automatically processes hit and wound rolls for AI attacks
func (gs *GameServer) handleAIDiceRolling(aiPlayer *Player) {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	// Find the AI player's match
	var match *Match
	for _, m := range gs.matches {
		if (m.Player1 == aiPlayer || m.Player2 == aiPlayer) && m.State == "manual_dice_rolling" {
			match = m
			break
		}
	}

	if match == nil {
		log.Printf("DEBUG: No match found for AI dice rolling for player %s", aiPlayer.Name)
		return
	}

	log.Printf("DEBUG: AI %s automatically handling dice rolls", aiPlayer.Name)

	// Process all weapons for hit and wound phases
	for match.CurrentWeaponIndex < len(match.AttackSequence) {
		currentWeapon := match.AttackSequence[match.CurrentWeaponIndex]

		// AI automatically rolls hits
		if match.CurrentPhase == "hit" {
			log.Printf("DEBUG: AI processing hit phase for weapon %d", match.CurrentWeaponIndex)
			gs.processHitPhase(match, currentWeapon)
			time.Sleep(500 * time.Millisecond) // Brief delay between phases
		}

		// AI automatically rolls wounds
		if match.CurrentPhase == "wound" {
			log.Printf("DEBUG: AI processing wound phase for weapon %d", match.CurrentWeaponIndex)
			gs.processWoundPhase(match, currentWeapon)

			// If wounds were caused, we stop here and wait for human to roll saves
			if currentWeapon["wounds"].(int) > 0 {
				log.Printf("DEBUG: AI caused wounds, waiting for human defender to roll saves")
				return // Exit and wait for human to roll saves
			}
		}

		// If no wounds were caused, move to next weapon
		if match.CurrentPhase == "hit" || (match.CurrentPhase == "wound" && currentWeapon["wounds"].(int) == 0) {
			// Move to next weapon
		}
	}
}
