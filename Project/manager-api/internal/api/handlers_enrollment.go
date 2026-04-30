package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/potbuddy/manager-api/internal/models"
)

func (s *Server) getEnrolledNodes(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.Query("SELECT id, name, type, site_id, address, mqtt_address, token, created_at FROM infrastructure_nodes ORDER BY id")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var nodes []models.InfrastructureNode
	for rows.Next() {
		var n models.InfrastructureNode
		var addr, mqttAddr sql.NullString
		if err := rows.Scan(&n.ID, &n.Name, &n.Type, &n.SiteID, &addr, &mqttAddr, &n.Token, &n.CreatedAt); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		n.Address = addr.String
		n.MQTTAddress = mqttAddr.String
		nodes = append(nodes, n)
	}

	w.Header().Set("Content-Type", "application/json")
	if nodes == nil {
		nodes = []models.InfrastructureNode{}
	}
	json.NewEncoder(w).Encode(nodes)
}

func (s *Server) enrollNode(w http.ResponseWriter, r *http.Request) {
	var n models.InfrastructureNode
	if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	if n.Name == "" || n.Address == "" {
		http.Error(w, "name and address are required", 400)
		return
	}

	n.Token = "pb_node_" + s.generateToken(16)
	if n.Type == "" {
		n.Type = "Local Node"
	}

	err := s.DB.QueryRow("INSERT INTO infrastructure_nodes (name, type, site_id, address, mqtt_address, token) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at", 
		n.Name, n.Type, n.SiteID, n.Address, n.MQTTAddress, n.Token).Scan(&n.ID, &n.CreatedAt)
	
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(n)
}

func (s *Server) updateEnrolledNode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var n models.InfrastructureNode
	if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	_, err := s.DB.Exec("UPDATE infrastructure_nodes SET name=$1, site_id=$2, address=$3, mqtt_address=$4 WHERE id=$5", 
		n.Name, n.SiteID, n.Address, n.MQTTAddress, id)
	
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(200)
}

func (s *Server) deleteEnrolledNode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_, err := s.DB.Exec("DELETE FROM infrastructure_nodes WHERE id=$1", id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(200)
}

func (s *Server) generateServerCert(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var n models.InfrastructureNode
	var addr, mqttAddr sql.NullString

	err := s.DB.QueryRow("SELECT name, address, mqtt_address FROM infrastructure_nodes WHERE id = $1", id).
		Scan(&n.Name, &addr, &mqttAddr)

	if err == sql.ErrNoRows {
		http.Error(w, "Node not found", 404)
		return
	} else if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	n.Address = addr.String
	n.MQTTAddress = mqttAddr.String

	if s.CA == nil {
		http.Error(w, "CA disabled", 503)
		return
	}

	dnsNames := []string{}
	ips := []net.IP{}

	target := n.MQTTAddress
	if target == "" {
		target = n.Address
	}

	if target != "" {
		dnsNames = append(dnsNames, target)
		if ip := net.ParseIP(target); ip != nil {
			ips = append(ips, ip)
		}
	}

	// Sign
	certPEM, keyPEM, err := s.CA.SignServerCertificate(target, dnsNames, ips)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"ca.crt":     string(s.CA.CertPEM),
		"server.crt": string(certPEM),
		"server.key": string(keyPEM),
	})
}

func (s *Server) generateClientCert(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var n models.InfrastructureNode
	var token sql.NullString

	err := s.DB.QueryRow("SELECT name, token FROM infrastructure_nodes WHERE id = $1", id).
		Scan(&n.Name, &token)

	if err == sql.ErrNoRows {
		http.Error(w, "Node not found", 404)
		return
	} else if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	n.Token = token.String

	if s.CA == nil {
		http.Error(w, "CA disabled", 503)
		return
	}

	tokenSuffix := n.Token
	if strings.HasPrefix(tokenSuffix, "pb_node_") {
		tokenSuffix = tokenSuffix[8:]
	}
	if len(tokenSuffix) > 8 {
		tokenSuffix = tokenSuffix[:8]
	}
	commonName := fmt.Sprintf("%s-%s", n.Name, tokenSuffix)

	// Sign
	certPEM, keyPEM, err := s.CA.SignCertificate(commonName)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"ca.crt":     string(s.CA.CertPEM),
		"client.crt": string(certPEM),
		"client.key": string(keyPEM),
	})
}

func (s *Server) getEnrolledDevices(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.Query("SELECT device_id, auth_token, created_at FROM devices ORDER BY created_at DESC")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var devices []models.EnrolledDevice
	for rows.Next() {
		var d models.EnrolledDevice
		if err := rows.Scan(&d.DeviceID, &d.AuthToken, &d.CreatedAt); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		devices = append(devices, d)
	}

	w.Header().Set("Content-Type", "application/json")
	if devices == nil {
		devices = []models.EnrolledDevice{}
	}
	json.NewEncoder(w).Encode(devices)
}

func (s *Server) enrollDevice(w http.ResponseWriter, r *http.Request) {
	var d models.EnrolledDevice
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	if d.DeviceID == "" {
		http.Error(w, "device_id is required", 400)
		return
	}

	d.AuthToken = "pb_dev_" + s.generateToken(16)

	err := s.DB.QueryRow("INSERT INTO devices (device_id, auth_token) VALUES ($1, $2) RETURNING created_at", 
		d.DeviceID, d.AuthToken).Scan(&d.CreatedAt)
	
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	bundle := map[string]string{
		"device_id":  d.DeviceID,
		"auth_token": d.AuthToken,
	}

	if s.CA != nil {
		certPEM, keyPEM, err := s.CA.SignCertificate(d.DeviceID)
		if err != nil {
			http.Error(w, "Failed to sign certificate: "+err.Error(), 500)
			return
		}
		bundle["ca.crt"] = string(s.CA.CertPEM)
		bundle["client.crt"] = string(certPEM)
		bundle["client.key"] = string(keyPEM)
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(bundle)
}

func (s *Server) deleteEnrolledDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_, err := s.DB.Exec("DELETE FROM devices WHERE device_id=$1", id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(200)
}

func (s *Server) bootstrapDevice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AuthToken string `json:"auth_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", 400)
		return
	}

	var deviceID string
	err := s.DB.QueryRow("SELECT device_id FROM devices WHERE auth_token = $1", req.AuthToken).Scan(&deviceID)
	if err == sql.ErrNoRows {
		http.Error(w, "Invalid AuthToken", http.StatusUnauthorized)
		return
	} else if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	if s.CA == nil {
		http.Error(w, "CA is disabled on this server", http.StatusServiceUnavailable)
		return
	}

	certPEM, keyPEM, err := s.CA.SignCertificate(deviceID)
	if err != nil {
		http.Error(w, "Failed to sign certificate: "+err.Error(), 500)
		return
	}

	bundle := map[string]string{
		"device_id":  deviceID,
		"ca.crt":     string(s.CA.CertPEM),
		"client.crt": string(certPEM),
		"client.key": string(keyPEM),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bundle)
}

func (s *Server) downloadNodeConfig(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var n models.InfrastructureNode
	var addr, mqttAddr sql.NullString

	err := s.DB.QueryRow("SELECT id, name, type, site_id, address, mqtt_address, token FROM infrastructure_nodes WHERE id = $1", id).
		Scan(&n.ID, &n.Name, &n.Type, &n.SiteID, &addr, &mqttAddr, &n.Token)

	if err == sql.ErrNoRows {
		http.Error(w, "Node not found", 404)
		return
	} else if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	n.Address = addr.String
	n.MQTTAddress = mqttAddr.String

	mqttBroker := n.MQTTAddress
	if mqttBroker == "" {
		mqttBroker = n.Address
	}

	tokenSuffix := n.Token
	if strings.HasPrefix(tokenSuffix, "pb_node_") {
		tokenSuffix = tokenSuffix[8:]
	}
	if len(tokenSuffix) > 8 {
		tokenSuffix = tokenSuffix[:8]
	}

	commonName := fmt.Sprintf("node-%d-%s", n.ID, tokenSuffix)
	mqttBrokerTLS := mqttBroker
	var certPEM, keyPEM []byte

	if s.CA != nil {
		var err error
		certPEM, keyPEM, err = s.CA.SignCertificate(commonName)
		if err != nil {
			http.Error(w, "Failed to sign certificate: "+err.Error(), 500)
			return
		}

		if !strings.Contains(mqttBrokerTLS, ":") {
			mqttBrokerTLS = mqttBrokerTLS + ":8883"
		} else {
			mqttBrokerTLS = strings.Replace(mqttBrokerTLS, ":1883", ":8883", 1)
		}

		if !strings.HasPrefix(mqttBrokerTLS, "ssl://") && !strings.HasPrefix(mqttBrokerTLS, "tcps://") {
			mqttBrokerTLS = "ssl://" + mqttBrokerTLS
		}
	} else {
		if !strings.HasPrefix(mqttBrokerTLS, "tcp://") {
			mqttBrokerTLS = "tcp://" + mqttBrokerTLS
		}
		if !strings.Contains(mqttBrokerTLS, ":") {
			mqttBrokerTLS = mqttBrokerTLS + ":1883"
		}
	}

	configYaml := fmt.Sprintf(`local_mqtt:
  broker: %q
  client_id: "potbuddy-local-node-%d"
  sub_topic: "potbuddy/+/raw"
  pub_topic: "potbuddy/processed"
  ca_file: "ca.crt"
  cert_file: "client.crt"
  key_file: "client.key"

cloud_mqtt:
  broker: "tcp://mqtt-0.iot.kaminjitt.com:1883"
  client_id: "potbuddy-cloud-publisher-%d"
  pub_topic: "potbuddy/telemetry"
  username: ""
  password: ""

http:
  port: 8080

store:
  buffer_size: 100

database:
  dsn: "postgres://postgres:postgres@postgresql.iot.kaminjitt.com:5432/potbuddy?sslmode=disable"

device_id: %q
`, mqttBrokerTLS, n.ID, n.ID, commonName)

	bundle := map[string]string{
		"config.yaml": configYaml,
	}
	if s.CA != nil {
		bundle["ca.crt"] = string(s.CA.CertPEM)
		bundle["client.crt"] = string(certPEM)
		bundle["client.key"] = string(keyPEM)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bundle)
}

func (s *Server) regenNodeToken(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	newToken := "pb_node_" + s.generateToken(16)

	result, err := s.DB.Exec("UPDATE infrastructure_nodes SET token=$1 WHERE id=$2", newToken, id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		http.Error(w, "Node not found", 404)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": newToken})
}

func (s *Server) regenDeviceAuthToken(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	newToken := "pb_dev_" + s.generateToken(16)

	result, err := s.DB.Exec("UPDATE devices SET auth_token=$1 WHERE device_id=$2", newToken, id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		http.Error(w, "Device not found", 404)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"auth_token": newToken})
}
