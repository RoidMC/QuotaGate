package kextls

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
	"path/filepath"
	"time"
)

type SelfSignedCert struct {
	CertFile string
	KeyFile  string
}

func GenerateSelfSignedCert(certDir, serviceName string) (*SelfSignedCert, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("kex-utils/tlsutil: failed to generate private key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("kex-utils/tlsutil: failed to generate serial number: %w", err)
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour)

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"RoidMC Studios"},
			CommonName:   fmt.Sprintf("%s - kex-utils Self Signed Cert (Development Only)", serviceName),
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost", "127.0.0.1"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, fmt.Errorf("kex-utils/tlsutil: failed to create certificate: %w", err)
	}

	if err := os.MkdirAll(certDir, 0755); err != nil {
		return nil, fmt.Errorf("kex-utils/tlsutil: failed to create cert directory: %w", err)
	}

	certFile := filepath.Join(certDir, fmt.Sprintf("%s-cert.pem", serviceName))
	keyFile := filepath.Join(certDir, fmt.Sprintf("%s-key.pem", serviceName))

	certOut, err := os.Create(certFile)
	if err != nil {
		return nil, fmt.Errorf("kex-utils/tlsutil: failed to create cert file: %w", err)
	}
	defer certOut.Close()

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return nil, fmt.Errorf("kex-utils/tlsutil: failed to write cert: %w", err)
	}

	keyOut, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, fmt.Errorf("kex-utils/tlsutil: failed to create key file: %w", err)
	}
	defer keyOut.Close()

	privBytes := x509.MarshalPKCS1PrivateKey(priv)
	if err := pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privBytes}); err != nil {
		return nil, fmt.Errorf("kex-utils/tlsutil: failed to write key: %w", err)
	}

	return &SelfSignedCert{
		CertFile: certFile,
		KeyFile:  keyFile,
	}, nil
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
