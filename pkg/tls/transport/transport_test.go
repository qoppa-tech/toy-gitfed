package transport

import (
	"context"
	"crypto/tls"
	"io"
	"testing"
	"time"

	pkgtls "github.com/qoppa-tech/gitfed/pkg/tls"
)

func newTestTransport(t *testing.T) Transport {
	t.Helper()
	store, err := pkgtls.NewSelfSignedStore(pkgtls.SelfSignedConfig{
		Hosts:    []string{"localhost", "127.0.0.1"},
		Duration: 1 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	profile := &pkgtls.TLSProfile{
		ServerCert: store,
		MinVersion: tls.VersionTLS13,
	}
	tr, err := NewTransport(profile)
	if err != nil {
		t.Fatal(err)
	}
	return tr
}

func TestNewTransport(t *testing.T) {
	tr := newTestTransport(t)
	if tr == nil {
		t.Fatal("expected non-nil transport")
	}
}

func TestTransportServerTLSConfig(t *testing.T) {
	tr := newTestTransport(t)
	cfg := tr.ServerTLSConfig()
	if cfg == nil {
		t.Fatal("expected non-nil server TLS config")
	}
	if cfg.MinVersion != tls.VersionTLS13 {
		t.Fatalf("expected TLS 1.3, got %d", cfg.MinVersion)
	}
}

func TestTransportClientTLSConfig(t *testing.T) {
	tr := newTestTransport(t)
	cfg := tr.ClientTLSConfig()
	if cfg == nil {
		t.Fatal("expected non-nil client TLS config")
	}
}

func TestTransportListenAndDial(t *testing.T) {
	store, err := pkgtls.NewSelfSignedStore(pkgtls.SelfSignedConfig{
		Hosts:    []string{"localhost", "127.0.0.1"},
		Duration: 1 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	profile := &pkgtls.TLSProfile{
		ServerCert: store,
		MinVersion: tls.VersionTLS13,
	}
	tr, err := NewTransport(profile)
	if err != nil {
		t.Fatal(err)
	}

	ln, err := tr.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	payload := []byte("hello tls")
	done := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		_, err = conn.Write(payload)
		done <- err
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := tr.Dial(ctx, ln.Addr().String())
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	buf, err := io.ReadAll(conn)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(buf) != string(payload) {
		t.Fatalf("expected %q, got %q", payload, buf)
	}

	if err := <-done; err != nil {
		t.Fatalf("server: %v", err)
	}
}

func TestTransportMTLSListenAndDial(t *testing.T) {
	serverStore, err := pkgtls.NewSelfSignedStore(pkgtls.SelfSignedConfig{
		Hosts:    []string{"localhost", "127.0.0.1"},
		Duration: 1 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	clientStore, err := pkgtls.NewSelfSignedStore(pkgtls.SelfSignedConfig{
		Hosts:    []string{"localhost", "127.0.0.1"},
		Duration: 1 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}

	serverProfile := &pkgtls.TLSProfile{
		ServerCert: serverStore,
		ClientCA:   clientStore,
		MinVersion: tls.VersionTLS13,
		VerifyPeer: true,
	}
	serverTr, err := NewTransport(serverProfile)
	if err != nil {
		t.Fatal(err)
	}

	clientProfile := &pkgtls.TLSProfile{
		ServerCert: serverStore,
		ClientCert: clientStore,
		MinVersion: tls.VersionTLS13,
	}
	clientTr, err := NewTransport(clientProfile)
	if err != nil {
		t.Fatal(err)
	}

	ln, err := serverTr.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	payload := []byte("mtls hello")
	done := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		_, err = conn.Write(payload)
		done <- err
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := clientTr.Dial(ctx, ln.Addr().String())
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	buf, err := io.ReadAll(conn)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(buf) != string(payload) {
		t.Fatalf("expected %q, got %q", payload, buf)
	}

	if err := <-done; err != nil {
		t.Fatalf("server: %v", err)
	}
}
