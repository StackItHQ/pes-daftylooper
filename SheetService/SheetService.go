package sheetservice

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"sync"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

var sheetRange = "Sheet1!A1:D10"

func GetSheetService() (*sheets.Service, error) {
	ctx := context.Background()
	b, err := os.ReadFile("credentials.json") // Path to your Google API credentials
	println(b)
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

func ClearSheet(srv *sheets.Service, spreadsheetId, sheetRange string) error {
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
	err := ClearSheet(srv, spreadsheetId, sheetRange)
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

func PollAndSyncSheet(srv *sheets.Service, db *sql.DB, sheetIds []string) {
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, sheetId := range sheetIds {
		wg.Add(1)
		go func(sheetID string) {
			defer wg.Done()

			// Poll Google Sheets
			data := GetSheetData(srv, sheetID)
			fmt.Printf("Data from sheet %s: %v\n", sheetID, data)

			// Store data in the database
			// storeDataInDatabase(db, sheetID, data)

			string,			// Here you would compare data between sheets and sync them accordingly
			// For simplicity, we assume one sheet's changes will be synced to the other.
			mu.Lock()
			for _, otherSheetId := range sheetIds {
				if otherSheetId != sheetID {
					// Example: Sync data to the other sheet
					// Replace with actual data sync logic
					PushDataToSheet(srv, otherSheetId, data)
					fmt.Printf("Pushed data from sheet %s to sheet %s\n", sheetID, otherSheetId)
				}
			}
			mu.Unlock()
		}(sheetId)
	}

	wg.Wait()
}
