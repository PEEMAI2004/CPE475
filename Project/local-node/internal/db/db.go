package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
	"github.com/potbuddy/local-node/internal/processor"
)

var (
	DB *sql.DB
)

// InitDB connects to the database.
func InitDB(dsn string) error {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("db: open: %w", err)
	}

	if err := db.Ping(); err != nil {
		return fmt.Errorf("db: ping: %w", err)
	}

	DB = db
	log.Println("[db] connected to postgres")
	return nil
}

// StartThresholdPoller repeatedly polls the database for updated profiles
// and mappings, pushing them to the processor package.
func StartThresholdPoller(interval time.Duration) {
	// Do an initial poll so we don't start blank
	poll()

	ticker := time.NewTicker(interval)
	for range ticker.C {
		poll()
	}
}

func poll() {
	if DB == nil {
		return
	}

	// 1. Fetch available profiles
	rows, err := DB.Query(`
		SELECT 
			id, name,
			soil_inner_low, soil_inner_high, soil_outer_low, soil_outer_high,
			temp_inner_low, temp_inner_high, temp_outer_low, temp_outer_high,
			hum_inner_low, hum_inner_high, hum_outer_low, hum_outer_high,
			light_inner_low, light_inner_high, light_outer_low, light_outer_high
		FROM profiles
	`)
	if err != nil {
		log.Printf("[db] failed to query profiles: %v", err)
		return
	}
	defer rows.Close()

	// Temp structures to hold loaded data
	// id -> Profile logic
	profileMap := make(map[int]processor.ThresholdProfile)
	
	// deviceId -> profile
	deviceMap := make(map[string]processor.ThresholdProfile)
	var defaultProfile processor.ThresholdProfile
	foundDefault := false

	for rows.Next() {
		var id int
		var name string
		var sil, sih, sol, soh *float64
		var til, tih, tol, toh *float64
		var hil, hih, hol, hoh *float64
		var lil, lih, lol, loh *float64

		err := rows.Scan(&id, &name,
			&sil, &sih, &sol, &soh,
			&til, &tih, &tol, &toh,
			&hil, &hih, &hol, &hoh,
			&lil, &lih, &lol, &loh,
		)
		if err != nil {
			log.Printf("[db] failed to scan profile row: %v", err)
			continue
		}

		tp := make(processor.ThresholdProfile)
		if sil != nil && sih != nil && sol != nil && soh != nil {
			tp["soil"] = processor.Bound{InnerLow: *sil, InnerHigh: *sih, OuterLow: *sol, OuterHigh: *soh}
		}
		if til != nil && tih != nil && tol != nil && toh != nil {
			tp["temp"] = processor.Bound{InnerLow: *til, InnerHigh: *tih, OuterLow: *tol, OuterHigh: *toh}
		}
		if hil != nil && hih != nil && hol != nil && hoh != nil {
			tp["hum"] = processor.Bound{InnerLow: *hil, InnerHigh: *hih, OuterLow: *hol, OuterHigh: *hoh}
		}
		if lil != nil && lih != nil && lol != nil && loh != nil {
			tp["light"] = processor.Bound{InnerLow: *lil, InnerHigh: *lih, OuterLow: *lol, OuterHigh: *loh}
		}

		profileMap[id] = tp
		if name == "default" {
			defaultProfile = tp
			foundDefault = true
		}
	}

	// 2. Fetch device to profile mappings
	devRows, err := DB.Query(`SELECT device_id, profile_id FROM device_profiles`)
	if err == nil {
		defer devRows.Close()
		for devRows.Next() {
			var dId string
			var pId int
			if err := devRows.Scan(&dId, &pId); err == nil {
				if prof, ok := profileMap[pId]; ok {
					deviceMap[dId] = prof
				}
			}
		}
	} else {
		log.Printf("[db] failed to query device_profiles: %v", err)
	}

	// 3. Update memory in processor
	if foundDefault {
		processor.UpdateThresholds(deviceMap, defaultProfile)
	} else {
		log.Printf("[db] WARNING: no profile named 'default' found, skipping update")
	}
}
