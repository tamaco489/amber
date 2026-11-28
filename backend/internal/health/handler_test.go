package health

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	rec := httptest.NewRecorder()
	Check(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "ok\n" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "ok\n")
	}
}

func TestFailingCheck(t *testing.T) {
	rec := httptest.NewRecorder()
	FailingCheck(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if want := "fail-health: this version fails the health check on purpose\n"; rec.Body.String() != want {
		t.Errorf("body = %q, want %q", rec.Body.String(), want)
	}
}

func TestVersion(t *testing.T) {
	tests := map[string]struct {
		path       string
		wantStatus int
		wantBody   string
	}{
		"バージョン番号を返す": {
			path:       "/",
			wantStatus: http.StatusOK,
			wantBody:   `{"version":"v1"}` + "\n",
		},
		"sleep の分だけ待ってから、バージョン番号を返す": {
			path:       "/?sleep=10ms",
			wantStatus: http.StatusOK,
			wantBody:   `{"version":"v1"}` + "\n",
		},
		"sleep が時間の形でなければ 400 を返す": {
			path:       "/?sleep=abc",
			wantStatus: http.StatusBadRequest,
		},
		"sleep が上限を超えれば 400 を返す": {
			path:       "/?sleep=1h",
			wantStatus: http.StatusBadRequest,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			Version("v1").ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.path, nil))

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantStatus == http.StatusBadRequest {
				if !strings.Contains(rec.Body.String(), "sleep") {
					t.Errorf("body = %q, want an error about sleep", rec.Body.String())
				}
				return
			}
			if rec.Body.String() != tt.wantBody {
				t.Errorf("body = %q, want %q", rec.Body.String(), tt.wantBody)
			}
		})
	}
}
