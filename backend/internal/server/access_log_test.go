package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithAccessLog(t *testing.T) {
	tests := map[string]struct {
		method     string
		path       string
		handler    http.HandlerFunc
		wantStatus int
	}{
		"ステータスを書かずに返したリクエストは 200 で出す": {
			method:     http.MethodGet,
			path:       "/",
			handler:    func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) },
			wantStatus: http.StatusOK,
		},
		"エラーを返したリクエストは、そのステータスで出す": {
			method:     http.MethodPost,
			path:       "/v1/carts/cart_01/checkout",
			handler:    func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadRequest) },
			wantStatus: http.StatusBadRequest,
		},
		"クエリはパスに含めない": {
			method:     http.MethodGet,
			path:       "/?sleep=abc",
			handler:    func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadRequest) },
			wantStatus: http.StatusBadRequest,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			restore := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
			t.Cleanup(func() { slog.SetDefault(restore) })

			h := WithAccessLog(tt.handler, "v1")
			req := httptest.NewRequestWithContext(t.Context(), tt.method, tt.path, nil)
			h.ServeHTTP(httptest.NewRecorder(), req)

			var got map[string]any
			if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
				t.Fatalf("log is not a JSON line: %v: %q", err, buf.String())
			}
			want := map[string]any{
				"msg":     "access",
				"method":  tt.method,
				"path":    req.URL.Path,
				"status":  float64(tt.wantStatus),
				"version": "v1",
			}
			for k, v := range want {
				if got[k] != v {
					t.Errorf("%s = %v, want %v", k, got[k], v)
				}
			}
			if _, ok := got["duration_ms"]; !ok {
				t.Errorf("duration_ms is missing: %v", got)
			}
		})
	}
}
