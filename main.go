package main

import (
	"fmt"
	"log"
	mysql "sheets-sync-db/MySQL"
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

	// Get MySQL database connection
	db, err := mysql.GetConnection()
	if err != nil {
		log.Fatalf("Failed to get database connection: %v", err)
	}
	defer db.Close()

	fmt.Println("Successfully connected to the database!")

	ticker := time.NewTicker(syncInterval)
	defer ticker.Stop()
	var wg sync.WaitGroup
	wg.Add(1)

	sheetIds := []string{
		"1sWkUx69XVCdWpa9N6FLv-8emYdSMSLIIZJ8QhtlCAdg",
		"1ymOzgTitOLpM1sg73xVcgxFU20xTA7zrO9fWq5GAnLY",
	}

	go func() {
		defer wg.Done()
		for {
			select {
			case <-ticker.C:
				fmt.Println("Polling and syncing sheets...")
				sheetservice.PollAndSyncSheet(srv, db, sheetIds)
			}
		}
	}()

	// Do conflict resolution somehow?

	// Wait for goroutines (they won’t finish unless interrupted)
	wg.Wait()
}
