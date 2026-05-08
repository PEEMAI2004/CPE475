package processor

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func resetStats() {
	trackerMu.Lock()
	defer trackerMu.Unlock()
	todayStats = make(map[string]*SunStats)
	yesterdayStats = make(map[string]*SunStats)
}

func TestTrackSunlight_Accumulation(t *testing.T) {
	resetStats()
	deviceID := "test-device"

	// 1. First reading - should initialize but not increment (requires 1 min diff)
	TrackSunlight(deviceID, 40000)
	
	// Wait a bit for the async restore to fail (no prom URL set)
	time.Sleep(50 * time.Millisecond)

	summary, _ := GetSunSummary(deviceID)
	if summary.DirectMinutes != 0 {
		t.Errorf("Expected 0 direct mins on first reading, got %d", summary.DirectMinutes)
	}

	// 2. Manipulate last update to simulate 61 seconds passing
	trackerMu.Lock()
	todayStats[deviceID].LastUpdate = time.Now().Add(-61 * time.Second)
	trackerMu.Unlock()

	// 3. Second reading (Direct)
	TrackSunlight(deviceID, 40000)
	summary, _ = GetSunSummary(deviceID)
	if summary.DirectMinutes != 1 {
		t.Errorf("Expected 1 direct min after 61s, got %d", summary.DirectMinutes)
	}

	// 4. Third reading (Indirect)
	trackerMu.Lock()
	todayStats[deviceID].LastUpdate = time.Now().Add(-61 * time.Second)
	trackerMu.Unlock()

	TrackSunlight(deviceID, 10000)
	summary, _ = GetSunSummary(deviceID)
	if summary.IndirectMinutes != 1 {
		t.Errorf("Expected 1 indirect min, got %d", summary.IndirectMinutes)
	}
	if summary.DirectMinutes != 1 {
		t.Errorf("Direct minutes should still be 1, got %d", summary.DirectMinutes)
	}

	// 5. Fourth reading (Dark)
	trackerMu.Lock()
	todayStats[deviceID].LastUpdate = time.Now().Add(-61 * time.Second)
	trackerMu.Unlock()

	TrackSunlight(deviceID, 100)
	summary, _ = GetSunSummary(deviceID)
	if summary.DirectMinutes != 1 || summary.IndirectMinutes != 1 {
		t.Errorf("Counts should not change in dark, got D:%d I:%d", summary.DirectMinutes, summary.IndirectMinutes)
	}
}

func TestSunAlert_TooMuch(t *testing.T) {
	resetStats()
	deviceID := "burn-test"

	// Initialize
	TrackSunlight(deviceID, 40000)
	time.Sleep(50 * time.Millisecond)

	// Set count to limit (default 180)
	trackerMu.Lock()
	todayStats[deviceID].DirectMinutes = 180
	todayStats[deviceID].LastUpdate = time.Now().Add(-61 * time.Second)
	trackerMu.Unlock()

	// Increment one more
	TrackSunlight(deviceID, 40000)

	summary, _ := GetSunSummary(deviceID)
	if summary.Status != StatusSunTooMuch {
		t.Errorf("Expected StatusSunTooMuch, got %s", summary.Status)
	}
}

func TestMidnightRollover(t *testing.T) {
	resetStats()
	deviceID := "rollover-device"

	// 1. Setup today's stats
	trackerMu.Lock()
	todayStats[deviceID] = &SunStats{
		DirectMinutes:   100,
		IndirectMinutes: 200,
		LastUpdate:      time.Now(),
		Status:          StatusHealthy,
	}
	trackerMu.Unlock()

	// 2. Simulate Rollover Logic (extracted from runSunlightJobs)
	trackerMu.Lock()
	// Logic from sun.go:
	for k, v := range todayStats {
		yesterdayStats[k] = &SunStats{
			DirectMinutes:   v.DirectMinutes,
			IndirectMinutes: v.IndirectMinutes,
			LastUpdate:      v.LastUpdate,
			Status:          v.Status,
			Restoring:       false,
		}
		v.DirectMinutes = 0
		v.IndirectMinutes = 0
		v.Status = StatusHealthy
	}
	trackerMu.Unlock()

	// 3. Verify
	summary, yesterday := GetSunSummary(deviceID)
	if summary.DirectMinutes != 0 {
		t.Errorf("Today's minutes should be reset to 0, got %d", summary.DirectMinutes)
	}
	if yesterday != 300 {
		t.Errorf("Yesterday's total should be 300, got %d", yesterday)
	}

	// 4. Ensure TrackSunlight doesn't restore after rollover
	TrackSunlight(deviceID, 40000)
	summary, _ = GetSunSummary(deviceID)
	if summary.Restoring {
		t.Error("Should NOT be restoring after rollover (stats exist in map)")
	}
}

func TestPrometheusRecovery(t *testing.T) {
	resetStats()
	deviceID := "recovery-device"

	// 1. Create mock Prometheus server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		val := "0"
		if strings.Contains(query, "sun_direct") {
			val = "42"
		} else if strings.Contains(query, "sun_indirect") {
			val = "88"
		}

		resp := map[string]interface{}{
			"status": "success",
			"data": map[string]interface{}{
				"resultType": "vector",
				"result": []map[string]interface{}{
					{
						"value": []interface{}{float64(time.Now().Unix()), val},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	SetPrometheusURL(server.URL)
	SetPromLookbackHours(1)

	// 2. Trigger first reading (starts recovery)
	TrackSunlight(deviceID, 5000)

	// 3. Wait for async recovery to complete
	maxWait := 10
	for i := 0; i < maxWait; i++ {
		summary, _ := GetSunSummary(deviceID)
		if !summary.Restoring && summary.DirectMinutes == 42 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// 4. Verify results
	summary, _ := GetSunSummary(deviceID)
	if summary.DirectMinutes != 42 {
		t.Errorf("Expected restored direct mins 42, got %d", summary.DirectMinutes)
	}
	if summary.IndirectMinutes != 88 {
		t.Errorf("Expected restored indirect mins 88, got %d", summary.IndirectMinutes)
	}
	if summary.Restoring {
		t.Error("Restoring flag should be false after completion")
	}
}

func TestEvaluate_IgnoreLowerLight(t *testing.T) {
	// This tests the logic in thresholds.go as summarized in "Ignoring Instantaneous Lower Bounds"
	deviceID := "light-logic-test"
	
	// Thresholds from thresholds.go init(): 2000, 50000, 500, 80000
	
	// 1. Very Dark - should be healthy (ignored lower bounds)
	status := Evaluate(deviceID, "light", 100)
	if status != StatusHealthy {
		t.Errorf("Expected healthy for 100 lux, got %s", status)
	}

	// 2. High Light (Warning)
	status = Evaluate(deviceID, "light", 60000)
	if status != StatusWarningHigh {
		t.Errorf("Expected warning_high for 60k lux, got %s", status)
	}

	// 3. Extreme Light (Critical)
	status = Evaluate(deviceID, "light", 90000)
	if status != StatusCriticalHigh {
		t.Errorf("Expected critical_high for 90k lux, got %s", status)
	}
}
