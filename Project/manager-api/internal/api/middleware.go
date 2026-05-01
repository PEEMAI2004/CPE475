package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/potbuddy/manager-api/internal/models"
)

func (s *Server) authMiddleware(next http.Handler) http.Handler {
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
		claims := &models.Claims{}

		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return s.JWTSecret, nil
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

func (s *Server) roleMiddleware(allowedRoles []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := r.Context().Value("user").(*models.Claims)
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

func (s *Server) machineAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-PotBuddy-Token")
		if token == "" {
			// Also support Authorization header
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				token = authHeader[7:]
			}
		}

		if token == "" {
			http.Error(w, "Missing AuthToken", http.StatusUnauthorized)
			return
		}

		var identity models.MachineIdentity

		// Check if it's a node token
		if strings.HasPrefix(token, "pb_node_") {
			var nodeName string
			var nodeID int
			err := s.DB.QueryRow("SELECT id, name FROM infrastructure_nodes WHERE token = $1", token).Scan(&nodeID, &nodeName)
			if err == nil {
				identity.ID = nodeName
				identity.DBID = nodeID
				identity.Type = "node"
			}
		} else if strings.HasPrefix(token, "pb_dev_") {
			var deviceID string
			err := s.DB.QueryRow("SELECT device_id FROM devices WHERE auth_token = $1", token).Scan(&deviceID)
			if err == nil {
				identity.ID = deviceID
				identity.Type = "device"
			}
		}

		if identity.ID == "" {
			http.Error(w, "Invalid AuthToken", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), "identity", &identity)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
