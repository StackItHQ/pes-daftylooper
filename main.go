package main

import (
	"fmt"
	"log"
	sheetservice "sheets-sync-db/SheetService"
	"sync"
	"time"
)

var syncInterval = 10 * time.Second // Polling interval

func main() {
	// Initialize Google Sheets service
	srv, err := sheetservice.GetSheetService()
	if err != nil {
		log.Fatalf("Failed to create Sheets service: %v", err)
	}

	ticker := time.NewTicker(syncInterval)
	defer ticker.Stop()

	var wg sync.WaitGroup
	wg.Add(1)

	// Start a goroutine to handle polling every 10 seconds
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ticker.C:
				// Poll Google Sheets and update the database
				fmt.Println("Polling Google Sheets...")
				data := sheetservice.GetSheetData(srv)
				fmt.Println("Google Sheet Data:", data)

			}
		}
	}()

	// Do conflict resolution somehow?

	// Wait for goroutines (they won’t finish unless interrupted)
	wg.Wait()
}
