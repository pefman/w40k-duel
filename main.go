package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
)

func main() {
	// Set log format to use 24H time
	log.SetFlags(log.LstdFlags | log.Ltime)

	rand.Seed(time.Now().UnixNano())

	// Initialize optimized cache for better API performance
	initOptimizedCache()
	log.Println("✅ Optimized cache initialized")

	gameServer := NewGameServer()

	r := mux.NewRouter()
	r.HandleFunc("/", gameServer.handleIndex)
	r.HandleFunc("/ws", gameServer.handleWebSocket)
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		stats := getCacheStatsOptimized()
		w.Write([]byte(`{"status":"ok","cache":` + fmt.Sprintf("%v", stats) + `}`))
	})

	// Get port from environment variable (required for Cloud Run)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting Warhammer 40K Duel server on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
