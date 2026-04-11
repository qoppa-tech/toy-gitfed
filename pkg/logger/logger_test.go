package logger

import "testing"

// Compile-time check: defaultLogger implements Logger.
var _ Logger = (*defaultLogger)(nil)

func TestNew_ReturnsLogger(t *testing.T) {
	l := New(Config{Env: "DEV", Level: "info"})
	if l == nil {
		t.Fatal("New returned nil")
	}
}
