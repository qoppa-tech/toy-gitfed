package tls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// FileCertStoreConfig configures a file-based certificate store.
type FileCertStoreConfig struct {
	CertPath string // path to PEM-encoded certificate
	KeyPath  string // path to PEM-encoded private key
	CAPath   string // optional path to PEM-encoded CA certificate(s)
}

// FileCertStore loads TLS certificates from PEM files on disk.
// Implements CertStore.
type FileCertStore struct {
	certPath string
	keyPath  string
	caPath   string
}

// NewFileCertStore creates a FileCertStore and validates that the cert/key can be loaded.
func NewFileCertStore(cfg FileCertStoreConfig) (*FileCertStore, error) {
	_, err := tls.LoadX509KeyPair(cfg.CertPath, cfg.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCertNotFound, err)
	}

	store := &FileCertStore{
		certPath: cfg.CertPath,
		keyPath:  cfg.KeyPath,
		caPath:   cfg.CAPath,
	}
	return store, nil
}

func (s *FileCertStore) GetCertificate() (*tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(s.certPath, s.keyPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCertInvalid, err)
	}
	return &cert, nil
}

func (s *FileCertStore) GetCACertPool() (*x509.CertPool, error) {
	if s.caPath == "" {
		return nil, nil
	}

	caPEM, err := os.ReadFile(s.caPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCertNotFound, err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("%w: failed to parse CA certificate", ErrCertInvalid)
	}
	return pool, nil
}
