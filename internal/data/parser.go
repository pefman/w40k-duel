package data

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Catalogue struct {
	XMLName    xml.Name    `xml:"catalogue"`
	Name       string      `xml:"name,attr"`
	Entries    []Entry     `xml:"selectionEntries>selectionEntry"`
	Shared     []Entry     `xml:"sharedSelectionEntries>selectionEntry"`
	EntryLinks []EntryLink `xml:"entryLinks>entryLink"`
}

type Entry struct {
	ID                   string                `xml:"id,attr" json:"id,omitempty"`
	Name                 string                `xml:"name,attr" json:"name"`
	Type                 string                `xml:"type,attr" json:"type"`
	Entries              []Entry               `xml:"selectionEntries>selectionEntry" json:"entries,omitempty"`
	Profiles             []Profile             `xml:"profiles>profile" json:"profiles,omitempty"`
	Costs                []Cost                `xml:"costs>cost" json:"costs,omitempty"`
	SelectionEntryGroups []SelectionEntryGroup `xml:"selectionEntryGroups>selectionEntryGroup" json:"selectionEntryGroups,omitempty"`
	Weapons              []Profile             `json:"weapons,omitempty"` // Extracted weapon profiles
}

type SelectionEntryGroup struct {
	ID      string  `xml:"id,attr" json:"id,omitempty"`
	Name    string  `xml:"name,attr" json:"name"`
	Entries []Entry `xml:"selectionEntries>selectionEntry" json:"entries,omitempty"`
}

type EntryLink struct {
	Name     string `xml:"name,attr"`
	TargetId string `xml:"targetId,attr"`
	Type     string `xml:"type,attr"`
}

type Profile struct {
	Name  string `xml:"name,attr" json:"name"`
	Type  string `xml:"typeName,attr" json:"type"`
	Lines []Line `xml:"characteristics>characteristic" json:"lines,omitempty"`
}

type Line struct {
	Name  string `xml:"name,attr" json:"name"`
	Value string `xml:",chardata" json:"value"`
}

type Cost struct {
	Name   string `xml:"name,attr" json:"name"`
	TypeId string `xml:"typeId,attr" json:"typeid,omitempty"`
	Value  string `xml:"value,attr" json:"value"`
}

type FactionOutput struct {
	FactionName string  `json:"factionname"`
	Units       []Entry `json:"units"`
}

var globalUnits map[string]Entry

func sanitizeName(name string) string {
	// Convert to lowercase and replace spaces/special chars with hyphens
	reg := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	sanitized := reg.ReplaceAllString(name, "-")
	sanitized = strings.ToLower(sanitized)
	sanitized = strings.Trim(sanitized, "-")
	return sanitized
}

// mergeCharacterWeapons handles character units that have separate model and Unit entries
// It finds weapons from model entries and copies them to the Unit entry
func mergeCharacterWeapons(entry *Entry) {
	var modelWeapons []Profile
	var unitEntry *Entry

	// Look for model entries with weapons and Unit entries
	for i := range entry.Entries {
		subEntry := &entry.Entries[i]
		if subEntry.Type == "model" && len(subEntry.Weapons) > 0 {
			modelWeapons = append(modelWeapons, subEntry.Weapons...)
		} else if subEntry.Type == "Unit" {
			unitEntry = subEntry
		}
	}

	// If we found both model weapons and a Unit entry, merge them
	if len(modelWeapons) > 0 && unitEntry != nil {
		if len(unitEntry.Weapons) == 0 {
			unitEntry.Weapons = modelWeapons
			log.Printf("Merged %d weapons from model to Unit entry: %s", len(modelWeapons), unitEntry.Name)
		}
	}
}

func collectEntries(entries []Entry) []Entry {
	var result []Entry
	for _, entry := range entries {
		// Extract weapons from nested structures and store them separately
		weaponProfiles := extractWeaponsFromEntry(entry)
		if len(weaponProfiles) > 0 {
			entry.Weapons = weaponProfiles
		}

		// Handle character units: merge weapons from model entries to Unit entries
		if len(entry.Entries) > 0 {
			mergeCharacterWeapons(&entry)
		}

		result = append(result, entry)
		if len(entry.Entries) > 0 {
			result = append(result, collectEntries(entry.Entries)...)
		}
	}
	return result
}

func extractWeaponsFromEntry(entry Entry) []Profile {
	var weapons []Profile

	// Check direct weapon profiles in this entry
	for _, profile := range entry.Profiles {
		if isWeaponProfile(profile) {
			weapons = append(weapons, profile)
		}
	}

	// Recursively search through direct entries for weapons
	for _, subEntry := range entry.Entries {
		// Check for weapon profiles in direct sub-entries
		for _, profile := range subEntry.Profiles {
			if isWeaponProfile(profile) {
				weapons = append(weapons, profile)
			}
		}
		// Recursively search nested entries
		weapons = append(weapons, extractWeaponsFromEntry(subEntry)...)
	}

	// Recursively search through selectionEntryGroups for weapons
	for _, group := range entry.SelectionEntryGroups {
		weapons = append(weapons, extractWeaponsFromGroup(group)...)
	}

	return weapons
}

func extractWeaponsFromGroup(group SelectionEntryGroup) []Profile {
	var weapons []Profile

	// Look for weapon profiles in this group's entries
	for _, entry := range group.Entries {
		// Check if this entry has weapon profiles
		for _, profile := range entry.Profiles {
			if isWeaponProfile(profile) {
				weapons = append(weapons, profile)
			}
		}

		// Recursively search nested entries for weapons (important for complex units)
		weapons = append(weapons, extractWeaponsFromEntry(entry)...)

		// Recursively search nested groups
		for _, nestedGroup := range entry.SelectionEntryGroups {
			weapons = append(weapons, extractWeaponsFromGroup(nestedGroup)...)
		}
	}

	return weapons
}

func isWeaponProfile(profile Profile) bool {
	weaponTypes := []string{"Ranged Weapons", "Melee Weapons", "Weapons"}
	for _, weaponType := range weaponTypes {
		if profile.Type == weaponType {
			return true
		}
	}
	return false
}

func convertEntryLinks(links []EntryLink) []Entry {
	var result []Entry
	for _, link := range links {
		resolved := resolveEntryLink(link)
		result = append(result, resolved)
	}
	return result
}

func collectUnitsFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var cat Catalogue
	if err := xml.Unmarshal(data, &cat); err != nil {
		return err
	}

	// Collect all entries and shared entries with their IDs
	allEntries := append(collectEntries(cat.Entries), collectEntries(cat.Shared)...)

	for _, entry := range allEntries {
		if entry.ID != "" {
			// Extract weapons for this unit before storing
			weaponProfiles := extractWeaponsFromEntry(entry)
			if len(weaponProfiles) > 0 {
				entry.Weapons = weaponProfiles
			}
			globalUnits[entry.ID] = entry
		}
	}

	return nil
}

func resolveEntryLink(link EntryLink) Entry {
	// Try to find the unit in our global database
	if unit, exists := globalUnits[link.TargetId]; exists {
		// Create a copy with the link's name (which might be different)
		resolved := unit
		if link.Name != "" {
			resolved.Name = link.Name
		}
		return resolved
	}

	// If not found, return a basic entry
	return Entry{
		Name: link.Name,
		Type: link.Type,
	}
}

func ParseCatFiles() error {
	// Initialize global units map
	globalUnits = make(map[string]Entry)

	files, err := filepath.Glob("static/raw/*.cat")
	if err != nil {
		return err
	}

	// First pass: collect all unit definitions from library files
	fmt.Println("🔍 Phase 1: Building global unit database...")
	for _, file := range files {
		if err := collectUnitsFromFile(file); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to collect units from %s: %v\n", file, err)
		}
	}

	fmt.Printf("📊 Collected %d units in global database\n", len(globalUnits))

	// Second pass: parse catalog files and resolve references
	fmt.Println("🔗 Phase 2: Parsing catalogs and resolving references...")
	for _, file := range files {
		if err := parseCatFile(file); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse %s: %v\n", file, err)
		}
	}

	return nil
}

func parseCatFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var cat Catalogue
	if err := xml.Unmarshal(data, &cat); err != nil {
		return err
	}

	sanitized := sanitizeName(cat.Name)
	output := filepath.Join("static/json", sanitized+".json")
	if err := os.MkdirAll("static/json", 0755); err != nil {
		return err
	}

	allEntries := append(collectEntries(cat.Entries), collectEntries(cat.Shared)...)
	allEntries = append(allEntries, convertEntryLinks(cat.EntryLinks)...)

	// Final pass: ensure all units have their weapons extracted
	for i := range allEntries {
		weaponProfiles := extractWeaponsFromEntry(allEntries[i])
		if len(weaponProfiles) > 0 {
			// Store weapons in the dedicated weapons field
			allEntries[i].Weapons = weaponProfiles

			log.Printf("Total unique weapons found for %s: %d", allEntries[i].Name, len(weaponProfiles))
		}
	}

	out := FactionOutput{
		FactionName: cat.Name,
		Units:       allEntries,
	}

	jsonData, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(output, jsonData, 0644); err != nil {
		return err
	}

	fmt.Println("✅ Parsed:", cat.Name, "→", output)
	return nil
}
