package sheetservice

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	mysql "sheets-sync-db/MySQL"
	"sync"
	"time"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

var (
	mu          sync.Mutex
	sheetHashes = make(map[string]string)    // In-memory storage for hashes
	lastUpdated = make(map[string]time.Time) // In-memory storage for last update timestamps
	sheetRange  = "Sheet1!A1:D10"
)

func GetSheetService() (*sheets.Service, error) {
	ctx := context.Background()
	b, err := os.ReadFile("credentials.json") // Path to your Google API credentials
	fmt.Println("Got instance of Google sheets!")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}
	config, err := google.JWTConfigFromJSON(b, sheets.SpreadsheetsScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := config.Client(ctx)

	// Updated method to create the service
	srv, err := sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client: %v", err)
	}

	return srv, nil
}

func clearSheet(srv *sheets.Service, spreadsheetId, sheetRange string) error {
	_, err := srv.Spreadsheets.Values.Clear(spreadsheetId, sheetRange, &sheets.ClearValuesRequest{}).Do()
	if err != nil {
		log.Fatalf("Unable to clear data from sheet: %v", err)
		return err
	}

	log.Println("Sheet cleared successfully")
	return nil
}

func GetSheetData(srv *sheets.Service, spreadsheetId string) [][]interface{} {
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetId, sheetRange).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve data from sheet: %v", err)
	}
	return resp.Values
}

// PushDataToSheet clears the sheet and then pushes new data to it.
func PushDataToSheet(srv *sheets.Service, spreadsheetId string, values [][]interface{}) error {
	// Clear the existing data in the specified range
	err := clearSheet(srv, spreadsheetId, sheetRange)
	if err != nil {
		return err
	}

	// Create a ValueRange object with the data to be pushed
	valueRange := &sheets.ValueRange{
		Values: values,
	}

	// Use the Update method to replace data in the sheet
	_, err = srv.Spreadsheets.Values.Update(spreadsheetId, sheetRange, valueRange).
		ValueInputOption("RAW"). // Use "RAW" for plain text or "USER_ENTERED" if you want Google Sheets to interpret it
		Do()
	if err != nil {
		log.Fatalf("Unable to push data to sheet: %v", err)
		return err
	}

	log.Println("Data pushed to Google Sheet successfully")
	return nil
}

// GenerateHash computes a SHA256 hash of the data.
func generateHash(data [][]interface{}) string {
	dataJSON, _ := json.Marshal(data)
	hash := sha256.Sum256(dataJSON)
	return hex.EncodeToString(hash[:])
}

// InitializeSheets initializes Google Sheets data in MySQL.
func InitializeSheets(srv *sheets.Service, db *sql.DB, sheetIds []string) error {
	// Truncate existing data in both tables
	if err := mysql.TruncateTables(db); err != nil {
		return fmt.Errorf("error truncating tables: %v", err)
	}

	// Uniform timestamp for initial data
	formattedTime := time.Now().UTC().Format(time.RFC3339)
	timestamp, err := time.Parse(time.RFC3339, formattedTime)
	if err != nil {
		return fmt.Errorf("error parsing initial timestamp: %v", err)
	}

	for _, sheetId := range sheetIds {
		// Poll Google Sheets
		data := GetSheetData(srv, sheetId)
		if data == nil {
			log.Printf("No data fetched for sheet %s", sheetId)
			continue
		}

		// Insert initial data into MySQL
		err := mysql.StoreDataInDatabase(db, sheetId, data)
		if err != nil {
			log.Printf("Error storing data in database for sheet %s: %v", sheetId, err)
			continue
		}

		// Insert initial timestamp into the timestamps table
		err = mysql.StoreTimestampInDatabase(db, sheetId, timestamp)
		if err != nil {
			log.Printf("Error storing timestamp in database for sheet %s: %v", sheetId, err)
		}
	}

	return nil
}

// PollAndSyncSheet polls Google Sheets, stores data in the database, and syncs updates to other sheets.
func PollAndSyncSheet(srv *sheets.Service, db *sql.DB, sheetIds []string) {
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Phase 1: Poll all sheets and update database
	for _, sheetID := range sheetIds {
		wg.Add(1)
		go func(sheetID string) {
			defer wg.Done()

			// Poll Google Sheets for data
			data := GetSheetData(srv, sheetID)
			// if data == nil {
			// 	log.Printf("No data fetched for sheet %s", sheetID)
			// 	return
			// }

			// Generate hash for the current sheet data
			hash := generateHash(data)

			// Lock mutex before checking/updating in-memory hashes and timestamps
			mu.Lock()
			storedHash, hashExists := sheetHashes[sheetID]

			// Check if the data has changed based on the hash comparison
			if !hashExists || storedHash != hash {
				log.Printf("\nChange detected in sheet: %s\n", sheetID)

				// Generate a new timestamp for this change
				newTimestamp := time.Now()

				// Store the new data in the database with the new timestamp
				err := mysql.StoreDataInDatabase(db, sheetID, data)
				if err != nil {
					log.Printf("Error storing data in database for sheet %s: %v", sheetID, err)
					mu.Unlock() // Unlock mutex before returning
					return
				}

				// Update in-memory hash and timestamp
				sheetHashes[sheetID] = hash
				lastUpdated[sheetID] = newTimestamp

				// Also store the new timestamp in the timestamp table in MySQL
				err = mysql.StoreTimestampInDatabase(db, sheetID, newTimestamp)
				if err != nil {
					log.Printf("Error updating timestamp in database for sheet %s: %v", sheetID, err)
					mu.Unlock() // Unlock mutex before returning
					return
				}
			}
			mu.Unlock() // Unlock mutex after updating hash and timestamp
		}(sheetID)
	}

	// Wait for all polling and database updates to complete
	wg.Wait()

	// Phase 2: Synchronize data across all sheets
	mu.Lock()
	// Retrieve the latest data from the database for each sheet
	latestData, err := mysql.GetLatestDataFromDatabase(db)
	log.Print("\n\n Latest Data:", latestData)
	if err != nil {
		log.Printf("Error getting latest data from database: %v", err)
	}
	// Push the latest data from the current sheet to other sheets
	for _, sheetID := range sheetIds {
		err := PushDataToSheet(srv, sheetID, latestData)
		if err != nil {
			log.Printf("Error pushing latest data to sheet %s: %v", sheetID, err)
		} else {
			fmt.Printf("Pushed data from to sheet %s\n", sheetID)
		}
	}
	mu.Unlock()
}
