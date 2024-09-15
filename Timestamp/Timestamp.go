package timestamp

import (
	"database/sql"
	"sync"
	"time"
)

// Store the last update timestamp for each sheet
var lastUpdate map[string]time.Time
var mu sync.Mutex

func init() {
	lastUpdate = make(map[string]time.Time)
}

// Get the latest timestamp from the database for a specific sheet
func getLatestTimestamp(db *sql.DB, sheetID string) (time.Time, error) {
	var timestamp time.Time
	err := db.QueryRow("SELECT timestamp FROM timestamps WHERE sheet_id = ?", sheetID).Scan(&timestamp)
	if err != nil {
		return time.Time{}, err
	}
	return timestamp, nil
}

// Update the timestamp in the database
func updateTimestamp(db *sql.DB, sheetID string, timestamp time.Time) error {
	_, err := db.Exec("REPLACE INTO timestamps (sheet_id, timestamp) VALUES (?, ?)", sheetID, timestamp)
	return err
}
