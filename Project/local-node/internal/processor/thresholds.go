package processor

import (
	"sync"
)

// Status constants represent plant health levels.
const (
	StatusHealthy      = "healthy"
	StatusWarningLow   = "warning_low"
	StatusWarningHigh  = "warning_high"
	StatusCriticalLow  = "critical_low"
	StatusCriticalHigh = "critical_high"

	// Legacy/Base constants for rollup logic
	StatusWarning      = "warning"
	StatusCritical     = "critical"

	// Sunlight specific statuses
	StatusSunTooMuch   = "too_much_sun"
	StatusSunTooLittle = "insufficient_sun"
)

// Bound defines a two-boundary range check.
type Bound struct {
	InnerLow  float64
	InnerHigh float64
	OuterLow  float64
	OuterHigh float64
}

// SunThresholds defines the requirements for sunlight exposure.
type SunThresholds struct {
	DirectThreshold    float64
	MaxDirectMinutes   int
	MinTotalMinutes    int
}

// ThresholdProfile maps sensor fields to a Bound and includes sun requirements.
type ThresholdProfile struct {
	Bounds map[string]Bound
	Sun    SunThresholds
}

// evaluate returns a descriptive status for a single numeric sensor value.
func (b Bound) evaluate(v float64) string {
	if v < b.OuterLow {
		return StatusCriticalLow
	}
	if v > b.OuterHigh {
		return StatusCriticalHigh
	}
	if v < b.InnerLow {
		return StatusWarningLow
	}
	if v > b.InnerHigh {
		return StatusWarningHigh
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
	defaultProfile = ThresholdProfile{
		Bounds: map[string]Bound{
			"soil":  {InnerLow: 1500, InnerHigh: 2500, OuterLow: 1000, OuterHigh: 3000},
			"temp":  {InnerLow: 18, InnerHigh: 30, OuterLow: 15, OuterHigh: 35},
			"hum":   {InnerLow: 40, InnerHigh: 70, OuterLow: 30, OuterHigh: 80},
			"light": {InnerLow: 2000, InnerHigh: 50000, OuterLow: 500, OuterHigh: 80000},
		},
		Sun: SunThresholds{
			DirectThreshold:  30000,
			MaxDirectMinutes: 180,
			MinTotalMinutes:  360,
		},
	}
}

// UpdateThresholds is called by the DB poller to replace the active thresholds map
func UpdateThresholds(deviceMap map[string]ThresholdProfile, defProfile ThresholdProfile) {
	mu.Lock()
	defer mu.Unlock()
	devices = deviceMap
	defaultProfile = defProfile
}

// GetSunThresholds returns the sunlight requirements for a specific device.
func GetSunThresholds(deviceID string) SunThresholds {
	mu.RLock()
	defer mu.RUnlock()
	if p, ok := devices[deviceID]; ok {
		return p.Sun
	}
	return defaultProfile.Sun
}

// Evaluate returns the health status string for a given sensor field and value.
func Evaluate(deviceID string, field string, value float64) string {
	mu.RLock()
	defer mu.RUnlock()

	// 1. Try to find device-specific profile
	p, ok := devices[deviceID]
	if !ok {
		// 2. Fall back to default
		p = defaultProfile
	}

	// 3. Find the field threshold in the active profile
	b, ok := p.Bounds[field]
	if !ok {
		return StatusHealthy
	}

	// Instantaneous low light is natural (night time) and handled via Daily Sun Tracker.
	// Ignore lower bounds to prevent false "Too Dark" critical alerts.
	if field == "light" {
		if value > b.OuterHigh {
			return StatusCriticalHigh
		}
		if value > b.InnerHigh {
			return StatusWarningHigh
		}
		return StatusHealthy
	}

	return b.evaluate(value)
}

// worstStatus returns the more severe of two status strings.
func worstStatus(a, b string) string {
	// Map descriptive statuses to their base severity for ranking
	rank := func(s string) int {
		switch {
		case s == StatusCriticalLow || s == StatusCriticalHigh || s == StatusCritical:
			return 2
		case s == StatusWarningLow || s == StatusWarningHigh || s == StatusWarning:
			return 1
		case s == StatusSunTooMuch || s == StatusSunTooLittle:
			return 1 // Sunlight issues treated as warnings for rollup
		default:
			return 0
		}
	}

	if rank(a) >= rank(b) {
		return a
	}
	// Return the actual descriptive status b if it's worse
	return b
}

