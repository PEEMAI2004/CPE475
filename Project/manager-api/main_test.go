package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// Helper to setup mock DB and request recorder
func setupTestDB(t *testing.T) (sqlmock.Sqlmock, func()) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to initialize sqlmock: %v", err)
	}
	
	// Override global DB with mocked database
	DB = db

	// Cleanup closure
	return mock, func() {
		db.Close()
	}
}

func floatPtr(v float64) *float64 { return &v }

func TestGetProfiles(t *testing.T) {
	mock, cleanup := setupTestDB(t)
	defer cleanup()

	// Provide mock SQL rows
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

	// Expect the query that getProfiles calls
	mock.ExpectQuery("SELECT \\* FROM profiles ORDER BY id").WillReturnRows(rows)

	req, err := http.NewRequest("GET", "/api/profiles", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(getProfiles)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("expected status 200, got %v", status)
	}

	var profiles []Profile
	if err := json.Unmarshal(rr.Body.Bytes(), &profiles); err != nil {
		t.Errorf("failed to unmarshal response: %v", err)
	}

	if len(profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(profiles))
	}
	
	if profiles[0].Name != "test_profile" {
		t.Errorf("expected 'test_profile', got '%s'", profiles[0].Name)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %s", err)
	}
}

func TestGetDevices(t *testing.T) {
	mock, cleanup := setupTestDB(t)
	defer cleanup()

	// Provide mock SQL rows for devices
	rows := sqlmock.NewRows([]string{"device_id", "profile_id"}).
		AddRow("esp32-alpha", 1).
		AddRow("esp32-beta", 2)

	mock.ExpectQuery("SELECT device_id, profile_id FROM device_profiles ORDER BY device_id").WillReturnRows(rows)

	req, err := http.NewRequest("GET", "/api/devices", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(getDevices)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("expected status 200, got %v", status)
	}

	var devices []Device
	if err := json.Unmarshal(rr.Body.Bytes(), &devices); err != nil {
		t.Errorf("failed to unmarshal device response: %v", err)
	}

	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(devices))
	}

	if devices[0].DeviceID != "esp32-alpha" {
		t.Errorf("expected device ID esp32-alpha, got '%s'", devices[0].DeviceID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %s", err)
	}
}

func TestUpdateDeviceProfile(t *testing.T) {
	mock, cleanup := setupTestDB(t)
	defer cleanup()

	// Simulate body: {"profile_id": 4}
	body := bytes.NewBuffer([]byte(`{"profile_id": 4}`))

	// Expect the specific execution
	mock.ExpectExec("UPDATE device_profiles SET profile_id=\\$1 WHERE device_id=\\$2").
		WithArgs(4, "hardware-123").
		WillReturnResult(sqlmock.NewResult(1, 1))

	req, err := http.NewRequest("PUT", "/api/devices/hardware-123", body)
	if err != nil {
		t.Fatal(err)
	}
	
	// Set the pathvalue dynamically for Go 1.22 testing
	req.SetPathValue("id", "hardware-123")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(updateDeviceProfile)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("expected status 200, got %v", status)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %s", err)
	}
}

func TestCreateProfile(t *testing.T) {
	mock, cleanup := setupTestDB(t)
	defer cleanup()

	body := bytes.NewBuffer([]byte(`{
		"name": "desert",
		"soil_inner_low": 3000,
		"soil_inner_high": 4000,
		"soil_outer_low": 2800,
		"soil_outer_high": 4095
	}`))

	// The query uses RETURNING id to get the generated ID
	mock.ExpectQuery("INSERT INTO profiles").
		WithArgs("desert", 
			float64(3000), float64(4000), float64(2800), float64(4095),
			nil, nil, nil, nil,
			nil, nil, nil, nil,
			nil, nil, nil, nil).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(99))

	req, err := http.NewRequest("POST", "/api/profiles", body)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(createProfile)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("expected status 200, got %v (body: %s)", status, rr.Body.String())
	}

	// Verify returned ID attached
	var p Profile
	json.Unmarshal(rr.Body.Bytes(), &p)
	if p.ID != 99 {
		t.Errorf("expected returning ID 99, got %v", p.ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %s", err)
	}
}

func TestUpdateProfile(t *testing.T) {
	mock, cleanup := setupTestDB(t)
	defer cleanup()

	body := bytes.NewBuffer([]byte(`{
		"name": "temperate",
		"soil_inner_low": 2000,
		"soil_inner_high": 3000,
		"soil_outer_low": 1500,
		"soil_outer_high": 3500
	}`))

	mock.ExpectExec("UPDATE profiles SET name=\\$1").
		WithArgs("temperate", 
			float64(2000), float64(3000), float64(1500), float64(3500),
			nil, nil, nil, nil, 
			nil, nil, nil, nil, 
			nil, nil, nil, nil,
			"5").
		WillReturnResult(sqlmock.NewResult(1, 1))

	req, err := http.NewRequest("PUT", "/api/profiles/5", body)
	if err != nil {
		t.Fatal(err)
	}
	req.SetPathValue("id", "5")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(updateProfile)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("expected status 200, got %v", status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %s", err)
	}
}

func TestDeleteProfile(t *testing.T) {
	mock, cleanup := setupTestDB(t)
	defer cleanup()

	mock.ExpectExec("DELETE FROM profiles WHERE id=\\$1").
		WithArgs("5").
		WillReturnResult(sqlmock.NewResult(1, 1))

	req, err := http.NewRequest("DELETE", "/api/profiles/5", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.SetPathValue("id", "5")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(deleteProfile)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("expected status 200, got %v", status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %s", err)
	}
}
