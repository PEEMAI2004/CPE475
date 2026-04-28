package api

import (
	"database/sql"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type PromResponse struct {
	Data struct {
		Result []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

func (s *Server) fetchPrometheus(query string) map[string]string {
	result := make(map[string]string)
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://prometheus.iot.kaminjitt.com:9090/api/v1/query?query=" + query)
	if err != nil {
		return result
	}
	defer resp.Body.Close()

	var pResp PromResponse
	if err := json.NewDecoder(resp.Body).Decode(&pResp); err != nil {
		return result
	}

	for _, r := range pResp.Data.Result {
		dev := r.Metric["device"]
		if dev == "" {
			continue
		}
		if len(r.Value) == 2 {
			if valStr, ok := r.Value[1].(string); ok {
				// Use lowercase for case-insensitive matching in other handlers
				result[strings.ToLower(dev)] = valStr
			}
		}
	}
	return result
}

type ServiceHealth struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Status  string `json:"status"`
	Address string `json:"address"`
}

func checkTCP(address string) string {
	if address == "" {
		return "offline"
	}
	if !strings.Contains(address, ":") {
		address = address + ":1883"
	}
	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		return "offline"
	}
	conn.Close()
	return "online"
}

func checkHTTP(url string) string {
	if url == "" {
		return "offline"
	}
	if !strings.HasPrefix(url, "http") {
		url = "http://" + url + ":8080/health"
	}
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil || resp.StatusCode >= 500 {
		return "offline"
	}
	return "online"
}

func (s *Server) getInfrastructureHealth(w http.ResponseWriter, r *http.Request) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := []ServiceHealth{}

	coreTargets := []struct {
		name      string
		typ       string
		checkType string
		addr      string
	}{
		{"Manager API", "backend", "always", "localhost:8081"},
		{"Manager UI", "frontend", "always", "localhost:8081"},
		{"Database", "postgres", "db", "postgresql.iot.kaminjitt.com:5432"},
		{"Prometheus Scraper", "prometheus", "http-raw", "http://prometheus.iot.kaminjitt.com:9090/-/healthy"},
		{"Grafana Dashboard", "grafana", "http-raw", "http://grafana.iot.kaminjitt.com:3000/api/health"},
	}

	for _, t := range coreTargets {
		wg.Add(1)
		go func(t struct{ name, typ, checkType, addr string }) {
			defer wg.Done()
			status := "offline"
			switch t.checkType {
			case "always":
				status = "online"
			case "db":
				if err := s.DB.Ping(); err == nil {
					status = "online"
				}
			case "http-raw":
				client := http.Client{Timeout: 2 * time.Second}
				resp, err := client.Get(t.addr)
				if err == nil && resp.StatusCode < 500 {
					status = "online"
				}
			}
			mu.Lock()
			results = append(results, ServiceHealth{Name: t.name, Type: t.typ, Status: status, Address: t.addr})
			mu.Unlock()
		}(t)
	}

	rows, err := s.DB.Query("SELECT name, site_id, address, mqtt_address FROM infrastructure_nodes")
	if err == nil {
		for rows.Next() {
			var n struct {
				name     string
				mqttAddr sql.NullString
				addr     string
				site_id  int
			}
			if err := rows.Scan(&n.name, &n.site_id, &n.addr, &n.mqttAddr); err != nil {
				continue
			}

			wg.Add(1)
			go func(name, addr string) {
				defer wg.Done()
				status := checkHTTP(addr)
				mu.Lock()
				results = append(results, ServiceHealth{Name: name + " (Node)", Type: "Local Node", Status: status, Address: addr})
				mu.Unlock()
			}(n.name, n.addr)

			wg.Add(1)
			go func(name, addr, mqttAddr string, site_id int) {
				defer wg.Done()
				finalMQTTAddr := mqttAddr
				if finalMQTTAddr == "" {
					finalMQTTAddr = addr
				}
				status := checkTCP(finalMQTTAddr + ":1883")
				mu.Lock()
				results = append(results, ServiceHealth{
					Name:    name + " (MQTT)",
					Type:    "mqtt",
					Status:  status,
					Address: finalMQTTAddr + ":1883",
				})
				mu.Unlock()
			}(n.name, n.addr, n.mqttAddr.String, n.site_id)
		}
		rows.Close()
	}

	wg.Wait()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}
