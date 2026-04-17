package processor

import (
	"encoding/json"
	"fmt"
	"time"
)

// RawReading represents the JSON payload published by the ESP32.
// Fields are pointers so we can detect absent values (e.g. DHT error path).
type RawReading struct {
	Light *float64 `json:"light"`
	Temp  *float64 `json:"temp"`
	Hum   *float64 `json:"hum"`
	Soil  *float64 `json:"soil"`
}

// FieldStatus holds the per-sensor health classification.
type FieldStatus struct {
	Overall string `json:"overall"`
	Light   string `json:"light"`
	Temp    string `json:"temp"`
	Hum     string `json:"hum"`
	Soil    string `json:"soil"`
}

// EnrichedPayload is the full processed message published to the cloud and
// stored in the ring buffer.
type EnrichedPayload struct {
	DeviceID  string      `json:"device_id"`
	Timestamp time.Time   `json:"timestamp"`
	Raw       RawReading  `json:"raw"`
	Status    FieldStatus `json:"status"`
	Message   string      `json:"message"`
}

// humanMessage returns a friendly description for the overall status.
func humanMessage(overall string) string {
	switch overall {
	case StatusHealthy:
		return "Plant is healthy today 🌱"
	case StatusWarning:
		return "Some conditions need attention ⚠️"
	default:
		return "Plant needs immediate care 🚨"
	}
}

// Parse unmarshals raw MQTT bytes into a RawReading.
func Parse(raw []byte) (RawReading, error) {
	var r RawReading
	if err := json.Unmarshal(raw, &r); err != nil {
		return RawReading{}, fmt.Errorf("processor: parse: %w", err)
	}
	return r, nil
}

// Enrich evaluates each sensor field, rolls up an overall status, and returns
// a fully populated EnrichedPayload ready for publishing and storage.
func Enrich(r RawReading, deviceID string) EnrichedPayload {
	overall := StatusHealthy

	// Evaluate light.
	lightStatus := StatusHealthy
	if r.Light != nil {
		lightStatus = Evaluate(deviceID, "light", *r.Light)
		overall = worstStatus(overall, lightStatus)
	}

	// Evaluate temperature.
	tempStatus := StatusHealthy
	if r.Temp != nil {
		tempStatus = Evaluate(deviceID, "temp", *r.Temp)
		overall = worstStatus(overall, tempStatus)
	}

	// Evaluate humidity.
	humStatus := StatusHealthy
	if r.Hum != nil {
		humStatus = Evaluate(deviceID, "hum", *r.Hum)
		overall = worstStatus(overall, humStatus)
	}

	// Evaluate soil moisture.
	soilStatus := StatusHealthy
	if r.Soil != nil {
		soilStatus = Evaluate(deviceID, "soil", *r.Soil)
		overall = worstStatus(overall, soilStatus)
	}

	return EnrichedPayload{
		DeviceID:  deviceID,
		Timestamp: time.Now().UTC(),
		Raw:       r,
		Status: FieldStatus{
			Overall: overall,
			Light:   lightStatus,
			Temp:    tempStatus,
			Hum:     humStatus,
			Soil:    soilStatus,
		},
		Message: humanMessage(overall),
	}
}
