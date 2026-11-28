package checkout

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandle(t *testing.T) {
	tests := map[string]struct {
		body       string
		wantStatus int
		wantBody   map[string]any // order_id を除いて比べる
	}{
		"必須の項目がそろっていれば、サーバーの単価で合計を返す": {
			body:       `{"items":[{"sku":"TSHIRT-BLK-M","quantity":2,"unit_price":1}],"shipping_address_id":"addr_01"}`,
			wantStatus: http.StatusOK,
			wantBody:   map[string]any{"total": float64(5960), "version": "v7"},
		},
		"unit_price が無ければ、400 と足りない項目を返す": {
			body:       `{"items":[{"sku":"TSHIRT-BLK-M","quantity":2}],"shipping_address_id":"addr_01"}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   map[string]any{"error": "items[0].unit_price is required", "version": "v7"},
		},
		"unit_price が null なら、無いものとして 400 を返す": {
			body:       `{"items":[{"sku":"TSHIRT-BLK-M","quantity":2,"unit_price":null}],"shipping_address_id":"addr_01"}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   map[string]any{"error": "items[0].unit_price is required", "version": "v7"},
		},
		"本文が JSON でなければ 400 を返す": {
			body:       `not json`,
			wantStatus: http.StatusBadRequest,
			wantBody:   map[string]any{"error": "body must be a JSON object", "version": "v7"},
		},
		"項目の型が違えば 400 を返す": {
			body:       `{"items":[{"sku":"TSHIRT-BLK-M","quantity":"2","unit_price":2980}],"shipping_address_id":"addr_01"}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   map[string]any{"error": "body must be a JSON object", "version": "v7"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/carts/cart_01/checkout", strings.NewReader(tt.body))
			Handle("v7").ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}

			var got map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("body is not JSON: %v: %q", err, rec.Body.String())
			}
			for k, v := range tt.wantBody {
				if got[k] != v {
					t.Errorf("%s = %v, want %v", k, got[k], v)
				}
			}
			if tt.wantStatus != http.StatusOK {
				return
			}
			if id, _ := got["order_id"].(string); !strings.HasPrefix(id, "ord_") {
				t.Errorf("order_id = %v, want a value starting with ord_", got["order_id"])
			}
		})
	}
}
