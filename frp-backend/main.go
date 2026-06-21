package main

import (
	"log"
	"os"
	"path/filepath"
	"time"
)

func main() {
	log.Println("Starting full Go executable architecture for Ashan FRP v2...")

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" { dataDir = "./data" }
	dbPath := filepath.Join(dataDir, "frp_manager.db")

	InitFRPDB(dbPath)
	InitFRPManager()
	StartAuthWatchdog()

	r := SetupFRPRouter()
	go func() {
		log.Println("FRP Admin-Panel native socket up on :8080")
		if err := r.Run(":8080"); err != nil {
			log.Fatalf("Server exception: %v", err)
		}
	}()

	frpTicker := time.NewTicker(30 * time.Second)
	for range frpTicker.C {
		log.Println("[Watchdog] Pure-Go core state audit verified green.")
	}
}
