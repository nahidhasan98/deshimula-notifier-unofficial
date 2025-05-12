package main

import (
	"log"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/nahidhasan98/deshimula-notifier-unofficial/config"
	"github.com/nahidhasan98/deshimula-notifier-unofficial/errorhandling"
	"github.com/nahidhasan98/deshimula-notifier-unofficial/interfacer"
	"github.com/nahidhasan98/deshimula-notifier-unofficial/mula"
	"github.com/nahidhasan98/deshimula-notifier-unofficial/oak"
)

func checkPeriodically(service interfacer.Service) {
	ticker := time.NewTicker(config.Interval)
	defer ticker.Stop()

	log.Println("Starting periodic story check every minute...")

	for range ticker.C {
		if err := service.FetchAndProcessStories(); err != nil {
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

	oakService, err := oak.New()
	if err != nil {
		log.Fatalf("Failed to initialize oak client: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if err := mulaService.FetchAndProcessStories(); err != nil {
			errorhandling.HandleError(err)
		}
	}()

	go func() {
		defer wg.Done()
		if err := oakService.FetchAndProcessStories(); err != nil {
			errorhandling.HandleError(err)
		}
	}()

	wg.Wait()

	go checkPeriodically(mulaService)
	go checkPeriodically(oakService)

	// Keep the main goroutine alive
	select {}
}
