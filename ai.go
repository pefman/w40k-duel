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

		unitSelection := UnitSelection{
			UnitName: unit.Name,
			Quantity: quantity,
			Weapons:  unit.Weapons[:], // Use all available weapons
		}

		army = append(army, unitSelection)
	}

	// Convert army data to interface{} slice
	armyInterface := make([]interface{}, len(army))
	for i, unit := range army {
		armyInterface[i] = map[string]interface{}{
			"unit_name": unit.UnitName,
			"quantity":  unit.Quantity,
			"weapons":   unit.Weapons,
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
	if len(units) <= maxUnits {
		return units
	}

	selected := make([]Unit, 0, maxUnits)

	switch difficulty {
	case "easy":
		// Easy AI prefers basic units (usually at the beginning of the list)
		for i := 0; i < maxUnits && i < len(units); i++ {
			selected = append(selected, units[i])
		}
	case "hard":
		// Hard AI prefers powerful units and good combinations
		// For now, select diverse units from different parts of the list
		step := len(units) / maxUnits
		for i := 0; i < maxUnits; i++ {
			idx := (i * step) % len(units)
			selected = append(selected, units[idx])
		}
	default: // medium
		// Medium AI selects randomly
		used := make(map[int]bool)
		for len(selected) < maxUnits {
			idx := rand.Intn(len(units))
			if !used[idx] {
				selected = append(selected, units[idx])
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
	gs.rollDice(aiPlayer, result)
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
