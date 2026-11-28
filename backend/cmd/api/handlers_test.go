package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tamaco489/amber/backend/internal/failmode"
	"github.com/tamaco489/amber/backend/internal/server"
)

func TestNewHandlers(t *testing.T) {
	tests := map[string]struct {
		mode       failmode.Mode
		path       string
		wantStatus int
	}{
		"fail-health のとき /health は 500 を返す": {
			mode:       failmode.Health,
			path:       "/health",
			wantStatus: http.StatusInternalServerError,
		},
		"fail-health のとき / は 200 を返す": {
			mode:       failmode.Health,
			path:       "/",
			wantStatus: http.StatusOK,
		},
		"normal のとき /health は 200 を返す": {
			mode:       failmode.Normal,
			path:       "/health",
			wantStatus: http.StatusOK,
		},
		"normal のとき / は 200 を返す": {
			mode:       failmode.Normal,
			path:       "/",
			wantStatus: http.StatusOK,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			mux := server.NewMux(newHandlers("v9", tt.mode))

			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.path, nil))

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}
