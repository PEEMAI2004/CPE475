package api

import (
	"encoding/json"
	"net/http"

	"github.com/potbuddy/manager-api/internal/models"
)

func (s *Server) getProfiles(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.Query("SELECT id, name, soil_inner_low, soil_inner_high, soil_outer_low, soil_outer_high, temp_inner_low, temp_inner_high, temp_outer_low, temp_outer_high, hum_inner_low, hum_inner_high, hum_outer_low, hum_outer_high, light_inner_low, light_inner_high, light_outer_low, light_outer_high, sun_direct_threshold, max_direct_sun_minutes, min_total_sun_minutes FROM profiles ORDER BY id")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var profiles []models.Profile
	for rows.Next() {
		var p models.Profile
		err := rows.Scan(&p.ID, &p.Name,
			&p.SoilInnerLow, &p.SoilInnerHigh, &p.SoilOuterLow, &p.SoilOuterHigh,
			&p.TempInnerLow, &p.TempInnerHigh, &p.TempOuterLow, &p.TempOuterHigh,
			&p.HumInnerLow, &p.HumInnerHigh, &p.HumOuterLow, &p.HumOuterHigh,
			&p.LightInnerLow, &p.LightInnerHigh, &p.LightOuterLow, &p.LightOuterHigh,
			&p.SunDirectThreshold, &p.MaxDirectSunMinutes, &p.MinTotalSunMinutes,
		)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		profiles = append(profiles, p)
	}

	w.Header().Set("Content-Type", "application/json")
	if profiles == nil {
		profiles = []models.Profile{}
	}
	json.NewEncoder(w).Encode(profiles)
}

func (s *Server) createProfile(w http.ResponseWriter, r *http.Request) {
	var p models.Profile
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	err := s.DB.QueryRow(`
		INSERT INTO profiles (name, 
			soil_inner_low, soil_inner_high, soil_outer_low, soil_outer_high,
			temp_inner_low, temp_inner_high, temp_outer_low, temp_outer_high,
			hum_inner_low, hum_inner_high, hum_outer_low, hum_outer_high,
			light_inner_low, light_inner_high, light_outer_low, light_outer_high,
			sun_direct_threshold, max_direct_sun_minutes, min_total_sun_minutes
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20) RETURNING id
	`, p.Name,
		p.SoilInnerLow, p.SoilInnerHigh, p.SoilOuterLow, p.SoilOuterHigh,
		p.TempInnerLow, p.TempInnerHigh, p.TempOuterLow, p.TempOuterHigh,
		p.HumInnerLow, p.HumInnerHigh, p.HumOuterLow, p.HumOuterHigh,
		p.LightInnerLow, p.LightInnerHigh, p.LightOuterLow, p.LightOuterHigh,
		p.SunDirectThreshold, p.MaxDirectSunMinutes, p.MinTotalSunMinutes,
	).Scan(&p.ID)

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(p)
}

func (s *Server) updateProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var p models.Profile
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	_, err := s.DB.Exec(`
		UPDATE profiles SET name=$1, 
			soil_inner_low=$2, soil_inner_high=$3, soil_outer_low=$4, soil_outer_high=$5,
			temp_inner_low=$6, temp_inner_high=$7, temp_outer_low=$8, temp_outer_high=$9,
			hum_inner_low=$10, hum_inner_high=$11, hum_outer_low=$12, hum_outer_high=$13,
			light_inner_low=$14, light_inner_high=$15, light_outer_low=$16, light_outer_high=$17,
			sun_direct_threshold=$18, max_direct_sun_minutes=$19, min_total_sun_minutes=$20
		WHERE id=$21
	`, p.Name,
		p.SoilInnerLow, p.SoilInnerHigh, p.SoilOuterLow, p.SoilOuterHigh,
		p.TempInnerLow, p.TempInnerHigh, p.TempOuterLow, p.TempOuterHigh,
		p.HumInnerLow, p.HumInnerHigh, p.HumOuterLow, p.HumOuterHigh,
		p.LightInnerLow, p.LightInnerHigh, p.LightOuterLow, p.LightOuterHigh,
		p.SunDirectThreshold, p.MaxDirectSunMinutes, p.MinTotalSunMinutes,
		id,
	)

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(200)
}

func (s *Server) deleteProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_, err := s.DB.Exec("DELETE FROM profiles WHERE id=$1", id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(200)
}
