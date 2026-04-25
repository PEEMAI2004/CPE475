package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
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

	rr := httptest.NewRecorder()
	s.Router.ServeHTTP(rr, req) // Testing the unauthenticated handler directly

	// NOTE: In real tests we'd need to mock authMiddleware or use a helper to generate valid JWTs
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

	rr := httptest.NewRecorder()
	s.Router.ServeHTTP(rr, req)
}
