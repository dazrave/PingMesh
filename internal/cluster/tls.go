package cluster

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

// GenerateCA creates a new EC P-256 CA certificate and key.
func GenerateCA(certsDir string) error {
	if err := os.MkdirAll(certsDir, 0700); err != nil {
		return fmt.Errorf("creating certs directory: %w", err)
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generating CA key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("generating serial number: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"PingMesh"},
			CommonName:   "PingMesh Internal CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour), // 10 years
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("creating CA certificate: %w", err)
	}

	// Write CA cert
	certPath := filepath.Join(certsDir, "ca.crt")
	certFile, err := os.OpenFile(certPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("creating CA cert file: %w", err)
	}
	defer certFile.Close()
	pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	// Write CA key
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("marshalling CA key: %w", err)
	}
	keyPath := filepath.Join(certsDir, "ca.key")
	keyFile, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("creating CA key file: %w", err)
	}
	defer keyFile.Close()
	pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return nil
}

// GenerateNodeCert creates a certificate for a node, signed by the CA.
func GenerateNodeCert(certsDir, nodeID string, addresses []string) error {
	// Load CA
	caCertPEM, err := os.ReadFile(filepath.Join(certsDir, "ca.crt"))
	if err != nil {
		return fmt.Errorf("reading CA cert: %w", err)
	}
	caKeyPEM, err := os.ReadFile(filepath.Join(certsDir, "ca.key"))
	if err != nil {
		return fmt.Errorf("reading CA key: %w", err)
	}

	caCertBlock, _ := pem.Decode(caCertPEM)
	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		return fmt.Errorf("parsing CA cert: %w", err)
	}

	caKeyBlock, _ := pem.Decode(caKeyPEM)
	caKey, err := x509.ParseECPrivateKey(caKeyBlock.Bytes)
	if err != nil {
		return fmt.Errorf("parsing CA key: %w", err)
	}

	// Generate node key
	nodeKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generating node key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("generating serial number: %w", err)
	}

	var ipAddresses []net.IP
	var dnsNames []string
	for _, addr := range addresses {
		if ip := net.ParseIP(addr); ip != nil {
			ipAddresses = append(ipAddresses, ip)
		} else {
			dnsNames = append(dnsNames, addr)
		}
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"PingMesh"},
			CommonName:   "pingmesh-" + nodeID,
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour), // 1 year
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IPAddresses: ipAddresses,
		DNSNames:    dnsNames,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, caCert, &nodeKey.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("creating node certificate: %w", err)
	}

	// Write node cert
	certPath := filepath.Join(certsDir, "node.crt")
	certFile, err := os.OpenFile(certPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("creating node cert file: %w", err)
	}
	defer certFile.Close()
	pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	// Write node key
	keyDER, err := x509.MarshalECPrivateKey(nodeKey)
	if err != nil {
		return fmt.Errorf("marshalling node key: %w", err)
	}
	keyPath := filepath.Join(certsDir, "node.key")
	keyFile, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("creating node key file: %w", err)
	}
	defer keyFile.Close()
	pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return nil
}

// GenerateNodeCertPEM creates a certificate for a node signed by the CA and
// returns the PEM-encoded certificate and key bytes instead of writing to disk.
func GenerateNodeCertPEM(certsDir, nodeID string, addresses []string) (certPEM, keyPEM []byte, err error) {
	// Load CA
	caCertPEMData, err := os.ReadFile(filepath.Join(certsDir, "ca.crt"))
	if err != nil {
		return nil, nil, fmt.Errorf("reading CA cert: %w", err)
	}
	caKeyPEMData, err := os.ReadFile(filepath.Join(certsDir, "ca.key"))
	if err != nil {
		return nil, nil, fmt.Errorf("reading CA key: %w", err)
	}

	caCertBlock, _ := pem.Decode(caCertPEMData)
	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing CA cert: %w", err)
	}

	caKeyBlock, _ := pem.Decode(caKeyPEMData)
	caKey, err := x509.ParseECPrivateKey(caKeyBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing CA key: %w", err)
	}

	// Generate node key
	nodeKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generating node key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, fmt.Errorf("generating serial number: %w", err)
	}

	var ipAddresses []net.IP
	var dnsNames []string
	for _, addr := range addresses {
		if ip := net.ParseIP(addr); ip != nil {
			ipAddresses = append(ipAddresses, ip)
		} else {
			dnsNames = append(dnsNames, addr)
		}
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"PingMesh"},
			CommonName:   "pingmesh-" + nodeID,
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IPAddresses: ipAddresses,
		DNSNames:    dnsNames,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, caCert, &nodeKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("creating node certificate: %w", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(nodeKey)
	if err != nil {
		return nil, nil, fmt.Errorf("marshalling node key: %w", err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return certPEM, keyPEM, nil
}
