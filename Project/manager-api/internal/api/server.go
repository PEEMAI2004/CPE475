package api

import (
	"database/sql"
	"net/http"
	"os"
	"strings"

	"github.com/potbuddy/manager-api/internal/ca"
)

type Server struct {
	DB             *sql.DB
	CA             *ca.CA
	JWTSecret      []byte
	GoogleClientID string
	Router         *http.ServeMux
}

func NewServer(db *sql.DB, c *ca.CA, jwtSecret []byte, googleClientID string) *Server {
	s := &Server{
		DB:             db,
		CA:             c,
		JWTSecret:      jwtSecret,
		GoogleClientID: googleClientID,
		Router:         http.NewServeMux(),
	}
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	// Public Routes
	s.Router.HandleFunc("POST /api/auth/login", s.loginHandler)

	// Protected API Routes
	s.Router.Handle("GET /api/profiles", s.authMiddleware(http.HandlerFunc(s.getProfiles)))
	s.Router.Handle("POST /api/profiles", s.authMiddleware(s.roleMiddleware([]string{"Super Admin", "Site Admin"}, http.HandlerFunc(s.createProfile))))
	s.Router.Handle("PUT /api/profiles/{id}", s.authMiddleware(s.roleMiddleware([]string{"Super Admin", "Site Admin"}, http.HandlerFunc(s.updateProfile))))
	s.Router.Handle("DELETE /api/profiles/{id}", s.authMiddleware(s.roleMiddleware([]string{"Super Admin"}, http.HandlerFunc(s.deleteProfile))))

	s.Router.Handle("GET /api/devices", s.authMiddleware(http.HandlerFunc(s.getDevices)))
	s.Router.Handle("PUT /api/devices/{id}", s.authMiddleware(s.roleMiddleware([]string{"Super Admin", "Site Admin"}, http.HandlerFunc(s.updateDeviceProfile))))

	s.Router.Handle("GET /api/infrastructure", s.authMiddleware(http.HandlerFunc(s.getInfrastructureHealth)))

	// Phase 2: Enrollment (Infrastructure)
	s.Router.Handle("GET /api/enrollment/nodes", s.authMiddleware(s.roleMiddleware([]string{"Super Admin"}, http.HandlerFunc(s.getEnrolledNodes))))
	s.Router.Handle("POST /api/enrollment/nodes", s.authMiddleware(s.roleMiddleware([]string{"Super Admin"}, http.HandlerFunc(s.enrollNode))))
	s.Router.Handle("PUT /api/enrollment/nodes/{id}", s.authMiddleware(s.roleMiddleware([]string{"Super Admin"}, http.HandlerFunc(s.updateEnrolledNode))))
	s.Router.Handle("DELETE /api/enrollment/nodes/{id}", s.authMiddleware(s.roleMiddleware([]string{"Super Admin"}, http.HandlerFunc(s.deleteEnrolledNode))))

	// Phase 2: Enrollment (Devices)
	s.Router.Handle("GET /api/enrollment/devices", s.authMiddleware(s.roleMiddleware([]string{"Super Admin", "Site Admin"}, http.HandlerFunc(s.getEnrolledDevices))))
	s.Router.Handle("POST /api/enrollment/devices", s.authMiddleware(s.roleMiddleware([]string{"Super Admin", "Site Admin"}, http.HandlerFunc(s.enrollDevice))))
	s.Router.Handle("DELETE /api/enrollment/devices/{id}", s.authMiddleware(s.roleMiddleware([]string{"Super Admin", "Site Admin"}, http.HandlerFunc(s.deleteEnrolledDevice))))

	// Phase 4: Bootstrapping (Public endpoint, secured by AuthToken)
	s.Router.HandleFunc("POST /api/enrollment/bootstrap", s.bootstrapDevice)
	s.Router.Handle("POST /api/enrollment/bootstrap/node", s.machineAuthMiddleware(http.HandlerFunc(s.bootstrapNodeConfig)))
	s.Router.Handle("POST /api/enrollment/bootstrap/mqtt", s.machineAuthMiddleware(http.HandlerFunc(s.bootstrapBrokerCert)))

	s.Router.Handle("/api/enrollment/nodes/{id}/config", s.authMiddleware(s.roleMiddleware([]string{"Super Admin"}, http.HandlerFunc(s.downloadNodeConfig))))
	s.Router.Handle("POST /api/enrollment/nodes/{id}/server-cert", s.authMiddleware(s.roleMiddleware([]string{"Super Admin"}, http.HandlerFunc(s.generateServerCert))))
	s.Router.Handle("POST /api/enrollment/nodes/{id}/client-cert", s.authMiddleware(s.roleMiddleware([]string{"Super Admin"}, http.HandlerFunc(s.generateClientCert))))

	s.Router.Handle("POST /api/enrollment/nodes/{id}/regen-token", s.authMiddleware(s.roleMiddleware([]string{"Super Admin"}, http.HandlerFunc(s.regenNodeToken))))
	s.Router.Handle("POST /api/enrollment/devices/{id}/regen-token", s.authMiddleware(s.roleMiddleware([]string{"Super Admin", "Site Admin"}, http.HandlerFunc(s.regenDeviceAuthToken))))

	// Machine-to-Machine API (Secured by Node/Device Tokens)
	s.Router.Handle("POST /api/infrastructure/heartbeat", s.machineAuthMiddleware(http.HandlerFunc(s.reportHeartbeat)))

	// User Management (Super Admin only)
	s.Router.Handle("GET /api/users", s.authMiddleware(s.roleMiddleware([]string{"Super Admin"}, http.HandlerFunc(s.getUsers))))
	s.Router.Handle("POST /api/users", s.authMiddleware(s.roleMiddleware([]string{"Super Admin"}, http.HandlerFunc(s.inviteUser))))
	s.Router.Handle("DELETE /api/users/{id}", s.authMiddleware(s.roleMiddleware([]string{"Super Admin"}, http.HandlerFunc(s.deleteUser))))
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api") {
		s.Router.ServeHTTP(w, r)
		return
	}

	// Serve the React frontend
	path := "./frontend/dist" + r.URL.Path
	info, err := os.Stat(path)
	if os.IsNotExist(err) || info.IsDir() {
		// Fallback to index.html for SPA routing or directory requests
		http.ServeFile(w, r, "./frontend/dist/index.html")
		return
	}

	// Serve the static file directly
	http.ServeFile(w, r, path)
}
