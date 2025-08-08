package main

import (
	"log"
)

func (gs *GameServer) sendFactions(player *Player) {
	factions, err := fetchFactions()
	if err != nil {
		log.Printf("Error fetching factions: %v", err)
		return
	}

	gs.sendToPlayer(player, map[string]interface{}{
		"type":     "factions_available",
		"factions": factions,
	})
}

func (gs *GameServer) selectFaction(player *Player, faction string) {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	player.Faction = faction
	player.Status = "selecting"

	log.Printf("Player %s selected faction: %s", player.Name, faction)

	units, err := fetchFactionUnitsCached(faction)
	if err != nil {
		log.Printf("Error fetching units for faction %s: %v", faction, err)
		return
	}

	log.Printf("Fetched %d units for faction %s", len(units), faction)

	// Prepare unit data with weapon type information
	unitData := make([]map[string]interface{}, 0)
	for _, unit := range units {
		weaponTypes := getUnitWeaponTypes(unit)
		unitInfo := map[string]interface{}{
			"name":         unit.Name,
			"wounds":       unit.Wounds,
			"attacks":      unit.Attacks,
			"strength":     unit.Strength,
			"toughness":    unit.Toughness,
			"weapon_types": weaponTypes,
		}
		unitData = append(unitData, unitInfo)
	}

	gs.sendToPlayer(player, map[string]interface{}{
		"type":    "faction_selected",
		"faction": faction,
		"units":   unitData,
	})
}

func (gs *GameServer) selectArmy(player *Player, armyData []interface{}) {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	army := make([]UnitSelection, 0)
	for _, unitData := range armyData {
		unitMap, ok := unitData.(map[string]interface{})
		if !ok {
			continue
		}

		unitName, _ := unitMap["unit_name"].(string)
		quantity, _ := unitMap["quantity"].(float64)
		weaponType, _ := unitMap["weapon_type"].(string)

		unit := UnitSelection{
			UnitName: unitName,
			Quantity: int(quantity),
			Weapons:  make([]Weapon, 0),
		}

		// Get units for this faction to find the selected unit
		units, err := fetchFactionUnitsCached(player.Faction)
		if err == nil {
			for _, unitInfo := range units {
				if unitInfo.Name == unitName {
					// Get weapons of the selected type
					unit.Weapons = getUnitWeaponsByType(unitInfo, weaponType)
					break
				}
			}
		}

		army = append(army, unit)
	}

	player.Army = army
	player.Status = "ready"

	gs.sendToPlayer(player, map[string]interface{}{
		"type":    "army_selected",
		"army":    army,
		"message": "Army selection complete. Waiting for opponent...",
	})

	// Check if both players are ready
	if match, exists := gs.matches[player.MatchID]; exists {
		if match.Player1.Status == "ready" && match.Player2.Status == "ready" {
			gs.startBattle(match)
		}
	}
}

func (gs *GameServer) getUnitWeapons(player *Player, unitName string) {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	if player.Faction == "" {
		gs.sendToPlayer(player, map[string]interface{}{
			"type":  "error",
			"error": "No faction selected",
		})
		return
	}

	// Get units for this faction
	units, err := fetchFactionUnitsCached(player.Faction)
	if err != nil {
		log.Printf("Error fetching units for faction %s: %v", player.Faction, err)
		gs.sendToPlayer(player, map[string]interface{}{
			"type":  "error",
			"error": "Failed to fetch unit data",
		})
		return
	}

	// Find the requested unit
	var targetUnit *Unit
	for _, unit := range units {
		if unit.Name == unitName {
			targetUnit = &unit
			break
		}
	}

	if targetUnit == nil {
		gs.sendToPlayer(player, map[string]interface{}{
			"type":  "error",
			"error": "Unit not found",
		})
		return
	}

	// Group weapons by type
	weaponsByType := make(map[string][]Weapon)
	for _, weapon := range targetUnit.Weapons {
		weaponType := weapon.Type
		if weaponType == "" || weaponType == "Unknown" {
			weaponType = "Other"
		}
		weaponsByType[weaponType] = append(weaponsByType[weaponType], weapon)
	}

	gs.sendToPlayer(player, map[string]interface{}{
		"type":      "unit_weapons",
		"unit_name": unitName,
		"unit_stats": map[string]interface{}{
			"wounds":    targetUnit.Wounds,
			"attacks":   targetUnit.Attacks,
			"strength":  targetUnit.Strength,
			"toughness": targetUnit.Toughness,
		},
		"weapons_by_type": weaponsByType,
	})
}
