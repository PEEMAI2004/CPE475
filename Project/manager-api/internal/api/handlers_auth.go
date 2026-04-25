package api

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/potbuddy/manager-api/internal/models"
	"google.golang.org/api/idtoken"
)

func (s *Server) generateToken(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

func (s *Server) loginHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDToken string `json:"idToken"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if s.GoogleClientID == "" {
		log.Println("WARNING: GOOGLE_CLIENT_ID is not set. Skipping verification for dev.")
	} else {
		payload, err := idtoken.Validate(r.Context(), req.IDToken, s.GoogleClientID)
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

	var user models.User
	err = s.DB.QueryRow("SELECT id, email, name, role FROM users WHERE email = $1", email).
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
	claims := &models.Claims{
		Email: user.Email,
		Role:  user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.JWTSecret)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Update name if it changed
	s.DB.Exec("UPDATE users SET name = $1 WHERE email = $2", name, email)

	json.NewEncoder(w).Encode(map[string]string{
		"token": tokenString,
		"role":  user.Role,
		"name":  name,
	})
}
