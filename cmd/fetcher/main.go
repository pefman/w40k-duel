package main

import (
	"log"

	"github.com/pefman/w40k-duel/internal/data"
)

func main() {
	if err := data.DownloadAndExtractCats(); err != nil {
		log.Fatalf("Failed to download: %v", err)
	}

	if err := data.ParseCatFiles(); err != nil {
		log.Fatalf("Failed to parse .cat files: %v", err)
	}
}
