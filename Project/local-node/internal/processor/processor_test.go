package processor

import (
	"encoding/json"
	"testing"
)

func ptr(v float64) *float64 { return &v }

func TestEnrich_AllHealthy(t *testing.T) {
	raw := RawReading{
		Light: ptr(5000),
		Temp:  ptr(25),
		Hum:   ptr(55),
		Soil:  ptr(2000),
	}
	p := Enrich(raw, "test-device")

	if p.Status.Overall != StatusHealthy {
		t.Errorf("expected healthy, got %s", p.Status.Overall)
	}
	if p.Status.Light != StatusHealthy {
		t.Errorf("light: expected healthy, got %s", p.Status.Light)
	}
	if p.Status.Temp != StatusHealthy {
		t.Errorf("temp: expected healthy, got %s", p.Status.Temp)
	}
	if p.Status.Hum != StatusHealthy {
		t.Errorf("hum: expected healthy, got %s", p.Status.Hum)
	}
	if p.Status.Soil != StatusHealthy {
		t.Errorf("soil: expected healthy, got %s", p.Status.Soil)
	}
}

func TestEnrich_DrySoil_CriticalHigh(t *testing.T) {
	raw := RawReading{
		Light: ptr(5000),
		Temp:  ptr(25),
		Hum:   ptr(55),
		Soil:  ptr(3500), // beyond outer high → critical_high
	}
	p := Enrich(raw, "test-device")

	if p.Status.Soil != StatusCriticalHigh {
		t.Errorf("soil: expected critical_high, got %s", p.Status.Soil)
	}
	if p.Status.Overall != StatusCriticalHigh {
		t.Errorf("overall: expected critical_high, got %s", p.Status.Overall)
	}
}

func TestEnrich_HotDay_WarningHigh(t *testing.T) {
	raw := RawReading{
		Light: ptr(5000),
		Temp:  ptr(32), // inside outer but outside inner high → warning_high
		Hum:   ptr(55),
		Soil:  ptr(2000),
	}
	p := Enrich(raw, "test-device")

	if p.Status.Temp != StatusWarningHigh {
		t.Errorf("temp: expected warning_high, got %s", p.Status.Temp)
	}
	if p.Status.Overall != StatusWarningHigh {
		t.Errorf("overall: expected warning_high, got %s", p.Status.Overall)
	}
}

func TestEnrich_Dark_CriticalLow(t *testing.T) {
	raw := RawReading{
		Light: ptr(100), // below outer low → critical_low
		Temp:  ptr(25),
		Hum:   ptr(55),
		Soil:  ptr(2000),
	}
	p := Enrich(raw, "test-device")

	if p.Status.Light != StatusCriticalLow {
		t.Errorf("light: expected critical_low, got %s", p.Status.Light)
	}
	if p.Status.Overall != StatusCriticalLow {
		t.Errorf("overall: expected critical_low, got %s", p.Status.Overall)
	}
}

func TestEnrich_DHTError_MissingFields(t *testing.T) {
	// Firmware publishes only light + soil when DHT11 read fails.
	raw := RawReading{
		Light: ptr(3000),
		Soil:  ptr(2000),
		// Temp and Hum are nil
	}
	p := Enrich(raw, "test-device")

	// Missing fields should default to healthy (not penalise the plant).
	if p.Status.Temp != StatusHealthy {
		t.Errorf("temp (nil): expected healthy default, got %s", p.Status.Temp)
	}
	if p.Status.Hum != StatusHealthy {
		t.Errorf("hum (nil): expected healthy default, got %s", p.Status.Hum)
	}
	if p.Status.Overall != StatusHealthy {
		t.Errorf("overall: expected healthy, got %s", p.Status.Overall)
	}
}

func TestParse_ValidJSON(t *testing.T) {
	payload := []byte(`{"light":3200,"temp":27,"hum":55,"soil":1900}`)
	r, err := Parse(payload)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if r.Light == nil || *r.Light != 3200 {
		t.Errorf("light: expected 3200, got %v", r.Light)
	}
}

func TestParse_InvalidJSON(t *testing.T) {
	_, err := Parse([]byte(`not json`))
	if err == nil {
		t.Error("expected parse error for invalid JSON")
	}
}

func TestEnrichedPayload_JSONRoundTrip(t *testing.T) {
	raw := RawReading{Light: ptr(5000), Temp: ptr(25), Hum: ptr(55), Soil: ptr(2000)}
	p := Enrich(raw, "test-device")

	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var p2 EnrichedPayload
	if err := json.Unmarshal(b, &p2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p2.Status.Overall != p.Status.Overall {
		t.Errorf("round-trip status mismatch: %s != %s", p2.Status.Overall, p.Status.Overall)
	}
}
