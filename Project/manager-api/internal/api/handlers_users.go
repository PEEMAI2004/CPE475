package api

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/potbuddy/manager-api/internal/models"
)

func (s *Server) getUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.Query("SELECT id, email, name, role, created_at FROM users ORDER BY id")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
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
		users = []models.User{}
	}
	json.NewEncoder(w).Encode(users)
}

func (s *Server) inviteUser(w http.ResponseWriter, r *http.Request) {
	var u models.User
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	err := s.DB.QueryRow("INSERT INTO users (email, role) VALUES ($1, $2) RETURNING id", u.Email, u.Role).Scan(&u.ID)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(u)
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_, err := s.DB.Exec("DELETE FROM users WHERE id=$1", id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(200)
}
