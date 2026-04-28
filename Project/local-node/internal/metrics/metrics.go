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
		Help: "Health status per field (0=healthy, 1=warning, 2=critical)",
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
)

// deviceHeartbeats tracks the last-seen time per device for the online watchdog.
var (
	heartbeatMu sync.Mutex
	heartbeats  = map[string]time.Time{}
)

func statusValue(s string) float64 {
	switch s {
	case "warning":
		return 1
	case "critical":
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
	// Use lowercase for all device IDs in metrics to ensure consistency with Grafana queries
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

	healthStatus.WithLabelValues(dev, "overall").Set(statusValue(p.Status.Overall))
	healthStatus.WithLabelValues(dev, "light").Set(statusValue(p.Status.Light))
	healthStatus.WithLabelValues(dev, "temp").Set(statusValue(p.Status.Temp))
	healthStatus.WithLabelValues(dev, "hum").Set(statusValue(p.Status.Hum))
	healthStatus.WithLabelValues(dev, "soil").Set(statusValue(p.Status.Soil))
	readingsTotal.WithLabelValues(dev).Inc()
}
