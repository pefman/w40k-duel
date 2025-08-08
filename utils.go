package main

import (
	"fmt"
	"math/rand"
	"strconv"
	"time"
)

// Utility functions
func generatePlayerID() string {
	return fmt.Sprintf("player_%d_%d", time.Now().Unix(), rand.Intn(1000))
}

func generateMatchID() string {
	return fmt.Sprintf("match_%d_%d", time.Now().Unix(), rand.Intn(1000))
}

func generatePlayerName() string {
	adjectives := []string{
		"Fabulous", "Screaming", "Naked", "Drunk", "Horny", "Smelly", "Farting", "Burping",
		"Ticklish", "Giggly", "Bouncy", "Squishy", "Wiggly", "Jiggly", "Fluffy", "Chunky",
		"Sneaky", "Dizzy", "Clumsy", "Goofy", "Silly", "Wacky", "Crazy", "Loopy",
		"Sweaty", "Hairy", "Greasy", "Slippery", "Sticky", "Icky", "Funky", "Stinky",
		"Cheeky", "Sassy", "Flirty", "Naughty", "Spicy", "Juicy", "Steamy", "Sultry",
		"Thicc", "Curvy", "Busty", "Bootylicious", "Seductive", "Tempting", "Alluring",
		"Giggling", "Moaning", "Panting", "Purring", "Whimpering", "Gasping",
	}

	warhammer_names := []string{
		"Guilliman", "Calgar", "Sicarius", "Lysander", "Grimaldus", "Helbrecht", "Azrael",
		"Belial", "Sammael", "Asmodai", "Ezekiel", "Chaplain", "Librarian", "Techmarine",
		"Sanguinor", "Mephiston", "Dante", "Tycho", "Corbulo", "Lemartes", "Astorath",
		"Logan Grimnar", "Bjorn", "Ragnar", "Lukas", "Ulrik", "Njal", "Arjac",
		"Vulkan", "Tu'Shan", "Adrax", "Bray'arth", "Forgefather", "Praetor",
		"Shrike", "Lysander", "Kantor", "Huron", "Lufgt", "Tyberos", "Asterion",
		"Abaddon", "Kharn", "Lucius", "Typhus", "Ahriman", "Fabius", "Erebus",
		"Kor Phaeron", "Lorgar", "Angron", "Mortarion", "Magnus", "Perturabo",
		"Fulgrim", "Alpharius", "Omegon", "Horus", "Curze", "Dorn", "Ferrus",
		"Jaghatai", "Leman Russ", "Roboute", "Sanguinius", "Corax", "Vulkan",
		"Eldrad", "Yvraine", "Vect", "Lelith", "Drazhar", "Jain Zar", "Karandras",
		"Fuegan", "Maugan Ra", "Baharroth", "Asurmen", "Phoenix Lord",
		"Ghazghkull", "Makari", "Boss Snikrot", "Kaptin Badrukk", "Old Zogwort",
		"Wazdakka", "Zagstruk", "Nazdreg", "Grotsnik", "Mad Dok", "Big Mek",
		"Shadowsun", "Farsight", "Darkstrider", "Longstrike", "Bravestorm",
		"Aun'Va", "Aun'Shi", "Commander", "Ethereal", "Crisis Suit",
		"Hive Tyrant", "Swarmlord", "Old One Eye", "Deathleaper", "Red Terror",
		"Doom of Malan'tai", "Parasite", "Tervigon", "Carnifex", "Lictor",
		"The Silent King", "Imotekh", "Szarekh", "Zahndrekh", "Obyron",
		"Anrakyr", "Trazyn", "Orikan", "Nemesor", "Overlord", "Cryptek",
		"Belisarius Cawl", "Kataphron", "Skitarii", "Tech Priest", "Dominus",
		"Manipulus", "Enginseer", "Servitor", "Kastelan", "Onager",
		"Celestine", "Jacobus", "Coteaz", "Eisenhorn", "Ravenor", "Creed",
		"Straken", "Pask", "Marbo", "Commissar", "Lord General",
	}

	return fmt.Sprintf("%s %s",
		adjectives[rand.Intn(len(adjectives))],
		warhammer_names[rand.Intn(len(warhammer_names))])
}

func parseStatValue(stat string) int {
	if stat == "" {
		return 1
	}
	val, err := strconv.Atoi(stat)
	if err != nil {
		return 1
	}
	return val
}
