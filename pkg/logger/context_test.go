package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestWithContext_RoundTrip(t *testing.T) {
	var buf bytes.Buffer
	l := newTestLogger(&buf, "PROD", "info")

	ctx := WithContext(context.Background(), l)
	got := FromContext(ctx)
	got.Info("round trip")

	var entry map[string]any
	json.NewDecoder(&buf).Decode(&entry)
	if entry["msg"] != "round trip" {
		t.Errorf("msg = %v, want 'round trip'", entry["msg"])
	}
}

func TestFromContext_FallbackToDefault(t *testing.T) {
	l := FromContext(context.Background())
	if l == nil {
		t.Fatal("FromContext should never return nil")
	}
	// Should not panic.
	l.Info("fallback works")
}

func TestSetDefault(t *testing.T) {
	var buf bytes.Buffer
	l := newTestLogger(&buf, "PROD", "info")

	old := globalDefault
	defer func() { globalDefault = old }()

	SetDefault(l)

	got := FromContext(context.Background())
	got.Info("from default")

	var entry map[string]any
	json.NewDecoder(&buf).Decode(&entry)
	if entry["msg"] != "from default" {
		t.Errorf("msg = %v, want 'from default'", entry["msg"])
	}
}
