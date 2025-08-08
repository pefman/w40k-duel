package data

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
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
	ID       string    `xml:"id,attr" json:"id,omitempty"`
	Name     string    `xml:"name,attr" json:"name"`
	Type     string    `xml:"type,attr" json:"type"`
	Entries  []Entry   `xml:"selectionEntries>selectionEntry" json:"entries,omitempty"`
	Profiles []Profile `xml:"profiles>profile" json:"profiles,omitempty"`
	Costs    []Cost    `xml:"costs>cost" json:"costs,omitempty"`
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

func collectEntries(entries []Entry) []Entry {
	var result []Entry
	for _, entry := range entries {
		result = append(result, entry)
		if len(entry.Entries) > 0 {
			result = append(result, collectEntries(entry.Entries)...)
		}
	}
	return result
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
