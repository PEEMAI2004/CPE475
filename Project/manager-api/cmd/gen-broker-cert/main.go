package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	_ "github.com/lib/pq"
	"github.com/potbuddy/manager-api/internal/ca"
	"github.com/potbuddy/manager-api/internal/models"
)

func main() {
	if len(os.Args) < 3 {
		log.Fatal("Usage: gen-cert <mode: broker|node> <id>")
	}
	mode := os.Args[1]
	id := os.Args[2]

	caInstance, err := ca.LoadOrCreateCA("certs/ca.crt", "certs/ca.key")
	if err != nil {
		log.Fatalf("Failed to load CA: %v", err)
	}

	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@postgresql.iot.kaminjitt.com:5432/potbuddy?sslmode=disable"
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if mode == "broker" {
		generateBrokerCert(db, caInstance, id)
	} else if mode == "node" {
		generateNodeConfig(db, caInstance, id)
	} else {
		log.Fatal("Invalid mode")
	}
}

func generateBrokerCert(db *sql.DB, caInstance *ca.CA, id string) {
	var target, addr, mqttAddr string
	err := db.QueryRow("SELECT address, mqtt_address FROM infrastructure_nodes WHERE id = $1", id).
		Scan(&addr, &mqttAddr)
	if err != nil {
		log.Fatalf("Failed to find node in DB: %v", err)
	}

	target = mqttAddr
	if target == "" {
		target = addr
	}

	dnsNames := []string{target}
	ips := []net.IP{}
	if ip := net.ParseIP(target); ip != nil {
		ips = append(ips, ip)
	}

	certPEM, keyPEM, err := caInstance.SignServerCertificate(target, dnsNames, ips)
	if err != nil {
		log.Fatalf("Failed to sign certificate: %v", err)
	}

	bundle := map[string]string{
		"ca.crt":     string(caInstance.CertPEM),
		"server.crt": string(certPEM),
		"server.key": string(keyPEM),
	}
	json.NewEncoder(os.Stdout).Encode(bundle)
}

func generateNodeConfig(db *sql.DB, caInstance *ca.CA, id string) {
	var n models.InfrastructureNode
	var addr, mqttAddr sql.NullString

	err := db.QueryRow("SELECT id, name, type, site_id, address, mqtt_address, token FROM infrastructure_nodes WHERE id = $1", id).
		Scan(&n.ID, &n.Name, &n.Type, &n.SiteID, &addr, &mqttAddr, &n.Token)

	if err != nil {
		log.Fatalf("Failed to find node: %v", err)
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
	
	certPEM, keyPEM, err := caInstance.SignCertificate(commonName)
	if err != nil {
		log.Fatalf("Failed to sign: %v", err)
	}

	if !strings.Contains(mqttBrokerTLS, ":") {
		mqttBrokerTLS = mqttBrokerTLS + ":8883"
	} else {
		mqttBrokerTLS = strings.Replace(mqttBrokerTLS, ":1883", ":8883", 1)
	}
	if !strings.HasPrefix(mqttBrokerTLS, "ssl://") && !strings.HasPrefix(mqttBrokerTLS, "tcps://") {
		mqttBrokerTLS = "ssl://" + mqttBrokerTLS
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
		"ca.crt":      string(caInstance.CertPEM),
		"client.crt":  string(certPEM),
		"client.key":  string(keyPEM),
	}
	json.NewEncoder(os.Stdout).Encode(bundle)
}
