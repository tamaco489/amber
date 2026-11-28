package main

import (
	"log/slog"
	"net/http"
	"time"
)

// withAccessLog は、リクエストごとに 1 行のアクセスログを出す
//   - 返し終えたあとに出す。SIGTERM を受けたあとに処理を終えたリクエストも、ここで出る
//   - /health (ALB のヘルスチェック) も出す。登録解除の間に、ヘルスチェックがいつ止まったかを見るため
func withAccessLog(next http.Handler, version string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		slog.Info("access",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"version", version,
		)
	})
}

// statusRecorder は、ハンドラーが返したステータスを覚える
//   - WriteHeader を呼ばずに書いたときは 200 になるため、既定は 200 にする
type statusRecorder struct {
	http.ResponseWriter
	status int // 返したステータス
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// Unwrap は、http.ResponseController が元の ResponseWriter に届くようにする
//
// DOC: https://pkg.go.dev/net/http#NewResponseController
func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}
