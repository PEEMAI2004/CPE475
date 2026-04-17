package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
)

const (
	clientIDPrefix = "potbuddy-simulator"
	interval       = 3 * time.Second
)

// simDevice represents a simulated ESP32 device with its own client and scenario list.
type simDevice struct {
	id     string
	client pahomqtt.Client
	cases  []testCase
	index  int
}

// testCase defines a simulation scenario.
type testCase struct {
	name    string
	payload map[string]any
}

// devices lists the fake ESP32s to simulate.
var deviceProfiles = []struct {
	id    string
	cases []testCase
}{
	{
		id: "plant-living-room",
		cases: []testCase{
			{name: "Normal", payload: map[string]any{"light": 4500, "temp": 26, "hum": 58, "soil": 1900}},
			{name: "Dry soil", payload: map[string]any{"light": 4500, "temp": 26, "hum": 58, "soil": 3600}},
			{name: "Normal", payload: map[string]any{"light": 4600, "temp": 25, "hum": 60, "soil": 2000}},
		},
	},
	{
		id: "plant-balcony",
		cases: []testCase{
			{name: "Bright + hot", payload: map[string]any{"light": 55000, "temp": 34, "hum": 42, "soil": 2200}},
			{name: "Normal", payload: map[string]any{"light": 30000, "temp": 29, "hum": 48, "soil": 2100}},
			{name: "Dark", payload: map[string]any{"light": 300, "temp": 25, "hum": 55, "soil": 2000}},
		},
	},
	{
		id: "plant-bedroom",
		cases: []testCase{
			{name: "Normal", payload: map[string]any{"light": 800, "temp": 24, "hum": 62, "soil": 1800}},
			{name: "Very dark", payload: map[string]any{"light": 50, "temp": 23, "hum": 65, "soil": 1700}},
			{name: "DHT error", payload: map[string]any{"light": 900, "soil": 1900}},
		},
	},
}

func connectDevice(broker, deviceID string) pahomqtt.Client {
	opts := pahomqtt.NewClientOptions().
		AddBroker(broker).
		SetClientID(clientIDPrefix + "-" + deviceID).
		SetCleanSession(true).
		SetAutoReconnect(true)

	client := pahomqtt.NewClient(opts)
	if tok := client.Connect(); tok.Wait() && tok.Error() != nil {
		log.Fatalf("simulate: connect %s: %v", deviceID, tok.Error())
	}
	return client
}

func main() {
	broker := os.Getenv("MQTT_BROKER")
	if broker == "" {
		broker = "tcp://localhost:1883"
	}

	topic := os.Getenv("MQTT_TOPIC_PREFIX")
	if topic == "" {
		topic = "potbuddy"
	}

	log.Printf("Simulator starting — broker: %s", broker)

	devices := make([]*simDevice, 0, len(deviceProfiles))
	for _, p := range deviceProfiles {
		d := &simDevice{
			id:    p.id,
			cases: p.cases,
		}
		d.client = connectDevice(broker, p.id)
		devices = append(devices, d)
		log.Printf("  [%s] connected", p.id)
	}

	fmt.Println()
	log.Println("Publishing sensor data — Press Ctrl+C to stop.")
	fmt.Println()

	tick := time.NewTicker(interval)
	defer tick.Stop()

	i := 0
	for range tick.C {
		for _, d := range devices {
			tc := d.cases[i%len(d.cases)]
			pubTopic := fmt.Sprintf("%s/%s/raw", topic, d.id)

			b, err := json.Marshal(tc.payload)
			if err != nil {
				log.Printf("simulate: marshal %s: %v", d.id, err)
				continue
			}

			tok := d.client.Publish(pubTopic, 1, false, b)
			tok.Wait()
			if tok.Error() != nil {
				log.Printf("[%s] publish error: %v", d.id, tok.Error())
			} else {
				log.Printf("[%s] %-20s → %s", d.id, tc.name, string(b))
			}
		}
		fmt.Println()
		i++
	}
}
