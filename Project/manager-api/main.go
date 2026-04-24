package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	_ "github.com/lib/pq"
	"github.com/rs/cors"
	"google.golang.org/api/idtoken"
)

var (
	DB             *sql.DB
	JWTSecret      = []byte(os.Getenv("JWT_SECRET"))
	GoogleClientID = os.Getenv("GOOGLE_CLIENT_ID")
	CAInstance     *CA
)

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

type User struct {
	ID        int       `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type InfrastructureNode struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"` // Kept for DB compatibility, but defaults to "Local Node"
	SiteID      int       `json:"site_id"`
	Address     string    `json:"address"`
	MQTTAddress string    `json:"mqtt_address"`
	Token       string    `json:"token"`
	CreatedAt   time.Time `json:"created_at"`
}

type EnrolledDevice struct {
	DeviceID  string    `json:"device_id"`
	AuthToken string    `json:"auth_token"`
	CreatedAt time.Time `json:"created_at"`
}

type Claims struct {
	Email string `json:"email"`
	Role  string `json:"role"`
	jwt.RegisteredClaims
}

func main() {
	if len(JWTSecret) == 0 {
		JWTSecret = []byte("dev-secret-keep-it-safe")
	}

	// Initialize CA
	var err error
	CAInstance, err = LoadOrCreateCA("certs/ca.crt", "certs/ca.key")
	if err != nil {
		log.Fatalf("Failed to initialize CA: %v", err)
	}

	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@postgresql.iot.kaminjitt.com:5432/potbuddy?sslmode=disable"
	}

	DB, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()

	// Public Routes
	mux.HandleFunc("POST /api/auth/login", loginHandler)

	// Protected API Routes
	mux.Handle("GET /api/profiles", authMiddleware(http.HandlerFunc(getProfiles)))
	mux.Handle("POST /api/profiles", authMiddleware(roleMiddleware([]string{"Super Admin", "Site Admin"}, http.HandlerFunc(createProfile))))
	mux.Handle("PUT /api/profiles/{id}", authMiddleware(roleMiddleware([]string{"Super Admin", "Site Admin"}, http.HandlerFunc(updateProfile))))
	mux.Handle("DELETE /api/profiles/{id}", authMiddleware(roleMiddleware([]string{"Super Admin"}, http.HandlerFunc(deleteProfile))))
	
	mux.Handle("GET /api/devices", authMiddleware(http.HandlerFunc(getDevices)))
	mux.Handle("PUT /api/devices/{id}", authMiddleware(roleMiddleware([]string{"Super Admin", "Site Admin"}, http.HandlerFunc(updateDeviceProfile))))
	
	mux.Handle("GET /api/infrastructure", authMiddleware(http.HandlerFunc(getInfrastructureHealth)))

	// Phase 2: Enrollment (Infrastructure)
	mux.Handle("GET /api/enrollment/nodes", authMiddleware(roleMiddleware([]string{"Super Admin"}, http.HandlerFunc(getEnrolledNodes))))
	mux.Handle("POST /api/enrollment/nodes", authMiddleware(roleMiddleware([]string{"Super Admin"}, http.HandlerFunc(enrollNode))))
	mux.Handle("PUT /api/enrollment/nodes/{id}", authMiddleware(roleMiddleware([]string{"Super Admin"}, http.HandlerFunc(updateEnrolledNode))))
	mux.Handle("DELETE /api/enrollment/nodes/{id}", authMiddleware(roleMiddleware([]string{"Super Admin"}, http.HandlerFunc(deleteEnrolledNode))))

	// Phase 2: Enrollment (Devices)
	mux.Handle("GET /api/enrollment/devices", authMiddleware(roleMiddleware([]string{"Super Admin", "Site Admin"}, http.HandlerFunc(getEnrolledDevices))))
	mux.Handle("POST /api/enrollment/devices", authMiddleware(roleMiddleware([]string{"Super Admin", "Site Admin"}, http.HandlerFunc(enrollDevice))))
	mux.Handle("DELETE /api/enrollment/devices/{id}", authMiddleware(roleMiddleware([]string{"Super Admin", "Site Admin"}, http.HandlerFunc(deleteEnrolledDevice))))

	// Phase 4: Bootstrapping (Public endpoint, secured by AuthToken)
	mux.HandleFunc("POST /api/enrollment/bootstrap", bootstrapDevice)

	mux.Handle("GET /api/enrollment/nodes/{id}/config", authMiddleware(roleMiddleware([]string{"Super Admin"}, http.HandlerFunc(downloadNodeConfig))))

	// User Management (Super Admin only)
	mux.Handle("GET /api/users", authMiddleware(roleMiddleware([]string{"Super Admin"}, http.HandlerFunc(getUsers))))
	mux.Handle("POST /api/users", authMiddleware(roleMiddleware([]string{"Super Admin"}, http.HandlerFunc(inviteUser))))
	mux.Handle("DELETE /api/users/{id}", authMiddleware(roleMiddleware([]string{"Super Admin"}, http.HandlerFunc(deleteUser))))

	// Serve the React frontend
	fs := http.FileServer(http.Dir("./frontend/dist"))
	mux.Handle("/", http.StripPrefix("/", fs))

	// CORS
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
	})
	handler := c.Handler(mux)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	log.Printf("Manager API listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}

// Utils
func generateToken(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

// Middleware
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
			return
		}

		tokenString := parts[1]
		claims := &Claims{}

		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return JWTSecret, nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		// Pass claims to context
		ctx := context.WithValue(r.Context(), "user", claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func roleMiddleware(allowedRoles []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := r.Context().Value("user").(*Claims)
		if !ok {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		isAllowed := false
		for _, role := range allowedRoles {
			if user.Role == role {
				isAllowed = true
				break
			}
		}

		if !isAllowed {
			http.Error(w, "Forbidden: insufficient permissions", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Handlers
func loginHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDToken string `json:"idToken"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if GoogleClientID == "" {
		log.Println("WARNING: GOOGLE_CLIENT_ID is not set. Skipping verification for dev.")
	} else {
		payload, err := idtoken.Validate(r.Context(), req.IDToken, GoogleClientID)
		if err != nil {
			http.Error(w, "Invalid ID token", http.StatusUnauthorized)
			return
		}
		_ = payload
	}

	payload, err := idtoken.ParsePayload(req.IDToken)
	if err != nil {
		http.Error(w, "Failed to parse token", http.StatusUnauthorized)
		return
	}
	email := payload.Claims["email"].(string)
	name := payload.Claims["name"].(string)

	var user User
	err = DB.QueryRow("SELECT id, email, name, role FROM users WHERE email = $1", email).
		Scan(&user.ID, &user.Email, &user.Name, &user.Role)

	if err == sql.ErrNoRows {
		http.Error(w, "User not invited", http.StatusForbidden)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Issue JWT
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		Email: user.Email,
		Role:  user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(JWTSecret)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Update name if it changed
	DB.Exec("UPDATE users SET name = $1 WHERE email = $2", name, email)

	json.NewEncoder(w).Encode(map[string]string{
		"token": tokenString,
		"role":  user.Role,
		"name":  name,
	})
}

// Phase 2: Infrastructure Enrollment Handlers
func getEnrolledNodes(w http.ResponseWriter, r *http.Request) {
	rows, err := DB.Query("SELECT id, name, type, site_id, address, mqtt_address, token, created_at FROM infrastructure_nodes ORDER BY id")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var nodes []InfrastructureNode
	for rows.Next() {
		var n InfrastructureNode
		var addr, mqttAddr sql.NullString
		if err := rows.Scan(&n.ID, &n.Name, &n.Type, &n.SiteID, &addr, &mqttAddr, &n.Token, &n.CreatedAt); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		n.Address = addr.String
		n.MQTTAddress = mqttAddr.String
		nodes = append(nodes, n)
	}

	w.Header().Set("Content-Type", "application/json")
	if nodes == nil {
		nodes = []InfrastructureNode{}
	}
	json.NewEncoder(w).Encode(nodes)
}

func enrollNode(w http.ResponseWriter, r *http.Request) {
	var n InfrastructureNode
	if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	n.Token = "pb_node_" + generateToken(16)
	if n.Type == "" {
		n.Type = "Local Node"
	}

	err := DB.QueryRow("INSERT INTO infrastructure_nodes (name, type, site_id, address, mqtt_address, token) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at", 
		n.Name, n.Type, n.SiteID, n.Address, n.MQTTAddress, n.Token).Scan(&n.ID, &n.CreatedAt)
	
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(n)
}

func updateEnrolledNode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var n InfrastructureNode
	if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	_, err := DB.Exec("UPDATE infrastructure_nodes SET name=$1, site_id=$2, address=$3, mqtt_address=$4 WHERE id=$5", 
		n.Name, n.SiteID, n.Address, n.MQTTAddress, id)
	
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(200)
}

func deleteEnrolledNode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_, err := DB.Exec("DELETE FROM infrastructure_nodes WHERE id=$1", id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(200)
}

// Phase 2: Device Enrollment Handlers
func getEnrolledDevices(w http.ResponseWriter, r *http.Request) {
	rows, err := DB.Query("SELECT device_id, auth_token, created_at FROM devices ORDER BY created_at DESC")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var devices []EnrolledDevice
	for rows.Next() {
		var d EnrolledDevice
		if err := rows.Scan(&d.DeviceID, &d.AuthToken, &d.CreatedAt); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		devices = append(devices, d)
	}

	w.Header().Set("Content-Type", "application/json")
	if devices == nil {
		devices = []EnrolledDevice{}
	}
	json.NewEncoder(w).Encode(devices)
}

func enrollDevice(w http.ResponseWriter, r *http.Request) {
	var d EnrolledDevice
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	if d.DeviceID == "" {
		http.Error(w, "device_id is required", 400)
		return
	}

	d.AuthToken = "pb_dev_" + generateToken(16)

	err := DB.QueryRow("INSERT INTO devices (device_id, auth_token) VALUES ($1, $2) RETURNING created_at", 
		d.DeviceID, d.AuthToken).Scan(&d.CreatedAt)
	
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// mTLS Certificate Generation for ESP32
	certPEM, keyPEM, err := CAInstance.SignCertificate(d.DeviceID)
	if err != nil {
		http.Error(w, "Failed to sign certificate: "+err.Error(), 500)
		return
	}

	bundle := map[string]string{
		"device_id":  d.DeviceID,
		"auth_token": d.AuthToken,
		"ca.crt":     string(CAInstance.CertPEM),
		"client.crt": string(certPEM),
		"client.key": string(keyPEM),
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(bundle)
}

func deleteEnrolledDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_, err := DB.Exec("DELETE FROM devices WHERE device_id=$1", id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(200)
}

func bootstrapDevice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AuthToken string `json:"auth_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", 400)
		return
	}

	var deviceID string
	err := DB.QueryRow("SELECT device_id FROM devices WHERE auth_token = $1", req.AuthToken).Scan(&deviceID)
	if err == sql.ErrNoRows {
		http.Error(w, "Invalid AuthToken", http.StatusUnauthorized)
		return
	} else if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// Generate mTLS Certificate for the device
	certPEM, keyPEM, err := CAInstance.SignCertificate(deviceID)
	if err != nil {
		http.Error(w, "Failed to sign certificate: "+err.Error(), 500)
		return
	}

	bundle := map[string]string{
		"device_id":  deviceID,
		"ca.crt":     string(CAInstance.CertPEM),
		"client.crt": string(certPEM),
		"client.key": string(keyPEM),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bundle)
}

func getUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := DB.Query("SELECT id, email, name, role, created_at FROM users ORDER BY id")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		var name sql.NullString
		if err := rows.Scan(&u.ID, &u.Email, &name, &u.Role, &u.CreatedAt); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		u.Name = name.String
		users = append(users, u)
	}

	w.Header().Set("Content-Type", "application/json")
	if users == nil {
		users = []User{}
	}
	json.NewEncoder(w).Encode(users)
}

func inviteUser(w http.ResponseWriter, r *http.Request) {
	var u User
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	err := DB.QueryRow("INSERT INTO users (email, role) VALUES ($1, $2) RETURNING id", u.Email, u.Role).Scan(&u.ID)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(u)
}

func deleteUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_, err := DB.Exec("DELETE FROM users WHERE id=$1", id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(200)
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
	if address == "" {
		return "offline"
	}
	// Add default port if missing
	if !strings.Contains(address, ":") {
		address = address + ":1883" // Default MQTT port
	}
	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		return "offline"
	}
	conn.Close()
	return "online"
}

func checkHTTP(url string) string {
	if url == "" {
		return "offline"
	}
	if !strings.HasPrefix(url, "http") {
		url = "http://" + url + ":8080/health" // Default Local Node health endpoint
	}
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

	// Core Components
	coreTargets := []struct {
		name      string
		typ       string
		checkType string
		addr      string
	}{
		{"Manager API", "backend", "always", "localhost:8081"},
		{"Manager UI", "frontend", "always", "localhost:8081"},
		{"Database", "postgres", "db", "postgresql.iot.kaminjitt.com:5432"},
		{"Prometheus Scraper", "prometheus", "http-raw", "http://prometheus.iot.kaminjitt.com:9090/-/healthy"},
		{"Grafana Dashboard", "grafana", "http-raw", "http://grafana.iot.kaminjitt.com:3000/api/health"},
	}

	for _, t := range coreTargets {
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
			case "http-raw":
				client := http.Client{Timeout: 2 * time.Second}
				resp, err := client.Get(t.addr)
				if err == nil && resp.StatusCode < 500 {
					status = "online"
				}
			}
			mu.Lock()
			results = append(results, ServiceHealth{Name: t.name, Type: t.typ, Status: status, Address: t.addr})
			mu.Unlock()
		}(t)
	}

	// Dynamic Site Nodes (Automatically tracks Node + Broker)
	rows, err := DB.Query("SELECT name, site_id, address, mqtt_address FROM infrastructure_nodes")
	if err == nil {
		for rows.Next() {
			var n struct {
				name     string
				mqttAddr sql.NullString
				addr     string
				site_id  int
			}
			if err := rows.Scan(&n.name, &n.site_id, &n.addr, &n.mqttAddr); err != nil {
				continue
			}

			// 1. Track the Local Node Processor
			wg.Add(1)
			go func(name, addr string) {
				defer wg.Done()
				status := checkHTTP(addr)
				mu.Lock()
				results = append(results, ServiceHealth{Name: name + " (Node)", Type: "Local Node", Status: status, Address: addr})
				mu.Unlock()
			}(n.name, n.addr)

			// 2. Track the Site MQTT Broker
			wg.Add(1)
			go func(name, addr, mqttAddr string, site_id int) {
				defer wg.Done()
				finalMQTTAddr := mqttAddr
				if finalMQTTAddr == "" {
					finalMQTTAddr = addr
				}
				status := checkTCP(finalMQTTAddr + ":1883")
				mu.Lock()
				results = append(results, ServiceHealth{
					Name:    name + " (MQTT)",
					Type:    "mqtt",
					Status:  status,
					Address: finalMQTTAddr + ":1883",
				})
				mu.Unlock()
			}(n.name, n.addr, n.mqttAddr.String, n.site_id)
		}
		rows.Close()
	}

	wg.Wait()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func downloadNodeConfig(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var n InfrastructureNode
	var addr, mqttAddr sql.NullString

	err := DB.QueryRow("SELECT id, name, type, site_id, address, mqtt_address, token FROM infrastructure_nodes WHERE id = $1", id).
		Scan(&n.ID, &n.Name, &n.Type, &n.SiteID, &addr, &mqttAddr, &n.Token)

	if err == sql.ErrNoRows {
		http.Error(w, "Node not found", 404)
		return
	} else if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	n.Address = addr.String
	n.MQTTAddress = mqttAddr.String

	// Use Node Address for MQTT if not specified
	mqttBroker := n.MQTTAddress
	if mqttBroker == "" {
		mqttBroker = n.Address
	}

	// Token suffix (after pb_node_)
	tokenSuffix := n.Token
	if strings.HasPrefix(tokenSuffix, "pb_node_") {
		tokenSuffix = tokenSuffix[8:]
	}
	if len(tokenSuffix) > 8 {
		tokenSuffix = tokenSuffix[:8]
	}

	// mTLS Configuration
	commonName := fmt.Sprintf("node-%d-%s", n.ID, tokenSuffix)
	certPEM, keyPEM, err := CAInstance.SignCertificate(commonName)
	if err != nil {
		http.Error(w, "Failed to sign certificate: "+err.Error(), 500)
		return
	}

	// Use port 8883 for mTLS
	mqttBrokerTLS := mqttBroker
	if !strings.Contains(mqttBrokerTLS, ":") {
		mqttBrokerTLS = mqttBrokerTLS + ":8883"
	} else {
		mqttBrokerTLS = strings.Replace(mqttBrokerTLS, ":1883", ":8883", 1)
		if !strings.Contains(mqttBrokerTLS, ":8883") && !strings.Contains(mqttBrokerTLS, ":1883") {
			// If it has a port but not 1883, we might need to be careful, but let's assume standard.
		}
	}

	if !strings.HasPrefix(mqttBrokerTLS, "ssl://") && !strings.HasPrefix(mqttBrokerTLS, "tcps://") {
		mqttBrokerTLS = "ssl://" + mqttBrokerTLS
	}

	// Generate YAML
	configYaml := fmt.Sprintf(`local_mqtt:
  broker: %q
  client_id: "potbuddy-local-node-%d"
  sub_topic: "potbuddy/+/raw"
  pub_topic: "potbuddy/processed"
  ca_file: "ca.crt"
  cert_file: "client.crt"
  key_file: "client.key"

cloud_mqtt:
  broker: "tcp://mqtt-0.iot.kaminjitt.com:1883"
  client_id: "potbuddy-cloud-publisher-%d"
  pub_topic: "potbuddy/telemetry"
  username: ""
  password: ""

http:
  port: 8080

store:
  buffer_size: 100

database:
  dsn: "postgres://postgres:postgres@postgresql.iot.kaminjitt.com:5432/potbuddy?sslmode=disable"

device_id: %q
`, mqttBrokerTLS, n.ID, n.ID, commonName)

	bundle := map[string]string{
		"config.yaml": configYaml,
		"ca.crt":      string(CAInstance.CertPEM),
		"client.crt":  string(certPEM),
		"client.key":  string(keyPEM),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bundle)
}
