package api

import (
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
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

	var req struct {
		CSR string `json:"csr"`
	}
	json.NewDecoder(r.Body).Decode(&req)

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

	var certPEM, keyPEM []byte
	if req.CSR != "" {

		certPEM, err = s.CA.SignCSR([]byte(req.CSR), []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, target, dnsNames, ips)
	} else {
		// Sign with generated key (Key Escrow - Legacy)
		certPEM, keyPEM, err = s.CA.SignServerCertificate(target, dnsNames, ips)
	}

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	res := map[string]string{
		"ca.crt":     string(s.CA.CertPEM),
		"server.crt": string(certPEM),
	}
	if keyPEM != nil {
		res["server.key"] = string(keyPEM)
	}

	// Add Mosquitto config template
	res["mosquitto.conf"] = fmt.Sprintf(`allow_anonymous true

# Standard listener
listener 1883

# mTLS listener
listener 8883
cafile /mosquitto/config/ca.crt
certfile /mosquitto/config/server.crt
keyfile /mosquitto/config/server.key
require_certificate true
use_identity_as_username true
`)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
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

	var req struct {
		CSR string `json:"csr"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	tokenSuffix := n.Token
	if strings.HasPrefix(tokenSuffix, "pb_node_") {
		tokenSuffix = tokenSuffix[8:]
	}
	if len(tokenSuffix) > 8 {
		tokenSuffix = tokenSuffix[:8]
	}
	commonName := fmt.Sprintf("%s-%s", n.Name, tokenSuffix)

	var certPEM, keyPEM []byte
	if req.CSR != "" {
		certPEM, err = s.CA.SignCSR([]byte(req.CSR), []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, commonName, nil, nil)
	} else {
		// Sign with generated key (Key Escrow - Legacy)
		certPEM, keyPEM, err = s.CA.SignCertificate(commonName)
	}

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	res := map[string]string{
		"ca.crt":     string(s.CA.CertPEM),
		"client.crt": string(certPEM),
	}
	if keyPEM != nil {
		res["client.key"] = string(keyPEM)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
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
	var req struct {
		DeviceID string `json:"device_id"`
		CSR      string `json:"csr"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	if req.DeviceID == "" {
		http.Error(w, "device_id is required", 400)
		return
	}

	authToken := "pb_dev_" + s.generateToken(16)
	var createdAt string

	err := s.DB.QueryRow("INSERT INTO devices (device_id, auth_token) VALUES ($1, $2) RETURNING created_at", 
		req.DeviceID, authToken).Scan(&createdAt)
	
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	bundle := map[string]string{
		"device_id":  req.DeviceID,
		"auth_token": authToken,
	}

	if s.CA != nil {
		var certPEM, keyPEM []byte
		var err error
		if req.CSR != "" {
			certPEM, err = s.CA.SignCSR([]byte(req.CSR), []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, req.DeviceID, nil, nil)
		} else {
			certPEM, keyPEM, err = s.CA.SignCertificate(req.DeviceID)
		}

		if err != nil {
			http.Error(w, "Failed to sign certificate: "+err.Error(), 500)
			return
		}
		bundle["ca.crt"] = string(s.CA.CertPEM)
		bundle["client.crt"] = string(certPEM)
		if keyPEM != nil {
			bundle["client.key"] = string(keyPEM)
		}
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
		CSR       string `json:"csr"`
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

	var certPEM, keyPEM []byte
	if req.CSR != "" {
		certPEM, err = s.CA.SignCSR([]byte(req.CSR), []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, deviceID, nil, nil)
	} else {
		certPEM, keyPEM, err = s.CA.SignCertificate(deviceID)
	}

	if err != nil {
		http.Error(w, "Failed to sign certificate: "+err.Error(), 500)
		return
	}

	bundle := map[string]string{
		"device_id":  deviceID,
		"ca.crt":     string(s.CA.CertPEM),
		"client.crt": string(certPEM),
	}
	if keyPEM != nil {
		bundle["client.key"] = string(keyPEM)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bundle)
}

func (s *Server) bootstrapNodeConfig(w http.ResponseWriter, r *http.Request) {
	identity, ok := r.Context().Value("identity").(*models.MachineIdentity)
	if !ok || identity.Type != "node" {
		http.Error(w, "Node identity required", http.StatusUnauthorized)
		return
	}

	// Use existing download logic but with ID from identity
	r.SetPathValue("id", strconv.Itoa(identity.DBID))
	s.downloadNodeConfig(w, r)
}

func (s *Server) bootstrapBrokerCert(w http.ResponseWriter, r *http.Request) {
	identity, ok := r.Context().Value("identity").(*models.MachineIdentity)
	if !ok || identity.Type != "node" {
		http.Error(w, "Node identity required", http.StatusUnauthorized)
		return
	}

	// Use existing server-cert logic but with ID from identity
	r.SetPathValue("id", strconv.Itoa(identity.DBID))
	s.generateServerCert(w, r)
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

	// For infrastructure nodes, we use a predictable CN that includes the ID
	commonName := fmt.Sprintf("node-%d-%s", n.ID, tokenSuffix)
	mqttBrokerTLS := mqttBroker
	
	var req struct {
		CSR string `json:"csr"`
	}
	if r.Method == http.MethodPost {
		json.NewDecoder(r.Body).Decode(&req)
	}

	dnsNames := []string{}
	ips := []net.IP{}
	if n.Address != "" {
		dnsNames = append(dnsNames, n.Address)
		if ip := net.ParseIP(n.Address); ip != nil {
			ips = append(ips, ip)
		}
	}

	var certPEM, keyPEM []byte
	if s.CA != nil {
		var err error
		if req.CSR != "" {
			certPEM, err = s.CA.SignCSR([]byte(req.CSR), []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}, commonName, dnsNames, ips)
		} else {
			// Fallback to Key Escrow if no CSR provided (backward compatibility)
			certPEM, keyPEM, err = s.CA.SignCombinedCertificate(commonName, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}, dnsNames, ips)
		}
		
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

	// Customize DSN if database is not local
	dbHost := "postgresql.iot.kaminjitt.com"
	if os.Getenv("ENV") == "docker" {
		dbHost = "postgres"
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
  ca_file: "ca.crt"
  cert_file: "client.crt"
  key_file: "client.key"

store:
  buffer_size: 100

database:
  dsn: "postgres://postgres:postgres@%s:5432/potbuddy?sslmode=disable"

device_id: %q
`, mqttBrokerTLS, n.ID, n.ID, dbHost, commonName)

	bundle := map[string]string{
		"config.yaml": configYaml,
	}
	if s.CA != nil {
		bundle["ca.crt"] = string(s.CA.CertPEM)
		bundle["client.crt"] = string(certPEM)
		if keyPEM != nil {
			bundle["client.key"] = string(keyPEM)
		}
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
