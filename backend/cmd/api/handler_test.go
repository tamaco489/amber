package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMux(t *testing.T) {
	mux := newMux("v1")

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantBody   string
	}{
		{name: "health", method: http.MethodGet, path: "/health", wantStatus: http.StatusOK, wantBody: "ok\n"},
		{name: "version", method: http.MethodGet, path: "/", wantStatus: http.StatusOK, wantBody: `{"version":"v1"}` + "\n"},
		{name: "version with sleep", method: http.MethodGet, path: "/?sleep=10ms", wantStatus: http.StatusOK, wantBody: `{"version":"v1"}` + "\n"},
		{name: "invalid sleep", method: http.MethodGet, path: "/?sleep=abc", wantStatus: http.StatusBadRequest},
		{name: "sleep too long", method: http.MethodGet, path: "/?sleep=1h", wantStatus: http.StatusBadRequest},
		{name: "unknown path", method: http.MethodGet, path: "/unknown", wantStatus: http.StatusNotFound},
		{name: "method not allowed", method: http.MethodPost, path: "/", wantStatus: http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), tt.method, tt.path, nil))

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantBody != "" && rec.Body.String() != tt.wantBody {
				t.Errorf("body = %q, want %q", rec.Body.String(), tt.wantBody)
			}
			if tt.wantStatus == http.StatusBadRequest && !strings.Contains(rec.Body.String(), "sleep") {
				t.Errorf("body = %q, want an error about sleep", rec.Body.String())
			}
		})
	}
}
