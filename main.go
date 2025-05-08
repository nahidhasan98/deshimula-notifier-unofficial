package main

import (
	"log"

	"github.com/joho/godotenv"
	"github.com/nahidhasan98/deshimula-notifier-unofficial/config"
	"github.com/nahidhasan98/deshimula-notifier-unofficial/errorhandling"
	"github.com/nahidhasan98/deshimula-notifier-unofficial/mula"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Failed to load .env file: %v", err)
	}

	mulaService, err := mula.New(config.NewHTTPConfig())
	if err != nil {
		log.Fatalf("Failed to initialize mula client: %v", err)
	}

	err = mulaService.FetchAndProcessStories()
	errorhandling.HandleError(err)
}
