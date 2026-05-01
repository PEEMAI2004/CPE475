package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
)

func main() {
	managerURL := flag.String("manager", "http://localhost:8081", "Manager API base URL")
	nodeID := flag.Int("id", 0, "Node ID (Legacy)")
	adminToken := flag.String("admin-token", "", "Admin Bearer token (Legacy)")
	nodeToken := flag.String("token", "", "Node Auth Token (pb_node_...)")
	commonName := flag.String("cn", "potbuddy-node", "Common Name for CSR")
	enrollType := flag.String("type", "node", "Enrollment type: node or mqtt")
	flag.Parse()

	if *nodeToken == "" && (*nodeID == 0 || *adminToken == "") {
		fmt.Println("Usage (Recommended): enroll -token <pb_node_...> [-manager <url>] [-type node|mqtt]")
		fmt.Println("Usage (Legacy): enroll -id <node_id> -admin-token <token> [-manager <url>] [-cn <common_name>]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// 1. Generate Private Key
	log.Printf("Generating 2048-bit RSA private key for %s...", *enrollType)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatalf("failed to generate key: %v", err)
	}

	keyBytes := x509.MarshalPKCS1PrivateKey(key)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes})
	
	keyFile := "client.key"
	if *enrollType == "mqtt" {
		keyFile = "server.key"
	}

	if err := os.WriteFile(keyFile, keyPEM, 0600); err != nil {
		log.Fatalf("failed to save %s: %v", keyFile, err)
	}
	fmt.Printf("Saved: %s\n", keyFile)

	// 2. Generate CSR
	log.Println("Generating Certificate Signing Request...")
	template := x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   *commonName,
			Organization: []string{"PotBuddy"},
		},
	}

	if *enrollType == "mqtt" {
		// For MQTT servers, we often want SANs
		if ip := net.ParseIP(*commonName); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, *commonName)
		}
	}

	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, &template, key)
	if err != nil {
		log.Fatalf("failed to create CSR: %v", err)
	}
	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrBytes})
	
	csrFile := "client.csr"
	if *enrollType == "mqtt" {
		csrFile = "server.csr"
	}
	if err := os.WriteFile(csrFile, csrPEM, 0644); err != nil {
		log.Fatalf("failed to save %s: %v", csrFile, err)
	}
	fmt.Printf("Saved: %s\n", csrFile)

	// 3. Submit to Manager API
	var url string
	if *nodeToken != "" {
		if *enrollType == "mqtt" {
			url = fmt.Sprintf("%s/api/enrollment/bootstrap/mqtt", *managerURL)
		} else {
			url = fmt.Sprintf("%s/api/enrollment/bootstrap/node", *managerURL)
		}
	} else {
		if *enrollType == "mqtt" {
			url = fmt.Sprintf("%s/api/enrollment/nodes/%d/server-cert", *managerURL, *nodeID)
		} else {
			url = fmt.Sprintf("%s/api/enrollment/nodes/%d/config", *managerURL, *nodeID)
		}
	}

	log.Printf("Submitting CSR to %s...", url)
	reqBody, _ := json.Marshal(map[string]string{
		"csr": string(csrPEM),
	})

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		log.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	
	if *nodeToken != "" {
		req.Header.Set("X-PotBuddy-Token", *nodeToken)
	} else {
		req.Header.Set("Authorization", "Bearer "+*adminToken)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Fatalf("enrollment failed (status %d): %s", resp.StatusCode, string(body))
	}

	var bundle map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&bundle); err != nil {
		log.Fatalf("failed to decode response: %v", err)
	}

	// 4. Save components
	if cert, ok := bundle["client.crt"]; ok {
		os.WriteFile("client.crt", []byte(cert), 0644)
		fmt.Println("Saved: client.crt")
	}
	if cert, ok := bundle["server.crt"]; ok {
		os.WriteFile("server.crt", []byte(cert), 0644)
		fmt.Println("Saved: server.crt")
	}
	if caCert, ok := bundle["ca.crt"]; ok {
		os.WriteFile("ca.crt", []byte(caCert), 0644)
		fmt.Println("Saved: ca.crt")
	}
	if configYaml, ok := bundle["config.yaml"]; ok {
		os.WriteFile("config.yaml", []byte(configYaml), 0644)
		fmt.Println("Saved: config.yaml")
	}
	if mqttConf, ok := bundle["mosquitto.conf"]; ok {
		os.WriteFile("mosquitto.conf", []byte(mqttConf), 0644)
		fmt.Println("Saved: mosquitto.conf")
	}

	fmt.Println("Enrollment successful!")
}
