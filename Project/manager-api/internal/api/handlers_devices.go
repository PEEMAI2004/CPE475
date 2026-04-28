package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/potbuddy/manager-api/internal/models"
)

func (s *Server) getDevices(w http.ResponseWriter, r *http.Request) {
	onlineMap := s.fetchPrometheus("potbuddy_device_online")
	healthMap := s.fetchPrometheus("potbuddy_health_status%7Bfield=%22overall%22%7D")

	rows, err := s.DB.Query("SELECT device_id, profile_id FROM device_profiles ORDER BY device_id")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var devices []models.Device
	for rows.Next() {
		var d models.Device
		err := rows.Scan(&d.DeviceID, &d.ProfileID)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		
		lookupID := strings.ToLower(d.DeviceID)
		d.Online = (onlineMap[lookupID] == "1")
		
		healthStr := "unknown"
		if val, exists := healthMap[lookupID]; exists {
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
		devices = []models.Device{}
	}
	json.NewEncoder(w).Encode(devices)
}

func (s *Server) updateDeviceProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		ProfileID int `json:"profile_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	_, err := s.DB.Exec("UPDATE device_profiles SET profile_id=$1 WHERE device_id=$2", req.ProfileID, id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(200)
}
