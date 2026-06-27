package tlsutil_test

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	tlsutil "github.com/roidmc/quotagate/internal/util/tls"
)

func TestGenerateSelfSignedCert(t *testing.T) {
	tmpDir := t.TempDir()

	cert, err := tlsutil.GenerateSelfSignedCert(tmpDir, "test-service")
	if err != nil {
		t.Fatalf("GenerateSelfSignedCert failed: %v", err)
	}

	// Verify files exist
	if _, err := os.Stat(cert.CertFile); os.IsNotExist(err) {
		t.Errorf("cert file not created: %s", cert.CertFile)
	}
	if _, err := os.Stat(cert.KeyFile); os.IsNotExist(err) {
		t.Errorf("key file not created: %s", cert.KeyFile)
	}

	// Verify key file permissions (should be 0600, Unix only)
	if runtime.GOOS != "windows" {
		info, err := os.Stat(cert.KeyFile)
		if err != nil {
			t.Fatalf("failed to stat key file: %v", err)
		}
		if info.Mode().Perm() != 0600 {
			t.Errorf("key file permissions = %o, want 0600", info.Mode().Perm())
		}
	}

	// Parse and verify certificate
	certPEM, err := os.ReadFile(cert.CertFile)
	if err != nil {
		t.Fatalf("failed to read cert file: %v", err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("failed to decode cert PEM")
	}

	parsedCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}

	// Verify certificate fields
	if len(parsedCert.Subject.Organization) == 0 || parsedCert.Subject.Organization[0] != "quotagate" {
		t.Errorf("organization = %v, want quotagate", parsedCert.Subject.Organization)
	}

	if !strings.Contains(parsedCert.Subject.CommonName, "Development Only") {
		t.Errorf("common name = %q, should contain 'Development Only'", parsedCert.Subject.CommonName)
	}

	// Verify validity period (~365 days)
	expectedNotAfter := parsedCert.NotBefore.Add(365 * 24 * time.Hour)
	diff := parsedCert.NotAfter.Sub(expectedNotAfter)
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Minute {
		t.Errorf("validity period incorrect: NotBefore=%v, NotAfter=%v", parsedCert.NotBefore, parsedCert.NotAfter)
	}

	// Verify key usage
	if parsedCert.KeyUsage&x509.KeyUsageKeyEncipherment == 0 {
		t.Error("missing KeyEncipherment key usage")
	}
	if parsedCert.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		t.Error("missing DigitalSignature key usage")
	}

	// Verify extended key usage
	hasServerAuth := false
	for _, usage := range parsedCert.ExtKeyUsage {
		if usage == x509.ExtKeyUsageServerAuth {
			hasServerAuth = true
			break
		}
	}
	if !hasServerAuth {
		t.Error("missing ServerAuth extended key usage")
	}
}

func TestGenerateSelfSignedCertEmptyServiceName(t *testing.T) {
	tmpDir := t.TempDir()

	cert, err := tlsutil.GenerateSelfSignedCert(tmpDir, "")
	if err != nil {
		t.Fatalf("GenerateSelfSignedCert with empty service name failed: %v", err)
	}

	// Should still create files with empty prefix
	if !strings.HasSuffix(cert.CertFile, "-cert.pem") {
		t.Errorf("cert file name = %q, expected suffix '-cert.pem'", cert.CertFile)
	}
}

func TestFileExists(t *testing.T) {
	tmpDir := t.TempDir()
	existingFile := filepath.Join(tmpDir, "exists.txt")

	if err := os.WriteFile(existingFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if !tlsutil.FileExists(existingFile) {
		t.Error("FileExists returned false for existing file")
	}

	if tlsutil.FileExists(filepath.Join(tmpDir, "not-exists.txt")) {
		t.Error("FileExists returned true for non-existing file")
	}
}
