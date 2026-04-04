package tls

import (
	"crypto/tls"
	"crypto/x509"
)

// CertStore abstracts where TLS certificates come from.
type CertStore interface {
	// GetCertificate returns a TLS certificate, possibly refreshing it.
	GetCertificate() (*tls.Certificate, error)
	// GetCACertPool returns the CA pool for peer verification.
	// Returns nil, nil if no CA pool is configured.
	GetCACertPool() (*x509.CertPool, error)
}
