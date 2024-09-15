package mysql

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql" // MySQL driver
)

type DBConfig struct {
	User     string `json:"user"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	DBName   string `json:"dbname"`
}

// LoadConfig loads the database configuration from a JSON file.
func loadConfig(filePath string) (*DBConfig, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("could not open file %s: %v", filePath, err)
	}
	defer file.Close()

	byteValue, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("could not read file %s: %v", filePath, err)
	}

	var config DBConfig
	err = json.Unmarshal(byteValue, &config)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal JSON: %v", err)
	}

	return &config, nil
}

// Connect establishes a connection to the MySQL database using the provided DBConfig.
func connect(config *DBConfig) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
		config.User,
		config.Password,
		config.Host,
		config.Port,
		config.DBName,
	)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func GetConnection() (*sql.DB, error) {
	// Load database configuration
	config, err := loadConfig("mysql_secret.json")
	if err != nil {
		return nil, fmt.Errorf("error loading config: %v", err)
	}

	// Connect to the database
	db, err := connect(config)
	if err != nil {
		return nil, fmt.Errorf("error connecting to database: %v", err)
	}

	return db, nil
}

// InsertOrUpdateSheetData inserts new sheet data or updates existing data (no timestamp).
func InsertOrUpdateSheetData(db *sql.DB, sheetID string, data interface{}) error {
	// Convert data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// Insert or update sheet data without timestamp
	query := `
        INSERT INTO sheet_data (sheet_id, data)
        VALUES (?, ?)
        ON DUPLICATE KEY UPDATE data = VALUES(data)
    `
	_, err = db.Exec(query, sheetID, jsonData)
	if err != nil {
		return err
	}
	return nil
}

// GetSheetData retrieves the data for a specific sheet based on the latest timestamp.
func GetSheetData(db *sql.DB, sheetID string) (map[string]interface{}, error) {
	var jsonData string

	// Use a query to retrieve the latest data for a sheet
	query := `
        SELECT sd.data
        FROM sheet_data sd
        JOIN timestamps t ON sd.sheet_id = t.sheet_id
        WHERE sd.sheet_id = ?
        ORDER BY t.timestamp DESC
        LIMIT 1
    `

	row := db.QueryRow(query, sheetID)
	err := row.Scan(&jsonData)
	if err != nil {
		return nil, err
	}

	// Convert JSON to map
	var data map[string]interface{}
	err = json.Unmarshal([]byte(jsonData), &data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// InsertOrUpdateTimestamp inserts or updates the last write timestamp for a specific sheet.
func InsertOrUpdateTimestamp(db *sql.DB, sheetID string) error {
	query := `
        INSERT INTO timestamps (sheet_id, timestamp)
        VALUES (?, NOW())
        ON DUPLICATE KEY UPDATE timestamp = NOW()
    `
	_, err := db.Exec(query, sheetID)
	if err != nil {
		return err
	}
	return nil
}

// GetLastWriteTimestamp retrieves the last write timestamp for a specific sheet.
func GetLastWriteTimestamp(db *sql.DB, sheetID string) (time.Time, error) {
	var lastWrite time.Time
	query := `SELECT timestamp FROM timestamps WHERE sheet_id = ?`
	row := db.QueryRow(query, sheetID)
	err := row.Scan(&lastWrite)
	if err != nil {
		return time.Time{}, err
	}
	return lastWrite, nil
}

// TruncateTables removes all data from the sheet_data and timestamps tables.
func TruncateTables(db *sql.DB) error {
	_, err := db.Exec("TRUNCATE TABLE sheet_data")
	if err != nil {
		return fmt.Errorf("could not truncate sheet_data table: %v", err)
	}

	_, err = db.Exec("TRUNCATE TABLE timestamps")
	if err != nil {
		return fmt.Errorf("could not truncate timestamps table: %v", err)
	}

	return nil
}

// StoreDataInDatabase inserts or updates data in the sheet_data table (no timestamp).
func StoreDataInDatabase(db *sql.DB, sheetID string, data [][]interface{}) error {
	// Prepare query for inserting/updating data
	query := `INSERT INTO sheet_data (sheet_id, data)
	          VALUES (?, ?)
	          ON DUPLICATE KEY UPDATE data = VALUES(data)`

	// Convert data to JSON
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("could not marshal data: %v", err)
	}

	_, err = db.Exec(query, sheetID, dataJSON)
	if err != nil {
		return fmt.Errorf("could not insert/update data: %v", err)
	}

	return nil
}

// StoreTimestampInDatabase inserts or updates the timestamp for the given sheet_id.
func StoreTimestampInDatabase(db *sql.DB, sheetID string, timestamp time.Time) error {
	// Convert timestamp to MySQL-compatible format (YYYY-MM-DD HH:MM:SS)
	formattedTimestamp := timestamp.Format("2006-01-02 15:04:05")

	_, err := db.Exec("TRUNCATE TABLE timestamps")
	if err != nil {
		return fmt.Errorf("could not truncate timestamps table: %v", err)
	}

	_, err = db.Exec(
		`INSERT INTO timestamps (sheet_id, timestamp) VALUES (?, ?)
			ON DUPLICATE KEY UPDATE timestamp = VALUES(timestamp)`,
		sheetID, formattedTimestamp)
	if err != nil {
		return fmt.Errorf("could not insert/update timestamp: %v", err)
	}
	return nil
}

// GetLatestDataFromDatabase retrieves the latest data from the database.
func GetLatestDataFromDatabase(db *sql.DB) ([][]interface{}, error) {
	// Prepare query to join sheet_data and timestamps on sheet_id and get the latest data
	query := `
        SELECT sd.data
        FROM sheet_data sd
        JOIN timestamps ts ON sd.sheet_id = ts.sheet_id
    `

	// Execute query
	row := db.QueryRow(query)

	// Scan the result
	var dataJSON string
	if err := row.Scan(&dataJSON); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no latest data found")
		}
		return nil, fmt.Errorf("could not scan data: %v", err)
	}

	// Unmarshal JSON to data
	var data [][]interface{}
	if err := json.Unmarshal([]byte(dataJSON), &data); err != nil {
		return nil, fmt.Errorf("could not unmarshal data: %v", err)
	}

	return data, nil
}
