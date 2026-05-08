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
	req.Header.Set("User-Agent", "gitfed-test")
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	if captured == nil {
		t.Fatal("middleware did not inject logger into context")
	}

	lines := splitJSONLines(buf.Bytes())
	if len(lines) < 1 {
		t.Fatalf("expected at least 1 log line, got %d", len(lines))
	}

	completed := lines[0]
	if completed["msg"] != "request completed" {
		t.Errorf("msg = %v, want 'request completed'", completed["msg"])
	}
	if completed["method"] != "GET" {
		t.Errorf("method = %v, want GET", completed["method"])
	}
	if completed["path"] != "/auth/google" {
		t.Errorf("path = %v, want /auth/google", completed["path"])
	}
	if completed["remote_ip"] != "10.0.0.1" {
		t.Errorf("remote_ip = %v, want 10.0.0.1", completed["remote_ip"])
	}
	if completed["user_agent"] != "gitfed-test" {
		t.Errorf("user_agent = %v, want gitfed-test", completed["user_agent"])
	}
	if completed["request_id"] == nil || completed["request_id"] == "" {
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
	if len(lines) < 1 {
		t.Fatalf("expected at least 1 log line, got %d", len(lines))
	}

	completed := lines[0]
	if completed["msg"] != "request completed" {
		t.Errorf("msg = %v, want 'request completed'", completed["msg"])
	}
	// JSON numbers decode as float64.
	if status, ok := completed["status"].(float64); !ok || int(status) != 404 {
		t.Errorf("status = %v, want 404", completed["status"])
	}
	if completed["duration_ms"] == nil {
		t.Error("missing duration_ms")
	}
	if bytesOut, ok := completed["bytes_out"].(float64); !ok || int(bytesOut) < 0 {
		t.Errorf("bytes_out = %v, want non-negative number", completed["bytes_out"])
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

func TestMiddleware_UsesInboundRequestID(t *testing.T) {
	var buf bytes.Buffer
	l := newTestLogger(&buf, "PROD", "info")

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	mw := Middleware(l)(inner)
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("X-Request-Id", "req-123")
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	lines := splitJSONLines(buf.Bytes())
	if len(lines) != 1 {
		t.Fatalf("expected 1 log line, got %d", len(lines))
	}
	if lines[0]["request_id"] != "req-123" {
		t.Fatalf("request_id = %v, want req-123", lines[0]["request_id"])
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
