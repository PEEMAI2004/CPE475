package processor

import (
	"sync"
)

// Status constants represent plant health levels.
const (
	StatusHealthy  = "healthy"
	StatusWarning  = "warning"
	StatusCritical = "critical"
)

// Bound defines a two-boundary range check.
type Bound struct {
	InnerLow  float64
	InnerHigh float64
	OuterLow  float64
	OuterHigh float64
}

// ThresholdProfile maps sensor fields to a Bound (e.g. "soil" -> Bound{...}).
type ThresholdProfile map[string]Bound

// evaluate returns the status for a single numeric sensor value.
func (b Bound) evaluate(v float64) string {
	if v < b.OuterLow || v > b.OuterHigh {
		return StatusCritical
	}
	if v < b.InnerLow || v > b.InnerHigh {
		return StatusWarning
	}
	return StatusHealthy
}

// Profile caching structures
var (
	mu             sync.RWMutex
	devices        map[string]ThresholdProfile // device_id -> profile
	defaultProfile ThresholdProfile            // fallback profile
)

func init() {
	devices = make(map[string]ThresholdProfile)
	
	// Fallback hardcoded defaults if DB isn't loaded yet
	defaultProfile = map[string]Bound{
		"soil":  {InnerLow: 1500, InnerHigh: 2500, OuterLow: 1000, OuterHigh: 3000},
		"temp":  {InnerLow: 18, InnerHigh: 30, OuterLow: 15, OuterHigh: 35},
		"hum":   {InnerLow: 40, InnerHigh: 70, OuterLow: 30, OuterHigh: 80},
		"light": {InnerLow: 2000, InnerHigh: 50000, OuterLow: 500, OuterHigh: 80000},
	}
}

// UpdateThresholds is called by the DB poller to replace the active thresholds map
func UpdateThresholds(deviceMap map[string]ThresholdProfile, defProfile ThresholdProfile) {
	mu.Lock()
	defer mu.Unlock()
	devices = deviceMap
	defaultProfile = defProfile
}

// Evaluate returns the health status string for a given sensor field and value.
func Evaluate(deviceID string, field string, value float64) string {
	mu.RLock()
	defer mu.RUnlock()

	// 1. Try to find device-specific profile
	prof, ok := devices[deviceID]
	if !ok {
		// 2. Fall back to default
		prof = defaultProfile
	}

	// 3. Find the field threshold in the active profile
	t, ok := prof[field]
	if !ok {
		return StatusHealthy
	}
	return t.evaluate(value)
}

// worstStatus returns the more severe of two status strings.
func worstStatus(a, b string) string {
	rank := map[string]int{StatusHealthy: 0, StatusWarning: 1, StatusCritical: 2}
	if rank[a] >= rank[b] {
		return a
	}
	return b
}
