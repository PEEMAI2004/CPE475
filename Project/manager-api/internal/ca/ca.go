package ca

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"time"
)

type CA struct {
	Cert     *x509.Certificate
	Key      *rsa.PrivateKey
	CertPEM  []byte
}

func LoadOrCreateCA(certPath, keyPath string) (*CA, error) {
	if _, err := os.Stat(certPath); err == nil {
		// Load existing
		certPEM, err := os.ReadFile(certPath)
		if err != nil {
			return nil, err
		}
		keyPEM, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, err
		}

		block, _ := pem.Decode(certPEM)
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}

		keyBlock, _ := pem.Decode(keyPEM)
		var key *rsa.PrivateKey
		if keyAny, err8 := x509.ParsePKCS8PrivateKey(keyBlock.Bytes); err8 == nil {
			var ok bool
			if key, ok = keyAny.(*rsa.PrivateKey); !ok {
				return nil, fmt.Errorf("key is not RSA")
			}
		} else {
			var err1 error
			key, err1 = x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
			if err1 != nil {
				return nil, err1
			}
		}

		return &CA{Cert: cert, Key: key, CertPEM: certPEM}, nil
	}

	// Create new
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"PotBuddy"},
			CommonName:   "PotBuddy Root CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		return nil, err
	}

	cert, _ := x509.ParseCertificate(certBytes)
	return &CA{Cert: cert, Key: key, CertPEM: certPEM}, nil
}

func (ca *CA) SignCertificate(commonName string) (certPEM []byte, keyPEM []byte, err error) {
	return ca.sign(commonName, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, nil, nil)
}

func (ca *CA) SignServerCertificate(commonName string, dnsNames []string, ipAddresses []net.IP) (certPEM []byte, keyPEM []byte, err error) {
	return ca.sign(commonName, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, dnsNames, ipAddresses)
}

func (ca *CA) SignCSR(csrPEM []byte, extKeyUsage []x509.ExtKeyUsage) (certPEM []byte, err error) {
	block, _ := pem.Decode(csrPEM)
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		return nil, fmt.Errorf("invalid CSR PEM")
	}

	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, err
	}

	if err := csr.CheckSignature(); err != nil {
		return nil, err
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      csr.Subject,
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(1, 0, 0),
		ExtKeyUsage:  extKeyUsage,
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		DNSNames:     csr.DNSNames,
		IPAddresses:  csr.IPAddresses,
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, ca.Cert, csr.PublicKey, ca.Key)
	if err != nil {
		return nil, err
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	return certPEM, nil
}

func (ca *CA) sign(commonName string, extKeyUsage []x509.ExtKeyUsage, dnsNames []string, ipAddresses []net.IP) (certPEM []byte, keyPEM []byte, err error) {
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, err
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"PotBuddy"},
			CommonName:   commonName,
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(1, 0, 0),
		ExtKeyUsage: extKeyUsage,
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		DNSNames:    dnsNames,
		IPAddresses: ipAddresses,
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, ca.Cert, &clientKey.PublicKey, ca.Key)
	if err != nil {
		return nil, nil, err
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientKey)})

	return certPEM, keyPEM, nil
}
