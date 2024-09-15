package mysql

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"

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

// InsertOrUpdateSheetData inserts new sheet data or updates existing data.
func InsertOrUpdateSheetData(db *sql.DB, sheetID string, data interface{}) error {
	// Convert data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// Insert or update sheet data
	query := `
        INSERT INTO sheet_data (sheet_id, data)
        VALUES (?, ?)
        ON DUPLICATE KEY UPDATE data = VALUES(data), timestamp = CURRENT_TIMESTAMP
    `
	_, err = db.Exec(query, sheetID, jsonData)
	if err != nil {
		return err
	}
	return nil
}

// GetSheetData retrieves the data for a specific sheet based on the latest timestamp using a single query.
func GetSheetData(db *sql.DB, sheetID string) (map[string]interface{}, error) {
	var jsonData string

	// Use a single query to join sheet_data and sheet_timestamps and get the latest data
	query := `
        SELECT sd.data
        FROM sheet_data sd
        JOIN sheet_timestamps st
        ON sd.sheet_id = st.sheet_id
        WHERE sd.sheet_id = ? AND sd.timestamp = st.last_write
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
        INSERT INTO sheet_timestamps (sheet_id, last_write)
        VALUES (?, NOW())
        ON DUPLICATE KEY UPDATE last_write = NOW()
    `
	_, err := db.Exec(query, sheetID)
	if err != nil {
		return err
	}
	return nil
}

// GetLastWriteTimestamp retrieves the last write timestamp for a specific sheet.
// func GetLastWriteTimestamp(db *sql.DB, sheetID string) (time.Time, error) {
// 	var lastWrite time.Time
// 	query := `SELECT last_write FROM sheet_timestamps WHERE sheet_id = ?`
// 	row := db.QueryRow(query, sheetID)
// 	err := row.Scan(&lastWrite)
// 	if err != nil {
// 		return time.Time{}, err
// 	}
// 	return lastWrite, nil
// }
