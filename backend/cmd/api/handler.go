package main

import (
	"encoding/json"
	"net/http"
	"time"
)

// / の sleep で待たせられる上限
//   - shutdownTimeout より短くし、終了処理の確認で待ち切れるようにする
const maxSleep = 15 * time.Second

func newMux(version string) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("GET /{$}", handleVersion(version))
	mux.HandleFunc("POST /v1/carts/{cart_id}/checkout", handleCheckout(version))
	return mux
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok\n"))
}

// バージョン番号を返す
//   - ?sleep=5s のように指定すると、その時間だけ待ってから返す (SIGTERM を受けたときに、処理中のリクエストを終えてから止まるかを確かめるため)
func handleVersion(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s := r.URL.Query().Get("sleep"); s != "" {
			d, err := time.ParseDuration(s)
			if err != nil || d < 0 || d > maxSleep {
				http.Error(w, "sleep must be a duration between 0s and "+maxSleep.String(), http.StatusBadRequest)
				return
			}
			select {
			case <-time.After(d):
			case <-r.Context().Done():
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"version": version})
	}
}
