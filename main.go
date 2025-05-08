package main

import (
	"log"
	"time"

	"github.com/joho/godotenv"
	"github.com/nahidhasan98/deshimula-notifier-unofficial/config"
	"github.com/nahidhasan98/deshimula-notifier-unofficial/errorhandling"
	"github.com/nahidhasan98/deshimula-notifier-unofficial/mula"
)

func checkPeriodically(mulaService *mula.Mula) {
	ticker := time.NewTicker(config.Interval)
	defer ticker.Stop()

	log.Println("Starting periodic story check every minute...")

	for range ticker.C {
		if err := mulaService.FetchAndProcessStories(); err != nil {
			errorhandling.HandleError(err)
		}
	}
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Failed to load .env file: %v", err)
	}

	mulaService, err := mula.New()
	if err != nil {
		log.Fatalf("Failed to initialize mula client: %v", err)
	}

	if err := mulaService.FetchAndProcessStories(); err != nil {
		errorhandling.HandleError(err)
	}

	checkPeriodically(mulaService)
}
