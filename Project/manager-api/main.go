package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"

	_ "github.com/lib/pq"
	"github.com/rs/cors"
)

var DB *sql.DB

type Profile struct {
	ID             int      `json:"id"`
	Name           string   `json:"name"`
	SoilInnerLow   *float64 `json:"soil_inner_low"`
	SoilInnerHigh  *float64 `json:"soil_inner_high"`
	SoilOuterLow   *float64 `json:"soil_outer_low"`
	SoilOuterHigh  *float64 `json:"soil_outer_high"`
	TempInnerLow   *float64 `json:"temp_inner_low"`
	TempInnerHigh  *float64 `json:"temp_inner_high"`
	TempOuterLow   *float64 `json:"temp_outer_low"`
	TempOuterHigh  *float64 `json:"temp_outer_high"`
	HumInnerLow    *float64 `json:"hum_inner_low"`
	HumInnerHigh   *float64 `json:"hum_inner_high"`
	HumOuterLow    *float64 `json:"hum_outer_low"`
	HumOuterHigh   *float64 `json:"hum_outer_high"`
	LightInnerLow  *float64 `json:"light_inner_low"`
	LightInnerHigh *float64 `json:"light_inner_high"`
	LightOuterLow  *float64 `json:"light_outer_low"`
	LightOuterHigh *float64 `json:"light_outer_high"`
}

type Device struct {
	DeviceID  string `json:"device_id"`
	ProfileID int    `json:"profile_id"`
}

func main() {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@10.0.0.66:5432/potbuddy?sslmode=disable"
	}

	var err error
	DB, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()

	// API Routes
	mux.HandleFunc("GET /api/profiles", getProfiles)
	mux.HandleFunc("POST /api/profiles", createProfile)
	mux.HandleFunc("PUT /api/profiles/{id}", updateProfile)
	mux.HandleFunc("DELETE /api/profiles/{id}", deleteProfile)
	
	mux.HandleFunc("GET /api/devices", getDevices)
	mux.HandleFunc("PUT /api/devices/{id}", updateDeviceProfile)

	// Serve the React frontend locally if the fallback is activated
	// Assumes the command runs in a direct path context alongside ./frontend/dist
	fs := http.FileServer(http.Dir("./frontend/dist"))
	mux.Handle("/", http.StripPrefix("/", fs))

	// Allow all origins for dev testing
	c := cors.AllowAll()
	handler := c.Handler(mux)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	log.Printf("Manager API listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}

func getProfiles(w http.ResponseWriter, r *http.Request) {
	rows, err := DB.Query("SELECT * FROM profiles ORDER BY id")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var profiles []Profile
	for rows.Next() {
		var p Profile
		err := rows.Scan(&p.ID, &p.Name,
			&p.SoilInnerLow, &p.SoilInnerHigh, &p.SoilOuterLow, &p.SoilOuterHigh,
			&p.TempInnerLow, &p.TempInnerHigh, &p.TempOuterLow, &p.TempOuterHigh,
			&p.HumInnerLow, &p.HumInnerHigh, &p.HumOuterLow, &p.HumOuterHigh,
			&p.LightInnerLow, &p.LightInnerHigh, &p.LightOuterLow, &p.LightOuterHigh,
		)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		profiles = append(profiles, p)
	}

	w.Header().Set("Content-Type", "application/json")
	if profiles == nil {
		profiles = []Profile{}
	}
	json.NewEncoder(w).Encode(profiles)
}

func createProfile(w http.ResponseWriter, r *http.Request) {
	var p Profile
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	err := DB.QueryRow(`
		INSERT INTO profiles (name, 
			soil_inner_low, soil_inner_high, soil_outer_low, soil_outer_high,
			temp_inner_low, temp_inner_high, temp_outer_low, temp_outer_high,
			hum_inner_low, hum_inner_high, hum_outer_low, hum_outer_high,
			light_inner_low, light_inner_high, light_outer_low, light_outer_high
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17) RETURNING id
	`, p.Name,
		p.SoilInnerLow, p.SoilInnerHigh, p.SoilOuterLow, p.SoilOuterHigh,
		p.TempInnerLow, p.TempInnerHigh, p.TempOuterLow, p.TempOuterHigh,
		p.HumInnerLow, p.HumInnerHigh, p.HumOuterLow, p.HumOuterHigh,
		p.LightInnerLow, p.LightInnerHigh, p.LightOuterLow, p.LightOuterHigh,
	).Scan(&p.ID)

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(p)
}

func updateProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var p Profile
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	_, err := DB.Exec(`
		UPDATE profiles SET name=$1, 
			soil_inner_low=$2, soil_inner_high=$3, soil_outer_low=$4, soil_outer_high=$5,
			temp_inner_low=$6, temp_inner_high=$7, temp_outer_low=$8, temp_outer_high=$9,
			hum_inner_low=$10, hum_inner_high=$11, hum_outer_low=$12, hum_outer_high=$13,
			light_inner_low=$14, light_inner_high=$15, light_outer_low=$16, light_outer_high=$17
		WHERE id=$18
	`, p.Name,
		p.SoilInnerLow, p.SoilInnerHigh, p.SoilOuterLow, p.SoilOuterHigh,
		p.TempInnerLow, p.TempInnerHigh, p.TempOuterLow, p.TempOuterHigh,
		p.HumInnerLow, p.HumInnerHigh, p.HumOuterLow, p.HumOuterHigh,
		p.LightInnerLow, p.LightInnerHigh, p.LightOuterLow, p.LightOuterHigh,
		id,
	)

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(200)
}

func deleteProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_, err := DB.Exec("DELETE FROM profiles WHERE id=$1", id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(200)
}

func getDevices(w http.ResponseWriter, r *http.Request) {
	rows, err := DB.Query("SELECT device_id, profile_id FROM device_profiles ORDER BY device_id")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var devices []Device
	for rows.Next() {
		var d Device
		err := rows.Scan(&d.DeviceID, &d.ProfileID)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		devices = append(devices, d)
	}

	w.Header().Set("Content-Type", "application/json")
	if devices == nil {
		devices = []Device{}
	}
	json.NewEncoder(w).Encode(devices)
}

func updateDeviceProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		ProfileID int `json:"profile_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	_, err := DB.Exec("UPDATE device_profiles SET profile_id=$1 WHERE device_id=$2", req.ProfileID, id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(200)
}
