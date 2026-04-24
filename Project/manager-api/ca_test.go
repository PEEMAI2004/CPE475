package main

import (
	"os"
	"testing"
)

func TestCASign(t *testing.T) {
	// Clean up
	os.Remove("test_ca.crt")
	os.Remove("test_ca.key")
	defer os.Remove("test_ca.crt")
	defer os.Remove("test_ca.key")

	ca, err := LoadOrCreateCA("test_ca.crt", "test_ca.key")
	if err != nil {
		t.Fatalf("Failed to create CA: %v", err)
	}

	certPEM, keyPEM, err := ca.SignCertificate("test-node")
	if err != nil {
		t.Fatalf("Failed to sign certificate: %v", err)
	}

	if len(certPEM) == 0 || len(keyPEM) == 0 {
		t.Error("Generated PEMs are empty")
	}
	
	t.Log("Successfully generated and signed certificate")
}
