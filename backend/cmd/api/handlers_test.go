package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tamaco489/amber/backend/internal/failmode"
	"github.com/tamaco489/amber/backend/internal/server"
)

const checkoutBody = `{"items":[{"sku":"TSHIRT-BLK-M","quantity":2}],"shipping_address_id":"addr_01"}`

func TestNewHandlers(t *testing.T) {
	tests := map[string]struct {
		mode       failmode.Mode
		method     string
		path       string
		wantStatus int
	}{
		"fail-health のとき /health は 500 を返す": {
			mode:       failmode.Health,
			method:     http.MethodGet,
			path:       "/health",
			wantStatus: http.StatusInternalServerError,
		},
		"fail-health のとき / は 200 を返す": {
			mode:       failmode.Health,
			method:     http.MethodGet,
			path:       "/",
			wantStatus: http.StatusOK,
		},
		"fail-500 のとき /health は 200 を返す": {
			mode:       failmode.Fail500,
			method:     http.MethodGet,
			path:       "/health",
			wantStatus: http.StatusOK,
		},
		"fail-500 のとき / は 500 を返す": {
			mode:       failmode.Fail500,
			method:     http.MethodGet,
			path:       "/",
			wantStatus: http.StatusInternalServerError,
		},
		"fail-500 のとき注文の確定は 500 を返す": {
			mode:       failmode.Fail500,
			method:     http.MethodPost,
			path:       "/v1/carts/cart_01/checkout",
			wantStatus: http.StatusInternalServerError,
		},
		"normal のとき /health は 200 を返す": {
			mode:       failmode.Normal,
			method:     http.MethodGet,
			path:       "/health",
			wantStatus: http.StatusOK,
		},
		"normal のとき / は 200 を返す": {
			mode:       failmode.Normal,
			method:     http.MethodGet,
			path:       "/",
			wantStatus: http.StatusOK,
		},
		"normal のとき注文の確定は 200 を返す": {
			mode:       failmode.Normal,
			method:     http.MethodPost,
			path:       "/v1/carts/cart_01/checkout",
			wantStatus: http.StatusOK,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			mux := server.NewMux(newHandlers("v9", tt.mode, slog.New(slog.DiscardHandler)))

			req := httptest.NewRequestWithContext(t.Context(), tt.method, tt.path, strings.NewReader(checkoutBody))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestFail500(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))

	rec := httptest.NewRecorder()
	fail500("v10-fail-500", logger).ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/carts/cart_01/checkout", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if want := "fail-500: this version returns 500 on purpose\n"; rec.Body.String() != want {
		t.Errorf("body = %q, want %q", rec.Body.String(), want)
	}

	var got map[string]any
	if err := json.Unmarshal(logs.Bytes(), &got); err != nil {
		t.Fatalf("log is not one JSON line: %v (%q)", err, logs.String())
	}
	want := map[string]string{
		"level":   "ERROR",
		"msg":     "returning 500 on purpose",
		"mode":    "fail-500",
		"method":  http.MethodPost,
		"path":    "/v1/carts/cart_01/checkout",
		"version": "v10-fail-500",
	}
	for key, value := range want {
		if got[key] != value {
			t.Errorf("log %s = %v, want %q", key, got[key], value)
		}
	}
}
