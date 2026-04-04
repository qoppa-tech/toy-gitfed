package tls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeTestPEM generates a self-signed cert+key and writes PEM files to dir.
// Returns paths to cert.pem, key.pem, ca.pem.
func writeTestPEM(t *testing.T, dir string) (certPath, keyPath, caPath string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "test"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(1 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"localhost"},
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")
	caPath = filepath.Join(dir, "ca.pem")

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	if err := os.WriteFile(certPath, certPEM, 0600); err != nil {
		t.Fatal(err)
	}

	keyDER, _ := x509.MarshalECPrivateKey(key)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(caPath, certPEM, 0600); err != nil {
		t.Fatal(err)
	}

	return certPath, keyPath, caPath
}

func TestFileCertStoreGetCertificate(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath, _ := writeTestPEM(t, dir)

	store, err := NewFileCertStore(FileCertStoreConfig{
		CertPath: certPath,
		KeyPath:  keyPath,
	})
	if err != nil {
		t.Fatalf("NewFileCertStore: %v", err)
	}

	cert, err := store.GetCertificate()
	if err != nil {
		t.Fatalf("GetCertificate: %v", err)
	}
	if cert == nil || len(cert.Certificate) == 0 {
		t.Fatal("expected valid certificate")
	}
}

func TestFileCertStoreGetCACertPool(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath, caPath := writeTestPEM(t, dir)

	store, err := NewFileCertStore(FileCertStoreConfig{
		CertPath: certPath,
		KeyPath:  keyPath,
		CAPath:   caPath,
	})
	if err != nil {
		t.Fatalf("NewFileCertStore: %v", err)
	}

	pool, err := store.GetCACertPool()
	if err != nil {
		t.Fatalf("GetCACertPool: %v", err)
	}
	if pool == nil {
		t.Fatal("expected non-nil CA pool")
	}
}

func TestFileCertStoreNoCACertPool(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath, _ := writeTestPEM(t, dir)

	store, err := NewFileCertStore(FileCertStoreConfig{
		CertPath: certPath,
		KeyPath:  keyPath,
	})
	if err != nil {
		t.Fatalf("NewFileCertStore: %v", err)
	}

	pool, err := store.GetCACertPool()
	if err != nil {
		t.Fatalf("GetCACertPool: %v", err)
	}
	if pool != nil {
		t.Fatal("expected nil CA pool when no CAPath")
	}
}

func TestFileCertStoreMissingCert(t *testing.T) {
	_, err := NewFileCertStore(FileCertStoreConfig{
		CertPath: "/nonexistent/cert.pem",
		KeyPath:  "/nonexistent/key.pem",
	})
	if err == nil {
		t.Fatal("expected error for missing files")
	}
}
