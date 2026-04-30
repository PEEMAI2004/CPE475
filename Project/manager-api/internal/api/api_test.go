package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/golang-jwt/jwt/v5"
	"github.com/potbuddy/manager-api/internal/models"
)

func setupTestServer(t *testing.T) (*Server, sqlmock.Sqlmock, func()) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to initialize sqlmock: %v", err)
	}
	
	s := NewServer(db, nil, []byte("test-secret"), "test-google-id")

	return s, mock, func() {
		db.Close()
	}
}

func generateTestToken(secret []byte, role string) string {
	claims := &models.Claims{
		Email: "test@example.com",
		Role:  role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString(secret)
	return tokenString
}

func TestGetProfiles(t *testing.T) {
	s, mock, cleanup := setupTestServer(t)
	defer cleanup()

	rows := sqlmock.NewRows([]string{
		"id", "name", 
		"soil_inner_low", "soil_inner_high", "soil_outer_low", "soil_outer_high",
		"temp_inner_low", "temp_inner_high", "temp_outer_low", "temp_outer_high",
		"hum_inner_low", "hum_inner_high", "hum_outer_low", "hum_outer_high",
		"light_inner_low", "light_inner_high", "light_outer_low", "light_outer_high",
	}).AddRow(
		1, "test_profile", 
		100.0, 200.0, 50.0, 250.0,
		20.0, 30.0, 15.0, 35.0,
		40.0, 60.0, 30.0, 70.0,
		1000.0, 3000.0, 500.0, 5000.0,
	)

	mock.ExpectQuery("SELECT \\* FROM profiles ORDER BY id").WillReturnRows(rows)

	req, err := http.NewRequest("GET", "/api/profiles", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+generateTestToken(s.JWTSecret, "Viewer"))

	rr := httptest.NewRecorder()
	s.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestUpdateDeviceProfile(t *testing.T) {
	s, mock, cleanup := setupTestServer(t)
	defer cleanup()

	body := bytes.NewBuffer([]byte(`{"profile_id": 4}`))

	mock.ExpectExec("UPDATE device_profiles SET profile_id=\\$1 WHERE device_id=\\$2").
		WithArgs(4, "hardware-123").
		WillReturnResult(sqlmock.NewResult(1, 1))

	req, err := http.NewRequest("PUT", "/api/devices/hardware-123", body)
	if err != nil {
		t.Fatal(err)
	}
	req.SetPathValue("id", "hardware-123")
	req.Header.Set("Authorization", "Bearer "+generateTestToken(s.JWTSecret, "Super Admin"))

	rr := httptest.NewRecorder()
	s.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestRegenNodeToken(t *testing.T) {
	s, mock, cleanup := setupTestServer(t)
	defer cleanup()

	mock.ExpectExec("UPDATE infrastructure_nodes SET token=\\$1 WHERE id=\\$2").
		WithArgs(sqlmock.AnyArg(), "1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	req, err := http.NewRequest("POST", "/api/enrollment/nodes/1/regen-token", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.SetPathValue("id", "1")
	req.Header.Set("Authorization", "Bearer "+generateTestToken(s.JWTSecret, "Super Admin"))

	rr := httptest.NewRecorder()
	s.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestMachineAuthMiddleware(t *testing.T) {
	s, mock, cleanup := setupTestServer(t)
	defer cleanup()

	// Test Node Token
	mock.ExpectQuery("SELECT name FROM infrastructure_nodes WHERE token = \\$1").
		WithArgs("pb_node_valid").
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("test-node"))

	req, _ := http.NewRequest("POST", "/api/infrastructure/heartbeat", nil)
	req.Header.Set("X-PotBuddy-Token", "pb_node_valid")

	rr := httptest.NewRecorder()
	s.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 for valid node token, got %d", rr.Code)
	}

	// Test Device Token
	mock.ExpectQuery("SELECT device_id FROM devices WHERE auth_token = \\$1").
		WithArgs("pb_dev_valid").
		WillReturnRows(sqlmock.NewRows([]string{"device_id"}).AddRow("test-device"))

	req, _ = http.NewRequest("POST", "/api/infrastructure/heartbeat", nil)
	req.Header.Set("Authorization", "Bearer pb_dev_valid")

	rr = httptest.NewRecorder()
	s.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 for valid device token, got %d", rr.Code)
	}

	// Test Invalid Token
	req, _ = http.NewRequest("POST", "/api/infrastructure/heartbeat", nil)
	req.Header.Set("X-PotBuddy-Token", "pb_invalid")

	rr = httptest.NewRecorder()
	s.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 for invalid token, got %d", rr.Code)
	}
}
