package processor

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// SunStats holds the exposure minutes for a device.
type SunStats struct {
	DirectMinutes   int       `json:"direct_minutes"`
	IndirectMinutes int       `json:"indirect_minutes"`
	LastUpdate      time.Time `json:"-"`
	Status          string    `json:"status"`
	Restoring       bool      `json:"-"`
}

var (
	trackerMu      sync.RWMutex
	todayStats     = make(map[string]*SunStats)
	yesterdayStats = make(map[string]*SunStats)
	promURL        string
	tzOffset       int = 7 // Default to UTC+7
)

func init() {
	// Start the daily rollover and sunset check background tasks
	go runSunlightJobs()
}

// SetPrometheusURL sets the URL used for historical recovery.
func SetPrometheusURL(url string) {
	promURL = url
}

// SetTimezoneOffset sets the timezone offset in hours.
func SetTimezoneOffset(offset int) {
	trackerMu.Lock()
	tzOffset = offset
	trackerMu.Unlock()
}

// TrackSunlight increments exposure counters based on the current light reading.
func TrackSunlight(deviceID string, lux float64) {
	// 1. Check if we need to restore this device from Prometheus
	trackerMu.Lock()
	stats, ok := todayStats[deviceID]
	if !ok {
		// New device seen by this process instance
		stats = &SunStats{LastUpdate: time.Now(), Restoring: true}
		todayStats[deviceID] = stats
		trackerMu.Unlock()

		// Attempt restoration in background
		go restoreFromPrometheus(deviceID)
		return
	}

	// If still restoring, skip this pulse to avoid race conditions
	if stats.Restoring {
		trackerMu.Unlock()
		return
	}
	trackerMu.Unlock()

	// 2. Standard Accumulation Logic
	trackerMu.Lock()
	defer trackerMu.Unlock()

	thresholds := GetSunThresholds(deviceID)
	now := time.Now()

	diff := now.Sub(stats.LastUpdate)

	// We increment if at least 1 minute has passed since last update for this device
	if diff >= 1*time.Minute {
		if lux >= thresholds.DirectThreshold {
			stats.DirectMinutes++
		} else if lux >= 500 { // Assume > 500 lux is 'Indirect'
			stats.IndirectMinutes++
		}
		stats.LastUpdate = now
	}

	// Immediate "Too Much" check
	if stats.DirectMinutes > thresholds.MaxDirectMinutes {
		stats.Status = StatusSunTooMuch
	} else if stats.Status == StatusSunTooMuch {
		stats.Status = StatusHealthy
	}
}

// restoreFromPrometheus fetches the latest metric values to resume counting.
func restoreFromPrometheus(deviceID string) {
	if promURL == "" {
		finalizeRestoration(deviceID, 0, 0)
		return
	}

	// Fetch Direct minutes
	direct := queryPromSingle(deviceID, "potbuddy_sun_direct_minutes")
	// Fetch Indirect minutes
	indirect := queryPromSingle(deviceID, "potbuddy_sun_indirect_minutes")

	log.Printf("[sun] restored %s from Prometheus: direct=%d, indirect=%d", deviceID, direct, indirect)
	finalizeRestoration(deviceID, direct, indirect)
}

func finalizeRestoration(deviceID string, direct, indirect int) {
	trackerMu.Lock()
	defer trackerMu.Unlock()
	if s, ok := todayStats[deviceID]; ok {
		s.DirectMinutes = direct
		s.IndirectMinutes = indirect
		s.Restoring = false
	}
}

func queryPromSingle(deviceID string, metric string) int {
	// Simple instant query for the latest value
	q := fmt.Sprintf("%s{device=\"%s\"}", metric, strings.ToLower(deviceID))
	apiURL := fmt.Sprintf("%s/api/v1/query?query=%s", promURL, url.QueryEscape(q))

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			Result []struct {
				Value []interface{} `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0
	}

	if len(result.Data.Result) > 0 && len(result.Data.Result[0].Value) > 1 {
		valStr, ok := result.Data.Result[0].Value[1].(string)
		if ok {
			val, _ := strconv.ParseFloat(valStr, 64)
			return int(val)
		}
	}
	return 0
}

// GetSunSummary returns the current sun status and yesterday's total.
func GetSunSummary(deviceID string) (current SunStats, yesterday int) {
	trackerMu.RLock()
	defer trackerMu.RUnlock()

	if s, ok := todayStats[deviceID]; ok {
		current = *s
	}
	if s, ok := yesterdayStats[deviceID]; ok {
		yesterday = s.DirectMinutes + s.IndirectMinutes
	}
	return
}

func runSunlightJobs() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		trackerMu.RLock()
		offset := tzOffset
		trackerMu.RUnlock()

		// Use Configured FixedZone to avoid relying on OS tzdata (often missing in Alpine)
		loc := time.FixedZone("Configured", offset*3600)
		now := time.Now().In(loc)

		trackerMu.Lock()
		if now.Hour() == 0 && now.Minute() < 10 {
			for k, v := range todayStats {
				yesterdayStats[k] = v
			}
			todayStats = make(map[string]*SunStats)
		}

		if now.Hour() >= 18 {
			for dID, stats := range todayStats {
				thresh := GetSunThresholds(dID)
				total := stats.DirectMinutes + stats.IndirectMinutes
				if total < thresh.MinTotalMinutes {
					stats.Status = StatusSunTooLittle
				}
			}
		}
		trackerMu.Unlock()
	}
}

// humanSunMessage returns advice based on sun status.
func humanSunMessage(status string, stats SunStats, yesterday int) string {
	switch status {
	case StatusSunTooMuch:
		return fmt.Sprintf("Too much sun! Move to shade. (%d mins direct)", stats.DirectMinutes)
	case StatusSunTooLittle:
		return "Insufficient sun today. Move to a brighter spot tomorrow."
	default:
		if yesterday > 0 && yesterday < 360 {
			return "Plant had low light yesterday. Ensure it gets more sun today."
		}
		return "Sunlight exposure is optimal."
	}
}
