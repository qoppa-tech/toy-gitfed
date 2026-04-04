package tls

import (
	"testing"
)

func TestNewACMEStore(t *testing.T) {
	dir := t.TempDir()

	store, err := NewACMEStore(ACMEConfig{
		Email:    "test@example.com",
		Domains:  []string{"example.com"},
		CacheDir: dir,
	})
	if err != nil {
		t.Fatalf("NewACMEStore: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestACMEStoreGetCACertPoolReturnsNil(t *testing.T) {
	dir := t.TempDir()

	store, err := NewACMEStore(ACMEConfig{
		Email:    "test@example.com",
		Domains:  []string{"example.com"},
		CacheDir: dir,
	})
	if err != nil {
		t.Fatalf("NewACMEStore: %v", err)
	}

	// ACME store uses system roots; GetCACertPool returns nil.
	pool, err := store.GetCACertPool()
	if err != nil {
		t.Fatalf("GetCACertPool: %v", err)
	}
	if pool != nil {
		t.Fatal("expected nil pool (system roots)")
	}
}

func TestACMEStoreEmptyDomains(t *testing.T) {
	dir := t.TempDir()

	_, err := NewACMEStore(ACMEConfig{
		Email:    "test@example.com",
		Domains:  []string{},
		CacheDir: dir,
	})
	if err == nil {
		t.Fatal("expected error for empty domains")
	}
}
