package tls

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"

	"golang.org/x/crypto/acme/autocert"
)

// ACMEConfig configures an ACME-based certificate store.
type ACMEConfig struct {
	Email    string   // registration email for Let's Encrypt
	Domains  []string // allowed hostnames
	CacheDir string   // on-disk certificate cache directory
}

// ACMEStore provisions and renews certificates via ACME (Let's Encrypt).
// Implements CertStore.
type ACMEStore struct {
	manager *autocert.Manager
}

// NewACMEStore creates an ACMEStore with the given config.
func NewACMEStore(cfg ACMEConfig) (*ACMEStore, error) {
	if len(cfg.Domains) == 0 {
		return nil, errors.New("tls: ACME requires at least one domain")
	}

	m := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Email:      cfg.Email,
		HostPolicy: autocert.HostWhitelist(cfg.Domains...),
	}
	if cfg.CacheDir != "" {
		m.Cache = autocert.DirCache(cfg.CacheDir)
	}

	return &ACMEStore{manager: m}, nil
}

func (s *ACMEStore) GetCertificate() (*tls.Certificate, error) {
	// ACME certificates are served via TLS handshake callback.
	// Callers should use TLSConfig() or TLSProfile.ServerTLSConfig() instead.
	return nil, errors.New("tls: ACME certificates are served via TLS handshake callback; use TLSProfile.ServerTLSConfig()")
}

// TLSConfig returns a *tls.Config that uses autocert's GetCertificate callback.
func (s *ACMEStore) TLSConfig() *tls.Config {
	return s.manager.TLSConfig()
}

// HTTPHandler returns an http.Handler for ACME HTTP-01 challenge responses.
// Mount this on port 80. Pass a fallback handler for non-ACME requests.
func (s *ACMEStore) HTTPHandler(fallback http.Handler) http.Handler {
	return s.manager.HTTPHandler(fallback)
}

func (s *ACMEStore) GetCACertPool() (*x509.CertPool, error) {
	// ACME certs chain to a public CA; use system roots.
	return nil, nil
}
