package tls

import (
	"crypto/tls"
	"errors"
)

// TLSProfile composes CertStores into use-case-specific TLS configurations.
type TLSProfile struct {
	ServerCert CertStore // server's own certificate
	ClientCA   CertStore // CA pool for verifying client certs (nil = no mTLS)
	ClientCert CertStore // client's own cert when acting as mTLS client (nil = server-only)
	MinVersion uint16    // minimum TLS version, e.g. tls.VersionTLS12
	VerifyPeer bool      // require and verify peer certificate
}

// ServerTLSConfig builds a *tls.Config suitable for a TLS server.
func (p *TLSProfile) ServerTLSConfig() (*tls.Config, error) {
	if p.ServerCert == nil {
		return nil, errors.New("tls: ServerCert is required")
	}

	var cfg *tls.Config

	// If the store provides a complete TLS config (e.g. ACME with callback-based
	// certificate provisioning), use it as the base.
	if provider, ok := p.ServerCert.(TLSConfigProvider); ok {
		cfg = provider.TLSConfig()
		cfg.MinVersion = p.MinVersion
	} else {
		cert, err := p.ServerCert.GetCertificate()
		if err != nil {
			return nil, err
		}
		cfg = &tls.Config{
			Certificates: []tls.Certificate{*cert},
			MinVersion:   p.MinVersion,
		}
	}

	if p.VerifyPeer && p.ClientCA != nil {
		pool, err := p.ClientCA.GetCACertPool()
		if err != nil {
			return nil, err
		}
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
		cfg.ClientCAs = pool
	}

	return cfg, nil
}

// ClientTLSConfig builds a *tls.Config suitable for a TLS client.
func (p *TLSProfile) ClientTLSConfig() (*tls.Config, error) {
	cfg := &tls.Config{
		MinVersion: p.MinVersion,
	}

	// Use server cert's CA pool as root CAs for verifying the server.
	if p.ServerCert != nil {
		pool, err := p.ServerCert.GetCACertPool()
		if err != nil {
			return nil, err
		}
		cfg.RootCAs = pool
	}

	// Attach client certificate for mTLS.
	if p.ClientCert != nil {
		cert, err := p.ClientCert.GetCertificate()
		if err != nil {
			return nil, err
		}
		cfg.Certificates = []tls.Certificate{*cert}
	}

	return cfg, nil
}
