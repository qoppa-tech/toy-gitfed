package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGoogleCallback_MissingParams(t *testing.T) {
	presenter := &SSOPresenter{}

	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{
			name:       "missing both",
			query:      "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing code",
			query:      "state=abc",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing state",
			query:      "code=abc",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/auth/google/callback?"+tt.query, nil)
			w := httptest.NewRecorder()

			presenter.GoogleCallback(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}
