package logger

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddleware_InjectsFields(t *testing.T) {
	var buf bytes.Buffer
	l := newTestLogger(&buf, "PROD", "info")

	var captured Logger
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	mw := Middleware(l)(inner)
	req := httptest.NewRequest("GET", "/auth/google", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	if captured == nil {
		t.Fatal("middleware did not inject logger into context")
	}

	// Check "request started" log line.
	lines := splitJSONLines(buf.Bytes())
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 log lines, got %d", len(lines))
	}

	started := lines[0]
	if started["msg"] != "request started" {
		t.Errorf("msg = %v, want 'request started'", started["msg"])
	}
	if started["method"] != "GET" {
		t.Errorf("method = %v, want GET", started["method"])
	}
	if started["path"] != "/auth/google" {
		t.Errorf("path = %v, want /auth/google", started["path"])
	}
	if started["ip"] != "10.0.0.1" {
		t.Errorf("ip = %v, want 10.0.0.1", started["ip"])
	}
	if started["request_id"] == nil || started["request_id"] == "" {
		t.Error("missing request_id")
	}
}

func TestMiddleware_LogsCompletion(t *testing.T) {
	var buf bytes.Buffer
	l := newTestLogger(&buf, "PROD", "info")

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	mw := Middleware(l)(inner)
	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	lines := splitJSONLines(buf.Bytes())
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 log lines, got %d", len(lines))
	}

	completed := lines[len(lines)-1]
	if completed["msg"] != "request completed" {
		t.Errorf("msg = %v, want 'request completed'", completed["msg"])
	}
	// JSON numbers decode as float64.
	if status, ok := completed["status"].(float64); !ok || int(status) != 404 {
		t.Errorf("status = %v, want 404", completed["status"])
	}
	if completed["duration"] == nil || completed["duration"] == "" {
		t.Error("missing duration")
	}
}

func TestMiddleware_HandlerCanEnrichLogger(t *testing.T) {
	var buf bytes.Buffer
	l := newTestLogger(&buf, "PROD", "info")

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqLog := FromContext(r.Context())
		enriched := reqLog.With("user_id", "u-123")
		ctx := WithContext(r.Context(), enriched)
		// Simulate downstream using enriched context.
		FromContext(ctx).Info("downstream log")
		w.WriteHeader(http.StatusOK)
	})

	mw := Middleware(l)(inner)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	lines := splitJSONLines(buf.Bytes())
	// Find the "downstream log" line.
	var found map[string]any
	for _, line := range lines {
		if line["msg"] == "downstream log" {
			found = line
			break
		}
	}
	if found == nil {
		t.Fatal("downstream log line not found")
	}
	if found["user_id"] != "u-123" {
		t.Errorf("user_id = %v, want u-123", found["user_id"])
	}
	if found["request_id"] == nil {
		t.Error("downstream log should still carry request_id")
	}
}

func splitJSONLines(data []byte) []map[string]any {
	var result []map[string]any
	dec := json.NewDecoder(bytes.NewReader(data))
	for dec.More() {
		var entry map[string]any
		if err := dec.Decode(&entry); err != nil {
			break
		}
		result = append(result, entry)
	}
	return result
}
