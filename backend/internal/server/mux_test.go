package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewMux(t *testing.T) {
	tests := map[string]struct {
		method      string
		path        string
		wantStatus  int
		wantHandler string // 呼ばれるハンドラーの名前 (呼ばれないときは空)
	}{
		"GET /health は Health に渡す": {
			method:      http.MethodGet,
			path:        "/health",
			wantStatus:  http.StatusOK,
			wantHandler: "health",
		},
		"GET / は Version に渡す": {
			method:      http.MethodGet,
			path:        "/",
			wantStatus:  http.StatusOK,
			wantHandler: "version",
		},
		"POST の注文の確定は Checkout に渡す": {
			method:      http.MethodPost,
			path:        "/v1/carts/cart_01/checkout",
			wantStatus:  http.StatusOK,
			wantHandler: "checkout",
		},
		"知らないパスは 404 を返す": {
			method:     http.MethodGet,
			path:       "/unknown",
			wantStatus: http.StatusNotFound,
		},
		"POST / は 405 を返す": {
			method:     http.MethodPost,
			path:       "/",
			wantStatus: http.StatusMethodNotAllowed,
		},
		"GET の注文の確定は 405 を返す": {
			method:     http.MethodGet,
			path:       "/v1/carts/cart_01/checkout",
			wantStatus: http.StatusMethodNotAllowed,
		},
		"cart_id の後ろが checkout でなければ 404 を返す": {
			method:     http.MethodPost,
			path:       "/v1/carts/cart_01/pay",
			wantStatus: http.StatusNotFound,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var called string
			record := func(name string) http.HandlerFunc {
				return func(http.ResponseWriter, *http.Request) { called = name }
			}
			mux := NewMux(&Handlers{
				Health:   record("health"),
				Version:  record("version"),
				Checkout: record("checkout"),
			})

			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), tt.method, tt.path, nil))

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if called != tt.wantHandler {
				t.Errorf("called = %q, want %q", called, tt.wantHandler)
			}
		})
	}
}
