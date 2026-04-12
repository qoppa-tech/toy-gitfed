package testutil

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWaitForHTTPSucceedsOnRegisterRouteStatus(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/auth/register" {
			http.NotFound(w, r)
			return
		}

		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad json"))
	}))
	t.Cleanup(srv.Close)

	httpAddr := strings.TrimPrefix(srv.URL, "http://")

	err := WaitForHTTP(httpAddr, time.Second)
	if err != nil {
		t.Fatalf("expected readiness success, got error: %v", err)
	}
}

func TestWaitForHTTPTimesOutWhenRouteMissing(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)

	httpAddr := strings.TrimPrefix(srv.URL, "http://")

	err := WaitForHTTP(httpAddr, 300*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error when register route is missing")
	}
}
