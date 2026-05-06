package metrics

import (
	"strings"
	"sync"
	"time"

	"github.com/potbuddy/local-node/internal/processor"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// deviceTimeout is how long without a reading before a device is marked offline.
const deviceTimeout = 30 * time.Second

// All metrics use a "device" label so Grafana can filter/group per ESP32.
var (
	lightLux = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "potbuddy_light_lux",
		Help: "Ambient light level in lux (BH1750)",
	}, []string{"device"})

	tempCelsius = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "potbuddy_temperature_celsius",
		Help: "Air temperature in Celsius (DHT11)",
	}, []string{"device"})

	humPercent = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "potbuddy_humidity_percent",
		Help: "Air humidity in percent (DHT11)",
	}, []string{"device"})

	soilRaw = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "potbuddy_soil_raw",
		Help: "Raw soil moisture ADC value (0-4095, higher = drier)",
	}, []string{"device"})

	healthStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "potbuddy_health_status",
		Help: "Health status. Overall: 0=H, 1=W, 2=C. Sensors: 0=H, 1=WL, 2=WH, 3=CL, 4=CH",
	}, []string{"device", "field"})

	readingsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "potbuddy_readings_total",
		Help: "Total number of sensor readings processed",
	}, []string{"device"})

	// deviceOnline is 1 while a device is actively sending data, 0 when silent.
	deviceOnline = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "potbuddy_device_online",
		Help: "1 if device sent a reading within the last 30 s, 0 otherwise",
	}, []string{"device"})

	// Sunlight Exposure Metrics
	sunDirectMinutes = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "potbuddy_sun_direct_minutes",
		Help: "Minutes of direct sun exposure today",
	}, []string{"device"})

	sunIndirectMinutes = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "potbuddy_sun_indirect_minutes",
		Help: "Minutes of indirect sun exposure today",
	}, []string{"device"})

	sunYesterdayTotal = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "potbuddy_sun_yesterday_total_minutes",
		Help: "Total sun exposure (direct + indirect) from yesterday",
	}, []string{"device"})

	sunStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "potbuddy_sun_status",
		Help: "Sun status (0=Healthy, 1=Too Much, 2=Too Little)",
	}, []string{"device"})
)

// deviceHeartbeats tracks the last-seen time per device for the online watchdog.
var (
	heartbeatMu sync.Mutex
	heartbeats  = map[string]time.Time{}
)

// sensorStatusValue maps directional strings to a 0-4 scale for specific alerts.
func sensorStatusValue(s string) float64 {
	switch s {
	case "warning_low":
		return 1
	case "warning_high":
		return 2
	case "critical_low":
		return 3
	case "critical_high":
		return 4
	case "warning":
		return 1
	case "critical":
		return 3
	default:
		return 0
	}
}

// overallStatusValue maps statuses to the traditional 0-2 scale for the main dashboard.
func overallStatusValue(s string) float64 {
	switch {
	case strings.HasPrefix(s, "warning"):
		return 1
	case strings.HasPrefix(s, "critical"):
		return 2
	default:
		return 0
	}
}

func sunStatusValue(s string) float64 {
	switch s {
	case processor.StatusSunTooMuch:
		return 1
	case processor.StatusSunTooLittle:
		return 2
	default:
		return 0
	}
}

// StartOnlineWatchdog launches a background goroutine that sets potbuddy_device_online
// to 0 for any device that has not sent a reading within deviceTimeout.
// Call this once from main() after starting the service.
func StartOnlineWatchdog() {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now()
			heartbeatMu.Lock()
			for dev, last := range heartbeats {
				if now.Sub(last) > deviceTimeout {
					deviceOnline.WithLabelValues(dev).Set(0)
				}
			}
			heartbeatMu.Unlock()
		}
	}()
}

// Update records an enriched payload as Prometheus metric values for the given device.
func Update(p processor.EnrichedPayload) {
	// Normalize device ID to lowercase to ensure consistency and prevent case-sensitive duplicates
	dev := strings.ToLower(p.DeviceID)

	// Mark device as online and refresh heartbeat.
	deviceOnline.WithLabelValues(dev).Set(1)
	heartbeatMu.Lock()
	heartbeats[dev] = time.Now()
	heartbeatMu.Unlock()

	if p.Raw.Light != nil {
		lightLux.WithLabelValues(dev).Set(*p.Raw.Light)
	}
	if p.Raw.Temp != nil {
		tempCelsius.WithLabelValues(dev).Set(*p.Raw.Temp)
	}
	if p.Raw.Hum != nil {
		humPercent.WithLabelValues(dev).Set(*p.Raw.Hum)
	}
	if p.Raw.Soil != nil {
		soilRaw.WithLabelValues(dev).Set(*p.Raw.Soil)
	}

	// Overall uses 0, 1, 2
	healthStatus.WithLabelValues(dev, "overall").Set(overallStatusValue(p.Status.Overall))

	// Individual sensors use 0, 1, 2, 3, 4
	healthStatus.WithLabelValues(dev, "light").Set(sensorStatusValue(p.Status.Light))
	healthStatus.WithLabelValues(dev, "temp").Set(sensorStatusValue(p.Status.Temp))
	healthStatus.WithLabelValues(dev, "hum").Set(sensorStatusValue(p.Status.Hum))
	healthStatus.WithLabelValues(dev, "soil").Set(sensorStatusValue(p.Status.Soil))

	// Sunlight metrics
	sunDirectMinutes.WithLabelValues(dev).Set(float64(p.Sun.TodayDirect))
	sunIndirectMinutes.WithLabelValues(dev).Set(float64(p.Sun.TodayIndirect))
	sunYesterdayTotal.WithLabelValues(dev).Set(float64(p.Sun.YesterdayTotal))
	sunStatus.WithLabelValues(dev).Set(sunStatusValue(p.Sun.Status))

	readingsTotal.WithLabelValues(dev).Inc()
}
