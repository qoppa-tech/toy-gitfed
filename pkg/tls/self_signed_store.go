package tls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"time"
)

// SelfSignedConfig configures a self-signed certificate store.
type SelfSignedConfig struct {
	Hosts    []string      // SANs: hostnames and/or IPs
	Duration time.Duration // certificate validity period
}

// SelfSignedStore generates and caches a self-signed certificate.
// Implements CertStore.
type SelfSignedStore struct {
	cert   *tls.Certificate
	caPool *x509.CertPool
}

// NewSelfSignedStore generates a self-signed certificate with the given config.
func NewSelfSignedStore(cfg SelfSignedConfig) (*SelfSignedStore, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}

	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "self-signed"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(cfg.Duration),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	for _, h := range cfg.Hosts {
		if ip := net.ParseIP(h); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, h)
		}
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}

	tlsCert := &tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  key,
	}

	parsed, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	pool.AddCert(parsed)

	return &SelfSignedStore{
		cert:   tlsCert,
		caPool: pool,
	}, nil
}

func (s *SelfSignedStore) GetCertificate() (*tls.Certificate, error) {
	return s.cert, nil
}

func (s *SelfSignedStore) GetCACertPool() (*x509.CertPool, error) {
	return s.caPool, nil
}
