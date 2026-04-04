package transport

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"

	pkgtls "github.com/qoppa-tech/toy-gitfed/pkg/tls"
)

// Transport provides protocol-agnostic secure connections.
type Transport interface {
	// Dial establishes a TLS-secured outbound connection.
	Dial(ctx context.Context, addr string) (net.Conn, error)
	// Listen returns a TLS-secured listener.
	Listen(addr string) (net.Listener, error)
	// ServerTLSConfig returns the underlying server TLS config.
	ServerTLSConfig() *tls.Config
	// ClientTLSConfig returns the underlying client TLS config.
	ClientTLSConfig() *tls.Config
}

type transport struct {
	serverCfg *tls.Config
	clientCfg *tls.Config
}

// NewTransport creates a Transport from a TLSProfile.
func NewTransport(profile *pkgtls.TLSProfile) (Transport, error) {
	serverCfg, err := profile.ServerTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("transport: %w", err)
	}

	clientCfg, err := profile.ClientTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("transport: %w", err)
	}

	return &transport{
		serverCfg: serverCfg,
		clientCfg: clientCfg,
	}, nil
}

func (t *transport) Dial(ctx context.Context, addr string) (net.Conn, error) {
	dialer := &tls.Dialer{Config: t.clientCfg}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("transport: %w", err)
	}
	return conn, nil
}

func (t *transport) Listen(addr string) (net.Listener, error) {
	ln, err := tls.Listen("tcp", addr, t.serverCfg)
	if err != nil {
		return nil, fmt.Errorf("transport: %w", err)
	}
	return ln, nil
}

func (t *transport) ServerTLSConfig() *tls.Config {
	return t.serverCfg
}

func (t *transport) ClientTLSConfig() *tls.Config {
	return t.clientCfg
}
