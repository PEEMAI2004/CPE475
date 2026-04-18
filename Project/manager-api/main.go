package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

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
	Online    bool   `json:"online"`
	Health    string `json:"health"`
}

func main() {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@postgresql.iot.kaminjitt.com:5432/potbuddy?sslmode=disable"
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
	
	mux.HandleFunc("GET /api/infrastructure", getInfrastructureHealth)

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

type PromResponse struct {
	Data struct {
		Result []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

func fetchPrometheus(query string) map[string]string {
	result := make(map[string]string)
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://prometheus.iot.kaminjitt.com:9090/api/v1/query?query=" + query)
	if err != nil {
		return result
	}
	defer resp.Body.Close()

	var pResp PromResponse
	if err := json.NewDecoder(resp.Body).Decode(&pResp); err != nil {
		return result
	}

	for _, r := range pResp.Data.Result {
		dev := r.Metric["device"]
		if dev == "" {
			continue
		}
		if len(r.Value) == 2 {
			if valStr, ok := r.Value[1].(string); ok {
				result[dev] = valStr
			}
		}
	}
	return result
}

func getDevices(w http.ResponseWriter, r *http.Request) {
	onlineMap := fetchPrometheus("potbuddy_device_online")
	// Using %22 to URL encode quotes since go client might not do it implicitly
	healthMap := fetchPrometheus("potbuddy_health_status%7Bfield=%22overall%22%7D")

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
		
		d.Online = (onlineMap[d.DeviceID] == "1")
		
		healthStr := "unknown"
		if val, exists := healthMap[d.DeviceID]; exists {
			switch val {
			case "0":
				healthStr = "healthy"
			case "1":
				healthStr = "warning"
			case "2":
				healthStr = "critical"
			}
		}
		if !d.Online {
			healthStr = "unknown"
		}
		d.Health = healthStr

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

type ServiceHealth struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Status  string `json:"status"`
	Address string `json:"address"`
}

func checkTCP(address string) string {
	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		return "offline"
	}
	conn.Close()
	return "online"
}

func checkHTTP(url string) string {
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil || resp.StatusCode >= 500 {
		return "offline"
	}
	return "online"
}

func getInfrastructureHealth(w http.ResponseWriter, r *http.Request) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := []ServiceHealth{}

	targets := []struct {
		name      string
		typ       string
		checkType string
		addr      string
	}{
		{"Manager API", "backend", "always", "localhost:8081"},
		{"Manager UI", "frontend", "always", "localhost:8081"},
		{"Database", "postgres", "db", "postgresql.iot.kaminjitt.com:5432"},
		{"Cloud MQTT Site 0", "mqtt", "tcp", "mqtt-0.iot.kaminjitt.com:1883"},
		{"Cloud MQTT Site 1", "mqtt", "tcp", "mqtt-1.iot.kaminjitt.com:1883"},
		{"Prometheus Scraper", "prometheus", "http", "http://prometheus.iot.kaminjitt.com:9090/-/healthy"},
		{"Grafana Dashboard", "grafana", "http", "http://grafana.iot.kaminjitt.com:3000/api/health"},
		{"Local Node Site 0", "processor", "http", "http://debian-0.iot.kaminjitt.com:8080/health"},
		{"Local Node Site 1", "processor", "http", "http://debian-1.iot.kaminjitt.com:8080/health"},
	}

	for _, t := range targets {
		wg.Add(1)
		go func(t struct{ name, typ, checkType, addr string }) {
			defer wg.Done()
			status := "offline"
			switch t.checkType {
			case "always":
				status = "online"
			case "db":
				if err := DB.Ping(); err == nil {
					status = "online"
				}
			case "tcp":
				status = checkTCP(t.addr)
			case "http":
				status = checkHTTP(t.addr)
			}
			
			// Hide pure HTTP prefixes for display cleanup if requested, but UI can do it too.
			mu.Lock()
			results = append(results, ServiceHealth{
				Name: t.name, Type: t.typ, Status: status, Address: t.addr,
			})
			mu.Unlock()
		}(t)
	}

	wg.Wait()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}
