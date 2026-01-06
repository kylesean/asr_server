package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

func main() {
	fmt.Println("[INFO] Generating SSL certificates...")

	// Create ssl directory
	sslDir := "../ssl"
	if err := os.MkdirAll(sslDir, 0755); err != nil {
		fmt.Printf("[ERROR] Failed to create SSL directory: %v\n", err)
		os.Exit(1)
	}

	// Generate private key
	fmt.Println("[INFO] Generating ECDSA private key...")
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		fmt.Printf("[ERROR] Failed to generate private key: %v\n", err)
		os.Exit(1)
	}

	// Create certificate template
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		fmt.Printf("[ERROR] Failed to generate serial number: %v\n", err)
		os.Exit(1)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization:  []string{"VAD ASR Server"},
			Country:       []string{"CN"},
			Province:      []string{"Beijing"},
			Locality:      []string{"Beijing"},
			StreetAddress: []string{},
			PostalCode:    []string{},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour), // 1 year validity
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		DNSNames:              []string{"localhost", "*.localhost"},
	}

	// Create certificate
	fmt.Println("[INFO] Generating self-signed certificate...")
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		fmt.Printf("[ERROR] Failed to create certificate: %v\n", err)
		os.Exit(1)
	}

	// Save certificate
	certPath := filepath.Join(sslDir, "cert.pem")
	certOut, err := os.Create(certPath)
	if err != nil {
		fmt.Printf("[ERROR] Failed to create certificate file: %v\n", err)
		os.Exit(1)
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		fmt.Printf("[ERROR] Failed to write certificate data: %v\n", err)
		os.Exit(1)
	}
	if err := certOut.Close(); err != nil {
		fmt.Printf("[ERROR] Failed to close certificate file: %v\n", err)
		os.Exit(1)
	}

	// Save private key
	keyPath := filepath.Join(sslDir, "key.pem")
	keyOut, err := os.Create(keyPath)
	if err != nil {
		fmt.Printf("[ERROR] Failed to create private key file: %v\n", err)
		os.Exit(1)
	}
	privBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		fmt.Printf("[ERROR] Failed to serialize private key: %v\n", err)
		os.Exit(1)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		fmt.Printf("[ERROR] Failed to write private key data: %v\n", err)
		os.Exit(1)
	}
	if err := keyOut.Close(); err != nil {
		fmt.Printf("[ERROR] Failed to close private key file: %v\n", err)
		os.Exit(1)
	}

	// Set file permissions (only on Unix-like systems)
	os.Chmod(keyPath, 0600)  // Private key readable only by owner
	os.Chmod(certPath, 0644) // Certificate can be read by others

	fmt.Println("[OK] SSL certificates generated successfully!")
	fmt.Printf("[INFO] Certificate location: %s\n", sslDir)
	fmt.Printf("[INFO] Certificate file: %s\n", certPath)
	fmt.Printf("[INFO] Private key file: %s\n", keyPath)
	fmt.Println("")
	fmt.Println("[WARN] Important notes:")
	fmt.Println("  - This is a self-signed certificate, browsers will show security warnings")
	fmt.Println("  - You need to manually accept the certificate on first visit")
	fmt.Println("  - Certificate validity: 365 days")
	fmt.Println("  - Supported domains: localhost, *.localhost")
	fmt.Println("  - Supported IPs: 127.0.0.1, ::1")
}
