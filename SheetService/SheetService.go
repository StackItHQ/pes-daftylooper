package sheetservice

import (
	"context"
	"log"
	"os"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

var spreadsheetId = "1sWkUx69XVCdWpa9N6FLv-8emYdSMSLIIZJ8QhtlCAdg" // maybe read from env??
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

func GetSheetData(srv *sheets.Service) [][]interface{} {
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetId, sheetRange).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve data from sheet: %v", err)
	}
	return resp.Values
}
